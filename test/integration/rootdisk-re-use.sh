#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

name=$(rand_name)
file=$PWD/file-$$

cleanup() {
   docker rm -f $name 2>/dev/null
   rm -f $file
   myexit
}
trap cleanup EXIT

dd if=/dev/zero of=$file bs=1M count=100 >/dev/null
mkfs.ext2 -F $file

hostname=foo
cmd="hostname | grep -w $hostname && grep $hostname /etc/hosts && grep $hostname /etc/hostname"
docker run \
    --runtime runq \
    --rm \
    --name $name \
    --hostname $hostname \
    --volume $file:/dev/runq/0001/none/ext2 \
    -e RUNQ_ROOTDISK=0001 \
    $image sh -c "$cmd"

checkrc $? 0 "set custom hostname"

hostname=bar
cmd="hostname | grep -w $hostname && grep $hostname /etc/hosts && grep $hostname /etc/hostname"
docker run \
    --runtime runq \
    --rm \
    --name $name \
    --hostname $hostname \
    --volume $file:/dev/runq/0001/none/ext2 \
    -e RUNQ_ROOTDISK=0001 \
    $image sh -c "$cmd"

checkrc $? 0 "hostname has changed"
