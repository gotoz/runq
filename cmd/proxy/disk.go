package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"code.cloudfoundry.org/bytefmt"
	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"
	"golang.org/x/sys/unix"
)

// detect vm.Disktype of backing file (raw file or Qcow image)
func disktype(path string) (vm.Disktype, error) {
	qcowMagic := []byte{0x51, 0x46, 0x49, 0xfb}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("os.Stat() failed: %v", err)
	}

	if !fileInfo.Mode().IsRegular() {
		return 0, fmt.Errorf("not a regular file: %s", path)
	}

	if fileInfo.Size() < 4 {
		return vm.DisktypeRawFile, nil
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
		return vm.DisktypeQcow2Image, nil
	}
	return vm.DisktypeRawFile, nil
}

// fsType returns the filesystem type for a given mount point (target), e.g. ext4
func fsType(path string) (string, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return "", err
	}
	defer file.Close()

	var fstype string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) > 2 {
			if f[1] == path {
				fstype = f[2]
				break
			}
		}
	}
	return fstype, scanner.Err()
}

var (
	reDiskID             = regexp.MustCompile(`^[a-zA-Z0-9-_]{1,36}$`)
	reDiskCache          = regexp.MustCompile(`^(none|writethrough|writeback|unsafe)$`)
	reDiskDir            = regexp.MustCompile(`^/[a-zA-Z0-9._/-]+$`)
	reDiskImg            = regexp.MustCompile(`^/[a-zA-Z0-9._/-]+\.(img|qcow|qcow2|raw)$`)
	reDiskLabel          = regexp.MustCompile(`^[a-zA-Z0-9._-]{0,16}$`)
	reDiskFstypeEmbedded = regexp.MustCompile(`^(ext2|ext4)$`)
	reDiskFstypeExternal = regexp.MustCompile(`^(ext2|ext3|ext4|xfs|btrfs)$`)
)

func syntaxErrorRunqmount(format string, a ...interface{}) error {
	syntax := `RUNQ_MOUNT="id=<id>,options=<mount-options>;..."`
	msg := fmt.Sprintf("ERROR: syntax error: "+format, a...)
	return fmt.Errorf("%s\nSyntax: %s", msg, syntax)
}

// env RUNQ_MOUNT defines mount options for external mounts.
// mounts are identified by the id
func parseRunqMountEnv(env string) (map[string][]string, error) {
	if env == "" {
		return nil, nil
	}
	opts := make(map[string][]string)
	for _, v := range strings.Split(env, ";") {
		var o []string
		var id string
		for _, vv := range strings.Split(v, ",") {
			f := strings.SplitN(vv, "=", 2)
			if len(f) != 2 || f[1] == "" {
				return nil, syntaxErrorRunqmount(vv)
			}
			switch f[0] {
			case "id":
				if !reDiskID.MatchString(f[1]) {
					return nil, syntaxErrorRunqmount(vv)
				}
				id = f[1]
			case "options":
				o = strings.Split(f[1], "+")
			default:
				return nil, syntaxErrorRunqmount(vv)
			}
		}
		if id == "" {
			return nil, syntaxErrorRunqmount(v)
		}
		opts[id] = o
	}
	return opts, nil
}

// ensure disk IDs are unique
func validateDiskIds(disks ...[]vm.Disk) error {
	ids := make(map[string]struct{})
	for _, d := range disks {
		for _, dd := range d {
			if dd.ID == "" {
				return fmt.Errorf("empty disk ID: %#v", dd)
			}
			if _, ok := ids[dd.ID]; ok {
				return fmt.Errorf("duplicate disk ID: '%s'", dd.ID)
			}
			ids[dd.ID] = struct{}{}
		}
	}
	return nil
}

func updateExternalDisksMountOptions(disks []vm.Disk) error {
	mountOptions, err := parseRunqMountEnv(os.Getenv("RUNQ_MOUNT"))
	if err != nil {
		return err
	}
	for i, d := range disks {
		//  0   1    2     3       4        5
		// /dev/runq/<id>/<cache>[/<fstype>/<dir>]
		f := strings.SplitN(strings.TrimLeft(d.Path, "/ "), "/", 6)

		if len(f) < 4 {
			return fmt.Errorf("invalid disk: %s", d.Path)
		}
		f = append(f, make([]string, 6-len(f))...)

		// id
		if !reDiskID.MatchString(f[2]) {
			return fmt.Errorf("invalid disk ID '%s'", f[2])
		}
		d.ID = f[2]

		// cache
		if !reDiskCache.MatchString(f[3]) {
			return fmt.Errorf("invalid cache type for %s", d.Path)
		}
		d.Cache = f[3]

		// fstype
		if f[4] != "" {
			if !reDiskFstypeExternal.MatchString(f[4]) {
				return fmt.Errorf("unsupported filesystem '%s' in %s", f[4], d.Path)
			}
			d.Fstype = f[4]
		}

		// dir
		if f[5] != "" {
			d.Dir = "/" + f[5]
		}

		if d.Type == vm.DisktypeUnknown {
			dt, err := disktype(d.Path)
			if err != nil {
				return fmt.Errorf("ERROR: can't get disktype of %q: %v", d.Path, err)
			}
			d.Type = dt
		}
		if d.Fstype != "" && d.Dir != "" {
			d.Mount = true
		}
		d.Serial = util.RandStr(12)
		if opts, exists := mountOptions[d.ID]; exists {
			d.MountOptions = opts
		}
		disks[i] = d
	}
	return nil
}

func syntaxErrorRunqdisk(format string, a ...interface{}) error {
	syntax := `RUNQ_DISK="dir=<mountpoint>,[id=<id>,size=<size>,cache=<cache>,fstype=<filesystem>,img=<diskimage>,options=<mount-options>,mount=<on|off>];..."`
	msg := fmt.Sprintf("ERROR: syntax error: "+format, a...)
	return fmt.Errorf("%s\nSyntax: %s", msg, syntax)
}

// Embedded disks are specified via the RUNQ_DISK environment variable.
// The backing files are stored inside the container file system.
func embeddedDisks() ([]vm.Disk, error) {
	env := os.Getenv("RUNQ_DISK")
	if env == "" {
		return nil, nil
	}
	fstype, err := fsType("/")
	if err != nil {
		return nil, fmt.Errorf("ERROR: can't get Docker storage driver: %v", err)
	}
	if !(fstype == "btrfs" || fstype == "overlay") {
		return nil, fmt.Errorf("ERROR: Docker storage driver must be btrfs or overlay, found: %s", fstype)
	}

	var disks []vm.Disk
	for _, v := range strings.Split(env, ";") {
		log.Printf("DEBUG: parsing %q", v)
		d := vm.Disk{
			Cache:  "none",
			Fstype: "ext4",
			Mount:  true,
			Serial: util.RandStr(12),
			Size:   vm.MinEmbeddedDiskSize,
			Type:   vm.DisktypeRawFile,
		}
		for _, vv := range strings.Split(v, ",") {
			f := strings.SplitN(vv, "=", 2)
			if len(f) != 2 || f[1] == "" {
				return nil, syntaxErrorRunqdisk(vv)
			}
			switch f[0] {
			case "cache":
				if !reDiskCache.MatchString(f[1]) {
					return nil, syntaxErrorRunqdisk(vv)
				}
				d.Cache = f[1]
			case "dir":
				if !reDiskDir.MatchString(f[1]) {
					return nil, syntaxErrorRunqdisk(vv)
				}
				d.Dir = "/" + strings.Trim(f[1], "/")
			case "fstype":
				if !reDiskFstypeEmbedded.MatchString(f[1]) {
					return nil, syntaxErrorRunqdisk(vv)
				}
				d.Fstype = f[1]
			case "id":
				if !reDiskID.MatchString(f[1]) {
					return nil, syntaxErrorRunqdisk(vv)
				}
				d.ID = f[1]
			case "img":
				if !reDiskImg.MatchString(f[1]) {
					return nil, syntaxErrorRunqdisk(vv)
				}
				d.Path = f[1]
			case "label":
				if !reDiskLabel.MatchString(f[1]) {
					return nil, syntaxErrorRunqdisk(vv)
				}
				d.Label = f[1]
			case "mount":
				switch f[1] {
				case "0", "off", "no":
					d.Mount = false
				case "1", "on", "yes":
					d.Mount = true
				default:
					return nil, syntaxErrorRunqdisk(vv)
				}
			case "options":
				d.MountOptions = strings.Split(f[1], "+")
			case "size":
				n, err := bytefmt.ToBytes(f[1])
				if err != nil {
					return nil, syntaxErrorRunqdisk("%s : %v", vv, err)
				}
				if n < vm.MinEmbeddedDiskSize {
					return nil, syntaxErrorRunqdisk("size too small, min size: %s", bytefmt.ByteSize(vm.MinEmbeddedDiskSize))
				}
				d.Size = n
			default:
				return nil, syntaxErrorRunqdisk(vv)
			}
		}

		if d.Dir == "" {
			return nil, syntaxErrorRunqdisk("dir=<mountpoint> is required")
		}
		if d.Path == "" {
			d.Path = d.Dir + "/runq.img"
		}
		if d.ID == "" {
			d.ID = fmt.Sprintf("%x", md5.Sum([]byte(d.Dir)))
		}
		disks = append(disks, d)
	}
	return disks, nil
}

// prepareEmbeddedDisks creates/resizes the embedded disks inside the container filesystem.
// It must run after chroot to /qemu because tools like 'mke2fs' exist only in the
// /qemu directory
func prepareEmbeddedDisks(disks []vm.Disk) error {
	for i, d := range disks {
		uPath := d.Path
		d.Path = "/rootfs" + d.Path
		requireFsck := true

		fileInfo, err := os.Stat(d.Path)
		if os.IsNotExist(err) {
			log.Printf("DEBUG: image %q does not exist. Creating new image with %s ...", uPath, d.Fstype)
			if err := resizeImage(d.Path, d.Size); err != nil {
				return err
			}
			cmd := exec.Command("/sbin/mke2fs", "-t", d.Fstype, "-b", "4k", "-F", "-L", d.Label, d.Path)
			log.Printf("DEBUG: %v", cmd.Args)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Print(string(out))
				return fmt.Errorf("ERROR: %v failed: %v", cmd.Args, err)
			}
			fileInfo, err = os.Stat(d.Path)
			requireFsck = false
		}
		if err != nil {
			return fmt.Errorf("ERROR: %v", err)
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("ERROR: %q: is not a regular file", uPath)
		}
		d.Type, err = disktype(d.Path)
		if err != nil {
			return fmt.Errorf("ERROR: can't get disktype of %q: %v", uPath, err)
		}
		if d.Type != vm.DisktypeRawFile {
			log.Print("DEBUG: disktype!=raw, no resize/fsck")
			disks[i] = d
			continue
		}

		currSize := uint64(fileInfo.Size())
		log.Printf("DEBUG: current size:%d", currSize)

		if d.Size > currSize {
			if err := resizeImage(d.Path, d.Size); err != nil {
				return err
			}
			cmd := exec.Command("/sbin/e2fsck", "-fpv", d.Path)
			log.Printf("DEBUG: %v", cmd.Args)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Print(string(out))
				rc, msg := util.ErrorToRc(err)
				if rc > 1 {
					return fmt.Errorf("ERROR: %v failed: %v : rc=%d msg:%s", cmd.Args, err, rc, msg)
				}
			}
			cmd = exec.Command("/sbin/resize2fs", d.Path)
			log.Printf("DEBUG: %v", cmd.Args)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Print(string(out))
				return fmt.Errorf("ERROR: %v failed: %v", cmd.Args, err)
			}
			requireFsck = false
		}

		if d.Mount == false {
			log.Print("DEBUG: mount=false, no fsck")
			requireFsck = false
		}
		if requireFsck {
			cmd := exec.Command("/sbin/e2fsck", "-fpv", d.Path)
			log.Printf("DEBUG: %v", cmd.Args)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Print(string(out))
				rc, msg := util.ErrorToRc(err)
				if rc > 1 {
					return fmt.Errorf("ERROR: %v failed: %v : rc=%d msg:%s", cmd.Args, err, rc, msg)
				}
			}
		}
		disks[i] = d
	}
	return nil
}

func resizeImage(path string, size uint64) error {
	if size > math.MaxInt64 {
		return fmt.Errorf("ERROR: size %d overflows int64", size)
	}
	log.Printf("DEBUG: resize/create Image(%s, %d)", path, size)
	d := filepath.Dir(path)
	if !util.DirExists(d) {
		if err := os.MkdirAll(d, 0700); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(path, unix.O_RDWR|os.O_CREATE|unix.O_DIRECT, 0600)
	if err != nil {
		return fmt.Errorf("ERROR: OpenFile(%q) failed: %v", path, err)
	}
	defer f.Close()

	if err := unix.Fallocate(int(f.Fd()), 0, 0, int64(size)); err != nil {
		return fmt.Errorf("ERROR: fallocate(%q) failed: %v", path, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("ERROR: Fsync(%q) failed: %v", path, err)
	}
	return nil
}
