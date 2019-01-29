#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

name=$(rand_name)
disk=$PWD/disk$$
image=postgres:alpine
PGPASSWORD=mysecretpassword

dd if=/dev/zero of=$disk bs=1M count=200 >/dev/null
mkfs.ext4 -F $disk

cleanup() {
    echo cleanup
    docker rm -f $name
    rm -f $disk
    myexit
}
trap cleanup 0 2 15

docker run \
    --runtime runq \
    --name $name \
    -e RUNQ_MEM=512 \
    -e RUNQ_CPU=2 \
    -e POSTGRES_PASSWORD=$PGPASSWORD \
    -v $disk:/dev/runq/$(uuid)/none/ext4/var/lib/postgresql \
    -d \
    $image

sleep 30

comment="Postgres"

docker run \
    --runtime runq \
    --rm \
    --link $name:postgres \
    -e PGPASSWORD=$PGPASSWORD \
    $image \
      psql -h postgres -U postgres -c "select 42;"

checkrc $? 0 "$comment"

