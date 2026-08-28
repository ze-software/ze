# VLAN QoS Lab

Proves that VLAN 802.1p QoS maps work on the wire, not just in kernel state.

## Topology

```
  zelab-ze netns                    zelab-peer netns
 ┌──────────────────┐              ┌──────────────┐
 │ eth0.100 (VLAN)  │              │              │
 │  10.0.0.1/24     │              │              │
 │  egress  6:6 7:7 │              │              │
 │  ingress 6:6 7:7 │              │              │
 │       │          │              │              │
 │     eth0 ←───────┼── veth pair ─┼──→ peer0     │
 └──────────────────┘              └──────────────┘
```

- **eth0.100**: VLAN sub-interface with QoS maps. Egress map translates
  skb->priority to PCP; ingress map translates PCP to skb->priority.
- **peer0**: raw capture/inject endpoint. Sees tagged 802.1Q frames.

## Scenarios

### Egress (PCP on the wire)

Send UDP from eth0.100 with `SO_PRIORITY=6`. The egress QoS map stamps
PCP 6 in the 802.1Q header. Capture on peer0 to verify.

Expected tcpdump output:
```
10:23:45.678 00:...:01 > 00:...:02, ethertype 802.1Q (0x8100), length 50:
    vlan 100, p 6, ethertype IPv4, 10.0.0.1 > 10.0.0.2: UDP
```

The `p 6` confirms PCP 6.

### Ingress (PCP classification)

Inject a raw 802.1Q frame with PCP 6 on peer0. The frame arrives on eth0,
the 8021q module strips the tag and delivers to eth0.100, and the ingress
QoS map translates PCP 6 to skb->priority 6. Verify with nftables
`meta priority` counter.

### Full chain (DSCP to PCP)

UDP with DSCP CS6 is classified by nftables to priority 6. The egress map
stamps PCP 6. This is the BNG-style CoS pipeline: DSCP classification at
L3, PCP marking at L2.

## Usage

The compiled integration test creates and removes its own network namespace,
veth pair, VLAN interfaces, addresses, neighbors, and nftables counters:

```bash
sudo env PATH="$PATH" go test -tags=integration ./internal/plugins/iface/netlink \
  -run '^TestVLANQoS' -count=1
```

The native test covers mapped and unmapped egress PCP, mapped and unmapped
ingress classification, and the complete DSCP CS6 to priority 6 to PCP 6
pipeline. Each assertion reads the packet counter produced by the kernel path.

## Prerequisites

- Linux with network namespace support
- Root or equivalent `CAP_NET_ADMIN` and `CAP_NET_RAW` capabilities
- `nft`

## Limitations

- Proves behavior on veth pairs (software), not physical NICs with
  hardware VLAN offload.
- Single-tag 802.1Q only. QinQ (double-tag) requires stacked VLAN
  support in Ze, which is out of scope.
- No throughput or latency assertions. This lab proves marking
  correctness, not QoS scheduling behavior.
