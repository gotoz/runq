FROM ubuntu:18.04

ENV DEBIAN_FRONTEND noninteractive
ENV GOPATH /go
ENV QEMU_ROOT /var/lib/runq/qemu

WORKDIR /go/src/github.com/gotoz/runq

RUN echo "do_initrd = no" >> /etc/kernel-img.conf \
    && apt-get update \
    && apt-get upgrade \
        --yes \
    && apt-get install \
        --yes \
        --no-install-recommends \
        --no-install-suggests \
        build-essential \
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
    && echo base /lib/modules/*/kernel/fs/fscache/fscache.ko        > $QEMU_ROOT/kernel.conf \
    && echo base /lib/modules/*/kernel/net/9p/9pnet.ko             >> $QEMU_ROOT/kernel.conf \
    && echo base /lib/modules/*/kernel/net/9p/9pnet_virtio.ko      >> $QEMU_ROOT/kernel.conf \
    && echo base /lib/modules/*/kernel/fs/9p/9p.ko                 >> $QEMU_ROOT/kernel.conf \
    && echo base /lib/modules/*/kernel/drivers/block/virtio_blk.ko >> $QEMU_ROOT/kernel.conf \
    && echo base /lib/modules/*/kernel/drivers/net/virtio_net.ko   >> $QEMU_ROOT/kernel.conf \
    && echo xfs  /lib/modules/*/kernel/lib/libcrc32c.ko            >> $QEMU_ROOT/kernel.conf \
    && echo xfs  /lib/modules/*/kernel/fs/xfs/xfs.ko               >> $QEMU_ROOT/kernel.conf

RUN cp /boot/vmlinuz-*-generic $QEMU_ROOT/kernel

RUN cp -a --parents \
    /lib64 \
    /lib/modules \
    /lib/x86_64-linux-gnu \
    /usr/bin/qemu-system-x86_64 \
    /usr/lib/ipxe/qemu \
    /usr/lib/x86_64-linux-gnu \
    /usr/share/qemu \
    /usr/share/seabios \
    $QEMU_ROOT/

