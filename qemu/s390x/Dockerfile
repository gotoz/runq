FROM ubuntu:18.04

RUN echo "do_initrd = no" >> /etc/kernel-img.conf

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get upgrade \
        --yes \
    && DEBIAN_FRONTEND=noninteractive apt-get install \
       --yes \
       --no-install-recommends \
       --no-install-suggests \
       qemu-kvm linux-virtual tar xz-utils

RUN mkdir -p \
    /qemu/dev \
    /qemu/lib \
    /qemu/proc \
    /qemu/rootfs \
    /qemu/sys

RUN cd /qemu && ln -s s390x-linux-gnu/ld-2.26.so lib/ld64.so.1

ADD extract-vmlinux.sh /extract-vmlinux.sh

RUN /extract-vmlinux.sh boot/vmlinuz-*-generic /qemu/kernel

RUN rm -f /lib/modules/*/build \
    && echo base /lib/modules/*/kernel/fs/fscache/fscache.ko        > /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/net/9p/9pnet.ko             >> /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/net/9p/9pnet_virtio.ko      >> /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/fs/9p/9p.ko                 >> /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/drivers/block/virtio_blk.ko >> /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/drivers/net/virtio_net.ko   >> /qemu/kernel.conf \
    && echo xfs  /lib/modules/*/kernel/lib/libcrc32c.ko            >> /qemu/kernel.conf \
    && echo xfs  /lib/modules/*/kernel/fs/xfs/xfs.ko               >> /qemu/kernel.conf

RUN cd / \
    && tar -cf /qemu.tar \
       lib/modules \
       lib/s390x-linux-gnu \
       usr/bin/qemu-system-s390x \
       usr/lib/s390x-linux-gnu \
       usr/share/qemu \
    && cd /qemu \
    && tar -rf /qemu.tar *

