package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/gotoz/runq/internal/cfg"
	"github.com/gotoz/runq/pkg/vm"
	"golang.org/x/sys/unix"
)

func mainEntrypoint() {
	if err := runEntrypoint(); err != nil {
		log.Fatalf("runEntrypoint failed: %v", err)
	}
}

func runEntrypoint() error {
	runtime.LockOSThread()

	rd := os.NewFile(uintptr(3), "rd")
	if rd == nil {
		return fmt.Errorf("FH 3 == nil")
	}
	buf, err := io.ReadAll(rd)
	if err != nil {
		return fmt.Errorf("io.ReadAll failed: %w", err)
	}
	if err := rd.Close(); err != nil {
		return err
	}

	entrypoint, err := vm.DecodeEntrypointGob(buf)
	if err != nil {
		return fmt.Errorf("vm.DecodeEntrypointGob failed: %w", err)
	}

	if err := mountEntrypointStage0(); err != nil {
		return fmt.Errorf("mountEntrypointStage0 failed: %w", err)
	}

	if entrypoint.DockerInit != "" {
		if err := bindMountFile("/sbin/docker-init", "/rootfs"+entrypoint.DockerInit); err != nil {
			return fmt.Errorf("bindMountFile failed: %w", err)
		}
	}

	if err := chroot("/rootfs"); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	if entrypoint.Runqenv {
		if err := writeEnvfile(cfg.Envfile, entrypoint.Env); err != nil {
			return fmt.Errorf("writeEnvfile %s failed: %w", cfg.Envfile, err)
		}
		if err := os.Chmod(cfg.Envfile, 0400); err != nil {
			return fmt.Errorf("chmod %s failed: %v", cfg.Envfile, err)
		}
		if err := os.Chown(cfg.Envfile, int(entrypoint.UID), int(entrypoint.GID)); err != nil {
			return fmt.Errorf("chown %s failed: %v", cfg.Envfile, err)
		}
	}

	if !entrypoint.Systemd {
		if err := mountEntrypointCgroups(); err != nil {
			return fmt.Errorf("mountEntrypointCgroups failed: %w", err)
		}
	}

	if err := maskPath(cfg.MaskedPaths); err != nil {
		return fmt.Errorf("maskPath failed: %w", err)
	}

	if err := readonlyPath(cfg.ReadonlyPaths); err != nil {
		return fmt.Errorf("readonlyPath failed: %w", err)
	}

	if err := prepareDeviceFiles(int(entrypoint.UID)); err != nil {
		return fmt.Errorf("prepareDeviceFiles failed: %w", err)
	}

	if err := setRlimits(entrypoint.Rlimits); err != nil {
		return fmt.Errorf("setRlimits failed: %w", err)
	}

	if entrypoint.NoNewPrivileges {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return fmt.Errorf("unix.Prctl() failed: %w", err)
		}
	}

	if err = os.Chdir(entrypoint.Cwd); err != nil {
		return fmt.Errorf("os.Chdir failed: %w", err)
	}

	if err := dropCapabilities(entrypoint.Capabilities); err != nil {
		return fmt.Errorf("dropCapabilities failed: %w", err)
	}

	if err := initSeccomp(entrypoint.SeccompGob); err != nil {
		return fmt.Errorf("initSeccomp failed: %w", err)
	}

	if err := setPATH(entrypoint.Env); err != nil {
		return fmt.Errorf("setPATH failed: %w", err)
	}

	if err := setIDs(entrypoint.UID, entrypoint.GID, entrypoint.AdditionalGids); err != nil {
		return fmt.Errorf("setIDs failed: %w", err)
	}

	path, err := exec.LookPath(os.Args[1])
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		if errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EISDIR) {
			os.Exit(126)
		}
		os.Exit(127)
	}

	if !entrypoint.Terminal {
		os.Stdin = nil
	}

	if err := unix.Exec(path, os.Args[1:], entrypoint.Env); err != nil {
		return fmt.Errorf("unix.Exec failed: %w", err)
	}
	return nil
}

func chroot(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("os.Chdir failed dir=%s: %w", dir, err)
	}
	if err := unix.Mount(dir, "/", "", unix.MS_MOVE, ""); err != nil {
		return fmt.Errorf("unix.Mount (move) failed: src:%s target:/ : %w", dir, err)
	}
	if err := unix.Chroot("."); err != nil {
		return fmt.Errorf("unix.Chroot failed: %w", err)
	}
	return nil
}

func prepareDeviceFiles(uid int) error {
	if err := os.Chown("/dev/console", uid, 5); err != nil {
		return fmt.Errorf("os.Chown /dev/console failed: %w", err)
	}
	if err := os.Chmod("/dev/console", 0620); err != nil {
		return fmt.Errorf("os.Chmod /dev/console failed: %w", err)
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
			return fmt.Errorf("os.Symlink failed: src:%s target:%s : %w", k, v, err)
		}
	}

	vports, err := filepath.Glob("/dev/vport*")
	if err != nil {
		return fmt.Errorf("filepath.Glob(/dev/vport*) failed : %w", err)
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
			return fmt.Errorf("setrlimit failed: %w", err)
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
					return fmt.Errorf("setenv PATH failed: %w", err)
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
	if err := syscall.Setgid(int(gid)); err != nil {
		return fmt.Errorf("setgid failed: %v", err)
	}
	if err := syscall.Setuid(int(uid)); err != nil {
		return fmt.Errorf("setuid failed: %v", err)
	}
	return nil
}

func writeEnvfile(path string, env []string) error {
	var buf bytes.Buffer
	for _, v := range env {
		f := strings.SplitN(v, "=", 2)
		if len(f) < 2 {
			continue
		}
		if f[1] == "" {
			v = fmt.Sprintf("%s=%q", f[0], "")
		} else {
			r := []rune(f[1])
			first := r[0]
			last := r[len(r)-1]
			if (first != '\'' && first != '"') || (last != '\'' && last != '"') {
				v = fmt.Sprintf("%s=%q", f[0], f[1])
			}
		}
		if _, err := buf.WriteString(v + "\n"); err != nil {
			return err
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o444); err != nil {
		return err
	}
	return nil
}
