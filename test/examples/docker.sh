#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

port=$(rand_port)
name=$(rand_name)
disk=/tmp/disk$$
dd if=/dev/zero of=$disk bs=1M count=100 >/dev/null
mkfs.ext4 -F $disk

cleanup() {
    echo cleanup
    docker rm -f $name
    rm -f $disk
    myexit
}
trap cleanup 0 2 15

comment="Docker"
docker run \
    --runtime runq \
    -e RUNQ_CPU=2 \
    -e RUNQ_MEM=1024 \
    -p $port:2375 \
    --name $name \
    -d \
    --volume $disk:/dev/disk/writethrough/ext4/docker \
    --security-opt seccomp=unconfined \
    --cap-add net_admin \
    --cap-add sys_admin \
    --cap-add sys_module \
    --cap-add sys_resource \
    docker:stable-dind \
    dockerd \
        --data-root /docker \
        -s overlay2 \
        --host=tcp://0.0.0.0:2375

# wait for dind to show up
for ((i=1;i<20;i++)); do
    sleep .5
    if docker -H tcp://localhost:$port ps &>/dev/null; then
        break
    fi
done

docker -H tcp://localhost:$port run alpine env
checkrc $? 0 "$comment"

