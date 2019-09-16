#!/bin/bash
# This is an example of the new "rootdisk" feature.
# We use a simple Docker image based on Ubuntu with systemd, ssh and Docker
# to submit some workload.
# A raw disk file is used as block device. In a real use case one would use
# a regular block device such as /dev/sdc1.
#
# 1. build the Docker image
#    docker build -t rootdisk -f Dockerfile.rootdisk .
# 2. create a runq container
#    sh ./rootdisk.sh
# 3. in a second terminal: run a second level docker container
#    docker -H tcp://localhost:3333 run alpine env

set -u
disk=$PWD/disk-$$

dd if=/dev/zero of=$disk bs=1M count=1k
mkfs.ext4 -F $disk

docker run \
  --name rootdisk \
  -e RUNQ_CPU=2 \
  -e RUNQ_MEM=2048 \
  -e RUNQ_SYSTEMD=1 \
  -e RUNQ_ROOTDISK=0001 \
  -e RUNQ_ROOTDISK_EXCLUDE="/etc/periodic" \
  --restart on-failure:3 \
  --runtime runq \
  --cap-add all \
  --security-opt seccomp=unconfined \
  --volume $disk:/dev/runq/0001/none/ext4 \
  -p 2222:22 \
  -p 3333:2375 \
  -ti rootdisk /sbin/init

# docker -H tcp://localhost:3333 run alpine env
# ssh -p 2222 root@localhost

