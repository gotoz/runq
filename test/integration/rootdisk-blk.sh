#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

test $UID -eq 0 || skip "reason: not running as root"

name=$(rand_name)
dev=/dev/ram0

cleanup() {
   docker rm -f $name 2>/dev/null
   myexit
}
trap cleanup 0 2 15

modprobe brd &>/dev/null

if [ ! -e $dev ]; then
    skip "$dev is not available"
fi

set -u

mkfs.ext2 -F $dev

docker run \
    --runtime runq \
    --name $name \
    --init \
    --device $dev:/dev/runq/0001/none/ext2 \
    -e RUNQ_ROOTDISK=0001 \
    -e RUNQ_ROOTDISK_EXCLUDE="/media" \
    -d \
    $image sleep 100

sleep 2
$runq_exec $name sh -c "grep '^/dev/vda / ext2' /proc/mounts"
checkrc $? 0  "rootfs is on block device"

$runq_exec $name sh -c "ls -d /media"
checkrc $? 1 "directory has been excluded"

$runq_exec $name sh -c "echo foobar > /etc/passwd"
checkrc $? 0 "update file"

$runq_exec $name sh -c "grep foobar /etc/passwd"
checkrc $? 0 "updated file is correct"

docker stop $name
checkrc $? 0 "container has been stopped"

docker start $name
checkrc $? 0 "container has been re-started"
sleep 2

$runq_exec $name sh -c "grep foobar /etc/passwd"
checkrc 0 0 "content of /etc/passwd is correct"

