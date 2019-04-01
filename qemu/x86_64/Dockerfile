FROM ubuntu:18.04

ENV DEBIAN_FRONTEND noninteractive
ENV GOPATH /go
ENV QEMU_ROOT /var/lib/runq/qemu

WORKDIR /go/src/github.com/gotoz/runq

RUN echo "do_initrd = no" >> /etc/kernel-img.conf \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        busybox-static \
        ca-certificates \
        cpio \
        git \
        golang \
        libseccomp-dev \
        linux-virtual \
        pkg-config \
        qemu-kvm \
        xz-utils

RUN go get -d github.com/opencontainers/runc

RUN mkdir -p \
    $QEMU_ROOT/dev \
    $QEMU_ROOT/proc \
    $QEMU_ROOT/lib \
    $QEMU_ROOT/rootfs \
    $QEMU_ROOT/sys

RUN rm -f /lib/modules/*/build \
    && echo base  /lib/modules/*/kernel/fs/fscache/fscache.ko                                > $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/net/9p/9pnet.ko                                     >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/net/9p/9pnet_virtio.ko                              >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/fs/9p/9p.ko                                         >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/drivers/block/virtio_blk.ko                         >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/drivers/net/virtio_net.ko                           >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/drivers/char/hw_random/virtio-rng.ko                >> $QEMU_ROOT/kernel.conf \
    && echo vsock /lib/modules/*/kernel/net/vmw_vsock/vsock.ko                              >> $QEMU_ROOT/kernel.conf \
    && echo vsock /lib/modules/*/kernel/net/vmw_vsock/vmw_vsock_virtio_transport_common.ko  >> $QEMU_ROOT/kernel.conf \
    && echo vsock /lib/modules/*/kernel/net/vmw_vsock/vmw_vsock_virtio_transport.ko         >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/lib/raid6/raid6_pq.ko                               >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/lib/zstd/zstd_compress.ko                           >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/crypto/xor.ko                                       >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/fs/btrfs/btrfs.ko                                   >> $QEMU_ROOT/kernel.conf \
    && echo xfs   /lib/modules/*/kernel/lib/libcrc32c.ko                                    >> $QEMU_ROOT/kernel.conf \
    && echo xfs   /lib/modules/*/kernel/fs/xfs/xfs.ko                                       >> $QEMU_ROOT/kernel.conf

RUN cp /boot/vmlinuz-*-generic $QEMU_ROOT/kernel

RUN cp -d --preserve=all --parents \
  /lib/x86_64-linux-gnu/* \
  /usr/lib/x86_64-linux-gnu/* \
  $QEMU_ROOT/ 2>&1 | grep -v 'omitting directory';:

RUN cp -a --parents \
    /bin/busybox \
    /lib64 \
    /lib/modules \
    /usr/bin/qemu-system-x86_64 \
    /usr/lib/ipxe \
    /usr/lib/x86_64-linux-gnu/pulseaudio \
    /usr/lib/x86_64-linux-gnu/qemu  \
    /usr/share/qemu \
    /usr/share/seabios \
    $QEMU_ROOT/

