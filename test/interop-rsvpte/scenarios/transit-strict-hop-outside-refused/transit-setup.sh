#!/bin/sh
# Ze transit (transit.conf) for transit-strict-hop-outside-refused. Its native route to the strict hop 198.51.100.9 runs through 172.29.81.4, outside both abstract nodes: Ze MUST refuse with PathErr 24/2.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
ip route add 198.51.100.9/32 via 172.29.81.4 dev eth0
exec ze start /etc/ze/ze.conf
