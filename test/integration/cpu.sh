#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="use default number of cpu"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    $image  \
    sh -c 'cpu=$(getconf _NPROCESSORS_ONLN); echo cpu=$cpu; exit $cpu;'

checkrc $? 1 "$comment"

#
#
#
comment="set custom number of cpu"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -e RUNQ_CPU=3 \
    $image  \
    sh -c 'cpu=$(getconf _NPROCESSORS_ONLN); echo cpu=$cpu; exit $cpu;'

checkrc $? 3 "$comment"

myexit
