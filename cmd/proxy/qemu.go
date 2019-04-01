package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func qemuConfig(vmdata *vm.Data, socket string) ([]string, []*os.File, error) {
	var cpuArgs, virtioArgs string
	if vmdata.NestedVM {
		cpuArgs = ",pmu=off"
		virtioArgs = ",disable-modern=true"
	}

	var args []string
	var bus string
	switch runtime.GOARCH {
	case "amd64":
		args = []string{
			"/usr/bin/qemu-system-x86_64",
			"-device", "virtio-rng-pci,max-bytes=1024,period=1000" + virtioArgs,
			"-device", "virtio-9p-pci,fsdev=rootfs_dev,mount_tag=rootfs" + virtioArgs,
			"-device", "virtio-serial-pci" + virtioArgs,
			"-serial", "chardev:console",
			"-no-acpi",
		}
		bus = "pci"
	case "s390x":
		args = []string{
			"/usr/bin/qemu-system-s390x",
			"-device", "virtio-rng-ccw,max-bytes=1024,period=1000",
			"-device", "virtio-9p-ccw,fsdev=rootfs_dev,mount_tag=rootfs",
			"-device", "virtio-serial-ccw",
			"-device", "sclpconsole,chardev=console",
		}
		bus = "ccw"
	}

	if vmdata.Vsockd.CID != 0 {
		device := fmt.Sprintf("vhost-vsock-%s,guest-cid=%#x%s", bus, vmdata.Vsockd.CID, virtioArgs)
		args = append(args, "-device", device)
	}

	args = append(args,
		"-machine", "accel=kvm,usb=off",
		"-monitor", "none",
		"-nodefaults",
		"-name", vmdata.ContainerID[:12],
		"-enable-kvm",
		"-cpu", "host"+cpuArgs,
		"-nographic",
		"-no-reboot",
		"-no-user-config",
		"-nodefconfig",
		"-kernel", "/kernel",
		"-initrd", "/initrd",
		"-msg", "timestamp=on",
		"-fsdev", "local,id=rootfs_dev,path=/rootfs,security_model=none",
		"-chardev", "socket,path="+socket+",id=channel1",
		"-device", "virtserialport,chardev=channel1,name=com.ibm.runq.channel.1",
		"-smp", strconv.Itoa(vmdata.CPU),
		"-m", strconv.Itoa(vmdata.Mem),
		"-append", vm.KernelParameters,
		"-chardev", "stdio,id=console,signal=off",
	)

	if len(vmdata.Disks) > 0 {
		args = append(args, "-object", "iothread,id=iothread1")
	}

	for i, d := range vmdata.Disks {
		var format string
		switch d.Type {
		case vm.Qcow2Image:
			format = "qcow2"
		case vm.BlockDevice, vm.RawFile:
			format = "raw"
		default:
			return nil, nil, fmt.Errorf("invalid disk type")
		}

		aio := "threads"
		if d.Cache == "none" {
			aio = "native"
		}
		id := fmt.Sprintf("disk%d", i)

		drive := fmt.Sprintf("file=%s,if=none,format=%s,cache=%s,aio=%s,id=%s", d.Path, format, d.Cache, aio, id)
		device := fmt.Sprintf("virtio-blk-%s,serial=%s,drive=%s,iothread=iothread1%s", bus, d.Serial, id, virtioArgs)
		args = append(args, "-drive", drive)
		args = append(args, "-device", device)
	}

	if vmdata.APDevice != "" {
		args = append(args, "-device", "vfio-ap,sysfsdev="+vmdata.APDevice)
	}

	// 0:stdin, 1:stdout, 2:stderr, 3:firstTAP, 4:2ndTAP ....
	var extraFiles []*os.File
	var fd = 3
	for i, nw := range vmdata.Networks {
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
			"-device", fmt.Sprintf("virtio-net-%s,netdev=net%d,mac=%s%s", bus, i, nw.MacAddress, virtioArgs),
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
