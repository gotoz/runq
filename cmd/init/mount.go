package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

func mountInit() error {
	mounts := []vm.Mount{
		{
			Source: "proc",
			Target: "/proc",
			Fstype: "proc",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
		},
		{
			Source: "dev",
			Target: "/dev",
			Fstype: "devtmpfs",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC,
			Data:   "mode=0755",
		},
		{
			Source: "sysfs",
			Target: "/sys",
			Fstype: "sysfs",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
		},
		{
			Source: "devpts",
			Target: "/dev/pts",
			Fstype: "devpts",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC,
			Data:   "newinstance,gid=5,mode=0620,ptmxmode=000",
		},
	}
	return mount(mounts)
}

func mount9pfs(extraMounts []vm.Mount) error {
	mounts := []vm.Mount{
		{
			Source: "rootfs",
			Target: "/rootfs",
			Fstype: "9p",
			Flags:  unix.MS_NODEV | unix.MS_DIRSYNC,
			Data:   "trans=virtio,cache=mmap",
		},
		{
			Source: "/rootfs/lib/modules",
			Target: "/lib/modules",
			Fstype: "",
			Flags:  unix.MS_BIND,
		},
	}
	for _, m := range extraMounts {
		m.Target = "/rootfs" + m.Target
		mounts = append(mounts, m)
	}
	return mount(mounts)
}

func mountRootfs() error {
	mounts := []vm.Mount{
		{
			Source: "proc",
			Target: "/rootfs/proc",
			Fstype: "proc",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
		},
		{
			Source: "sysfs",
			Target: "/rootfs/sys",
			Fstype: "sysfs",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
		},
		{
			Source: "dev",
			Target: "/rootfs/dev",
			Fstype: "devtmpfs",
			Flags:  unix.MS_NOSUID,
			Data:   "mode=0755",
		},
		{
			Source: "devpts",
			Target: "/rootfs/dev/pts",
			Fstype: "devpts",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC,
			Data:   "newinstance,gid=5,mode=0620,ptmxmode=000",
		},
		{
			Source: "shm",
			Target: "/rootfs/dev/shm",
			Fstype: "tmpfs",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
			Data:   "size=65536k",
		},
		{
			Source: "mqueue",
			Target: "/rootfs/dev/mqueue",
			Fstype: "mqueue",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
		},
		{
			Source: "/rootfs/lib/modules",
			Target: "/rootfs/lib/modules",
			Fstype: "",
			Flags:  unix.MS_BIND,
		},
		{
			Source: "/rootfs/lib/modules",
			Target: "/rootfs/lib/modules",
			Fstype: "",
			Flags:  unix.MS_REMOUNT | unix.MS_BIND | unix.MS_RDONLY | unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
		},
	}

	return mount(mounts)
}

func mountCgroups() error {
	if !util.FileExists("/proc/cgroups") {
		return nil
	}
	if !util.DirExists("/sys/fs/cgroup") {
		return nil
	}
	mounts := []vm.Mount{
		{
			Source: "tmpfs",
			Target: "/sys/fs/cgroup",
			Fstype: "tmpfs",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
			Data:   "mode=0755",
		},
	}

	file, err := os.Open("/proc/cgroups")
	if err != nil {
		return errors.WithStack(err)
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		field := strings.Fields(sc.Text())
		if len(field) < 4 {
			continue
		}
		if field[3] != "1" {
			continue
		}
		mounts = append(mounts, vm.Mount{
			Source: "cgroup",
			Target: "/sys/fs/cgroup/" + field[0],
			Fstype: "cgroup",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
			Data:   field[0],
		})
	}

	if err := sc.Err(); err != nil {
		return errors.WithStack(err)
	}
	return mount(mounts)
}

func mount(mounts []vm.Mount) error {
	for _, m := range mounts {
		if util.FileExists(m.Target) {
			return errors.Errorf("invalid path: %v", m.Target)
		}
		if !util.DirExists(m.Target) {
			if err := os.MkdirAll(m.Target, 0755); err != nil {
				return err
			}
		}
		if err := unix.Mount(m.Source, m.Target, m.Fstype, uintptr(m.Flags), m.Data); err != nil {
			return fmt.Errorf("Mount failed: src:%s dst:%s fs:%s id:%s reason: %v", m.Source, m.Target, m.Fstype, m.ID, err)
		}
	}
	return nil
}

func bindMountFile(src, target string) error {
	dir := filepath.Dir(target)
	if !util.DirExists(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Errorf("os.MkdirAll failed: %v", err)
		}
	}
	if !util.FileExists(target) {
		if _, err := os.Create(target); err != nil {
			return errors.Errorf("os.Create failed: %v", err)
		}
	}
	if err := unix.Mount(src, target, "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("Mount failed: src:%s dst:%s reason: %v", src, target, err)
	}
	return nil
}

// readonlyPath will make paths read only.
func readonlyPath(paths []string) error {
	for _, p := range paths {
		if err := unix.Mount(p, p, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := unix.Mount(p, p, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_REC, ""); err != nil {
			return err
		}
	}
	return nil
}

// For files, maskPath bind mounts /dev/null over the top of the specified path.
// For directories, maskPath mounts read-only tmpfs over the top of the specified path.
func maskPath(paths []string) error {
	for _, p := range paths {
		stat, err := os.Stat(p)
		if err != nil && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if stat.Mode().IsDir() {
			if err := unix.Mount("tmpfs", p, "tmpfs", unix.MS_RDONLY, ""); err != nil {
				return err
			}
		} else {
			if err := unix.Mount("/dev/null", p, "", unix.MS_BIND, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// Unmount real filesystems in revers order of /proc/mounts.
// Ignore almost all errors but retry to catch nested mounts.
func umountRootfs() {
	for i := 0; i < 10; i++ {
		fd, err := os.Open("/proc/mounts")
		if err != nil {
			break
		}

		sc := bufio.NewScanner(fd)

		var dirs []string
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) > 2 {
				if f[1] == "/" {
					continue
				}
				switch f[2] {
				case "ext2", "ext3", "ext4", "xfs", "btrfs":
					dirs = append([]string{f[1]}, dirs...)
				}
			}
		}

		if err := sc.Err(); err != nil {
			log.Print(err)
		}
		if len(dirs) == 0 {
			fd.Close()
			break
		}
		for _, d := range dirs {
			unix.Unmount(d, unix.MNT_DETACH)
		}
	}
}
