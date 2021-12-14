package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gotoz/runq/internal/cfg"
	"github.com/gotoz/runq/internal/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"

	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
)

var (
	gitCommit string        // set via Makefile
	ackChan   chan uint8    // to send acknowledge messages back to proxy
	msgChan   <-chan vm.Msg // to receives messages from proxy
	_         = os.DirFS    // force Go compiler version >= 1.16
)

func init() {
	log.SetPrefix(fmt.Sprintf("[%s(%d) %s] ", filepath.Base(os.Args[0]), os.Getpid(), gitCommit))
	log.SetFlags(0)
}

func main() {
	switch os.Args[0] {
	case "entrypoint":
		mainEntrypoint()
		return
	case "/sbin/modprobe":
		mainModprobe()
		return
	}

	signal.Ignore(unix.SIGTERM, unix.SIGUSR1, unix.SIGUSR2)

	if err := runInit(); err != nil {
		shutdown(1, fmt.Sprintf("%+v", err))
	}
	shutdown(0, "")
}

func runInit() error {
	if os.Args[0] != "/init" || len(os.Args) > 1 || os.Getpid() != 1 {
		fmt.Printf("%s (%s)\n", gitCommit, runtime.Version())
		os.Exit(0)
	}

	runtime.LockOSThread()
	if err := mountInitStage0(); err != nil {
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

	if vmdata.MachineType == "z13" {
		if err := loadKernelModules("z13", ""); err != nil {
			return err
		}
	} else {
		if err := loadKernelModules("z14+", ""); err != nil {
			return err
		}
	}

	if !vmdata.NoExec {
		if err := loadKernelModules("vsock", ""); err != nil {
			return err
		}
	}

	if !vmdata.Entrypoint.Terminal {
		if _, err := terminal.MakeRaw(0); err != nil {
			return errors.WithStack(err)
		}
	}

	// By default the 9pfs share contains the container root filesystem
	// including /lib/modules.
	// When using a rootdisk the 9pfs share contains only /lib/modules
	if vmdata.Rootdisk == "" {
		if err := mountInitShare("rootfs", "/rootfs", vmdata.Cache9p); err != nil {
			return err
		}
	} else {
		if err := setupRootdisk(vmdata); err != nil {
			return err
		}
		if err := mountInitShare("share", "/rootfs/lib/modules", vmdata.Cache9p); err != nil {
			return err
		}
	}
	if err := util.CreateSymlink("/rootfs/lib/modules", "/lib/modules"); err != nil {
		return err
	}

	// Remove empty mountpoint.
	if err := os.Remove("/rootfs/qemu"); err != nil && !os.IsNotExist(err) {
		return errors.WithStack(err)
	}

	if err := mountInitStage1(vmdata.Mounts); err != nil {
		return err
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

	if vmdata.APDevice != "" {
		if err := setupAPDevice(); err != nil {
			return err
		}
	}

	// Start reaper to wait4 zombie processes.
	go reaper()

	if err := setModprobe(); err != nil {
		return fmt.Errorf("setModprobe failed: %v", err)
	}

	// Start entrypoint process.
	entrypoint, err := newEntrypoint(vmdata.Entrypoint)
	if err != nil {
		shutdown(util.ErrorToRc(err))
	}
	pidEntrypoint := entrypoint.Process.Pid
	go wait4Entrypoint(pidEntrypoint, vmdata.Entrypoint.Systemd)

	// Start vsockd process.
	if vmdata.Vsockd.CID != 0 {
		vmdata.Vsockd.EntrypointPid = pidEntrypoint
		vmdata.Vsockd.EntrypointEnv = vmdata.Entrypoint.Env
		vsockd, err := newVsockd(vmdata.Vsockd, pidEntrypoint)
		if err != nil {
			shutdown(util.ErrorToRc(err))
		}
		if err := ioutil.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", vsockd.Process.Pid), []byte("-1000"), 0644); err != nil {
			shutdown(1, "can't adjust oom score of vsockd")
		}
		go wait4Vsockd(vsockd.Process.Pid)
	}

	// Main loop to process messages from proxy.
	for {
		msg := <-msgChan
		switch msg.Type {
		case vm.Signal:
			// Forward signal to application.
			sig := unix.Signal(int(msg.Data[0]))
			if err := signalProcess(pidEntrypoint, sig); err != nil {
				log.Printf("send signal %#v to %d: %v", sig, pidEntrypoint, err)
			}
		default:
			return errors.Errorf("received invalid message: %v", msg)
		}
	}
}

func wait4Vsockd(pid int) {
	var wstatus unix.WaitStatus
	_, err := unix.Wait4(pid, &wstatus, 0, nil)

	switch {
	case wstatus.Exited():
		return
	case wstatus.Signaled():
		if wstatus.Signal() == unix.SIGKILL {
			// regular shoutdown, don't print message
			return
		}
	}
	if err != nil {
		log.Println(err)
	}
}

// wait4Entrypoint waits for the entrypoint process to finsh and then call
// shutdown with an exit code following Docker exit codes which in fact
// follow Bash exit codes.
// Running Systemd as Docker entrypoint the exit code must be treated
// differently. Init terminates by sending SIGINT or SIGHUP to itself.
//   poweroff, halt -> SIGINT (2)-> no container restart -> exit code 0 (forced)
//   reboot         -> SIGHUP (1)-> container restart	 -> exit code 1
func wait4Entrypoint(pid int, systemd bool) {
	var wstatus unix.WaitStatus
	_, err := unix.Wait4(pid, &wstatus, 0, nil)
	switch {
	case err != nil:
		shutdown(1, fmt.Sprintf("%+v", errors.WithStack(err)))
	case wstatus.Exited():
		shutdown(uint8(wstatus.ExitStatus()), "")
	case wstatus.Signaled():
		sig := wstatus.Signal()
		if systemd {
			switch sig {
			case unix.SIGINT:
				shutdown(0, "")
			default:
				shutdown(1, "")
			}
		}
		shutdown(128+uint8(sig), "")
	}
}

func signalProcess(pid int, sig unix.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return errors.WithStack(err)
	}
	return proc.Signal(sig)
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
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return fmt.Errorf("invalid kernel config: %q", line)
		}
		if fields[0] != kind {
			continue
		}
		f, err := os.Open(filepath.Join(prefix + fields[1]))
		if err != nil {
			return err
		}
		defer f.Close()
		params := strings.Join(fields[2:], " ")
		if err := unix.FinitModule(int(f.Fd()), params, 0); err != nil {
			if !os.IsExist(err) {
				return fmt.Errorf("load kernel module %q failed: %v", fields[1], err)
			}
		}
	}
	return errors.WithStack(scanner.Err())
}

func setSysctl(vmdataSysctl map[string]string) error {
	for k, v := range cfg.SysctlDefault {
		if err := util.SetSysctl(k, v); err != nil {
			return err
		}
	}
	for k, v := range vmdataSysctl {
		if err := util.SetSysctl(k, v); err != nil {
			return err
		}
	}
	for k, v := range cfg.SysctlOverride {
		if err := util.SetSysctl(k, v); err != nil {
			return err
		}
	}
	return nil
}

func setupAPDevice() error {
	if _, err := os.Stat("/sys/bus/ap/devices"); err != nil {
		return err
	}
	files, _ := filepath.Glob("/sys/bus/ap/devices/card*")
	if len(files) == 0 {
		return fmt.Errorf("no ap device found")
	}

	if err := loadKernelModules("zcrypt", "/rootfs"); err != nil {
		return err
	}

	if err := os.Chmod("/dev/z90crypt", 0666); err != nil {
		return fmt.Errorf("can't chmod /dev/z90crypt: %v", err)
	}

	return nil
}

var onceShutdown sync.Once

func shutdown(rc uint8, msg string) {
	onceShutdown.Do(func() {
		if msg != "" {
			log.Print(msg)
		}
		// Send exit code of entrypoint to proxy.
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
			umountInit()
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

func setModprobe() error {
	path := "/sbin/modprobe"
	os.Mkdir("/sbin", 0755)
	if err := util.CreateSymlink("/init", path); err != nil {
		return err
	}
	return nil
}
