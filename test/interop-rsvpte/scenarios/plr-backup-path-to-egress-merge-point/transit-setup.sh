#!/bin/sh
# Ze PLR (transit.conf) for plr-backup-path-to-egress-merge-point. prot0 is the
# protected link, VLAN 100 to the freeRouter egress (10.0.14.14). The bypass
# runs untagged through the freeRouter relay to the same egress, which is the
# merge point. The checker takes prot0 down to trigger local repair.
set -eu
ip link add link eth0 name prot0 type vlan id 100
ip addr add 10.0.14.3/24 dev prot0
ip link set prot0 up
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.mpls.conf.prot0.input=1 net.ipv4.ip_forward=1
exec ze start /etc/ze/ze.conf
