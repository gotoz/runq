FROM ubuntu:18.04

RUN echo "do_initrd = no" >> /etc/kernel-img.conf

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get upgrade \
        --yes \
    && DEBIAN_FRONTEND=noninteractive apt-get install \
        --yes \
        --no-install-recommends \
        --no-install-suggests \
        qemu-kvm linux-virtual tar

RUN mkdir -p \
    /qemu/dev \
    /qemu/proc \
    /qemu/rootfs \
    /qemu/sys

RUN cp boot/vmlinuz-*-generic /qemu/kernel \
    && rm -f /lib/modules/*/build \
    && echo base /lib/modules/*/kernel/fs/fscache/fscache.ko        > /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/net/9p/9pnet.ko             >> /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/net/9p/9pnet_virtio.ko      >> /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/fs/9p/9p.ko                 >> /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/drivers/block/virtio_blk.ko >> /qemu/kernel.conf \
    && echo base /lib/modules/*/kernel/drivers/net/virtio_net.ko   >> /qemu/kernel.conf \
    && echo xfs  /lib/modules/*/kernel/lib/libcrc32c.ko            >> /qemu/kernel.conf \
    && echo xfs  /lib/modules/*/kernel/fs/xfs/xfs.ko               >> /qemu/kernel.conf

RUN cd /qemu \
    && tar -cf /qemu.tar \
       /lib64 \
       /lib/modules \
       /lib/x86_64-linux-gnu \
       /usr/bin/qemu-system-x86_64 \
       /usr/lib/ipxe/qemu \
       /usr/lib/x86_64-linux-gnu \
       /usr/share/qemu \
       /usr/share/seabios \
       *

