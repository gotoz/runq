package loopback

import (
	"fmt"
	"os"

	"github.com/gotoz/runq/internal/util"
	"golang.org/x/sys/unix"
)

// Loopback defines a loopback device
type Loopback struct {
	Name   string
	device *os.File
}

// New finds the first unused loopback device
func New() (*Loopback, error) {
	f, err := os.OpenFile("/dev/loop-control", os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	nr, _, e1 := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.LOOP_CTL_GET_FREE, 0)
	if e1 != 0 {
		return nil, fmt.Errorf("Get next free loop device file: %v", e1.Error())
	}
	name := fmt.Sprintf("/dev/loop%d", nr)

	if _, err := os.Stat(name); err != nil {
		if err := util.Mknod(name, "b", 0600, 7, int(nr)); err != nil {
			return nil, fmt.Errorf("create %q failed: %v", name, err)
		}
	}

	return &Loopback{
		Name: name,
	}, nil
}

// Attach attaches a given raw file to an loop back device
func (l *Loopback) Attach(file *os.File) error {
	f, err := os.OpenFile(l.Name, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	l.device = f
	if _, _, e1 := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.LOOP_SET_FD, file.Fd()); e1 != 0 {
		return fmt.Errorf("Attach file to %q failed: %v", l.Name, e1.Error())
	}
	return nil
}

// Detach detaches the device
func (l *Loopback) Detach() error {
	defer l.device.Close()
	if _, _, e1 := unix.Syscall(unix.SYS_IOCTL, l.device.Fd(), unix.LOOP_CLR_FD, 0); e1 != 0 {
		return fmt.Errorf("Detach file from %q failed: %v", l.Name, e1.Error())
	}
	return nil
}
