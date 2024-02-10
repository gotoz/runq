package util

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// RandStr returns a random string [0-9a-f]{n}.
func RandStr(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Panicf("rand.Read: %v", err)
	}
	return fmt.Sprintf("%x", b)[:n]
}

// Mknod creates character or block device files.
func Mknod(path, devtype string, fmode uint32, major, minor int) error {
	dir := filepath.Dir(path)
	if !DirExists(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Mkdir %s failed: %v", path, err)
		}
	}
	var mode uint32
	switch devtype {
	case "b":
		mode = unix.S_IFBLK | fmode
	case "c":
		mode = unix.S_IFCHR | fmode
	default:
		return fmt.Errorf("type %s not supported", devtype)
	}

	umask := unix.Umask(0)
	defer unix.Umask(umask)

	dev := int(minor&0xfff00<<12 | major&0xfff<<8 | minor&0xff)
	if err := unix.Mknod(path, mode, dev); err != nil {
		return fmt.Errorf("Mknod %s failed: %v", path, err)
	}
	return nil
}

// SetSysctl sets a syscontrol.
func SetSysctl(name string, value string) error {
	path := "/proc/sys/" + strings.Replace(name, ".", "/", -1)
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		return fmt.Errorf("WriteFile %s failed: %v", path, err)
	}
	return nil
}

// FileExists returns whether the given regular file exists.
func FileExists(path string) bool {
	stat, err := os.Stat(path)
	if err == nil && stat.Mode().IsRegular() {
		return true
	}
	return false
}

// DirExists returns whether the given directory exists.
func DirExists(path string) bool {
	stat, err := os.Stat(path)
	if err == nil && stat.Mode().IsDir() {
		return true
	}
	return false
}

// MajorMinor returns major and minor device number for a given dev file.
func MajorMinor(path string) (int, int, error) {
	// cat /sys/class/virtio-ports/vport0p1/dev
	// 252:1
	buf, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, fmt.Errorf("ReadFile %s failed: %v", path, err)
	}

	var major, minor int
	if n, err := fmt.Sscanf(string(buf), "%d:%d", &major, &minor); err != nil || n != 2 {
		return 0, 0, fmt.Errorf("Sscanf %s failed: %v", path, err)
	}
	return major, minor, nil
}

// UserHome returns the home directory for uid
// or "/" if a home directory can't be found.
func UserHome(uid int) string {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil || u.HomeDir == "" {
		return "/"
	}
	return u.HomeDir
}

// Killall sends SIGKILL to all processes except init.
func Killall() {
	filepath.Walk("/proc", func(path string, f os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return filepath.SkipDir
		}
		if path == "/proc" {
			return nil
		}
		pid, err := strconv.Atoi(f.Name())
		if err != nil {
			return filepath.SkipDir
		}
		if pid == 1 {
			return filepath.SkipDir
		}
		// ignore kernel threads
		buf, _ := os.ReadFile(fmt.Sprintf("/proc/%s/cmdline", f.Name()))
		if len(buf) == 0 {
			return filepath.SkipDir
		}
		unix.Kill(pid, unix.SIGKILL)
		return filepath.SkipDir
	})
}

// ErrorToRc turns an error value into a Bash like exit code and an error message.
func ErrorToRc(err error) (uint8, string) {
	if err == nil {
		return 0, ""
	}

	var rc uint8 = 1
	switch err := err.(type) {
	case *exec.ExitError:
		if waitStatus, ok := err.Sys().(syscall.WaitStatus); ok {
			switch {
			case waitStatus.Exited():
				rc = uint8(waitStatus.ExitStatus())
			case waitStatus.Signaled():
				// bash like: signal number + 128
				rc = uint8(waitStatus.Signal()) + 128
			}
		}
	case *os.PathError:
		switch err.Err {
		case syscall.EACCES, syscall.ENOEXEC, syscall.EISDIR, syscall.EPERM:
			rc = 126
		case syscall.ENOENT:
			rc = 127
		}
	case *exec.Error:
		rc = 127
	}

	return rc, fmt.Sprintf("%+v", err)
}

// CreateSymlink creates a symbolic link. An existing newPath will be removed.
// target path must be absolute.
func CreateSymlink(target, newPath string) error {
	newPath = filepath.Clean(newPath)
	if !filepath.IsAbs(newPath) {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(filepath.Dir(target)); err != nil {
			return err
		}
		defer os.Chdir(wd)
		target = filepath.Base(target)
	}
	if err := os.RemoveAll(newPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("create symlink %q -> %q failed: %v", newPath, target, err)
		}
	}

	if err := os.Symlink(target, newPath); err != nil {
		return fmt.Errorf("create symlink %q -> %q failed: %v", newPath, target, err)
	}
	return nil
}

// ToBool returns true for strings that represent a true value.
func ToBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "on", "yes", "true":
		return true
	}
	return false
}

// MachineType returns the s390x machine type
// z13 : 2965
// z14 : 3906
// z15 : 8561 (T01) or 8562 (T02)
func MachineType() (string, error) {
	if runtime.GOARCH != "s390x" {
		return "", nil
	}
	f, err := os.Open("/proc/sysinfo")
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var typ string
	for scanner.Scan() {
		field := strings.Fields(scanner.Text())
		if len(field) < 2 {
			continue
		}
		if field[0] == "Type:" {
			typ = field[1]
			break
		}
	}
	if scanner.Err() != nil {
		return "", fmt.Errorf("can't read maschine type: %v", err)
	}
	if len(typ) != 4 {
		return "", fmt.Errorf("invalid machine type %q", typ)
	}

	var res string
	switch typ[:1] {
	case "2":
		res = "z13"
	case "3":
		res = "z14"
	case "8":
		res = "z15"
	default:
		return "", fmt.Errorf("unknown machine type %q", typ)
	}

	return res, nil
}
