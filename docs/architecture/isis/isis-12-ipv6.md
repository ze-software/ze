# IS-IS Dual Stack

Single-topology IPv6 (RFC 5308): originate TLV 232, TLV 236 and NLPID `0x8E`, run
IPv6 route extraction over the **shared** SPF tree, install IPv6 routes through
the same Loc-RIB path as IPv4, and redistribute IPv6 in both directions.

The **wire** rules, including the TLV 232 and TLV 236 layouts and the address
scope by PDU, are documented in [`../wire/isis.md`](../wire/isis.md). This page
carries the runtime decisions.

| Concern | File |
|---------|------|
| Origination scope filters | `lsdb/origination_ipv6.go` |
| IPv6 leaf extraction, next hop, install seam | `spf/ipv6.go` |
| IPv6 redistribution, both directions | `redistribute/ipv6.go` |

## Decision: one SPF tree, a second leaf and install pass

The IPv6 path reuses the **same** results and graphs the IPv4 Dijkstra produces.
The computer builds one tree per level, then runs the IPv4 route build (TLV 135)
and the IPv6 route build (TLV 236) over that tree, each feeding its own family
installer. There is no second Dijkstra.

The IPv6 route build reads the node's IPv6 prefixes and applies the IPv6 maximum
path metric filter. The multi-level arbitration is **shared and unchanged**,
because RFC 5308 section 5 orders preference exactly as IPv4 does.

<!-- source: internal/plugins/isis/spf/ipv6.go -- MaxV6PathMetric, NextHopResolverV6, BuildRoutesV6, resolveHopsV6 -->

## Decision: the installer is parameterized by family and AFI

The IPv4 and IPv6 constructors both call one shared constructor. The only
per-family differences are the Loc-RIB family and the AFI label on the
installed-routes gauge. The insert, diff, ECMP and remove logic is identical.

The IPv6 install is the **same** FIB path as IPv4: a Loc-RIB path with the IS-IS
protocol ID and admin distance 115. It is not a redistribute event.

<!-- source: internal/plugins/isis/spf/install.go -- NewInstaller, NewInstallerV6, newInstaller -->

## Decision: the IPv6 next hop is the neighbor's link-local

A second resolver interface reads the neighbor's link-local address, which the
adjacency already stored from the neighbor's hello TLV 232, and carries the
circuit name as the interface. A link-local next hop is unusable without the
interface.

## Decision: scope is enforced at origination, not in the codec

The codec round-trips whatever it is given. The scope rules live at the
origination sites:

| PDU | TLV 232 contents |
|-----|------------------|
| Hello | **only** the link-local address, from the circuit layer |
| LSP | **only** non-link-local addresses, filtered by the level-state builder |

TLV 236 never carries a link-local prefix. The filter is applied at the engine
merge **and** defensively at the redistribution consumer and the connected
helper.

<!-- source: internal/plugins/isis/lsdb/origination_ipv6.go -- NonLinkLocalV6Prefixes, NonLinkLocalV6Addrs -->

## Decision: one redistribution source, the AFI selects the family

IS-IS keeps **one** `isis` protocol ID and one source for both families. The
route-change batch's AFI field, not the protocol ID, selects IPv4 or IPv6. The
IPv4 emit delegates to a generalized per-family emit, and the IPv6 SPF change
emits at AFI 2. The SPF computer carries a separate IPv6 change callback so the
IPv6 delta carries its own family.

<!-- source: internal/plugins/isis/redistribute/ipv6.go -- emitDeltaFamily, OnSPFChangeV6, ConnectedPrefixInfosV6 -->

Redistributed IPv6 sets the TLV 236 external bit (RFC 5308 section 2). IPv4 TLV
135 has no external bit. The up/down bit clears on injection for both families
(RFC 2966).

## Consequences

One adjacency carries both families. Enabling the IPv6 address family per
interface gates **all** IPv6: the NLPID, the hello TLV 232, the LSP TLV 232 and
236, and the IPv6 SPF pass.

The IPv6 install pass is always wired and is harmless on an IPv4-only topology:
no TLV 236 leaves means an empty IPv6 route set.

No new metric series was added. IPv6 sets `afi=ipv6` on the existing
`ze_isis_routes_installed{level,afi}` and the redistribution counters. Both
installers register the same gauge name; the metrics registry caches by name and
returns the same handle, so two registrations share one series rather than
panicking on a duplicate.

## Trap: the maximum IPv6 path metric filter is strictly greater

The per-entry filter drops a prefix metric strictly **above** the maximum. A
prefix metric exactly at the maximum passes that filter and is then dropped by
the accumulated-cost ceiling as unreachable. Both boundary cases are tested.

## Limit: single topology assumes congruent topologies

A link that carries IPv4 but not IPv6 blackholes IPv6. RFC 5120 multi-topology is
the real fix and is not implemented.
