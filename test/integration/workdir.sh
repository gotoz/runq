#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

local_image=runq-test-workdir
build_dir=$(mktemp -d)
cleanup() {
    rm -rf $build_dir
    docker rmi $local_image
}
trap "cleanup; myexit" EXIT

#
# custom workdir
#
comment="set custom workdir"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --workdir /work \
    $image  \
    sh -c 'pwd | grep -w /work'

checkrc $? 0 "$comment"

#
# restricted workdir
#
pushd $build_dir
cat << EOF > Dockerfile
FROM $image
RUN adduser -DH demo
RUN mkdir -m 0700 /work
RUN chown demo:demo /work
WORKDIR /work
EOF
docker build -t $local_image .
popd

comment="restricted workdir"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    $local_image \
    sh -c 'pwd | grep -w /work'

checkrc $? 0 "$comment"


#
# restricted workdir and custom user
#
pushd $build_dir
cat << EOF > Dockerfile
FROM $image
RUN adduser -DH -u 4242 demo
RUN mkdir -m 0700 /work
RUN chown demo:demo /work
WORKDIR /work
EOF
docker build -t $local_image .
popd

comment="restricted workdir and custom user"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --user 4242 \
    --rm \
    $local_image \
    sh -c 'pwd | grep -w /work && id | grep 4242'

checkrc $? 0 "$comment"

