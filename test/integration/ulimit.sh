#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="default ulimit nofile"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    $image  \
    sh -c "grep '^Max open files[ ]*65536[ ]*65536[ ]*file' /proc/self/limits"
checkrc $? 0 "$comment"

#
#
#
comment="set custom ulimit nofile"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --ulimit nofile=999 \
    $image  \
    sh -c "grep '^Max open files[ ]*999[ ]*999[ ]*file' /proc/self/limits"

checkrc $? 0 "$comment"

#
#
#
comment="set custom ulimit core,cpu,nofile,nproc"
cmd="set -e; grep '^Max cpu time[ ]*999[ ]*1000[ ]*seconds' /proc/self/limits"
cmd="$cmd; grep '^Max core file size[ ]*999[ ]*1000[ ]*bytes' /proc/self/limits"
cmd="$cmd; grep '^Max processes[ ]*999[ ]*1000[ ]*processes' /proc/self/limits"
cmd="$cmd; grep '^Max open files[ ]*999[ ]*1000[ ]*file' /proc/self/limits"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --ulimit core=999:1000 \
    --ulimit cpu=999:1000 \
    --ulimit nofile=999:1000 \
    --ulimit nproc=999:1000 \
    $image sh -c "$cmd"

checkrc $? 0 "$comment"

myexit
