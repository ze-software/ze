#!/bin/sh
# Ze transit (transit.conf) for transit-strict-hop-forwarded. The strict hop 172.29.81.14 is adjacent, so Ze forwards the PATH with the ERO shortened to it.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
exec ze start /etc/ze/ze.conf
