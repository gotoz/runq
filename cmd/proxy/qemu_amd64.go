package main

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/gotoz/runq/internal/cfg"
	"github.com/gotoz/runq/pkg/vm"
)

func qemuArgs(vmdata *vm.Data, socket, share string) ([]string, error) {
	shareName := filepath.Base(share)

	var cpuArgs, virtioArgs string
	if vmdata.NestedVM {
		cpuArgs = ",pmu=off"
		virtioArgs = ",disable-modern=true"
	}

	args := []string{
		"/usr/bin/qemu-system-x86_64",
		"-device", "virtio-rng-pci,max-bytes=1024,period=1000" + virtioArgs,
		"-device", "virtio-9p-pci,fsdev=share,mount_tag=" + shareName + virtioArgs,
		"-device", "virtio-serial-pci" + virtioArgs,
		"-serial", "chardev:console",
		"-no-acpi",
		"-machine", "accel=kvm,usb=off",
		"-monitor", "none",
		"-nodefaults",
		"-name", vmdata.ContainerID[:12],
		"-enable-kvm",
		"-cpu", vmdata.CPUArgs + cpuArgs,
		"-vnc", "none",
		"-display", "none",
		"-no-reboot",
		"-no-user-config",
		"-kernel", "/kernel",
		"-initrd", "/initrd",
		"-msg", "timestamp=on",
		"-fsdev", "local,id=share,path=" + share + ",security_model=none,multidevs=remap",
		"-chardev", "socket,path=" + socket + ",id=channel1",
		"-device", "virtserialport,chardev=channel1,name=com.ibm.runq.channel.1",
		"-smp", fmt.Sprintf("%d,sockets=%d,cores=1,threads=1", vmdata.CPU, vmdata.CPU),
		"-m", strconv.Itoa(vmdata.Mem),
		"-append", cfg.KernelParameters,
		"-chardev", "stdio,id=console,signal=off",
	}

	if vmdata.Vsockd.CID != 0 {
		device := fmt.Sprintf("vhost-vsock-pci,guest-cid=%#x%s", vmdata.Vsockd.CID, virtioArgs)
		args = append(args, "-device", device)
	}

	if len(vmdata.Disks) > 0 {
		args = append(args, "-object", "iothread,id=iothread1")
		for i, d := range vmdata.Disks {
			var format string
			switch d.Type {
			case vm.Qcow2Image:
				format = "qcow2"
			case vm.BlockDevice, vm.RawFile:
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
			device := fmt.Sprintf("virtio-blk-pci,serial=%s,drive=%s,iothread=iothread1%s", d.Serial, id, virtioArgs)
			args = append(args, "-drive", drive)
			args = append(args, "-device", device)
		}
	}

	for i, nw := range vmdata.Networks {
		fd := i + 3 // 0=stdin, 1=stdout, 2=stderr, 3=1st.TAP, 4=2nd.TAP ...
		args = append(args,
			"-device", fmt.Sprintf("virtio-net-pci,netdev=net%d,mac=%s%s", i, nw.MacAddress, virtioArgs),
			"-netdev", fmt.Sprintf("tap,id=net%d,vhost=on,fd=%d", i, fd),
		)
	}

	return args, nil
}
