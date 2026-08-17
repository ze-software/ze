#!/usr/bin/env bash
# Manual runner for the VLAN QoS lab.
# Sets up a two-netns topology with a veth pair and VLAN sub-interface
# carrying 802.1p QoS maps. Interactive mode prints tcpdump instructions;
# --selftest runs the AC-1 egress PCP scenario and exits 0/1.
#
# Usage: ./run.sh [--selftest]
#
# Prerequisites: Linux, root, ip, tcpdump, python3

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

SELFTEST=false
[ "${1:-}" = "--selftest" ] && SELFTEST=true

if [ "$(uname)" != "Linux" ]; then
    echo "error: requires Linux (netns, veth, AF_PACKET)"
    exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
    echo "error: requires root (netns, veth, AF_PACKET)"
    exit 1
fi

for cmd in ip tcpdump python3; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "error: $cmd not found"
        exit 1
    fi
done

NS_ZE="zelab-ze"
NS_PEER="zelab-peer"

cleanup() {
    ip netns del "$NS_ZE" 2>/dev/null || true
    ip netns del "$NS_PEER" 2>/dev/null || true
    kill "$(jobs -p)" 2>/dev/null || true
    wait 2>/dev/null || true
    rm -f /tmp/zelab-capture.txt
}
trap cleanup EXIT

# Build ze (sanity check; config reference in ze-vlan-qos.conf)
if [ ! -x "$REPO_ROOT/bin/ze" ]; then
    echo "Building ze..."
    (cd "$REPO_ROOT" && CGO_ENABLED=0 go build -tags 'ze_core ze_distro' -o bin/ze ./cmd/ze)
fi

# Create two namespaces connected by a veth pair
ip netns add "$NS_ZE"
ip netns add "$NS_PEER"
ip link add eth0 type veth peer name peer0
ip link set eth0 netns "$NS_ZE"
ip link set peer0 netns "$NS_PEER"

# Ze-side: VLAN sub-interface with egress and ingress QoS maps
ip netns exec "$NS_ZE" ip link set lo up
ip netns exec "$NS_ZE" ip link set eth0 up
ip netns exec "$NS_ZE" ip link add link eth0 name eth0.100 type vlan id 100 \
    egress-qos-map 0:0 6:6 7:7 \
    ingress-qos-map 0:0 6:6 7:7
ip netns exec "$NS_ZE" ip link set eth0.100 up
ip netns exec "$NS_ZE" ip addr add 10.0.0.1/24 dev eth0.100
ip netns exec "$NS_ZE" ip neigh add 10.0.0.2 lladdr 00:00:00:00:00:02 dev eth0.100

# Peer-side: raw capture endpoint
ip netns exec "$NS_PEER" ip link set lo up
ip netns exec "$NS_PEER" ip link set peer0 up

echo "Topology ready:"
echo "  $NS_ZE:   eth0 -> eth0.100 (VLAN 100, egress 6:6 7:7, ingress 6:6 7:7)"
echo "  $NS_PEER: peer0 (capture/inject endpoint)"
echo ""

if $SELFTEST; then
    # AC-1: SO_PRIORITY=6 through egress-qos-map 6:6 must produce PCP 6
    ip netns exec "$NS_PEER" timeout 3 tcpdump -i peer0 -e -c 1 'vlan 100' \
        > /tmp/zelab-capture.txt 2>/dev/null &
    TCPDUMP_PID=$!
    sleep 0.5

    ip netns exec "$NS_ZE" python3 -c "
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.setsockopt(socket.SOL_SOCKET, socket.SO_PRIORITY, 6)
s.bind(('10.0.0.1', 0))
s.sendto(b'qos-test', ('10.0.0.2', 9999))
s.close()
"

    wait "$TCPDUMP_PID" 2>/dev/null || true

    if grep -q 'p 6' /tmp/zelab-capture.txt; then
        echo "PASS: PCP 6 observed on peer0 (egress QoS map verified)"
        exit 0
    else
        echo "FAIL: PCP 6 not found in capture:"
        cat /tmp/zelab-capture.txt
        exit 1
    fi
fi

# Interactive mode
cat <<'INSTRUCTIONS'
Egress test (verify PCP bits on the wire):

  Terminal 1 -- capture on peer:
    ip netns exec zelab-peer tcpdump -i peer0 -e -n

  Terminal 2 -- send with priority 6:
    ip netns exec zelab-ze python3 -c "
    import socket
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.setsockopt(socket.SOL_SOCKET, socket.SO_PRIORITY, 6)
    s.bind(('10.0.0.1', 0))
    s.sendto(b'qos-test', ('10.0.0.2', 9999))
    s.close()
    "

  Expected tcpdump output contains:
    vlan 100, p 6, ethertype IPv4

Ingress test (verify PCP classification):

  Terminal 1 -- set up nftables counter in ze-ns:
    ip netns exec zelab-ze nft add table ip zelab
    ip netns exec zelab-ze nft add chain ip zelab prerouting \
        '{ type filter hook prerouting priority -300 ; }'
    ip netns exec zelab-ze nft add rule ip zelab prerouting \
        iifname eth0.100 meta priority 6 counter accept

  Terminal 2 -- inject tagged frame from peer:
    (requires AF_PACKET; use the QEMU integration test for automated version)

  Terminal 1 -- check counter:
    ip netns exec zelab-ze nft list table ip zelab
    # Look for "counter packets 1" on the priority-6 rule

Press Ctrl+C to stop.
INSTRUCTIONS

while true; do sleep 86400; done
