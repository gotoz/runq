package main

import (
	"fmt"
	"runtime"
	"strconv"

	"github.com/gotoz/runq/pkg/vm"
)

func qemuConfig(vmdata *vm.Data, socket string) ([]string, error) {
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

	disks := append([]vm.Disk(nil), vmdata.ExternalDisks...)
	disks = append(disks, vmdata.EmbeddedDisks...)
	if len(disks) > 0 {
		args = append(args, "-object", "iothread,id=iothread1")
	}

	for i, d := range disks {
		var format string
		switch d.Type {
		case vm.DisktypeQcow2Image:
			format = "qcow2"
		case vm.DisktypeBlockDevice, vm.DisktypeRawFile:
			format = "raw"
		default:
			return nil, fmt.Errorf("invalid disk type")
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

	for i, nw := range vmdata.Networks {
		fd := i + 3 // 0=stdin, 1=stdout, 2=stderr, 3=1st.TAP, 4=2nd.TAP ...
		args = append(args,
			"-device", fmt.Sprintf("virtio-net-%s,netdev=net%d,mac=%s%s", bus, i, nw.MacAddress, virtioArgs),
			"-netdev", fmt.Sprintf("tap,id=net%d,vhost=on,fd=%d", i, fd),
		)
	}

	return args, nil
}
