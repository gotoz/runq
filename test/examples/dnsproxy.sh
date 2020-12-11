#!/bin/bash
. $(cd ${0%/*};pwd;)/../common.sh

DIR=$(cd ${0%/*};pwd;)
cleanup() {
    docker rm -f dns1 dns2 foo >/dev/null
    docker network rm mynet >/dev/null
}
trap "cleanup; myexit" EXIT

# Note:
# /etc/resolv.conf of runq containers is written only once at container start.
# Therefore the IP address of the DNS proxy container must not change.
# /etc/resolv.conf must not contain a search option.

# build dnsmasq image
docker build -t dnsmasq -f $DIR/Dockerfile.dnsmasq .

#
# example Docker network with default network address
#
  # create network
  docker network create mynet

  # start DNS proxy container with name (runc)
  docker run --runtime runc --net mynet --cap-add=NET_ADMIN --restart unless-stopped --name dns1 -d dnsmasq
  docker run --runtime runc --net mynet --cap-add=NET_ADMIN --restart unless-stopped --name dns2 -d dnsmasq

  # start named runq container foo (daemon)
  docker run --net mynet --name foo --runtime runq --rm -td alpine sh

  # resolve foo's IP via DNS proxy
  docker run --net mynet --runtime runq --rm -e RUNQ_DNS=dns1,dns2 alpine ping -c 3 foo

checkrc $? 0 "dnsproxy by name"
cleanup

#
# example Docker network with custom network address
#
  # create network
  docker network create --subnet=172.30.0.0/16 mynet

  # start DNS proxy container with fixed IP (runc)
  docker run --runtime runc --net mynet --cap-add=NET_ADMIN --ip 172.30.0.254 --name dns1 -d dnsmasq
  docker run --runtime runc --net mynet --cap-add=NET_ADMIN --ip 172.30.0.253 --name dns2 -d dnsmasq

  # start named runq container
  docker run --net mynet --name foo --runtime runq --rm -td alpine sh

  # resolve foo's IP via DNS proxy
  docker run --net mynet --runtime runq --rm -e RUNQ_DNS=172.30.0.254,172.30.0.253 alpine ping -c 3 foo

checkrc $? 0 "dnsproxy by IP address"
