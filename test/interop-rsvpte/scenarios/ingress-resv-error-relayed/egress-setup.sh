#!/bin/sh
# Ze egress (egress.conf) for ingress-resv-error-relayed. It answers the PATH and receives the ResvErr freeRouter relays.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
exec ze start /etc/ze/ze.conf
