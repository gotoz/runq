#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

set -u

comment="rootfs is not writable"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --read-only \
    --rm \
    $image  \
    sh -c "touch /test 2>&1 | grep -q 'Read-only file system'"

checkrc $? 0 "$comment"

comment="rootfs is read-only"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --read-only \
    --rm \
    $image  \
    awk '$2=="/" { exit(!match($4, "(^|,)ro(,|$)")) }' /proc/mounts

checkrc $? 0 "$comment"

name=$(rand_name)
file=$PWD/file-$$

cleanup() {
   docker rm -f $name 2>/dev/null
   rm -f $file
   myexit
}
trap cleanup 0 2 15

dd if=/dev/zero of=$file bs=1M count=100 >/dev/null
mkfs.ext4 -F $file

docker run \
    --runtime runq \
    --name $name \
    --init \
    --volume $file:/dev/runq/0001/none/ext4 \
    -e RUNQ_ROOTDISK=0001 \
    --read-only \
    -d \
    $image sleep 100

sleep 2

$runq_exec $name sh -c "touch /test 2>&1 | grep -q 'Read-only file system'"
checkrc $? 0  "rootfs on block device is not writable"

$runq_exec $name awk '$2=="/" { exit(!match($4, "(^|,)ro(,|$)")) }' /proc/mounts
checkrc $? 0  "rootfs on block device is read-only"

myexit
