#!/bin/bash
DIR=$(cd ${0%/*};pwd;)
. $DIR/../common.sh

runtime="${1:-runq}"

if ! docker info --format '{{json .SecurityOptions}}'| grep -q 'name=seccomp'; then
    skip "reason: Docker daemon does not support seccomp profiles"
fi

default_profile=$DIR/../testdata/seccomp.json
custom_profile=`mktemp`
cleanup() {
    rm -f $custom_profile
    myexit
}
trap "cleanup; myexit" 0 2 15

comment="default profile disallows unshare"
docker run \
    --runtime $runtime \
    --name `rand_name` \
    --rm \
    $image \
    unshare true

checkrc $? 1 "$comment"


cat $default_profile > $custom_profile
comment="custom profile disallows unshare"
docker run \
    --runtime $runtime \
    --name `rand_name` \
    --rm \
    --security-opt seccomp=$custom_profile \
    $image \
    unshare true

checkrc $? 1 "$comment"


cat $default_profile | sed 's/"writev"$/"writev","unshare"/'> $custom_profile
comment="custom profile allows unshare"
docker run \
    --runtime $runtime \
    --name `rand_name` \
    --rm \
    --security-opt seccomp=$custom_profile \
    $image \
    unshare true

checkrc $? 0 "$comment"


comment="seccomp=unconfined allows unshare"
docker run \
    --runtime $runtime \
    --name `rand_name` \
    --rm \
    --security-opt seccomp=unconfined \
    $image \
    unshare true

checkrc $? 0 "$comment"

