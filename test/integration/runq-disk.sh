#!/bin/bash
DIR=$(cd ${0%/*};pwd;)
. $DIR/../common.sh

set -u

# tempdir must not be of type tmpfs
tempdir=$DIR/temp$$
mkdir -p $tempdir

cleanup() {
    rm -rf $tempdir
}

trap "cleanup; myexit" 0 2 15

#
#
#
set -e
name=$(rand_name)
mnt1=/mnt/data1
mnt2=/mnt/data2
mkdir -p ${tempdir}${mnt1}
mkdir -p ${tempdir}${mnt2}
file1=${tempdir}${mnt1}/file.img
file2=${tempdir}${mnt2}/file.img
truncate -s 128M $file1
truncate -s 128M $file2
mkfs.ext2 -F $file1 &>/dev/null
mkfs.ext4 -F $file2 &>/dev/null

comment="use existing disk images"
cmd="set -e"
cmd="$cmd; df -hT | grep ^/dev/vd"
cmd="$cmd; dd if=/dev/zero of=$mnt1/testfile bs=1M count=10"
cmd="$cmd; dd if=/dev/zero of=$mnt2/testfile bs=1M count=10"
cmd="$cmd; exit \$?"

set +e
docker run \
    --runtime runq \
    --name $name \
    -v $tempdir/mnt:/mnt \
    -e RUNQ_DISK="img=$mnt1/file.img,dir=$mnt1,fstype=ext2;img=$mnt2/file.img,dir=$mnt2" \
    --rm \
    -ti \
    $image \
    sh -c "$cmd"

checkrc $? 0 "$comment"

comment="resize existing disk images"
cmd="set -e"
cmd="$cmd; df -hT | grep ^/dev/vd"
cmd="$cmd; dd if=/dev/zero of=$mnt1/testfile bs=1M count=50"
cmd="$cmd; dd if=/dev/zero of=$mnt2/testfile bs=1M count=50"
cmd="$cmd; exit \$?"

docker run \
    --runtime runq \
    --name $name \
    -v $tempdir/mnt:/mnt \
    -e RUNQ_DISK="img=$mnt1/file.img,dir=$mnt1,fstype=ext2,size=128M;img=$mnt2/file.img,dir=$mnt2,size=128M" \
    --rm \
    $image \
    sh -c "$cmd"

checkrc $? 0 "$comment"

#
#
#
comment="create new disk images ext2,ext4"
mnt1=/a
mnt2=/b
name=$(rand_name)
script=$(mktemp)
cat <<EOF > $script
#!/bin/sh
ls -l /dev/disk/by-runq-id/*
df -hT | grep ^/dev/vd
if [ -f $mnt1/testfile.md5 ]; then
    set -o pipefail
    set -e
    md5sum -c $mnt1/testfile.md5
    md5sum -c $mnt1/testfile.md5 2>&1 | grep ': OK' | wc -l | xargs test 2 -eq
else
    set -e
    dd if=/dev/urandom of=$mnt1/testfile bs=1M count=10
    dd if=/dev/urandom of=$mnt2/testfile bs=1M count=10
    md5sum $mnt1/testfile $mnt2/testfile | tee $mnt1/testfile.md5
fi
EOF
chmod 755 $script

RUNQ_DISK="cache=writethrough,fstype=ext2,dir=$mnt1,id=ext2,label=ext2"
RUNQ_DISK="$RUNQ_DISK;fstype=ext4,dir=$mnt2,id=ext4,label=ext4"
docker run \
    --runtime runq \
    --name $name \
    -v $script:/run \
    -e RUNQ_DISK=$RUNQ_DISK \
    $image /run

checkrc $? 0 "$comment"

comment="restart container"
docker start -a $name
checkrc $? 0 "$comment"
docker rm -f $name
rm -f $script

#
#
#
comment="mount options"
cmd="grep '/mnt ext4 ro,nosuid' /proc/mounts"

docker run \
    --runtime runq \
    --name $name \
    -v $tempdir/mnt:/mnt \
    -e RUNQ_DISK="dir=/mnt,options=ro+nosuid+mode=1700" \
    --rm \
    $image \
    sh -c "$cmd"

checkrc $? 0 "$comment"

