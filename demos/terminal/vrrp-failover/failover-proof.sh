#!/usr/bin/env bash
set -euo pipefail

state=/src/tmp/terminal-demos/state/vrrp-failover
kill -KILL "$(cat "${state}/ze.pid")"
rm "${state}/ze.pid"
sleep 1
ip netns del vrrp-ze
sleep 8
ip -n vrrp-peer -br addr show vrrp.10
if timeout 5 ping -q -c 2 -W 1 192.0.2.1; then
    echo "VIP answered both probes"
else
    echo "VIP probe failed" >&2
    exit 1
fi
