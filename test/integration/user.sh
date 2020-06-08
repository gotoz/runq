#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="set custom user id"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --user nobody \
    $image  \
    sh -c 'id -u -n | grep -w nobody'

checkrc $? 0 "$comment"

#
#
#
comment="set additional user group"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --user nobody \
    --group-add games \
    $image  \
    sh -c 'id -G -n | grep -w "nobody games"'

checkrc $? 0 "$comment"

myexit
