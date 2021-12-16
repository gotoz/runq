#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

set -o pipefail

name1=$(rand_name)
name2=$(rand_name)
cleanup() {
    docker rm -f $name1 $name2 &>/dev/null
}
trap "cleanup; myexit" EXIT

#
#
#
comment="default cpu args is 'host'"
docker run \
    --runtime runq \
    --name $name1 \
    -d \
    $image  \
    cat

sleep 2

ps -e -o cmd | grep -q -- "[-]cpu host "
checkrc $? 0 "$comment"

#
#
#
comment="set custom qemu cpu arguments"
case "$(uname -m)" in
    s390x)
        . /etc/lsb-release
        if [ "$DISTRIB_CODENAME" = "bionic" ]; then
            cpuargs="z13"
        else
            cpuargs="host,apqi=off"
        fi
        ;;
    x86_64)
        cpuargs="host,rtm=off"
        ;;
esac

docker run \
    --runtime runq \
    --name $name2 \
    -e RUNQ_CPUARGS=$cpuargs \
    -d \
    $image  \
    cat

sleep 2

ps -e -o cmd | grep -q -- "[-]cpu .*$cpuargs "
checkrc $? 0 "$comment"

