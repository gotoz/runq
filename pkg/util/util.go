package util

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

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
	if err := ioutil.WriteFile(path, []byte(value), 0644); err != nil {
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

// Insmod loads a kernel module.
func Insmod(path string, args []string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("insmod %s failed: %v", path, err)
	}
	defer f.Close()

	a2 := []byte(strings.Join(args, " ") + "\x00")
	_, _, errno := unix.Syscall(unix.SYS_FINIT_MODULE, uintptr(f.Fd()), uintptr(unsafe.Pointer(&a2[0])), 0)
	if errno == 0 || errno == unix.EEXIST {
		return nil
	}
	return fmt.Errorf("insmod %s failed: %v", path, os.NewSyscallError("SYS_FINIT_MODULE", errno))
}

// MajorMinor returns major and minor device number for a given dev file.
func MajorMinor(path string) (int, int, error) {
	// cat /sys/class/virtio-ports/vport0p1/dev
	// 252:1
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, 0, fmt.Errorf("ReadFile %s failed: %v", path, err)
	}

	var major, minor int
	if n, err := fmt.Sscanf(string(buf), "%d:%d", &major, &minor); err != nil || n != 2 {
		return 0, 0, fmt.Errorf("Sscanf %s failed: %v", path, err)
	}
	return major, minor, nil
}

// UserHome returns the home directory for a given userid and the
// location of passwd. It returns "/" if user cannot be found and on
// any error.
func UserHome(uid int) string {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return "/"
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	// 0    1        2   3   4     5         6
	// name:password:UID:GID:GECOS:directory:shell
	for sc.Scan() {
		field := strings.Split(sc.Text(), ":")
		if len(field) < 7 {
			continue
		}
		uidInt, err := strconv.Atoi(field[2])
		if err != nil {
			continue
		}
		if uidInt == uid && field[5] != "" {
			return field[5]
		}
	}
	return "/"
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
		buf, _ := ioutil.ReadFile(fmt.Sprintf("/proc/%s/cmdline", f.Name()))
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

// CreateSymlink creates a symbolic link. An existing target will be removed.
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
	os.Remove(newPath)
	if err := os.Symlink(target, newPath); err != nil {
		return fmt.Errorf("create symlink %q -> %q failed: %v", newPath, target, err)
	}
	return nil
}
