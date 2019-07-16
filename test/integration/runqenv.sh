#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

comment="/.runqenv contains container env variables"
cmd="source /.runqenv && exit \$RC"
docker run \
    --runtime runq \
    --rm \
    -e RUNQ_RUNQENV=1 \
    -e A= \
    -e B=" " \
    -e C="\" \"" \
    -e RC=42 \
    -e D=\;exit \
    -e E=\;exit\;\" \
    -e F="\"&&exit" \
    -e G=\`exit\` \
    -e H='`exit`' \
    -e I="\`exit\`" \
    -e J=世界 \
    $image \
      sh -c "$cmd"

checkrc $? 42 "$comment"

