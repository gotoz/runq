#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

docker info --format {{.Runtimes.runq.Args}} | grep -F -- --noexec
test $? -eq 0 || skip "reason: --noexec is not configured"

runq_exec=/var/lib/runq/runq-exec
name=$(rand_name)

cleanup() {
    docker rm -f $name &>/dev/null
}
trap "cleanup; myexit" 0 2 15

#
#
#
docker run \
    --runtime runq \
    --name $name \
    -dt \
    $image sh

sleep 2

$runq_exec $name true
checkrc $? 1 "runq-exec is blocked globally"

#
#
#
docker run \
    --runtime runq \
    --rm \
    $image sh -c "grep ^vsock /proc/modules"

checkrc $? 1 "vsock kernel module is not loaded"

