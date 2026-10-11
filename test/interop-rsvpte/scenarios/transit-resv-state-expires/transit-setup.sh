#!/bin/sh
# Ze transit (transit.conf) for transit-resv-state-expires. Once the LSP is up
# the checker freezes the freeRouter egress, so no RESV refreshes Ze's
# reservation: Ze MUST time it out, send a ResvTear upstream and withdraw its swap.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
ip route add 198.51.100.4/32 via 172.29.81.14 dev eth0
ip route add 198.51.100.2/32 via 172.29.81.12 dev eth0
exec ze start /etc/ze/ze.conf
