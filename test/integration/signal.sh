#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

name="$(rand_name)"
tmp_dir=$(mktemp -d)
docker run \
    --runtime runq \
    --rm \
    --name $name \
    -v $tmp_dir:/mnt \
    -td \
    $image  \
    sh -c "trap 'touch /mnt/SIGTERM' SIGTERM;trap 'touch /mnt/SIGINT; sync;exit;' SIGINT; while :;do sleep .1; done"

sleep 2
docker kill --signal SIGTERM $name
sleep 1
docker kill --signal SIGINT $name
sleep 1

stat $tmp_dir/SIGTERM >/dev/null
checkrc $? 0 "forward SIGTERM"

stat $tmp_dir/SIGINT >/dev/null
checkrc $? 0 "forward SIGINT"

rm -rf $tmp_dir

myexit
