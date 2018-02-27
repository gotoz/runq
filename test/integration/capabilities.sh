#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

tmpfileA=$(mktemp)
tmpfileB=$(mktemp)
cleanup() {
    rm -f $tmpfileA
    rm -f $tmpfileB
}
trap "cleanup; myexit" 0 2 15

comment="insmod is forbidden without extra caps"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -e RUNQ_CPU=2 \
   $image \
   sh -c "modprobe xfs"

checkrc $? 1 "$comment"

#
#
#
comment="insmod is allowd via extra cap 'sys_module'"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --cap-add sys_module \
    -e RUNQ_CPU=2 \
    $image \
        sh -c "modprobe xfs"

checkrc $? 0 "$comment"

#
#
#
comment="capture capabilities from runc"
docker run \
    --runtime runc \
    --name $(rand_name) \
    --rm \
    -e RUNQ_CPU=2 \
    -v $tmpfileA:/results \
    --cap-add sys_time \
    --cap-add sys_admin \
    $image \
    sh -c 'grep ^Cap /proc/$$/status >/results'

checkrc $? 0 "$comment"

#
#
# Note: sys_time is already defined in /etc/docker/daemon.json
comment="capture capabilities from runq"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    -e RUNQ_CPU=2 \
    -v $tmpfileB:/results \
    --cap-add sys_time \
    --cap-add sys_admin \
    $image \
    sh -c 'grep ^Cap /proc/$$/status >/results'

checkrc $? 0 "$comment"

#
#
#
comment="runc and runq drop same capabilities"
diff $tmpfileA $tmpfileB
checkrc $? 0 "$comment"

