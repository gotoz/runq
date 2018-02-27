#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="default is net.ipv4.ip_forward=0"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    $image \
    sh -c "grep ^0$ /proc/sys/net/ipv4/ip_forward"

checkrc $? 0 "$comment"

comment="set net.ipv4.ip_forward=1"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -e RUNQ_CPU=3 \
    --sysctl net.ipv4.ip_forward=1 \
    $image \
    sh -c "grep ^1$ /proc/sys/net/ipv4/ip_forward"

checkrc $? 0 "$comment"

myexit
