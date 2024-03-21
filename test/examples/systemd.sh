#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

name=$(rand_name)

docker build -t systemd -f Dockerfile.systemd .

cleanup() {
    echo cleanup
    docker rm -f $name
    myexit
}
trap cleanup EXIT

( sleep 15; docker stop $name; )&


docker run \
    -e RUNQ_MEM=2048 \
    -e RUNQ_CPU=2 \
    -e RUNQ_SYSTEMD=1 \
    --name $name \
    --restart on-failure:3 \
    --runtime runq \
    --cap-add all \
    --security-opt seccomp=unconfined \
    systemd /usr/bin/systemd

checkrc $? 0 "systemd exit code rc=0"
