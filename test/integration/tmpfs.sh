#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="default runtime tmpfs directories"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    $image  \
    sh -c 'rc=$(df | egrep /tmp$\|/var/tmp$ | grep -c ^tmpfs); exit $rc'

checkrc $? 2 "$comment"

comment="set multiple tmpfs with subdir"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --tmpfs /mytmp \
    --tmpfs /mnt/tmp \
    $image  \
    sh -c 'rc=$(df | egrep /mytmp$\|/mnt/tmp$ | grep -c ^tmpfs); exit $rc'

checkrc $? 2 "$comment"

#
#
#
comment="set tmpfs with custom arguments (1)"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --tmpfs /tmp2:size=5M,noatime,noexec,nodev \
    $image  \
    sh -c 'grep -q "tmpfs /tmp2 tmpfs rw,nosuid,nodev,noexec,noatime,size=5120k" /proc/mounts'

checkrc $? 0 "$comment"

#
#
#
comment="set tmpfs with custom arguments (2)"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --tmpfs /tmp2:ro,dev,exec,suid,strictatime \
    $image  \
    sh -c 'grep -q "tmpfs /tmp2 tmpfs ro" /proc/mounts'

checkrc $? 0 "$comment"

myexit

