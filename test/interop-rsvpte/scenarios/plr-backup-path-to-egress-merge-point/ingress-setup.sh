#!/bin/sh
# Ze ingress (ingress.conf) for plr-backup-path-to-egress-merge-point. The tunnel
# asks for facility protection and names the PLR and the egress's VLAN 100
# address strict, so the protected hop is the PLR's prot0 link.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
ip route add 10.0.14.0/24 via 172.29.81.3
exec ze start /etc/ze/ze.conf
