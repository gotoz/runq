package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"
)

type symlink struct {
	dev, dir, name string
}

func setupDisks(external, embedded []vm.Disk) error {
	disks := append([]vm.Disk(nil), external...)
	disks = append(disks, embedded...)

	var mounts []vm.Mount
	var links []symlink

	for _, disk := range disks {
		dev, err := findDisk(disk.Serial)
		if err != nil {
			return err
		}
		if dev == "" {
			return errors.New("disk not found: " + disk.Dir)
		}
		if disk.ID != "" {
			links = append(links, symlink{
				dev:  dev,
				dir:  "/dev/disk/by-runq-id",
				name: disk.ID,
			})
		}

		if !disk.Mount {
			continue
		}

		if err := loadKernelModules(disk.Fstype, "/rootfs"); err != nil {
			return err
		}

		flags, data, mode := util.ParseMountOptions(disk.MountOptions)
		mounts = append(mounts, vm.Mount{
			ID:     disk.ID,
			Source: "/dev/" + dev,
			Target: "/rootfs" + disk.Dir,
			Fstype: disk.Fstype,
			Data:   data,
			Flags:  flags,
			Mode:   mode,
		})
	}
	if err := mount(mounts); err != nil {
		return err
	}
	return createSymlinks(links)
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

func createSymlinks(links []symlink) error {
	wd, err := os.Getwd()
	if err != nil {
		return errors.WithStack(err)
	}
	defer os.Chdir(wd)

	for _, l := range links {
		if !util.DirExists(l.dir) {
			if err := os.MkdirAll(l.dir, 0755); err != nil {
				return err
			}
		}
		if err := os.Chdir(l.dir); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Symlink("../../"+l.dev, l.name); err != nil {
			return fmt.Errorf("can't create symlink: %v", err)
		}
	}
	return nil
}
