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
    sh -c "echo $port | nc -l -p $port"

test "$(curl -m $timeout -s localhost:$port)" = "$port"
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
    sh -c "echo $port | nc -l -p $port"

addr="$(docker port $name | awk '{print $NF}')"
test "$(curl -m $timeout -s $addr)" = "$port"
checkrc $? 0 "$comment"

docker rm -f $name 2>/dev/null

myexit
