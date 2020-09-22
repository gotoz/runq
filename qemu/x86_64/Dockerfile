FROM ubuntu:18.04

ENV RUNC_ROOT /runq/runc
ENV RUNQ_ROOT /var/lib/runq
ENV QEMU_ROOT /var/lib/runq/qemu

ENV DEBIAN_FRONTEND noninteractive
ENV GO_URL https://golang.org/dl/go1.15.2.linux-amd64.tar.gz
ENV GO_SHA256 b49fda1ca29a1946d6bb2a5a6982cf07ccd2aba849289508ee0f9918f6bb4552
ENV PATH /usr/local/go/bin:$PATH

WORKDIR /runq

RUN echo "do_initrd = no" >> /etc/kernel-img.conf \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        cpio \
        e2fsprogs \
        git \
        libseccomp-dev \
        linux-virtual \
        pkg-config \
        qemu-kvm \
        rsync \
        wget \
        xz-utils

RUN set -eu; \
    wget -q -O go.tar.gz $GO_URL; \
    echo "$GO_SHA256 go.tar.gz" | sha256sum -c -; \
    tar -C /usr/local -xzf go.tar.gz; \
    rm -f go.tar.gz

RUN mkdir -p \
    $QEMU_ROOT/dev \
    $QEMU_ROOT/proc \
    $QEMU_ROOT/lib \
    $QEMU_ROOT/rootfs \
    $QEMU_ROOT/share \
    $QEMU_ROOT/sys

RUN    echo base  /lib/modules/*/kernel/fs/fscache/fscache.ko                                > $QEMU_ROOT/kernel.conf \
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
    /etc/mtab \
    /lib64 \
    /lib/modules \
    /sbin/e2fsck \
    /sbin/fsck.ext2 \
    /sbin/fsck.ext4 \
    /sbin/mke2fs \
    /sbin/mkfs.ext2 \
    /sbin/mkfs.ext4 \
    /usr/bin/qemu-system-x86_64 \
    /usr/bin/rsync \
    /usr/lib/ipxe \
    /usr/lib/x86_64-linux-gnu/pulseaudio \
    /usr/lib/x86_64-linux-gnu/qemu  \
    /usr/share/qemu \
    /usr/share/seabios \
    $QEMU_ROOT/

