#!/bin/sh
# Ze transit for transit-loose-ero-expansion. The loose hop 198.51.100.4 is the
# egress loopback, reached natively through the egress's segment address
# 172.29.81.14, which no ERO subobject names: Ze MUST insert it.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
ip route add 198.51.100.4/32 via 172.29.81.14 dev eth0
ip route add 198.51.100.2/32 via 172.29.81.12 dev eth0
exec ze start /etc/ze/ze.conf
