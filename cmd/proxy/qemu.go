package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"
)

func qemuConfig(vmdata *vm.Data, socket string) ([]string, []*os.File, error) {
	var cpuArgs, virtioArgs string
	if vmdata.NestedVM {
		cpuArgs += ",pmu=off"
		virtioArgs = ",disable-modern=true"
	}

	commonArgs := []string{
		"-machine", "accel=kvm,usb=off",
		"-monitor", "none",
		"-nodefaults",
		"-name", vmdata.ContainerID,
		"-enable-kvm",
		"-cpu", "host" + cpuArgs,
		"-nographic",
		"-no-reboot",
		"-no-user-config",
		"-nodefconfig",
		"-kernel", "/kernel",
		"-initrd", "/initrd",
		"-msg", "timestamp=on",
		"-fsdev", "local,id=rootfs_dev,path=/rootfs,security_model=none",
		"-device", "virtio-serial" + virtioArgs,
		"-chardev", "socket,path=" + socket + ",id=channel1",
		"-device", "virtserialport,chardev=channel1,name=com.ibm.runq.channel.1",
		"-smp", strconv.Itoa(vmdata.CPU),
		"-m", strconv.Itoa(vmdata.Mem),
		"-append", vm.KernelParameters,
	}

	customArgs := map[string][]string{
		"amd64": {
			"/usr/bin/qemu-system-x86_64",
			"-device", "virtio-9p-pci,fsdev=rootfs_dev,mount_tag=rootfs" + virtioArgs,
			"-chardev", "stdio,id=console,signal=off",
			"-serial", "chardev:console",
			"-no-acpi",
		},
		"s390x": {
			"/usr/bin/qemu-system-s390x",
			"-device", "virtio-9p-ccw,fsdev=rootfs_dev,mount_tag=rootfs",
			"-chardev", "stdio,id=console,signal=off",
			"-device", "sclpconsole,chardev=console",
		},
	}

	bus := map[string]string{
		"amd64": "pci",
		"s390x": "ccw",
	}

	arch := runtime.GOARCH

	args := append(customArgs[arch], commonArgs...)

	for i, d := range vmdata.Disks {
		var writeCache string
		switch d.Cache {
		case "writeback", "none", "unsafe":
			writeCache = "on"
		case "writethrough", "directsync":
			writeCache = "off"
		default:
			return nil, nil, fmt.Errorf("invalid cache type: %s", d.Cache)
		}

		var format string
		switch d.Type {
		case vm.Qcow2Image:
			format = "qcow2"
		case vm.BlockDevice, vm.RawFile:
			format = "raw"
		default:
			return nil, nil, fmt.Errorf("invalid disk type")
		}

		id := fmt.Sprintf("disk%d", i)
		drive := fmt.Sprintf("file=%s,if=none,format=%s,cache=%s,id=%s", d.Path, format, d.Cache, id)
		device := fmt.Sprintf("virtio-blk-%s,serial=%s,drive=%s,write-cache=%s%s", bus[arch], d.Serial, id, writeCache, virtioArgs)
		args = append(args, "-drive", drive)
		args = append(args, "-device", device)
	}

	// Append tap devices.
	// 0:stdin, 1:stdout, 2:stderr, 3:firstTAP, 4:2ndTAP ....
	var extraFiles []*os.File
	var fd = 3
	for i, nw := range vmdata.Networks {
		// tap network
		name, err := createTapDevice(nw.MvtName, nw.MvtIndex)
		if err != nil {
			return nil, nil, err
		}
		f, err := os.OpenFile(name, os.O_RDWR, 0600|os.ModeExclusive)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		extraFiles = append(extraFiles, f)

		args = append(args,
			"-device", fmt.Sprintf("virtio-net-%s,netdev=net%d,mac=%s%s", bus[arch], i, nw.MacAddress, virtioArgs),
			"-netdev", fmt.Sprintf("tap,id=net%d,vhost=on,fd=%d", i, fd),
		)
		fd++
	}

	return args, extraFiles, nil
}

func createTapDevice(name string, index int) (string, error) {
	sys := fmt.Sprintf("/sys/devices/virtual/net/%s/tap%d/dev", name, index)
	buf, err := ioutil.ReadFile(sys)
	if err != nil {
		return "", errors.WithStack(err)
	}

	s := strings.Split(strings.TrimSpace(string(buf)), ":")
	major, err := strconv.Atoi(s[0])
	if err != nil {
		return "", errors.Wrapf(err, "Atoi %s", s[0])
	}
	minor, err := strconv.Atoi(s[1])
	if err != nil {
		return "", errors.Wrapf(err, "Atoi %s", s[1])
	}

	path := "/dev/" + name
	if err := util.Mknod(path, "c", 0600, major, minor); err != nil {
		return "", err
	}
	return path, nil
}
