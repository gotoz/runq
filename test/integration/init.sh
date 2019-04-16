#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="default run is without tini init"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    $image  \
    sh -c "ps -o args | egrep '^/(sbin/docker-init|dev/init)'"
checkrc $? 1 "$comment"

#
#
#
comment="run with tini init"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --init \
    $image  \
    sh -c "ps -o args | egrep '^/(sbin/docker-init|dev/init)'"
checkrc $? 0 "$comment"

myexit
