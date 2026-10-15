# VLAN 802.1p QoS Maps

A VLAN unit carries two independent maps between the 3-bit PCP field of the
802.1Q tag and the kernel's internal packet priority. `ingress-qos-map` is
keyed by received PCP and gives the internal priority. `egress-qos-map` is keyed
by internal priority and gives the PCP put on the wire.

The named-profile layer above these maps is
[`../traffic/cos-plugin.md`](../traffic/cos-plugin.md). This page is the
mechanism the interface component owns.

<!-- source: internal/plugins/iface/netlink/manage_linux.go -- CreateVLAN, UpdateVLANQoSMap, qosReplaceMaps, validateQoSMap -->
<!-- source: internal/component/iface/iface.go -- IngressQoSMap, EgressQoSMap -->

## Decision: two maps, because the kernel has two

The maps are separate netlink attributes on the VLAN link,
`IFLA_VLAN_INGRESS_QOS` and `IFLA_VLAN_EGRESS_QOS`, carried inside
`RTM_NEWLINK`. A nil map emits no attribute, so an interface with no QoS config
is created exactly as before the feature existed. A bidirectional shorthand
would hide that the two directions are independent.

## Decision: the range check runs before the netlink call

IEEE 802.1Q gives the PCP field 3 bits, so both sides of every entry are 0 to 7.
`validateQoSMap` rejects an out-of-range entry and names it, on the create path
and on the update path. The kernel would accept a larger value and truncate it.

## Decision: a map change modifies the link, it does not recreate it

`UpdateVLANQoSMap` looks the device up, asserts it is a VLAN device and calls
`LinkModify`. Deleting and recreating the subinterface would drop its addresses,
its routes and every session on it for a QoS-only change.

The call is a full replace: afterwards the VLAN holds exactly the given maps,
and a nil or empty map leaves no mapping. The kernel merges a QoS attribute per
entry and the library sends no attribute for a nil map, so the backend sends all
8 ingress slots (an absent one at priority 0, the kernel default) and zeroes
each egress key the installed map holds and the call omits (`qosReplaceMaps`).
`LinkModify` is sent a minimal `Vlan` carrying only the index, the name, the
VLAN id and the two maps, so no other attribute of the fetched snapshot is
re-sent. Calls are serialised under a package mutex held from the read to the
write, because the cos RADIUS handler and an interface apply can overlap. The
VPP backend writes a whole egress row, disables the mark for an empty egress
map, and serialises its two requests under a backend mutex.

`show interface` reports the maps the kernel holds, not the maps the config
asked for.

<!-- source: internal/plugins/iface/netlink/show_linux.go -- the reported QoS maps -->

## Trap: kernel state is not wire behavior

A test that reads the maps back from the kernel proves the netlink attribute
landed. It does not prove a frame leaves with the mapped PCP bits, and it does
not prove an arriving PCP reaches the mapped priority. The two are different
claims and only the second one is what an operator buys.

Three wire-level proofs cover it, each in a network namespace with a veth pair:

| Proof | What it asserts |
|-------|-----------------|
| Egress | a frame captured on the peer side carries the PCP the egress map assigns to the packet priority |
| Ingress | a tagged frame with a given PCP increments a tc filter that matches the mapped internal priority |
| Full chain | DSCP-marked IP traffic is classified to a priority by the firewall stage and leaves with the mapped PCP |

The capture side decodes the tag from raw bytes rather than parsing tcpdump
text: TPID `0x8100` at octet 12, then `TCI = PCP(3) | DEI(1) | VID(12)`, so
`PCP = TCI >> 13`. A bit-shift error there passes every positive-only assertion,
which is why the decoder has its own unit test over all eight PCP values plus a
truncated frame and an untagged frame.

Ingress cannot be observed directly, because no interface reads `skb->priority`
back. The tc filter hit counter is the available proxy.
