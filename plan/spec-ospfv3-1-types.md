# Spec: ospfv3-1-types

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospfv3-0-umbrella.md |
| Phase | follow-up 1/13 |
| Updated | 2026-06-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `plan/spec-ospfv3-0-umbrella.md` for the OSPFv3 scope, package layout, and child dependency graph.
3. `rfc/short/rfc5340.md` for OSPFv3 identifiers, LSA type scope, prefix encoding, and header fields.
4. `rfc/short/rfc5838.md` for the multi-address-family deferral and Instance ID guardrail.
5. `ai/rules/module-tiers.md`, `ai/rules/buffer-first.md`, and `ai/rules/memory-architecture.md`.
6. `internal/analyze/statistics.go`, `internal/mrt/types.go`, and `internal/component/bgp/plugins/nlri/ls/types.go` to preserve existing non-routing OSPFv3 constants.

## Task

Create the OSPFv3 domain type leaf package `internal/plugins/ospfv3/types/`.
This is the first follow-up spec from `spec-ospfv3-0-umbrella.md` and the
foundation for the OSPFv3 packet codec, transport, LSDB, SPF, auth, CLI, and
interop specs. The package is pure value code: no sockets, timers, goroutines,
config loading, plugin lifecycle, LSDB maps, or route installation.

The package owns the reusable OSPFv3 identifiers, scalar ranges, bit fields,
and LSDB keys that every later child spec must share. It must not import the
OSPFv2 implementation. OSPFv2 and OSPFv3 both use Router IDs and LSA concepts,
but OSPFv3 has a 16-bit LS Type with embedded flooding scope, an Instance ID,
Interface IDs, 24-bit Options, IPv6 prefix encoding, and no OSPFv2 AuType
header fields.

## Required Reading

### Architecture Docs

- [ ] `plan/spec-ospfv3-0-umbrella.md` - OSPFv3 package layout and child dependency graph
  -> Constraint: `types` is a leaf package under `internal/plugins/ospfv3/types/`; later specs import it, it imports no OSPFv3 runtime package.
- [ ] `plan/spec-ospf-1-types.md` - OSPFv2 leaf-package pattern
  -> Constraint: mirror the value-type, parse, format, `FromBytes`, and buffer-first `WriteTo` conventions, but do not share OSPFv2 types.
- [ ] `ai/rules/module-tiers.md` - edge-plugin placement
  -> Constraint: new OSPFv3 code lives under `internal/plugins/ospfv3/` because it is a config-driven edge protocol with no reverse dependency.
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - encode/decode and allocation discipline
  -> Constraint: wire serialization writes into caller-owned buffers; helpers avoid per-packet allocations.
- [ ] `ai/rules/no-sprintf-alloc.md` - allocation-light formatting
  -> Constraint: hot-path `String()` and append helpers must not use `fmt.Sprintf` for Router ID, Area ID, Instance ID, Interface ID, LSA keys, or LS Types.

### RFC Summaries

- [ ] `rfc/short/rfc5340.md` - OSPFv3 base protocol
  -> Constraint: implement Router ID, Area ID, Instance ID, Interface ID, LS Type with U/S2/S1/function bits, LSA key fields, PrefixLength, PrefixOptions, and 24-bit Options exactly as RFC 5340 models them.
  -> Constraint: LS Type scope values are link-local `00`, area `01`, AS `10`, and reserved `11`; base code rejects reserved scope unless a later extension explicitly owns it.
  -> Constraint: Prefix byte length is `((PrefixLength + 31) / 32) * 4`; prefix length 0 is default route and consumes zero prefix bytes.
- [ ] `rfc/short/rfc5838.md` - support of address families in OSPFv3
  -> Constraint: Instance ID is explicit and validated now, but only IPv6 unicast is supported by this spec set. Multi-AF mappings are deferred.

**Key insights:**
- OSPFv3 LSDB identity is the OSPFv3 LSA header identity: LS Type, Link State ID, Advertising Router. LS Type carries flooding scope, so helpers must expose `Scope()` and `FunctionCode()` rather than treating the 16-bit value as a flat enum.
- Prefixes are not stored in Router-LSAs or Network-LSAs. Prefix helpers belong in this leaf package because packet codec, SPF prefix attachment, ABR summaries, externals, and NSSA all need the same length and padding rules.
- Existing OSPFv3 names in MRT and BGP-LS code are external-format metadata only. They must not become dependencies of the routing engine.

## Current Behavior

**Source files read:**
- [ ] `internal/mrt/types.go` - defines MRT type codes `TypeOSPFv3` and `TypeOSPFv3ET` for trace/analyze input.
  -> Constraint: do not move, rename, or reuse MRT constants as routing-engine packet constants.
- [ ] `internal/analyze/statistics.go` - maps MRT type `TypeOSPFv3` to display string `ospfv3`.
  -> Constraint: preserve analyze output and keep it independent from the new plugin.
- [ ] `internal/component/bgp/plugins/nlri/ls/types.go` - defines BGP-LS `ProtoOSPFv3` protocol ID 6.
  -> Constraint: BGP-LS protocol identifiers remain external metadata, not OSPFv3 engine types.
- [ ] `docs/guide/command-catalogue.md` - records OSPFv3 command spelling expectations.
  -> Constraint: later CLI spec uses `show ipv6 ospf`; this leaf package does not change docs.

**Behavior to preserve:**
- Existing MRT, analyze, and BGP-LS OSPFv3 constants keep their behavior and imports.
- OSPFv2 types remain under `internal/plugins/ospf/types/` and are not imported or aliased.
- No route installation, config parsing, CLI, web, metrics, or transport behavior changes in this spec.

**Behavior to change:**
- Add a new `internal/plugins/ospfv3/types/` package with OSPFv3-specific value types and tests.
- Add no production imports from other OSPFv3 runtime packages because those packages do not exist yet.

## Domain Types

| Type | Width / shape | Meaning |
|------|---------------|---------|
| `RouterID` | 4-byte fixed value, dotted-quad text | Router identity carried in OSPFv3 headers and LSA Advertising Router fields |
| `AreaID` | 4-byte fixed value, dotted-quad or integer text | Area identifier; zero is backbone |
| `InstanceID` | uint8 | Link-local OSPFv3 instance selector from the common header |
| `InterfaceID` | uint32 | Router-local interface identifier used in Hello, Router-LSA, Network-LSA, and SPF graph records |
| `LinkStateID` | 4-byte fixed value | Type-specific LSA identifier |
| `FloodScope` | enum | Link-local, area, AS, or reserved scope derived from LS Type S2/S1 bits |
| `LSType` | uint16 | OSPFv3 LSA type with U-bit, S2/S1 flooding-scope bits, and 13-bit function code |
| `LSAKey` | tuple `(LSType, LinkStateID, AdvertisingRouter)` | Comparable LSDB lookup key; sequence, age, checksum, and length are not identity |
| `LSSequenceNumber` | signed 32-bit | LSA version with InitialSequenceNumber `0x80000001` and MaxSequenceNumber `0x7fffffff` |
| `LSAge` | uint16 seconds | LSA age value; `MaxAge` is 3600 seconds |
| `Options` | 24-bit bitset stored in uint32 | OSPFv3 Options field from Hello, DD, Router-LSA, Network-LSA, and other LSAs |
| `PrefixLength` | uint8, range 0..128 | IPv6 prefix length in bits |
| `PrefixOptions` | uint8 bitset | NU, LA, P, DN, and reserved prefix option bits |
| `Metric` | uint32 with 24-bit bound | Generic OSPFv3 route metric value for prefix/external summaries; interface cost users may narrow to 16 bits |

Known LSA type constants in this package:

| Constant | Value | Scope | Function |
|----------|-------|-------|----------|
| `LSTypeRouter` | `0x2001` | area | Router-LSA |
| `LSTypeNetwork` | `0x2002` | area | Network-LSA |
| `LSTypeInterAreaPrefix` | `0x2003` | area | Inter-Area-Prefix-LSA |
| `LSTypeInterAreaRouter` | `0x2004` | area | Inter-Area-Router-LSA |
| `LSTypeASExternal` | `0x4005` | AS | AS-External-LSA |
| `LSTypeNSSA` | `0x2007` | area | NSSA-LSA |
| `LSTypeLink` | `0x0008` | link-local | Link-LSA |
| `LSTypeIntraAreaPrefix` | `0x2009` | area | Intra-Area-Prefix-LSA |

## Data Flow

### Entry Point

- Config strings: Router ID, Area ID, Instance ID, Interface ID, metrics, and prefix filters from the future `ospfv3` YANG subtree.
- CLI strings: identifiers in future `show ipv6 ospf database` filters.
- Wire bytes: OSPFv3 common header fields and LSA header fields handed to this package by `spec-ospfv3-2-wire.md`.
- Prefix bytes: IPv6 prefix fields inside Link, Intra-Area-Prefix, Inter-Area-Prefix, AS-External, and NSSA LSAs.

### Transformation Path

1. Parse printable strings into value types: `ParseRouterID`, `ParseAreaID`, `ParseInstanceID`, `ParseInterfaceID`, `ParseLSType`, `ParsePrefixLength`, and `ParseMetric`.
2. Parse wire bytes into value types: `RouterIDFromBytes`, `AreaIDFromBytes`, `InterfaceIDFromBytes`, `LinkStateIDFromBytes`, `LSTypeFromBytes`, and `LSAKeyFromHeader`.
3. Compute derived values: `LSType.Scope()`, `LSType.FunctionCode()`, `LSType.UnknownHandling()`, `PrefixLength.WordLen()`, `PrefixLength.ByteLen()`, and `PrefixOptions` predicates.
4. Use value types as map keys and sort keys: `LSAKey` is comparable and stable for LSDB indexes; formatting is canonical for CLI/web output.
5. Serialize value types into caller-owned buffers through `WriteTo(buf, off) int` helpers for the wire codec.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config / CLI string -> type | Parse helpers reject malformed and out-of-range input | `TestOSPFv3IdentifierParseFormat` |
| Wire bytes -> type | `FromBytes` helpers validate exact width before reading | `TestOSPFv3TypesFromBytesBounds` |
| LS Type -> scope/function | Bit helpers decode U/S2/S1/function code | `TestOSPFv3LSTypeScopeFunction` |
| Prefix fields -> byte length | Prefix length helpers compute RFC 5340 padded word length | `TestOSPFv3PrefixEncodingBoundaries` |
| Type -> wire bytes | `WriteTo` helpers write big-endian fixed-width fields into caller buffer | `TestOSPFv3TypesWriteTo` |
| Type -> LSDB key | `LSAKey` comparable struct used directly as map key | `TestOSPFv3LSAKeyComparable` |

### Integration Points

- `spec-ospfv3-2-wire.md` imports this package for header fields, LSA header fields, LS Type constants, prefix length helpers, and options bitsets.
- `spec-ospfv3-4-plugin-config.md` imports parse helpers for router-id, area-id, instance-id, interface-id, and metrics.
- `spec-ospfv3-5-interface-ism.md` imports `Options`, `InstanceID`, and `InterfaceID`.
- `spec-ospfv3-7-lsdb-flooding.md` imports `LSAKey`, `LSType`, `FloodScope`, `LSAge`, and `LSSequenceNumber`.
- `spec-ospfv3-8-spf-rib.md` imports Router ID, Interface ID, metric, prefix length/options, and LSA key types.
- `spec-ospfv3-12-auth.md` may import `InstanceID` and packet type constants from the packet package, not from this leaf package.

### Architectural Verification

- [ ] No bypassed layers: packet codec, config, LSDB, SPF, and CLI all use the same leaf types.
- [ ] No unintended coupling: this package imports no OSPFv2, IS-IS, BGP-LS, MRT, transport, LSDB, SPF, config, or plugin lifecycle package.
- [ ] No duplicated functionality: later specs do not redeclare LS Type constants, prefix length math, or LSDB key structs.
- [ ] Zero-copy preserved: serialization writes into caller buffers; prefix helpers compute lengths without allocating.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | The listed domain types are enough for the packet codec to start without adding another identifier package | OSPFv3 umbrella child table and RFC 5340 summary | `ospfv3-2-wire` must add a missing leaf type | `go test ./internal/plugins/ospfv3/packet/...` compiles against these types without local identifier duplicates | unvalidated |
| A-2 | `LSAKey` identity is `(LSType, LinkStateID, AdvertisingRouter)` because LS Type already carries scope | RFC 5340 LSA header and LS Type scope bits | LSDB needs an explicit duplicate scope field, risking mismatch | `TestOSPFv3LSAKeyComparable` and LSDB tests key by `LSType` directly | unvalidated |
| A-3 | `Options` is a 24-bit OSPFv3 bitset, not the OSPFv2 8-bit Options field | RFC 5340 packet formats and Options field | Hello/DD/LSA codec truncates high OSPFv3 option bits | `TestOSPFv3Options24BitRoundTrip` | unvalidated |
| A-4 | RFC 5838 multi-AF support should not affect the first-pass type API beyond preserving Instance ID | OSPFv3 umbrella out-of-scope table and RFC 5838 summary | Multi-AF later needs incompatible Instance ID or LSDB key shape | Future RFC 5838 spec can add AF mapping without changing `InstanceID` or `LSAKey` | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Accidentally sharing OSPFv2 identifiers hides OSPFv3 differences | Imports from `internal/plugins/ospf` appear in this package | Add a unit or lint guard that rejects OSPFv2 imports in `internal/plugins/ospfv3/` |
| R-2 | Treating LS Type as a flat enum loses flooding scope | Flood code has separate ad-hoc scope tables | Keep `Scope()` and `FunctionCode()` on `LSType`; LSDB/flooding uses those helpers only |
| R-3 | Prefix padding bugs pass self tests but fail FRR | FRR rejects LSAs with odd prefix lengths | Add boundary tests for /0, /1, /31, /32, /33, /64, /127, /128 and non-zero padding |
| R-4 | Formatter allocations become visible in CLI/database dumps | Benchmarks show allocation per LSA key or prefix | Provide append-style formatting helpers and benchmark database key formatting |

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ospfv3-2-wire` decodes a Router-LSA header | -> | `types.LSAKeyFromHeader`, `LSType.Scope`, `LSType.FunctionCode` | `TestOSPFv3WireUsesTypesLSAKey` |
| `ospfv3-4-plugin-config` parses `instance-id 0` and `area 0.0.0.0` | -> | `ParseInstanceID`, `ParseAreaID` | `TestOSPFv3ConfigUsesTypesParsers` |
| `ospfv3-8-spf-rib` attaches an IPv6 prefix from Intra-Area-Prefix-LSA | -> | `PrefixLength.ByteLen`, `PrefixOptions` | `TestOSPFv3SPFUsesTypesPrefix` |

These wiring tests are owned by later child specs because this package is a pure leaf. This spec still creates the helper-level tests named below.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Parse Router ID `1.2.3.4` and Area ID `0` / `0.0.0.0` | Values round-trip to canonical dotted-quad text and bytes |
| AC-2 | Parse Instance ID boundaries `0` and `255` | Accepted as uint8; `256` and negative strings are rejected |
| AC-3 | Parse Interface ID | Non-zero values accepted for active interface contexts; zero is available only for explicitly allowed placeholder contexts |
| AC-4 | Decode known LS Types | Scope and function code match RFC 5340 for Router, Network, Inter-Area-Prefix, Inter-Area-Router, AS-External, NSSA, Link, and Intra-Area-Prefix LSAs |
| AC-5 | Decode reserved LS Type scope | Reserved scope is identified and rejected by base helpers unless a caller explicitly allows extension handling |
| AC-6 | Build an `LSAKey` | Key is comparable, excludes age/sequence/checksum/length, and sorts stably by LS Type, Link State ID, Advertising Router |
| AC-7 | Prefix length values 0..128 | Byte and word lengths match RFC 5340; 129 is rejected |
| AC-8 | Prefix padding validation | Non-zero padding bits after PrefixLength are rejected |
| AC-9 | OSPFv3 Options values | 24-bit values round-trip; values above `0xFFFFFF` are rejected |
| AC-10 | LS sequence number boundaries | Initial, max, and reserved sequence values are represented and compared correctly for later freshness checks |
| AC-11 | Metric boundaries | Metric 1 and `0xFFFFFF` accepted; 0 and `0x1000000` rejected for generic route metrics |
| AC-12 | Package imports | `internal/plugins/ospfv3/types` imports no OSPFv2, runtime OSPFv3, IS-IS, BGP-LS, MRT, transport, config, LSDB, or SPF packages |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Configures `ospfv3 { router-id 1.1.1.1 area 0 interface eth0 instance-id 0 }` | config parser -> OSPFv3 type parsers -> typed config -> plugin start | `TestOSPFv3ConfigUsesTypesParsers` in `spec-ospfv3-4-plugin-config.md` |
| 2 | Receives an OSPFv3 Link-LSA from FRR | raw IPv6 -> packet codec -> `LSTypeLink` + `LSAKey` -> scope-aware LSDB | `TestOSPFv3WireUsesTypesLSAKey` in `spec-ospfv3-2-wire.md` |
| 3 | Installs a `/127` IPv6 route learned from Intra-Area-Prefix-LSA | LSA prefix fields -> `PrefixLength` helpers -> SPF prefix attachment -> Loc-RIB | `TestOSPFv3SPFUsesTypesPrefix` in `spec-ospfv3-8-spf-rib.md` |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3IdentifierParseFormat` | `internal/plugins/ospfv3/types/identifier_test.go` | Router ID, Area ID, Link State ID parse, bytes, and canonical string | |
| `TestOSPFv3InstanceIDBoundaries` | `internal/plugins/ospfv3/types/instance_test.go` | Instance ID range 0..255 | |
| `TestOSPFv3InterfaceIDBoundaries` | `internal/plugins/ospfv3/types/interface_test.go` | Interface ID parse, bytes, zero policy helper | |
| `TestOSPFv3LSTypeScopeFunction` | `internal/plugins/ospfv3/types/lsa_test.go` | U-bit, S2/S1 scope, function code extraction | |
| `TestOSPFv3KnownLSATypes` | `internal/plugins/ospfv3/types/lsa_test.go` | RFC 5340 base LSA constants and scopes | |
| `TestOSPFv3LSAKeyComparable` | `internal/plugins/ospfv3/types/lsa_test.go` | Map-key comparability and stable sort | |
| `TestOSPFv3SequenceBoundaries` | `internal/plugins/ospfv3/types/sequence_test.go` | Initial, max, reserved, and comparison helpers | |
| `TestOSPFv3AgeBoundaries` | `internal/plugins/ospfv3/types/age_test.go` | LS Age range and MaxAge predicate | |
| `TestOSPFv3Options24BitRoundTrip` | `internal/plugins/ospfv3/types/options_test.go` | 24-bit Options bitset and overflow rejection | |
| `TestOSPFv3PrefixEncodingBoundaries` | `internal/plugins/ospfv3/types/prefix_test.go` | Prefix length, byte length, word length, padding validation | |
| `TestOSPFv3MetricBoundaries` | `internal/plugins/ospfv3/types/metric_test.go` | 24-bit metric acceptance and rejection | |
| `TestOSPFv3TypesWriteTo` | `internal/plugins/ospfv3/types/write_test.go` | Buffer-first serialization writes exact big-endian bytes | |
| `TestOSPFv3TypesNoRuntimeImports` | `internal/plugins/ospfv3/types/imports_test.go` | Leaf package does not import forbidden runtime or sibling packages | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Router ID | 4 octets | `255.255.255.255` | 3 bytes | 5 bytes |
| Area ID | uint32 | `255.255.255.255` | n/a | 5-byte input |
| Instance ID | 0..255 | 255 | negative string | 256 |
| Interface ID | uint32 | 0xffffffff | n/a | 5-byte input |
| LS Type scope | S2/S1 00, 01, 10 valid; 11 reserved | `0x4005` | n/a | reserved scope accepted by default |
| LS Type function | 13-bit function code | 8191 | n/a | masked overflow changes U/S bits |
| PrefixLength | 0..128 | 128 | n/a | 129 |
| Prefix bytes | `((PrefixLength + 31) / 32) * 4` | 16 bytes for /128 | too short | non-zero padding bits |
| Options | 0..0xffffff | 0xffffff | n/a | 0x1000000 |
| LSSequenceNumber | signed 32-bit excluding reserved `0x80000000` | 0x7fffffff | reserved value | n/a |
| LSAge | 0..3600 for live LSAs | 3600 | n/a | 3601 unless explicitly allowed by caller |
| Metric | 1..0xffffff | 0xffffff | 0 | 0x1000000 |

### Functional Tests

None in this leaf spec. Functional tests begin when config and packet paths call these types:

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-adjacency.ci` | `test/ospfv3/ospfv3-adjacency.ci` | OSPFv3 config, Hello, and adjacency path uses Router ID, Area ID, Instance ID, Interface ID | owned by `spec-ospfv3-5-interface-ism.md` |
| `ospfv3-route-install.ci` | `test/ospfv3/ospfv3-route-install.ci` | Prefix helpers feed SPF route installation | owned by `spec-ospfv3-8-spf-rib.md` |

### Interop Tests

No interop test is owned by this leaf package. Interop starts with packet, transport, and FSM specs. This spec's required interop support is preserving exact RFC 5340 constants and prefix math for later `ospfv3-p2p-frr` and `ospfv3-broadcast-frr` scenarios.

## Files to Modify

None. This spec creates a new leaf package only.

## Files to Create

- `internal/plugins/ospfv3/types/routerid.go` - `RouterID`, `AreaID`, `LinkStateID`, parse/format/from-bytes/write helpers.
- `internal/plugins/ospfv3/types/instance.go` - `InstanceID` parse/range/write helpers.
- `internal/plugins/ospfv3/types/interface.go` - `InterfaceID` parse/range/write helpers and zero-policy predicates.
- `internal/plugins/ospfv3/types/lsa.go` - `FloodScope`, `LSType`, known LSA constants, U-bit/scope/function helpers, `LSAKey`.
- `internal/plugins/ospfv3/types/sequence.go` - `LSSequenceNumber`, initial/max/reserved constants, freshness comparison helpers.
- `internal/plugins/ospfv3/types/age.go` - `LSAge`, `MaxAge`, range helpers.
- `internal/plugins/ospfv3/types/options.go` - 24-bit OSPFv3 Options bitset and predicates.
- `internal/plugins/ospfv3/types/prefix.go` - `PrefixLength`, `PrefixOptions`, prefix byte/word length and padding validation helpers.
- `internal/plugins/ospfv3/types/metric.go` - 24-bit metric type and range helpers.
- `internal/plugins/ospfv3/types/*_test.go` - unit and boundary tests listed above.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | owned by `spec-ospfv3-4-plugin-config.md` |
| Config validators | No | later spec consumes parse helpers |
| Packet codec | No | owned by `spec-ospfv3-2-wire.md` |
| CLI | No | owned by `spec-ospfv3-13-cli-diag-interop.md` |
| Metrics | No | runtime specs own metrics |
| Docs | No | user-visible docs start when config or CLI exists |
| Interop | No | transport/FSM specs own interop |

## Implementation Steps

1. **Package skeleton** - create `internal/plugins/ospfv3/types/` with package docs and compile-only tests.
   - Tests: `TestOSPFv3TypesNoRuntimeImports` initially fails until package exists.
   - Verify: package imports only stdlib and approved leaf helpers.
2. **Identifier values** - implement Router ID, Area ID, Link State ID, Instance ID, Interface ID parse/from-bytes/string/write helpers.
   - Tests: identifier, instance, and interface boundary tests.
   - Verify: canonical strings and big-endian bytes round-trip.
3. **LSA keys and LS Type** - implement `FloodScope`, `LSType`, known constants, helpers, and `LSAKey`.
   - Tests: LS Type scope/function, known LSA constants, comparable key tests.
   - Verify: reserved scope is not silently treated as area or AS scope.
4. **Sequence, age, options, metrics** - implement scalar range helpers and predicates.
   - Tests: sequence, age, options, metric boundary tests.
   - Verify: 24-bit Options and Metric ranges are not truncated to OSPFv2 widths.
5. **Prefix helpers** - implement PrefixLength and PrefixOptions with byte/word length and padding validation.
   - Tests: prefix boundary and padding tests.
   - Verify: /0, /1, /31, /32, /33, /64, /127, and /128 cases pass.
6. **Wire integration handoff** - add comments in the spec audit naming which helpers `spec-ospfv3-2-wire.md` must consume.
   - Tests: no new code tests here.
   - Verify: no duplicate type declarations are needed in the wire spec.
7. **Full verification** - run `go test ./internal/plugins/ospfv3/types` and the spec validation command.

## Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every type in the Domain Types table has implementation and tests |
| Correctness | LS Type scope/function and prefix byte length match RFC 5340 |
| Boundaries | Every numeric range has last-valid and invalid-above tests |
| Architecture | Package is leaf-only and path is under `internal/plugins/ospfv3/` |
| No duplication | No OSPFv2 type imports or re-export aliases |
| Allocation | Formatting and serialization avoid hot-path allocation |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Add first OSPFv3 follow-up spec | planned | `plan/spec-ospfv3-1-types.md` | This file is the first implementation child from the umbrella |
| Keep OSPFv3 separate from OSPFv2 | planned | `internal/plugins/ospfv3/types/` | New leaf package, no OSPFv2 imports |
| Capture RFC 5340 type constraints | planned | Domain Types, TDD Plan | LS Type scope and prefix encoding are explicit |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-12 | planned | unit tests listed in TDD Plan | Fill with file:line and test output during implementation |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Unit and boundary tests | planned | `internal/plugins/ospfv3/types/*_test.go` | Created during implementation |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/ospfv3/types/` | planned | New package |

## Goal Validation

| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| Follow-up spec exists | spec file | `plan/spec-ospfv3-1-types.md` |
| First OSPFv3 implementation layer is scoped | spec content | Domain Types, Files to Create, TDD Plan, AC table |
| No OSPFv2 coupling planned | spec content | Required Reading constraints and AC-12 |

## Pre-Commit Verification

| Check | Command or Evidence |
|-------|---------------------|
| Spec metadata visible | `make ze-spec-status-json` shows `ospfv3-1-types` as `design` |
| Unit tests | `go test ./internal/plugins/ospfv3/types` after implementation |
| Stale placeholder scan | Search this spec for unresolved template markers and missing concrete tests |
| Wrong imports | `TestOSPFv3TypesNoRuntimeImports` plus code review of import list |

## Cross-References

- `plan/spec-ospfv3-0-umbrella.md` - parent umbrella.
- `rfc/short/rfc5340.md` - base protocol details.
- `rfc/short/rfc5838.md` - multi-AF deferral and Instance ID guardrail.
- `plan/spec-ospfv3-2-wire.md` - next spec after this one.
