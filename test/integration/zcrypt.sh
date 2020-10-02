#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh
test "$(uname -m)" = "s390x" || skip "reason: machine type not s390x"

uuids=""
for p in $(ls -d /sys/devices/vfio_ap/matrix/????????-????-????-????-????????????); do
    uuids="$uuids $(basename $p)"
done

test -z "$uuids" && skip "reason: no mediated device found"

rc_all=0
for uuid in $uuids; do
    docker run \
        --runtime runq \
        --name $(rand_name) \
        --rm \
        -e RUNQ_APUUID=$uuid \
        $image \
            sh -c 't="`cat /sys/devices/ap/card*/hwtype`" && test "$t" -gt 10'
    rc=$?
    checkrc $rc 0 "mediated device: $uuid"
    test $rc -ne 0 && ((rc_all++))

done

checkrc 0 $rc_all "crypto device passthrough"

myexit
