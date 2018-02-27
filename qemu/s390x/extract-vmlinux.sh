#!/bin/sh

IN=$1
OUT=$2
MAGIC="\xfd\x37\x7a\x58\x5a\x00"
tmp=$(mktemp)

trap "rm -f $tmp" 0

for pos in $(LANG=C grep -obUaP "$MAGIC" $IN); do
    pos=${pos%%:*}
    pos=$((pos + 1))
    tail -c+$pos $IN | unxz > $tmp 2>/dev/null
    magic=$(dd if=$tmp skip=65544 bs=1 count=6 2> /dev/null)
    if [ "$magic" = "S390EP" ]; then
        cat $tmp > $OUT
        exit 0
    fi
done

exit 1

