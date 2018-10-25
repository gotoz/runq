#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

test $UID -eq 0 || skip "reason: not running as root"

modprobe brd &>/dev/null

dev0=/dev/ram0
dev1=/dev/ram1
dev2=$(mktemp)
dev3=$(mktemp)

test -e $dev0 || skip "reason: $dev0 not available"
test -e $dev1 || skip "reason: $dev1 not available"

set -u

cleanup() {
    rm -f $dev2 $dev3
    test -b $dev0 && dd if=/dev/zero of=$dev0 bs=1M >/dev/null 2>&1
    test -b $dev1 && dd if=/dev/zero of=$dev1 bs=1M >/dev/null 2>&1
}
trap "cleanup; myexit" 0 2 15

mkfs.ext2 -F $dev0
mkfs.ext3 -F $dev1
truncate -s 32M $dev2
mkfs.ext4 -F $dev2
mkfs.xfs -dfile,name=$dev3,size=32m

comment="combine old/new syntax"
cmd='set -x; set -e; for f in ext2 ext3 ext4 xfs; do grep "/$f $f" /proc/mounts; done'
cmd="$cmd; ls -l /dev/disk/by-runq-id/ext3 /dev/disk/by-runq-id/xfs"
docker run \
    --runtime runq \
    --rm \
    --device "$dev0:/dev/disk/writethrough/ext2/ext2" \
    --device "$dev1:/dev/runq/ext3/writethrough/ext3/ext3" \
    --volume "$dev2:/dev/disk/writethrough/ext4/ext4" \
    --volume "$dev3:/dev/runq/xfs/writethrough/xfs/xfs" \
    -ti \
    alpine sh -c "$cmd"

checkrc $? 0 "$comment"
