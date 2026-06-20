# Spec: ospf-1-types

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - umbrella scope, package layout, dependency graph, "Shared Contracts (canonical)" (this is row ospf-1)
4. `docs/research/ospf-implementation-guide.md` - §3 LSA header + per-type bodies (lines 140-243), §4 Domain Types and Constraints (lines 217-239), §13 Known Hard Problems (Fletcher §13.1, sequence §13.2, max-age §13.3, clock vs monotonic §13.14, lines 1432-1506), §2 common header + checksums (lines 65-137)
5. `internal/component/ospf/types/` - the package this spec creates (does not exist yet)
6. `plan/spec-isis-1-types.md` - the sibling IS-IS leaf-type spec; OSPF mirrors its leaf-package conventions but shares no code

## Task

Create the OSPFv2 domain type leaf package `internal/component/ospf/types/`. This
is phase 1 of the OSPF umbrella (`plan/spec-ospf-0-umbrella.md`) and the bottom
layer of the layered, leaf-first package design declared there: `types` (leaf)
<- `packet` codec (ospf-2) <- `transport`/`iface`/`neighbor`/`lsdb`/`spf` runtime
(ospf-3..ospf-13). This package implements the pure value types that every higher
OSPF layer keys on, with no network I/O, no timers, no goroutines, and no imports
from the OSPF runtime. It is the OSPFv2 equivalent of a self-contained
address/identifier/checksum library, the OSPF counterpart to the IS-IS
`internal/component/isis/types/` package created by `plan/spec-isis-1-types.md`.

The types and their meanings come from RFC 2328 (OSPF Version 2, §12 LSA
structure, §A.4 LSA layouts, §B architectural constants) as distilled in
`docs/research/ospf-implementation-guide.md` §3-§4, and from the two checksum
standards the umbrella "Two distinct checksums" contract names: RFC 905 Annex /
ISO 8473 (Fletcher-16, for LSAs) and RFC 1071 (the IP one's-complement Internet
checksum, for packets). The set is:

| Type | Width / shape | Meaning |
|------|---------------|---------|
| `RouterID` | 4-byte fixed value, dotted-quad text | 32-bit router identifier; uniquely names a router in the AS |
| `AreaID` | 4-byte fixed value, dotted-quad OR integer text; `0.0.0.0` = backbone | 32-bit scalar area identifier (no structural hierarchy, unlike IS-IS area addresses) |
| `LSAKey` | tuple `(LSType, LinkStateID, AdvertisingRouter)` (1 + 4 + 4 bytes packed) | The LSDB lookup key; identity of an LSA, EXCLUDING sequence/age/checksum |
| `LSType` | 1 byte (1 Router, 2 Network, 3/4 Summary, 5 AS-External, 7 NSSA; 9/10/11 Opaque out of scope) | LSA type discriminator inside `LSAKey` |
| `LinkStateID` | 4-byte value, dotted-quad text | Type-specific LSA identifier (Router ID, DR interface address, network prefix, ...) |
| `LSSequenceNumber` | signed 32-bit; `InitialSequenceNumber` 0x80000001, `MaxSequenceNumber` 0x7FFFFFFF, 0x80000000 reserved/never used | LSA version, with the RFC 2328 §13.1 "greater than" comparison and wraparound rule |
| `LSAge` | 16-bit seconds, 0..3600 (`MaxAge`), DoNotAge bit 0x8000 | LSA age; monotonic-clock-based ageing (§13.14); low 15 bits are the age, high bit is DoNotAge |
| `Metric` | 16-bit interface output cost (1..65535; LSA-body summary/external metrics are 24-bit, see note) | Link cost; lower is better; `ReferenceBandwidth / InterfaceBandwidth`, at least 1 |
| `Options` | 1 byte of capability bits (E, MC, N/P, L, DC, O, DN) | OSPF Options field carried in Hellos, DD, and the LSA header |

For each type the package provides: parse from a printable string (dotted-quad,
or integer for `AreaID`), parse from wire bytes, format for display, equality,
ordering where it is semantically meaningful (notably `LSAKey` and
`LSSequenceNumber`, used to index the LSDB and decide freshness), and
buffer-first byte serialization consistent with `ai/rules/buffer-first.md`.

This package ALSO owns the two checksum ALGORITHMS (not their application): the
Fletcher-16 LSA checksum and the IP one's-complement Internet checksum, each with
the byte-range conventions the umbrella "Two distinct checksums" contract fixes.
ospf-2 (codec) and ospf-7 (LSA re-origination) APPLY them; this spec only
implements and vector-tests the raw algorithms.

Ze has no OSPF types today; `internal/component/ospf/` does not exist. The
closest in-tree artefact is the IS-IS Fletcher implementation
(`internal/component/isis/packet/checksum.go`), which uses the SAME Fletcher-16
algorithm but over a DIFFERENT covered range (IS-IS covers from byte 12; OSPF
covers from the Options field, byte 2, excluding LS Age). The two are NOT shared
(umbrella "Separate from IS-IS"): OSPF gets its own implementation, vector-tested
independently. This package is brand new and stands alone; it is consumed first
by the wire codec spec `spec-ospf-2-wire.md`, and transitively by every later
OSPF child (notably `spec-ospf-7-lsdb-flooding.md` for the LSDB keyed on
`LSAKey` and the §13.1 freshness compare, and `spec-ospf-8-spf-rib.md` for SPF).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations -- these survive compaction. -->
- [ ] `docs/research/ospf-implementation-guide.md` §3-§4 (lines 140-243) - LSA common header, per-type body layouts, and the authoritative Domain Types and Constraints list
  → Decision: implement `RouterID`/`AreaID`/`LinkStateID` as 4-byte fixed comparable values displayed as dotted quad; `AreaID` ALSO parses an integer form (`0` == `0.0.0.0`); `LSAKey` is the `(LSType, LinkStateID, AdvertisingRouter)` triple, comparable and usable directly as a Go map key
  → Constraint: `LSAKey` equality and hashing must NOT include the sequence number, age, or checksum -- those are the LSA "version", not its identity (guide §4 "Comparison does not include the sequence number")
  → Constraint: a Router ID stored as `net.IP` and compared by slice identity is a known bug (guide §4 final paragraph); use a 4-byte array value type compared by `==`
- [ ] `docs/research/ospf-implementation-guide.md` §13 (lines 1432-1506) - Known Hard Problems / traps that constrain THESE types
  → Constraint: §13.1 Fletcher-16 is computed over the LSA EXCLUDING LS Age (bytes 0-1) but INCLUDING the checksum field position; the checksum field is zeroed for the forward computation and the RFC 905 / ISO 8473 adjustment places the result -- test encode AND decode against RFC 905 Annex B vectors (a common bug is encode-correct / verify-wrong, so self-interop passes and cross-interop fails)
  → Constraint: §13.2 sequence comparison is signed but freshness-aware: `0x80000001` (InitialSequenceNumber) is OLDEST, `0x7FFFFFFF` (MaxSequenceNumber) is NEWEST; a naive "higher is newer" fails at wraparound. `0x80000000` is reserved and never used on the wire
  → Constraint: §13.3 max-age -- `MaxAge` (3600) is the purge marker; the TYPE must represent it distinctly (an `IsMaxAge()` predicate) so the runtime (ospf-7) can retain purges, but retention itself is a runtime concern
  → Constraint: §13.14 LS Age is wall-clock seconds but must be derived from a MONOTONIC origination timestamp as a delta; this spec's `LSAge` is the value type, the monotonic-clock policy is enforced by the runtime, but `LSAge` exposes the DoNotAge bit (0x8000) and a "ageing arithmetic" helper that saturates at `MaxAge`
- [ ] `plan/spec-ospf-0-umbrella.md` "Shared Contracts (canonical)" - the cross-spec contracts this package must satisfy verbatim
  → Constraint: "Two distinct checksums" -- packet checksum is RFC 1071 IP one's-complement over the whole packet EXCLUDING the 8-byte Authentication field (bytes 16..23) with the Checksum field zeroed; LSA checksum is Fletcher-16 starting at the Options field (EXCLUDING LS Age) with the LS Checksum field treated as zero during the forward computation. This spec owns both ALGORITHMS; ospf-2 applies them
  → Constraint: "LSA header + body layout" -- the 20-byte LSA header is LS Age(2), Options(1), LS Type(1), Link State ID(4), Advertising Router(4), LS Sequence Number(4), LS Checksum(2), Length(2). `LSAKey` reads LS Type, Link State ID, Advertising Router from this header
  → Decision: `types` is a leaf package and MUST NOT import anything from the OSPF runtime (`packet`, `transport`, `iface`, `neighbor`, `lsdb`, `spf`, the component root) nor from IS-IS; it may import only the Go standard library (and Ze leaf helpers such as `internal/core/textbuf` for zero-alloc formatting)
- [ ] `plan/spec-isis-1-types.md` - the sibling leaf-type spec; OSPF mirrors its leaf-package conventions, boundary-test rigour, and zero-alloc `String()` discipline
  → Constraint: copy the conventions (value-typed fixed identifiers comparable with `==`; `Parse*` for strings, `*FromBytes` for wire; buffer-first `WriteTo(buf, off) int`; reserved/never-valid numeric values exposed via an explicit predicate). Do NOT couple OSPF to IS-IS code: the Fletcher covered range and the metric semantics differ
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, no-alloc encode
  → Constraint: byte serialization is buffer-first: write into a caller-supplied buffer at an offset and return the byte count; do not allocate a fresh slice per call. The Fletcher and Internet checksum functions read a caller-supplied `[]byte` window and return a `uint16`, allocating nothing
- [ ] `ai/rules/no-sprintf-alloc.md` - string building without per-call allocation on hot paths
  → Constraint: dotted-quad `String()` for `RouterID`/`AreaID`/`LinkStateID` must avoid `fmt.Sprintf` allocation in any path the codec or CLI calls repeatedly; use a `textbuf`-style append or a fixed-width dotted-quad helper
- [ ] `ai/rules/go-standards.md` - value-typed, no cross-boundary pointers
  → Constraint: fixed-width identifiers (`RouterID`, `AreaID`, `LinkStateID`) and the `LSAKey` triple are value-typed (4-byte arrays / a plain comparable struct) so they are comparable with `==` and usable directly as map keys (the LSDB index)

### RFC Summaries (MUST for protocol work; created via `/ze-rfc` at implementation time)
<!-- Summaries are created at implementation time per the umbrella RFC Coverage table; the
     validate-spec / RFC-summary hooks require them before protocol code lands, not now. -->
- [ ] RFC 2328 short summary (`rfc/short/rfc2328.md`, to create) - OSPF Version 2 base: §12 LSA structure, §13.1 freshness compare, §A.4 LSA layouts, §B constants (`MaxAge` 3600, `LSRefreshTime` 1800, `MaxAgeDiff` 900, `CheckAge` 300, `InitialSequenceNumber` 0x80000001, `MaxSequenceNumber` 0x7FFFFFFF)
  → Constraint: `LSSequenceNumber` is a signed 32-bit value; origination starts at `InitialSequenceNumber`; at `MaxSequenceNumber` the originator flushes via `MaxAge` then re-originates at `InitialSequenceNumber` (the type exposes the comparison and the wraparound boundary, not the runtime flush)
  → Constraint: §13.1 freshness orders by sequence first, then by LS Age (a `MaxAge` copy is newer), then by checksum (higher Fletcher value is newer) only as a final tiebreak; this spec's `LSSequenceNumber.Newer` covers the sequence step, and a documented freshness helper exposes the full ordering for ospf-7
- [ ] RFC 905 short summary (`rfc/short/rfc905.md`, to create) - ISO transport Fletcher-16 checksum (Annex), the SAME algorithm IS-IS uses (ISO 8473)
  → Constraint: the Fletcher-16 covered range for OSPF starts at the Options field (byte 2), EXCLUDING the 2-byte LS Age; the LS Checksum field is treated as zero during the forward computation; the RFC 905 Annex B adjustment derives the two-byte result. Test against RFC 905 Annex B vectors
- [ ] RFC 1071 short summary (`rfc/short/rfc1071.md`, to create) - the Internet (IP) one's-complement checksum
  → Constraint: the packet checksum sums 16-bit words in one's-complement over the whole OSPF packet EXCLUDING the 8-byte Authentication field, with the Checksum field zeroed during computation; verify by re-summing including the stored checksum and checking the result is 0xFFFF. Test against RFC 1071 vectors

**Key insights:** (minimal context to resume after compaction)
- The domain types are all immutable keys, counters, or bit-fields; correctness risk is parse/format/compare/serialize round-trip fidelity plus the TWO checksum algorithms (Fletcher-16 over LSA-minus-LS-Age; Internet checksum over packet-minus-Auth), which are fully unit-testable against RFC 905 / RFC 1071 vectors with no network
- `LSAKey` ordering and equality are load-bearing: the LSDB (ospf-7) is keyed on `(LSType, LinkStateID, AdvertisingRouter)` and `show ip ospf database` (ospf-13) lists by it; `LSSequenceNumber` freshness comparison (§13.1/§13.2) gates whether a received LSA replaces the stored copy (ospf-7)
- `LSAge` carries the DoNotAge bit (0x8000) and the `MaxAge` (3600) purge marker; the value type exposes these distinctly, the runtime (ospf-7) enforces monotonic-clock ageing and purge retention
- This is a leaf package: stdlib-only, zero imports from the OSPF runtime, from IS-IS, or from BGP-LS

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey)
- [ ] `internal/component/isis/packet/checksum.go` - the existing in-tree Fletcher-16, over the IS-IS covered range (from byte 12); read to mirror the algorithm shape, NOT to share it
  → Constraint: OSPF needs its own Fletcher with the OSPF covered range (from the Options field, byte 2, excluding LS Age); do NOT import or reuse the IS-IS one (different covered range, different LSA header layout)
- [ ] `internal/component/isis/types/` - the sibling leaf-type package created by `spec-isis-1-types.md`; read for the value-typed-identifier / `Parse*` / `*FromBytes` / buffer-first `WriteTo` conventions
  → Constraint: mirror the conventions (fixed-array value identifiers, explicit reserved-value predicates, zero-alloc `String()`); do NOT import it (independent type domains)
- [ ] `internal/core/textbuf` - allocation-light string building used elsewhere in Ze
  → Constraint: reuse the established hex/append pattern for dotted-quad `String()` rather than inventing a new formatter; keep `String()`/`AppendTo` zero-alloc on the hot path

**Behavior to preserve:** (unless the user explicitly said to change)
- Nothing exists to preserve in OSPF: there are no existing OSPF callers; this package introduces the API, it does not change one
- BGP, BGP-LS, IS-IS, RSVP-TE, MRT decoders remain untouched and independent; OSPF does not refactor the IS-IS Fletcher in place
- The EXISTING `rib.admin-distance.ospf` leaf (default 110) and the sysrib ECMP path-group expansion are unchanged and are NOT touched by this leaf type package (they are consumed by ospf-8)

**Behavior to change:** (only what the user explicitly requested)
- New package `internal/component/ospf/types/` with the nine value types and their methods, plus the two checksum algorithms (Fletcher-16 and Internet checksum)
- No change to any other package; redistribution, component wiring, and the admin-distance reuse are owned by later specs (ospf-4, ospf-8, ospf-10)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Strings from config and CLI: an operator-configured `router-id 10.0.0.1`, an `area 0` or `area 0.0.0.0`, a `show ip ospf database` filter naming an LSA by `(type, link-state-id, advertising-router)`
- Bytes from the wire: the 24-byte OSPF common header and the 20-byte LSA header handing raw octet slices to constructors (consumed by `spec-ospf-2-wire.md`)
- Byte windows for checksum: a caller-supplied `[]byte` over an OSPF packet (Internet checksum) or over an LSA (Fletcher-16), with the field-zeroing conventions applied by the caller (ospf-2)
- Format at entry: printable dotted-quad (or integer for `AreaID`) strings, or big-endian octet slices of the exact fixed length

### Transformation Path
1. **Parse:** a printable string (`ParseRouterID`, `ParseAreaID`, `ParseLinkStateID`) or a byte slice (`RouterIDFromBytes`, `LSAKeyFromHeader`, `LSAgeFromBytes`, ...) produces a typed value, validating length and shape
2. **Compare / key:** typed values are compared for equality and (where meaningful) ordered; `LSAKey` is used directly as a Go map key (the LSDB index in ospf-7); `LSSequenceNumber.Newer` and the freshness helper decide whether a received LSA replaces the stored copy
3. **Checksum:** `FletcherChecksum(buf)` returns the Fletcher-16 over the LSA-minus-LS-Age window; `InternetChecksum(buf)` returns the RFC 1071 one's-complement over the packet-minus-Auth window; both allocate nothing and verify by re-running over the stored value
4. **Format:** `String()` renders the canonical printable form (dotted quad for identifiers, decimal for metric/age/sequence) for CLI / web / logs
5. **Serialize:** `WriteTo(buf, off) int` writes the big-endian octets into a caller buffer for the wire codec, returning the byte count

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config / CLI string <-> type | `Parse*` constructors and `String()` formatters | [ ] |
| Wire bytes <-> type | `*FromBytes` / `LSAKeyFromHeader` constructors and `WriteTo(buf, off)` serializers | [ ] |
| Type <-> map-key / set membership | value-typed fixed arrays + the `LSAKey` comparable struct usable directly as Go map keys | [ ] |
| Byte window <-> checksum value | `FletcherChecksum(buf) uint16`, `InternetChecksum(buf) uint16` (no allocation, no payload mutation) | [ ] |

### Integration Points
- Consumed by `spec-ospf-2-wire.md` (common-header Router ID / Area ID / packet Checksum; LSA header `LSAKey` / `LSSequenceNumber` / `LSAge` / `Options` / LSA Fletcher checksum)
- Consumed by `spec-ospf-5-interface-ism.md` (Hello Options E/N bits gate adjacency; Router ID / DR `LinkStateID` interface address)
- Consumed by `spec-ospf-7-lsdb-flooding.md` (LSDB keyed on `LSAKey`; §13.1 freshness via `LSSequenceNumber` + `LSAge`; `MaxAge` purge marker; Fletcher refresh on re-origination)
- Consumed by `spec-ospf-8-spf-rib.md` (Router ID / `LinkStateID` graph keys; `Metric` accumulation)
- No upstream dependency: this package imports only the Go standard library (and Ze leaf helpers such as `textbuf`)

### Architectural Verification
- [ ] No bypassed layers (types are pure; higher layers call constructors and the checksum functions, never reach past them)
- [ ] No unintended coupling (leaf package: zero imports from the OSPF runtime, from IS-IS, or from BGP-LS; OSPF Fletcher is independent of the IS-IS Fletcher)
- [ ] No duplicated functionality (single canonical `RouterID`/`AreaID`/`LSAKey`/checksum; later specs reuse, do not re-declare)
- [ ] Zero-copy preserved where applicable (serialize is buffer-first; checksum reads a caller window without copying or mutating; parse may copy 4-byte arrays since they are small value types)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The nine types in guide §4 plus the two checksum algorithms are the complete leaf-type set needed by ospf-2 | `docs/research/ospf-implementation-guide.md` lines 217-239 + umbrella "Two distinct checksums" | ospf-2 discovers a missing type (e.g. `DDSequenceNumber`, `InterfaceID`) and must add it here | ospf-2 wire codec compiles against this package without adding new identifier/checksum types | unvalidated |
| A-2 | A 4-byte array value type (`RouterID`/`AreaID`/`LinkStateID`) and a plain comparable `LSAKey` struct are acceptable Go map keys for the LSDB index | `ai/rules/go-standards.md` value-typed identifiers; guide §3 "per-area map keyed on this triple" | LSDB keying needs a different representation | ospf-7 LSDB uses `LSAKey` directly as a map key and compiles | unvalidated |
| A-3 | OSPF needs its OWN Fletcher (covered range from the Options field, byte 2) and must NOT reuse the IS-IS Fletcher (covered range from byte 12) | umbrella "Separate from IS-IS" + `isis/packet/checksum.go` covered range | The covered ranges turn out identical and one implementation suffices | RFC 905 Annex B vector test passes over the OSPF window; an IS-IS-range Fletcher fails the OSPF vector | unvalidated |
| A-4 | `Metric` as a 16-bit interface output cost is the right leaf type; the 24-bit Summary/External LSA-body metric is decoded inline by ospf-2 (not a separate leaf type) | guide §4 "typically 16 bits for link cost, 24 bits for LSA metrics"; umbrella LSA body layout | ospf-9/ospf-10 need a dedicated 24-bit metric leaf type | ospf-2 encodes the 24-bit Summary/External metric inline against a 3-byte helper, or escalates to add a leaf type here | unvalidated |
| A-5 | `LSSequenceNumber` signed-comparison freshness (§13.2) is fully expressible in the leaf type; the runtime only supplies the LS Age / checksum tiebreak inputs | guide §13.2; RFC 2328 §13.1 | The full §13.1 freshness needs runtime context the leaf type cannot model | ospf-7 freshness decision is implemented with `LSSequenceNumber.Newer` + a documented LS-Age/checksum tiebreak helper, no re-implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fletcher-16 implemented over the wrong covered range (including LS Age, or starting at the IS-IS offset) so self-interop passes but FRR cross-interop fails | ospf-2 round-trip passes, FRR interop checksum mismatch | RFC 905 Annex B vector test in THIS spec over the exact OSPF window (from Options, excluding LS Age) before any runtime; a covered-range boundary test (flipping an LS-Age byte must NOT change the checksum) |
| R-2 | Internet checksum includes the Authentication field, or fails to zero the Checksum field, so packets are rejected | ospf-2 packet verify fails; auth (ospf-12) always mismatches | RFC 1071 vector test in THIS spec; explicit "Auth field excluded" and "Checksum field zeroed" boundary tests |
| R-3 | `LSSequenceNumber` compared as unsigned / "higher is newer", losing updates at wraparound (`0x80000001` vs `0x7FFFFFFF`) | ospf-7 origination/flap tests fail or loop | implement the §13.2 signed freshness compare; boundary test `0x80000001` (oldest) vs `0x7FFFFFFF` (newest) and `0x7FFFFFFF` vs `0x7FFFFFFF`; `0x80000000` reserved, never produced |
| R-4 | `LSAge` DoNotAge bit (0x8000) not masked, so a frozen LSA reads as age >= 32768 and is mistaken for a purge | a DoNotAge LSA is purged spuriously in ospf-7 | mask the low 15 bits for the age; expose `DoNotAge()` and `IsMaxAge()` distinctly; boundary tests at 3600, 0x8000, 0x8000|3600 |
| R-5 | `LSAKey` accidentally includes sequence/age/checksum in equality, so a refreshed LSA is treated as a different LSA and the LSDB grows unbounded | ospf-7 LSDB has duplicate keys per LSA | `LSAKey` is exactly `(LSType, LinkStateID, AdvertisingRouter)`; an explicit test that two headers differing only in sequence/age/checksum yield the SAME key |
| R-6 | `String()` allocates per call on a hot path (CLI list, log, `show ip ospf database`) | benchmark allocation in ospf-13 | buffer-first dotted-quad append per `ai/rules/no-sprintf-alloc.md`, with a bench guarding zero-alloc |
| R-7 | `AreaID` integer form (`area 0`) and dotted-quad form (`area 0.0.0.0`) parse to different values | config round-trip mismatch; backbone not recognised | a single `ParseAreaID` accepts both forms and normalises; test `0` == `0.0.0.0` == backbone |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- This is a leaf type package: its "entry point" is its public API, exercised by tests
     in this package and, at the next layer, by the wire codec (spec-ospf-2-wire.md).
     The wiring chain proven here is parse -> compare/key -> checksum -> format -> serialize
     round-trip, plus the consumption proof that the wire codec links against these types.
     There is no new .ci here: user-facing functional tests live in the consuming specs that
     produce observable CLI / wire behaviour (ospf-2/4/13). This is the standing leaf-type
     pattern from spec-isis-1-types.md; the existing OSPF .ci suite is owned downstream. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| operator string `router-id 10.0.0.1` | → | `ParseRouterID` builds a `RouterID`; `String()` returns `10.0.0.1` | `TestRouterIDParseFormatRoundTrip` |
| operator string `area 0` and `area 0.0.0.0` | → | `ParseAreaID` normalises both to the backbone value; `IsBackbone()` true | `TestAreaIDIntegerAndDottedForms` |
| wire octets of a 20-byte LSA header | → | `LSAKeyFromHeader` builds the `(type, link-state-id, advertising-router)` key; `LSAge`/`LSSequenceNumber`/`Options` decode | `TestLSAKeyFromHeader` |
| an LSA byte window (from Options, LS-Age zeroed) | → | `FletcherChecksum` returns the RFC 905 Annex B vector result | `TestFletcherRFC905Vectors` |
| an OSPF packet byte window (Auth excluded, Checksum zeroed) | → | `InternetChecksum` returns the RFC 1071 vector result | `TestInternetChecksumRFC1071Vectors` |
| ospf-2 wire codec references these types | → | `internal/component/ospf/packet` imports `types` and builds headers/LSAs (existing OSPF .ci suite, owned downstream by ospf-2/13) | `spec-ospf-2-wire.md` build + its round-trip/fuzz tests (downstream wiring proof) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ParseRouterID("10.0.0.1")` | Returns a 4-byte `RouterID`; its `String()` is `10.0.0.1` (round-trip); `RouterIDFromBytes` over the same 4 octets is equal |
| AC-2 | `RouterIDFromBytes` / `AreaIDFromBytes` / `LinkStateIDFromBytes` with a slice of length != 4 | Returns an error; no partial value leaks |
| AC-3 | `ParseAreaID("0")`, `ParseAreaID("0.0.0.0")` | Both return the backbone `AreaID`; `IsBackbone()` true; `String()` is the canonical dotted-quad `0.0.0.0`; a non-zero integer and its dotted-quad equivalent are also equal |
| AC-4 | `LSAKey{LSType, LinkStateID, AdvertisingRouter}` from a 20-byte LSA header; a second header identical except for sequence/age/checksum | Both yield the SAME `LSAKey` (equality excludes version fields); the key is usable as a Go map key |
| AC-5 | `LSSequenceNumber` `0x80000001` vs `0x7FFFFFFF`; `0x7FFFFFFF` vs `0x7FFFFFFF` | `0x7FFFFFFF` is reported newer than `0x80000001` (§13.2 signed freshness); equal sequences are not "newer" either way; `0x80000000` is reported reserved/never-used |
| AC-6 | `LSSequenceNumber` at `MaxSequenceNumber` (0x7FFFFFFF) | Reported as the wraparound boundary (`IsMax()` true); the documented re-origination rule (flush via `MaxAge`, restart at `InitialSequenceNumber`) is exposed for ospf-7; the increment helper never produces `0x80000000` |
| AC-7 | `LSAge` of 0, 3600, 0x8000, and 0x8000\|3600 | 0..3600 representable; `IsMaxAge()` true at 3600; `DoNotAge()` true when bit 0x8000 set; the masked age ignores the DoNotAge bit; an ageing helper saturates at `MaxAge` |
| AC-8 | `Metric` of 1 and 65535 (interface output cost) | Both representable; 16-bit serialize; default-cost derivation (`ReferenceBandwidth / bandwidth`, floored at 1) is exposed as a helper |
| AC-9 | `Options` with E, N/P, MC, DC, O, DN bits | Each bit set/cleared/tested independently; `String()` lists the set bits; round-trips through the 1-byte serialize |
| AC-10 | `FletcherChecksum` over an RFC 905 Annex B test vector (LSA window from Options, LS Age excluded, checksum field zeroed) | Returns the documented vector result; flipping a byte in the excluded LS-Age region does NOT change the result |
| AC-11 | `InternetChecksum` over an RFC 1071 test vector (packet window, Auth field excluded, Checksum field zeroed) | Returns the documented vector result; re-summing including the stored checksum yields 0xFFFF |
| AC-12 | `WriteTo(buf, off)` for every type | Writes the exact big-endian octets and returns the correct count; bytes match the `*FromBytes` input (serialize/parse round-trip) |
| AC-13 | `RouterID` / `AreaID` / `LinkStateID` / `LSAKey` used as Go map keys | Compile and behave as comparable value types (no pointer identity surprises; `net.IP`-slice comparison bug avoided) |

## End-to-End User Stories (MANDATORY for new features)

<!-- Foundational domain types have no direct user-facing story: an operator never
     "uses" a RouterID type in isolation. The user-facing value is realized only
     once the wire codec (ospf-2) and runtime (ospf-4+) consume these types. The
     stories below record the chains these types ENABLE, and name the downstream
     spec where the user-facing test lives. Mirrors spec-isis-1-types.md. -->

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (enabling) Configures `router-id 10.0.0.1` and `area 0` | config string -> `ParseRouterID` / `ParseAreaID` -> typed values consumed by config resolve | this package: `TestRouterIDParseFormatRoundTrip`, `TestAreaIDIntegerAndDottedForms`; user-facing config story in `spec-ospf-4-component-config.md` |
| 2 | (enabling) A peer's LSA arrives on the wire | wire octets -> `LSAKeyFromHeader` / `LSSequenceNumber` / `LSAge` -> LSDB freshness compare; `FletcherChecksum` verifies the LSA | this package: `TestLSAKeyFromHeader`, `TestFletcherRFC905Vectors`; user-facing wire story in `spec-ospf-2-wire.md`; LSDB story in `spec-ospf-7-lsdb-flooding.md` |
| 3 | (enabling) Operator runs `show ip ospf database` | LSDB keyed by `LSAKey` -> `String()` renders type / link-state-id / advertising-router rows | this package: `TestLSAKeyFromHeader`, `TestRouterIDParseFormatRoundTrip`; user-facing CLI story in `spec-ospf-13-cli-diag-interop.md` |
| 4 | (enabling) A node receives an OSPF Hello | wire octets -> `InternetChecksum` validates the packet; `Options` E-bit gates adjacency | this package: `TestInternetChecksumRFC1071Vectors`, `TestOptionsBits`; user-facing story in `spec-ospf-5-interface-ism.md` |

<!-- No broken links: every chain above terminates in a downstream spec that owns the
     user-facing test. This spec's obligation is the typed-value + checksum correctness those chains rely on. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouterIDParseFormatRoundTrip` | `internal/component/ospf/types/routerid_test.go` | dotted-quad string -> `RouterID` -> string identity; bytes round-trip; length validation | |
| `TestAreaIDIntegerAndDottedForms` | `internal/component/ospf/types/areaid_test.go` | `0` == `0.0.0.0` == backbone; non-zero integer == dotted-quad equivalent; `IsBackbone()` | |
| `TestLinkStateIDRoundTrip` | `internal/component/ospf/types/linkstateid_test.go` | dotted-quad parse/format/bytes round-trip; length validation | |
| `TestLSAKeyFromHeader` | `internal/component/ospf/types/lsakey_test.go` | `(LSType, LinkStateID, AdvertisingRouter)` extracted from a 20-byte header; version fields excluded; map-key usable | |
| `TestLSAKeyEqualityExcludesVersion` | `internal/component/ospf/types/lsakey_test.go` | two headers differing only in sequence/age/checksum yield the SAME `LSAKey` (R-5) | |
| `TestLSAKeyOrder` | `internal/component/ospf/types/lsakey_test.go` | total order over type/link-state-id/advertising-router consistent with equality (LSDB listing) | |
| `TestLSTypeKnownValues` | `internal/component/ospf/types/lstype_test.go` | 1/2/3/4/5/7 known; 9/10/11 recognised-but-opaque; unknown types flagged | |
| `TestLSSequenceFreshness` | `internal/component/ospf/types/sequence_test.go` | §13.2 signed freshness: `0x7FFFFFFF` newer than `0x80000001`; equal not newer; `0x80000000` reserved | |
| `TestLSSequenceWraparound` | `internal/component/ospf/types/sequence_test.go` | `IsMax()` at `0x7FFFFFFF`; increment never yields `0x80000000`; re-origination boundary exposed | |
| `TestLSAgeBitsAndMaxAge` | `internal/component/ospf/types/lsage_test.go` | masked age ignores DoNotAge; `IsMaxAge()` at 3600; `DoNotAge()` at 0x8000; ageing helper saturates at `MaxAge` | |
| `TestMetricRangeAndCost` | `internal/component/ospf/types/metric_test.go` | 16-bit cost 1..65535; `ReferenceBandwidth`-based default-cost helper floors at 1; 16-bit serialize | |
| `TestOptionsBits` | `internal/component/ospf/types/options_test.go` | E/MC/N-P/L/DC/O/DN set/clear/test independently; `String()` lists set bits; 1-byte round-trip | |
| `TestFletcherRFC905Vectors` | `internal/component/ospf/types/checksum_test.go` | Fletcher-16 over the OSPF window (from Options, LS Age excluded) matches RFC 905 Annex B vectors; encode AND verify | |
| `TestFletcherIgnoresLSAge` | `internal/component/ospf/types/checksum_test.go` | flipping an LS-Age byte does NOT change the Fletcher result (covered-range boundary, R-1) | |
| `TestInternetChecksumRFC1071Vectors` | `internal/component/ospf/types/checksum_test.go` | RFC 1071 one's-complement over the packet window (Auth excluded, Checksum zeroed); re-sum yields 0xFFFF (R-2) | |
| `TestChecksumNoAlloc` | `internal/component/ospf/types/checksum_test.go` | both checksum functions allocate 0 and do not mutate the input window (`testing.AllocsPerRun`) | |
| `TestStringNoAlloc` | `internal/component/ospf/types/format_test.go` | dotted-quad `String()`/`AppendTo` for `RouterID`/`AreaID`/`LinkStateID` is zero-alloc | |
| `TestParseRejectsMalformed` | `internal/component/ospf/types/parse_test.go` | wrong octet count, out-of-range octets (> 255), bad separators, empty all error | |
| `TestWriteToRoundTrip` | `internal/component/ospf/types/serialize_test.go` | `WriteTo(buf, off)` for every type reproduces the `*FromBytes` octets and returns the correct count | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `RouterID` / `LinkStateID` length | 4 bytes | 4 | 3 | 5 |
| `AreaID` length (wire) | 4 bytes | 4 | 3 | 5 |
| dotted-quad octet value | 0..255 | 255 | N/A (unsigned) | 256 |
| `AreaID` integer form | 0..4294967295 | 4294967295 | N/A (unsigned) | N/A (full 32-bit) |
| `LSType` (in scope) | 1,2,3,4,5,7 | 7 | 0 (invalid) | 8..11 opaque/unknown (recognised, out of scope) |
| `LSSequenceNumber` | 0x80000001..0x7FFFFFFF (signed; 0x80000000 reserved, never used) | 0x7FFFFFFF (`MaxSequenceNumber`, newest) | 0x80000001 (`InitialSequenceNumber`, oldest) | wraps -> flush via MaxAge, re-originate at 0x80000001 (runtime, ospf-7) |
| `LSAge` (low 15 bits) | 0..3600 (`MaxAge`) | 3600 | N/A (unsigned) | 3601..32767 invalid age; 0x8000 is the DoNotAge bit, not an age |
| `Metric` (interface output cost) | 1..65535 | 65535 | 0 (cost floored to >= 1 by the default-cost helper) | N/A (16-bit) |
| `Options` | single byte, bits E/MC/N-P/L/DC/O/DN | 0xFF (all defined bits) | N/A | N/A (8-bit) |

### Functional Tests
<!-- This is a pure leaf type package with no runtime entry point. End-user-facing
     functional tests (.ci) belong to the consuming specs that produce observable
     CLI / wire behaviour. Listed here for traceability, owned downstream. N/A here. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (owned by ospf-4) | `test/ospf/` | operator-configured router-id / area parses and resolves | owned by `spec-ospf-4-component-config.md` |
| (owned by ospf-13) | `test/ospf/ospf-show.ci` | `show ip ospf database` renders `LSAKey` rows | owned by `spec-ospf-13-cli-diag-interop.md` |
| N/A for this leaf spec | n/a | Pure value types and checksum algorithms have no standalone .ci behaviour; functional coverage is owned downstream (ospf-2/4/13) | N/A |

### Interop Tests (MANDATORY for protocol features)
<!-- Pure domain types and checksum algorithms carry no wire behaviour on their own;
     interop is proven where the codec and runtime emit/consume real frames (ospf-13). -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (none for this spec) | n/a | n/a | N/A: type and checksum correctness is exercised by the ospf-2 round-trip/fuzz tests and by every FRR `ospfd` interop scenario (ospf-13) that exchanges these encoded values and validates the two checksums on the wire | N/A |

### Future (if deferring any tests)
- A dedicated 24-bit `SummaryMetric`/`ExternalMetric` leaf type: only if ospf-9/ospf-10 prove inline 24-bit decode is insufficient (assumption A-4). Requires explicit user approval to defer.

## Files to Modify
<!-- This spec is almost entirely new files. No existing Go file changes in phase 1. -->
- `internal/component/ospf/types/` - (new leaf package; see Files to Create). No existing file is modified: redistribution, component wiring, and the EXISTING `rib.admin-distance.ospf` reuse are owned by later specs (ospf-4, ospf-8, ospf-10). The OSPF Fletcher is implemented fresh here and does NOT modify `internal/component/isis/packet/checksum.go`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | Owned by `spec-ospf-4-component-config.md` (router-id/area-id leaves); this spec only supplies the types the validators will call |
| YANG validation constraints | No | The `router-id` and `area-id` validators in ospf-4 call back into these `Parse*` constructors |
| YANG custom validators | No | ospf-4 `ValidateFn`/`CompleteFn` reuse `ParseRouterID`/`ParseAreaID`; this spec exposes them error-returning and side-effect-free |
| CLI commands/flags | No | Owned by ospf-13 |
| CLI grammar (action before identifier) | No | Owned by ospf-13 |
| Editor autocomplete | No | Owned by ospf-4 (CompleteFn over these types) |
| Functional test for new RPC/API | No | Owned by ospf-4 / ospf-13 |
| Pipe completeness | No | Owned by ospf-13 |
| Env var registration | No | Not applicable (no env-only settings) |
| Doctor check for runtime dependencies | No | Not applicable (no sockets/paths/services in a pure type package; the `CAP_NET_RAW` doctor check is owned by ospf-3) |
| Prometheus counters/metrics | No | Not applicable (no observable runtime state; the OSPF metric series are owned by ospf-3..ospf-12 per the umbrella Metrics table) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No user-facing surface in phase 1; OSPF feature row added by ospf-13 in `docs/features.md` |
| 2 | Config syntax changed? | No | router-id/area-id config syntax documented by ospf-4 |
| 3 | CLI command added/changed? | No | Owned by ospf-13 |
| 4 | API/RPC added/changed? | No | None in phase 1 |
| 5 | Plugin added/changed? | No | OSPF is a component, registered by ospf-4 |
| 6 | Has a user guide page? | No | `docs/guide/ospf.md` owned by ospf-13 |
| 7 | Wire format changed? | No | `docs/architecture/wire/ospf.md` owned by ospf-2 (these types and the two checksums appear there as the encoded values) |
| 8 | Plugin SDK/protocol changed? | No | No |
| 9 | RFC behavior implemented? | Yes | RFC 2328 (§12 LSA structure, §13.1 freshness, §B constants), RFC 905 (Fletcher-16), RFC 1071 (Internet checksum) - note the type-level constraints (RouterID/AreaID 4-byte, LSAKey excludes version, LSSequenceNumber signed freshness, LSAge DoNotAge/MaxAge, two checksum covered ranges). Short summaries created via `/ze-rfc` at implementation time |
| 10 | Test infrastructure changed? | No | No new test infra; standard Go unit tests with RFC 905 / RFC 1071 vectors |
| 11 | Affects daemon comparison? | No | OSPF comparison row added by ospf-13 |
| 12 | Internal architecture changed? | No | New component introduced by ospf-4; the `types` subpackage is described in the umbrella package-layout |
| 13 | Route metadata keys added/changed? | No | No |
| 14 | Prometheus counters added/changed? | No | No |
| 15 | Registered plugin/event/command/capability changed? | No | No (no registration in a leaf type package) |
| 16 | Any changed source file referenced by existing doc source anchors? | No | Grep at completion (no existing files changed) |
| 17 | Existing docs show examples for this area? | No | Grep at completion (OSPF docs do not exist yet) |

## Files to Create
- `internal/component/ospf/types/routerid.go` - `RouterID` (4-byte value) parse/format/equal/serialize
- `internal/component/ospf/types/areaid.go` - `AreaID` (4-byte value), dotted-quad AND integer parse, `IsBackbone()`, serialize
- `internal/component/ospf/types/linkstateid.go` - `LinkStateID` (4-byte value) parse/format/serialize (DR interface address / network / Router ID per LSA type)
- `internal/component/ospf/types/lstype.go` - `LSType` (1 byte) known-value set (1/2/3/4/5/7; 9/10/11 opaque-but-out-of-scope)
- `internal/component/ospf/types/lsakey.go` - `LSAKey` `(LSType, LinkStateID, AdvertisingRouter)` triple: equality (excludes version), ordering, map-key safe, `LSAKeyFromHeader`
- `internal/component/ospf/types/sequence.go` - `LSSequenceNumber` (signed 32-bit), `InitialSequenceNumber`/`MaxSequenceNumber`/reserved-0x80000000, §13.2 `Newer`, `IsMax()`, increment helper
- `internal/component/ospf/types/lsage.go` - `LSAge` (16-bit), DoNotAge bit (0x8000), `MaxAge` (3600) marker, masked age, saturating ageing helper, `LSRefreshTime`/`MaxAgeDiff`/`CheckAge` constants
- `internal/component/ospf/types/metric.go` - `Metric` (16-bit interface output cost) with `ReferenceBandwidth`-based default-cost helper, 16-bit serialize
- `internal/component/ospf/types/options.go` - `Options` (1 byte) E/MC/N-P/L/DC/O/DN bit set/clear/test + `String()`
- `internal/component/ospf/types/checksum.go` - `FletcherChecksum(buf) uint16` (OSPF covered range, from Options, LS Age excluded) and `InternetChecksum(buf) uint16` (RFC 1071, Auth excluded, Checksum zeroed)
- `internal/component/ospf/types/format.go` - shared zero-alloc dotted-quad append/parse helpers and the package error set
- `internal/component/ospf/types/doc.go` - package doc stating the leaf-package constraint (no runtime / IS-IS / BGP-LS imports)
- `internal/component/ospf/types/routerid_test.go` - RouterID unit + boundary tests
- `internal/component/ospf/types/areaid_test.go` - AreaID integer/dotted-quad + backbone tests
- `internal/component/ospf/types/linkstateid_test.go` - LinkStateID round-trip tests
- `internal/component/ospf/types/lstype_test.go` - LSType known-value tests
- `internal/component/ospf/types/lsakey_test.go` - LSAKey equality (version-excluded) + order + map-key tests
- `internal/component/ospf/types/sequence_test.go` - LSSequenceNumber §13.2 freshness + wraparound tests
- `internal/component/ospf/types/lsage_test.go` - LSAge DoNotAge/MaxAge/ageing boundary tests
- `internal/component/ospf/types/metric_test.go` - Metric range + default-cost tests
- `internal/component/ospf/types/options_test.go` - Options bit-field tests
- `internal/component/ospf/types/checksum_test.go` - Fletcher RFC 905 + Internet RFC 1071 vector + no-alloc + covered-range tests
- `internal/component/ospf/types/format_test.go` - zero-alloc `String()` assertions
- `internal/component/ospf/types/parse_test.go` - malformed-input rejection tests
- `internal/component/ospf/types/serialize_test.go` - `WriteTo(buf, off)` round-trip tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `plan/spec-ospf-0-umbrella.md` + guide §3-§4 / §13 |
| 2. Audit | Files to Create, TDD Test Plan - confirm `internal/component/ospf/types/` does not yet exist |
| 3. Wiring phase | Wiring Test table - parse/key/checksum/format/serialize round-trip skeletons |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section - run `/ze-review`; fix every BLOCKER/ISSUE; re-run until only NOTEs remain |
| 6. Full verification | `make ze-lint-changed && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow (Deliverables, Security, re-verify, Executive Summary) |

### Implementation Phases

<!-- Phase 1 is ALWAYS wiring: create the package and a failing round-trip test. -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - create the package and failing round-trip tests
   - Tests: `TestRouterIDParseFormatRoundTrip`, `TestLSAKeyFromHeader`, `TestFletcherRFC905Vectors`, `TestInternetChecksumRFC1071Vectors` (failing against stubs)
   - Files: `doc.go`, `routerid.go`, `lsakey.go`, `checksum.go` with stub `Parse*`/`String`/`WriteTo`/`FletcherChecksum`/`InternetChecksum` signatures
   - Verify: package compiles, tests fail because stubs return zero values; leaf-import constraint holds (no runtime / IS-IS / BGP-LS imports)
2. **Phase: Fixed-width identifiers** - RouterID, AreaID, LinkStateID, LSType, LSAKey
   - Tests: `TestRouterID*`, `TestAreaIDIntegerAndDottedForms`, `TestLinkStateIDRoundTrip`, `TestLSTypeKnownValues`, `TestLSAKey*`
   - Files: `routerid.go`, `areaid.go`, `linkstateid.go`, `lstype.go`, `lsakey.go`, `format.go`
   - Verify: dotted-quad parse/format identity; AreaID integer/dotted equivalence + backbone; `LSAKey` equality excludes version fields, order total, map-key usable
3. **Phase: Sequence, age, metric, options** - the numeric and bit-field types
   - Tests: `TestLSSequenceFreshness`, `TestLSSequenceWraparound`, `TestLSAgeBitsAndMaxAge`, `TestMetricRangeAndCost`, `TestOptionsBits`
   - Files: `sequence.go`, `lsage.go`, `metric.go`, `options.go`
   - Verify: §13.2 signed freshness correct at the wraparound boundaries; DoNotAge/MaxAge distinct; Metric floored at 1; Options bits independent
4. **Phase: Checksums** - Fletcher-16 (LSA) and Internet checksum (packet)
   - Tests: `TestFletcherRFC905Vectors`, `TestFletcherIgnoresLSAge`, `TestInternetChecksumRFC1071Vectors`, `TestChecksumNoAlloc`
   - Files: `checksum.go`
   - Verify: both match their RFC vectors; covered ranges correct (Fletcher excludes LS Age; Internet excludes Auth, zeroes Checksum); zero-alloc; input window not mutated
5. **Phase: Robustness and allocation** - malformed-input rejection and zero-alloc formatting + serialize
   - Tests: `TestParseRejectsMalformed`, `TestStringNoAlloc`, `TestWriteToRoundTrip`
   - Files: `format.go`, `serialize_test.go`, `parse_test.go`
   - Verify: every malformed input errors; `String()` zero-alloc per `testing.AllocsPerRun`; every type `WriteTo` round-trips
6. **Full verification** - `make ze-lint-changed && make ze-unit-test`
7. **Complete spec** - fill audit tables, write learned summary to `plan/learned/NNN-ospf-1-types.md`; TWO commits (code+spec+learned, then `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1..AC-13) has a test with file:line |
| Feature completeness | All types from guide §4 implemented with parse/format/equal/order/serialize, plus BOTH checksum algorithms (Fletcher-16 LSA, Internet packet) with RFC 905 / RFC 1071 vectors |
| Correctness | `LSAKey` excludes version fields; `LSSequenceNumber` §13.2 signed freshness (0x7FFFFFFF newest, 0x80000001 oldest, 0x80000000 reserved); `LSAge` DoNotAge 0x8000 masked, `MaxAge` 3600 distinct; Fletcher covered range from Options (LS Age excluded); Internet checksum excludes Auth, zeroes Checksum |
| Naming | Exported `RouterID`/`AreaID`/`LinkStateID`/`LSType`/`LSAKey`/`LSSequenceNumber`/`LSAge`/`Metric`/`Options`; `FletcherChecksum`/`InternetChecksum`; constructors `Parse*` (string) and `*FromBytes` / `LSAKeyFromHeader` (wire) |
| Data flow | Leaf package: zero imports from OSPF runtime, IS-IS, or BGP-LS; serialize is buffer-first `WriteTo(buf, off) int`; checksum reads a caller window without mutation |
| Rule: buffer-first / no-sprintf-alloc | `String()` zero-alloc; serialize writes into caller buffer; checksum allocates nothing |
| Rule: go-standards | Fixed identifiers and `LSAKey` are comparable value types usable as map keys; no `net.IP`-slice comparison |
| Rule: no-layering | OSPF Fletcher is a fresh implementation; the IS-IS Fletcher is NOT imported or modified |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `types` package directory | `ls internal/component/ospf/types/` |
| Type + checksum files | `ls internal/component/ospf/types/{routerid,areaid,linkstateid,lstype,lsakey,sequence,lsage,metric,options,checksum}.go` |
| Test files for each type + checksum | `ls internal/component/ospf/types/*_test.go` |
| Leaf-import constraint holds | `go list -deps ./internal/component/ospf/types` shows only stdlib + Ze leaf helpers, no OSPF runtime, no IS-IS, no BGP-LS |
| RFC 905 / RFC 1071 vectors pass | `go test ./internal/component/ospf/types/ -run 'Checksum|Fletcher'` passes |
| All boundary rows tested | `go test ./internal/component/ospf/types/` passes including boundary cases |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every `*FromBytes` / `LSAKeyFromHeader` validates exact length before indexing; no slice out-of-range on attacker-controlled wire lengths |
| Input validation | Every `Parse*` rejects wrong octet count, octets > 255, and bad separators with an explicit error |
| Checksum windows | `FletcherChecksum` / `InternetChecksum` bound-check the caller window; no read past the slice; do NOT mutate the input (re-entrant, safe on shared buffers) |
| Resource exhaustion | Fixed-width types allocate nothing per-call; no unbounded allocation from a length field |
| Error leakage | Parse errors describe the shape problem without echoing unbounded attacker input |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read guide §3-§4 / §13 / the RFC summary |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Checksum vector fails | Re-check the covered range (Fletcher from Options excluding LS Age; Internet excluding Auth, Checksum zeroed) against RFC 905 / RFC 1071 |
| Boundary test fails | Re-check the range table against RFC 2328 §B constants |
| Audit finds missing type | Back to the relevant phase and implement |
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
<!-- LIVE -- write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| 4-byte array value types for `RouterID`/`AreaID`/`LinkStateID` | `net.IP` slices / pointers | Comparable with `==`, usable as map keys (LSDB index), no heap pointer churn, avoids the guide §4 `net.IP`-slice-identity comparison bug |
| `LSAKey` is exactly `(LSType, LinkStateID, AdvertisingRouter)`, excluding version fields | Include sequence/age/checksum in the key | Guide §3-§4: the LSDB keys on identity, not version; including version would treat every refresh as a new LSA and the LSDB would grow unbounded |
| `AreaID` parses BOTH `0` (integer) and `0.0.0.0` (dotted quad), normalising to one value | Dotted-quad only | OSPF area IDs are scalar 32-bit; operators write `area 0`; `0` and `0.0.0.0` must compare equal and both mean the backbone |
| OSPF Fletcher-16 is a FRESH implementation, not shared with IS-IS | Share `isis/packet/checksum.go` | Umbrella "Separate from IS-IS": the Fletcher covered range differs (OSPF from the Options field excluding LS Age; IS-IS from byte 12); a shared helper would leak LSA-vs-LSP header detail into both |
| `LSSequenceNumber` exposes §13.2 signed freshness + the reserved 0x80000000 distinctly | Treat the sequence as an ordinary unsigned int | RFC 2328 §13.1/§13.2: naive "higher is newer" loses updates at wraparound; 0x80000000 is reserved and never used; masking these causes origination/flap bugs |
| `LSAge` exposes DoNotAge (0x8000) and `MaxAge` (3600) as distinct predicates | Treat the 16 bits as a plain age | The high bit is the DoNotAge flag, not part of the age; `MaxAge` is the purge marker the runtime must retain; conflating them purges frozen LSAs |

## Known Limitations
- The 24-bit Summary/External LSA-body metric is decoded inline by ospf-2, not modelled as a dedicated leaf type here (wide cost is the 16-bit interface output cost; revisit only if ospf-9/ospf-10 demand a dedicated type, assumption A-4)
- No runtime behaviour lives here: monotonic-clock ageing, purge retention, sequence wraparound flush, and the full §13.1 freshness (LS Age / checksum tiebreak under runtime context) are owned by `spec-ospf-7-lsdb-flooding.md`. This package supplies the value types and the raw comparison/checksum primitives those runtime procedures call
- Opaque LSA types (9/10/11) are recognised by `LSType` as out-of-scope discriminators only; no opaque framework is implemented (deferred per the umbrella)

## RFC Documentation

Add `// RFC 2328 Section X.Y: "<quoted requirement>"`, `// RFC 905 Annex B ...`,
and `// RFC 1071 ...` above the enforcing code. MUST document: RouterID/AreaID
4-byte width and the `0.0.0.0` backbone; `LSAKey` identity excluding the
sequence/age/checksum version fields (RFC 2328 §12.1, §13.1); `LSSequenceNumber`
signed freshness with `InitialSequenceNumber` 0x80000001 / `MaxSequenceNumber`
0x7FFFFFFF / reserved 0x80000000 (RFC 2328 §12.1.6, §13.1); `LSAge` DoNotAge bit
0x8000 and `MaxAge` 3600 (RFC 2328 §12.1.1, §B); the Fletcher-16 covered range
(from the Options field, excluding LS Age, RFC 905 Annex B); and the Internet
checksum covered range (excluding the Authentication field, Checksum zeroed,
RFC 1071).

## Implementation Summary

### What Was Implemented
- [Filled at implementation time]

### Bugs Found/Fixed
- [Filled at implementation time]

### Documentation Updates
- [Filled at implementation time]

### Deviations from Plan
- [Filled at implementation time]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

<!-- MANDATORY: Maps each stated goal to concrete proof it was achieved. -->
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Nine OSPF domain types with parse/format/compare/serialize | unit test | [Filled at implementation: `go test ./internal/component/ospf/types/` PASS] |
| Two checksum algorithms (Fletcher-16 LSA, Internet packet) vector-correct | unit test | [Filled at implementation: `TestFletcherRFC905Vectors`, `TestInternetChecksumRFC1071Vectors` PASS] |
| Leaf package, no runtime / IS-IS / BGP-LS imports | dependency check | [Filled at implementation: `go list -deps` closure is stdlib + leaf helpers only] |
| Consumed by the wire codec (downstream) | downstream build | [Filled at implementation: `grep -rln ospf/types internal/component/ospf/` shows importers incl. `packet/`] |
| Round-trip fidelity (parse/format/bytes/serialize) | unit test | [Filled at implementation: per-type round-trip tests PASS] |

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist): run /ze-review BEFORE the final verify. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: re-verify everything independently; paste command evidence. -->

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
- [ ] AC-1..AC-13 all demonstrated
- [ ] End-to-End User Stories: each enabling chain terminates in a downstream spec with a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + unit tests for this package)
- [ ] Feature code integrated (`internal/component/ospf/types/`)
- [ ] Leaf-import constraint proven (`go list -deps`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented (none in phase 1; owned downstream)
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (RFC 2328 / RFC 905 / RFC 1071)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (no 24-bit metric type unless ospf-9/10 demand it)
- [ ] No speculative features (no opaque framework; LSType recognises 9/10/11 only as out-of-scope)
- [ ] Single responsibility per type file
- [ ] Explicit > implicit behavior (LSAKey excludes version; sequence freshness explicit; checksum covered ranges documented)
- [ ] Minimal coupling (leaf package; OSPF Fletcher independent of IS-IS)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests owned downstream (ospf-2/4/13) referenced; N/A here with justification
- [ ] Interop N/A with justification (types and checksum algorithms carry no wire behaviour alone)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospf-1-types.md`
- [ ] **Commit A:** code + tests + spec (with all edits) + learned summary
- [ ] **Commit B:** `git rm plan/spec-ospf-1-types.md` only (preserves edited spec in git history from commit A)
