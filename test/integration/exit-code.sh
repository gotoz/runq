#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

docker run --runtime runq --rm --name $(rand_name) $image true
checkrc $? 0 "valid executable rc=0"

docker run --runtime runq --rm --name $(rand_name) $image false
checkrc $? 1 "valid executable rc=1"

docker run --runtime runq --rm --name $(rand_name) $image /etc/hosts
checkrc $? 126 "no exec permission rc=126 permission denied"

docker run --runtime runq --rm --name $(rand_name) $image /etc
checkrc $? 126 "not an executable rc=126 permission denied"

docker run --runtime runq --rm --name $(rand_name) $image foobar
checkrc $? 127 "executable not found rc=127"

docker run --runtime runq --rm --name $(rand_name) $image sh -c "exit 42;"
checkrc $? 42 "custom exit code rc=42"

docker run --runtime runq --rm --name $(rand_name) $image sh -c "exit -1;"
checkrc $? 2 "illegal exit code"

myexit
