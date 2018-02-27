#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

tmpfile=$(mktemp)
trap "rm -f $tmpfile; myexit" 0 2 15

for ((i=0; i< 1000; i++)); do
    foo=$(dd if=/dev/urandom bs=1k count=10 2>/dev/null | tr -dc A-Za-z0-9 | tail -c 1024)
    echo foo$i=$foo >> $tmpfile
done
md5_999=$(echo -n $foo|md5sum|awk '{print $1}')
echo $md5_999

comment="apply very large environment (1000 vars, each 1k)"
docker run \
    --runtime runq \
    --name $(rand_name) \
    --rm \
    --env-file $tmpfile \
    --env md5_999=$md5_999 \
    $image  \
    sh -c 'md5=$(echo -n $foo999|md5sum|awk "{print \$1}"); echo $md5; test $md5 = $md5_999'

checkrc $? 0 "$comment"

