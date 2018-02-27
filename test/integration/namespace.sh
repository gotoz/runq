#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="nsenter mnt"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --cap-add sys_admin \
    $image  \
    nsenter --mount=/proc/self/ns/mnt /bin/hostname

checkrc $? 0 "$comment"

myexit
