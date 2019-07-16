[![Build Status](https://travis-ci.org/gotoz/runq.svg?branch=master)](https://travis-ci.org/gotoz/runq)

# runq

runq is a hypervisor-based Docker runtime based on [runc](https://github.com/opencontainers/runc)
to run regular Docker images in a lightweight KVM/Qemu virtual machine.
The focus is on solving real problems, not on number of features.

Key differences to other hypervisor-based runtimes:
* minimalistic design, small code base
* no modification to existing Docker tools (dockerd, containerd, runc...)
* coexistence of runq containers and regular runc containers
* no extra state outside of Docker (no libvirt, no changes to /var/run/...)
* small init program, no systemd
* no custom guest kernel or custom qemu needed
* runs on x86_64 and s390x

## runc vs. runq
```
       runc container                   runq container
       +-------------------------+      +-------------------------+
       |                         |      |                     VM  |
       |                         |      | +---------------------+ |
       |                         |      | |                     | |
       |                         |      | |                     | |
       |                         |      | |                     | |
       |       application       |      | |     application     | |
       |                         |      | |                     | |
       |                         |      | |                     | |
       |                         |      | +---------------------+ |
       |                         |      | |     guest kernel    | |
       |                         |      | +---------------------+ |
       |                         |      |           qemu          |
       +-------------------------+      +-------------------------+
 ----------------------------------------------------------------------
                                host kernel
```

## Installation
runq requires a host kernel >= 4.8 with KVM and VHOST_VSOCK support enabled.
The easiest way to build runq and to put all dependencies together is using Docker.
For fast development cycles a regular build environment might be more
efficient. For this refer to section *Developing runq* below.

```
# get the source
git clone https://github.com/gotoz/runq.git
cd runq

# compile and create a release tar file in Docker container
make release

# install runq to `/var/lib/runq`
make release-install
```

Register runq as Docker runtime with appropriate defaults. See [daemon.json](test/testdata/daemon.json) for more options.
```
/etc/docker/daemon.json
{
  "runtimes": {
    "runq": {
      "path": "/var/lib/runq/runq",
      "runtimeArgs": [
        "--cpu", "1",
        "--mem", "256",
        "--dns", "8.8.8.8,8.8.4.4",
        "--tmpfs", "/tmp"
      ]
    }
  }
}
```

reload Docker config
```
systemctl reload docker.service
```

#### TLS certificates
*runq-exec* creates a secure connection between host and VM guests. Users of *runq-exec* are
authenticated via a client certificate. Access to the client certificate must be limitted to
Docker users only.

The CA and server certificates must be installed in `/var/lib/runq/qemu/certs`.
Access must be limmited to the root user only.

Examples of server and client TLS certificates can be created with the script:
```
/var/lib/runq/qemu/mkcerts.sh
```
Note: The host must provide sufficient entropy to the VM guests. If there is not enough
entropie available booting of guests can fail with a timeout error. The entropy that's
currently available can be checked with:
```
cat /proc/sys/kernel/random/entropy_avail
```
The number returned should always be greater than 1000.

#### Kernel module vhost_vsock
The kernel module `vhost_vsock` must be loaded on the host. This can be achieved by creating
a config file for the systemd-modules-load service: `/etc/modules-load.d/vhost-vsock.conf`:
```
# Load vhost_vsock for runq
vhost_vsock
```

## Usage examples

the simplest example
```
docker run --runtime runq -ti busybox sh
```

custom VM with 512MiB memory and 2 CPUs
```
docker run --runtime runq -e RUNQ_MEM=512 -e RUNQ_CPU=2 -ti busybox sh
```

allow loading of extra kernel modules by adding the SYS_MODULE capabilitiy
```
docker run --runtime runq --cap-add sys_module -ti busybox sh -c "modprobe brd && lsmod"

```

full example PostgreSQL with custom storage
```
dd if=/dev/zero of=data.img bs=1M count=200
mkfs.ext4 -F data.img

docker run \
    --runtime runq \
    --name pgserver \
    -e RUNQ_CPU=2 \
    -e RUNQ_MEM=512 \
    -e POSTGRES_PASSWORD=mysecret \
    -v $PWD/data.img:/dev/runq/0001/none/ext4/var/lib/postgresql \
    -d postgres:alpine

sleep 10

docker run \
    --runtime runq \
    --link pgserver:postgres \
    --rm \
    -e PGPASSWORD=mysecret \
    postgres:alpine psql -h postgres -U postgres -c "select 42 as answer;"

#  answer
# --------
#      42
# (1 row)

```

### Container with Systemd
To run images that have their own init program such as Systemd the entry-point string
must contain `/sbin/init`. This ensures that `poweroff` and `reboot` executed inside
the container work as expected.(The exit code of systemd must be handled differently.)

See [test/examples/Dockerfile.systemd](test/examples/Dockerfile.systemd)
and [test/examples/systemd.sh](test/examples/systemd.sh) for an example.

### /.runqenv
Runq can write the container environment variables in a file named `/.runqenv` placed in
the root directroy of the container. This might be useful for containers running Systemd
as entry point. This feature can be enabled globally by configuring `--runqenv` in
[/etc/docker/daemon.json](test/testdata/daemon.json) or for a single container via the
environment variable `RUNQ_RUNQENV`.

## runq Components
```
   docker cli
      dockerd engine
         docker-containerd-shim
               runq                                           container
              +--------------------------------------------------------+
              |                                                        |
  docker0     |                                                  VM    |
    `veth <------> veth                 +--------------------------+   |
              |        `<--- macvtap ---|-> eth0                   |   |
              |  proxy  <-----------------> init                   |   |
  runq-exec <-----------tls----------------> `vsockd               |   |
              |                         |---------------namespace--|   |
 overlayfs <-----9pfs-------------------|-> /                      |   |
              |                         |                          |   |
 block dev <-----virtio-blk-------------|-> /dev/vdx               |   |
              |                         |                          |   |
              |                         |                          |   |
              |                         |                          |   |
              |                         |       application        |   |
              |                         |                          |   |
              |                         +--------------------------+   |
              |                         |       guest kernel       |   |
              |                         +--------------------------+   |
              |                                     qemu               |
              +--------------------------------------------------------+

 --------------------------------------------------------------------------
                                host kernel
```
* cmd/runq
    - new docker runtime

* cmd/proxy
    - new Docker entry point
    - first process in container (PID 1)
    - configures and starts Qemu (network, disks, ...)
    - forwards signals to VM init
    - receives application exit code

* cmd/init
    - first process in VM (PID 1)
    - initializes the VM guest (network, disks, ...)
    - starts entry-point in PID and Mount namespace
    - sends signals to target application
    - forwards application exit code back to proxy

* cmd/runq-exec
    - command line utility similar to *docker exec*

* cmd/nsenter
    - enters the namespaces of entry-point for runq-exec

* qemu
    - creates `/var/lib/runq/qemu`
    - read-only volume attached to every container
    - contains qemu rootfs (proxy, qemu, kernel and initrd)

* initrd
    - prepares the initrd to boot the VM

* pkg
    - helper packages

## runq-exec
runq-exec (`/var/lib/runq/runq-exec`) is a command line utility similar to **docker exec**. It allows running
additional commands in existing runq containers executed from the host. It uses
[VirtioVsock](https://wiki.qemu.org/Features/VirtioVsock) for the communication
between host and VMs. TLS is used for encryption and client authorization. Support for
`runq-exec` can be disabled by setting the container environment variable `RUNQ_NOEXEC`
or by `--noexec` in [/etc/docker/daemon.json](test/testdata/daemon.json).
```
Usage:
  runq-exec [options] <container> command args

Run a command in a running runq container

Options:
  -c, --tlscert string    TLS certificate file (default "/var/lib/runq/cert.pem")
  -k, --tlskey string     TLS private key file (default "/var/lib/runq/key.pem")
  -e, --env stringArray   Set environment variables for command
  -h, --help              Print this help
  -i, --interactive       Keep STDIN open even if not attached
  -t, --tty               Allocate a pseudo-TTY
  -v, --version           Print version

Environment Variable:
  DOCKER_HOST    specifies the Docker daemon socket.

Example:
  runq-exec -ti a6c3b7c bash
```

## Qemu and guest Kernel
runq runs Qemu and Linux Kernel from the `/var/lib/runq/qemu` directory
on the host. This directory is populated by `make -C qemu`. For simplicity
Qemu and the Linux kernel are taken from the Ubuntu 18.04 LTS Docker base image.
See [qemu/x86_64/Dockerfile](qemu/x86_64/Dockerfile) for details.
This makes runq independent of the Linux distribution on the host.
Qemu does not need to be installed on the host.

The kernel modules directory (`/var/lib/runq/qemu/lib/modules`)
is `bind-mounted` into every container to `/lib/modules`.
This allows the loading of extra kernel modules in any container if needed.
For this SYS_MODULES capability is required (`--cap-add sys_modules`).

## Networking
runq uses Macvtap devices to connect Qemu VirtIO interfaces to Docker
bridges. By default a single ethernet interface is created.
Multiple networks can be used by connecting a container to the networks
before start. See [test/integration/net.sh](test/integration/net.sh) as an
example.

runq container can also be connected to one or more Docker networks of type Macvlan.
This allows a direct connection between the VM and the physical host network
without bridge and without NAT. See https://docs.docker.com/network/macvlan/ for details.

For custom networks the docker daemon implements an embedded DNS server which provides
built-in service discovery for any container created with a valid container name.
This Docker DNS server (listen address 127.0.0.11:53) is reachable only by runc containers
and not by runq containers.
A work-around is to run one or more DNS proxy container in the custom network with runc and
use the proxy IP address for DNS of runq containers.
See [test/examples/dnsproxy.sh](test/examples/dnsproxy.sh) for details on how to setup a DNS proxy.

DNS configuration without proxy can be done globally via runtime options specified in
'/etc/docker/daemon.json' (see example above) or via environment variables for each
container at container start.
The environment variables are `RUNQ_DNS`, `RUNQ_DNS_OPT` and `RUNQ_DNS_SEARCH`.
Environment variables have priority over global options.


## Storage
Extra storage can be added in the form of Qcow2 images, raw file images or
regular block devices. Storage devices will be mounted automatically if
a filesytem and a mount point has been specified.
Supported filesystems are ext2, ext3, ext4, xfs and btrfs.
Cache type must be writeback, writethrough, none or unsafe.
Cache type "none" is recommended for filesystems that support `O_DIRECT`.
See man qemu(1) for details about different cache types.

Syntax:
```
--volume <image  name>:/dev/runq/<id>/<cache type>[/<filesystem type><mount point>]
--device <device name>:/dev/runq/<id>/<cache type>[/<filesystem type><mount point>]
```

`<id>` is used to create symbolic links inside the VM guest that point to the Qemu Virtio device
files. The `id` can be any character string that matches the regex pattern `"^[a-zA-Z0-9-_]{1,36}$"`
but it must be unique within a container.
```
/dev/disk/by-runq-id/0001 -> ../../vda
```

### Storage examples
Mount the existing Qcow image `/data.qcow2` with xfs filesystem to `/mnt/data`:
```
docker run -v /data.qcow2:/dev/runq/0001/none/xfs/mnt/data ...
```

Attach the host device `/dev/sdb1` formatted with ext4 to `/mnt/data2`:
```
docker run --device /dev/sdb1:/dev/runq/0002/writethrough/ext4/mnt/data2 ...
```

Attach the host device `/dev/sdb2` without mounting:
```
docker run --device /dev/sdb2:/dev/runq/0003/writethrough ...
```

## Capabilities
By default runq drops all capabilities except those needed (same as regular Docker does).
The white list of the remaining capabilities is provided by the Docker engine.

`AUDIT_WRITE CHOWN DAC_OVERRIDE FOWNER FSETID KILL MKNOD NET_BIND_SERVICE
NET_RAW SETFCAP SETGID SETPCAP SETUID SYS_CHROOT`

See `man capabilities` for a list of all available capabilities.
Additional Capabilities can be added to the whitelist at container start:
```
docker run --cap-add SYS_TIME --cap-add SYS_MODULE ...`
```

## Seccomp
runq supports the [default Docker seccomp profile](https://github.com/docker/docker-ce/blob/master/components/engine/profiles/seccomp/default.json) as well as custom profiles.
```
docker run --security-opt seccomp=<profile-file> ...
```
The default profile is defined by the Docker daemon and gets applied automatically.
Note: Only the runq init binary is statically linked against libseccomp.
Therefore libseccomp is needed only at compile time.

If the host operating system where runq is beeing built does not provide static libseccomp
libraries one can also simply build and install [libseccomp](https://github.com/seccomp/libseccomp/)
from the sources.

Seccomp can be disabled at container start:
```
docker run --security-opt seccomp=unconfined ...
```

Note: Some Docker daemon don't support custom Seccomp profiles. Run `docker info` to verify
that Seccomp is supported by your daemon. If it is supported the output of `docker info` looks like this:
```
Security Options:
 seccomp
  Profile: default
```

## AP adapter passthrough (s390x only)
AP devices provide cryptographic functions to all CPUs assigned to a linux system running in
an IBM Z system LPAR. AP devices can be made available to a runq container by passing a VFIO mediated
device from the host through Qemu into the runq VM guest. VFIO mediaded devices are enabled by the
`vfio_ap` kernel module and allow for partitioning of AP devices and domains. The environment variable RUNQ_APUUID spcecifies the VFIO mediated device UUID. runq automatically loads the required zcrypt kernel modules inside the VM. E.g.:
```
docker run --runtime runq -e RUNQ_APUUID=b34543ee-496b-4769-8312-83707033e1de ...
```
For details on how to setup mediated devices on the host see
https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/Documentation/s390/vfio-ap.txt

## Limitations
Most docker commands and options work as expected. However, due to
the fact that the target application runs inside a Qemu VM which itself runs
inside a Docker container and because of the minimalistic design principle of runq
some docker commands and options don't work. E.g:
* adding / removing networks and storage dynamically
* docker exec (see runq-exec)
* docker swarm
* privileged mode
* apparmor, selinux, ambient
* docker HEALTHCHECK

The following common options of `docker run` are supported:
```
--attach                    --name
--cap-add                   --network
--cap-drop                  --publish
--cpus                      --restart
--cpuset-cpus               --rm
--detach                    --runtime
--entrypoint                --sysctl
--env                       --security-opt seccomp=unconfined
--env-file                  --security-opt no-new-privileges
--expose                    --security-opt seccomp=<filter-file>
--group-add                 --tmpfs
--help                      --tty
--hostname                  --ulimit
--init                      --user
--interactive               --volume
--ip                        --volumes-from
--link                      --workdir
--mount
```

### Nested VM
A nested VM is a virtual machine that runs inside of a virtual machine. In plain KVM this feature is
considered working but not meant for production use. Running KVM guests inside guests of other
hypervisors such as VMware might not work as expected or might not work at all.
However to try out runq in a VM guest the (experimental) runq runtime configuration parameter
`--nestedvm` can be used. It modifies the parameters of the Qemu process.

## Developing runq
For fast development cycles runq can be build on the host as follows:
1. Prerequisites:
* Docker >= 17.09.x-ce
* Go >= 1.11
* `/var/lib/runq` must be writable by the current user
* [Libseccomp](https://github.com/seccomp/libseccomp/) static library.
E.g. `libseccomp-dev` for Ubuntu or `libseccomp-static` for Fedora

2. Download runc and runq source code
    ```
    go get -d -u github.com/opencontainers/runc
    go get -d -u github.com/gotoz/runq
    ```
3. Install Qemu and guest kernel to `/var/lib/runq/qemu`<br>
All files are taken from the Ubuntu 18.04 LTS Docker base image.
    ```
    cd $GOPATH/src/github.com/gotoz/runq
    make -C qemu all install
    ```
4. Compile and install runq components to `/var/lib/runq`
    ```
    make install
    ```
5. Create TLS certificates
    ```
    /var/lib/runq/qemu/mkcerts.sh
    ```
6. Adjust file and directory permissions
    ```
    sudo chown -R root:root /var/lib/runq
    ```
7. Register runq as Docker runtime with appropriate defaults as shown in section *Installation* above.

## Contributing
See [CONTRIBUTING](CONTRIBUTING.md) for details.

## License
The code is licensed under the Apache License 2.0.<br>
See [LICENSE](LICENSE) for further details.
