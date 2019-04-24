package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"
)

func setupDisks(disks []vm.Disk) error {
	var mounts []vm.Mount

	for _, disk := range disks {
		dev, err := findDisk(disk.Serial)
		if err != nil {
			return err
		}
		if dev == "" {
			return errors.New("disk not found: " + disk.Dir)
		}
		if disk.ID != "" {
			if err := createDiskSymlink(dev, disk.ID); err != nil {
				return err
			}
		}

		if !disk.Mount {
			continue
		}

		if err := loadKernelModules(disk.Fstype, "/rootfs"); err != nil {
			return err
		}

		mounts = append(mounts, vm.Mount{
			ID:     disk.ID,
			Source: "/dev/" + dev,
			Target: "/rootfs" + disk.Dir,
			Fstype: disk.Fstype,
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
		})
	}
	if err := mount(mounts); err != nil {
		return err
	}
	return nil
}

// findDisk searches for a block device in sysfs for a given serial number.
func findDisk(serial string) (string, error) {
	files, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		return "", errors.WithStack(err)
	}

	for _, f := range files {
		if f.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if !strings.HasPrefix(f.Name(), "vd") {
			continue
		}

		path := filepath.Join("/sys/block", f.Name(), "serial")
		buf, err := ioutil.ReadFile(path)
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
