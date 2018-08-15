#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

runq_exec=/var/lib/runq/runq-exec
tmpfile=$(mktemp)
name=$(rand_name)

cleanup() {
    rm -f $tmpfile
}
trap "cleanup; myexit" 0 2 15

#
#
#
comment="capture stdout"
docker run \
    --runtime runq \
    --name $name \
    -v $runq_exec:/runq-exec \
    -dt \
    $image sh

sleep 2

md5=$($runq_exec $name cat /runq-exec | md5sum | awk '{print $1}')
echo "$md5 $runq_exec" > $tmpfile
md5sum -c $tmpfile
checkrc $? 0 "$comment"
docker rm -f $name

#
#
#
name=$(rand_name)
docker run \
    --runtime runq \
    --name $name \
    -v $runq_exec:/runq-exec \
    -dt \
    $image sh

sleep 2

# tty
$runq_exec -t invalid_name true
checkrc $? 1 "tty, check rc: invalid id/name rc=1"

$runq_exec -t $name true
checkrc $? 0 "tty, check rc: valid executable rc=0"

$runq_exec -t $name false
checkrc $? 1 "tty, check rc: valid executable rc=1"

$runq_exec -t $name /etc/hosts
checkrc $? 126 "tty, check rc: no exec permission rc=126 permission denied"

$runq_exec -t $name /etc
checkrc $? 126 "tty, check rc: not an executable rc=126 permission denied"

# no tty
$runq_exec invalid_name true
checkrc $? 1 "no tty, check rc: invalid id/name rc=1"

$runq_exec $name true
checkrc $? 0 "no tty, check rc: valid executable rc=0"

$runq_exec $name false
checkrc $? 1 "no tty, check rc: valid executable rc=1"

$runq_exec $name /etc/hosts
checkrc $? 126 "no tty, check rc: no exec permission rc=126 permission denied"

$runq_exec $name /etc
checkrc $? 126 "no tty, check rc: not an executable rc=126 permission denied"

n=10
for i in `seq 1 $n`; do
    $runq_exec $name sleep 10 &
done
sleep 5
r="`$runq_exec $name pidof sleep | wc -w`"
wait
checkrc $n $r "run $n exec commands simultaneously"

docker rm -f $name
