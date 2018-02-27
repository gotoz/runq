#!/bin/bash
DIR=$(cd ${0%/*};pwd;)
set -u
set -e

TMP_DIR=$DIR/tmp
rm -rf $TMP_DIR
mkdir $TMP_DIR

pushd $QEMU_ROOT >/dev/null
while read x f; do
    [[ $x == \#* ]] && continue
    cp --parents ./$f $TMP_DIR/
done < kernel.conf

cp kernel.conf $TMP_DIR/
popd >/dev/null

cp $DIR/../cmd/init/init $TMP_DIR/

pushd $TMP_DIR >/dev/null
find . | cpio -o -H newc | gzip > $DIR/initrd
popd >/dev/null

