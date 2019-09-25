package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gotoz/runq/pkg/loopback"
	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"
	"golang.org/x/sys/unix"

	"github.com/pkg/errors"
)

// disktype detects the type of a disk at the given path
func disktype(path string) (vm.Disktype, error) {
	qcowMagic := []byte{0x51, 0x46, 0x49, 0xfb}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	if fileInfo.Mode()&os.ModeDevice > 0 {
		return vm.BlockDevice, nil
	}
	if !fileInfo.Mode().IsRegular() {
		return vm.DisktypeUnknown, nil
	}
	if fileInfo.Size() < 4 {
		return vm.RawFile, nil
	}

	r, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	buf := make([]byte, 4)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}
	if bytes.Equal(qcowMagic, buf) {
		return vm.Qcow2Image, nil
	}
	return vm.RawFile, nil
}

var reDiskID = regexp.MustCompile("^[a-zA-Z0-9-_]{1,36}$")

func updateDisks(disks []vm.Disk) error {
	ids := make(map[string]bool)
	for i, d := range disks {
		//  0   1    2     3       4             5
		// /dev/runq/<id>/<cache>[/<filesystem>/<mountpoint>]
		f := strings.SplitN(strings.TrimLeft(d.Path, "/ "), "/", 6)

		if len(f) < 4 {
			return fmt.Errorf("invalid disk: %s", d.Path)
		}
		f = append(f, make([]string, 6-len(f))...)

		if !reDiskID.MatchString(f[2]) {
			return fmt.Errorf("invalid disk ID '%s'", f[2])
		}
		if _, ok := ids[f[2]]; ok {
			return fmt.Errorf("duplicate disk ID: '%s'", f[2])
		}
		d.ID = f[2]
		ids[d.ID] = true

		switch f[3] {
		case "none", "writeback", "writethrough", "unsafe":
			d.Cache = f[3]
		default:
			return fmt.Errorf("invalid cache type for %s", d.Path)
		}

		switch f[4] {
		case "", "ext2", "ext3", "ext4", "xfs", "btrfs":
			d.Fstype = f[4]
		default:
			return fmt.Errorf("unsupported filesystem '%s' in %s", f[4], d.Path)
		}

		if f[5] != "" {
			d.Dir = "/" + f[5]
		}

		if d.Fstype != "" && d.Dir != "" {
			d.Mount = true
		}

		if d.Type == vm.DisktypeUnknown {
			dt, err := disktype(d.Path)
			if err != nil {
				return fmt.Errorf("%s: detect disktype failed: %v", d.Path, err)
			}
			if dt == vm.DisktypeUnknown {
				return fmt.Errorf("%s: unknown disktype", d.Path)
			}
			d.Type = dt
		}
		if d.Type == vm.DisktypeUnknown {
			return fmt.Errorf("%s: unknown disktype", d.Path)
		}

		d.Serial = util.RandStr(12)

		disks[i] = d
	}
	return nil
}

// prepareRootdisk copies the content of the container root directory into a
// bootdisk. The disk must have an empty ext2 or ext4 filesystem.
// prepareRootdisk must run after pivot_root to /qemu so that the container
// files are in /qemu/rootfs.
func prepareRootdisk(vmdata *vm.Data) error {
	var disk *vm.Disk
	for _, d := range vmdata.Disks {
		if d.ID == vmdata.Rootdisk {
			disk = &d
			break
		}
	}
	if disk == nil {
		return fmt.Errorf("rootdisk %q not found", vmdata.Rootdisk)
	}

	dtype, err := disktype(disk.Path)
	if err != nil {
		return err
	}
	if dtype != vm.BlockDevice && dtype != vm.RawFile {
		return fmt.Errorf("rootdisk %s: unsupported disktype", disk.Path)
	}

	excl := []string{"/dev", "/lib/modules", "/lost+found", "/proc", "/qemu", "/sys"}
	excl = append(excl, vmdata.RootdiskExclude...)

	if disk.Fstype != "ext2" && disk.Fstype != "ext4" {
		return fmt.Errorf("rootdisk: fstype %q is not supported, use ext2 or ext4", disk.Fstype)
	}

	cmd := exec.Command("/sbin/e2fsck", "-pv", disk.Path)
	if out, err := cmd.CombinedOutput(); err != nil {
		rc, _ := util.ErrorToRc(err)
		if rc > 0 {
			log.Println(string(out))
		}
		if rc > 1 {
			return fmt.Errorf("e2fsck failed: %v", err)
		}
	}

	src := "/rootfs"
	dest := "/dev/rootdisk"
	if err := os.Mkdir(dest, 0700); err != nil {
		return err
	}

	if dtype == vm.RawFile {
		loop, err := loopback.New()
		if err != nil {
			return err
		}

		file, err := os.OpenFile(disk.Path, os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		defer file.Close()

		if err := loop.Attach(file); err != nil {
			return err
		}
		defer loop.Detach()

		if err := unix.Mount(loop.Name, dest, disk.Fstype, 0, ""); err != nil {
			return fmt.Errorf("mount loopback failed: %v", err)
		}
	} else {
		if err := unix.Mount(disk.Path, dest, disk.Fstype, 0, ""); err != nil {
			return fmt.Errorf("mount rootdisk failed: %v", err)
		}
	}

	defer func() {
		if err := unix.Unmount(dest, unix.MNT_DETACH); err != nil {
			log.Printf("umount rootdisk failed: %v", err)
		}
		os.Remove(dest)
	}()

	diskIsEmpty, err := dirIsEmpty(dest)
	if err != nil {
		return err
	}
	if !diskIsEmpty {
		return nil
	}

	args := []string{"/usr/bin/rsync", "-aRH"}
	for _, d := range excl {
		args = append(args, "--exclude", d)
	}
	args = append(args, "./", dest)

	if err := os.Chdir(src); err != nil {
		return fmt.Errorf("chdir to %s failed: %v ", src, err)
	}
	defer os.Chdir("/")

	cmd = exec.Command(args[0], args[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		rc, msg := util.ErrorToRc(err)
		if rc > 0 {
			log.Println(string(out))
			return fmt.Errorf("rsync failed: %v rc=%d %s", err, rc, msg)
		}
	}
	if err := os.MkdirAll("/lib/modules", 0755); err != nil {
		return err
	}
	return nil
}

func dirIsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()
	dirs, err := f.Readdirnames(2)
	if err != nil {
		if err == io.EOF {
			return true, nil
		}
		return false, err
	}
	for _, d := range dirs {
		if d != "lost+found" {
			return false, nil
		}
	}
	return true, nil
}
