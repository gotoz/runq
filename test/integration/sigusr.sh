#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

cmd1=/var/lib/runq/qemu/.runq/SIGUSR1
cmd2=/var/lib/runq/qemu/.runq/SIGUSR2

test -x $cmd1 || skip "reason: $cmd1 not found"
test -x $cmd2 || skip "reason: $cmd2 not found"

tmp_dir=`mktemp -d`
name=`rand_name`
docker run \
    --runtime runq \
    --rm \
    --name $name \
    -v $tmp_dir:/mnt \
    -d \
    $image  \
    sleep 20

sleep 2
docker kill --signal SIGUSR1 $name
docker kill --signal SIGUSR2 $name
sleep 1
docker rm -f $name

stat $tmp_dir/SIGUSR1 >/dev/null
checkrc $? 0 "execute script on SIGUSR1"

stat $tmp_dir/SIGUSR2 >/dev/null
checkrc $? 0 "execute script on SIGUSR2"

rm -rf $tmp_dir

myexit
