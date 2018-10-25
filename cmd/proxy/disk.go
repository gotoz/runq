package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"
)

func disktype(path string) (vm.Disktype, error) {
	qcowMagic := []byte{0x51, 0x46, 0x49, 0xfb}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	if !fileInfo.Mode().IsRegular() {
		return 0, fmt.Errorf("not a regular file: %s", path)
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
		if strings.HasPrefix(d.Path, "/dev/disk/") {
			if err := updateDiskOldSyntax(&d); err != nil {
				return err
			}
			log.Printf("Warning: deprecated syntax: '<source>:%s': want: \"<source>:/dev/runq/<id>/<cache>[/<filesystem>/<mountpoint>]\"", d.Path)
			disks[i] = d
			continue
		}
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
				return fmt.Errorf("can't get disktype of %q: %v", d.Path, err)
			}
			d.Type = dt
		}

		d.Serial = util.RandStr(12)

		disks[i] = d
	}
	return nil
}

func updateDiskOldSyntax(d *vm.Disk) error {
	//  0   1     2       3           4
	// /dev/disk/<cache>/<filesystem>/<mountpoint>
	f := strings.SplitN(strings.Trim(d.Path, "/ "), "/", 5)
	if len(f) < 5 {
		return fmt.Errorf("invalid disk: %s", d.Path)
	}

	switch f[2] {
	case "none", "writeback", "writethrough", "unsafe":
		d.Cache = f[2]
	default:
		return fmt.Errorf("invalid cache type for %s", d.Path)
	}

	switch f[3] {
	case "ext2", "ext3", "ext4", "xfs", "btrfs":
		d.Fstype = f[3]
		d.Dir = f[4]
		d.Mount = true
	case "none":
		d.Fstype = ""
	default:
		return fmt.Errorf("unsupported filesystem '%s' in %s", f[3], d.Path)
	}

	d.Dir = "/" + f[4]

	if d.Type == vm.DisktypeUnknown {
		dt, err := disktype(d.Path)
		if err != nil {
			return fmt.Errorf("can't get disktype of %q: %v", d.Path, err)
		}
		d.Type = dt
	}
	d.Serial = util.RandStr(12)
	d.ID = util.RandStr(8)
	return nil
}
