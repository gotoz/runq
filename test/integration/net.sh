#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

n_bridge=2
n_macvlan=2
name=$(rand_name)

cleanup() {
    docker rm -f $name
    for n in $(docker network ls --format '{{.Name}}' | grep ^runqtest); do
        docker network rm $n &>/dev/null
    done
}

trap "cleanup; myexit" 0 2 15

comment="attach container to multiple networks"
docker create \
    --runtime runq \
    --rm \
    --name $name \
    $image sh -c 'rc=$(ls -d /sys/class/net/*|wc -l); exit $rc'

for ((i=0; i<n_bridge; i++)); do
    net_name=runqtest-bridge-$i
    docker network create -d bridge $net_name
    docker network connect $net_name $name
done

for ((i=0; i<n_macvlan; i++)); do
    net_name=runqtest-macvlan-$i
    docker network create -d macvlan $net_name
    docker network connect $net_name $name
done

docker start -ai $name
checkrc $? $((n_bridge + n_macvlan + 2)) "$comment"

#
#
#
comment="exchange ip message via macvlan"
net_name=runqtest-macvlan-42
docker network create -d macvlan --subnet=192.168.42.0/24 $net_name

docker run \
    --runtime runq \
    --name $name \
    --net $net_name \
    --ip 192.168.42.2 \
    --rm \
    -td \
    $image sh

docker run \
    --runtime runq \
    --rm \
    --net $net_name \
    --ip 192.168.42.3 \
    $image ping -c 3 -w 3 -W 3 192.168.42.2

checkrc $? 0 "$comment"

