#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

set -u

dev=/tmp/file-$$
mnt=/mnt

cleanup() {
    rm -f $dev
}

trap "cleanup; myexit" 0 2 15

for fs in ext2 ext3 ext4 xfs; do
    rm -f $dev
    case "${fs:0:3}" in
        ext)
            dd if=/dev/zero of=$dev bs=1M count=100 >/dev/null
            mkfs.$fs -F $dev
            ;;
        xfs)
            mkfs.xfs -dfile,name=$dev,size=100m
            ;;
    esac

    comment="mount $fs"
    cmd="df -T | awk '/\/dev\/vda/{ print \$2 }' | grep -w $fs"

    docker run \
        --runtime runq \
        --name $(rand_name) \
        --rm \
        -v $dev:/dev/disk/writeback/$fs/$mnt \
        $image \
        sh -c "$cmd"

    checkrc $? 0 "$comment"
done

#
# FS = none
#
comment="attache disk without FS (none)"
cmd="ls -l /dev/vda"

dd if=/dev/zero of=$dev bs=1M count=100 >/dev/null

docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -v $dev:/dev/disk/writeback/none/0001 \
    $image \
    sh -c "$cmd"

checkrc $? 0 "$comment"

#
# unsupported filesystem
#
comment="unsupported filesystem"
cmd="ls -l /dev/vda"

dd if=/dev/zero of=$dev bs=1M count=100 >/dev/null

docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -v $dev:/dev/disk/writeback/btrfs/mnt \
    $image \
    sh -c "$cmd"

checkrc $? 1 "$comment"

