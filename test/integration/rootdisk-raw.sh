#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

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

set -u

docker run \
    --runtime runq \
    --name $name \
    --init \
    --volume $file:/dev/runq/0001/none/ext4 \
    -e RUNQ_ROOTDISK=0001 \
    -e RUNQ_ROOTDISK_EXCLUDE="/media" \
    -d \
    $image sleep 100

sleep 2
/var/lib/runq/runq-exec $name sh -c "grep '^/dev/vda / ext4' /proc/mounts"
checkrc $? 0  "rootfs is on block device"

/var/lib/runq/runq-exec $name sh -c "ls -d /media"
checkrc $? 1 "directory has been excluded"

/var/lib/runq/runq-exec $name sh -c "echo foobar > /etc/passwd"
checkrc $? 0 "update file"

/var/lib/runq/runq-exec $name sh -c "grep foobar /etc/passwd"
checkrc $? 0 "updated file is correct"

docker stop $name
checkrc $? 0 "container has been stopped"

docker start $name
checkrc $? 0 "container has been re-started"
sleep 2

/var/lib/runq/runq-exec $name sh -c "grep foobar /etc/passwd"
checkrc 0 0 "content of /etc/passwd is correct"

