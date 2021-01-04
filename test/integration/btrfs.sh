#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

set -x
test $UID -eq 0 || skip "reason: not running as root"
command -v qemu-img || skip "reason: qemu-img not found"
command -v qemu-nbd || skip "reason: qemu-nbd not found"
command -v mkfs.btrfs || skip "reason: mkfs.btrfs not found"

modprobe nbd &>/dev/null
modprobe btrfs &>/dev/null

grep -q -w btrfs /proc/filesystems || skip "reason: host does not support BTRFS"
dev1=/dev/nbd0
dev2=/tmp/file-$$

qcow1=/tmp/qcow1-$$

mnt1=/a
mnt2=/b

test -e $dev1 || skip "reason: $dev1 not available"

set -u

cleanup() {
    qemu-nbd -d $dev1
    rm -f $qcow1
    rm -f $dev2
}
trap "cleanup; myexit" EXIT


qemu-img create -f qcow2 $qcow1 200m >/dev/null
qemu-nbd -d $dev1
sleep 1
qemu-nbd -c $dev1 $qcow1

mkfs.btrfs -f $dev1
dd if=/dev/zero of=$dev2 bs=1M count=200
mkfs.btrfs -f $dev2

qemu-nbd -d $dev1

comment="create and mount qcow2, raw file and block device"
cmd="set -e"
cmd="$cmd;   dd if=/dev/urandom of=$mnt1/testfile bs=1M count=10"
cmd="$cmd && dd if=/dev/urandom of=$mnt2/testfile bs=1M count=10"
cmd="$cmd && md5sum $mnt1/testfile $mnt2/testfile > $mnt1/testfile.md5"
cmd="$cmd; exit \$?"

docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -v $qcow1:/dev/runq/$(uuid)/writeback/btrfs/$mnt1 \
    -v $dev2:/dev/runq/$(uuid)/unsafe/btrfs/$mnt2 \
    $image \
    sh -c "$cmd"

checkrc $? 0 "$comment"

comment="re-mount and verify qcow2, raw file and block device"
cmd="cat $mnt1/testfile.md5"
cmd="$cmd && md5sum -c $mnt1/testfile.md5"
cmd="$cmd && set -x; md5sum -c $mnt1/testfile.md5 2>&1 | grep ': OK' | wc -l | xargs test 2 -eq "
exit

#
#
#
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -v $qcow1:/dev/runq/$(uuid)/writeback/btrfs/$mnt1 \
    -v $dev2:/dev/runq/$(uuid)/unsafe/btrfs/$mnt2 \
    $image \
    sh -c "$cmd"

checkrc $? 0 "$comment"


