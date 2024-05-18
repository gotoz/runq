FROM ubuntu:20.04

ENV RUNC_ROOT /runq/runc
ENV RUNQ_ROOT /var/lib/runq
ENV QEMU_ROOT /var/lib/runq/qemu

ENV DEBIAN_FRONTEND noninteractive
ENV GO_URL https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
ENV GO_SHA256 3f934f40ac360b9c01f616a9aa1796d227d8b0328bf64cb045c7b8c4ee9caea4
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
        qemu-system-x86 \
        rsync \
        wget \
        xz-utils

RUN set -eu; \
    wget -q -O go.tar.gz $GO_URL; \
    echo "$GO_SHA256 go.tar.gz" | sha256sum -c -; \
    tar -C /usr/local -xzf go.tar.gz; \
    rm -f go.tar.gz

RUN mkdir -p \
    $QEMU_ROOT/etc \
    $QEMU_ROOT/dev \
    $QEMU_ROOT/proc \
    $QEMU_ROOT/rootfs \
    $QEMU_ROOT/share \
    $QEMU_ROOT/sys

RUN    echo base  /lib/modules/*/kernel/fs/fscache/fscache.ko                                > $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/net/9p/9pnet.ko                                     >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/fs/9p/9p.ko                                         >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/net/9p/9pnet.ko                                     >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/net/9p/9pnet_virtio.ko                              >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/net/core/failover.ko                                >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/drivers/net/net_failover.ko                         >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/drivers/block/virtio_blk.ko                         >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/drivers/net/virtio_net.ko                           >> $QEMU_ROOT/kernel.conf \
    && echo base  /lib/modules/*/kernel/drivers/char/hw_random/virtio-rng.ko                >> $QEMU_ROOT/kernel.conf \
    && echo vsock /lib/modules/*/kernel/net/vmw_vsock/vsock.ko                              >> $QEMU_ROOT/kernel.conf \
    && echo vsock /lib/modules/*/kernel/net/vmw_vsock/vmw_vsock_virtio_transport_common.ko  >> $QEMU_ROOT/kernel.conf \
    && echo vsock /lib/modules/*/kernel/net/vmw_vsock/vmw_vsock_virtio_transport.ko         >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/lib/libcrc32c.ko                                    >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/lib/raid6/raid6_pq.ko                               >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/lib/zstd/zstd_compress.ko                           >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/crypto/xor.ko                                       >> $QEMU_ROOT/kernel.conf \
    && echo btrfs /lib/modules/*/kernel/fs/btrfs/btrfs.ko                                   >> $QEMU_ROOT/kernel.conf \
    && echo xfs   /lib/modules/*/kernel/lib/libcrc32c.ko                                    >> $QEMU_ROOT/kernel.conf \
    && echo xfs   /lib/modules/*/kernel/fs/xfs/xfs.ko                                       >> $QEMU_ROOT/kernel.conf

RUN cp /boot/vmlinuz-*-generic $QEMU_ROOT/kernel

RUN cp -d --preserve=all --parents \
  /usr/lib/x86_64-linux-gnu/* \
  $QEMU_ROOT/ 2>&1 | grep -v 'omitting directory';:

 RUN cp -a --parents \
     /usr/bin/qemu-system-x86_64 \
     /usr/bin/rsync \
     /usr/lib64/ld-linux-x86-64.so.2 \
     /usr/lib/ipxe/qemu \
     /usr/lib/modules \
     /usr/lib/x86_64-linux-gnu/ceph \
     /usr/lib/x86_64-linux-gnu/qemu \
     /usr/sbin/e2fsck \
     /usr/sbin/fsck.ext2 \
     /usr/sbin/fsck.ext4 \
     /usr/sbin/mke2fs \
     /usr/sbin/mkfs.ext2 \
     /usr/sbin/mkfs.ext4 \
     /usr/share/qemu \
     /usr/share/seabios \
     $QEMU_ROOT/

RUN cd $QEMU_ROOT \
    && ln -s /proc/mounts etc/mtab \
    && ln -s usr/bin bin \
    && ln -s usr/lib64 lib64 \
    && ln -s usr/lib lib \
    && ln -s usr/sbin sbin

RUN    git config --global --add safe.directory /runq \
    && git config --global --add safe.directory /runq/runc
