#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="set multiple tmpfs with subdir"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --tmpfs /mytmp \
    --tmpfs /tmp/tmp \
    $image  \
    sh -c 'rc=$(df | egrep /mytmp$\|/tmp/tmp$ | grep -c ^tmpfs); exit $rc'

checkrc $? 2 "$comment"

#
#
#
comment="set tmpfs with custom arguments"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --tmpfs /tmp/tmp:size=5M,noatime,noexec,nodev \
    $image  \
    sh -c 'grep -q "tmpfs /tmp/tmp tmpfs rw,nosuid,nodev,noexec,noatime,size=5120k" /proc/mounts'

checkrc $? 0 "$comment"

myexit
