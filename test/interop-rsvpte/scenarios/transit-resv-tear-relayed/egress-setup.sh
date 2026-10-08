#!/bin/sh
# Ze egress (egress.conf) for transit-resv-tear-relayed. The checker freezes it once the LSP is up.
set -eu
sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1
exec ze start /etc/ze/ze.conf
