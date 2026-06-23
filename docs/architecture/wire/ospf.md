# OSPFv2 Wire Codec

The OSPFv2 wire codec lives in `internal/plugins/ospf/packet`.

Layering is deliberately the same shape as IS-IS: `types` is the leaf package,
`packet` is the serialization boundary, and runtime packages own sockets,
timers, LSDB, flooding, SPF, and RIB install. The packet codec imports only
`internal/plugins/ospf/types` and the Go standard library.

## IPv4 transport

OSPFv2 runs as IPv4 protocol 89. The Linux transport opens per-interface raw
IPv4 sockets under `internal/plugins/ospf/transport`: one receive socket joined
to AllSPFRouters (`224.0.0.5`) and AllDRouters (`224.0.0.6`) as needed, and one
transmit socket with multicast TTL 1, per-interface source selection, and
multicast loopback disabled.
<!-- source: internal/plugins/ospf/transport/backend_linux.go -- OpenInterface, setMulticastOptions, joinGroup -->

Configured interface names remain Ze logical names. The transport resolves them
through the shared iface resolver before binding sockets, so `os-name` and
`mac/match` selectors behave like the IS-IS transport.
<!-- source: internal/plugins/ospf/transport/backend_linux.go -- resolveOSPFInterface -->

Receive strips the IPv4 header by IHL and delivers source address, receiving
ifindex, and the OSPF payload bytes unchanged. Packet type dispatch remains in
the packet/runtime layer, not in the transport.
<!-- source: internal/plugins/ospf/transport/transport.go -- StripIPv4Header, RawPacket -->

## Packet framing

`DecodePacket` expects an OSPF payload beginning at the 24-byte OSPF common
header. The IP header is stripped by transport code.

The common header is validated before dispatch:

- Version must be 2.
- Type must be 1 Hello, 2 Database Description, 3 Link State Request, 4 Link
  State Update, or 5 Link State Acknowledgment.
- Packet Length must be at least 24 and no larger than the supplied slice.
- Router ID, Area ID, checksum, AuType, and the 8-byte Authentication field are
  exposed in `packet.Header`.

`Packet.WriteTo` is buffer-first. It writes the header with zero length and
checksum, writes the body, backfills Packet Length, then backfills the RFC 1071
packet checksum. For AuType 2 and 3, the checksum field remains zero so the auth
layer can sign the packet.

The packet checksum covers the full OSPF packet except header bytes 16..23, the
Authentication field. The checksum field is zero while computing.

## LSA framing

`DecodeLSA` reads the 20-byte LSA header and uses the Length field to retain the
raw LSA span. Body parsing is on demand through `DecodeRouter`, `DecodeNetwork`,
`DecodeSummary`, and `DecodeExternal`. Opaque LSA types 9, 10, and 11 are retained
as raw bytes for verbatim re-flooding.

`LSA.WriteTo` recomputes Length and the RFC 905 Fletcher checksum for constructed
LSAs. The Fletcher checksum covers `lsa[2:]`, excluding LS Age and including the
checksum field position with that field zero during generation.

In-scope bodies:

| Type | Body |
|------|------|
| 1 | Router-LSA flags plus 12-byte link records |
| 2 | Network mask plus attached router IDs |
| 3 / 4 | Summary-LSA network mask plus 24-bit metric |
| 5 | AS-External-LSA mask, E-bit, 24-bit metric, forwarding address, route tag |
| 7 | NSSA-LSA using the Type 5 body layout |
| 9 / 10 / 11 | Opaque passthrough only |

The Type 2 Network-LSA Link State ID remains the raw DR interface address in the
common LSA header. The codec does not reinterpret it as a network prefix.

## LSDB, origination, and flooding

`internal/plugins/ospf/lsdb` stores LSAs as a single owned raw-byte copy plus
the parsed 20-byte metadata header. SPF and CLI consumers parse bodies lazily;
flooding retransmits the stored raw LSA with only LS Age advanced, which
preserves the Fletcher checksum because LS Age is outside the checksum region.
<!-- source: internal/plugins/ospf/lsdb/entry.go -- Entry, Header, Raw -->

Area-scoped LSAs are held in per-area tables keyed by `(LS Type, Link State ID,
Advertising Router)`. Type 5 AS-External LSAs live in one AS-wide table and are
visible from every normal area, but are dropped on stub and NSSA interfaces.
<!-- source: internal/plugins/ospf/lsdb/lsdb.go -- LSDB, Install, LookupLSA -->
<!-- source: internal/plugins/ospf/lsdb/flooding.go -- shouldDropByArea -->

Self-origination regenerates Router-LSAs from the live interface and Full
neighbour snapshots, and originates Network-LSAs when this router is DR. RFC
6987 max-metric mode sets non-stub Router-LSA links to `0xffff` while preserving
stub-network metrics.
<!-- source: internal/plugins/ospf/lsdb/origination.go -- OriginateRouter, OriginateNetwork -->

The flooding path follows RFC 2328 Section 13: LS Updates arrive through the
packet dispatcher, newer LSAs install into the LSDB, flood to other eligible
interfaces, and queue per-neighbour retransmit entries until an LS Ack or
implicit acknowledgement clears them. The aging tick applies Section 14 MaxAge
purge retention and LSRefreshTime refresh for self-originated LSAs.
<!-- source: internal/plugins/ospf/instance.go -- handleLSUpdate, handleLSAck -->
<!-- source: internal/plugins/ospf/lsdb/flooding.go -- ReceiveUpdate, ReceiveAck, RetransmitTick -->
<!-- source: internal/plugins/ospf/lsdb/aging.go -- Tick, RefreshSelf -->

## Offline decode tool

`ze ospf-decode` reads ASCII hex or raw bytes from stdin and emits JSON for one
OSPFv2 packet. It is a codec wiring proof, not the final `show ip ospf` runtime
CLI.

Functional fixtures live in `test/ospf-wire/` and run with:

```bash
make ze-ospf-wire-test
```
