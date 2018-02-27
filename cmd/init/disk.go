package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"
)

func setupDisks(disks []vm.Disk) error {
	var mounts []vm.Mount

	for _, disk := range disks {
		name, err := findDisk(disk.Serial)
		if err != nil {
			return err
		}
		if name == "" {
			return errors.New("disk not found: " + disk.Dir)
		}
		dev := "/dev/" + name

		if disk.Fstype == "" {
			continue
		}

		if err := loadKernelModules(disk.Fstype); err != nil {
			return err
		}

		mounts = append(mounts, vm.Mount{
			Source: dev,
			Target: "/rootfs" + disk.Dir,
			Fstype: disk.Fstype,
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
		})
	}
	return mount(mounts)
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
