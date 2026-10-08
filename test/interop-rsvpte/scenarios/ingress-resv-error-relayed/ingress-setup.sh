#!/bin/sh
# Ze ingress (ingress.conf) for ingress-resv-error-relayed. Its interface reserves 1000 bit/s against a 1 Mbit/s tunnel, so it refuses the RESV with admission control failure.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
exec ze start /etc/ze/ze.conf
