#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="set custom hostname"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --hostname foobar \
    $image  \
    sh -c 'hostname | grep -w foobar'

checkrc $? 0 "$comment"

myexit
