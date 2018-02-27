#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

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
checkrc $? 0 "execute script on SIGUSR1 (requires testdata install)"

stat $tmp_dir/SIGUSR2 >/dev/null
checkrc $? 0 "execute script on SIGUSR2 (requires testdata install)"

rm -rf $tmp_dir

myexit
