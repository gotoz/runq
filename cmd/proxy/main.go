package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/gotoz/runq/internal/cfg"
	"github.com/gotoz/runq/internal/util"
	"github.com/gotoz/runq/internal/vs"
	"github.com/gotoz/runq/pkg/vm"
	"github.com/pkg/errors"
)

var gitCommit string // set via Makefile

func init() {
	rand.Seed(time.Now().UnixNano())
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
		if err = unix.Mount(d, vm.QemuMountPt+d, "none", unix.MS_MOVE, ""); err != nil {
			return 1, errors.WithStack(err)
		}
	}

	if err = unix.PivotRoot(vm.QemuMountPt, vm.QemuMountPt+"/rootfs"); err != nil {
		return 1, errors.WithStack(err)
	}

	// At this point the container files are in /rootfs.
	// - without rootdisk (default):
	//   /lib/modules will be bind-mounted to /rootfs/lib/modules.
	//   /rootfs will be shared via 9p to the VM.
	// - with rootdisk:
	//   The content of /rootfs will be copied into a block device.
	//   /lib/modules will be bind-mounted to /share
	//   /share will be shared via 9p to the VM.
	var share, modulesMountDir string
	if vmdata.Rootdisk == "" {
		// w/o rootdisk
		share = "/rootfs"
		modulesMountDir = "/rootfs/lib/modules"
	} else {
		// with rootdisk
		if err := prepareRootdisk(vmdata); err != nil {
			return 1, err
		}
		if err := unix.Unmount("/rootfs", unix.MNT_DETACH); err != nil {
			return 1, err
		}
		share = "/share"
		modulesMountDir = "/share"
	}

	if err = bindMountKernelModules(modulesMountDir); err != nil {
		return 1, err
	}

	// ackChan receives acknowledge messages from init.
	// msgChan to send messages to init.
	const vmsocket = "/dev/runq.sock"
	msgChan, ackChan, err := mkChannel(vmsocket)
	if err != nil {
		return 1, err
	}

	if err := getQemuVersion(vmdata); err != nil {
		return 1, err
	}

	args, err := qemuArgs(vmdata, vmsocket, share)
	if err != nil {
		return 1, err
	}

	var extraFiles []*os.File
	for _, nw := range vmdata.Networks {
		f, err := os.OpenFile(nw.TapDevice, os.O_RDWR, 0600|os.ModeExclusive)
		if err != nil {
			return 1, errors.WithStack(err)
		}
		extraFiles = append(extraFiles, f)
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
	timeout := time.Second * time.Duration(10+vmdata.Mem/2048)
	select {
	case <-ackChan:
		// all good
	case err = <-doneChan:
		// qemu exited too early
		return 1, err
	case <-time.After(timeout):
		// init didn't send ack message in time
		cmd.Process.Kill()
		msg := fmt.Sprintf("no ack msg from init within %.0f sec", timeout.Seconds())
		if !vmdata.NoExec {
			msg += ". Possibly not enough entropy."
		}
		return 1, fmt.Errorf(msg)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, cfg.Signals...)
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

	// Disalow foreign processes.
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
	if vmdata.Mem < cfg.MinMem {
		return fmt.Errorf("invalid value for memory: %d", vmdata.Mem)
	}

	for _, v := range os.Environ() {
		if strings.HasPrefix(v, "HOME=") {
			continue
		}
		vmdata.Entrypoint.Env = append(vmdata.Entrypoint.Env, v)
	}
	vmdata.Hostname = os.Getenv("HOSTNAME")
	home := util.UserHome(int(vmdata.Entrypoint.UID))
	vmdata.Entrypoint.Env = append(vmdata.Entrypoint.Env, fmt.Sprintf("HOME=%s", home))
	sort.Strings(vmdata.Entrypoint.Env)

	val, ok := os.LookupEnv("RUNQ_DNS")
	if ok {
		vmdata.DNS.Server = []string{}
		for _, v := range strings.Split(val, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			vmdata.DNS.Server = append(vmdata.DNS.Server, v)
		}
	}
	val, ok = os.LookupEnv("RUNQ_DNS_OPTS")
	if ok {
		vmdata.DNS.Options = val
	}
	val, ok = os.LookupEnv("RUNQ_DNS_SEARCH")
	if ok {
		vmdata.DNS.Search = val
	}
	vmdata.DNS.Preserve = util.ToBool(os.Getenv("RUNQ_DNS_PRESERVE"))

	err = writeResolvConf(vmdata.DNS)
	if err != nil {
		return err
	}

	val, ok = os.LookupEnv("RUNQ_ROOTDISK")
	if ok {
		vmdata.Rootdisk = val
	}
	val, ok = os.LookupEnv("RUNQ_ROOTDISK_EXCLUDE")
	if ok {
		for _, v := range strings.Split(val, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			vmdata.RootdiskExclude = append(vmdata.RootdiskExclude, v)
		}
	}

	vmdata.Networks, err = setupNetwork()
	if err != nil {
		return err
	}

	if err := updateDisks(vmdata.Disks); err != nil {
		return err
	}

	// runq_exec can be disabled globally in daemon.json via the "--noexec" flag
	// or via the container env variable "RUNQ_NOEXEC" with a true value.
	if vmdata.NoExec == false {
		vmdata.NoExec = util.ToBool(os.Getenv("RUNQ_NOEXEC"))
	}
	if !vmdata.NoExec {
		// CID (uint32) is taken from the first 8 characters of the Docker container ID.
		// In the unlikely event that there is already a container with a container ID that
		// begins with the same 8 characters an error will be thrown: "unable to set guest
		// cid..." and the container fails to start.
		vmdata.Vsockd.CID, err = vs.ContextID(vmdata.ContainerID)
		if err != nil {
			return err
		}
		if vmdata.Vsockd.CACert, err = ioutil.ReadFile(vm.QemuMountPt + "/certs/ca.pem"); err != nil {
			return err
		}
		if vmdata.Vsockd.Cert, err = ioutil.ReadFile(vm.QemuMountPt + "/certs/cert.pem"); err != nil {
			return err
		}
		if vmdata.Vsockd.Key, err = ioutil.ReadFile(vm.QemuMountPt + "/certs/key.pem"); err != nil {
			return err
		}
		vmdata.Vsockd.EntrypointEnv = make([]string, len(vmdata.Entrypoint.Env))
		copy(vmdata.Vsockd.EntrypointEnv, vmdata.Entrypoint.Env)
	}

	if val, ok = os.LookupEnv("RUNQ_RUNQENV"); ok {
		vmdata.Entrypoint.Runqenv = util.ToBool(val)
	}
	vmdata.Entrypoint.Systemd = util.ToBool(os.Getenv("RUNQ_SYSTEMD"))

	// https://www.kernel.org/doc/html/latest/_sources/filesystems/9p.rst.txt
	// default 9p chache mode 'mmap' is set in runc
	if val, ok = os.LookupEnv("RUNQ_9PCACHE"); ok {
		switch val {
		case "none", "loose", "fscache", "mmap":
			vmdata.Cache9p = val
		default:
			return fmt.Errorf("env RUNQ_9PCACHE: invalid value %q, want (none|loose|fscache|mmap)", val)
		}
	}

	// default cpuargs 'host' is set in runc
	if val, ok = os.LookupEnv("RUNQ_CPUARGS"); ok {
		if val == "" {
			return fmt.Errorf("env RUNQ_CPUARGS must not be empty")
		}
		vmdata.CPUArgs = val
	}

	arg0 := vmdata.Entrypoint.Args[0]
	if arg0 == "/dev/init" {
		if _, err := os.Stat(arg0); err == nil {
			vmdata.Entrypoint.DockerInit = arg0
		}
	} else if arg0 == "/sbin/docker-init" { // since docker 19.03
		if _, err := os.Stat(arg0); err == nil {
			vmdata.Entrypoint.DockerInit = arg0
		}
	}

	if vmdata.MachineType, err = util.MachineType(); err != nil {
		return err
	}

	os.Clearenv()
	return nil
}

func bindMountKernelModules(dest string) error {
	var src = "/lib/modules"
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

func getQemuVersion(vmdata *vm.Data) error {
	var exe string
	switch runtime.GOARCH {
	case "amd64":
		exe = "/usr/bin/qemu-system-x86_64"
	case "s390x":
		exe = "/usr/bin/qemu-system-s390x"
	default:
		return fmt.Errorf("%s not supported", runtime.GOARCH)
	}
	cmd := exec.Command(exe, "-version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`^QEMU emulator version (\d+\.\d+\.\d+)`)
	match := re.FindStringSubmatch(string(out))
	if match == nil || len(match) < 2 {
		return fmt.Errorf("can't find Qemu version in %q", string(out))
	}
	vmdata.QemuVersion = match[1]
	return nil
}
