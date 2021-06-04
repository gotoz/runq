#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

timeout=5

comment="publish a container's port to the host"
port=$(rand_port)
name=$(rand_name)
docker run \
    --runtime runq \
    --rm \
    --name $name \
    -d \
    -p $port:$port \
    $image  \
    sh -c "nc -l -p $port"

sleep 2

nc -z localhost $port
checkrc $? 0 "$comment"

docker rm -f $name 2>/dev/null

#
#
#
comment="publish an exposed port to a random port"
port=$(rand_port)
name=$(rand_name)
docker run \
    --runtime runq \
    --rm \
    --expose $port \
    --name $name \
    -d \
    -P \
    $image  \
    sh -c "nc -l -p $port"

host_port="$(docker port $name | grep -v ':::' | awk -F: '{print $NF}')"

sleep 2

nc -z localhost $host_port
checkrc $? 0 "$comment"

docker rm -f $name 2>/dev/null

myexit
