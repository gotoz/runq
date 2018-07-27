package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"
)

var gitCommit string // set via Makefile

func init() {
	log.SetPrefix(fmt.Sprintf("[%s %s] ", filepath.Base(os.Args[0]), gitCommit))
	log.SetFlags(0)
}

func main() {
	version := flag.Bool("version", false, "output version and exit")
	_ = flag.String("name", "", "") // dummy for fixed container id

	flag.Parse()
	if *version {
		fmt.Printf("%s (%s)\n", gitCommit, runtime.Version())
		os.Exit(0)
	}
	if len(flag.Args()) != 1 {
		log.Print("invalid/missing arguments")
		os.Exit(1)
	}
	if os.Getpid() != 1 {
		log.Print("Error: must run as PID 1 in container")
		flag.Usage()
		os.Exit(1)
	}

	// First non-flag argument is vmdata encoded in Base64.
	rc, err := run(flag.Args()[0])
	if err != nil {
		log.Printf("%+v", err)
	}
	os.Exit(rc)
}

func run(vmdataB64 string) (int, error) {
	vmdata, err := vm.ZipDecodeBase64(vmdataB64)
	if err != nil {
		return 1, errors.WithStack(err)
	}

	// Ensure runq and proxy are built from same commit.
	if gitCommit == "" || vmdata.GitCommit == "" || vmdata.GitCommit != gitCommit {
		return 1, fmt.Errorf("binary missmatch runq:%q proxy:%q", vmdata.GitCommit, gitCommit)
	}

	if err = completeVmdata(vmdata); err != nil {
		return 1, err
	}

	for _, d := range []string{"/dev", "/proc", "/sys"} {
		if err = unix.Mount(d, "/qemu"+d, "none", unix.MS_MOVE, ""); err != nil {
			return 1, errors.WithStack(err)
		}
	}

	if err = unix.PivotRoot("/qemu", "/qemu/rootfs"); err != nil {
		return 1, errors.WithStack(err)
	}

	if err = bindMountKernelModules(); err != nil {
		return 1, err
	}

	// ackChan receives acknowledge messages from init.
	// msgChan to send messages to init.
	const vmsocket = "/dev/runq.sock"
	msgChan, ackChan, err := mkChannel(vmsocket)
	if err != nil {
		return 1, err
	}

	args, extraFiles, err := qemuConfig(vmdata, vmsocket)
	if err != nil {
		return 1, err
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
	cmd.Dir = "/"
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = extraFiles

	if err = cmd.Start(); err != nil {
		return 1, errors.WithStack(err)
	}

	// doneChan receives the cmd exit code.
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- cmd.Wait()
	}()

	// send Vmdata message to init.
	data, err := vm.Encode(vmdata)
	if err != nil {
		return 1, errors.WithStack(err)
	}
	msgChan <- vm.Msg{
		Type: vm.Vmdata,
		Data: data,
	}

	// wait for ack message, early failure, or timeout
	select {
	case <-ackChan:
		// all good
	case err = <-doneChan:
		// qemu exited too early
		return 1, err
	case <-time.After(time.Second * 10):
		// init didn't send ack message in time
		cmd.Process.Kill()
		return 1, fmt.Errorf("no ack msg from init within 10 sec")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, vm.Signals...)
	go func() {
		for {
			sig := <-sigChan
			log.Printf("forwarding signal '%#v' to init", sig)
			msgChan <- vm.Msg{
				Type: vm.Signal,
				Data: []byte{uint8(sig.(syscall.Signal))},
			}
		}
	}()

	// Prevent anything from running inside our container.
	if err = unix.Mount("/", "/", "none", unix.MS_REMOUNT|unix.MS_NOEXEC, ""); err != nil {
		return 1, errors.WithStack(err)
	}

	// Wait for VM to finish.
	if err = <-doneChan; err != nil {
		return 1, err
	}
	for _, f := range cmd.ExtraFiles {
		if err = f.Close(); err != nil {
			return 1, err
		}
	}

	// Wait for exit code sent by init.
	var rc = 1
	select {
	case rc = <-ackChan:
	case <-time.After(time.Millisecond * 100):
		err = fmt.Errorf("no exit code received from init")
	}
	return rc, err
}

func completeVmdata(vmdata *vm.Data) error {
	var err error

	if v := os.Getenv("RUNQ_CPU"); v != "" {
		if vmdata.CPU, err = strconv.Atoi(v); err != nil {
			return fmt.Errorf("invalid value for cpu: %s", v)
		}
	}
	if vmdata.CPU < 1 {
		return fmt.Errorf("invalid value for cpu: %d", vmdata.CPU)
	}

	if v := os.Getenv("RUNQ_MEM"); v != "" {
		if vmdata.Mem, err = strconv.Atoi(v); err != nil {
			return fmt.Errorf("invalid value for memory: %s", v)
		}
	}
	if vmdata.Mem < vm.MinMem {
		return fmt.Errorf("invalid value for memory: %d", vmdata.Mem)
	}

	for _, v := range os.Environ() {
		if strings.HasPrefix(v, "HOME=") {
			continue
		}
		vmdata.Env = append(vmdata.Env, v)
	}
	vmdata.Hostname = os.Getenv("HOSTNAME")
	home := util.UserHome(int(vmdata.UID))
	vmdata.Env = append(vmdata.Env, fmt.Sprintf("HOME=%s", home))

	vmdata.Networks, err = setupNetwork()
	if err != nil {
		return err
	}

	val, ok := os.LookupEnv("RUNQ_DNS")
	if ok {
		vmdata.DNS = []string{}
		for _, v := range strings.Split(val, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			vmdata.DNS = append(vmdata.DNS, v)
		}
	}
	val, ok = os.LookupEnv("RUNQ_DNS_OPTS")
	if ok {
		vmdata.DNSOpts = val
	}
	val, ok = os.LookupEnv("RUNQ_DNS_SEARCH")
	if ok {
		vmdata.DNSSearch = val
	}

	err = setupDNS(vmdata.DNS, vmdata.DNSOpts, vmdata.DNSSearch)
	if err != nil {
		return err
	}

	if err := updateDisks(vmdata.Disks); err != nil {
		return err
	}

	os.Clearenv()
	return nil
}

func bindMountKernelModules() error {
	var src = "/lib/modules"
	var dest = "/rootfs/lib/modules"
	if !util.DirExists(src) {
		return nil
	}
	if !util.DirExists(dest) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return errors.WithStack(err)
		}
	}
	err := unix.Mount(src, dest, "bind", syscall.MS_BIND|syscall.MS_RDONLY, "")
	return errors.WithStack(err)
}
