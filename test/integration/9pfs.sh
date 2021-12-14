#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="default 9p cache mode is 'mmap'"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    $image  \
    sh -c "grep '^rootfs.*,mmap' /proc/mounts"

checkrc $? 0 "$comment"

comment="set custom 9p cache mode"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -e RUNQ_9PCACHE=fscache \
    $image  \
    sh -c "grep '^rootfs.*,fscache' /proc/mounts"

checkrc $? 0 "$comment"

myexit
