package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli"

	"github.com/gotoz/runq/pkg/vm"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/vishvananda/netlink"
)

const (
	runqOciVersion  = "1.0"
	runqQemuRoot    = "/var/lib/runq/qemu"
	runqStartcmd    = "/qemu/proxy"
	runqQemuMountPt = "/qemu"
)

var runqCommit = "" // set via Makefile

// ProxyCapabilities same as OCI defaults plus CAP_NET_ADMIN and CAP_SYS_ADMIN
var proxyCapabilities = []string{
	"CAP_AUDIT_WRITE",
	"CAP_CHOWN",
	"CAP_DAC_OVERRIDE",
	"CAP_FOWNER",
	"CAP_FSETID",
	"CAP_KILL",
	"CAP_MKNOD",
	"CAP_NET_BIND_SERVICE",
	"CAP_NET_RAW",
	"CAP_SETFCAP",
	"CAP_SETGID",
	"CAP_SETPCAP",
	"CAP_SETUID",
	"CAP_SYS_CHROOT",
	"CAP_NET_ADMIN",
	"CAP_SYS_ADMIN",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// turnToRunq turns runc into runq.
func turnToRunq(context *cli.Context, spec *specs.Spec) error {
	if !strings.HasPrefix(spec.Version, runqOciVersion) {
		return fmt.Errorf("unsupported spec (%s), need %s.x", spec.Version, runqOciVersion)
	}

	// Check if running in privileged mode.
	if len(spec.Linux.MaskedPaths) == 0 {
		return fmt.Errorf("privileged mode is not supported")
	}
	for _, d := range spec.Linux.Devices {
		if d.Path == "/dev/mem" {
			return fmt.Errorf("privileged mode is not supported")
		}
	}

	var vmdata vm.Data

	//
	// Linux
	//
	vmdata.Linux = vm.Linux{
		ContainerID: strings.TrimSpace((context.Args()[0] + strings.Repeat(" ", 12))[:12]),
		CPU:         context.GlobalInt("cpu"),
		DNSOpts:     strings.TrimSpace(context.GlobalString("dns-opts")),
		DNSSearch:   strings.TrimSpace(context.GlobalString("dns-search")),
		GitCommit:   runqCommit,
		Mem:         context.GlobalInt("mem"),
		NestedVM:    context.GlobalBool("nestedvm"),
		Sigusr:      context.GlobalBool("sigusr"),
		Sysctl:      spec.Linux.Sysctl,
	}

	spec.Linux.Sysctl = nil

	for _, v := range strings.Split(context.GlobalString("dns"), ",") {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		vmdata.Linux.DNS = append(vmdata.Linux.DNS, v)
	}

	if err := specDevices(spec, &vmdata); err != nil {
		return err
	}

	if err := specMounts(spec, &vmdata); err != nil {
		return err
	}

	//
	// Process
	//
	vmdata.Process = vm.Process{
		Terminal:        spec.Process.Terminal,
		Cwd:             spec.Process.Cwd,
		NoNewPrivileges: spec.Process.NoNewPrivileges,
		Args:            spec.Process.Args,
	}

	spec.Process.ApparmorProfile = ""
	spec.Process.SelinuxLabel = ""
	spec.Process.Env = append(spec.Process.Env, "RUNQ_COMMIT="+runqCommit)

	vmdata.Process.Capabilities = vm.AppCapabilities{
		Ambient:     spec.Process.Capabilities.Ambient,
		Bounding:    spec.Process.Capabilities.Bounding,
		Effective:   spec.Process.Capabilities.Effective,
		Inheritable: spec.Process.Capabilities.Inheritable,
		Permitted:   spec.Process.Capabilities.Permitted,
	}

	// Capabilities for proxy process
	spec.Process.Capabilities.Ambient = proxyCapabilities
	spec.Process.Capabilities.Bounding = proxyCapabilities
	spec.Process.Capabilities.Effective = proxyCapabilities
	spec.Process.Capabilities.Inheritable = proxyCapabilities
	spec.Process.Capabilities.Permitted = proxyCapabilities

	// Transform Seccomp value (type *specs.LinuxSeccomp) into a Gob
	// so we don't have to translate it into a vm type first.
	// This is save as long as we build against a fixed OCI release.
	// We have to do this anyway. (See Gopkg.toml)
	if spec.Linux.Seccomp != nil {
		gob, err := vm.Encode(*spec.Linux.Seccomp)
		if err != nil {
			return err
		}
		vmdata.Process.SeccompGob = gob
		spec.Linux.Seccomp = nil
	}

	vmdata.Process.Rlimits = make(map[string]vm.Rlimit)
	for _, v := range spec.Process.Rlimits {
		vmdata.Process.Rlimits[v.Type] = vm.Rlimit{Hard: v.Hard, Soft: v.Soft}
	}
	spec.Process.Rlimits = nil

	//
	// User
	//
	vmdata.User = vm.User{
		UID:            spec.Process.User.UID,
		GID:            spec.Process.User.GID,
		AdditionalGids: spec.Process.User.AdditionalGids,
	}

	spec.Process.User.UID = 0
	spec.Process.User.GID = 0
	spec.Process.User.AdditionalGids = nil

	// Compress vmdata struct and encode into a Base64 string.
	// Compress is not really necessary but makes the output of ps
	// look a little less messy.
	data, err := vm.ZipEncode(vmdata)
	if err != nil {
		return fmt.Errorf("vm.Encode(vmdata): %v", err)
	}
	vmdataB64 := base64.StdEncoding.EncodeToString(data)

	spec.Process.Args = []string{runqStartcmd, "-name", vmdata.Linux.ContainerID, vmdataB64}

	return validateProcessSpec(spec.Process)
}

func specDevices(spec *specs.Spec, vmdata *vm.Data) error {
	iPtr := func(i int64) *int64 { return &i }

	major, err := macvtapMajor()
	if err != nil {
		return err
	}
	if major == 0 {
		return fmt.Errorf("can't get macvtap major device number")
	}

	// /dev/tap*
	spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, specs.LinuxDeviceCgroup{
		Allow: true, Type: "c", Major: &major, Access: "rwm",
	})
	// /dev/kvm
	spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, specs.LinuxDeviceCgroup{
		Allow: true, Type: "c", Major: iPtr(10), Minor: iPtr(232), Access: "rwm",
	})
	// /dev/vhost-net
	spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, specs.LinuxDeviceCgroup{
		Allow: true, Type: "c", Major: iPtr(10), Minor: iPtr(238), Access: "rwm",
	})

	// /dev/disk/...
	for _, d := range spec.Linux.Devices {
		if d.Type == "b" {
			switch {
			case strings.HasPrefix(d.Path, "/dev/disk/"):
				vmdata.Disks = append(vmdata.Disks, vm.Disk{Path: d.Path, Type: vm.BlockDevice})
			default:
				return fmt.Errorf("invalid path: %s", d.Path)
			}
		}
	}

	filemode := os.FileMode(0600)
	id := uint32(0)
	spec.Linux.Devices = append(spec.Linux.Devices, specs.LinuxDevice{
		Path:     "/dev/kvm",
		Type:     "c",
		Major:    10,
		Minor:    232,
		FileMode: &filemode,
		UID:      &id,
		GID:      &id,
	})

	spec.Linux.Devices = append(spec.Linux.Devices, specs.LinuxDevice{
		Path:     "/dev/vhost-net",
		Type:     "c",
		Major:    10,
		Minor:    238,
		FileMode: &filemode,
		UID:      &id,
		GID:      &id,
	})
	return nil
}

func specMounts(spec *specs.Spec, vmdata *vm.Data) error {
	var mounts []specs.Mount

	for _, m := range spec.Mounts {
		// Ignore invalid mounts.
		if strings.HasPrefix(m.Destination, runqQemuMountPt) {
			return fmt.Errorf("invalid mount point: %s", m.Destination)
		}
		if m.Type == "tmpfs" {
			if m.Destination == "/dev" {
				mounts = append(mounts, m)
			} else {
				flags, data := parseTmfpsMount(m)
				vmdata.Mounts = append(vmdata.Mounts, vm.Mount{
					Source: "tmpfs",
					Target: m.Destination,
					Fstype: "tmpfs",
					Flags:  flags,
					Data:   data,
				})
			}
			continue
		}
		// Add extra disks.
		if strings.HasPrefix(m.Destination, "/dev/disk/") {
			vmdata.Disks = append(vmdata.Disks, vm.Disk{Path: m.Destination, Type: vm.DisktypeUnknown})
		}

		mounts = append(mounts, m)
	}

	// Bind-mount Qemu root directory to every VM.
	mounts = append(mounts, specs.Mount{
		Destination: runqQemuMountPt,
		Type:        "bind",
		Source:      runqQemuRoot,
		Options:     []string{"rbind", "nosuid", "nodev", "ro", "rprivate"},
	})

	spec.Mounts = mounts

	return nil
}

func parseTmfpsMount(m specs.Mount) (int, string) {
	var dataArray []string
	var flags int
	for _, o := range m.Options {
		switch o {
		case "default":
		case "rw":
		case "noatime":
			flags |= syscall.MS_NOATIME
		case "nodiratime":
			flags |= syscall.MS_NODIRATIME
		case "nodev":
			flags |= syscall.MS_NODEV
		case "noexec":
			flags |= syscall.MS_NOEXEC
		case "nosuid":
			flags |= syscall.MS_NOSUID
		case "ro":
			flags |= syscall.MS_RDONLY
		case "rprivate", "rshared", "rslave", "runbindable":
		default:
			dataArray = append(dataArray, o)
		}
	}
	data := strings.Join(dataArray, ",")
	return flags, data
}

// macvtapMajor tries to figure out the dynamic device number of the
// macvtap driver. It creates a dummy macvtap device to force
// loading of the macvtap kernel modules.
func macvtapMajor() (int64, error) {
	major, err := parseProcDevice()
	if err != nil {
		return 0, err
	}
	if major != 0 {
		return major, nil
	}

	links, err := netlink.LinkList()
	if err != nil {
		return 0, fmt.Errorf("LinkList: %v", err)
	}

	for _, link := range links {
		if link.Type() == "bridge" {
			la := netlink.NewLinkAttrs()
			la.Name = fmt.Sprintf("tap%d", rand.Int31())
			la.ParentIndex = link.Attrs().Index
			mvt := &netlink.Macvtap{
				Macvlan: netlink.Macvlan{
					LinkAttrs: la,
					Mode:      netlink.MACVLAN_MODE_BRIDGE,
				},
			}
			if err := netlink.LinkAdd(mvt); err != nil {
				return 0, fmt.Errorf("LinkAdd: %v", err)
			}

			mvtLink, err := netlink.LinkByName(la.Name)
			if err != nil {
				return 0, fmt.Errorf("LinkByName: %v", err)
			}
			if err := netlink.LinkDel(mvtLink); err != nil {
				return 0, fmt.Errorf("LinkDel: %v", err)
			}
			break
		}
	}
	return parseProcDevice()
}

func parseProcDevice() (int64, error) {
	var err error

	fd, err := os.Open("/proc/devices")
	if err != nil {
		return 0, err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)

	var major int64
	for scanner.Scan() {
		s := strings.Fields(scanner.Text())
		if len(s) > 1 {
			if s[1] == "macvtap" {
				major, err = strconv.ParseInt(s[0], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("Atoi %s: %v", s[0], err)
				}
				break
			}
		}
	}
	return major, scanner.Err()
}
