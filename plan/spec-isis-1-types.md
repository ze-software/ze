# Spec: isis-1-types

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` - umbrella scope, package layout, dependency graph (this is row isis-1)
4. `docs/research/isis-implementation-guide.md` section 3 "Domain Types and Constraints" (lines 200-225) - authoritative type list
5. `internal/component/isis/types/` - the package this spec creates (does not exist yet)

## Task

Create the IS-IS domain type leaf package `internal/component/isis/types/`. This
is phase 1 of the IS-IS umbrella (`plan/spec-isis-0-umbrella.md`) and the bottom
layer of the layered, leaf-first package design declared there: `types` (leaf)
<- `packet` codec <- `server` runtime. This package implements the pure value
types that every higher IS-IS layer keys on, with no network I/O, no timers, no
goroutines, and no imports from the IS-IS runtime. It is the IS-IS equivalent of
a self-contained address/identifier library.

The types and their meanings come from ISO/IEC 10589 (ISO/IEC 10589 as the
consolidated normative reference) section 1.4 and section 6.2 (addressing model),
and from RFC 5305 (wide metric). The set is:

| Type | Width / shape | Meaning |
|------|---------------|---------|
| `SystemID` | 6-byte fixed array | Uniquely identifies a router |
| `SourceID` | 7 bytes = `SystemID` (6) + pseudonode ID (1) | Identifies a node: router (pseudonode 0) or LAN pseudonode (non-zero) |
| `LSPID` | 8 bytes = `SourceID` (7) + LSP number (1) | Uniquely identifies one LSP fragment |
| `NET` (Network Entity Title) | variable 8..20 bytes = `AreaID` (1..13) + `SystemID` (6) + SEL (1) | The IS-IS address configured on a node |
| `AreaID` | variable 1..13 bytes | Identifies a level-1 area |
| `Metric` (IS reachability, TLV 22) | 24-bit, 0..16777215 | IS-reachability link cost (RFC 5305 wide metric) |
| `PrefixMetric` (IP/IPv6 prefix, TLV 135 / TLV 236) | 32-bit, 0..4294967295 | IPv4 / IPv6 prefix cost (RFC 5305 TLV 135, RFC 5308 TLV 236) |
| `SequenceNumber` | 32-bit, 1..0xFFFFFFFF (0 is reserved and never a valid version) | LSP version, monotonically increasing |
| `HoldingTime` / `RemainingLifetime` | 16-bit seconds, 0..65535 | Hello hold time; LSP remaining lifetime |

For each type the package provides: parse from a printable string, parse from
wire bytes, format for display, equality, ordering where it is semantically
meaningful (notably `LSPID` and `AreaID`, used to bound CSNP ranges), and
buffer-first byte serialization consistent with `ai/rules/buffer-first.md`.

Ze has no IS-IS types today. The closest artefacts (BGP-LS in
`internal/component/bgp/plugins/nlri/ls/`, MRT link-state decoders) carry
link-state topology inside BGP NLRI and do not model the IS-IS addressing types.
This package is brand new and stands alone; it is consumed first by the wire
codec spec `spec-isis-2-wire.md`, and transitively by every later IS-IS child.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/research/isis-implementation-guide.md` section 3 (lines 200-225) - the authoritative type list and constraints
  -> Decision: implement the types listed there with the printable formats given (`SystemID` `0001.0002.0003`, `LSPID` `0001.0002.0003.00-01`, `NET` `49.0001.1234.5678.9abc.0000.00`), splitting the metric into two distinct types: `Metric` (24-bit IS reachability, TLV 22) and `PrefixMetric` (32-bit IP/IPv6 prefix, TLV 135 / TLV 236, per RFC 5305 / RFC 5308)
  -> Constraint: `NET` total length is 8..20 bytes; parse by taking the System ID from the last 7 bytes before the final SEL byte, the SEL from the final byte, and the Area ID from everything before that
  -> Constraint: `SequenceNumber` 0 is reserved and never a valid originated LSP version (origination starts at 1); a purge is signalled by Remaining Lifetime 0 at runtime (isis-6), NOT by sequence 0; wraparound at 0xFFFFFFFF is a runtime concern (isis-6), but the type MUST represent 0 distinctly (as reserved) and not silently coerce it
- [ ] `plan/spec-isis-0-umbrella.md` - umbrella scope, layered package layout, design principles
  -> Constraint: `types` is a leaf package and MUST NOT import anything from the IS-IS runtime (`packet`, `transport`, `circuit`, `adjacency`, `lsdb`, `spf`, the component root); it may import only Go stdlib
  -> Constraint: wide metrics only are originated; the narrow 6-bit metric is out of scope for this type package. Two distinct wide metric types are modelled: `Metric` is the 24-bit IS-reachability metric (TLV 22, range 0..16777215) and `PrefixMetric` is the 32-bit IP/IPv6 prefix metric (TLV 135 / TLV 236, range 0..4294967295). Capping a prefix metric at 24-bit would reject or mangle valid peer routes, so the two widths must NOT be conflated
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, no-alloc encode
  -> Constraint: byte serialization is buffer-first: write into a caller-supplied buffer at an offset and return the number of bytes written; do not allocate a fresh slice per call
- [ ] `ai/rules/no-sprintf-alloc.md` - string building without per-call allocation on hot paths
  -> Constraint: display formatting (`String()`) must avoid `fmt.Sprintf` allocation in any path the codec or CLI calls repeatedly; use a `textbuf`-style append or hex helper
- [ ] `ai/rules/go-standards.md` - value-typed, no cross-boundary pointers
  -> Constraint: fixed-width identifiers (`SystemID`, `SourceID`, `LSPID`) are value-typed fixed arrays so they are comparable with `==` and usable directly as map keys

### RFC Summaries (MUST for protocol work)
- [ ] `iso/short/iso10589.md` - IS-IS base, section 1.4 (definitions) and section 6.2 (addressing) (CREATED per umbrella)
  -> Constraint: System ID is 6 octets; NET = Area Address + System ID + NSEL; NSEL (SEL) is 0x00 for an IS (router)
  -> Constraint: an LSP with Remaining Lifetime 0 is a purge (a RemainingLifetime property, isis-6); Sequence Number 0 is reserved and never a valid originated LSP version (origination starts at 1), and is NOT itself a purge signal
- [ ] `rfc/short/rfc5305.md` - wide metrics, Extended IS/IP Reachability (CREATED per umbrella)
  -> Constraint: the IS-reachability metric (TLV 22) is a 24-bit unsigned value, range 0..16777215; default link metric is 10
  -> Constraint: the IP prefix metric (TLV 135) is a 32-bit unsigned value, range 0..4294967295; IPv6 prefix metric (TLV 236, RFC 5308) is likewise 32-bit. `Metric` (24-bit) and `PrefixMetric` (32-bit) are therefore separate types with separate serialization widths (3 vs 4 octets)

**Key insights:** (minimal context to resume after compaction)
- The domain types are all immutable keys or values; equality and ordering must handle fixed arrays (`SystemID`/`SourceID`/`LSPID`) and variable-length slices (`AreaID`/`NET`) correctly; two metric widths exist (`Metric` 24-bit IS reachability, `PrefixMetric` 32-bit prefix)
- `LSPID` ordering and `AreaID` ordering are load-bearing: CSNP advertises a start/end LSPID range (isis-7) and area comparison gates L1 adjacency (isis-5)
- This is a leaf package; the only correctness risk is parse/format/compare/serialize round-trip fidelity, which is fully unit-testable without any network

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey)
- [ ] Ze has no IS-IS implementation and no IS-IS types; `internal/component/isis/` does not exist yet
  -> Constraint: this package is created from scratch under `internal/component/isis/types/`
- [ ] BGP-LS (`internal/component/bgp/plugins/nlri/ls/`) carries link-state topology inside BGP NLRI; it does not model NET / SystemID / LSPID as reusable IS-IS types
  -> Constraint: do not import or couple to BGP-LS; these are independent type domains
- [ ] `internal/core/textbuf` provides allocation-light string building used elsewhere in Ze
  -> Constraint: reuse the established hex/append pattern for `String()` rather than inventing a new formatter

**Behavior to preserve:** (nothing exists to preserve in IS-IS)
- No existing IS-IS callers; this package introduces the API, it does not change one
- BGP, BGP-LS, MRT decoders remain untouched and independent

**Behavior to change:**
- New package `internal/component/isis/types/` with the eight value types and their methods
- No change to any other package in this spec (later specs add redistribution, component wiring, etc.). IS-IS reuses the EXISTING `rib.admin-distance.isis` leaf -- NO new admin-distance leaf is added anywhere (see `spec-isis-9-spf-rib.md`, which is the authoritative admin-distance model)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Strings from config and CLI: an operator-configured `net 49.0001.0000.0000.0001.00`, a `show isis database` filter naming an `LSPID`
- Bytes from the wire: the IS-IS PDU header and TLVs handing raw octet slices to constructors (consumed by `spec-isis-2-wire.md`)
- Format at entry: printable dotted-hex strings, or big-endian octet slices of the exact fixed or bounded length

### Transformation Path
1. **Parse:** a printable string (`ParseSystemID`, `ParseNET`, `ParseLSPID`, ...) or a byte slice (`SystemIDFromBytes`, `NETFromBytes`, ...) produces a typed value, validating length and shape
2. **Compare / key:** typed values are compared for equality and (where meaningful) ordered; fixed-array types are used directly as map keys (adjacency table, LSDB index in later specs)
3. **Format:** `String()` renders the canonical printable form for CLI / web / logs
4. **Serialize:** `WriteTo(buf, off) int` (or equivalent) writes the big-endian octets into a caller buffer for the wire codec, returning the byte count

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config / CLI string <-> type | `Parse*` constructors and `String()` formatters | [ ] |
| Wire bytes <-> type | `*FromBytes` constructors and `WriteTo(buf, off)` serializers | [ ] |
| Type <-> map-key / set membership | value-typed fixed arrays comparable with `==` | [ ] |

### Integration Points
- Consumed by `spec-isis-2-wire.md` (PDU header SystemID/LSPID, LSP entry LSPID+sequence+lifetime, TLV area addresses)
- Consumed by `spec-isis-5-adjacency.md` (neighbour SystemID, L1 area match on AreaID)
- Consumed by `spec-isis-6-lsdb.md` and `spec-isis-7-flooding.md` (LSPID-keyed LSDB, CSNP start/end LSPID range ordering)
- Consumed by `spec-isis-8-dis-broadcast.md` (SourceID pseudonode ID non-zero for pseudonode LSPs)
- No upstream dependency: this package imports only the Go standard library (and Ze leaf helpers such as `textbuf`)

### Architectural Verification
- [ ] No bypassed layers (types are pure; higher layers call constructors, never reach past them)
- [ ] No unintended coupling (leaf package: zero imports from IS-IS runtime or BGP-LS)
- [ ] No duplicated functionality (single canonical SystemID/NET/LSPID; later specs reuse, do not re-declare)
- [ ] Zero-copy preserved where applicable (serialize is buffer-first; parse may copy fixed arrays since they are small value types)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The eight types in research guide section 3 are the complete set needed by isis-2 wire | `docs/research/isis-implementation-guide.md` lines 200-225 | isis-2 discovers a missing type and must add it here | isis-2 wire codec compiles against this package without adding new identifier types | unvalidated |
| A-2 | Fixed-array value types (`SystemID`/`SourceID`/`LSPID`) are acceptable as Go map keys for the adjacency table and LSDB | `ai/rules/go-standards.md` value-typed identifiers | LSDB keying needs a different representation | isis-6 LSDB uses `LSPID` directly as a map key and compiles | unvalidated |
| A-3 | `NET` parsing by "last 7 bytes are SystemID+SEL, the rest is AreaID" is unambiguous for all valid 8..20 byte inputs | research guide line 210 | a real NET form breaks the heuristic | round-trip tests over the full 1..13 byte AreaID range | unvalidated |
| A-4 | Two wide metric types are needed: `Metric` (24-bit IS reachability, TLV 22) and `PrefixMetric` (32-bit IP/IPv6 prefix, TLV 135/236); the narrow 6-bit metric stays out | umbrella "Metric width? Wide metrics only" decision + RFC 5305 TLV 135 / RFC 5308 TLV 236 32-bit prefix metric | isis-2 needs a narrow-metric type for interop decode, or the two widths can be unified | isis-2 encodes/decodes TLV 22 (24-bit) and TLV 135/236 (32-bit) against these two types; narrow metric decoded inline without a dedicated type, or escalates | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Variable-length comparison (`AreaID`, `NET`) implemented with wrong ordering (length-first vs lexicographic) breaks CSNP range bounds | CSNP sync stalls or floods extra LSPs in isis-7 | Define ordering explicitly and document it; boundary tests on AreaID lengths 1 and 13 and on equal-prefix-different-length pairs |
| R-2 | `SequenceNumber` 0 silently treated as a valid version, masking the reserved-zero rule | isis-6 origination tests fail or loop | Represent 0 distinctly; expose an `IsReserved()`/zero check; never auto-wrap inside the type. (Purge is a RemainingLifetime 0 concern, not a SequenceNumber concern) |
| R-3 | `String()` allocates per call on a hot path (CLI list, log) | benchmark allocation in isis-13 | buffer-first append formatting per `ai/rules/no-sprintf-alloc.md`, with a bench guarding zero-alloc |
| R-4 | Parsing accepts malformed dotted-hex (wrong group count, odd nibbles) and produces a wrong value instead of an error | fuzz / boundary test surfaces silent corruption | strict length and separator validation in every `Parse*`; explicit error returns |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- This is a leaf type package: its "entry point" is its public API, exercised by tests
     in this package and, at the next layer, by the wire codec (spec-isis-2-wire.md).
     The wiring chain proven here is parse -> compare -> format -> serialize round-trip,
     plus the consumption proof that the wire codec links against these types. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| operator string `49.0001.0000.0000.0001.00` | -> | `ParseNET` builds a `NET`; `String()` returns the same canonical text | `TestNETParseFormatRoundTrip` |
| wire octets for an LSP header | -> | `LSPIDFromBytes` builds an `LSPID`; `WriteTo` reproduces the octets | `TestLSPIDBytesRoundTrip` |
| two area addresses on an L1 link | -> | `AreaID.Equal` / ordering decides L1 area match | `TestAreaIDEqualAndOrder` |
| isis-2 wire codec references these types | -> | `internal/component/isis/packet` imports `types` and builds PDUs | `spec-isis-2-wire.md` build + its round-trip tests (downstream wiring proof) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ParseSystemID("0001.0002.0003")` | Returns a 6-byte `SystemID`; its `String()` is `0001.0002.0003` (round-trip) |
| AC-2 | `SystemIDFromBytes` with a slice of length != 6 | Returns an error; no partial value leaks |
| AC-3 | `ParseLSPID("0001.0002.0003.00-01")` | Returns an `LSPID` with SystemID `0001.0002.0003`, pseudonode 0, LSP number 1; `String()` round-trips |
| AC-4 | `ParseNET("49.0001.0000.0000.0001.00")` | Returns a `NET` with AreaID `49.0001`, SystemID `0000.0000.0001`, SEL 0x00; `String()` round-trips |
| AC-5 | `NETFromBytes` over the full AreaID range 1..13 bytes (total 8..20) | Parses correctly; `AreaID()` and `SystemID()` accessors return the right slices |
| AC-6 | `NETFromBytes` with total length 7 or 21 | Returns an error (below / above the 8..20 byte bound) |
| AC-7 | `Metric` with 16777215 then 16777216; `PrefixMetric` with 4294967295 (then the 32-bit max) | `Metric`: 16777215 valid, 16777216 rejected (above 24-bit range); `PrefixMetric`: 4294967295 valid (full 32-bit range); each serializes to its own width (3 vs 4 octets) |
| AC-8 | `SequenceNumber(0)` | Reported as the reserved value (e.g. `IsReserved()` true); 0 is never a valid originated version and is never produced by a normal increment helper (purge is signalled by `RemainingLifetime` 0, not by sequence 0) |
| AC-9 | `RemainingLifetime`/`HoldingTime` of 0 and 65535 | Both representable; 65535 is the maximum 16-bit value |
| AC-10 | `LSPID` and `AreaID` ordering | Total order consistent with equality; usable to bound a CSNP start/end LSPID range and to compare area addresses |
| AC-11 | `WriteTo(buf, off)` for every type | Writes the exact big-endian octets and returns the correct count; bytes match the `*FromBytes` input (serialize/parse round-trip) |
| AC-12 | `SystemID` / `SourceID` / `LSPID` used as Go map keys | Compile and behave as comparable value types (no pointer identity surprises) |

## End-to-End User Stories (MANDATORY for new features)

<!-- Foundational domain types have no direct user-facing story: an operator never
     "uses" a SystemID type in isolation. The user-facing value is realized only
     once the wire codec (isis-2) and runtime (isis-4+) consume these types. The
     stories below therefore record the chains these types ENABLE, and name the
     downstream spec where the user-facing test lives. -->

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (enabling) Configures `net 49.0001...` | config string -> `ParseNET` -> typed `NET` consumed by config resolve | this package: `TestNETParseFormatRoundTrip`; user-facing config story in `spec-isis-4-component-config.md` |
| 2 | (enabling) A peer's LSP arrives on the wire | wire octets -> `LSPIDFromBytes` / `SequenceNumber` / `RemainingLifetime` -> PDU struct | this package: `TestLSPIDBytesRoundTrip`; user-facing wire story in `spec-isis-2-wire.md` |
| 3 | (enabling) Operator runs `show isis database` | LSDB keyed by `LSPID` -> `String()` renders `0001.0002.0003.00-01` rows | this package: `TestLSPIDParseFormatRoundTrip`; user-facing CLI story in `spec-isis-13-cli-diag-interop.md` |

<!-- No broken links: every chain above terminates in a downstream spec that owns the
     user-facing test. This spec's obligation is the typed-value correctness those chains rely on. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSystemIDParseFormatRoundTrip` | `internal/component/isis/types/systemid_test.go` | string -> `SystemID` -> string identity; canonical lowercase dotted-hex | |
| `TestSystemIDBytesRoundTrip` | `internal/component/isis/types/systemid_test.go` | bytes -> `SystemID` -> `WriteTo` identity; length validation | |
| `TestSourceIDParseFormatRoundTrip` | `internal/component/isis/types/sourceid_test.go` | SystemID + pseudonode round-trip; pseudonode 0 vs non-zero | |
| `TestLSPIDParseFormatRoundTrip` | `internal/component/isis/types/lspid_test.go` | `0001.0002.0003.00-01` parse/format identity | |
| `TestLSPIDBytesRoundTrip` | `internal/component/isis/types/lspid_test.go` | 8-byte serialize/parse identity | |
| `TestLSPIDOrder` | `internal/component/isis/types/lspid_test.go` | total order consistent with equality; CSNP range bounding | |
| `TestNETParseFormatRoundTrip` | `internal/component/isis/types/net_test.go` | `49.0001.0000.0000.0001.00` parse/format identity | |
| `TestNETAccessors` | `internal/component/isis/types/net_test.go` | `AreaID()`, `SystemID()`, `SEL()` extraction across AreaID lengths | |
| `TestAreaIDEqualAndOrder` | `internal/component/isis/types/areaid_test.go` | equality and ordering for variable-length area addresses | |
| `TestMetricRange` | `internal/component/isis/types/metric_test.go` | `Metric` 24-bit construction, rejection above 16777215, 3-octet big-endian serialize | |
| `TestPrefixMetricRange` | `internal/component/isis/types/metric_test.go` | `PrefixMetric` 32-bit construction, full 0..4294967295 range accepted, 4-octet big-endian serialize | |
| `TestSequenceNumberReserved` | `internal/component/isis/types/sequence_test.go` | 0 is the reserved value (never a valid version); increment helper behaviour up to the maximum | |
| `TestLifetimeAndHoldingTime` | `internal/component/isis/types/lifetime_test.go` | 16-bit range, 0 and 65535 boundaries, serialize | |
| `TestStringNoAlloc` | `internal/component/isis/types/format_test.go` | `String()` for each type is zero-alloc (benchmark / `testing.AllocsPerRun`) | |
| `TestParseRejectsMalformed` | `internal/component/isis/types/parse_test.go` | wrong group count, odd nibbles, bad separators all error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `SystemID` length | 6 bytes | 6 | 5 | 7 |
| `SourceID` length | 7 bytes | 7 | 6 | 8 |
| `LSPID` length | 8 bytes | 8 | 7 | 9 |
| `AreaID` length | 1..13 bytes | 13 | 0 | 14 |
| `NET` total length | 8..20 bytes | 20 | 7 | 21 |
| `Metric` (IS reachability, TLV 22, 24-bit) | 0..16777215 | 16777215 | N/A (unsigned) | 16777216 |
| `PrefixMetric` (IP/IPv6 prefix, TLV 135/236, 32-bit) | 0..4294967295 | 4294967295 | N/A (unsigned) | N/A (full 32-bit range) |
| `SequenceNumber` | 1..0xFFFFFFFF (0 reserved, never a valid version) | 0xFFFFFFFF | 0 (reserved; purge is RemainingLifetime 0, not sequence 0) | wraps -> re-originate from 1 (runtime, isis-6) |
| `RemainingLifetime` / `HoldingTime` | 0..65535 s | 65535 | N/A (unsigned) | 65536 |

### Functional Tests
<!-- This is a pure leaf type package with no runtime entry point. End-user-facing
     functional tests (.ci) belong to the consuming specs that produce observable
     CLI / wire behaviour. Listed here for traceability, owned downstream. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (owned by isis-4) | `test/isis/` | operator-configured NET parses and resolves | owned by `spec-isis-4-component-config.md` |
| (owned by isis-13) | `test/isis/isis-show.ci` | `show isis database` renders LSPIDs | owned by `spec-isis-13-cli-diag-interop.md` |

### Interop Tests (MANDATORY for protocol features)
<!-- Pure domain types carry no wire behaviour on their own; interop is proven where
     the wire codec and runtime emit/consume real frames. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (none for this spec) | n/a | n/a | Type correctness is exercised by the isis-2 round-trip/fuzz tests and by all FRR interop scenarios that exchange these encoded values | n/a |

### Future (if deferring any tests)
- Narrow (6-bit) metric type: only if isis-2 interop decode proves a dedicated type is needed rather than inline decode (assumption A-4)

## Files to Modify
<!-- This spec is almost entirely new files. The only repository touch beyond
     internal/component/isis/types/ is creating the directory itself, which the
     umbrella already anticipates. No existing Go file changes in phase 1. -->
- (none) - phase 1 adds a new leaf package and does not modify existing files; redistribution and component wiring are owned by later specs (isis-4, isis-9, isis-11). IS-IS reuses the EXISTING `rib.admin-distance.isis` leaf, so NO new admin-distance leaf is introduced in any spec (authoritative model: `spec-isis-9-spf-rib.md`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | Owned by `spec-isis-4-component-config.md` (NET/system-id leaves); this spec only supplies the types the validators will use |
| YANG validation constraints | No | The `system-id` pattern and `net` validator in isis-4 call back into these `Parse*` constructors |
| YANG custom validators | No | isis-4 `ValidateFn`/`CompleteFn` reuse `ParseNET`/`ParseSystemID`; this spec exposes them error-returning and side-effect-free |
| CLI commands/flags | No | Owned by isis-13 |
| CLI grammar (action before identifier) | No | Owned by isis-13 |
| Editor autocomplete | No | Owned by isis-4 (CompleteFn over these types) |
| Functional test for new RPC/API | No | Owned by isis-4 / isis-13 |
| Pipe completeness | No | Owned by isis-13 |
| Env var registration | No | Not applicable (no env-only settings) |
| Doctor check for runtime dependencies | No | Not applicable (no sockets/paths/services in a pure type package) |
| Prometheus counters/metrics | No | Not applicable (no observable runtime state) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No user-facing surface in phase 1; IS-IS feature row added by isis-13 in `docs/features.md` |
| 2 | Config syntax changed? | No | NET/system-id config syntax documented by isis-4 |
| 3 | CLI command added/changed? | No | Owned by isis-13 |
| 4 | API/RPC added/changed? | No | None in phase 1 |
| 5 | Plugin added/changed? | No | IS-IS is a component, registered by isis-4 |
| 6 | Has a user guide page? | No | `docs/guide/isis.md` owned by isis-13 |
| 7 | Wire format changed? | No | `docs/architecture/wire/isis.md` owned by isis-2 (these types appear there as the encoded values) |
| 8 | Plugin SDK/protocol changed? | No | No |
| 9 | RFC behavior implemented? | Yes | `iso/short/iso10589.md`, `rfc/short/rfc5305.md` (and `rfc/short/rfc5308.md` for TLV 236) - note the type-level constraints (SystemID width, NET shape, SEL=0, `Metric` 24-bit range, `PrefixMetric` 32-bit range, sequence 0 reserved -- purge is RemainingLifetime 0) |
| 10 | Test infrastructure changed? | No | No new test infra; standard Go unit tests |
| 11 | Affects daemon comparison? | No | IS-IS comparison row added by isis-13 |
| 12 | Internal architecture changed? | No | New component introduced by isis-4; the `types` subpackage is described in the umbrella package-layout |
| 13 | Route metadata keys added/changed? | No | No |
| 14 | Prometheus counters added/changed? | No | No |
| 15 | Registered plugin/event/command/capability changed? | No | No (no registration in a leaf type package) |
| 16 | Any changed source file referenced by existing doc source anchors? | No | Grep at completion (no existing files changed) |
| 17 | Existing docs show examples for this area? | No | Grep at completion (IS-IS docs do not exist yet) |

## Files to Create
- `internal/component/isis/types/systemid.go` - `SystemID` (6-byte array) parse/format/equal/serialize
- `internal/component/isis/types/sourceid.go` - `SourceID` (SystemID + pseudonode ID)
- `internal/component/isis/types/lspid.go` - `LSPID` (SourceID + LSP number), ordering for CSNP ranges
- `internal/component/isis/types/net.go` - `NET` and `AreaID` (variable-length), parse/format/accessors/ordering
- `internal/component/isis/types/metric.go` - 24-bit IS-reachability `Metric` (TLV 22) and 32-bit `PrefixMetric` (TLV 135 / TLV 236), each range-checked with its own 3-octet / 4-octet serialization
- `internal/component/isis/types/sequence.go` - `SequenceNumber` (32-bit), reserved-zero semantics (0 never a valid version; purge is a RemainingLifetime concern)
- `internal/component/isis/types/lifetime.go` - `RemainingLifetime` and `HoldingTime` (16-bit seconds)
- `internal/component/isis/types/format.go` - shared zero-alloc dotted-hex append helpers (if not folded into each file)
- `internal/component/isis/types/doc.go` - package doc stating the leaf-package constraint (no runtime imports)
- `internal/component/isis/types/systemid_test.go` - SystemID unit + boundary tests
- `internal/component/isis/types/sourceid_test.go` - SourceID unit tests
- `internal/component/isis/types/lspid_test.go` - LSPID unit + ordering tests
- `internal/component/isis/types/net_test.go` - NET/AreaID unit + boundary tests
- `internal/component/isis/types/metric_test.go` - `Metric` (24-bit) and `PrefixMetric` (32-bit) boundary tests
- `internal/component/isis/types/sequence_test.go` - SequenceNumber reserved-zero/boundary tests
- `internal/component/isis/types/lifetime_test.go` - lifetime/holding-time boundary tests
- `internal/component/isis/types/format_test.go` - zero-alloc `String()` assertions
- `internal/component/isis/types/parse_test.go` - malformed-input rejection tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `plan/spec-isis-0-umbrella.md` |
| 2. Audit | Files to Create, TDD Test Plan - confirm `internal/component/isis/types/` does not yet exist |
| 3. Wiring phase | Wiring Test table - parse/compare/format/serialize round-trip skeletons |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint-changed && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - create the package and failing round-trip tests
   - Tests: `TestSystemIDParseFormatRoundTrip`, `TestLSPIDBytesRoundTrip`, `TestNETParseFormatRoundTrip` (failing against stubs)
   - Files: `doc.go`, `systemid.go`, `lspid.go`, `net.go` with stub `Parse*`/`String`/`WriteTo` signatures
   - Verify: package compiles, tests fail because stubs return zero values; leaf-import constraint holds (no runtime imports)
2. **Phase: Fixed-width identifiers** - SystemID, SourceID, LSPID
   - Tests: `TestSystemID*`, `TestSourceID*`, `TestLSPID*` including `TestLSPIDOrder`
   - Files: `systemid.go`, `sourceid.go`, `lspid.go`, `format.go`
   - Verify: dotted-hex parse/format identity; bytes round-trip; ordering total and equality-consistent; usable as map keys
3. **Phase: Variable-length addressing** - AreaID and NET
   - Tests: `TestNETParseFormatRoundTrip`, `TestNETAccessors`, `TestAreaIDEqualAndOrder`
   - Files: `net.go`
   - Verify: NET parsing splits AreaID / SystemID / SEL correctly across the full 1..13 AreaID range; ordering documented and tested
4. **Phase: Numeric values** - Metric, PrefixMetric, SequenceNumber, RemainingLifetime, HoldingTime
   - Tests: `TestMetricRange`, `TestPrefixMetricRange`, `TestSequenceNumberReserved`, `TestLifetimeAndHoldingTime`
   - Files: `metric.go`, `sequence.go`, `lifetime.go`
   - Verify: all boundary rows pass; `Metric` capped at 24-bit (3-octet serialize), `PrefixMetric` full 32-bit (4-octet serialize); sequence 0 distinctly reported as reserved; big-endian serialize matches `*FromBytes`
5. **Phase: Robustness and allocation** - malformed-input rejection and zero-alloc formatting
   - Tests: `TestParseRejectsMalformed`, `TestStringNoAlloc`
   - Files: `format.go`, `parse_test.go`
   - Verify: every malformed input errors; `String()` is zero-alloc per `testing.AllocsPerRun`
6. **Full verification** - `make ze-lint-changed && make ze-unit-test`
7. **Complete spec** - fill audit tables, write learned summary to `plan/learned/NNN-isis-1-types.md`; two commits (code+spec+learned, then `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1..AC-12) has a test with file:line |
| Feature completeness | All types from research guide section 3 implemented with parse/format/equal/order/serialize, including BOTH metric widths (`Metric` 24-bit, `PrefixMetric` 32-bit) |
| Correctness | NET split (last 7 bytes SystemID+SEL), SEL=0x00 default, `Metric` 24-bit cap (TLV 22), `PrefixMetric` 32-bit (TLV 135/236, no 24-bit cap), sequence 0 reserved (never a valid version; purge is RemainingLifetime 0), lengths exactly per ISO/IEC 10589 |
| Naming | Exported types `SystemID`/`SourceID`/`LSPID`/`NET`/`AreaID`/`Metric`/`PrefixMetric`/`SequenceNumber`/`RemainingLifetime`/`HoldingTime`; constructors `Parse*` (string) and `*FromBytes` (wire) |
| Data flow | Leaf package: zero imports from IS-IS runtime or BGP-LS; serialize is buffer-first `WriteTo(buf, off) int` |
| Rule: buffer-first / no-sprintf-alloc | `String()` zero-alloc; serialize writes into caller buffer without per-call allocation |
| Rule: go-standards | Fixed identifiers are comparable value arrays usable as map keys |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `types` package directory | `ls internal/component/isis/types/` |
| Type files (metric.go holds both `Metric` and `PrefixMetric`) | `ls internal/component/isis/types/{systemid,sourceid,lspid,net,metric,sequence,lifetime}.go` |
| Test files for each type | `ls internal/component/isis/types/*_test.go` |
| Leaf-import constraint holds | `go list -deps ./internal/component/isis/types` shows only stdlib + Ze leaf helpers, no IS-IS runtime |
| All boundary rows tested | `go test ./internal/component/isis/types/` passes including boundary cases |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every `*FromBytes` validates exact/bounded length before indexing; no slice out-of-range on attacker-controlled wire lengths |
| Input validation | Every `Parse*` rejects wrong group count, odd nibble counts, and bad separators with an explicit error |
| Resource exhaustion | Variable-length `AreaID`/`NET` capped at 13 / 20 bytes; no unbounded allocation from a length field |
| Error leakage | Parse errors describe the shape problem without echoing unbounded attacker input |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read research guide section 3 / RFC summary |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Boundary test fails | Re-check the range table against ISO/IEC 10589 / RFC 5305 |
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
| Fixed-array value types for `SystemID`/`SourceID`/`LSPID` | Slices / pointers | Comparable with `==`, usable as map keys (adjacency table, LSDB), no heap pointer churn |
| Variable-length slice-backed `AreaID`/`NET` with explicit ordering | Fixed max-size arrays | Honest representation of 1..13 byte areas; ordering documented so CSNP range bounds are correct |
| Two wide metric types: `Metric` 24-bit (TLV 22 IS reachability) and `PrefixMetric` 32-bit (TLV 135/236 IP/IPv6 prefix) | One unified 24-bit type; also model narrow 6-bit metric | TLV 135 (RFC 5305) and TLV 236 (RFC 5308) carry a 32-bit prefix metric; capping it at 24-bit would reject/mangle valid peer routes. Narrow 6-bit metric stays out (wide only, per umbrella) |
| `SequenceNumber` exposes the reserved value (0) distinctly | Treat 0 as ordinary | ISO/IEC 10589: 0 is reserved and never a valid originated version; masking it causes origination bugs. (Purge is signalled by RemainingLifetime 0, a separate type) |

## Known Limitations
- Narrow 6-bit metric is not modelled as a type (wide metric only, per umbrella); revisit only if isis-2 interop decode demands a dedicated type
- No runtime behaviour (timers, wraparound handling) lives here; sequence wraparound and lifetime decrement are runtime concerns owned by `spec-isis-6-lsdb.md`

## RFC Documentation

Add `// ISO/IEC 10589 Section X.Y: "<quoted requirement>"` and `// RFC 5305 ...`
above enforcing code. MUST document: SystemID width (6 octets), NET shape
(Area + SystemID + NSEL, NSEL=0x00 for an IS), AreaID length bound (1..13),
IS-reachability metric range (24-bit, 0..16777215, TLV 22), IP/IPv6 prefix
metric range (32-bit, 0..4294967295, TLV 135 / TLV 236), and SequenceNumber 0
reserved (never a valid originated version; purge is signalled by RemainingLifetime 0).

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Eight IS-IS domain types with parse/format/compare/serialize | unit test | `go test ./internal/component/isis/types/` |
| Leaf package, no runtime imports | dependency check | `go list -deps ./internal/component/isis/types` |
| Consumed by the wire codec | downstream build | `spec-isis-2-wire.md` compiles against these types |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] End-to-End User Stories: each enabling chain terminates in a downstream spec with a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + unit tests for this package)
- [ ] Feature code integrated (`internal/component/isis/types/`)
- [ ] Leaf-import constraint proven (`go list -deps`)
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features (wide metric only; no narrow type unless isis-2 demands it)
- [ ] Single responsibility per type file
- [ ] Explicit > implicit behavior (sequence 0 reserved is explicit; lengths validated)
- [ ] Minimal coupling (leaf package)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests owned downstream (isis-2/4/13) referenced
- [ ] Interop N/A with justification (types carry no wire behaviour alone)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-isis-1-types.md`
- [ ] Summary included in commit
