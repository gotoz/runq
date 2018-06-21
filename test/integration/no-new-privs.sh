#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

local_image=runq-test-nonewprivs
build_dir=$(mktemp -d)
cleanup() {
    rm -rf $build_dir
    docker rmi $local_image
}
trap "cleanup; myexit" 0 2 15

pushd $build_dir

cat << EOF > getid.c
#include <stdio.h>
#include <unistd.h>
int main(int argc, char *argv[])
{
    printf("uid:%d euid:%d\n", getuid(), geteuid());
    return 0;
}
EOF
gcc -Wall -o getid -static getid.c

cat << EOF > Dockerfile
FROM $image
ADD getid /getid
RUN echo "demo:x:1000:100:demo:/:/bin/sh" >> /etc/passwd && chmod +s /getid
EOF
docker build -t $local_image .
popd

comment="set no-new-privileges to 1 makes suid root useless"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --user 1000 \
    --security-opt=no-new-privileges \
    $local_image \
    sh -c "/getid | grep '^uid:1000 euid:1000$'"

checkrc $? 0 "$comment"


comment="countercheck no-new-privileges not set"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --user 1000 \
    $local_image \
    sh -c "/getid | grep '^uid:1000 euid:0$'"

checkrc $? 0 "$comment"

#
# requires kernel >= 4.10
#
comment="check /proc/self/status"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --user 1000 \
    --security-opt=no-new-privileges \
    $local_image \
    sh -c "grep '^NoNewPrivs:.*1$' /proc/self/status"

checkrc $? 0 "$comment"

comment="countercheck /proc/self/status"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --user 1000 \
    $local_image \
    sh -c "grep '^NoNewPrivs:.*0$' /proc/self/status"

checkrc $? 0 "$comment"

