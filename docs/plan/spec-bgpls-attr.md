# Spec: BGP-LS Attribute Type 29 TLV Support

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/wire/nlri-bgpls.md` - BGP-LS wire format
4. `docs/architecture/wire/bgpls-attribute-naming.md` - TLV naming convention (JSON keys + API keywords)
5. `internal/component/bgp/plugins/nlri/ls/types.go` - existing NLRI types and TLV constants
6. `internal/component/bgp/plugins/nlri/ls/types_descriptor.go` - descriptor WriteTo pattern
7. `cmd/ze/bgp/decode_bgpls.go` - CLI-only attribute parser (to be replaced)

## Task

Bring Ze's BGP-LS attribute type 29 support to parity with GoBGP. Ze's NLRI layer (types 1-4, 6) is complete with buffer-first encoding, fuzz tests, and VPN SAFI 72. The gap is the attribute TLV layer: Ze has CLI-only parsing for ~15 TLVs in `decode_bgpls.go`, while GoBGP has ~36 with full encode/decode as structured types.

This spec adds:
- TLV registration framework with `init()` pattern and decode-on-demand iterator
- ~40 attribute TLV types with `WriteTo(buf, off) int` + `Len() int` encoding
- 2 descriptor sub-TLVs (RFC 9086)
- Migration of CLI decoder to use new structured types
- Functional tests for all TLV categories

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri-bgpls.md` - BGP-LS wire format and Ze implementation patterns
  → Constraint: NLRI uses cached wire bytes + WriteTo pattern; attribute TLVs must follow same approach
- [ ] `docs/architecture/wire/attributes.md` - path attribute handling
  → Decision: attribute type 29 bytes flow through opaquely today; new types add structured access
- [ ] `docs/architecture/pool-architecture.md` - pool and dedup patterns
  → Constraint: attribute TLVs are not pooled/deduped in this spec (follow-up work)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7752.md` - BGP-LS base: NLRI format, attribute type 29, node/link/prefix TLVs
  → Constraint: TLV format is Type(2) + Length(2) + Value(variable); attribute is optional transitive
- [ ] `rfc/short/rfc9552.md` - BGP-LS updated (obsoletes 7752): any changes to attribute TLVs
  → Constraint: check for TLV changes vs RFC 7752
- [ ] `rfc/short/rfc9085.md` - SR-MPLS extensions: SR Capabilities, Adjacency SID, Prefix SID
  → Constraint: Adjacency SID flags V/L control label(3) vs index(4) encoding; SR Capabilities has sub-TLV ranges
- [ ] `rfc/short/rfc9086.md` - BGP-EPE: descriptor sub-TLVs 516/517, Peer SIDs 1101-1103
  → Constraint: Protocol-ID 7 (BGP); Peer SID TLVs share flag/weight/SID format with Adjacency SID
- [ ] `rfc/short/rfc9514.md` - SRv6: End.X SID, endpoint behavior, SID structure
  → Constraint: SRv6 End.X SID has nested sub-TLV 1252 (SID Structure)
- [ ] `rfc/short/rfc8571.md` - TE performance metrics: delay, jitter, loss, bandwidth TLVs
  → Constraint: delay values in microseconds; IEEE float32 for bandwidth; A flag for anomalous

**Key insights:**
- All attribute TLVs share the same Type(2)+Length(2)+Value(N) wire format
- TLV types are scoped by NLRI type (node attrs for Node NLRI, link attrs for Link NLRI, etc.)
- Forward compatibility: unknown TLVs silently skipped per RFC 7752
- SRv6 End.X SID and SR Capabilities contain nested sub-TLVs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/ls/types.go` (487 lines) - NLRI types, TLV constants 256-265/512-518, parser, WriteTo helpers
- [ ] `internal/component/bgp/plugins/nlri/ls/types_descriptor.go` (241 lines) - NodeDescriptor, LinkDescriptor, PrefixDescriptor with full WriteTo/Len/Bytes
- [ ] `internal/component/bgp/plugins/nlri/ls/types_nlri.go` (349 lines) - BGPLSNode, BGPLSLink, BGPLSPrefix with full WriteTo/Len/Bytes/Parse
- [ ] `internal/component/bgp/plugins/nlri/ls/types_srv6.go` (179 lines) - BGPLSSRv6SID and SRv6SIDDescriptor
- [ ] `internal/component/bgp/plugins/nlri/ls/plugin.go` (658 lines) - decode-only plugin: NLRI parsing + JSON formatting
- [ ] `internal/component/bgp/plugins/nlri/ls/register.go` (51 lines) - init() registration with registry.Register()
- [ ] `cmd/ze/bgp/decode_bgpls.go` (337 lines) - CLI-only attribute TLV parser (switch/case on ~15 TLV codes)
- [ ] `cmd/ze/bgp/decode_update.go` - calls parseBGPLSAttribute() at line 191 for attribute code 29
- [ ] `internal/component/bgp/attribute/attribute.go` - AttrBGPLS = 29 registered as constant

**Behavior to preserve:**
- NLRI encode/decode unchanged (types 1-4, 6 are complete)
- Existing .ci test expectations for NLRI decode output
- JSON key naming convention (kebab-case)
- Forward compatibility: unknown TLV types silently skipped
- Attribute type 29 bytes flow through opaquely in reactor (raw bytes forwarded unchanged)

**Behavior to change:**
- CLI decoder (`decode_bgpls.go`) will use new structured types instead of inline switch/case
- JSON output from attribute TLVs will be produced by typed TLV structs, not ad-hoc map building
- Plugin will gain attribute TLV decode capability (not just NLRI)

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes arrive as BGP UPDATE path attribute type 29 (optional, transitive)
- Attribute bytes are raw TLV stream: `[Type:2][Len:2][Value:N]...`

### Transformation Path
1. UPDATE received by reactor, attribute type 29 bytes extracted
2. For display/CLI: TLV iterator walks raw bytes, yields `(code uint16, value []byte)` pairs
3. Consumer calls registered decoder for codes it needs: `tlvRegistry[code](value) -> typed struct`
4. For encoding: typed struct `.WriteTo(buf, off) int` writes TLV header + value
5. For JSON: typed struct has `.ToJSON() map[string]any` method

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Attribute bytes | Attribute parser extracts type 29 value | [ ] |
| Attribute bytes -> TLV iterator | Iterator yields (code, raw) pairs | [ ] |
| Raw TLV -> Typed struct | Registry lookup + decode function | [ ] |
| Typed struct -> Wire | WriteTo(buf, off) into pooled buffer | [ ] |
| Typed struct -> JSON | ToJSON() for display | [ ] |

### Integration Points
- `internal/component/bgp/attribute/attribute.go` - AttrBGPLS constant (already exists)
- `cmd/ze/bgp/decode_update.go:191` - CLI attribute decode (replace parseBGPLSAttribute call)
- `internal/component/bgp/plugins/nlri/ls/plugin.go` - plugin decode path (add attribute TLV support)
- `internal/component/bgp/plugins/nlri/ls/register.go` - extend registration if needed

### Architectural Verification
- [ ] No bypassed layers (TLV types live in ls/ plugin, accessed through registry)
- [ ] No unintended coupling (reactor never imports ls/ directly)
- [ ] No duplicated functionality (replaces CLI switch/case with registry pattern)
- [ ] Zero-copy preserved where applicable (iterator walks raw bytes without copying)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze bgp decode` with UPDATE containing attr 29 | -> | TLV registry decode | `test/decode/bgp-ls-attr-node.ci` |
| `ze bgp decode` with UPDATE containing SR TLVs | -> | SR TLV decode | `test/decode/bgp-ls-attr-sr.ci` |
| `ze bgp decode` with UPDATE containing delay TLVs | -> | Delay TLV decode | `test/decode/bgp-ls-attr-delay.ci` |
| `ze bgp decode` with UPDATE containing SRv6 attr TLVs | -> | SRv6 attr TLV decode | `test/decode/bgp-ls-attr-srv6.ci` |
| `ze bgp decode` with UPDATE containing EPE peer SIDs | -> | EPE TLV decode | `test/decode/bgp-ls-attr-epe.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | UPDATE with node attribute TLVs (1024-1029) | JSON output contains `node-flags`, `node-name`, `area-id`, `local-router-ids` with correct values |
| AC-2 | UPDATE with link attribute TLVs (1088-1092, 1095-1098) | JSON output contains `admin-group-mask`, `maximum-link-bandwidth`, `igp-metric`, `link-name` etc. |
| AC-3 | UPDATE with SR Capabilities TLV (1034) | JSON output contains `sr-capabilities` with flags and label ranges |
| AC-4 | UPDATE with Adjacency SID TLV (1099) | JSON output contains `sr-adj` with flags (V/L), weight, SID values |
| AC-5 | UPDATE with Peer SID TLVs (1101-1103) | JSON output contains `peer-node-sid`, `peer-adj-sid`, `peer-set-sid` |
| AC-6 | UPDATE with delay metric TLVs (1114-1116) | JSON output contains `unidirectional-link-delay`, `min-max-link-delay`, `delay-variation` |
| AC-7 | UPDATE with SRv6 attribute TLVs (1250-1252) | JSON output contains `srv6-endpoint-behavior`, `srv6-sid-structure` |
| AC-8 | UPDATE with Prefix SID TLV (1158) and SID/Label (1161) | JSON output contains `prefix-sid` with flags, algorithm, SID value |
| AC-9 | TLV round-trip encode/decode | For each TLV type: construct struct -> WriteTo -> parse back -> fields match |
| AC-10 | Unknown TLV code in attribute | Silently skipped, other TLVs still parsed correctly |
| AC-11 | Truncated TLV data | Returns error, does not panic |
| AC-12 | Existing NLRI decode .ci tests still pass | No regressions in NLRI output format |
| AC-13 | Node descriptor sub-TLVs 516/517 | Node NLRI with BGP Router-ID or Confed Member parsed correctly |
| AC-14 | TLV registration via init() | Each TLV type self-registers; no central switch/case in parser |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTLVIterator` | `internal/.../ls/attr_test.go` | Iterator yields correct (code, value) pairs from raw bytes | |
| `TestTLVIteratorTruncated` | `internal/.../ls/attr_test.go` | Iterator stops cleanly on truncated data | |
| `TestTLVIteratorEmpty` | `internal/.../ls/attr_test.go` | Iterator handles empty input | |
| `TestTLVRegistration` | `internal/.../ls/attr_test.go` | All expected TLV codes have registered decoders | |
| `TestNodeFlagBits` | `internal/.../ls/attr_node_test.go` | TLV 1024: decode flags, encode round-trip | |
| `TestOpaqueNodeAttr` | `internal/.../ls/attr_node_test.go` | TLV 1025: decode/encode opaque bytes | |
| `TestNodeName` | `internal/.../ls/attr_node_test.go` | TLV 1026: decode/encode string | |
| `TestISISAreaID` | `internal/.../ls/attr_node_test.go` | TLV 1027: decode/encode variable bytes | |
| `TestLocalRouterID` | `internal/.../ls/attr_node_test.go` | TLVs 1028/1029: IPv4 and IPv6 | |
| `TestSRCapabilities` | `internal/.../ls/attr_node_test.go` | TLV 1034: flags + label ranges with sub-TLV 1161 | |
| `TestSRAlgorithm` | `internal/.../ls/attr_node_test.go` | TLV 1035: algorithm byte array | |
| `TestSRLocalBlock` | `internal/.../ls/attr_node_test.go` | TLV 1036: flags + label ranges | |
| `TestRemoteRouterID` | `internal/.../ls/attr_link_test.go` | TLVs 1030/1031: IPv4 and IPv6 | |
| `TestAdminGroup` | `internal/.../ls/attr_link_test.go` | TLV 1088: 4-byte mask | |
| `TestBandwidthTLVs` | `internal/.../ls/attr_link_test.go` | TLVs 1089/1090/1091: IEEE float32 bandwidth | |
| `TestTEDefaultMetric` | `internal/.../ls/attr_link_test.go` | TLV 1092: 4-byte metric | |
| `TestIGPMetric` | `internal/.../ls/attr_link_test.go` | TLV 1095: variable-length (1/2/3/4 bytes) | |
| `TestSRLG` | `internal/.../ls/attr_link_test.go` | TLV 1096: array of uint32 | |
| `TestOpaqueLinkAttr` | `internal/.../ls/attr_link_test.go` | TLV 1097: opaque bytes | |
| `TestLinkName` | `internal/.../ls/attr_link_test.go` | TLV 1098: string | |
| `TestAdjacencySID` | `internal/.../ls/attr_link_test.go` | TLV 1099: V/L flag combos, weight, label vs index | |
| `TestPeerSIDs` | `internal/.../ls/attr_link_test.go` | TLVs 1101/1102/1103: flags, weight, SID | |
| `TestSRv6EndXSID` | `internal/.../ls/attr_link_test.go` | TLV 1106: behavior, flags, SIDs, nested SID Structure | |
| `TestSRv6LANEndXSID` | `internal/.../ls/attr_link_test.go` | TLVs 1107/1108: IS-IS (6-byte) and OSPFv3 (4-byte) neighbor ID | |
| `TestDelayMetrics` | `internal/.../ls/attr_link_test.go` | TLVs 1114/1115/1116: microseconds, A flag, min/max | |
| `TestIGPFlags` | `internal/.../ls/attr_prefix_test.go` | TLV 1152: D/N/L flags | |
| `TestPrefixMetric` | `internal/.../ls/attr_prefix_test.go` | TLV 1155: 4-byte metric | |
| `TestOpaquePrefixAttr` | `internal/.../ls/attr_prefix_test.go` | TLV 1157: opaque bytes | |
| `TestPrefixSID` | `internal/.../ls/attr_prefix_test.go` | TLV 1158: flags, algorithm, SID (label vs index) | |
| `TestSIDLabel` | `internal/.../ls/attr_prefix_test.go` | TLV 1161: 3-byte label, 4-byte index | |
| `TestSRPrefixAttrFlags` | `internal/.../ls/attr_prefix_test.go` | TLV 1170: X/R/N flags | |
| `TestSourceRouterID` | `internal/.../ls/attr_prefix_test.go` | TLV 1171: 4 or 16 byte router ID | |
| `TestSRv6EndpointBehavior` | `internal/.../ls/attr_srv6_test.go` | TLV 1250: behavior, flags, algorithm | |
| `TestSRv6BGPPeerNodeSID` | `internal/.../ls/attr_srv6_test.go` | TLV 1251: flags, weight, peer AS, peer BGP ID | |
| `TestSRv6SIDStructure` | `internal/.../ls/attr_srv6_test.go` | TLV 1252: loc_block, loc_node, func, arg lengths | |
| `TestBGPRouterID` | `internal/.../ls/types_descriptor_test.go` | Descriptor sub-TLV 516: 4-byte router ID | |
| `TestConfedMember` | `internal/.../ls/types_descriptor_test.go` | Descriptor sub-TLV 517: 4-byte AS | |
| `FuzzAttrTLVParse` | `internal/.../ls/attr_fuzz_test.go` | Fuzz: random bytes to TLV iterator + decode | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| TLV Length | 0-65535 | 65535 | N/A | N/A (2-byte field) |
| IGP Metric length | 1-4 bytes | 4 | 0 (empty) | 5+ (truncate to 4) |
| Adjacency SID V/L flags | 0b00, 0b11 | 0b11 (label) | N/A | 0b01, 0b10 (invalid combos) |
| IEEE float32 bandwidth | 0.0 - 3.4e38 | max float32 | N/A | N/A |
| Delay microseconds | 0-16777215 (24 bits) | 16777215 | N/A | bit 24+ are flags |
| SRv6 SID length | 16 bytes | 16 | 15 (truncated) | 17+ (extra ignored) |
| SR Label | 0-1048575 (20 bits) | 1048575 | N/A | N/A (3-byte field) |
| SID/Label TLV length | 3 or 4 | 4 (index) | 2 (truncated) | 5+ |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-ls-attr-node` | `test/decode/bgp-ls-attr-node.ci` | Decode UPDATE with node attributes (flags, name, router IDs) | |
| `bgp-ls-attr-link` | `test/decode/bgp-ls-attr-link.ci` | Decode UPDATE with link attributes (BW, metric, admin group) | |
| `bgp-ls-attr-sr` | `test/decode/bgp-ls-attr-sr.ci` | Decode UPDATE with SR TLVs (capabilities, adj SID, prefix SID) | |
| `bgp-ls-attr-delay` | `test/decode/bgp-ls-attr-delay.ci` | Decode UPDATE with delay metric TLVs | |
| `bgp-ls-attr-srv6` | `test/decode/bgp-ls-attr-srv6.ci` | Decode UPDATE with SRv6 endpoint behavior and SID structure | |
| `bgp-ls-attr-epe` | `test/decode/bgp-ls-attr-epe.ci` | Decode UPDATE with EPE peer SID TLVs | |

### Future (if deferring any tests)
- Pool/dedup integration tests (requires pool architecture for attribute type 29, separate spec)
- Attribute encoding into UPDATE building (requires UPDATE builder support for type 29, separate spec)

## Files to Modify
- `internal/component/bgp/plugins/nlri/ls/types.go` - add attribute TLV constants, TLV interface, registration map
- `internal/component/bgp/plugins/nlri/ls/types_descriptor.go` - add sub-TLVs 516/517 to NodeDescriptor
- `internal/component/bgp/plugins/nlri/ls/plugin.go` - integrate attribute TLV decode into plugin
- `cmd/ze/bgp/decode_bgpls.go` - replace switch/case with registry-based decode
- `cmd/ze/bgp/decode_update.go` - update call site at line 191

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| RPC count in architecture docs | No | - |
| CLI commands/flags | No | - |
| CLI usage/help text | No | - |
| API commands doc | No | - |
| Plugin SDK docs | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No (decode tests cover it) | - |

## Files to Create

### Source Files
| File | Purpose |
|------|---------|
| `internal/.../ls/attr.go` | TLV interface, registration map, iterator, common decode helpers |
| `internal/.../ls/attr_node.go` | Node attribute TLV types (1024-1029, 1034-1036) with init() registration |
| `internal/.../ls/attr_link.go` | Link attribute TLV types (1030-1031, 1088-1099, 1101-1103, 1106-1108, 1114-1116) with init() registration |
| `internal/.../ls/attr_prefix.go` | Prefix attribute TLV types (1152, 1155, 1157-1158, 1161, 1170-1171) with init() registration |
| `internal/.../ls/attr_srv6.go` | SRv6 attribute TLV types (1250-1252) with init() registration |

### Test Files
| File | Purpose |
|------|---------|
| `internal/.../ls/attr_test.go` | Framework tests: iterator, registration, round-trip |
| `internal/.../ls/attr_node_test.go` | Node attribute TLV unit tests |
| `internal/.../ls/attr_link_test.go` | Link attribute TLV unit tests |
| `internal/.../ls/attr_prefix_test.go` | Prefix attribute TLV unit tests |
| `internal/.../ls/attr_srv6_test.go` | SRv6 attribute TLV unit tests |
| `internal/.../ls/attr_fuzz_test.go` | Fuzz tests for attribute TLV parsing |
| `test/decode/bgp-ls-attr-node.ci` | Functional test: node attributes |
| `test/decode/bgp-ls-attr-link.ci` | Functional test: link attributes |
| `test/decode/bgp-ls-attr-sr.ci` | Functional test: SR TLVs |
| `test/decode/bgp-ls-attr-delay.ci` | Functional test: delay metrics |
| `test/decode/bgp-ls-attr-srv6.ci` | Functional test: SRv6 attribute TLVs |
| `test/decode/bgp-ls-attr-epe.ci` | Functional test: EPE peer SIDs |

## TLV Inventory

### Attribute TLV Types (40 total)

#### Node Attributes (9 types)
| Code | Name | Length | RFC | Phase |
|------|------|--------|-----|-------|
| 1024 | Node Flag Bits | 1 byte (O/T/E/B/R/V flags) | 7752 3.3.1.1 | 1 |
| 1025 | Opaque Node Attr | variable bytes | 7752 3.3.1.5 | 1 |
| 1026 | Node Name | variable string | 7752 3.3.1.3 | 1 |
| 1027 | IS-IS Area ID | variable bytes | 7752 3.3.1.2 | 1 |
| 1028 | IPv4 Router-ID Local | 4 bytes | 7752 3.3.1.4 | 1 |
| 1029 | IPv6 Router-ID Local | 16 bytes | 7752 3.3.1.4 | 1 |
| 1034 | SR Capabilities | flags + label ranges (sub-TLV 1161) | 9085 3 | 2 |
| 1035 | SR Algorithm | byte array | 9085 4 | 2 |
| 1036 | SR Local Block | flags + label ranges (sub-TLV 1161) | 9085 5 | 2 |

#### Link Attributes (22 types)
| Code | Name | Length | RFC | Phase |
|------|------|--------|-----|-------|
| 1030 | IPv4 Router-ID Remote | 4 bytes | 7752 3.3.2.1 | 1 |
| 1031 | IPv6 Router-ID Remote | 16 bytes | 7752 3.3.2.1 | 1 |
| 1088 | Admin Group | 4 bytes (bit mask) | 7752 3.3.2.5 | 1 |
| 1089 | Max Link Bandwidth | 4 bytes (IEEE float32) | 7752 3.3.2.3 | 1 |
| 1090 | Max Reservable Bandwidth | 4 bytes (IEEE float32) | 7752 3.3.2.4 | 1 |
| 1091 | Unreserved Bandwidth | 32 bytes (8 x IEEE float32) | 7752 3.3.2.4 | 1 |
| 1092 | TE Default Metric | 4 bytes | 7752 3.3.2.7 | 1 |
| 1095 | IGP Metric | 1-4 bytes (variable) | 7752 3.3.2.4 | 1 |
| 1096 | SRLG | variable (array of uint32) | 7752 3.3.2.6 | 1 |
| 1097 | Opaque Link Attr | variable bytes | 7752 3.3.2.10 | 1 |
| 1098 | Link Name | variable string | 7752 3.3.2.7 | 1 |
| 1099 | Adjacency SID | flags + weight + 2B reserved + SID | 9085 2.2.1 | 2 |
| 1101 | Peer Node SID | flags + weight + SID | 9086 5 | 3 |
| 1102 | Peer Adjacency SID | flags + weight + SID | 9086 5 | 3 |
| 1103 | Peer Set SID | flags + weight + SID | 9086 5 | 3 |
| 1106 | SRv6 End.X SID | behavior + flags + algo + weight + SIDs + sub-TLVs | 9514 4 | 3 |
| 1107 | IS-IS SRv6 LAN End.X SID | same as 1106 + 6-byte neighbor ID | 9514 4 | 3 |
| 1108 | OSPFv3 SRv6 LAN End.X SID | same as 1106 + 4-byte neighbor ID | 9514 4 | 3 |
| 1114 | Unidirectional Link Delay | 4 bytes (A flag + 24-bit microseconds) | 8571 3 | 3 |
| 1115 | Min/Max Link Delay | 8 bytes (A flag + 2 x 24-bit microseconds) | 8571 4 | 3 |
| 1116 | Delay Variation | 4 bytes (24-bit microseconds) | 8571 5 | 3 |

#### Prefix Attributes (7 types)
| Code | Name | Length | RFC | Phase |
|------|------|--------|-----|-------|
| 1152 | IGP Flags | 1 byte (D/N/L flags) | 7752 3.3.3.1 | 1 |
| 1155 | Prefix Metric | 4 bytes | 7752 3.3.3.4 | 1 |
| 1157 | Opaque Prefix Attr | variable bytes | 7752 3.3.3.6 | 1 |
| 1158 | Prefix SID | flags + algo + SID (label or index) | 9085 2.3.1 | 2 |
| 1161 | SID/Label | 3-byte label or 4-byte index | 9085 2.1.1 | 2 |
| 1170 | SR Prefix Attr Flags | 1 byte (X/R/N flags) | 9085 2.3.2 | 2 |
| 1171 | Source Router ID | 4 or 16 bytes | 9085 2.3.3 | 2 |

#### SRv6 Attributes (3 types)
| Code | Name | Length | RFC | Phase |
|------|------|--------|-----|-------|
| 1250 | SRv6 Endpoint Behavior | 2+1+1 bytes (behavior + flags + algo) | 9514 8 | 3 |
| 1251 | SRv6 BGP Peer Node SID | flags + weight + peer AS + peer BGP ID | 9514 5.1 | 3 |
| 1252 | SRv6 SID Structure | 4 bytes (block + node + func + arg lengths) | 9514 8 | 3 |

### Descriptor Sub-TLVs (2 types, added to NodeDescriptor)
| Code | Name | Length | RFC | Phase |
|------|------|--------|-----|-------|
| 516 | BGP Router-ID | 4 bytes (IPv4 address) | 9086 4.1 | 3 |
| 517 | BGP Confederation Member | 4 bytes (AS number) | 9086 4.2 | 3 |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: Framework + RFC 7752 TLVs** -- TLV interface, registration, iterator, and 20 base attribute types
   - Create `attr.go`: `LsAttrTLV` interface (`Code() uint16`, `Len() int`, `WriteTo(buf, off) int`, `ToJSON() map[string]any`), `RegisterLsAttrTLV(code, decoderFunc)` map, `IterateAttrTLVs(data) iter` function
   - Create `attr_node.go`: TLVs 1024-1029 with init() registration
   - Create `attr_link.go`: TLVs 1030-1031, 1088-1092, 1095-1098 with init() registration
   - Create `attr_prefix.go`: TLVs 1152, 1155, 1157 with init() registration
   - Tests: `TestTLVIterator*`, `TestNodeFlagBits`, `TestNodeName`, all RFC 7752 TLV tests
   - Files: `attr.go`, `attr_node.go`, `attr_link.go`, `attr_prefix.go` + test files
   - Verify: tests fail -> implement -> tests pass

2. **Phase 2: SR-MPLS extensions (RFC 9085)** -- 8 SR attribute TLV types
   - Add to `attr_node.go`: TLVs 1034-1036 (SR Capabilities with sub-TLV parsing, SR Algorithm, SR Local Block)
   - Add to `attr_link.go`: TLV 1099 (Adjacency SID with V/L flag encoding)
   - Add to `attr_prefix.go`: TLVs 1158, 1161, 1170, 1171 (Prefix SID, SID/Label, SR flags, Source Router ID)
   - Tests: `TestSRCapabilities`, `TestAdjacencySID`, `TestPrefixSID`, `TestSIDLabel`
   - Verify: tests fail -> implement -> tests pass

3. **Phase 3: EPE + Delay + SRv6 + Descriptors** -- 14 types + 2 descriptor sub-TLVs
   - Add to `attr_link.go`: TLVs 1101-1103 (Peer SIDs), 1106-1108 (SRv6 End.X variants), 1114-1116 (delay metrics)
   - Create `attr_srv6.go`: TLVs 1250-1252 (SRv6 endpoint behavior, BGP peer node SID, SID structure)
   - Add to `types_descriptor.go`: Sub-TLVs 516/517 in NodeDescriptor struct + WriteTo + parse
   - Add to `types.go`: TLV constants 516/517; update parseNodeDescriptorTLVs switch
   - Tests: `TestPeerSIDs`, `TestDelayMetrics`, `TestSRv6EndpointBehavior`, `TestBGPRouterID`, `TestConfedMember`
   - Verify: tests fail -> implement -> tests pass

4. **Phase 4: Integration + CLI migration** -- Replace CLI decoder with registry pattern
   - Modify `cmd/ze/bgp/decode_bgpls.go`: replace switch/case with `IterateAttrTLVs` + registry lookup + `ToJSON()`
   - Modify `cmd/ze/bgp/decode_update.go`: update call site
   - Modify `plugin.go`: add attribute TLV decode path in plugin decode
   - Create all `.ci` functional tests
   - Create `attr_fuzz_test.go`
   - Verify: all existing .ci tests pass, new .ci tests pass

5. **Full verification** -- `make ze-verify` (lint + all tests except fuzz)
6. **Complete spec** -- Fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every TLV code in the inventory has a struct, WriteTo, Len, ToJSON, init() registration |
| Correctness | IEEE float32 bandwidth encoding uses `math.Float32frombits`/`math.Float32bits`; delay microseconds masked to 24 bits |
| Naming | JSON keys match existing `decode_bgpls.go` output (kebab-case); struct names follow `LsTLV<Name>` pattern |
| Data flow | Iterator walks raw bytes without copy; decoder produces typed struct; WriteTo writes to caller buffer |
| Rule: no-layering | CLI switch/case fully replaced by registry decode, no hybrid |
| Rule: buffer-first | All WriteTo methods write to caller buffer at offset, no append(), no returning []byte from helpers |
| Rule: init() registration | Every TLV type registers in init(), no central switch/case in framework |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `attr.go` with LsAttrTLV interface and registry | `grep "RegisterLsAttrTLV" internal/.../ls/attr.go` |
| `attr_node.go` with 9 TLV types | `grep "func init()" internal/.../ls/attr_node.go` |
| `attr_link.go` with 22 TLV types | `grep "func init()" internal/.../ls/attr_link.go` |
| `attr_prefix.go` with 7 TLV types | `grep "func init()" internal/.../ls/attr_prefix.go` |
| `attr_srv6.go` with 3 TLV types | `grep "func init()" internal/.../ls/attr_srv6.go` |
| NodeDescriptor handles TLVs 516/517 | `grep "516\|517" internal/.../ls/types_descriptor.go` |
| CLI decoder uses registry | `grep "IterateAttrTLVs\|tlvRegistry" cmd/ze/bgp/decode_bgpls.go` |
| 6 functional .ci tests | `ls test/decode/bgp-ls-attr-*.ci` |
| Fuzz test for attribute parsing | `grep "FuzzAttrTLV" internal/.../ls/attr_fuzz_test.go` |
| All 40 TLV codes registered | test `TestTLVRegistration` passes with count check |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | TLV length checked against remaining data before access; no out-of-bounds reads |
| Integer overflow | TLV length is uint16 (max 65535); offset arithmetic checked against buffer bounds |
| IEEE float parsing | `math.Float32frombits` is safe; NaN/Inf values passed through (not security issue) |
| String handling | Node name and link name from TLV value; length bounded by TLV length field |
| Panic prevention | No index operations without length check; fuzz test covers random input |
| Resource exhaustion | Iterator bounded by input length; no unbounded allocation from TLV data |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, TLV length constraints, flag definitions, IEEE float encoding.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit**
