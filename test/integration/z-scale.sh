#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

n=10
timeout=30
ports=()
names=()

for ((i=0; i<n; i++)); do
    port=$(rand_port)
    name=$(rand_name)
    ports+=($port)
    names+=($name)
    docker run \
        --runtime runq \
        --rm \
        --name $name \
        -e RUNQ_CPU=1 \
        -e RUNQ_MEM=128 \
        -d \
        -p $port:$port \
        $image \
        sh -c "echo $port | nc -l -p $port" &
done
wait

sleep $n

rc=0
for p in "${ports[@]}"; do
    nc -z localhost $p
    rc=$(($? + rc))
    echo checked localhost:$p $rc
    sleep .5
done


docker rm -f ${names[@]} 2>/dev/null

checkrc $rc 0 "start $n containers in parallel"

myexit
