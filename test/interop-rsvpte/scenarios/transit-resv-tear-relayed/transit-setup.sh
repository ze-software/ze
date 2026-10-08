#!/bin/sh
# Ze transit (transit.conf) for transit-resv-tear-relayed. Its reservation times out once the egress is frozen, and it sends a ResvTear upstream.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
exec ze start /etc/ze/ze.conf
