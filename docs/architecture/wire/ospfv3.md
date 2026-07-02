# OSPFv3 Wire Transport

OSPFv3 (RFC 5340) is Ze's IPv6 address family for the unified OSPF engine.
The IPv6 wire codec and raw transport stay in `internal/plugins/ospf/v3/{types,packet,transport}`
as leaf packages, while the shared engine in `internal/plugins/ospf` owns the FSM,
LSDB, flooding, SPF, origination, and redistribution policy. This page documents
the raw IPv6 transport and how it diverges from the OSPFv2 IPv4 transport.

## IPv6 transport

OSPFv3 runs as IPv6 protocol 89 with link-local source addresses and the
multicast groups `ff02::5` (AllSPFRouters) and `ff02::6` (AllDRouters). The Linux
transport opens one `golang.org/x/net/ipv6.PacketConn` per interface over a raw
`ip6:89` socket, sets multicast hop limit 1 (link-local scope), per-interface
egress, and multicast loopback off, and requests the destination, interface, and
hop-limit control messages.
<!-- source: internal/plugins/ospf/v3/transport/backend_linux.go -- OpenInterface, setupSocket -->

Configured interface names remain Ze logical names. The transport resolves them
through the shared iface resolver before binding sockets, so `os-name` and
`mac/match` selectors behave like the OSPFv2 and IS-IS transports. The interface
IPv6 link-local (`fe80::/10`) address is selected as the source; if it is not yet
ready (IPv6 Duplicate Address Detection), the open is marked pending and retried
by the periodic rescan and on `interface/addr-added`.
<!-- source: internal/plugins/ospf/v3/transport/backend_linux.go -- resolveOSPFv3Interface, interfaceLinkLocal -->

Unlike the OSPFv2 IPv4 raw socket, an IPv6 raw socket delivers the upper-layer
payload with **no IP header to strip**. The receiving source comes from
`ReadFrom`; the destination group, receiving ifindex, and hop limit come from the
ancillary control message. The transport delivers `(ifindex, src, dst, hopLimit,
payload)` upward.
<!-- source: internal/plugins/ospf/v3/transport/transport.go -- RawPacket, rxLoop -->

## Address-bound checksum

The OSPFv3 packet checksum is the IPv6 upper-layer checksum over a pseudo-header
that includes the IPv6 source and destination (RFC 5340 §A.3.1), so the codec's
`Packet.WriteTo` leaves the checksum field zero. The transport is the only layer
that knows the on-wire source, so on transmit it finalizes the checksum from the
egress link-local source and destination, and binds that same source as the
explicit send source (`ControlMessage.Src`) so the on-wire source provably equals
the pseudo-header source. When an RFC 7166 Authentication Trailer signer is
installed the checksum is left zero instead (the trailer covers integrity).
<!-- source: internal/plugins/ospf/v3/transport/transport.go -- SendPacket -->
<!-- source: internal/plugins/ospf/v3/packet/checksum.go -- FinalizePacketChecksum, PacketChecksum, VerifyPacketChecksum -->

On receive, checksum verification, version, area, and RFC 7166 validation are
owned by the shared OSPF dispatcher through the OSPFv3 codec, which verifies over `payload[:Length]`
(the checksum covers the OSPF packet length, not any trailing bytes). The
transport performs exactly one header-level check: the Instance ID demux
(RFC 5340 §4.2.1) drops a packet whose Instance ID (common-header byte 14) does
not match the receiving interface's configured Instance ID.
<!-- source: internal/plugins/ospf/v3/transport/transport.go -- rxLoop -->
<!-- source: internal/plugins/ospf/v3/packet/header.go -- PeekInstanceID -->

## Multiple address families (RFC 5838)

RFC 5838 carries several address families over the one OSPFv3 wire by mapping each to an
Instance-ID range (§2.1: IPv6-unicast 0-31, IPv6-multicast 32-63, IPv4-unicast 64-95,
IPv4-multicast 96-127) and tagging its Hello/DD Options with the **AF-bit** (§2.4, bit 8 /
`0x000100` of the 24-bit Options). The wire format is otherwise unchanged: the same 16-byte
common header, the same five packet types, the same scope-aware LSA model. Ze runs one engine
instance per configured AF, each with its own `dispatcher.instanceID`, so the existing §4.2.2
Instance-ID demux (drop on mismatch) routes a datagram to the right instance without any
multi-value match. The default IPv6-unicast AF emits the AF-bit only when the router is
multi-AF-aware, so a lone IPv6-unicast instance is byte-identical to the IPv6 base; a legacy
peer that omits the AF-bit still forms the default-AF adjacency (§2.6), while a non-default AF
requires it (§2.5). An IPv4 prefix on an IPv4-AF instance rides the RFC 5340 §A.4.1 prefix
encoding as a single 32-bit word (§2.7).
<!-- source: internal/plugins/ospf/v3/types/options.go -- OptAF -->
<!-- source: internal/plugins/ospf/multiaf.go -- afFromInstanceID, afHelloAccepted -->

## Membership

AllSPFRouters (`ff02::5`) is joined on every enabled interface; AllDRouters
(`ff02::6`) is joined only when the router is DR or BDR on that interface and left
on losing the role or on interface teardown (RFC 5340 §2.9).
<!-- source: internal/plugins/ospf/v3/transport/backend_linux.go -- joinLeave -->

## Hop limit and interop

OSPFv3 sends multicast and unicast packets with IPv6 Hop Limit 1, matching FRR
`ospf6d`, which drops received multicast OSPFv3 packets whose hop limit is not 1
(RFC 5340 Appendix A.1). Base OSPFv3 does not use the GTSM hop-limit-255 check;
the link-local source confines scope on the link. The received hop limit is
carried upward so a later policy check needs no transport change.
<!-- source: internal/plugins/ospf/v3/transport/backend_linux.go -- Send, deliver -->

## Segment Routing (RFC 8666)

The IPv6 family carries Segment Routing over the RFC 8362 Extended LSAs, added as
the SR subset needed by RFC 8666. The SR control plane (SRGB/SRLB, index-to-label
arithmetic, the NP/E/M push/swap/PHP decision, and the `mpls-fib` install) is the
same shared code both address families use; only the wire carriage differs.

**SR capabilities** ride the OSPFv3 Router Information Opaque LSA (RFC 7770,
area `0xA00C` / AS `0xC00C`): the SR-Algorithm, SID/Label Range (SRGB), SR Local
Block (SRLB) and SRMS-Preference TLVs reuse the RFC 8665 encodings unchanged
(RFC 8666 §4). Origination hangs off `v6OriginateRI`; reception reads them through
`LSAViewsByType` in `srRemoteCapabilities`.

**Prefix-SIDs and Adj-SIDs** ride the RFC 8362 Extended LSAs, framed by
`v3/packet/lsa_extended.go` (`EncodeExtendedLSABody` / `SubTLVsAt` /
`AppendSubTLVs`) with the RFC 8666 sub-TLV value codecs in `sr/codec_v6.go`:

| Element | Carrier LSA | Parent TLV | Sub-TLV (RFC 8666 type) |
|---------|-------------|------------|--------------------------|
| Prefix-SID (node loopback) | E-Intra-Area-Prefix-LSA `0x2029` | Intra-Area-Prefix TLV (RFC 8362 type 6) | Prefix-SID (4) |
| Inter-area Prefix-SID (ABR) | E-Inter-Area-Prefix-LSA `0x2023` | Extended Prefix Range TLV (9) | Prefix-SID (4) |
| Adj-SID / LAN-Adj-SID | E-Router-LSA `0x2021` | Router-Link TLV (RFC 8362 type 1) | Adj-SID (5) / LAN-Adj-SID (6) |

The SID/Index/Label width is inferred from the V/L flag pair (4-octet index for
V=0/L=0, 3-octet local label for V=1/L=1); every other combination is ignored. The
Explicit NULL label is 2 for IPv6 (0 for IPv4). The E-LSA types are registered in
`v6ManagedSelfTypes`, so the self-LSA stale flush refreshes them while SR is
enabled and MaxAge-purges them when SR is disabled or a SID is withdrawn. Reception
(`srRemotePrefixSIDsV6`) parses the E-prefix LSAs into a per-prefix Prefix-SID map
that the shared post-SPF install driver (`sr_install.go`) turns into `mpls-fib`
push/swap entries toward the SPF next-hop. An ABR re-advertises a learned
intra-area Prefix-SID into the other areas with NP set and E clear (RFC 8666 §8.2).
Inter-area propagation, malformed carriage, and the index-to-label math are
bound-checked and never panic (RFC 8666 §11).
<!-- source: internal/plugins/ospf/sr_origination_v6.go, sr_reception_v6.go, sr_interarea_v6.go; internal/plugins/ospf/v3/packet/lsa_extended.go; internal/plugins/ospf/sr/codec_v6.go -->
