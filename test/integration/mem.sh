#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="use default memory size (256MiB)"
min=200000
max=260000
cmd="mem=\$(awk '/MemTotal/{print \$2}' /proc/meminfo); echo mem=\$mem; test \$mem -gt $min -a \$mem -lt $max"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    $image  \
    sh -c "$cmd"

checkrc $? 0 "$comment"

#
#
#
runq_mem=512
comment="set custom memory size (${runq_mem}MiB)"
min=450000
max=510000
cmd="mem=\$(awk '/MemTotal/{print \$2}' /proc/meminfo); echo mem=\$mem; test \$mem -gt $min -a \$mem -lt $max"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -e RUNQ_MEM=$runq_mem \
    $image  \
    sh -c "$cmd"

checkrc $? 0 "$comment"

myexit
