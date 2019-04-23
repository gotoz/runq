#!/bin/bash
# This is an example to test the "rootdisk" feature.
# We use a simple Docker image based on Ubuntu with systemd, ssh and Docker.
# /dev/ram0 will be used as block device. In a real use case one would use
# a regular block device such as /dev/sdc1. The size of the block device
# must be at least 1 GB.
#
# 1. build the Docker image
#    docker build -t rootdisk -f Dockerfile.rootdisk .
# 2. create a runq container
#    sh ./rootdisk.sh
# 3. in a second terminal: run a second level docker container
#    docker -H tcp://localhost:3333 run alpine env

if [ $(id -u) -ne 0 ]; then
    echo "must run as root"
    exit 1
fi

set -u
image=rootdisk
disk=/dev/ram0

# make sure the size of a ramdisk is at least 1GB
grep ^brd /proc/modules || sudo modprobe brd rd_size=1048576

mkfs.ext4 -F $disk

docker run \
  --name rootdisk \
  -e RUNQ_CPU=2 \
  -e RUNQ_MEM=2048 \
  -e RUNQ_ROOTDISK=0001 \
  -e RUNQ_ROOTDISK_EXCLUDE="/etc/periodic" \
  --restart on-failure:3 \
  --runtime runq \
  --cap-add all \
  --security-opt seccomp=unconfined \
  --device $disk:/dev/runq/0001/none/ext4 \
  -p 2222:22 \
  -p 3333:2375 \
  -ti $image /sbin/init

# docker -H tcp://localhost:3333 run alpine env
# ssh -p 2222 root@localhost

