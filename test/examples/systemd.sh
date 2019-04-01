#!/bin/bash

docker run \
    -e RUNQ_MEM=2048 \
    -e RUNQ_CPU=2 \
    --restart on-failure:3 \
    --runtime runq \
    --cap-add all \
    --security-opt seccomp=unconfined \
    -p 2222:22 \
    -ti systemd /sbin/init

