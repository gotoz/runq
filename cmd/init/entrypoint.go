package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/gotoz/runq/internal/cfg"
	"github.com/gotoz/runq/pkg/vm"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func mainEntrypoint() {
	if err := runEntrypoint(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func runEntrypoint() error {
	runtime.LockOSThread()

	rd := os.NewFile(uintptr(3), "rd")
	if rd == nil {
		return errors.New("rd == nil")
	}
	buf, err := ioutil.ReadAll(rd)
	if err != nil {
		return errors.WithStack(err)
	}
	if err := rd.Close(); err != nil {
		return errors.WithStack(err)
	}

	entrypoint, err := vm.DecodeEntrypointGob(buf)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := mountEntrypointStage0(); err != nil {
		return err
	}

	if entrypoint.DockerInit != "" {
		if err := bindMountFile("/sbin/docker-init", "/rootfs"+entrypoint.DockerInit); err != nil {
			return err
		}
	}

	if err := chroot("/rootfs"); err != nil {
		return err
	}

	if !entrypoint.Systemd {
		if err := mountEntrypointCgroups(); err != nil {
			return err
		}
	}

	if err := maskPath(cfg.MaskedPaths); err != nil {
		return err
	}

	if err := readonlyPath(cfg.ReadonlyPaths); err != nil {
		return err
	}

	if err := prepareDeviceFiles(int(entrypoint.UID)); err != nil {
		return err
	}

	if err := setRlimits(entrypoint.Rlimits); err != nil {
		return err
	}

	if entrypoint.NoNewPrivileges {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return errors.WithStack(err)
		}
	}

	if err := dropCapabilities(entrypoint.Capabilities); err != nil {
		return err
	}

	if err := initSeccomp(entrypoint.SeccompGob); err != nil {
		return err
	}

	if err := setPATH(entrypoint.Env); err != nil {
		return err
	}

	if err := setIDs(entrypoint.UID, entrypoint.GID, entrypoint.AdditionalGids); err != nil {
		return err
	}

	if err = os.Chdir(entrypoint.Cwd); err != nil {
		return err
	}

	path, err := exec.LookPath(os.Args[1])
	if err != nil {
		fmt.Println(err)
		if e, ok := err.(*exec.Error); ok && e.Err == os.ErrPermission {
			os.Exit(126)
		}
		os.Exit(127)
	}

	if !entrypoint.Terminal {
		os.Stdin = nil
	}

	if err := unix.Exec(path, os.Args[1:], entrypoint.Env); err != nil {
		return errors.Wrap(err, "Exec() failed")
	}
	return nil
}

func chroot(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return errors.WithStack(err)
	}
	if err := unix.Mount(dir, "/", "", unix.MS_MOVE, ""); err != nil {
		return errors.WithStack(err)
	}
	if err := unix.Chroot("."); err != nil {
		return errors.WithStack(err)
	}
	return nil
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
func setRlimits(limits map[string]syscall.Rlimit) error {
	merged := make(map[string]syscall.Rlimit)
	for k, v := range cfg.Rlimits {
		merged[k] = v
	}
	for k, v := range limits {
		merged[k] = v
	}
	for k, v := range merged {
		t, ok := cfg.RlimitsMap[k]
		if !ok {
			return fmt.Errorf("invalid rlimit type: %s", k)
		}
		if err := syscall.Setrlimit(t, &syscall.Rlimit{Cur: v.Cur, Max: v.Max}); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
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

func setIDs(uid, gid uint32, gids []uint32) error {
	if len(gids) > 0 {
		var g []int
		for _, v := range gids {
			g = append(g, int(v))
		}
		if err := unix.Setgroups(g); err != nil {
			return fmt.Errorf("setgroups failed: %v", err)
		}
	}
	_, _, errno := unix.RawSyscall(unix.SYS_SETGID, uintptr(gid), 0, 0)
	if errno != 0 {
		return fmt.Errorf("setgid failed: %v", os.NewSyscallError("SYS_SETGID", errno))
	}
	_, _, errno = unix.RawSyscall(unix.SYS_SETUID, uintptr(uid), 0, 0)
	if errno != 0 {
		return fmt.Errorf("setuid failed: %v", os.NewSyscallError("SYS_SETUID", errno))
	}
	return nil
}
