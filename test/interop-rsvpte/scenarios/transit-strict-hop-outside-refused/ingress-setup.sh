#!/bin/sh
# Ze ingress (ingress.conf) for transit-strict-hop-outside-refused. Every ERO hop is strict.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
ip route add 198.51.100.9/32 via 172.29.81.15 dev eth0
exec ze start /etc/ze/ze.conf
