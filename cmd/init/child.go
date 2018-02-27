package main

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/gotoz/runq/pkg/vm"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func mainChild() {
	if err := runChild(); err != nil {
		log.Fatalf("%+v", err)
	}
	os.Exit(0)
}

func runChild() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	rd := os.NewFile(uintptr(3), "rd")
	if rd == nil {
		return errors.New("rd == nil")
	}
	wr := os.NewFile(uintptr(4), "wr")
	if wr == nil {
		return errors.New("wr == nil")
	}
	defer func() {
		if err := wr.Close(); err != nil {
			fmt.Print(err)
		}
		if err := rd.Close(); err != nil {
			fmt.Print(err)
		}
	}()

	unix.CloseOnExec(3)
	unix.CloseOnExec(4)

	buf, err := ioutil.ReadAll(rd)
	if err != nil {
		return errors.WithStack(err)
	}

	process, err := vm.DecodeProcessGob(buf)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := setRlimits(process.Rlimits); err != nil {
		return err
	}

	if err := setSyscallLimits(process.NoNewPrivileges, process.Capabilities, process.SeccompGob); err != nil {
		return err
	}

	if err := setPATH(process.Env); err != nil {
		return err
	}

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = process.Env
	cmd.Dir = process.Cwd
	if process.Terminal {
		cmd.Stdin = os.Stdin
	}
	// Define process attributes only if needed.
	// This allows dropping of all caps: --cap-drop ALL --user 0:0
	// Setting Credentials requires CAP_SETGID
	if process.UID != 0 || process.GID != 0 || len(process.AdditionalGids) > 0 || process.Terminal {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setctty: process.Terminal,
			Setsid:  true,
			Credential: &syscall.Credential{
				Uid:    process.UID,
				Gid:    process.GID,
				Groups: process.AdditionalGids,
			},
		}
	}

	if err := cmd.Start(); err != nil {
		rc, msg := errorToRc(err)
		log.Print(msg)
		os.Exit(int(rc))
	}

	pidBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(pidBuf, uint32(cmd.Process.Pid))
	_, err = wr.Write(pidBuf)
	return errors.WithStack(err)
}

func setRlimits(limits map[string]vm.Rlimit) error {
	merged := make(map[string]vm.Rlimit)
	for k, v := range vm.Rlimits {
		merged[k] = v
	}
	for k, v := range limits {
		merged[k] = v
	}
	for k, v := range merged {
		t, ok := vm.RlimitsMap[k]
		if !ok {
			return fmt.Errorf("invalid rlimit type: %s", k)
		}
		if err := unix.Setrlimit(t, &unix.Rlimit{Cur: v.Soft, Max: v.Hard}); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func setSyscallLimits(noNewPrivileges bool, caps vm.AppCapabilities, sec []byte) error {
	if noNewPrivileges {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return errors.WithStack(err)
		}
	}
	// seccomp(2):
	// In order to use the SECCOMP_SET_MODE_FILTER operation, either the caller must
	// have the CAP_SYS_ADMIN capability in its user namespace, or the thread must
	// already have the no_new_privs bit set.
	//
	// With noNewPrivileges = true or with CAP_SYS_ADMIN capability we can apply the
	// seccomp filter as late as posible after droping capabilities.
	// Otherwise we must apply seccomp filter befor dropping capabilities.
	if noNewPrivileges || hasSysadminCap(caps) {
		if err := dropCapabilities(caps); err != nil {
			return err
		}
		if err := initSeccomp(sec); err != nil {
			return err
		}
	} else {
		if err := initSeccomp(sec); err != nil {
			return err
		}
		if err := dropCapabilities(caps); err != nil {
			return err
		}
	}
	return nil
}

func hasSysadminCap(caps vm.AppCapabilities) bool {
	for _, v := range caps.Effective {
		if v == "CAP_SYS_ADMIN" {
			return true
		}
	}
	return false
}

func setPATH(env []string) error {
	for _, v := range env {
		s := strings.SplitN(v, "=", 2)
		if len(s) > 1 {
			if s[0] == "PATH" {
				if err := os.Setenv("PATH", s[1]); err != nil {
					return errors.WithStack(err)
				}
				break
			}
		}
	}
	return nil
}
