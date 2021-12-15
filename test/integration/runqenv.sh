#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

docker info --format {{.Runtimes.runq.Args}} | grep -q -F -- --runqenv
GLOBAL_RUNQENV=$?

#
# runqenv via global config in daemon.json
#
comment="runqenv via global config"
if [ $GLOBAL_RUNQENV -eq 0 ]; then
    cmd="unset RC;source /.runqenv && exit \$RC"
    docker run \
        --runtime runq \
        --rm \
        -e RUNQ_RUNQENV=1 \
        -e RC=42 \
        $image \
          sh -c "$cmd"

    checkrc $? 42 "$comment"
else
   skip_msg "$comment" "reason: --runqenv is not configured"
fi

#
# runqenv via env variable
#
comment="runqenv via env variable"
cmd="unset RC;source /.runqenv && exit \$RC"
docker run \
    --runtime runq \
    --rm \
    -e RUNQ_RUNQENV=1 \
    -e RC=42 \
    $image \
      sh -c "$cmd"

checkrc $? 42 "$comment"

#
# /.runqenv permissions
#
comment="/.runqenv file permissions"
cmd="stat -c '%a%U%G' /.runqenv | grep 400nobodynobody"

docker run \
    --runtime runq \
    --rm \
    -e RUNQ_RUNQENV=1 \
    -u nobody:nobody \
    $image \
      sh -c "$cmd"

checkrc $? 0 "$comment"

myexit
