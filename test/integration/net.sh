#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

n_bridge=2
n_macvlan=2

cleanup() {
    for ((i=0; i<n_bridge; i++)); do
        docker network rm my-bridge-net-$$-$i &>/dev/null
    done
    for ((i=0; i<n_macvlan; i++)); do
        docker network rm my-macvlan-net-$$-$i &>/dev/null
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

for ((i=0; i<n_bridge; i++)); do
    net_name=my-bridge-net-$$-$i
    if [[ -z $(docker network ls -q  --filter name=$net_name) ]]; then
        docker network create -d bridge $net_name
    fi
    docker network connect $net_name $name
done

for ((i=0; i<n_macvlan; i++)); do
    net_name=my-macvlan-net-$$-$i
    if [[ -z $(docker network ls -q  --filter name=$net_name) ]]; then
        docker network create -d macvlan $net_name
    fi
    docker network connect $net_name $name
done

docker start -ai $name
checkrc $? $((n_bridge + n_macvlan + 2)) "$comment"

