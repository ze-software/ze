# Spec: ospf-2-wire

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospf-1-types.md |
| Phase | 8/10 |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - umbrella scope; this child is row "ospf-2". Read "Shared Contracts": "LSA inventory", "LSA header + body layout", "Two distinct checksums", "Packet receive dispatcher", and the `packet/` Architecture row
4. `docs/research/ospf-implementation-guide.md` - sec 2 (Wire Format and Packet Types, lines ~81-137), sec 3 (LSA Registry and per-type body layouts, lines ~140-213), sec 13 (Known Hard Problems: Fletcher #1, sequence compare #2, Network-LSA LS ID confusion #5, zeroed-checksum auth #10, checksum refresh on re-origination #13, lines ~1432-1500)
5. `ai/rules/buffer-first.md`, `ai/rules/no-sprintf-alloc.md` - buffer-first `WriteTo(buf, off) int`, no hot-path allocation
6. Sibling specs: `spec-ospf-1-types.md` (domain types + both checksum algorithms), `spec-ospf-12-auth.md` (AuType 2 / RFC 7474 trailer verify/sign), `spec-isis-2-wire.md` (the IS-IS codec sibling; same shape, different protocol)

## Task

Implement the OSPFv2 packet and LSA wire codec as a pure, self-contained package
(`internal/plugins/ospf/packet/`) that depends only on the domain types and
checksum algorithms from `spec-ospf-1-types.md` (`internal/plugins/ospf/types`).
This child is the protocol's serialization boundary: it parses received datagrams
(after ospf-3 has stripped the IP header) into packet/LSA views and serializes
packet/LSA structs back to bytes. It contains no runtime, no sockets, no timers,
no LSDB, and no FSM; those live in later children (ospf-3 transport, ospf-5 ISM,
ospf-6 NSM, ospf-7 LSDB/flooding).

The codec covers the 24-byte OSPF common header (Version 2, Type, Packet Length,
Router ID, Area ID, Checksum, AuType 0/1/2/3, the 8-byte Authentication field), the
5 packet types (Hello, Database Description, Link State Request, Link State
Update, Link State Acknowledgment), the 20-byte LSA common header, and the LSA
bodies for Type 1 Router-LSA, Type 2 Network-LSA, Type 3/4 Summary-LSA, Type 5
AS-External-LSA, and Type 7 NSSA-LSA per the umbrella "LSA header + body layout"
contract. It APPLIES the two checksums whose algorithms ospf-1 owns: the IP
one's-complement packet checksum (excluding the 8-byte Authentication field) and
the Fletcher-16 LSA checksum (excluding LS Age). Unknown LSA types (9/10/11
opaque) are retained as opaque byte spans so they can be re-flooded verbatim once
a framework is added later (out of scope for v1 origination).

Decode is lazy (return views over the caller's byte slice, parse bodies on
demand), consistent with Ze's zero-copy philosophy. Encode is buffer-first
(`WriteTo(buf, off) int` into a caller-owned or pooled buffer), consistent with
`ai/rules/buffer-first.md`. The codec is exercised in v1 by the `ze` decode CLI
and, later, by the OSPF runtime; it must round-trip every packet and LSA type and
never panic on arbitrary input (fuzz target).

Authentication is a CODEC-ONLY concern here: this spec encodes and decodes the
common header's 8-byte Authentication field for all FOUR AuTypes -- 0 Null; 1
Simple (8-byte password); 2 Cryptographic (RFC 2328/5709: Reserved(2)=0 + Key
ID(1) + Auth Data Length(1) + 32-bit Cryptographic Sequence Number); and 3
Cryptographic with Extended Sequence Numbers (RFC 7474: a RESTRUCTURED field --
Reserved(3)=0 + Key ID(4, in the former sequence-number position) + Auth Data
Length(1) -- with a 64-bit Cryptographic Sequence Number appended before the
digest) -- and reserves the trailing digest region for AuType 2 and 3. The HMAC
sign/verify logic, key store, RFC 5709/7474 algorithm and sequence-number
semantics, and the zeroed-checksum rule enforcement live in
`spec-ospf-12-auth.md`. This spec only guarantees that the AuType field framing
round-trips for all four codes and that the codec can write a packet with a
zeroed Checksum field (so ospf-12 can sign over it).

## Required Reading

### Architecture Docs
- [ ] `docs/research/ospf-implementation-guide.md` sec 2 - packet type details, addressing, sequence/age/checksum field semantics
  -> Decision: adopt the per-packet-type file split (hello / dbdesc / lsreq / lsupdate / lsack) and the per-LSA-family file split (lsa_router / lsa_network / lsa_summary / lsa_external), the BIRD middle path between FRR's two monoliths (`ospf_packet.c` + `ospf_lsa.c`)
  -> Constraint: OSPF packet checksum is RFC 1071 one's-complement over the WHOLE packet EXCLUDING the 8-byte Authentication field (bytes 16..23), with the Checksum field (bytes 12..13) zeroed during computation; for AuType 2 and 3 the Checksum field is set to 0 and authentication replaces it (sec 2 "Sequence Numbers, Ages, and Checksums")
  -> Constraint: LSA checksum is Fletcher-16 over the LSA EXCLUDING the LS Age field (bytes 0..1), including the checksum field position; identical algorithm to IS-IS, different covered range (sec 2 + sec 13.1)
- [ ] `docs/research/ospf-implementation-guide.md` sec 3 - LSA common header, the per-type body layouts, the LSDB key triple
  -> Constraint: the LSDB key is `(LS Type, Link State ID, Advertising Router)`; the Network-LSA (Type 2) Link State ID is the DR's INTERFACE ADDRESS, not the network prefix (sec 3 + sec 13.5 trap #5); the codec must surface the LS ID verbatim and not reinterpret it per type
  -> Constraint: Router-LSA link records are 12 bytes each (Link ID 4, Link Data 4, Type 1, #TOS 1, Metric 2); #TOS is 0 in v1 and the obsolete per-TOS metric block is ignored; Type field is 1 p2p / 2 transit / 3 stub / 4 virtual (sec 3 Type 1)
- [ ] `docs/research/ospf-implementation-guide.md` sec 13 - hard traps relevant to the codec
  -> Constraint: Fletcher #1 (test encode AND verify directions against RFC 905 vectors; self-interop can pass while cross-interop fails); sequence-compare #2 belongs to ospf-7 freshness but the codec must preserve the signed 32-bit value verbatim; Network-LSA LS ID #5; zeroed-checksum auth #10 (codec must be able to emit a zero Checksum for ospf-12); checksum refresh on re-origination #13 (codec recomputes Fletcher on encode, never opportunistically)
- [ ] `ai/rules/buffer-first.md` - pooled, bounded buffers; skip-and-backfill for length and checksum fields
  -> Constraint: every packet and LSA has `WriteTo(buf []byte, off int) int`; Packet Length, packet Checksum, LSA Length, and LSA Checksum are written via skip-and-backfill, never via a `Len()`-then-`WriteTo()` double traversal on the hot path
- [ ] `ai/rules/no-sprintf-alloc.md` - no `fmt`/`+`/`.String()` concatenation on the wire path
  -> Constraint: any human-readable rendering (CLI decode) uses `textbuf.Buffer` / `AppendTo`, never `fmt.Sprintf`

### RFC Summaries (MUST for protocol work; existing, read before implementation)
- [ ] `rfc/short/rfc2328.md` - OSPF Version 2 base; Appendix A (packet and LSA formats)
  -> Constraint: §A.3.1 common header (Version 2, Type 1..5, Packet Length, Router ID, Area ID, Checksum, AuType, 8-byte Authentication); §A.3.2 Hello; §A.3.3 Database Description (I/M/MS flag bits, MTU, DD sequence); §A.3.4 LS Request (12-byte triples); §A.3.5 LS Update (4-byte count + LSAs); §A.3.6 LS Ack (LSA headers); §A.4.1 LSA header; §A.4.2 Router; §A.4.3 Network; §A.4.4 Summary 3/4; §A.4.5 AS-External; §12.1.6/§12.1.7 sequence/checksum
  -> Constraint: the LS Sequence Number is a SIGNED 32-bit integer, InitialSequenceNumber 0x80000001, MaxSequenceNumber 0x7FFFFFFF; the codec reads and writes the raw 4 bytes and does not normalise (freshness compare is ospf-7)
- [ ] `rfc/short/rfc905.md` - ISO Fletcher-16 checksum (Annex); shared algorithm with IS-IS (owned by ospf-1, applied here)
  -> Constraint: the LSA checksum covered range starts at the Options field (byte 2), excluding LS Age (bytes 0..1); the LS Checksum field position participates in the computation and is treated as zero on the forward pass
- [ ] `rfc/short/rfc1071.md` - Internet (IP) one's-complement checksum (owned by ospf-1, applied here)
  -> Constraint: the packet checksum covers the entire packet EXCLUDING the 8-byte Authentication field, with the Checksum field zeroed during computation
- [ ] `rfc/short/rfc3101.md` - NSSA Type 7 (same body as Type 5; P-bit in the LSA-header Options)
  -> Constraint: Type 7 NSSA-LSA body is byte-identical in layout to Type 5 AS-External; the P-bit lives in the LSA-header Options field, not in the body; the codec round-trips the Options byte verbatim so ospf-11 can read/set the P-bit

**Key insights:**
- The 24-byte common header is identical for all 5 packet types; the Type byte selects the body layout. The 8-byte Authentication field is OUTSIDE the packet-checksum coverage by design (so it can hold a digest)
- The 20-byte LSA common header is identical for all LSA types; the LS Type byte selects the body layout. The LSDB key is `(LS Type, Link State ID, Advertising Router)`
- Two distinct checksums with DIFFERENT covered ranges: packet IP one's-complement (whole packet minus the 8-byte auth field, Checksum zeroed) vs LSA Fletcher-16 (LSA minus the 2-byte LS Age, checksum-field position participates). Getting the covered range wrong is the single highest interop risk (R-1)
- Network-LSA (Type 2) Link State ID is the DR's INTERFACE ADDRESS, not the network prefix (trap #5); the codec keeps LS ID as an opaque 4-byte value and never reinterprets it
- Router-LSA (Type 1) transit-link encoding (Link ID = DR interface address, Link Data = own interface address) is what the SPF two-way check (ospf-8) depends on; the codec must round-trip every 12-byte link record exactly
- Summary metric (Type 3/4) and External metric (Type 5/7) are 24-bit (3 octets), distinct from the Router/Network 16-bit metric; the codec must read/write the right width per LSA type and never truncate
- The AS-External / NSSA per-TOS block carries the E-bit in the high bit (0x80) of the first metric byte (E=0 -> E1, E=1 -> E2), then a 3-byte metric, a 4-byte Forwarding Address, and a 4-byte External Route Tag
- Lazy decode means a packet/LSA view holds the raw slice plus offsets; LSA iteration in an LS Update yields `(header, body-slice)` without copying; unknown LSA types are kept as raw spans for verbatim re-flood

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey for this child; types come from ospf-1)
- [ ] Ze has no OSPF packet/LSA codec today; `internal/plugins/ospf/packet/` does not exist
  -> Constraint: this is entirely new; nothing to preserve inside the package
- [ ] `internal/plugins/rsvpte/transport_linux.go` shows the proto-based raw-IP receive path: the kernel delivers the full datagram and the consumer strips the IP header by IHL before handing the payload up
  -> Constraint: ospf-3 (not this codec) strips the IP header; this codec is handed the OSPF payload starting at the common header. Do NOT parse IP headers here
- [ ] `internal/plugins/isis/packet/` (isis-2) is the sibling codec: lazy views, buffer-first `WriteTo`, Fletcher checksum, opaque-TLV passthrough, fuzz targets, `ze` decode CLI
  -> Constraint: mirror the structure and conventions; do NOT import or couple to the IS-IS codec (different protocol, different framing). The shared Fletcher algorithm lives in ospf-1 (and is logically the same as IS-IS's) but is consumed, not imported across protocols
- [ ] `internal/plugins/ospf/types` (spec-ospf-1) provides RouterID, AreaID, LSAKey, LSSequenceNumber, LSAge, Metric, Options with parse/format/compare, plus `Fletcher16` (LSA) and the IP one's-complement checksum
  -> Constraint: this codec consumes those types and both checksum functions; it must not redefine them. Packet/LSA structs hold typed fields where a domain type exists

**Behavior to preserve:**
- RSVP-TE raw-socket code unchanged (this codec adds no transport)
- IS-IS codec independent and unchanged (no cross-protocol coupling)
- `internal/plugins/ospf/types` public API (from ospf-1) is consumed as-is; this spec adds no types to that package

**Behavior to change:**
- New package `internal/plugins/ospf/packet/` with packet and LSA codecs
- New `ze` decode surface (CLI) able to decode an OSPF packet from hex/pcap bytes (wiring proof; full CLI polish is ospf-13)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Decode: a `[]byte` containing one OSPF packet (the IP header is stripped by ospf-3 before this codec sees the bytes; in v1 the `ze` decode CLI supplies the bytes directly from hex or a captured pcap payload)
- Encode: a packet struct (Hello / DD / LS Request / LS Update / LS Ack) populated with typed fields, plus, for LS Update / LS Ack, a list of LSA structs/headers, plus a caller-owned buffer

### Transformation Path
1. **Common header parse:** read the 24-byte common header; validate Version == 2, parse Type, validate Packet Length against the slice length; extract Router ID, Area ID, Checksum, AuType, and the 8-byte Authentication field as a typed view. Checksum and auth VERIFICATION are ospf-12 / the instance dispatcher (ospf-4); the codec exposes the fields and a `VerifyChecksum` helper
2. **Packet dispatch:** switch on Type to the body parser (Hello / DD / LS Request / LS Update / LS Ack); parse the packet-specific fixed fields into a typed view over the slice
3. **LSA iterate (lazy):** for LS Update, an `LSAIterator` walks the LSA region using each LSA's Length field, yielding `(LSAHeader, body []byte)` without copying; the per-type body decoder is called on demand. For LS Ack and DD, the 20-byte LSA headers are iterated likewise. Unknown LSA types are retained as opaque spans
4. **Encode (buffer-first):** the packet struct writes the common header (Checksum and Packet Length skipped), then the packet-specific fixed fields, then each body/LSA via `WriteTo(buf, off) int`; Packet Length is backfilled; the packet Checksum is computed (RFC 1071, excluding the auth field) and backfilled last UNLESS AuType is 2 or 3 (then Checksum stays zero, ospf-12 appends the digest). Each LSA's Fletcher-16 checksum is computed over the LSA (excluding LS Age) and backfilled into its own LS Checksum field

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Raw bytes <-> packet view | lazy decode: view holds slice + offsets, no copy | [ ] |
| Packet view <-> LSA stream | `LSAIterator` yields `(header, body-slice)` driven by LSA Length | [ ] |
| Packet/LSA struct <-> bytes | buffer-first `WriteTo(buf, off) int`, skip-and-backfill length/checksum | [ ] |
| types <-> packet | typed fields (RouterID, AreaID, LSAKey, metric, sequence) parsed/formatted via ospf-1; both checksums from ospf-1 | [ ] |
| packet <-> ospf-12 auth | the 8-byte Authentication field round-trips for AuType 0/1/2; a packet can be written with a zeroed Checksum for AuType 2 signing | [ ] |

### Integration Points
- `internal/plugins/ospf/types` (ospf-1) - typed fields inside packet/LSA structs; `Fletcher16` (LSA) and the IP one's-complement checksum (packet)
- `internal/plugins/ospf/transport` (ospf-3) - hands stripped OSPF payload bytes to the decoder, takes encoded bytes for sending (consumer, not built here)
- `internal/plugins/ospf/lsdb` (ospf-7) - stores raw LSA bytes + parsed metadata (LSAKey, sequence, age, checksum) obtained from this codec; re-floods unknown LSA types verbatim; refreshes the Fletcher checksum on re-origination
- `internal/plugins/ospf/packet` (AuType field) <-> `spec-ospf-12-auth` - structural auth-field codec only; verify/sign and the RFC 7474 trailer are ospf-12
- `ze` decode CLI (ospf-13 polish) - human-readable rendering via `AppendTo`

### Architectural Verification
- [ ] No bypassed layers (bytes -> common header -> dispatch -> body/LSA iterate; encode struct -> header -> fixed -> bodies/LSAs -> length+checksum backfill)
- [ ] No unintended coupling (independent of IS-IS and BGP; depends only on ospf-1 types + checksums)
- [ ] No duplicated functionality (domain types and both checksums from ospf-1, not redefined; each LSA body codec in one file)
- [ ] Zero-copy preserved (decode returns views; unknown LSA types kept as spans; encode is buffer-first)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The RFC 1071 packet checksum over the whole packet MINUS the 8-byte Authentication field (Checksum zeroed) matches what FRR/BIRD compute | guide sec 2; RFC 1071 | every packet we send is rejected; interop fails | `TestOSPFPacketChecksum`, `TestOSPFPacketChecksumExcludesAuth`, `make ze-ospf-wire-test` real capture decode | partially-confirmed |
| A-2 | The Fletcher-16 covered range (from Options, excluding LS Age) yields a checksum that re-verifies to zero over the LSA | guide sec 2 + sec 13.1; RFC 905 | every LSA we originate is rejected; SPF on peers drops our vertices | `TestFletcherRFC905Vectors`, `TestOSPFLSAChecksum`, `TestOSPFLSAChecksumExcludesAge`, real LS Update capture decode | confirmed |
| A-3 | ospf-1 types expose parse/format/compare for RouterID, AreaID, LSAKey, LSSequenceNumber, LSAge, Metric, Options and BOTH checksum functions sufficient for codec fields | umbrella architecture (ospf-1 row) | codec must define helpers locally, duplicating ospf-1 | packet package builds against ospf-1; added `InternetChecksumPair` for auth-excluded packet checksum coverage | confirmed |
| A-4 | All 5 packet types share the identical 24-byte common header and differ only in the body | guide sec 2; RFC 2328 §A.3.1 | header parse must branch per type before length is known | `TestOSPFHeaderRoundTrip`, `TestOSPFHelloRoundTrip`, `TestOSPFDDRoundTrip`, `TestOSPFLSReqRoundTrip`, `TestOSPFLSUpdateRoundTrip`, `TestOSPFLSAckRoundTrip` | confirmed |
| A-5 | Lazy views over the caller's slice are safe because the transport hands a stable buffer for the packet lifetime | buffer-first philosophy; ospf-3 owns the read buffer | views dangle when the transport recycles the buffer | `doc.go` lifetime contract; `LSA.RawBytes`/`Body` retained as caller-owned views; ospf-7 still must copy retained LSAs | partially-confirmed |
| A-6 | The Network-LSA LS ID (DR interface address) and all per-type LS IDs can be carried as an opaque 4-byte field without per-type reinterpretation in the codec | guide sec 3 + sec 13.5 trap #5 | the codec misreads the Network-LSA key, SPF drops half the topology | `TestOSPFNetworkLSARoundTrip` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Wrong covered range for EITHER checksum (packet excludes auth field; LSA excludes LS Age) | round-trip/interop checksum failures; self-interop passes, cross-interop fails | dedicated covered-range tests for both; `Verify(encode(x))` property tests; decode a real FRR/Wireshark capture and verify |
| R-2 | LSA Length-driven iteration off by one (header 20 bytes counted vs not) leaves a trailing or truncated LSA in an LS Update | round-trip mismatch; LSA count vs bytes disagree | explicit boundary test: LS Update with 1 and N LSAs; Length includes the 20-byte header; iterate by Length and assert consumed == Packet Length |
| R-3 | Decoder panics on truncated/malformed bytes (slice out of range) | fuzz crash | every read bound-checked before slicing; LSA Length and Packet Length sanity-checked against the slice; fuzz target asserts no panic |
| R-4 | Network-LSA LS ID reinterpreted as a prefix (trap #5) | SPF drops transit segments | keep LS ID as raw 4 bytes; round-trip test asserts byte-preservation; no per-type LS ID rewriting in the codec |
| R-5 | Metric width confused (16-bit Router/Network vs 24-bit Summary/External) | round-trip mismatch; metric truncation at boundary | per-LSA-type metric-width boundary tests (65535 vs 16777215); read/write the exact octet count per type |
| R-6 | AuType 2 packet written with a non-zero Checksum, breaking the digest (trap #10) | auth always fails against FRR | the encoder, when AuType == 2, leaves Checksum zero and never backfills it; a test asserts Checksum == 0 for AuType 2 output |
| R-7 | Unknown LSA-type passthrough drops bytes, breaking re-flood | LSDB re-flood differs from received | opaque-LSA round-trip test: decode then re-encode equals input byte-for-byte (driven by the LSA Length field) |
| R-8 | DD I/M/MS flag bits packed in the wrong bit positions | DD master/slave negotiation never completes (ospf-6) | bit-position test pinning I=0x04, M=0x02, MS=0x01 in the flags byte; round-trip all 8 combinations |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze` decode of an OSPF packet hex string | -> | common header parse + packet dispatch + body iterate | `test/ospf-wire/ospf-packet-1.ci` |
| Each packet struct encoded then decoded | -> | `(*Packet).WriteTo` + `DecodePacket` round-trip | `TestOSPFPacketRoundTrip` |
| A packet encoded | -> | RFC 1071 checksum backfilled, `VerifyChecksum` returns true | `TestOSPFPacketChecksum` |
| Each LSA struct encoded then decoded | -> | `(*LSA).WriteTo` + `DecodeLSA` round-trip; Fletcher backfilled, `VerifyLSAChecksum` true | `TestOSPFLSARoundTrip`, `TestOSPFLSAChecksum` |
| An LS Update with an unknown LSA type decoded then re-encoded | -> | opaque span retained and re-serialized verbatim | `TestOSPFUnknownLSAPassthrough` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Bytes of any of the 5 packet types | common header parse validates Version == 2 and Packet Length, dispatches to the correct body parser, exposes a typed view including Router ID, Area ID, Checksum, AuType, and the 8-byte Authentication field |
| AC-2 | Each packet struct (Hello, DD, LS Request, LS Update, LS Ack) | `WriteTo` then decode reproduces every field, LSA header, and LSA body (round-trip identity) |
| AC-3 | A packet being encoded with AuType 0 or 1 | RFC 1071 checksum computed over the packet EXCLUDING the 8-byte Authentication field with the Checksum field zeroed, backfilled; `VerifyChecksum` over the encoded packet returns true |
| AC-4 | A packet being encoded with AuType 2 | the Checksum field is left zero (never backfilled); the 8-byte Authentication field carries Key ID + Auth Data Length + Cryptographic Sequence Number; ospf-12 appends the digest (codec leaves room and writes zero checksum) |
| AC-5 | Each LSA being encoded (Types 1, 2, 3, 4, 5, 7) | Fletcher-16 computed over the LSA EXCLUDING the LS Age field, backfilled into LS Checksum; `VerifyLSAChecksum` returns true; LSA Length includes the 20-byte header |
| AC-6 | Known RFC 905 / RFC 1071 vectors (via ospf-1) | the codec's checksum application over a fixed packet/LSA matches each vector exactly |
| AC-7 | A Type 1 Router-LSA with p2p, transit, stub, and virtual link records | the flags byte (V/E/B) and every 12-byte link record (Link ID, Link Data, Type 1..4, #TOS, 16-bit metric) round-trip; #TOS 0 |
| AC-8 | A Type 2 Network-LSA | Network Mask + the attached-router list (including the DR) round-trip; the Link State ID (DR interface address) is preserved verbatim and never reinterpreted as a prefix |
| AC-9 | A Type 3 and a Type 4 Summary-LSA | Network Mask (0.0.0.0 for Type 4) + 1-byte TOS (0) + 24-bit Metric round-trip; the LS ID is the network (Type 3) or the ASBR Router ID (Type 4) |
| AC-10 | A Type 5 AS-External-LSA | Network Mask, E-bit (0x80 of the first metric byte) + 24-bit Metric, Forwarding Address, and External Route Tag round-trip for both E1 (E=0) and E2 (E=1) |
| AC-11 | A Type 7 NSSA-LSA with the P-bit set in the LSA-header Options | the body (identical layout to Type 5) round-trips and the Options byte (carrying the P-bit) is preserved verbatim |
| AC-12 | An LS Update with 1 and with N LSAs | the 4-byte LSA count and every LSA round-trip; iteration is driven by each LSA's Length field; consumed bytes equal Packet Length |
| AC-13 | A DD packet | Interface MTU, Options, the I/M/MS flag bits (I=0x04, M=0x02, MS=0x01), DD Sequence Number, and the list of 20-byte LSA headers round-trip; all 8 I/M/MS combinations are valid |
| AC-14 | An LS Request | the list of 12-byte (LS Type, Link State ID, Advertising Router) triples round-trips |
| AC-15 | An LS Acknowledgment | the list of 20-byte LSA headers round-trips |
| AC-16 | An LS Update carrying an LSA type the codec does not recognise (9/10/11 opaque) | the unknown LSA is retained as an opaque span and re-encoded byte-for-byte identical to the input |
| AC-17 | Arbitrary random bytes fed to the decoder | no panic; either a parsed view or a typed error is returned |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze` decode on a captured OSPF Hello hex (or pcap payload) | CLI -> common header parse -> packet dispatch (Hello) -> body fields + neighbour list -> rendered output | `test/ospf-wire/ospf-packet-1.ci` |
| 2 | OSPF runtime (later) originates a Router-LSA in an LS Update | LSA struct -> `WriteTo` -> Fletcher backfill -> packet `WriteTo` -> RFC 1071 checksum backfill -> bytes handed to transport | `TestOSPFLSARoundTrip`, `TestOSPFLSAChecksum`, `TestOSPFPacketChecksum` |
| 3 | OSPF runtime (later) re-floods a received LSA carrying an opaque type it does not understand | decode (retain opaque span) -> store raw in LSDB -> re-encode verbatim | `TestOSPFUnknownLSAPassthrough` |
| 4 | OSPF runtime (later) builds a DD packet summarising its LSDB | DD struct with a list of 20-byte LSA headers + I/M/MS flags -> `WriteTo` -> decode -> headers and flags match | `TestOSPFDDRoundTrip` |
| 5 | OSPF runtime (later) decodes a real FRR-originated LS Update off the wire | captured payload -> `DecodePacket` -> LSA iterate -> per-type body -> checksum verifies | `test/ospf-wire/ospf-lsupdate-frr.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFPacketTypeConstants` | `internal/plugins/ospf/packet/header_test.go` | the 5 packet Type bytes (1 Hello, 2 DD, 3 LS Request, 4 LS Update, 5 LS Ack) and Version == 2 | |
| `TestOSPFHeaderRoundTrip` | `internal/plugins/ospf/packet/header_test.go` | 24-byte common header encode/decode for all 5 types; rejects Version != 2 and a Packet Length exceeding the slice | |
| `TestOSPFHeaderAuTypeField` | `internal/plugins/ospf/packet/header_test.go` | AuType 0/1/2 and the 8-byte Authentication field round-trip; AuType 2 decomposes into Key ID + Auth Data Length + Cryptographic Sequence Number | |
| `TestOSPFHelloRoundTrip` | `internal/plugins/ospf/packet/hello_test.go` | network mask, hello-interval, options, priority, dead-interval, DR, BDR, and the neighbour list (0, 1, N entries) | |
| `TestOSPFDDRoundTrip` | `internal/plugins/ospf/packet/dbdesc_test.go` | interface MTU, options, I/M/MS flags (all 8 combinations), DD sequence, list of 20-byte LSA headers | |
| `TestOSPFLSReqRoundTrip` | `internal/plugins/ospf/packet/lsreq_test.go` | list of 12-byte (LS Type, LS ID, Advertising Router) triples (0, 1, N) | |
| `TestOSPFLSUpdateRoundTrip` | `internal/plugins/ospf/packet/lsupdate_test.go` | 4-byte LSA count + N LSAs; iteration driven by LSA Length; consumed == Packet Length | |
| `TestOSPFLSAckRoundTrip` | `internal/plugins/ospf/packet/lsack_test.go` | list of 20-byte LSA headers (0, 1, N) | |
| `TestOSPFPacketChecksum` | `internal/plugins/ospf/packet/checksum_test.go` | RFC 1071 over the packet minus the 8-byte auth field, Checksum zeroed; verify(encode)==true | |
| `TestOSPFPacketChecksumExcludesAuth` | `internal/plugins/ospf/packet/checksum_test.go` | flipping bytes in the 8-byte auth field does NOT change the packet checksum; flipping any other byte does | |
| `TestOSPFPacketChecksumZeroForAuType2` | `internal/plugins/ospf/packet/checksum_test.go` | encoding with AuType 2 leaves the Checksum field zero (trap #10, R-6) | |
| `TestOSPFLSAHeaderRoundTrip` | `internal/plugins/ospf/packet/lsa_test.go` | 20-byte LSA header (age, options, type, LS ID, advertising router, signed sequence, checksum, length) | |
| `TestOSPFLSAChecksum` | `internal/plugins/ospf/packet/checksum_test.go` | Fletcher-16 over the LSA minus LS Age; verify(encode)==true; Length includes the 20-byte header | |
| `TestOSPFLSAChecksumExcludesAge` | `internal/plugins/ospf/packet/checksum_test.go` | flipping LS Age does NOT change the Fletcher checksum; flipping any covered byte does | |
| `TestOSPFRouterLSARoundTrip` | `internal/plugins/ospf/packet/lsa_router_test.go` | V/E/B flags + link records p2p/transit/stub/virtual; #TOS 0; 16-bit metric boundary (65535) | |
| `TestOSPFNetworkLSARoundTrip` | `internal/plugins/ospf/packet/lsa_network_test.go` | network mask + attached-router list incl. DR; LS ID (DR iface addr) preserved verbatim (trap #5, R-4) | |
| `TestOSPFSummaryLSARoundTrip` | `internal/plugins/ospf/packet/lsa_summary_test.go` | Type 3 + Type 4; mask (0.0.0.0 for Type 4) + TOS 0 + 24-bit metric boundary (16777215) | |
| `TestOSPFExternalLSARoundTrip` | `internal/plugins/ospf/packet/lsa_external_test.go` | Type 5 + Type 7; mask + E-bit (E1/E2) + 24-bit metric + forwarding address + route tag; Type 7 P-bit in Options | |
| `TestOSPFUnknownLSAPassthrough` | `internal/plugins/ospf/packet/lsa_opaque_test.go` | unknown LSA type (9/10/11) decode then re-encode is byte-identical (Length-driven) | |
| `TestOSPFLSAIteratorTruncated` | `internal/plugins/ospf/packet/lsupdate_test.go` | iterator stops cleanly on a truncated LSA / bad Length, no panic | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Version (common header) | 2 only | 2 | 1 (reject) | 3 (reject; that is OSPFv3) |
| Packet Type | 1..5 | 5 | 0 (reject) | 6 (reject / unknown) |
| Packet Length | 24..65535 | 65535 | <24 (smaller than the header) | > slice length (reject) |
| LS Type | 1..11 (1,2,3,4,5,7 parsed; 9/10/11 opaque) | 11 | 0 (reject) | 12 (reject / unknown) |
| LS Age | 0..3600 (MaxAge); DoNotAge bit 0x8000 | 3600 | N/A | >3600 treated as MaxAge by ospf-7 (codec preserves raw value) |
| LS Sequence Number | 0x80000001..0x7FFFFFFF (signed) | 0x7FFFFFFF | 0x80000000 (reserved) | wraps -> ospf-7 flush/re-originate (codec preserves raw 4 bytes) |
| LSA Length | 20..65535 | 65535 | <20 (smaller than the LSA header) | > remaining slice (reject) |
| Router/Network metric | 0..65535 (16-bit) | 65535 | N/A | 65536 (exceeds the 2-octet field) |
| Summary/External metric | 0..16777215 (24-bit) | 16777215 | N/A | 16777216 (exceeds the 3-octet field) |
| DR priority (Hello) | 0..255 | 255 | N/A | 256 |
| Hello / Dead interval | 1..65535 / 0..0xFFFFFFFF | 65535 / max | 0 (Hello interval) | 65536 (Hello, 2-octet) |
| Interface MTU (DD) | 0..65535 | 65535 | N/A | 65536 |
| DD I/M/MS flags | 3 bits (I=0x04, M=0x02, MS=0x01) | all 3 set | N/A | upper 5 bits reserved (preserved on decode) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-packet-1` | `test/ospf-wire/ospf-packet-1.ci` | `ze` decode of a captured OSPF Hello renders the common header + body + neighbour list | |
| `ospf-lsupdate-frr` | `test/ospf-wire/ospf-lsupdate-frr.ci` | `ze` decode of a real FRR-originated LS Update payload renders each LSA; both checksums verify | |
| `ospf-truncated` | `test/ospf-wire/ospf-truncated.ci` | `ze` decode of a truncated packet exits with a typed error, no panic (AC-17) | |

### Interop Tests (MANDATORY for protocol features)
This child is a pure codec with no wire I/O of its own; on-the-wire interop with
FRR `ospfd` is exercised by the runtime children (ospf-13 scenarios: P2P,
broadcast/DR, multi-area, stub, NSSA, redistribution, auth, convergence). The
codec is validated here by exhaustive round-trip unit tests, the checksum
covered-range tests, the RFC 905 / RFC 1071 vectors (via ospf-1), the fuzz
target, and a decode of at least one real Wireshark/tcpdump/FRR capture
(`test/ospf-wire/ospf-lsupdate-frr.ci`). This mirrors how `spec-isis-2-wire.md`
frames its interop: wire compatibility is proven end-to-end by ospf-13, and
ospf-2 itself does decode-vector + fuzz.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (covered by ospf-13) | `test/interop/scenarios/` | FRR ospfd | on-wire packet/LSA interop once the runtime exists | |

### Fuzz Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `FuzzOSPFDecodePacket` | `internal/plugins/ospf/packet/fuzz_test.go` | decoder never panics on arbitrary bytes; bound checks before every slice | |
| `FuzzOSPFLSAIterator` | `internal/plugins/ospf/packet/fuzz_test.go` | LSA iteration over arbitrary LS Update bytes terminates without panic | |
| `FuzzOSPFRoundTrip` | `internal/plugins/ospf/packet/fuzz_test.go` | decode-then-encode of a valid corpus is stable (no byte drift) | |

### Future (if deferring any tests)
- Opaque LSA types 9/10/11 are passthrough-only in v1 (decode-as-opaque + verbatim re-encode); per-TLV parsing of opaque bodies (TE / SR / Router Information) is a future framework, not this codec

## Files to Modify
- `plan/spec-ospf-0-umbrella.md` - no content change; this child realises the ospf-2 row (cross-reference only)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | none; pure codec, no config surface (config is ospf-4) |
| YANG validation constraints | No | n/a |
| YANG custom validators | No | n/a |
| CLI commands/flags | No | `ze` decode wiring is a thin caller; full CLI is ospf-13 |
| CLI grammar (action before identifier) | No | n/a for this child |
| Editor autocomplete | No | n/a |
| Functional test for new RPC/API | Yes | `test/ospf-wire/ospf-packet-1.ci`, `ospf-lsupdate-frr.ci`, `ospf-truncated.ci` |
| Pipe completeness | No | n/a (decode output polish is ospf-13) |
| Env var registration | No | n/a |
| Doctor check for runtime dependencies | No | n/a (no sockets/paths in this child; transport doctor check is ospf-3) |
| Prometheus counters/metrics | No | n/a (codec has no runtime state; metrics are ospf-3/ospf-13) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | codec is internal; user-facing OSPF row tracked in ospf-13 |
| 2 | Config syntax changed? | No | none (config is ospf-4) |
| 3 | CLI command added/changed? | No | `ze` decode polish is ospf-13 |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | `docs/guide/ospf.md` is ospf-13 |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospf.md` (packet + LSA codec) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md`, `rfc905.md`, `rfc1071.md`, `rfc3101.md` (packet/LSA codec sections) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/ospf-wire/`) |
| 11 | Affects daemon comparison? | No | comparison row is ospf-13 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/ospf.md` (codec layering: types <- packet) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin/event/command/capability changed? | No | none |
| 16 | Changed files referenced by doc source anchors? | No | grep `docs/` at completion |
| 17 | Existing docs show examples for this area? | No | grep `docs/architecture/wire/` at completion |

## Files to Create
- `internal/plugins/ospf/packet/header.go` - common 24-byte header encode/decode + packet Type constants + Version validation + packet dispatch + AuType field framing
- `internal/plugins/ospf/packet/hello.go` - Hello body codec (mask, intervals, options, priority, DR, BDR, neighbour list)
- `internal/plugins/ospf/packet/dbdesc.go` - Database Description body codec (MTU, options, I/M/MS flags, DD sequence, LSA-header list)
- `internal/plugins/ospf/packet/lsreq.go` - Link State Request body codec (12-byte triples)
- `internal/plugins/ospf/packet/lsupdate.go` - Link State Update body codec (count + LSAs) + `LSAIterator` (Length-driven)
- `internal/plugins/ospf/packet/lsack.go` - Link State Acknowledgment body codec (LSA-header list)
- `internal/plugins/ospf/packet/lsa.go` - 20-byte LSA common header encode/decode + LS Type constants + per-type dispatch
- `internal/plugins/ospf/packet/lsa_router.go` - Type 1 Router-LSA body codec (V/E/B flags + 12-byte link records)
- `internal/plugins/ospf/packet/lsa_network.go` - Type 2 Network-LSA body codec (mask + attached-router list); LS ID preserved verbatim
- `internal/plugins/ospf/packet/lsa_summary.go` - Type 3/4 Summary-LSA body codec (mask + TOS + 24-bit metric)
- `internal/plugins/ospf/packet/lsa_external.go` - Type 5 AS-External + Type 7 NSSA body codec (mask + E-bit/24-bit metric + forwarding address + route tag)
- `internal/plugins/ospf/packet/lsa_opaque.go` - unknown LSA-type opaque span retention + verbatim re-serialization (Length-driven)
- `internal/plugins/ospf/packet/checksum.go` - packet RFC 1071 application (covered range excludes the 8-byte auth field) + LSA Fletcher-16 application (covered range excludes LS Age) + verify helpers (algorithms imported from ospf-1)
- `internal/plugins/ospf/packet/header_test.go` - common header + Type constant + AuType field tests
- `internal/plugins/ospf/packet/hello_test.go` - Hello round-trip tests
- `internal/plugins/ospf/packet/dbdesc_test.go` - DD round-trip + I/M/MS flag tests
- `internal/plugins/ospf/packet/lsreq_test.go` - LS Request round-trip tests
- `internal/plugins/ospf/packet/lsupdate_test.go` - LS Update round-trip + iterator-truncation tests
- `internal/plugins/ospf/packet/lsack_test.go` - LS Ack round-trip tests
- `internal/plugins/ospf/packet/lsa_test.go` - LSA header round-trip tests
- `internal/plugins/ospf/packet/lsa_router_test.go` - Router-LSA round-trip + metric boundary tests
- `internal/plugins/ospf/packet/lsa_network_test.go` - Network-LSA round-trip + LS ID preservation tests
- `internal/plugins/ospf/packet/lsa_summary_test.go` - Summary-LSA round-trip + 24-bit metric boundary tests
- `internal/plugins/ospf/packet/lsa_external_test.go` - External/NSSA round-trip + E1/E2 + P-bit tests
- `internal/plugins/ospf/packet/lsa_opaque_test.go` - unknown-LSA passthrough test
- `internal/plugins/ospf/packet/checksum_test.go` - both checksum covered-range + vector + corruption tests
- `internal/plugins/ospf/packet/fuzz_test.go` - decode/iterator/round-trip fuzz targets
- `test/ospf-wire/ospf-packet-1.ci` - `ze` decode functional test for a captured OSPF Hello
- `test/ospf-wire/ospf-lsupdate-frr.ci` - `ze` decode of a real FRR LS Update capture
- `test/ospf-wire/ospf-truncated.ci` - truncated-input error-path test (AC-17)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `plan/spec-ospf-0-umbrella.md` |
| 2. Audit | Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- common header parse + packet dispatch skeleton + failing wiring tests
   - Tests: `TestOSPFPacketTypeConstants`, `TestOSPFHeaderRoundTrip`, `test/ospf-wire/ospf-packet-1.ci` (failing)
   - Files: `internal/plugins/ospf/packet/header.go`
   - Verify: a packet's common header parses, validates Version == 2 and Packet Length, and dispatches; bodies are stubs; wiring test fails because bodies are not implemented
2. **Phase: Both checksum applications** -- isolate the highest-risk item first
   - Tests: `TestOSPFPacketChecksum`, `TestOSPFPacketChecksumExcludesAuth`, `TestOSPFPacketChecksumZeroForAuType2`, `TestOSPFLSAChecksum`, `TestOSPFLSAChecksumExcludesAge`
   - Files: `internal/plugins/ospf/packet/checksum.go`
   - Verify: packet RFC 1071 over the right covered range (auth field excluded), Checksum zeroed during compute, left zero for AuType 2; LSA Fletcher over the right covered range (LS Age excluded); both `Verify(encode(x))` properties hold; corruption detected
3. **Phase: LSA common header + opaque passthrough** -- 20-byte header + Length-driven opaque retention
   - Tests: `TestOSPFLSAHeaderRoundTrip`, `TestOSPFUnknownLSAPassthrough`, `TestOSPFLSAIteratorTruncated`
   - Files: `internal/plugins/ospf/packet/lsa.go`, `lsa_opaque.go`
   - Verify: LSA header round-trips; unknown LSA type re-encodes byte-identical; iteration never panics on a bad Length
4. **Phase: LSA bodies** -- Router (1), Network (2), Summary (3/4), External (5) / NSSA (7)
   - Tests: `TestOSPFRouterLSARoundTrip`, `TestOSPFNetworkLSARoundTrip`, `TestOSPFSummaryLSARoundTrip`, `TestOSPFExternalLSARoundTrip` + metric boundary tests + Network-LSA LS ID preservation + Type 7 P-bit
   - Files: `lsa_router.go`, `lsa_network.go`, `lsa_summary.go`, `lsa_external.go`
   - Verify: each body round-trips; 16-bit (Router/Network) vs 24-bit (Summary/External) metric width correct at boundaries; Network-LSA LS ID preserved verbatim (trap #5); E-bit E1/E2; Type 7 P-bit in Options
5. **Phase: Packet bodies** -- Hello, DD, LS Request, LS Update, LS Ack
   - Tests: `TestOSPFHelloRoundTrip`, `TestOSPFDDRoundTrip`, `TestOSPFLSReqRoundTrip`, `TestOSPFLSUpdateRoundTrip`, `TestOSPFLSAckRoundTrip`, `TestOSPFHeaderAuTypeField`
   - Files: `hello.go`, `dbdesc.go`, `lsreq.go`, `lsupdate.go`, `lsack.go`
   - Verify: every packet round-trips; DD I/M/MS bit positions pinned; LS Update LSA iteration consumes exactly Packet Length; AuType field round-trips; wiring test `test/ospf-wire/ospf-packet-1.ci` passes
6. **Phase: Fuzz** -- decode/iterator/round-trip fuzz targets
   - Tests: `FuzzOSPFDecodePacket`, `FuzzOSPFLSAIterator`, `FuzzOSPFRoundTrip`
   - Files: `internal/plugins/ospf/packet/fuzz_test.go`
   - Verify: short fuzz run finds no panic; bound checks confirmed
7. **Functional + real-capture tests** -- `ospf-packet-1.ci` (Hello), `ospf-lsupdate-frr.ci` (real FRR LS Update; both checksums verify), `ospf-truncated.ci` (error path)
8. **RFC refs** -- add `// RFC 2328 Section A.X` (and RFC 905 / RFC 1071 / RFC 3101) comments above enforcing code
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- fill audit tables, write learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; all 5 packet types and all listed LSA types (1,2,3,4,5,7 + opaque passthrough) covered |
| Feature completeness | Every End-to-End User Story has a working path; codec parity with FRR/BIRD for the in-scope packet/LSA set; a real capture decodes |
| Correctness | Both checksum covered ranges exact (packet excludes the 8-byte auth field; LSA excludes LS Age); metric widths per LSA type; Network-LSA LS ID preserved; DD I/M/MS bit positions; AuType 2 Checksum left zero |
| Naming | Exported codec API consistent (`DecodeX`, `(*X).WriteTo`); no IS-IS/BGP coupling; types and checksums from ospf-1 |
| Data flow | Decode is lazy (views, no copy); encode is buffer-first with skip-and-backfill; unknown LSA types retained verbatim |
| CLI grammar | n/a (no new CLI verbs in this child beyond the `ze` decode wiring) |
| Doctor checks | n/a (no runtime dependencies in this child) |
| YANG validation | n/a (no config surface) |
| Prometheus counters | n/a (no runtime state) |
| Rule: buffer-first | no `append`-grown buffers, no `make([]byte)` helpers, no `Len()`-then-`WriteTo()` on the hot path |
| Rule: no-sprintf-alloc | rendering uses `textbuf`/`AppendTo`, never `fmt.Sprintf` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| packet package | `ls internal/plugins/ospf/packet/` |
| all 5 packet codecs | `grep -l 'func Decode' internal/plugins/ospf/packet/{hello,dbdesc,lsreq,lsupdate,lsack}.go` |
| all in-scope LSA codecs | `grep -l 'func Decode' internal/plugins/ospf/packet/lsa_{router,network,summary,external}.go` |
| both checksums + vectors | `go test ./internal/plugins/ospf/packet/ -run TestOSPF.*Checksum` |
| unknown-LSA passthrough | `go test ./internal/plugins/ospf/packet/ -run TestOSPFUnknownLSAPassthrough` |
| fuzz targets | `go test ./internal/plugins/ospf/packet/ -run Fuzz -fuzztime=10s` |
| functional + real-capture tests | `ls test/ospf-wire/ospf-packet-1.ci test/ospf-wire/ospf-lsupdate-frr.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | every packet/LSA length validated before slicing; Packet Length and LSA Length checked against the slice; no read past the buffer; LSA iterator stops on truncation |
| Resource exhaustion | LSA / neighbour / triple / header iteration bounded by the declared lengths; no unbounded loops on crafted Length fields |
| Spoofing | checksum/auth verification is enforced by the instance dispatcher (ospf-4) and ospf-12; this codec must not silently accept a packet whose Packet Length disagrees with the slice |
| Error leakage | decode errors are typed and do not echo raw attacker bytes into logs unbounded |
| Panic safety | fuzz target proves no panic on arbitrary input |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC summary / research guide sec 2 + sec 3 |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Checksum covered-range fails | Re-read guide sec 2 + RFC 1071 / RFC 905; verify the packet excludes the auth field and the LSA excludes LS Age; test encode and verify directions separately |
| Real-capture decode fails | Compare field-by-field with Wireshark; fix the field offset/width |
| Fuzz panic | Add the missing bound check; add the crashing input to the corpus |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Core Insight
The whole package reduces to two invariants and one bookkeeping rule: decode
never copies and never panics (lazy views + bound checks + Length-driven
iteration + opaque LSA retention); encode is buffer-first with both checksums
backfilled last over their EXACT covered ranges (packet RFC 1071 excluding the
8-byte Authentication field, LSA Fletcher-16 excluding the 2-byte LS Age); and
the Network-LSA LS ID is the DR interface address, carried verbatim and never
reinterpreted. Get the two covered ranges and the LS ID preservation right and
the rest is mechanical fixed-field framing.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-packet-type + per-LSA-family files | One monolith per concern (FRR `ospf_packet.c` + `ospf_lsa.c`); one file per field | BIRD middle path: sane file count, clear ownership, matches the umbrella `packet/` layout |
| Lazy decode views over the caller's slice | Eager parsed structs | Buffer-first philosophy; cheap unknown-LSA re-flood; parse bodies on demand |
| Both checksums applied here, algorithms owned by ospf-1 | Reimplement Fletcher/RFC 1071 in the codec | Single source of truth in ospf-1 (RFC 905 / RFC 1071 vectors live there); the codec only chooses the covered range |
| AuType field codec-only here | Full verify/sign + RFC 7474 trailer in this spec | Separation: field framing + zeroed-checksum-for-AuType-2 here, crypto + key store + enforcement in ospf-12 |
| LS ID carried as raw 4 bytes, never per-type reinterpreted | Decode LS ID into a typed prefix/address per LSA type | Avoids trap #5 (Network-LSA LS ID is the DR interface address, not a prefix); SPF (ospf-8) interprets LS IDs, not the codec |
| Opaque LSA types 9/10/11 passthrough-only | Parse opaque TLV bodies now | Origination/parsing is a future framework; verbatim re-flood is the only v1 requirement |

## Known Limitations
- Only LSA types 1, 2, 3, 4, 5, 7 are parsed/originated; opaque types 9/10/11 are decode-as-opaque + verbatim re-encode only (no TLV parsing)
- Per-TOS metric blocks in Router/Summary/External LSAs are not originated (#TOS 0); the obsolete TOS extension is ignored on decode (base metric only), matching FRR/BIRD
- Checksum and authentication VERIFICATION enforcement (reject-on-mismatch) is the instance dispatcher (ospf-4) and ospf-12, not this codec; the codec provides `VerifyChecksum` / `VerifyLSAChecksum` helpers and the AuType field
- On-wire interop is proven by the runtime children (ospf-13), not by this codec child directly (codec is validated by round-trip + checksum vectors + fuzz + a real-capture decode)

## RFC Documentation

Add `// RFC 2328 Section A.X: "<quoted requirement>"` above enforcing code (and
RFC 905 / RFC 1071 / RFC 3101 as applicable). MUST document: common header
field validation (Version 2, Packet Length), the packet checksum covered range
(RFC 2328 §A.3.1 / §D auth, RFC 1071; excludes the 8-byte Authentication field,
Checksum zeroed, left zero for AuType 2), the LSA checksum covered range
(RFC 2328 §12.1.7, RFC 905; excludes LS Age), the signed LS Sequence Number
(§12.1.6), the Network-LSA LS ID semantics (§A.4.3; DR interface address), the
DD I/M/MS flag bit positions (§A.3.3), the Router-LSA link-record layout and
link types (§A.4.2), the 24-bit Summary/External metric (§A.4.4 / §A.4.5), and
the Type 7 P-bit in the Options field (RFC 3101 §2.4).

## Implementation Summary

### What Was Implemented
- Created `internal/plugins/ospf/packet/` as the OSPFv2 wire codec, matching the IS-IS package structure: `doc.go`, `header.go`, per-packet body files, per-LSA body files, checksum application, lazy LSA views, JSON rendering, and fuzz targets.
- Added `internal/plugins/ospf/cli/` with the offline `ze ospf-decode` root command, mirroring `ze isis-decode`: stdin hex/raw input -> `packet.DecodePacket` -> stable JSON output.
- Registered the OSPF wire test suite as `ze-test ospf-wire` and added `make ze-ospf-wire-test`.
- Added `test/ospf-wire/` fixtures for Hello decode, LS Update decode from a public Wireshark capture, and truncated-input error handling.
- Added `docs/architecture/wire/ospf.md` and updated `docs/functional-tests.md` / `Makefile` discovery text for the new OSPF wire suite.

### Bugs Found/Fixed
- `ExternalLSA.WriteTo` initially wrote a 15-byte body by treating the E/TOS byte as part of the 24-bit metric. `TestOSPFExternalLSARoundTrip` failed with `LSA.WriteTo wrote 35, want 36`; fixed by writing E/TOS as one byte, metric as the following 3 bytes, forwarding address at offset 8, and route tag at offset 12.
- The first LS Update `.ci` fixture was synthetic. Replaced it with the public Wireshark `ospf-ls-update-with-41-lsas.pcap` payload extracted from issue #6302 so the codec decodes a real capture.
- Added `types.InternetChecksumPair` / `InternetChecksumPairValid` after packet checksum implementation exposed the non-contiguous OSPF coverage window (packet minus auth bytes 16..23). This keeps the algorithm in `ospf/types` and avoids allocating a temporary concatenated packet.
- Fixed reviewer-found malformed LS Update allocation risk by validating the declared LSA count before allocating the `[]LSA`.
- Fixed reviewer-found opaque passthrough drift by making decoded opaque LSAs keep `RawBytes` authoritative instead of setting a typed `Opaque` body.

### Documentation Updates
- Added `docs/architecture/wire/ospf.md` documenting packet header validation, checksum ranges, LSA body coverage, opaque passthrough, and `ze ospf-decode`.
- Updated `docs/functional-tests.md` with `ospf-wire` runner and `make ze-ospf-wire-test`.
- Updated `Makefile` help and `mk/test-functional.mk` to expose `ze-ospf-wire-test`.

### Deviations from Plan
- Added `internal/plugins/ospf/cli/` because the wiring acceptance criterion requires a runnable `ze ospf-decode` entry point, matching the IS-IS codec child.
- Added `internal/plugins/ospf/packet/json.go` for the offline decode JSON view, matching `internal/plugins/isis/packet/json.go`.
- Added `InternetChecksumPair` to `internal/plugins/ospf/types` so OSPF packet checksum application can exclude the 8-byte auth field without copying.
- `test/ospf-wire/ospf-lsupdate-frr.ci` uses a public Wireshark LS Update capture. The filename follows the spec, but the source issue does not identify the originating daemon as FRR.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Pure packet/LSA codec under `internal/plugins/ospf/packet/` | implemented | `internal/plugins/ospf/packet/doc.go`, `header.go`, `lsa.go` | No runtime sockets, timers, LSDB, or FSM |
| Depends on OSPF types/checksum algorithms, not IS-IS | implemented | `internal/plugins/ospf/packet/checksum.go`; `go list -deps ./internal/plugins/ospf/packet` | Dependency output includes `internal/plugins/ospf/types`; no `internal/plugins/isis` |
| Common 24-byte OSPF header and 5 packet types | implemented | `header.go`, `hello.go`, `dbdesc.go`, `lsreq.go`, `lsupdate.go`, `lsack.go` | Decode dispatch and buffer-first encode |
| LSA header plus Types 1,2,3,4,5,7 and opaque passthrough | implemented | `lsa.go`, `lsa_router.go`, `lsa_network.go`, `lsa_summary.go`, `lsa_external.go`, `lsa_opaque.go` | Opaque 9/10/11 raw-body passthrough |
| Packet checksum and LSA checksum application | implemented | `packet/checksum.go`, `types/checksum.go` | Packet excludes auth; LSA excludes LS Age |
| Offline `ze ospf-decode` wiring | implemented | `internal/plugins/ospf/cli/*.go`, `cmd/ze/ze_core_dispatch.go` | Functional suite passes |
| Functional test suite | implemented | `test/ospf-wire/*.ci`, `internal/test/cli/register.go`, `mk/test-functional.mk` | `make ze-ospf-wire-test` passes 3/3 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | implemented | `TestOSPFHeaderRoundTrip`, `TestOSPFHeaderRejectsBadVersionAndLength` | Version, type, Packet Length, Router ID, Area ID, AuType/Auth exposed |
| AC-2 | implemented | `TestOSPFHelloRoundTrip`, `TestOSPFDDRoundTrip`, `TestOSPFLSReqRoundTrip`, `TestOSPFLSUpdateRoundTrip`, `TestOSPFLSAckRoundTrip` | All packet bodies round-trip |
| AC-3 | implemented | `TestOSPFPacketChecksum`, `TestOSPFPacketChecksumExcludesAuth` | RFC 1071 excludes auth field |
| AC-4 | implemented | `TestOSPFPacketChecksumZeroForAuType2`, `TestOSPFHeaderAuTypeField` | AuType 2 checksum stays zero; auth field framing exposed |
| AC-5 | implemented | `TestOSPFLSAChecksum`, `TestOSPFLSAChecksumExcludesAge` | Fletcher over `lsa[2:]`; Length includes header |
| AC-6 | implemented | `TestInternetChecksumPairMatchesConcatenatedWindow` in `ospf/types`, packet checksum tests, LSA checksum tests | Algorithms owned by `ospf/types`, applied by packet |
| AC-7 | implemented | `TestOSPFRouterLSARoundTrip` | Flags plus p2p/transit/stub/virtual links, metric boundary |
| AC-8 | implemented | `TestOSPFNetworkLSARoundTrip` | Network-LSA LS ID preserved verbatim |
| AC-9 | implemented | `TestOSPFSummaryLSARoundTrip` | Types 3/4 and 24-bit metric max |
| AC-10 | implemented | `TestOSPFExternalLSARoundTrip` | Type 5 E-bit, metric, forwarding address, route tag |
| AC-11 | implemented | `TestOSPFExternalLSARoundTrip` | Type 7 uses Type 5 body and preserves Options N/P bit |
| AC-12 | implemented | `TestOSPFLSUpdateRoundTrip` | Count and Length-driven LSA iteration |
| AC-13 | implemented | `TestOSPFDDRoundTrip` | I/M/MS bits pinned across all 8 combinations |
| AC-14 | implemented | `TestOSPFLSReqRoundTrip` | 12-byte request triples |
| AC-15 | implemented | `TestOSPFLSAckRoundTrip` | Consecutive 20-byte LSA headers |
| AC-16 | implemented | `TestOSPFUnknownLSAPassthrough` | Decode and re-encode is byte-identical |
| AC-17 | implemented | `TestOSPFLSAIteratorTruncated`, `TestOSPFLSUpdateRejectsHugeCountBeforeAllocation`, `FuzzOSPFDecodePacket`, `FuzzOSPFLSAIterator`, `test/ospf-wire/ospf-truncated.ci` | No panic; typed error path; count bounded before allocation |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Header and AuType tests | pass | `header_test.go` | Constants, header round-trip, bad version/length, AuType 2 field |
| Packet body round-trip tests | pass | `packet_body_test.go` | Hello, DD, LSReq, LSUpdate, LSAck |
| Checksum tests | pass | `checksum_test.go`, `types/checksum_test.go` | Packet auth exclusion, AuType2 zero, LSA age exclusion, two-segment RFC1071 |
| LSA header/body tests | pass | `lsa_test.go` | Router, Network, Summary, External/NSSA, opaque, iterator truncation |
| Fuzz targets | pass | `fuzz_test.go` | `FuzzOSPFDecodePacket`, `FuzzOSPFLSAIterator`, `FuzzOSPFRoundTrip` |
| Functional fixtures | pass | `test/ospf-wire/*.ci` | Hello, public LS Update capture, truncated error |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/ospf/packet/*.go` | created | Includes one extra helper file `wire.go` and JSON view `json.go` |
| `internal/plugins/ospf/packet/*_test.go` | created | Includes planned unit/fuzz coverage |
| `internal/plugins/ospf/cli/*.go` | created | Offline decode command matching IS-IS |
| `test/ospf-wire/*.ci` | created | Functional wire fixtures |
| `docs/architecture/wire/ospf.md` | created | Wire codec architecture doc |
| `docs/functional-tests.md`, `mk/test-functional.mk`, `Makefile`, `internal/test/cli/register.go`, `cmd/ze/ze_core_dispatch.go` | updated | Discovery, runner, and root command wiring |

### Audit Summary
- **Total items:** 17 ACs, 7 task requirements, 6 test groups, 6 file groups.
- **Done:** Codec, CLI wiring, docs, unit tests, fuzz targets, functional tests.
- **Partial:** `make ze-validate` still reports exported OSPF packet/type APIs without cross-package non-test callers; later runtime children consume them. The LS Update capture is real but not proven FRR-originated by its source issue.
- **Skipped:** none.
- **Changed:** Added JSON/CLI files and `InternetChecksumPair` support to satisfy zero-copy checksum application and wiring proof.

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Round-trip every packet type | unit test | `go test ./internal/plugins/ospf/types ./internal/plugins/ospf/packet ./internal/plugins/ospf/cli ./internal/test/cli` passed; packet tests listed above |
| Round-trip every LSA type | unit test | `TestOSPFRouterLSARoundTrip`, `TestOSPFNetworkLSARoundTrip`, `TestOSPFSummaryLSARoundTrip`, `TestOSPFExternalLSARoundTrip`, `TestOSPFUnknownLSAPassthrough` passed |
| Both checksums correct over the right covered range | unit test | `TestOSPFPacketChecksum*`, `TestOSPFLSAChecksum*`, and `TestInternetChecksumPairMatchesConcatenatedWindow` passed |
| Unknown-LSA verbatim re-flood | unit test | `TestOSPFUnknownLSAPassthrough` passed |
| Decoder never panics on arbitrary input | fuzz test | `go test ./internal/plugins/ospf/packet -run '^$' -fuzz=FuzzOSPFDecodePacket -fuzztime=2s`, `FuzzOSPFLSAIterator`, and `FuzzOSPFRoundTrip` all passed |
| Decode a real capture | functional test | `make ze-ospf-wire-test` passed 3/3 with `test/ospf-wire/ospf-lsupdate-frr.ci` using Wireshark `ospf-ls-update-with-41-lsas.pcap` payload |
| Codec wires end-to-end to the user | functional test | `make ze-ospf-wire-test` passed 3/3; `test/ospf-wire/ospf-packet-1.ci` exercises `ze ospf-decode` Hello JSON output |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `make ze-validate` reports exported OSPF packet/type APIs with no cross-package non-test callers | `internal/plugins/ospf/packet/*.go`, `internal/plugins/ospf/types/*.go` | Expected until OSPF runtime children consume codec/type APIs; keep tracking across later children |
| 2 | ISSUE | `DecodeLSUpdate` used the untrusted 32-bit LSA count as slice capacity before checking how many LSAs could fit in the declared body | `internal/plugins/ospf/packet/lsupdate.go` | Fixed by bounding count against `(len(body)-4)/20` before allocation and adding `TestOSPFLSUpdateRejectsHugeCountBeforeAllocation` |
| 3 | ISSUE | Decoded opaque LSAs set `Opaque`, disabling the `RawBytes` copy branch and allowing checksum normalization instead of byte-for-byte passthrough | `internal/plugins/ospf/packet/lsa.go` | Fixed by leaving decoded opaque LSAs as raw views; JSON renders opaque from `Header.Type` + `Body`; test asserts `decoded.Opaque == nil` and byte equality |
| 4 | PASS | `/ze-review` fix re-run found 0 BLOCKER / 0 ISSUE | `internal/plugins/ospf/packet/lsupdate.go`, `lsa.go`, tests | No further action |

### Fixes applied
- Bounded LS Update count before allocation and added regression coverage for count `0xffffffff` with no LSA space.
- Preserved decoded opaque LSAs as raw byte views so `LSA.WriteTo` copies `RawBytes` for re-flood passthrough.
- Re-ran `go test ./internal/plugins/ospf/types ./internal/plugins/ospf/packet ./internal/plugins/ospf/cli ./internal/test/cli`, `make ze-ospf-wire-test`, and all three packet fuzz targets after the fixes; all passed.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | PASS | Reviewer re-run confirmed LS Update count is bounded before allocation and decoded opaque LSAs copy `RawBytes` byte-for-byte | `internal/plugins/ospf/packet/lsupdate.go`, `lsa.go` | Continue to `spec-ospf-3-ip-transport.md`; exported API validation remains tracked for later runtime consumers |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled - 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/packet/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features (opaque LSAs passthrough-only; no opaque TLV parsing)
- [ ] Single responsibility (codec only; no runtime)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (ospf-1 types + checksums only; no IS-IS/BGP)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests (covered by ospf-13; codec validated by round-trip + checksum vectors + fuzz + real-capture decode)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-2-wire.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary
- [ ] **Commit B:** `git rm plan/spec-ospf-2-wire.md` only
