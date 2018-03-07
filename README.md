[![Build Status](https://travis-ci.org/gotoz/runq.svg?branch=master)](https://travis-ci.org/gotoz/runq)

# runq

runq is a hypervisor-based Docker runtime based on [runc](https://github.com/opencontainers/runc)
to run regular Docker images in a lightweight KVM/Qemu virtual machine.
The focus is on solving real problems, not on number of features.

Key differences to other hypervisor-based runtimes:
* minimalistic design, small code base
* no modification to exiting Docker tools (dockerd, containerd, runc...)
* coexistance of runq containers and regular runc containers
* no extra state outside of Docker (no libvirt, no changes to /var/run/...)
* simple init daemon, no systemd, no busybox
* no custom guest kernel or custom qemu needed
* runs on x86_64 and s390x

## runc vs. runq
```
       runc container                   runq container
       +-------------------------+      +-------------------------+
       |                         |      | +---------------------+ |
       |                         |      | |                  VM | |
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
        "--dns", "8.8.8.8,8.8.4.4"
      ]
    }
  }
}
```

reload Docker config
```
systemctl reload docker.service
```
Note: To deploy runq on further Docker hosts only `/var/lib/runq` and `/etc/docker/daemon.json`
must be copied.

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
    -v $PWD/data.img:/dev/disk/writeback/ext4/var/lib/postgresql \
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

## runq Components
```
 docker cli
    dockerd engine
       docker-containerd-shim
            runq                                           container
          +--------------------------------------------------------+
          |                                                        |
docker0   |                                                  VM    |
   veth <---> veth                  +--------------------------+   |
          |        `<--- macvtap ---|-------> eth0             |   |
          |                         |                          |   |
          |   proxy                 |      init                |   |
          |                         |                          |   |
          |     msg, signals  <-----|------->   vport          |   |
          |                         |                          |   |
          |     /overlayfs    <-----|------->   /app           |   |
          |                         |                          |   |
          |     block dev     <-----|------->   /dev/xvda      |   |
          |                         |                          |   |
          |                         +--------------------------+   |
          |                         |       guest kernel       |   |
          |                         +--------------------------+   |
          |                                     qemu               |
          |                                                        |
          +--------------------------------------------------------+

 --------------------------------------------------------------------------
                                host kernel
```
* cmd/runq
    - new docker runtime

* cmd/proxy
    - first process in container (PID 1)
    - new Docker entry point
    - configures and starts Qemu (network, disks, ...)
    - forwards signals to VM init
    - receives application exit code

* cmd/init
    - first process in VM (PID 1)
    - initializes the VM guest (network, disks, ...)
    - starts/stops target app (Docker entry point)
    - sends signals to target application
    - forwards application exit code back to proxy

* qemu
    - creates `/var/lib/runq/qemu`
    - read-only volume attached to every container
    - contains proxy, qemu, kernel and initrd

* initrd
    - temporary root file system of the VM
    - contains only init and a few kernel modules

* pkg/vm
    - type definitions and configuration

* pkg/util
    - utiliy functions used accross all commands

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

Docker uses an embedded DNS server (127.0.0.11) for containers that are
connected to custom networks. This IP is not reachable from within the VM.
Therefore DNS for runq containers must be configured seperatetly.

DNS configuration can be done globally via runtime options specified in
'daemon.json' (see example above) or via environment variables for each
container at container start.
The environment variables are `RUNQ_DNS`, `RUNQ_DNS_OPT` and `RUNQ_DNS_SEARCH`.
Environment variables have priority over global options.

## Storage
Extra storage can be added in the form of Qcow2 images, raw file images or
regular block devices. Devices will be mounted automatically if they contain a
supported filesytem and a mountpoint has been specified.
Supported filesystems are ext2, ext3, ext4 and xfs.

The mount point must be prefixed with `/dev/disk` and one of the
supported cache types (writeback, writethrough, none or unsafe).
See man qemu(1) for details.

Syntax:
```
--volume <image  name>:/dev/disk/<cache type>/<filesystem type>/<mountpoint>
--device <device name>:/dev/disk/<cache type>/<filesystem type>/<mountpoint>
```

### Storage examples
Mount the existing Qcow image  `/data.qcow2` that contains an xfs filesystem
to `/mnt/data`.
```
docker run --volume /data.qcow2:/dev/disk/writeback/xfs/mnt/data ...
```

Attach the host device `/dev/sdb1`  with an ext4 filesystem to `/mnt/data2`.
```
docker run --device /dev/sdb1:/dev/disk/writethrough/ext4/mnt/data2 ...
```

Attach the host device `/dev/sdb2` but disable outomatic mounting. Use `none` as
filesystem and any uniq ID to distingish multiple devices. The device
will show up as `/dev/vda` inside the container.
```
docker run --device /dev/sdb2:/dev/disk/writethrough/none/0001 ...
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

## SIGUSR1 and SIGUSR2
When sigusr is enabled the directory `/var/lib/runq/qemu/.runq`
will be bind-mounted into the container VM under /.runq (read-only).
Sending a signal SIGUSR1 or SIGUSR2 to the container will then trigger
the execution of `/.runq/SIGUSR1` or `/.runq/SIGUSR2` in the VM.

This feature must be enabled explicitly via the `--sigusr` runtime
option (see daemon.json).

The siguser command will run with uid 0 and gid 0, environment variables.
The seccomp profile and the capabilities are the same as for the application
process. If this feature is not enabled then the signals SIGUSR1 and SIGUSR2
will be forwarded to the application process as usual.
Note: The default behavior of a process receiving SIGUSR1 or SIGUSR2 is to terminate.
```
docker kill --signal SIGUSR1 <container ID>
```
## Limitations
Most docker commands and options work as expected. However, due to
the fact that the target application runs inside a Qemu VM which itself runs
inside a Docker container and because of the minimalistic design principle of runq
some docker commands and options don't work. E.g:
* adding / removing networks and storage dynamically
* docker exec
* docker swarm
* privileged mode
* apparmor, selinux, ambient

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
--interactive               --user
--ip                        --volume
--link                      --volumes-from
--mount                     --workdir
```

## Developing runq
For fast development cycles runq can be build on the host as follows:
1. Prerequisites:
* Docker >= 17.09.x-ce
* Go >= 1.9
* GOPATH must be set
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
    make -C qemu
    ```
4. Compile and install runq components to `/var/lib/runq`
    ```
    make install
    sudo chown -R root:root /var/lib/runq
    ```
5. Register runq as Docker runtime with appropriate defaults as shown in section *Installation* above.

## Contributing
See [CONTRIBUTING](CONTRIBUTING.md) for details.

## License
The code is licensed under the Apache License 2.0.<br>
See [LICENSE](LICENSE) for further details.
