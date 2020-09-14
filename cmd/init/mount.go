package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotoz/runq/internal/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

func mountInitStage0() error {
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
	return mount(mounts...)
}

func mountInitShare(source, target string, readonly bool) error {
	mnt := vm.Mount{
		Source: source,
		Target: target,
		Fstype: "9p",
		Flags:  unix.MS_NODEV | unix.MS_DIRSYNC,
		Data:   "trans=virtio,cache=mmap",
	}
	if readonly {
		mnt.Flags |= unix.MS_RDONLY
	}
	return mount(mnt)
}

func mountInitStage1(extraMounts []vm.Mount) error {
	for i := range extraMounts {
		extraMounts[i].Target = "/rootfs" + extraMounts[i].Target
	}
	return mount(extraMounts...)
}

func mountEntrypointStage0() error {
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
	}
	return mount(mounts...)
}

func mountEntrypointCgroups() error {
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

	// #subsys_name	hierarchy	num_cgroups	enabled
	// cpuset	11	1	1
	// cpu	3	44	1
	// cpuacct 	3	44	1
	var links []symlink
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		field := strings.Fields(sc.Text())
		if len(field) < 4 {
			continue
		}
		if field[3] != "1" {
			continue
		}
		var data, target string
		switch field[0] {
		case "cpu":
			continue
		case "cpuacct":
			data = "cpu,cpuacct"
			target = "/sys/fs/cgroup/cpu,cpuacct"
			links = append(links, symlink{target, "cpu"})
			links = append(links, symlink{target, "cpuacct"})
		case "net_cls":
			continue
		case "net_prio":
			data = "net_cls,net_prio"
			target = "/sys/fs/cgroup/net_cls,net_prio"
			links = append(links, symlink{target, "net_cls"})
			links = append(links, symlink{target, "net_prio"})
		default:
			data = field[0]
			target = "/sys/fs/cgroup/" + field[0]
		}
		mounts = append(mounts, vm.Mount{
			Source: "cgroup",
			Target: target,
			Fstype: "cgroup",
			Flags:  unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
			Data:   data,
		})
	}

	if err := sc.Err(); err != nil {
		return errors.WithStack(err)
	}
	if err := mount(mounts...); err != nil {
		return err
	}
	return createSymlink(links...)
}

func mount(mounts ...vm.Mount) error {
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

func bindMountDir(src, target string) error {
	if !util.DirExists(target) {
		if err := os.MkdirAll(target, 0755); err != nil {
			return errors.Errorf("os.MkdirAll failed: %v", err)
		}
	}
	if err := unix.Mount(src, target, "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("bindMountDir failed: src:%s dst:%s reason: %v", src, target, err)
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
		return fmt.Errorf("bindMountFile failed: src:%s dst:%s reason: %v", src, target, err)
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
func umountInit() {
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

type symlink struct {
	target, newPath string
}

func createSymlink(links ...symlink) error {
	for _, l := range links {
		if err := util.CreateSymlink(l.target, l.newPath); err != nil {
			return fmt.Errorf("createSymlink %q -> %q failed: %v", l.target, l.newPath, err)
		}
	}
	return nil
}
