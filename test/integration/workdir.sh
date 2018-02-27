#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="set custom workdir"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --workdir /foobar \
    $image  \
    sh -c 'pwd | grep -w /foobar'

checkrc $? 0 "$comment"

myexit
