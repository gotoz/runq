package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"
)

func updateDisktype(d *vm.Disk) error {
	qcowMagic := []byte{0x51, 0x46, 0x49, 0xfb}

	fi, err := os.Stat(d.Path)
	if err != nil {
		return errors.WithStack(err)
	}

	if !fi.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", d.Path)
	}

	r, err := os.Open(d.Path)
	if err != nil {
		return err
	}
	defer r.Close()

	buf := make([]byte, 4)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return err
	}
	if bytes.Equal(qcowMagic, buf) {
		d.Type = vm.Qcow2Image
	} else {
		d.Type = vm.RawFile
	}
	return nil
}

func updateDisks(disks []vm.Disk) error {
	for i, d := range disks {
		var err error

		d.Serial = util.RandStr(12)

		//  0    1     2           3    4
		// /dev/disk/writethrough/ext4/mnt
		// /dev/disk/writethrough/none/0001
		p := strings.Split(strings.Trim(d.Path, "/ "), "/")

		if len(p) < 5 {
			return fmt.Errorf("invalid disk: %s", d.Path)
		}

		switch p[2] {
		case "none", "writeback", "writethrough", "unsafe":
			d.Cache = p[2]
		default:
			return fmt.Errorf("invalid cache type for %s", d.Path)
		}

		switch p[3] {
		case "ext2", "ext3", "ext4", "xfs", "btrfs":
			d.Fstype = p[3]
			d.Dir = "/" + filepath.Join(p[4:]...)
		case "none":
		default:
			return fmt.Errorf("unsupported filesystem '%s' in %s", p[3], d.Path)
		}

		if d.Type == vm.DisktypeUnknown {
			if err = updateDisktype(&d); err != nil {
				return errors.WithStack(err)
			}
		}
		disks[i] = d
	}
	return nil
}
