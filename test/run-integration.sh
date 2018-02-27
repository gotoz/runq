#!/bin/bash
trap exit 2 15
DIR=$(cd ${0%/*};pwd;)
for f in $DIR/integration/*.sh; do
    test -x $f || continue
    if [ "$1" = "-v" ]; then
        $f
    else
        $f 2>&1 | grep ^test
    fi
done
