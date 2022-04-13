FROM ubuntu:18.04

ENV RUNC_ROOT /runq/runc
ENV RUNQ_ROOT /var/lib/runq
ENV QEMU_ROOT /var/lib/runq/qemu

ENV DEBIAN_FRONTEND noninteractive
ENV GO_URL https://golang.org/dl/go1.17.1.linux-s390x.tar.gz
ENV GO_SHA256 f770b72e1e1281239b12b64825f87a928a1788943c7f09cac7f28985ef2cf692
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
        linux-image-virtual \
        linux-image-extra-virtual \
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

RUN    echo base   /lib/modules/*/kernel/fs/fscache/fscache.ko                               >  $QEMU_ROOT/kernel.conf \
    && echo base   /lib/modules/*/kernel/net/9p/9pnet.ko                                     >> $QEMU_ROOT/kernel.conf \
    && echo base   /lib/modules/*/kernel/net/9p/9pnet_virtio.ko                              >> $QEMU_ROOT/kernel.conf \
    && echo base   /lib/modules/*/kernel/fs/9p/9p.ko                                         >> $QEMU_ROOT/kernel.conf \
    && echo base   /lib/modules/*/kernel/drivers/block/virtio_blk.ko                         >> $QEMU_ROOT/kernel.conf \
    && echo base   /lib/modules/*/kernel/drivers/net/virtio_net.ko                           >> $QEMU_ROOT/kernel.conf \
    && echo vsock  /lib/modules/*/kernel/net/vmw_vsock/vsock.ko                              >> $QEMU_ROOT/kernel.conf \
    && echo vsock  /lib/modules/*/kernel/net/vmw_vsock/vmw_vsock_virtio_transport_common.ko  >> $QEMU_ROOT/kernel.conf \
    && echo vsock  /lib/modules/*/kernel/net/vmw_vsock/vmw_vsock_virtio_transport.ko         >> $QEMU_ROOT/kernel.conf \
    && echo btrfs  /lib/modules/*/kernel/lib/raid6/raid6_pq.ko                               >> $QEMU_ROOT/kernel.conf \
    && echo btrfs  /lib/modules/*/kernel/lib/zlib_deflate/zlib_deflate.ko                    >> $QEMU_ROOT/kernel.conf \
    && echo btrfs  /lib/modules/*/kernel/lib/zstd/zstd_compress.ko                           >> $QEMU_ROOT/kernel.conf \
    && echo btrfs  /lib/modules/*/kernel/crypto/xor.ko                                       >> $QEMU_ROOT/kernel.conf \
    && echo btrfs  /lib/modules/*/kernel/fs/btrfs/btrfs.ko                                   >> $QEMU_ROOT/kernel.conf \
    && echo xfs    /lib/modules/*/kernel/lib/libcrc32c.ko                                    >> $QEMU_ROOT/kernel.conf \
    && echo xfs    /lib/modules/*/kernel/fs/xfs/xfs.ko                                       >> $QEMU_ROOT/kernel.conf \
    && echo zcrypt /lib/modules/*/kernel/drivers/s390/crypto/zcrypt.ko                       >> $QEMU_ROOT/kernel.conf \
    && echo zcrypt /lib/modules/*/kernel/drivers/s390/crypto/zcrypt_cex4.ko                  >> $QEMU_ROOT/kernel.conf \
    && echo z13    /lib/modules/*/kernel/drivers/char/hw_random/virtio-rng.ko                >> $QEMU_ROOT/kernel.conf \
    && echo z14+   /lib/modules/*/kernel/drivers/char/hw_random/s390-trng.ko                 >> $QEMU_ROOT/kernel.conf


ADD extract-vmlinux.sh /extract-vmlinux.sh

RUN /extract-vmlinux.sh /boot/vmlinuz-*-generic $QEMU_ROOT/kernel

RUN cp -d --preserve=all --parents \
    /lib/s390x-linux-gnu/* \
    /usr/lib/s390x-linux-gnu/* \
    $QEMU_ROOT/ 2>&1 | grep -v 'omitting directory';:

RUN cp -a --parents \
    /etc/mtab \
    /lib/ld64.so.1 \
    /lib/modules \
    /sbin/e2fsck \
    /sbin/fsck.ext2 \
    /sbin/fsck.ext4 \
    /sbin/mke2fs \
    /sbin/mkfs.ext2 \
    /sbin/mkfs.ext4 \
    /usr/bin/qemu-system-s390x \
    /usr/bin/rsync \
    /usr/lib/s390x-linux-gnu/pulseaudio \
    /usr/lib/s390x-linux-gnu/qemu \
    /usr/share/qemu \
    $QEMU_ROOT/

RUN    git config --global --add safe.directory /runq \
    && git config --global --add safe.directory /runq/runc
