package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"

	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
)

var (
	gitCommit string        // set via Makefile
	ackChan   chan uint8    // to send acknowledge messages back to proxy
	msgChan   <-chan vm.Msg // to receives messages from proxy
)

func init() {
	log.SetPrefix(fmt.Sprintf("[%s(%d) %s] ", filepath.Base(os.Args[0]), os.Getpid(), gitCommit))
	log.SetFlags(0)
}

func main() {
	switch os.Args[0] {
	case "child":
		mainChild()
		return
	case "vsockd":
		mainVsockd()
		return
	}

	signal.Ignore(unix.SIGTERM, unix.SIGUSR1, unix.SIGUSR2)
	parseCmdline()

	if err := runInit(); err != nil {
		shutdown(1, fmt.Sprintf("%+v", err))
	}
	shutdown(0, "")
}

func runInit() error {
	if err := mountInit(); err != nil {
		return err
	}

	if err := loadKernelModules("base", ""); err != nil {
		return err
	}

	vportDev, err := vportDevice()
	if err != nil {
		return err
	}

	ackChan, msgChan, err = mkChannel(vportDev)
	if err != nil {
		return err
	}

	// Wait for vmdata.
	msg := <-msgChan
	if msg.Type != vm.Vmdata {
		return errors.New("received invalid first message")
	}
	ackChan <- 0
	<-ackChan

	vmdataGob := msg.Data
	vmdata, err := vm.DecodeDataGob(vmdataGob)
	if err != nil {
		return errors.WithStack(err)
	}

	// Ensure runq and init are build from same commit.
	if gitCommit == "" || vmdata.GitCommit == "" || vmdata.GitCommit != gitCommit {
		shutdown(1, fmt.Sprintf("binary missmatch proxy:%q init:%q", vmdata.GitCommit, gitCommit))
	}

	if !vmdata.Terminal {
		if _, err := terminal.MakeRaw(0); err != nil {
			return errors.WithStack(err)
		}
	}

	if err := mountRootfs(vmdata.Mounts); err != nil {
		return err
	}

	if err := mountRootfsCgroups(); err != nil {
		return err
	}

	if err := mountRemount(); err != nil {
		return err
	}

	// Remove empty mountpoint.
	if err := os.Remove("/rootfs/qemu"); err != nil && !os.IsNotExist(err) {
		return errors.WithStack(err)
	}

	if err := setSysctl(vmdata.Sysctl); err != nil {
		return err
	}

	if err := unix.Sethostname([]byte(vmdata.Hostname)); err != nil {
		return errors.WithStack(err)
	}
	if err := setupNetwork(vmdata.Networks); err != nil {
		return err
	}

	if err := setupDisks(vmdata.Disks); err != nil {
		return err
	}

	if err := chroot("/rootfs"); err != nil {
		return err
	}

	if err := maskPath(vm.MaskedPaths); err != nil {
		return err
	}

	if err := readonlyPath(vm.ReadonlyPaths); err != nil {
		return err
	}

	if err := prepareDeviceFiles(int(vmdata.UID)); err != nil {
		return err
	}

	forker := newForker(vmdata.Process)

	// Start reaper to wait4 zombie processes.
	go reaper()

	// Start vsock daemon.
	vsockd := forker.forkVsockd(vmdata.Certificates)
	if err := vsockd.start(); err != nil {
		shutdown(util.ErrorToRc(err))
	}
	if err := ioutil.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", vsockd.pid), []byte("-1000"), 0644); err != nil {
		return err
	}
	go vsockd.wait()

	// Start entry point process.
	entryPoint := forker.forkEntryPoint()
	if err := entryPoint.start(); err != nil {
		shutdown(util.ErrorToRc(err))
	}
	go entryPoint.wait()

	// Main loop to process messages received from proxy.
	for {
		msg := <-msgChan
		switch msg.Type {
		case vm.Signal:
			// Forward signal to application.
			sig := syscall.Signal(int(msg.Data[0]))
			if err := entryPoint.signal(sig); err != nil {
				log.Printf("send signal %#v to %d: %v", sig, entryPoint.pid, err)
			}
		default:
			return errors.Errorf("received invalid message: %v", msg)
		}
	}
}

func vportDevice() (string, error) {
	var vports []os.FileInfo
	var err error
	for i := 0; i < 100; i++ {
		vports, err = ioutil.ReadDir("/sys/class/virtio-ports")
		if len(vports) == 1 {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}
	if err != nil {
		return "", errors.WithStack(err)
	}
	if len(vports) != 1 {
		return "", errors.Errorf("Found %d vports. Expected 1", len(vports))
	}
	return "/dev/" + vports[0].Name(), nil
}

func prepareDeviceFiles(uid int) error {
	if err := os.Chown("/dev/console", uid, 5); err != nil {
		return errors.WithStack(err)
	}
	if err := os.Chmod("/dev/console", 0620); err != nil {
		return errors.WithStack(err)
	}

	m := map[string]string{
		"/proc/kcore":     "/dev/core",
		"/proc/self/fd":   "/dev/fd",
		"/proc/self/fd/0": "/dev/stdin",
		"/proc/self/fd/1": "/dev/stdout",
		"/proc/self/fd/2": "/dev/stderr",
	}
	for k, v := range m {
		if err := os.Symlink(k, v); err != nil {
			return errors.Wrapf(err, "Symlink %s %s:", k, v)
		}
	}

	vports, err := filepath.Glob("/dev/vport*")
	if err != nil {
		return errors.WithStack(err)
	}
	for _, f := range vports {
		os.Remove(f)
	}
	return nil
}

func parseCmdline() {
	showVersion := flag.Bool("version", false, "output version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s (%s)\n", gitCommit, runtime.Version())
		os.Exit(0)
	}
	if len(flag.Args()) > 0 {
		log.Printf("Warning: unexpected arguments: %v", flag.Args())
	}

	if os.Getpid() != 1 {
		log.Print("Error: must run as PID 1")
		os.Exit(1)
	}
}

func loadKernelModules(kind, prefix string) error {
	file, err := os.Open("/kernel.conf")
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || len(line) == 0 {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			return errors.New("invalid kernel config")
		}
		if f[0] != kind {
			continue
		}
		if err := util.Insmod(prefix+f[1], f[2:]); err != nil {
			return err
		}
	}
	return errors.WithStack(scanner.Err())
}

func chroot(dir string) error {
	for _, d := range []string{"/proc", "/sys", "/dev"} {
		if err := unix.Unmount(d, syscall.MNT_DETACH); err != nil {
			return errors.WithStack(err)
		}
	}
	if err := os.Chdir(dir); err != nil {
		return errors.WithStack(err)
	}
	if err := unix.Mount(dir, "/", "", unix.MS_MOVE, ""); err != nil {
		return errors.WithStack(err)
	}
	err := unix.Chroot(".")
	return errors.WithStack(err)
}

func setSysctl(vmdataSysctl map[string]string) error {
	for k, v := range vm.SysctlDefault {
		if err := util.SetSysctl(k, v); err != nil {
			return err
		}
	}
	for k, v := range vmdataSysctl {
		if err := util.SetSysctl(k, v); err != nil {
			return err
		}
	}
	for k, v := range vm.SysctlOverride {
		if err := util.SetSysctl(k, v); err != nil {
			return err
		}
	}
	return nil
}

var once sync.Once

func shutdown(rc uint8, msg string) {
	once.Do(func() {
		if msg != "" {
			log.Print(msg)
		}
		// Send exit code of grandchild to proxy.
		select {
		case ackChan <- rc:
		case <-time.After(time.Millisecond * 100):
			log.Println("ackChan <- rc timed out")
		}

		// Wait for response to ensure message has been sent.
		select {
		case <-ackChan:
		case <-time.After(time.Millisecond * 100):
			log.Println("<- ackChan timed out")
		}

		ch := make(chan int, 1)
		go func() {
			util.SetSysctl("kernel.printk", "0")
			util.Killall()
			unix.Sync()
			umountRootfs()
			ch <- 1
		}()
		select {
		case <-ch:
		case <-time.After(time.Second * 10):
			log.Print("Warning: cleanup timed out")
		}
		unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
	})
}
