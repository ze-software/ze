FROM alpine:3.24.3
# The host's LabBinary staging applies the recorder's Go overlay to this package.
COPY test/interop-l2tp/avp-relay-linux /usr/local/bin/avp-relay
ENTRYPOINT ["/usr/local/bin/avp-relay"]
