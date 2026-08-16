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

### Authentication pipeline

OSPFv2 signs all five outgoing packet types at the transport signer. It verifies
each received packet in the dispatcher before an ISM, NSM, or LSDB handler runs.
The signer rewrites AuType and checksum framing before it applies the configured
RFC 5709 or RFC 7474 digest and replay sequence.
<!-- source: internal/plugins/ospf/auth_wiring.go -- installAuthHooks, signPacket, verifyPacket -->
<!-- source: internal/plugins/ospf/dispatcher.go -- dispatcher.dispatch -->

## LSA framing

`DecodeLSA` reads the 20-byte LSA header and uses the Length field to retain the
raw LSA span. Body parsing is on demand through `DecodeRouter`, `DecodeNetwork`,
`DecodeSummary`, and `DecodeExternal`. Opaque LSA types 9, 10, and 11 (RFC 5250)
are retained as raw bytes for verbatim re-flooding. For an opaque LSA the 32-bit
Link State ID splits into an 8-bit Opaque Type and a 24-bit Opaque ID, read with
`LSA.OpaqueType()` / `LSA.OpaqueID()` (or `packet.OpaqueTypeOf` / `OpaqueIDOf`), and
composed with `packet.OpaqueLinkStateID`; the split lives in the codec, never in the
LSDB key. `packet.OpaqueTLV` / `OpaqueTLVIterator` carry a consumer's opaque body as
4-byte-aligned type-length-value triples (buffer-first emit, zero-copy bound-checked
iteration). On top of that iterator, the RFC 7684 Extended Prefix (Opaque type 7) and
Extended Link (Opaque type 8) bodies are coded by `packet.EncodeExtPrefixLSA` /
`DecodeExtPrefixLSA` and `EncodeExtLinkLSA` / `DecodeExtLinkLSA`: an Extended Prefix TLV is
an 8-octet fixed header (Route Type, Prefix Length, AF, Flags, then a 32-bit Address Prefix
for AF 0 regardless of Prefix Length) followed by nested sub-TLVs; an Extended Link TLV is a
12-octet fixed header (Link Type, 3-octet Reserved, Link ID, Link Data mirrored from the
Router-LSA link) followed by nested sub-TLVs. A TLV or sub-TLV overrunning its parent, or
trailing data smaller than a header, is reported as an error (RFC 7684 §5), never a panic.
Opaque capability is advertised with the O-bit (`types.OptionO`) in the
OSPF Options field of Database Description packets only; a router floods opaque LSAs
only to neighbours whose DD carried the O-bit and ignores the O-bit outside DD.

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

## Traffic Engineering LSA body (RFC 3630, RFC 5392)

`packet.TELSA` / `DecodeTELSA` code the Traffic Engineering LSA body carried inside a
Type 10 (or, for inter-AS, Type 10/11) opaque LSA, built on the `OpaqueTLV` framing.
A TE LSA carries exactly one top-level TLV: the Router Address TLV (type 1, a 4-octet
IPv4 address) or the Link TLV (type 2). The Link TLV nests the RFC 3630 §2.5 sub-TLVs:
Link Type (1, one octet: 1 point-to-point, 2 multi-access), Link ID (2, 4 octets),
Local (3) and Remote (4) Interface IP (4N octets), TE Metric (5, uint32), Maximum (6)
and Maximum-Reservable (7) Bandwidth, Unreserved Bandwidth (8, eight values priority 0
first through 7 last), and Administrative Group (9, a 32-bit mask, LSB = group 0). The
three bandwidth sub-TLVs encode 32-bit IEEE-754 single-precision **bytes per second**
(not bits/sec, not integers); `TELink` stores them as `float64`. Every TLV is padded to
a 4-octet boundary with the pad excluded from the Length field (RFC 3630 §2.3.2).

RFC 5392 (Opaque type 6) adds no top-level TLV: the Link TLV carries the Remote AS
Number sub-TLV (21, 4 octets, a 2-byte ASN zero-extended into the high 16 bits), the
IPv4 Remote ASBR ID (22, 4 octets), and the IPv6 Remote ASBR ID (24 -- **not** 23 --
16 octets); the Link ID sub-TLV is prohibited (§3.2.1). `DecodeTELSA` uses the
bound-checked `OpaqueTLVIterator` and never panics on a malformed body or sub-TLV,
returning an error instead (fuzzed by `FuzzOSPFTEBody`).
<!-- source: internal/plugins/ospf/packet/te_lsa.go -- TELSA, TELink, DecodeTELSA, Encode -->
<!-- source: internal/plugins/ospf/packet/te_interas.go -- appendInterAsSubTLVs, parseInterAsSubTLV -->

## Router Information LSA body (RFC 7770)

The Router Information (RI) LSA carries the same 4-byte-aligned TLV stream in both address
families; `packet.EncodeRITLVs` / `DecodeRITLVStream` code it over the generic `OpaqueTLV`
framing. Its carriage differs: OSPFv2 uses an Opaque LSA with Opaque type 4 (the 24-bit
Opaque ID is the RI Instance ID, Instance 0 = LS ID `4.0.0.0`); OSPFv3 uses a native LSA
with function code 12 and the U-bit set, so the 16-bit LS Type is `0x800C` link, `0xA00C`
area, `0xC00C` AS (`ospfv3/types.LSTypeRouterInformation*`). The U-bit (RFC 5340 §A.4.2.1)
makes a non-supporting router still flood the area/AS LSA rather than confine it to link-local
scope; Ze recognizes a received RI LSA by function code regardless of the U-bit.

The body's first TLV in Instance 0 is the Router Informational Capabilities TLV (type 1, a
4-octet capability word, bits numbered MSB = bit 0: 0 GR-capable, 1 GR-helper, 2 stub-router,
3 TE); a Functional Capabilities TLV (type 2) is carried empty. `RICapabilitiesValue` /
`RIReadCapabilities` encode and decode the word. Registered downstream TLVs follow in
ascending TLV-type order and overflow into subsequent Instance IDs (RFC 7770 §3); a receiver
uses the smallest Instance ID for an unspecified-multi-instance TLV. `DecodeRITLVStream` uses
the bound-checked iterator and returns an error on a malformed body rather than panicking. An
AS-scope OSPFv3 RI LSA (0xC00C) shares the AS-wide store with AS-External LSAs but is
distinguished by function code: `LSType.ASWide()` routes it AS-wide while `LSType.ASExternal()`
(function code 5 only) keeps it out of SPF and the route table.
<!-- source: internal/plugins/ospf/packet/ri_tlv.go -- RITLV, EncodeRITLVs, DecodeRITLVStream, RICapabilitiesValue, RIReadCapabilities -->
<!-- source: internal/plugins/ospf/v3/types/lsa.go -- LSTypeRouterInformationArea, RIFunctionCode, Known -->
<!-- source: internal/plugins/ospf/types/lstype.go -- ASWide, ASExternal -->

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

`ze ospf decode` reads ASCII hex or raw bytes from stdin and emits JSON for one
OSPFv2 packet. It is a codec wiring proof, not the final `show ospf` runtime
CLI.

Functional fixtures live in `test/ospf-wire/` and run with:

```bash
make ze-functional-ospf-wire-test
```
