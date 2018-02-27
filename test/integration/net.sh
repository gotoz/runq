#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

n=3
cleanup() {
    for ((i=0; i<n; i++)); do
        docker network rm mynet-$$-$i &>/dev/null
    done
}

trap "cleanup; myexit" 0 2 15

comment="attach container to multiple networks"
name=$(rand_name)
docker create \
    --runtime runq \
    --rm \
    --name $name \
    $image sh -c 'rc=$(ls -d /sys/class/net/*|wc -l); exit $rc'

for ((i=0; i<n; i++)); do
    net_name=mynet-$$-$i
    if [[ -z $(docker network ls -q  --filter name=$net_name) ]]; then
        docker network create $net_name
    fi
    docker network connect $net_name $name
done

docker start -ai $name
checkrc $? $((n+2)) "$comment"

