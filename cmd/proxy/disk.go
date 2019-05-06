package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

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

		d.Serial = util.RandStr(12)

		disks[i] = d
	}
	return nil
}
