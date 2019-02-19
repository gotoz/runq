FROM alpine:latest
RUN apk add --no-cache dnsmasq
ENTRYPOINT dnsmasq \
    -k \
    --log-facility=- \
    --log-queries \
    --cache-size=0 \
    --no-hosts \
    --no-resolv \
    --server=/#/127.0.0.11

