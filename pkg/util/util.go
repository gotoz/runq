package util

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"github.com/pkg/errors"

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
			return errors.WithStack(err)
		}
	}
	var mode uint32
	switch devtype {
	case "b":
		mode = unix.S_IFBLK | fmode
	case "c":
		mode = unix.S_IFCHR | fmode
	default:
		return errors.Errorf("type %s not supported", devtype)
	}

	umask := unix.Umask(0)
	defer unix.Umask(umask)

	dev := int(minor&0xfff00<<12 | major&0xfff<<8 | minor&0xff)
	err := unix.Mknod(path, mode, dev)
	return errors.Wrapf(err, "Mknod(%s)", path)
}

// SetSysctl sets a syscontrol.
func SetSysctl(name string, value string) error {
	path := "/proc/sys/" + strings.Replace(name, ".", "/", -1)
	err := ioutil.WriteFile(path, []byte(value), 0644)
	return errors.Wrapf(err, "WriteFile %s", path)
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
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return errors.Wrapf(err, "insmod %s", path)
	}
	defer unix.Close(fd)

	var p1 *byte
	p1, err = unix.BytePtrFromString(strings.Join(args, " "))
	if err != nil {
		return errors.WithStack(err)
	}
	_, _, e1 := unix.Syscall(unix.SYS_FINIT_MODULE, uintptr(fd), uintptr(unsafe.Pointer(p1)), 0)
	if e1 == 0 || e1 == unix.EEXIST {
		return nil
	}
	return errors.Errorf("insmod %s: %v", path, os.NewSyscallError("SYS_FINIT_MODULE", e1))
}

// MajorMinor returns major and minor device number for a given syspath.
func MajorMinor(syspath string) (int, int, error) {
	// cat /sys/class/virtio-ports/vport0p1/dev
	// 252:1
	buf, err := ioutil.ReadFile(syspath)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "ReadFile %s", syspath)
	}
	s := strings.Split(strings.TrimSpace(string(buf)), ":")
	major, err := strconv.Atoi(s[0])
	if err != nil {
		return 0, 0, errors.Wrapf(err, "Atoi %s", s[0])
	}
	minor, err := strconv.Atoi(s[1])
	return major, minor, errors.Wrapf(err, "Atoi %s", s[1])
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
		log.Printf("killing: %d (%s)", pid, string(buf))
		unix.Kill(pid, unix.SIGKILL)
		return filepath.SkipDir
	})
}
