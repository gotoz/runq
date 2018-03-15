#!/bin/bash

rc_total=0
for f in $@; do
    test -x $f || continue
    log=$(mktemp)
    $f &>$log
    if [ $? -ne 0 ]; then
        ((rc_total++))
        echo -e "#\n# BEGIN LOG $(basename $f)\n#"
        cat $log
        echo -e "#\n# END LOG $(basename $f)\n#\n"
    fi
    grep -E '^test (succeeded|failed|skipped)' $log
    rm -f $log
done

exit $rc_total
