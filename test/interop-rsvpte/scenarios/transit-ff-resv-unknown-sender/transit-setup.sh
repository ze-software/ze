#!/bin/sh
# Ze transit (transit.conf) for transit-ff-resv-unknown-sender. The freeRouter
# egress sends fixed-filter RESVs with a second descriptor naming a sender
# that has no path state (egress-env.txt): Ze MUST refuse that descriptor alone.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
ip route add 198.51.100.4/32 via 172.29.81.14 dev eth0
ip route add 198.51.100.2/32 via 172.29.81.12 dev eth0
exec ze start /etc/ze/ze.conf
