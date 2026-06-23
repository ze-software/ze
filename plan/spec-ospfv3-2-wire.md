# Spec: ospfv3-2-wire

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospfv3-0-umbrella.md, spec-ospfv3-1-types.md |
| Phase | follow-up 2/13 |
| Updated | 2026-06-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `plan/spec-ospfv3-0-umbrella.md` for the OSPFv3 scope, package layout, and child dependency graph.
3. `plan/spec-ospfv3-1-types.md` for the leaf type API this codec consumes.
4. `rfc/short/rfc5340.md` for the OSPFv3 header, packet types, LSA formats, and prefix encoding.
5. `rfc/short/rfc2328.md` for the OSPFv2 base formats RFC 5340 modifies (Fletcher LSA checksum, DD/LSReq/LSUpdate/LSAck shapes).
6. `internal/plugins/ospf/packet/*.go` for the buffer-first `WriteTo(buf, off) int` and lazy-decode patterns to mirror (not import).
7. `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`, `ai/rules/no-sprintf-alloc.md`.

## Task

Create the OSPFv3 packet and LSA codec leaf package `internal/plugins/ospfv3/packet/`.
This is the second follow-up spec from `spec-ospfv3-0-umbrella.md` and the wire
foundation for the transport, FSM, LSDB, SPF, auth, and CLI specs. The package
encodes and decodes the RFC 5340 common header, the five packet types, the LSA
header, the eight base LSA bodies, the IPv6 prefix encoding, and the two
checksums. It consumes `internal/plugins/ospfv3/types` for every scalar and key.

The codec is pure wire code: no sockets, timers, goroutines, config, LSDB maps,
or route installation. It mirrors the OSPFv2 codec's conventions (buffer-first
`WriteTo(buf, off) int`, lazy/zero-copy LSA decode, explicit length validation
before slicing, typed sentinel errors) but shares no code with it, because the
OSPFv3 wire contract differs: a 16-byte header with an Instance ID and no AuType
fields, an IPv6 upper-layer (pseudo-header) packet checksum, 24-bit Options,
address-free Router-LSAs and Network-LSAs, separate Link-LSA and
Intra-Area-Prefix-LSA carriers, and RFC 5340 IPv6 prefix encoding.

## Required Reading

### Architecture Docs

- [ ] `plan/spec-ospfv3-0-umbrella.md` - OSPFv3 package layout and child dependency graph
  -> Constraint: `packet` is a leaf codec under `internal/plugins/ospfv3/packet/`; transport/FSM/LSDB import it, it imports only `internal/plugins/ospfv3/types` and stdlib.
- [ ] `plan/spec-ospfv3-1-types.md` - OSPFv3 leaf type API
  -> Constraint: every wire field maps to a `types` value (`LSType`, `Options`, `PrefixLength`, `PrefixOptions`, `RouterID`, `AreaID`, `LinkStateID`, `InstanceID`, `InterfaceID`, `Metric`, `LSSequenceNumber`, `LSAge`, `LSAKey`); do not redeclare them.
- [ ] `plan/spec-ospf-2-wire.md` - OSPFv2 packet/LSA codec pattern
  -> Constraint: mirror the `WriteTo(buf, off) int`, `EncodedLen()`, lazy `RawBytes` retention, skip-and-backfill, and decode-bounds conventions; do not import or alias OSPFv2 packet code.
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - encode/decode and allocation discipline
  -> Constraint: encode writes into caller-owned buffers; decode validates length before slicing; the hot path (flood re-encode) reuses retained raw bytes, no re-marshal.
- [ ] `ai/rules/no-sprintf-alloc.md` - allocation-light formatting
  -> Constraint: no `fmt.Sprintf` in encode/decode or LSA key/string helpers.
- [ ] `ai/rules/rfc-compliance.md` - RFC comment discipline
  -> Constraint: every enforced MUST carries a `// RFC 5340 Section X.Y: "quote"` comment above the enforcing code.

### RFC Summaries

- [ ] `rfc/short/rfc5340.md` - OSPFv3 base protocol
  -> Constraint: 16-byte common header (Version 3, Type, Packet Length, Router ID, Area ID, Checksum, Instance ID, Reserved); the OSPFv2 AuType/Authentication 8 octets are removed.
  -> Constraint: the packet checksum is the IPv6 upper-layer checksum over the IPv6 pseudo-header (src, dst, upper-layer length, Next Header 89) plus the OSPF packet; it is NOT the OSPFv2 over-the-packet checksum. Checksum encode/verify therefore takes the IPv6 source and destination from transport.
  -> Constraint: the LSA header is 20 octets with a 16-bit LS Type and no Options byte; the LSA body checksum remains the Fletcher checksum over the LSA excluding LS Age, unchanged from OSPFv2, and must not be zero.
  -> Constraint: Router-LSA and Network-LSA carry NO IP addresses or network masks; addresses live in Link-LSA and Intra-Area-Prefix-LSA.
  -> Constraint: prefix encoding is PrefixLength (1) + PrefixOptions (1) + a type-specific 16-bit field + `((PrefixLength+31)/32)*4` address bytes with zero padding past the prefix length.
- [ ] `rfc/short/rfc2328.md` - OSPFv2 base the formats modify
  -> Constraint: DD (I/M/MS flags 0x04/0x02/0x01 + DD sequence), LSReq, LSUpdate (LSA count + LSAs), and LSAck (LSA headers) keep their OSPFv2 shape except for OSPFv3 field widths, reordering, and the absent network mask.
- [ ] `rfc/short/rfc7166.md` - OSPFv3 authentication trailer (awareness only)
  -> Constraint: this codec does not sign/verify; it must leave room for an appended Authentication Trailer (the AT-bit in Options, the trailer after the packet body) so `spec-ospfv3-12-auth.md` can wrap the encoded bytes. Packet Length excludes the trailer; when the trailer is present the packet (header) checksum computation is omitted (RFC 7166 §2.2).

**Key insights:**
- The OSPFv3 packet checksum needs the IPv6 pseudo-header, so unlike OSPFv2 the checksum functions take the source and destination IPv6 addresses. Transport supplies them; the codec must not invent or cache them. This is the single biggest divergence from the OSPFv2 codec.
- LSA identity and freshness already live in `types` (`LSAKey`, `LSSequenceNumber`, `LSAge`); the codec frames bytes and builds a `types.LSAKey{Type, LinkStateID, AdvertisingRouter}` from the decoded header, never re-implementing comparison.
- Lazy decode matters for flooding: a received LSA is stored and re-flooded by raw bytes; the codec must retain `RawBytes` and re-encode without re-marshalling the typed body. Unknown LS Types are retained as opaque spans for verbatim re-flood.
- The repo ships RFC *summaries*, not full RFC 5340/2328/7166 text. Two layouts the summary does not pin to the bit (AS-External §A.4.7 flag/Referenced-LS-Type packing and its conditional trailing-field order) must be confirmed against an FRR/Wireshark OSPFv3 capture (R-6 / A-5).

## Current Behavior

**Source files read:**
- [ ] `internal/plugins/ospfv3/types/*.go` - the leaf types this codec consumes.
  -> Constraint: use `LSType.WriteTo`/`Scope`/`FunctionCode`/`UBit`, `LSTypeFromBytes`, `Options.WriteTo`, `PrefixLength.ByteLen/WordLen/ValidatePadding`, `RouterID/AreaID/LinkStateID/InterfaceID/InstanceID/Metric/LSAge.WriteTo`, and the `LSAKey` struct; add no duplicate scalar types here.
- [ ] `internal/plugins/ospf/packet/header.go`, `lsa.go`, `hello.go`, `dbdesc.go`, `lsupdate.go`, `checksum.go`, `wire.go` - the OSPFv2 codec pattern to mirror.
  -> Constraint: reuse the structural shape (Packet struct, `WriteTo`/`EncodedLen`, `DecodeHeader`, `LSAIterator`, Fletcher checksum, fixed-width read/write helpers) but not the code, field widths, or the 24-byte header.
- [ ] `internal/plugins/ospf/types/checksum.go` - OSPFv2 Fletcher and Internet checksum helpers.
  -> Constraint: the LSA Fletcher checksum (covered range `lsa[2:length]`, result non-zero) is identical; the packet checksum differs (IPv6 pseudo-header). Do not share; OSPFv3 owns its own copy under `ospfv3/packet` per the no-cross-version rule.

**Behavior to preserve:**
- OSPFv2 codec under `internal/plugins/ospf/packet/` is unchanged and not imported.
- `internal/plugins/ospfv3/types` keeps its API; this spec only consumes it.
- No transport, FSM, LSDB, SPF, config, CLI, web, or metrics behavior changes.

**Behavior to change:**
- Add `internal/plugins/ospfv3/packet/` with the header, five packet types, LSA header, eight LSA bodies, prefix encoding, and both checksums, plus unit and boundary tests.
- Add no production imports from OSPFv3 runtime packages (transport/FSM/LSDB do not exist yet); the wiring tests that exercise this codec end-to-end are owned by those later specs.

## Wire Format

### Common header (16 octets, RFC 5340 §A.3.1)

| Offset | Width | Field | Type |
|--------|-------|-------|------|
| 0 | 1 | Version (= 3) | const |
| 1 | 1 | Type (1..5) | `PacketType` |
| 2 | 2 | Packet Length (header + body, excludes auth trailer) | uint16 |
| 4 | 4 | Router ID | `types.RouterID` |
| 8 | 4 | Area ID | `types.AreaID` |
| 12 | 2 | Checksum (IPv6 upper-layer, 0 while computing) | uint16 |
| 14 | 1 | Instance ID | `types.InstanceID` |
| 15 | 1 | Reserved (0) | const |

`CommonHeaderLen = 16`. Removed vs OSPFv2: the 8-octet AuType + Authentication field; the two reclaimed octets are Instance ID + Reserved.

### Packet types (RFC 5340 §A.3.2-A.3.6)

| Type | Name | Body layout (offsets body-relative) | Differences from OSPFv2 |
|------|------|--------------------------------------|--------------------------|
| 1 | Hello | InterfaceID(4)@0, RtrPri(1)@4, Options(3)@5, HelloInterval(2)@8, RouterDeadInterval(2)@10, DR(4)@12, BDR(4)@16, NeighborIDs(4×N)@20 | 20-byte fixed prefix; Interface ID replaces network mask; Options 24-bit; DeadInterval is 2 octets (was 4); DR/BDR are Router IDs (not addresses) |
| 2 | Database Description | Reserved(1)@0, Options(3)@1, MTU(2)@4, Reserved(1)@6, Flags I/M/MS(1)@7, DDSeq(4)@8, LSAHeaders(20×N)@12 | 12-byte fixed prefix; fields reordered; Options 24-bit; I=0x04 M=0x02 MS=0x01 unchanged; MTU=0 over virtual links |
| 3 | Link State Request | per 12-byte entry: Reserved(2)@0, LSType(2)@2, LinkStateID(4)@4, AdvRouter(4)@8 | LS Type is genuinely 16-bit in a 32-bit slot (leading 2 reserved octets) |
| 4 | Link State Update | #LSAs(4)@0, LSAs@4 (each Length-driven) | LSAs use the 20-octet OSPFv3 LSA header |
| 5 | Link State Acknowledgment | LSA headers (20×N) | LSA header width 20, LS Type 16-bit |

### LSA header (20 octets, RFC 5340 §A.4.2.1)

| Offset | Width | Field | Type |
|--------|-------|-------|------|
| 0 | 2 | LS Age (DoNotAge bit 0x8000) | `types.LSAge` |
| 2 | 2 | LS Type (U-bit 0x8000 + S2/S1 scope 0x6000 + 13-bit function 0x1fff) | `types.LSType` |
| 4 | 4 | Link State ID | `types.LinkStateID` |
| 8 | 4 | Advertising Router | `types.RouterID` |
| 12 | 4 | LS Sequence Number (signed; Initial 0x80000001, Max 0x7fffffff, reserved 0x80000000) | `types.LSSequenceNumber` |
| 16 | 2 | LS Checksum (Fletcher over `lsa[2:length]`, non-zero) | uint16 |
| 18 | 2 | Length (entire LSA) | uint16 |

`LSAHeaderLen = 20`. There is no Options byte in the OSPFv3 LSA header (the OSPFv2 Options@2 + 8-bit Type@3 become the single 16-bit LS Type@2). `LSAKey` = `types.LSAKey{Type, LinkStateID, AdvertisingRouter}` built from the decoded header (LS Type via `types.LSTypeFromBytes`). The Fletcher checksum and its coverage are unchanged from OSPFv2. Scope decoding (`LSType.Scope()`): `00` link-local, `01` area, `10` AS, `11` reserved; U-bit (`LSType.UBit()`) governs unknown-LSA flooding.

### Base LSA bodies (RFC 5340 §A.4.3-A.4.10, offsets body-relative)

| LS Type | Name | Body |
|---------|------|------|
| `0x2001` | Router-LSA | Flags(1)@0 (W=0x08,V=0x04,E=0x02,B=0x01) + Options(3)@1 + N×16-byte link records {Type(1), reserved(1), Metric(2), InterfaceID(4), NeighborInterfaceID(4), NeighborRouterID(4)}. No #links field (derive N from Length). NO IP addresses, no #TOS |
| `0x2002` | Network-LSA | reserved(1)@0 + Options(3)@1 + attached RouterIDs(4×N)@4. NO network mask; the header's Link State ID is the DR's Interface ID (preserve verbatim) |
| `0x2003` | Inter-Area-Prefix-LSA | reserved(1)@0 + Metric(3)@1 + PrefixLength(1)@4 + PrefixOptions(1)@5 + Reserved(2)@6 + AddressPrefix(ByteLen)@8 |
| `0x2004` | Inter-Area-Router-LSA | reserved(1)@0 + Options(3)@1 + reserved(1)@4 + Metric(3)@5 + DestinationRouterID(4)@8 (fixed 12 bytes) |
| `0x4005` | AS-External-LSA | Flags E/F/T(1)@0 (low 3 bits; shares the 32-bit word with the Metric) + Metric(3)@1 + PrefixLength(1)@4 + PrefixOptions(1)@5 + ReferencedLSType(2, 16-bit)@6 + AddressPrefix(ByteLen)@8 + ForwardingAddr(16, iff F) + ExternalRouteTag(4, iff T) + ReferencedLinkStateID(4, iff Referenced LS Type != 0). U-bit set (AS scope) |
| `0x2007` | NSSA-LSA | body byte-identical to AS-External; LS Type differs (U=0, area scope). The P-bit lives in PrefixOptions (`0x08`), not a header Options byte |
| `0x0008` | Link-LSA | RtrPri(1)@0 + Options(3)@1 + LinkLocalInterfaceAddress(16)@4 + #prefixes(4)@20 + prefix list@24 (link-local scope; never flood off-link) |
| `0x2009` | Intra-Area-Prefix-LSA | #prefixes(2)@0 + ReferencedLSType(2)@2 + ReferencedLinkStateID(4)@4 + ReferencedAdvRouter(4)@8 + prefix list@12 (each prefix's 16-bit field carries a Metric) |

### IPv6 prefix encoding (RFC 5340 §A.4.1)

Two carriage forms over `types.PrefixLength`/`PrefixOptions`:
- **Inlined** (Inter-Area-Prefix, AS-External, NSSA): the LSA lays `PrefixLength(1) + PrefixOptions(1) + a 16-bit field` individually in its fixed part, then `AddressPrefix` of `types.PrefixLength.ByteLen()` bytes.
- **Repeating entry** (Link-LSA, Intra-Area-Prefix): each prefix is a 4-octet header `PrefixLength(1) + PrefixOptions(1) + 16-bit field(2)` then `AddressPrefix(ByteLen)`. The 16-bit field is the prefix **Metric** in Intra-Area-Prefix-LSA and **Reserved (0)** in Link-LSA.

`AddressPrefix` length = `types.PrefixLength.ByteLen()` = `((PrefixLength+31)/32)*4`; bits past the prefix length MUST be zero (`types.PrefixLength.ValidatePadding`). Byte counts: /0 -> 0, /1..32 -> 4, /33..64 -> 8, /65..96 -> 12, /97..128 -> 16. Expose one `decodePrefix`/`encodePrefix` helper used by both forms. PrefixOptions bits: NU=0x01, LA=0x02, P=0x08, DN=0x10.

### Checksums

- **Packet checksum** (header offset 12): the IPv6 upper-layer checksum (one's-complement RFC 1071 sum) over the pseudo-header (src IPv6 16, dst IPv6 16, 32-bit upper-layer length = Packet Length, 3 zero octets, Next Header = 89) followed by the OSPF packet with the checksum field zeroed. Encode and verify take the source and destination IPv6 addresses from transport. When an RFC 7166 Authentication Trailer is present this computation is omitted (RFC 7166 §2.2).
- **LSA checksum** (LSA header offset 16): the Fletcher checksum over `lsa[2:length]` (LS Age excluded), identical to OSPFv2, and must not be zero.

## Data Flow

### Entry Point

- Wire bytes: raw IPv6 proto-89 payloads handed to `DecodeHeader` / `DecodePacket` by `spec-ospfv3-3-ipv6-transport.md`.
- Typed state: FSM/LSDB build `Packet` and `LSA` values and call `WriteTo` to emit bytes for transport.
- Checksum inputs: transport supplies the IPv6 source/destination for the packet checksum.

### Transformation Path

1. Decode the 16-octet common header: validate Version 3, Type 1..5, Packet Length within the datagram; expose Router ID, Area ID, Instance ID, Checksum.
2. Verify the IPv6 upper-layer checksum using transport-supplied addresses (cold validation path; skipped when an auth trailer covers integrity).
3. Decode the type-specific body into a typed `Packet` (Hello/DD/LSReq/LSUpdate/LSAck), validating each fixed field width and any repeating-record count against the remaining length.
4. For LSUpdate/LSAck, iterate LSAs via `LSAIterator`: each LSA decodes its 20-octet header (building `types.LSAKey`) and retains `RawBytes` for lazy re-flood; the typed body is decoded on demand; unknown LS Types are kept as opaque spans.
5. Encode is the inverse: `EncodedLen()` then `WriteTo(buf, off)` lays every field big-endian into a caller buffer; the packet checksum is computed last over the assembled bytes plus the pseudo-header.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Wire bytes -> header | `DecodeHeader` validates version/type/length before slicing | `TestOSPFv3DecodeHeaderBounds` |
| Wire bytes -> packet body | each packet decoder checks record counts against remaining length | `TestOSPFv3HelloRoundTrip`, `TestOSPFv3DBDescRoundTrip`, ... |
| Wire bytes -> LSA | `LSAIterator` checks LSA Length within the buffer; body decode validates per-LSA | `TestOSPFv3LSAIteratorBounds` |
| Prefix bytes -> typed prefix | `ByteLen`/`ValidatePadding` reject short buffers and non-zero padding | `TestOSPFv3WirePrefixBoundaries` |
| Typed packet -> wire | `WriteTo` writes exact big-endian widths into a caller buffer | `TestOSPFv3PacketWriteTo` |
| Packet -> checksum | IPv6 pseudo-header checksum over packet + addresses | `TestOSPFv3PacketChecksum` |
| LSA -> checksum | Fletcher over LSA excluding LS Age | `TestOSPFv3LSAChecksum` |

### Integration Points

- `spec-ospfv3-3-ipv6-transport.md` calls `DecodeHeader`/`DecodePacket` on receive and `WriteTo` on send, and supplies the IPv6 addresses for the checksum.
- `spec-ospfv3-5-interface-ism.md` builds and parses Hello packets.
- `spec-ospfv3-6-neighbor-nsm.md` builds and parses DD, LSReq, LSUpdate, LSAck.
- `spec-ospfv3-7-lsdb-flooding.md` stores `LSA.RawBytes`, re-floods without re-encoding, and reads `LSAKey`.
- `spec-ospfv3-8-spf-rib.md` decodes Router-LSA, Network-LSA, and Intra-Area-Prefix-LSA bodies.
- `spec-ospfv3-12-auth.md` appends/strips the RFC 7166 Authentication Trailer around the encoded bytes; Packet Length excludes it.

### Architectural Verification

- [ ] No bypassed layers: transport/FSM/LSDB all frame bytes through this codec, never ad hoc.
- [ ] No unintended coupling: the package imports only `internal/plugins/ospfv3/types` + stdlib; no OSPFv2, transport, LSDB, SPF, config, or plugin package.
- [ ] No duplicated functionality: no scalar type re-declared; the LSA Fletcher checksum has one OSPFv3 owner.
- [ ] Zero-copy preserved: decode retains `RawBytes`; re-flood encodes from retained bytes; encode writes into caller buffers.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | The `types` leaf API is sufficient for every wire field without a new scalar | spec-ospfv3-1-types domain table | the codec must add a leaf type or amend `types` | the codec compiles against `types` with no local scalar duplicates | unvalidated |
| A-2 | The packet checksum needs the IPv6 source/destination from transport | RFC 5340 §A.3.1 (IPv6 upper-layer checksum) | a checksum computed without the pseudo-header fails FRR interop | `TestOSPFv3PacketChecksum` matches an FRR-captured vector | unvalidated |
| A-3 | Lazy `RawBytes` retention lets flooding re-emit a received LSA byte-for-byte | spec-ospf-2-wire lazy-decode pattern | flooding re-marshals and changes bytes/age handling | `TestOSPFv3LSARawBytesRoundTrip` re-encodes equal to the input | unvalidated |
| A-4 | The RFC 7166 Authentication Trailer can wrap the encoded bytes without changing the codec API now | RFC 7166 / umbrella auth child | the trailer needs codec changes, not just an append | `spec-ospfv3-12-auth.md` appends a trailer over `WriteTo` output without editing the codec | unvalidated |
| A-5 | AS-External/NSSA optional fields (Forwarding Address, Tag, Referenced LS ID) are gated by F/T flags and a non-zero Referenced LS Type, in that fixed order | RFC 5340 §A.4.7 (summary-silent on exact bit packing) | optional-field presence is misread and the body length drifts | `TestOSPFv3ExternalOptionalFields` round-trips each flag combination + an FRR capture vector | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Carrying an OSPFv2 codec assumption (24-byte header, network mask, over-the-packet checksum) into OSPFv3 | self round-trip passes, FRR interop fails | the wire-difference table is the checklist; cross-check each field against rfc5340.md |
| R-2 | The IPv6 pseudo-header checksum is computed wrong (length, next-header 89, zeroed field, odd-length pad) | FRR drops our packets silently | match against a captured FRR vector before the FSM specs build on it |
| R-3 | Variable-length prefix records mis-size the body and over/under-read | decode panics on a crafted LSA, or trailing bytes leak | bound every prefix read with `ByteLen` and validate against the LSA Length; boundary tests at /0,/1,/64,/127,/128 |
| R-4 | Optional AS-External fields read in the wrong order | external routes decode with a wrong forwarding address/tag | encode/decode the optional fields in RFC order, gated by E/F/T and the referenced LS Type, with a per-combination test |
| R-5 | Re-encoding for flood diverges from the received bytes | flooded LSA checksum mismatches on a third router | retain and re-emit `RawBytes`; only re-marshal self-originated LSAs |
| R-6 | The repo ships RFC summaries, not full RFC 5340 text; an internally-consistent but wire-wrong AS-External packing round-trips green undetected (this happened in the first implementation: flags were placed at body offset 6 and Referenced LS Type truncated to 8 bits, caught only by review) | a self-consistent encoding passes every round-trip test while FRR misparses it | RESOLVED: the codec now follows RFC 5340 §A.4.7 (flags in byte 0 sharing the Metric word; 16-bit Referenced LS Type at offset 6), locked by a hardcoded golden vector (`TestOSPFv3ExternalGoldenWire`). FRR-capture validation is still scheduled with the interop specs as defense, but the layout is no longer a guess. Lesson: every wire codec needs at least one golden vector, not only round-trips |

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ospfv3-3-transport` decodes a received datagram | -> | `packet.DecodeHeader`, `packet.DecodePacket` | `TestOSPFv3TransportDecodesPacket` (owned by ospfv3-3) |
| `ospfv3-5-ism` builds and sends a Hello | -> | `packet.Hello` + `WriteTo` | `TestOSPFv3ISMEncodesHello` (owned by ospfv3-5) |
| `ospfv3-7-lsdb` stores and re-floods an LSA | -> | `packet.LSA.RawBytes`, `types.LSAKey` | `TestOSPFv3WireUsesTypesLSAKey` |

These end-to-end wiring tests are owned by the importing specs because this codec is a leaf with no runtime caller yet. This spec creates the unit-level encode/decode tests named in the TDD plan. `TestOSPFv3WireUsesTypesLSAKey` (the umbrella's named wire-uses-types test) is created here as a codec unit test that decodes an LSA header and asserts the built `types.LSAKey` matches the header fields.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Encode then decode the common header | Version 3, Type, Packet Length, Router ID, Area ID, Instance ID round-trip; Reserved is zero; header length is 16 |
| AC-2 | Decode a header with Version != 3, Type outside 1..5, or Packet Length beyond the datagram | Rejected with a typed error, no slice past the buffer |
| AC-3 | Encode/decode a Hello | Interface ID, 24-bit Options, priority, intervals (DeadInterval 2 octets), DR/BDR Router IDs, and neighbor list round-trip; there is no network mask field |
| AC-4 | Encode/decode a DD | 24-bit Options, Interface MTU, I/M/MS flags, and DD sequence round-trip in the 12-byte fixed layout; trailing LSA headers iterate |
| AC-5 | Encode/decode an LSReq | each request entry round-trips (reserved 16 bits, 16-bit LS Type, Link State ID, Advertising Router) |
| AC-6 | Encode/decode an LSUpdate and an LSAck | LSA count matches; each LSA / LSA header round-trips; an over-long count is rejected |
| AC-7 | Decode an LSA header | LS Age, 16-bit LS Type (scope/function via `types`), Link State ID, Advertising Router, sequence, checksum, and length round-trip; the built `types.LSAKey` equals the header fields |
| AC-8 | Encode/decode a Router-LSA | flags W/V/E/B and 16-byte link records {Type, Metric, Interface ID, Neighbor Interface ID, Neighbor Router ID} round-trip; no IP addresses; link count derived from Length |
| AC-9 | Encode/decode a Network-LSA | Options and attached Router IDs round-trip; no network mask is present |
| AC-10 | Encode/decode Inter-Area-Prefix and Inter-Area-Router LSAs | metric, inlined prefix (length/options/address) and destination Router ID round-trip |
| AC-11 | Encode/decode an AS-External-LSA and an NSSA-LSA | E/F/T flags, metric, prefix, and the optional Forwarding Address / External Route Tag / Referenced LS ID round-trip for every flag combination |
| AC-12 | Encode/decode a Link-LSA | Rtr priority, Options, Link-Local Interface Address (128-bit), 32-bit prefix count, and the prefix list round-trip |
| AC-13 | Encode/decode an Intra-Area-Prefix-LSA | 16-bit prefix count, referenced LS Type / Link State ID / Advertising Router, and per-prefix metric round-trip |
| AC-14 | Encode an IPv6 prefix at lengths 0,1,31,32,33,64,127,128 | address bytes = `((len+31)/32)*4`; padding bits are zero; a non-zero padding bit is rejected on decode |
| AC-15 | Compute and verify the LSA Fletcher checksum | matches an OSPFv2-identical Fletcher over `lsa[2:length]`; the result is non-zero; a flipped byte fails |
| AC-16 | Compute and verify the packet checksum | the IPv6 upper-layer checksum over the pseudo-header (src, dst, length, Next Header 89) plus the zero-checksum packet matches; a wrong source address fails verification |
| AC-17 | Decode a received LSA and re-encode for flood | the re-encoded bytes equal the received `RawBytes` (no re-marshal drift); an unknown LS Type round-trips as an opaque span |
| AC-18 | Decode a truncated or oversized packet / LSA / prefix | rejected with a typed error; no panic and no out-of-bounds read |
| AC-19 | Package imports | `internal/plugins/ospfv3/packet` imports only `internal/plugins/ospfv3/types` and stdlib (no OSPFv2, transport, LSDB, SPF, config) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Brings up OSPFv3 and the node receives a Hello from FRR `ospf6d` | raw IPv6 -> transport -> `packet.DecodeHeader`/`DecodePacket` -> Hello -> ISM | `TestOSPFv3ISMEncodesHello` + `ospfv3-p2p-frr` (owned by ospfv3-5/13) |
| 2 | The node floods a received Link-LSA to a third router | LSUpdate decode -> `LSA.RawBytes` retained -> re-flood encode -> LSAck | `TestOSPFv3WireUsesTypesLSAKey` + `TestOSPFv3LSARawBytesRoundTrip` |
| 3 | The node installs a `/64` IPv6 route from an Intra-Area-Prefix-LSA | LSA prefix fields -> `types.PrefixLength` helpers -> SPF prefix attachment -> Loc-RIB | `TestOSPFv3IntraAreaPrefixRoundTrip` + `TestOSPFv3SPFUsesTypesPrefix` (owned by ospfv3-8) |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3HeaderRoundTrip` | `internal/plugins/ospfv3/packet/header_test.go` | 16-octet header encode/decode, version/type/length validation | |
| `TestOSPFv3DecodeHeaderBounds` | `internal/plugins/ospfv3/packet/header_test.go` | wrong version/type, length beyond datagram rejected | |
| `TestOSPFv3HelloRoundTrip` | `internal/plugins/ospfv3/packet/hello_test.go` | Hello fields incl. Interface ID + 24-bit Options, no network mask | |
| `TestOSPFv3DBDescRoundTrip` | `internal/plugins/ospfv3/packet/dbdesc_test.go` | 12-byte fixed layout, Options, MTU, I/M/MS, DD sequence, trailing LSA headers | |
| `TestOSPFv3LSReqRoundTrip` | `internal/plugins/ospfv3/packet/lsreq_test.go` | request entry (reserved + 16-bit LS Type + IDs) | |
| `TestOSPFv3LSUpdateRoundTrip` | `internal/plugins/ospfv3/packet/lsupdate_test.go` | LSA count + LSA list, over-long count rejected | |
| `TestOSPFv3LSAckRoundTrip` | `internal/plugins/ospfv3/packet/lsack_test.go` | LSA-header list round-trip | |
| `TestOSPFv3LSAHeaderRoundTrip` | `internal/plugins/ospfv3/packet/lsa_test.go` | 20-octet LSA header, `LSAKey` via types | |
| `TestOSPFv3WireUsesTypesLSAKey` | `internal/plugins/ospfv3/packet/lsa_test.go` | decoded key equals the header fields as a `types.LSAKey` | |
| `TestOSPFv3LSAIteratorBounds` | `internal/plugins/ospfv3/packet/lsa_test.go` | LSA Length validated within buffer; truncation rejected; opaque passthrough | |
| `TestOSPFv3LSARawBytesRoundTrip` | `internal/plugins/ospfv3/packet/lsa_test.go` | retained raw bytes re-encode equal to input | |
| `TestOSPFv3RouterLSARoundTrip` | `internal/plugins/ospfv3/packet/lsa_router_test.go` | flags + address-free 16-byte link records, count from Length | |
| `TestOSPFv3NetworkLSARoundTrip` | `internal/plugins/ospfv3/packet/lsa_network_test.go` | Options + attached routers, no mask | |
| `TestOSPFv3InterAreaPrefixRoundTrip` | `internal/plugins/ospfv3/packet/lsa_interarea_prefix_test.go` | metric + inlined prefix | |
| `TestOSPFv3InterAreaRouterRoundTrip` | `internal/plugins/ospfv3/packet/lsa_interarea_router_test.go` | options + metric + destination router | |
| `TestOSPFv3ExternalOptionalFields` | `internal/plugins/ospfv3/packet/lsa_external_test.go` | E/F/T flags gate Forwarding Address / Tag / Referenced LS ID | |
| `TestOSPFv3NSSARoundTrip` | `internal/plugins/ospfv3/packet/lsa_nssa_test.go` | NSSA body equals external body, area scope, P-bit in PrefixOptions | |
| `TestOSPFv3LinkLSARoundTrip` | `internal/plugins/ospfv3/packet/lsa_link_test.go` | priority, options, link-local address, 32-bit count, prefix list | |
| `TestOSPFv3IntraAreaPrefixRoundTrip` | `internal/plugins/ospfv3/packet/lsa_intraarea_prefix_test.go` | referenced LS type/id/adv router + 16-bit count + per-prefix metric | |
| `TestOSPFv3WirePrefixBoundaries` | `internal/plugins/ospfv3/packet/prefix_test.go` | prefix byte length + padding at boundary lengths, both carriage forms | |
| `TestOSPFv3LSAChecksum` | `internal/plugins/ospfv3/packet/checksum_test.go` | Fletcher over LSA excluding age, non-zero; tamper fails | |
| `TestOSPFv3PacketChecksum` | `internal/plugins/ospfv3/packet/checksum_test.go` | IPv6 pseudo-header checksum; wrong source fails | |
| `TestOSPFv3PacketNoRuntimeImports` | `internal/plugins/ospfv3/packet/imports_test.go` | imports only `types` + stdlib | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Packet Length | 16..datagram | datagram length | < 16 (header) | > datagram |
| Packet Type | 1..5 | 5 | 0 | 6 |
| Hello neighbor count | body length / 4 | exact fit | n/a | count implies bytes past Packet Length |
| LSUpdate LSA count | 0..fits in body | exact fit | n/a | count > LSAs present |
| LSA Length | 20..remaining | remaining buffer | < 20 (header) | > remaining buffer |
| PrefixLength | 0..128 | 128 | n/a | 129 |
| Prefix bytes | `((len+31)/32)*4` | 16 for /128 | buffer shorter than ByteLen | non-zero padding bits |
| Options | 0..0xffffff | 0xffffff | n/a | 4th octet non-zero on the wire |

### Functional Tests

None in this codec spec. Functional tests begin when transport and FSM call the codec:

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-adjacency.ci` | `test/ospfv3/ospfv3-adjacency.ci` | Hello/DD/LSReq/LSUpdate framing over the daemon | owned by `spec-ospfv3-5-interface-ism.md` / `-6` |
| `ospfv3-route-install.ci` | `test/ospfv3/ospfv3-route-install.ci` | Intra-Area-Prefix-LSA prefix decode feeds route install | owned by `spec-ospfv3-8-spf-rib.md` |

### Interop Tests

No interop scenario is owned by this codec spec; interop begins with transport + FSM. This spec's interop obligation is byte-exact RFC 5340 framing and an FRR-captured checksum vector (A-2) plus the AS-External packing vector (R-6) so `ospfv3-p2p-frr` and `ospfv3-broadcast-frr` succeed later. The packet-checksum, LSA-checksum, and AS-External tests SHOULD assert against a captured FRR `ospf6d` vector where available.

## Files to Modify

None. This spec creates a new leaf package only.

## Files to Create

- `internal/plugins/ospfv3/packet/doc.go` - package doc and the no-cross-version rationale.
- `internal/plugins/ospfv3/packet/header.go` - `PacketType`, common-header `WriteTo`/`EncodedLen`/`DecodeHeader`/`DecodePacket`, sentinel errors, `writeUint16/32`, `readUint16/32`.
- `internal/plugins/ospfv3/packet/hello.go` - `Hello` body encode/decode.
- `internal/plugins/ospfv3/packet/dbdesc.go` - `DBDesc` body encode/decode.
- `internal/plugins/ospfv3/packet/lsreq.go` - `LSReq` body encode/decode.
- `internal/plugins/ospfv3/packet/lsupdate.go` - `LSUpdate` body + LSA list encode/decode.
- `internal/plugins/ospfv3/packet/lsack.go` - `LSAck` body encode/decode.
- `internal/plugins/ospfv3/packet/lsa.go` - `LSA`, `LSAHeader`, `LSAIterator`, lazy `RawBytes` retention, opaque-LSA passthrough, `types.LSAKey` construction.
- `internal/plugins/ospfv3/packet/lsa_router.go` - Router-LSA body.
- `internal/plugins/ospfv3/packet/lsa_network.go` - Network-LSA body.
- `internal/plugins/ospfv3/packet/lsa_interarea_prefix.go` - Inter-Area-Prefix-LSA body.
- `internal/plugins/ospfv3/packet/lsa_interarea_router.go` - Inter-Area-Router-LSA body.
- `internal/plugins/ospfv3/packet/lsa_external.go` - AS-External-LSA body (optional fields).
- `internal/plugins/ospfv3/packet/lsa_nssa.go` - NSSA-LSA body (reuses external body, area scope).
- `internal/plugins/ospfv3/packet/lsa_link.go` - Link-LSA body.
- `internal/plugins/ospfv3/packet/lsa_intraarea_prefix.go` - Intra-Area-Prefix-LSA body.
- `internal/plugins/ospfv3/packet/prefix.go` - IPv6 prefix encode/decode (both carriage forms) over `types.PrefixLength`/`PrefixOptions`.
- `internal/plugins/ospfv3/packet/checksum.go` - Fletcher LSA checksum + IPv6 pseudo-header packet checksum.
- `internal/plugins/ospfv3/packet/*_test.go` - unit and boundary tests listed above.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | owned by `spec-ospfv3-4-plugin-config.md` |
| Config validators | No | codec consumes no config |
| Transport wiring | No | owned by `spec-ospfv3-3-ipv6-transport.md` |
| CLI | No | owned by `spec-ospfv3-13-cli-diag-interop.md` |
| Metrics | No | runtime specs own metrics |
| Docs | No (architecture wire doc updated when transport lands) | `docs/architecture/wire/ospfv3.md` owned by ospfv3-3+ |
| Interop | No (vectors only) | transport/FSM specs own interop scenarios |

### Documentation Update Checklist (BLOCKING)

| Category | Update? | File / action |
|----------|---------|---------------|
| Feature list | No | OSPFv3 stays unlisted until it adjacency-forms (ospfv3-5/6) |
| User guide | No | no user surface in a codec |
| Config syntax | No | no config |
| CLI reference | No | no CLI |
| API/RPC docs | No | no RPC |
| Wire format | Deferred | `docs/architecture/wire/ospfv3.md` is authored with transport (ospfv3-3); this spec leaves source anchors in code comments |
| RFC compliance | Yes (in-code) | every enforced MUST carries a `// RFC 5340 Section X.Y` comment |
| Comparison table | No | no user-visible capability yet |
| Test infrastructure | No | unit tests only |
| Architecture design | No | umbrella already records the package layout |

## Implementation Steps

### Implementation Phases

1. **Header + errors + IO helpers** - `header.go` with `PacketType`, `DecodeHeader`, common-header `WriteTo`/`EncodedLen`, sentinel errors, big-endian read/write helpers.
   - Tests: `TestOSPFv3HeaderRoundTrip`, `TestOSPFv3DecodeHeaderBounds` (fail until implemented).
2. **Packet bodies** - Hello, DD, LSReq, LSUpdate, LSAck encode/decode, each validating record counts against remaining length.
   - Tests: the five `*RoundTrip` tests + count-overflow rejection.
3. **LSA header + iterator + lazy bytes** - `lsa.go` with `LSAHeader`, `LSA` (typed body + retained `RawBytes`), `LSAIterator`, opaque passthrough, `types.LSAKey` construction.
   - Tests: `TestOSPFv3LSAHeaderRoundTrip`, `TestOSPFv3WireUsesTypesLSAKey`, `TestOSPFv3LSAIteratorBounds`, `TestOSPFv3LSARawBytesRoundTrip`.
4. **Base LSA bodies** - the eight bodies, address-free Router/Network and prefix-bearing LSAs, reusing `prefix.go`.
   - Tests: the per-LSA `*RoundTrip` tests + `TestOSPFv3ExternalOptionalFields`.
5. **Prefix records** - `prefix.go` encoding `PrefixLength`/`PrefixOptions`/address over `types` in both carriage forms, with padding validation.
   - Tests: `TestOSPFv3WirePrefixBoundaries`.
6. **Checksums** - Fletcher LSA checksum (port the OSPFv2 algorithm) and the IPv6 pseudo-header packet checksum (takes src/dst).
   - Tests: `TestOSPFv3LSAChecksum`, `TestOSPFv3PacketChecksum`.
7. **Bounds + import guard** - fuzz-style truncation/oversize rejection across header, packet, LSA, and prefix; `imports_test.go`.
   - Tests: `TestOSPFv3DecodeHeaderBounds` extended, `TestOSPFv3PacketNoRuntimeImports`.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Header width | the common header is exactly 16 octets; no AuType/Authentication field is encoded or expected |
| Packet checksum | uses the IPv6 pseudo-header (src, dst, upper-layer length, Next Header 89) and the zeroed checksum field; omitted under an auth trailer |
| LSA checksum | Fletcher over `lsa[2:length]` (LS Age excluded), byte-identical to OSPFv2, non-zero |
| Address-free topology LSAs | Router-LSA and Network-LSA encode no IPv6 addresses or masks; Router-LSA link count derived from Length |
| Prefix math | every prefix uses `types.PrefixLength.ByteLen` and validates padding; both carriage forms covered; no hardcoded /128 length |
| Optional external fields | Forwarding Address / Tag / Referenced LS ID are gated by E/F/T and the referenced LS Type, in RFC order |
| Lazy re-flood | a received LSA re-encodes from `RawBytes`; only self-originated LSAs are re-marshalled; unknown LS Types pass through opaque |
| Bounds | every slice is preceded by a length check; a crafted short buffer cannot panic |
| Buffer-first | encode is `WriteTo(buf, off) int` with `EncodedLen`; no `append`/`make` per field on the hot path |
| Imports | only `internal/plugins/ospfv3/types` + stdlib |

### Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| Header + 5 packet codecs | `go test ./internal/plugins/ospfv3/packet/ -run RoundTrip` green |
| 8 LSA body codecs | per-LSA `*RoundTrip` tests green |
| Prefix encoding | `TestOSPFv3WirePrefixBoundaries` green at /0../128 |
| Both checksums | `TestOSPFv3LSAChecksum`, `TestOSPFv3PacketChecksum` green |
| Bounds safety | truncation/oversize tests green; `go test -race` clean |
| Import isolation | `TestOSPFv3PacketNoRuntimeImports` green; `make ze-tier-check` 0 |
| Lint | `make ze-lint-changed` 0 |

### Security Review Checklist

| Concern | Check |
|---------|-------|
| Hostile short packet | `DecodeHeader` and every body decoder bound-check before slicing; no panic, no OOB read |
| Crafted LSA Length | `LSAIterator` rejects an LSA Length that exceeds the remaining buffer or is below 20 |
| Prefix length overflow | PrefixLength > 128 rejected; address bytes never read past the LSA Length |
| Integer truncation | 16-bit lengths and 24-bit metric/options use the `types` helpers; no silent `uint16` wrap on counts |
| Allocation bound | decode allocates per LSA proportional to the validated Length only; no count-driven pre-allocation before validation |
| Checksum spoofing | the packet checksum binds the IPv6 source/destination (pseudo-header), so a spoofed source fails verification |

## Implementation Audit

### Requirements from Task

- [ ] OSPFv3 common header codec (16 octets, no AuType).
- [ ] Five packet-type codecs (Hello, DD, LSReq, LSUpdate, LSAck).
- [ ] 20-octet LSA header codec with `types.LSAKey` construction.
- [ ] Eight base LSA body codecs.
- [ ] IPv6 prefix encoding over `types.PrefixLength`/`PrefixOptions`, both carriage forms.
- [ ] Fletcher LSA checksum and IPv6 pseudo-header packet checksum.
- [ ] Leaf isolation: imports only `internal/plugins/ospfv3/types` + stdlib.

### Acceptance Criteria

- [ ] AC-1 .. AC-19 each mapped to a named test with file:line at completion.

### Tests from TDD Plan

- [ ] Every unit and boundary test in the TDD plan exists and passes.

### Files from Plan

- [ ] Every file in Files to Create exists.

### Audit Summary

- [ ] Filled at implementation close.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence |
|------------------|----------|
| RFC 5340 byte-exact framing | round-trip tests per packet/LSA + (where available) an FRR-captured checksum and AS-External vector |
| No OSPFv2 carry-over | wire-difference table verified; `TestOSPFv3PacketNoRuntimeImports`; header is 16 octets; address-free Router/Network LSAs |
| Lazy re-flood | `TestOSPFv3LSARawBytesRoundTrip` |
| Bounds safety | truncation/oversize tests + `go test -race` |

## Pre-Commit Verification

### Files Exist (ls)

- [ ] `ls internal/plugins/ospfv3/packet/` shows every file in Files to Create.

### AC Verified (grep/test)

- [ ] `go test ./internal/plugins/ospfv3/packet/ -count=1` green; each AC mapped to a test.

### Wiring Verified (end-to-end)

- [ ] `TestOSPFv3WireUsesTypesLSAKey` passes; later transport/FSM specs consume the codec (their wiring tests are listed, not owned here).

### Assumptions Resolved

- [ ] A-1 .. A-5 each `confirmed` or `broken` with evidence.

### Documentation Verified

- [ ] In-code `// RFC 5340` comments present on enforced MUSTs; `docs/architecture/wire/ospfv3.md` deferral recorded.

## Cross-References

- Parent: `plan/spec-ospfv3-0-umbrella.md`
- Depends on: `plan/spec-ospfv3-1-types.md`
- Pattern source (not imported): `plan/spec-ospf-2-wire.md`, `internal/plugins/ospf/packet/`
- Consumed by: `spec-ospfv3-3-ipv6-transport.md`, `-5-interface-ism`, `-6-neighbor-nsm`, `-7-lsdb-flooding`, `-8-spf-rib`, `-12-auth`
