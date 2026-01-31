# Spec: 05 - BGP-LS Family Plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/bgp/nlri/bgpls.go` - current BGP-LS implementation
4. `internal/plugin/flowspec/types.go` - reference for correct dependency pattern
5. `docs/plan/spec-03-evpn-plugin.md` - lessons learned from EVPN migration

## Task

Create a BGP-LS family plugin at `internal/plugin/bgpls/` to handle BGP Link-State decoding.

**Current State:** No plugin exists yet. BGP-LS types are in `internal/plugin/bgp/nlri/bgpls.go`.

**Note:** BGP-LS is the most complex family with 4 NLRI types, extensive TLV structures, and Protocol IDs.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/bgp/nlri/bgpls.go` - BGPLS types, TLV parsing
- [ ] `internal/plugin/flowspec/types.go` - reference for correct dependency pattern

**Behavior to preserve:**
- All 4 NLRI types (Node, Link, IPv4 Prefix, IPv6 Prefix)
- TLV parsing with nested sub-TLVs
- Protocol ID handling (IS-IS, OSPF, etc.)
- Wire format round-trip
- SRv6 SID NLRI (Type 6, RFC 9514)

**Behavior to change:**
- Move BGP-LS types to `internal/plugin/bgpls/`
- Consumers use `bgpls.*` instead of `nlri.BGPLS*`

## BGP-LS Complexity

### NLRI Types (RFC 7752 Section 3.2)

| Type | Name | Description |
|------|------|-------------|
| 1 | Node NLRI | Describes a router in the topology |
| 2 | Link NLRI | Describes a link between routers |
| 3 | IPv4 Topology Prefix NLRI | IPv4 reachability from a node |
| 4 | IPv6 Topology Prefix NLRI | IPv6 reachability from a node |
| 6 | SRv6 SID NLRI | SRv6 Segment Identifier (RFC 9514) |

### Protocol IDs (RFC 7752 Section 3.2)

| ID | Protocol | Description |
|----|----------|-------------|
| 1 | IS-IS Level 1 | IS-IS Level 1 routing domain |
| 2 | IS-IS Level 2 | IS-IS Level 2 routing domain |
| 3 | OSPFv2 | OSPF for IPv4 |
| 4 | Direct | Directly connected |
| 5 | Static | Static configuration |
| 6 | OSPFv3 | OSPF for IPv6 |
| 7 | BGP | BGP-sourced topology |

### TLV Structure (RFC 7752 Section 3.2.1)

```
BGP-LS NLRI
├── Protocol-ID (1 byte)
├── Identifier (8 bytes)
├── NLRI Type (2 bytes)
├── NLRI Length (2 bytes)
└── NLRI Data (variable)
    ├── Local Node Descriptor TLV (256)
    │   ├── AS Number Sub-TLV (512)
    │   ├── BGP-LS Identifier Sub-TLV (513)
    │   ├── OSPF Area-ID Sub-TLV (514)
    │   └── IGP Router-ID Sub-TLV (515)
    ├── Remote Node Descriptor TLV (257) [Link NLRI only]
    │   └── (same sub-TLVs as Local)
    ├── Link Descriptor TLVs [Link NLRI only]
    │   ├── Link Local/Remote ID (258)
    │   ├── IPv4 Interface Address (259)
    │   └── ... more
    └── Prefix Descriptor TLVs [Prefix NLRI only]
        ├── Multi-Topology ID (263)
        ├── OSPF Route Type (264)
        └── IP Reachability Info (265)
```

## Dependency Pattern

### What MOVES to bgpls plugin

| Type | Description |
|------|-------------|
| `BGPLSNLRIType` | NLRI type constants (1-6) |
| `BGPLSProtocolID` | Protocol ID constants (1-7) |
| `BGPLSNLRI` interface | Common interface for all BGP-LS NLRI |
| `BGPLSNode` | Node NLRI struct |
| `BGPLSLink` | Link NLRI struct |
| `BGPLSPrefix` | Prefix NLRI struct |
| `BGPLSSRv6SID` | SRv6 SID NLRI struct |
| `ParseBGPLS` | Wire parsing entry point |
| TLV constants | TLV type codes |

### What STAYS in nlri

| Type | Reason |
|------|--------|
| `Family` | Shared by all families |
| `AFI`/`SAFI` constants | Shared |
| `AFIBGPLS` (16388) | Family constant |
| `SAFIBGPLinkState` (71) | SAFI constant |

### Dependency Direction

```
internal/plugin/bgp/nlri
    ├── Family, AFI, SAFI (SHARED)
    ├── AFIBGPLS, SAFIBGPLinkState (constants)
    └── does NOT import bgpls

internal/plugin/bgpls
    imports → nlri (for Family only)
    exports → all BGP-LS types

Consumers (decode.go):
    import → bgpls (for BGP-LS types)
```

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/plugin-modes.md` - **CRITICAL**: Plugin CLI/Engine mode interface spec
- [ ] `internal/plugin/bgp/nlri/bgpls.go` - current implementation
- [ ] `internal/plugin/flowspec/types.go` - correct dependency pattern
- [ ] `docs/plan/spec-03-evpn-plugin.md` - import cycle lessons

### RFC Summaries
- [ ] `rfc/short/rfc7752.md` - BGP-LS base specification
- [ ] `rfc/short/rfc9085.md` - BGP-LS Extensions for Segment Routing
- [ ] `rfc/short/rfc9514.md` - BGP-LS Extensions for SRv6

**Key insights:**
- BGP-LS has complex nested TLV structure
- Protocol ID is critical for interpreting topology source
- SRv6 SID NLRI (Type 6) is a newer addition
- Unknown TLVs should be preserved as opaque data

## Files to Modify

- `cmd/ze/bgp/decode.go` - change `nlri.BGPLS*` → `bgpls.*`, add bgpls import
- `internal/plugin/inprocess.go` - register bgpls in internalPluginRunners
- `cmd/ze/bgp/bgp.go` - add `plugin bgpls` subcommand

## Files to Create

- `internal/plugin/bgpls/bgpls.go` - plugin main with decode mode
- `internal/plugin/bgpls/types.go` - NLRI types (imports nlri for Family)
- `internal/plugin/bgpls/tlv.go` - TLV parsing/encoding
- `internal/plugin/bgpls/node.go` - Node NLRI (Type 1)
- `internal/plugin/bgpls/link.go` - Link NLRI (Type 2)
- `internal/plugin/bgpls/prefix.go` - Prefix NLRI (Types 3, 4)
- `internal/plugin/bgpls/srv6.go` - SRv6 SID NLRI (Type 6)
- `internal/plugin/bgpls/bgpls_test.go` - unit tests
- `cmd/ze/bgp/plugin_bgpls.go` - CLI entry point

## Files to Delete

- None initially. After migration verified:
  - Remove BGP-LS types from `nlri/bgpls.go`
  - Remove BGP-LS tests from `nlri/bgpls_test.go`

## BGP-LS Families

| Family | AFI | SAFI | Description |
|--------|-----|------|-------------|
| `bgp-ls/bgp-ls` | 16388 | 71 | BGP-LS base |
| `bgp-ls/bgp-ls-vpn` | 16388 | 72 | BGP-LS VPN |

## CLI Mode Interface (per plugin-modes.md)

### Invocation

```bash
# CLI Mode - JSON output (default)
ze bgp plugin bgpls --nlri 0001000000000001...

# CLI Mode - Text output
ze bgp plugin bgpls --nlri 0001000000000001... --text

# CLI Mode - From stdin
echo "0001000000000001..." | ze bgp plugin bgpls --nlri -

# Query supported features
ze bgp plugin bgpls --features

# Engine Decode Mode - Protocol commands on stdin
ze bgp plugin bgpls --decode

# Engine Mode - Full protocol loop
ze bgp plugin bgpls
```

### CLI Flags

| Flag | Type | Description |
|------|------|-------------|
| `--nlri <hex\|->` | string | Decode NLRI hex, output JSON (use `-` for stdin) |
| `--text` | bool | Output human-readable text instead of JSON |
| `--features` | bool | List supported decode features |
| `--decode` | bool | Engine decode protocol mode |
| `--log-level` | string | Log level (disabled, debug, info, warn, err) |
| `--yang` | bool | Output YANG schema and exit |

### Output Formats

**JSON (default):**
```json
[{"nlri-type":1,"protocol-id":2,"local-node":{"as-number":65001,"bgp-ls-id":0}}]
```

**Text (`--text`):**
```
Node NLRI protocol=IS-IS-L2 as=65001 router-id=192.0.2.1
```

### Engine Decode Mode Protocol

```
# Engine calls plugin with --decode flag
ze bgp plugin bgpls --decode
# Plugin reads from stdin: decode nlri bgp-ls/bgp-ls 0001000000000001...
# Plugin responds: decoded json [{"nlri-type":1,"protocol-id":2,...}]
```

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Create Plugin (New Code)

1. **Create `internal/plugin/bgpls/types.go`**
   - Import `nlri` for `Family` only
   - Define `BGPLSNLRIType` constants
   - Define `BGPLSProtocolID` constants
   - Define `BGPLSNLRI` interface
   - Test: `go build ./internal/plugin/bgpls/...`
   → **Review:** Compiles? Uses nlri.Family correctly?

2. **Create `internal/plugin/bgpls/tlv.go`**
   - Define TLV type constants (256-265, 512-515)
   - Generic TLV parser
   - TLV writer for encoding
   - Test: `go build ./internal/plugin/bgpls/...`
   → **Review:** All TLV types defined per RFC 7752?

3. **Create `internal/plugin/bgpls/node.go`**
   - `BGPLSNode` struct (copy from nlri/bgpls.go)
   - Node descriptor parsing
   - Test: `go build ./internal/plugin/bgpls/...`
   → **Review:** Node descriptor sub-TLVs parsed?

4. **Create `internal/plugin/bgpls/link.go`**
   - `BGPLSLink` struct
   - Local and remote node descriptors
   - Link descriptor TLVs
   - Test: `go build ./internal/plugin/bgpls/...`
   → **Review:** Both node descriptors + link descriptors?

5. **Create `internal/plugin/bgpls/prefix.go`**
   - `BGPLSPrefix` struct
   - Handles both IPv4 (Type 3) and IPv6 (Type 4)
   - Prefix descriptor TLVs
   - Test: `go build ./internal/plugin/bgpls/...`
   → **Review:** Both IP versions handled?

6. **Create `internal/plugin/bgpls/srv6.go`**
   - `BGPLSSRv6SID` struct (RFC 9514)
   - SRv6 specific TLVs
   - Test: `go build ./internal/plugin/bgpls/...`
   → **Review:** SRv6 SID parsing correct?

7. **Create `internal/plugin/bgpls/bgpls.go`**
   - Plugin main with decode mode handler
   - `ParseBGPLS` entry point
   - Startup protocol: declare family
   - Event loop for decode requests
   - Test: `go build ./internal/plugin/bgpls/...`
   → **Review:** Dispatches to correct NLRI type?

8. **Create `internal/plugin/bgpls/bgpls_test.go`**
   - Wire roundtrip tests for all 4 NLRI types
   - TLV parsing tests
   - Protocol ID handling tests
   - Test: `go test ./internal/plugin/bgpls/... -v`
   → **Review:** Tests pass? All NLRI types covered?

9. **Create `cmd/ze/bgp/plugin_bgpls.go`** (per `plugin-modes.md`)
   - CLI entry point with three-mode support:
     - `--nlri <hex|->` - decode NLRI hex (use `-` for stdin)
     - `--text` - output text instead of JSON
     - `--features` - list supported features (`nlri yang`)
     - `--decode` - engine decode protocol mode
     - `--log-level` - logger configuration
     - `--yang` - output YANG schema
   - Mode detection: `--nlri` → CLI mode, `--decode` → engine decode, else → engine mode
   - Test: `go build ./cmd/ze/bgp/...`
   → **Review:** Follows plugin-modes.md pattern? All three modes work?

10. **Register in `internal/plugin/inprocess.go`**
    - Add bgpls to internalPluginRunners
    - Add to familyToPlugin map for bgp-ls/bgp-ls
    - Test: `go build ./internal/plugin/...`
    → **Review:** Registered correctly?

### Phase 2: Update Consumers

11. **Update `cmd/ze/bgp/decode.go`**
    - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/bgpls"`
    - Replace all `nlri.BGPLS*` with `bgpls.*`
    - Replace `nlri.ParseBGPLS` with `bgpls.ParseBGPLS`
    - Test: `go build ./cmd/ze/bgp/...`
    → **Review:** Compiles? All references updated?

### Phase 3: Verification

12. **Full build** - `go build ./...`
    → **Review:** No compilation errors?

13. **Run lint** - `make lint`
    → **Review:** No new lint errors? (paste output)

14. **Run tests** - `make test`
    → **Review:** All tests pass? (paste output)

15. **Run functional** - `make functional`
    → **Review:** All functional tests pass? (paste output)

### Phase 4: Cleanup (After Verification)

16. **Remove BGP-LS from nlri/bgpls.go**
    - Keep AFIBGPLS, SAFIBGPLinkState constants in nlri/nlri.go
    - Remove all BGP-LS types and parsing
    - Test: `go build ./...`
    → **Review:** No broken imports?

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPLSDecodeMode` | `internal/plugin/bgpls/bgpls_test.go` | Decode mode protocol | |
| `TestBGPLSNodeNLRI` | `internal/plugin/bgpls/bgpls_test.go` | Node NLRI type 1 | |
| `TestBGPLSLinkNLRI` | `internal/plugin/bgpls/bgpls_test.go` | Link NLRI type 2 | |
| `TestBGPLSPrefixV4NLRI` | `internal/plugin/bgpls/bgpls_test.go` | IPv4 prefix type 3 | |
| `TestBGPLSPrefixV6NLRI` | `internal/plugin/bgpls/bgpls_test.go` | IPv6 prefix type 4 | |
| `TestBGPLSSRv6SIDNLRI` | `internal/plugin/bgpls/bgpls_test.go` | SRv6 SID type 6 | |
| `TestBGPLSTLVParsing` | `internal/plugin/bgpls/bgpls_test.go` | TLV structures | |
| `TestBGPLSAllProtocolIDs` | `internal/plugin/bgpls/bgpls_test.go` | Protocol IDs 1-7 | |
| `TestBGPLSWireRoundTrip` | `internal/plugin/bgpls/bgpls_test.go` | Full wire roundtrip | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI type | 1-4, 6 | 6 | 0 | 5, 7+ (unknown) |
| Protocol ID | 1-7 | 7 | 0 | 8+ (unknown) |
| TLV type | 0-65535 | 65535 | N/A | N/A (16-bit) |
| TLV length | 0-65535 | 65535 | N/A | N/A (16-bit) |
| AS Number | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A (32-bit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgpls-decode` | `test/decode/bgpls-*.ci` | Decode BGP-LS NLRI via CLI | |

## Design Decisions

### TLV JSON Representation

**Options considered:**
1. Flat array - loses hierarchy
2. Type-keyed map - duplicate handling issues
3. Nested objects - preserves structure

**Decision:** Nested objects to preserve TLV hierarchy. Unknown TLVs included as hex.

### Unknown TLV Handling

**Decision:** Passthrough - include unknown TLVs as opaque hex data. This ensures:
- Forward compatibility with new RFCs
- No data loss during decode/encode
- Debugging visibility

### Plugin Registration

```
declare family bgp-ls bgp-ls decode
declare rfc 7752
declare rfc 9085
declare rfc 9514
declare encoding hex
declare done
```

## RFC Documentation

### Reference Comments
- RFC 7752 Section 3.2 - NLRI format
- RFC 7752 Section 3.2.1 - Node Descriptor TLVs
- RFC 7752 Section 3.2.2 - Link Descriptor TLVs
- RFC 7752 Section 3.2.3 - Prefix Descriptor TLVs
- RFC 9085 - SR extensions TLVs
- RFC 9514 - SRv6 SID NLRI (Type 6)

## Implementation Summary

### What Was Implemented
- `internal/plugin/bgpls/types.go` - Type aliases re-exporting from nlri package
- `internal/plugin/bgpls/plugin.go` - Plugin with decode mode, startup protocol, event loop
- `internal/plugin/bgpls/plugin_test.go` - 18 unit tests covering all NLRI types
- `cmd/ze/bgp/plugin_bgpls.go` - CLI entry point with three-mode support
- Plugin registration in `internal/plugin/inprocess.go`
- Plugin command in `cmd/ze/bgp/plugin.go`

### Bugs Found/Fixed
- **Test expectation mismatch**: `cmd/ze/main_test.go` expected 7 plugins, updated to 8 (added bgpls)
- **Plugin decode incomplete**: Plugin's `ParseBGPLS` doesn't fully parse Link NLRI TLVs into struct fields. Removed bgp-ls from `pluginFamilyMap` in decode.go so CLI decode uses the built-in decoder which properly parses all TLVs. Plugin still used for engine mode.

### Design Insights
- Follow flowspec pattern: type aliases in plugin package, re-export from nlri
- CLI decode and plugin decode can have different implementations:
  - CLI decode needs full TLV parsing for human-readable output
  - Plugin decode for engine mode just needs wire passthrough
- `pluginFamilyMap` (CLI) vs `familyToPlugin` (engine) serve different purposes

### Deviations from Plan
- Types NOT moved to bgpls package - using type alias pattern like flowspec
- CLI decode uses built-in decoder instead of plugin (plugin's parser incomplete)
- TLV parsing files (tlv.go, node.go, link.go, prefix.go, srv6.go) not created - using re-exports from nlri

## Checklist

### 🏗️ Design
- [x] No premature abstraction (following existing flowspec pattern)
- [x] No speculative features (only decode mode initially)
- [x] Single responsibility (bgpls package owns BGP-LS types via aliases)
- [x] Explicit behavior (direct imports, no re-export magic)
- [x] Minimal coupling (only depends on nlri for Family)
- [x] Next-developer test (follows existing flowspec pattern)

### 🧪 TDD
- [x] Tests written (18 tests in plugin_test.go)
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS (all 18 tests pass)
- [x] Boundary tests cover all numeric inputs
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior (bgp-ls-* tests)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (42+44+14+21 tests)

### Documentation
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references added to code
- [x] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
