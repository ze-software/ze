#!/bin/sh
# Ze ingress (ingress.conf) for transit-strict-hop-forwarded. Every ERO hop is strict.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
exec ze start /etc/ze/ze.conf
