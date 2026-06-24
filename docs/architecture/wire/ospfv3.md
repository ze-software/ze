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
