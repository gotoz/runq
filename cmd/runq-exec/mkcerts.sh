#!/bin/bash
# Create TLS server certificates in $QEMU_ROOT/certs and corresponding client
# certificates in $RUNQ_ROOT. Existing certificates will be overwritten.

set -e
set -u

PASSPHRASE=`cat /proc/sys/kernel/random/uuid`
DAYS=3650
SUBJ="/emailAddress=ignore@example.com"

RUNQ_ROOT=${RUNQ_ROOT:-/var/lib/runq}
QEMU_ROOT=${QEMU_ROOT:-$RUNQ_ROOT/qemu}

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" 0 2 15
pushd $TMPDIR

# ca
openssl genrsa -aes256 -out ca.key -passout pass:$PASSPHRASE 2048
openssl req -new -x509 -days $DAYS -key ca.key -sha256 -out ca.cert \
  -passin pass:$PASSPHRASE -subj "$SUBJ"

# server
openssl genrsa -out server.key 2048
openssl req -subj "/CN=server" -sha256 -new -key server.key -out server.csr

echo extendedKeyUsage = serverAuth > server.cnf
openssl x509 -req -days $DAYS -sha256 -in server.csr -CA ca.cert -CAkey ca.key \
  -CAcreateserial -out server.cert -extfile server.cnf -passin pass:$PASSPHRASE

# client
openssl genrsa -out client.key 2048
openssl req -subj '/CN=client' -new -key client.key -out client.csr

echo extendedKeyUsage = clientAuth > client.cnf
openssl x509 -req -days $DAYS -sha256 -in client.csr -CA ca.cert -CAkey ca.key \
  -CAcreateserial -out client.cert -extfile client.cnf -passin pass:$PASSPHRASE

rm -rf $QEMU_ROOT/certs
mkdir -p $QEMU_ROOT/certs
cp ca.cert $QEMU_ROOT/certs/ca.pem
cp server.cert $QEMU_ROOT/certs/cert.pem
cp server.key $QEMU_ROOT/certs/key.pem
chmod 0400 $QEMU_ROOT/certs/*
chmod 0700 $QEMU_ROOT/certs

rm -f $RUNQ_ROOT/{cert.pem,key.pem}
cp client.cert $RUNQ_ROOT/cert.pem
cp client.key $RUNQ_ROOT/key.pem
chmod 0440 $RUNQ_ROOT/{cert.pem,key.pem}

chmod 0750 $RUNQ_ROOT
