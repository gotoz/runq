package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotoz/runq/internal/util"
	"github.com/gotoz/runq/pkg/vm"
	"golang.org/x/sys/unix"

	"github.com/pkg/errors"
)

func setupDisks(disks []vm.Disk) error {
	for _, disk := range disks {
		dev, err := findDisk(disk.Serial)
		if err != nil {
			return err
		}
		if dev == "" {
			return errors.New("disk not found: " + disk.Dir)
		}
		if err := createDiskSymlink(dev, disk.ID); err != nil {
			return err
		}

		if !disk.Mount {
			continue
		}

		if err := loadKernelModules(disk.Fstype, "/rootfs"); err != nil {
			return err
		}

		mnt := vm.Mount{
			ID:     disk.ID,
			Source: "/dev/" + dev,
			Target: "/rootfs" + disk.Dir,
			Fstype: disk.Fstype,
			Flags:  unix.MS_NOSUID | unix.MS_NODEV,
		}
		if err := mount(mnt); err != nil {
			return err
		}
	}
	return nil
}

func setupRootdisk(vmdata *vm.Data) error {
	var disk vm.Disk
	for i, d := range vmdata.Disks {
		if d.ID == vmdata.Rootdisk {
			disk = d
			// remove rootdisk from disks
			vmdata.Disks = append(vmdata.Disks[:i], vmdata.Disks[i+1:]...)
			break
		}
	}

	dev, err := findDisk(disk.Serial)
	if err != nil {
		return err
	}
	if dev == "" {
		return errors.New("rootdisk not found")
	}

	if err := createDiskSymlink(dev, disk.ID); err != nil {
		return err
	}

	mnt := vm.Mount{
		ID:     disk.ID,
		Source: "/dev/" + dev,
		Target: "/rootfs",
		Fstype: disk.Fstype,
		Flags:  0,
	}
	if err := mount(mnt); err != nil {
		return err
	}
	return nil
}

// findDisk searches for a block device in sysfs for a given serial number.
func findDisk(serial string) (string, error) {
	files, err := os.ReadDir("/sys/block")
	if err != nil {
		return "", errors.WithStack(err)
	}

	for _, f := range files {
		fi, _ := f.Info()
		if fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if !strings.HasPrefix(f.Name(), "vd") {
			continue
		}

		path := filepath.Join("/sys/block", f.Name(), "serial")
		buf, err := os.ReadFile(path)
		if err != nil {
			return "", errors.WithStack(err)
		}

		if serial == strings.TrimSpace(string(buf)) {
			return f.Name(), nil
		}
	}
	return "", nil
}

func createDiskSymlink(dev, name string) error {
	wd, err := os.Getwd()
	if err != nil {
		return errors.WithStack(err)
	}
	defer os.Chdir(wd)

	const dir = "/dev/disk/by-runq-id"
	if !util.DirExists(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if err := os.Chdir(dir); err != nil {
		return errors.WithStack(err)
	}
	if err := os.Symlink("../../"+dev, name); err != nil {
		return fmt.Errorf("can't create symlink: %v", err)
	}
	return nil
}
