# Spec: 05 - BGP-LS Family Plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-01-family-plugin-infrastructure.md` - infrastructure
4. `docs/plan/spec-02-flowspec-plugin.md` - reference for plugin pattern
5. `internal/plugin/bgp/nlri/bgpls.go` - current BGP-LS implementation

## Task

Move BGP-LS NLRI implementation from `internal/plugin/bgp/nlri/bgpls.go` to a standalone family plugin at `internal/plugin/bgpls/`.

**Prerequisites:** spec-01-family-plugin-infrastructure must be complete.

**Note:** BGP-LS is the most complex family with multiple NLRI types and extensive TLV structures.

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-01-family-plugin-infrastructure.md` - infrastructure
- [ ] `internal/plugin/bgp/nlri/bgpls.go` - current implementation
- [ ] `docs/architecture/wire/nlri-bgpls.md` - BGP-LS specific (if exists)
- [ ] `internal/plugin/hostname/hostname.go` - plugin pattern

### RFC Summaries
- [ ] `rfc/short/rfc7752.md` - BGP-LS base
- [ ] `rfc/short/rfc9085.md` - BGP-LS Extensions for Segment Routing
- [ ] `rfc/short/rfc9514.md` - BGP-LS Extensions for SRv6

**Key insights:**
- BGP-LS has 4 NLRI types (Node, Link, IPv4 Prefix, IPv6 Prefix)
- Complex TLV structure with nested sub-TLVs
- Protocol ID distinguishes IS-IS, OSPF, etc.
- Used for topology distribution to controllers

## BGP-LS NLRI Types

| Type | Name | Description |
|------|------|-------------|
| 1 | Node NLRI | Describes a router |
| 2 | Link NLRI | Describes a link between routers |
| 3 | IPv4 Topology Prefix NLRI | IPv4 reachability |
| 4 | IPv6 Topology Prefix NLRI | IPv6 reachability |

## BGP-LS Protocol IDs

| ID | Protocol |
|----|----------|
| 1 | IS-IS Level 1 |
| 2 | IS-IS Level 2 |
| 3 | OSPF |
| 4 | Direct |
| 5 | Static |
| 6 | OSPFv3 |
| 7 | BGP |

## BGP-LS Families

| Family | AFI | SAFI | Description |
|--------|-----|------|-------------|
| `bgp-ls/bgp-ls` | 16388 | 71 | BGP-LS |
| `bgp-ls/bgp-ls-vpn` | 16388 | 72 | BGP-LS VPN |

## Target State

### Plugin Structure

| File | Purpose |
|------|---------|
| `internal/plugin/bgpls/bgpls.go` | Plugin main, decode mode handler |
| `internal/plugin/bgpls/parse.go` | Wire bytes → BGP-LS struct |
| `internal/plugin/bgpls/encode.go` | BGP-LS struct → wire bytes |
| `internal/plugin/bgpls/json.go` | BGP-LS ↔ JSON conversion |
| `internal/plugin/bgpls/nlri.go` | NLRI type definitions |
| `internal/plugin/bgpls/tlv.go` | TLV parsing/encoding |
| `internal/plugin/bgpls/node.go` | Node NLRI handling |
| `internal/plugin/bgpls/link.go` | Link NLRI handling |
| `internal/plugin/bgpls/prefix.go` | Prefix NLRI handling |
| `internal/plugin/bgpls/bgpls_test.go` | Unit tests |
| `cmd/ze/bgp/plugin_bgpls.go` | CLI entry point |

### Plugin Registration

| Declaration | Purpose |
|-------------|---------|
| `declare family bgp-ls bgp-ls decode` | Claim BGP-LS decoding |
| `declare family bgp-ls bgp-ls-vpn decode` | Claim BGP-LS VPN decoding |
| `declare rfc 7752` | RFC reference |
| `declare rfc 9085` | RFC reference |
| `declare rfc 9514` | RFC reference |
| `declare encoding hex` | Wire encoding |

### JSON Format

| Field | Type | Description |
|-------|------|-------------|
| `family` | string | `"bgp-ls/bgp-ls"` |
| `nlri-type` | integer | 1-4 |
| `nlri-type-name` | string | Human readable |
| `protocol-id` | integer | 1-7 |
| `protocol-name` | string | Human readable |
| `identifier` | integer | Instance identifier |
| `local-node` | object | Local node descriptors |
| `remote-node` | object | Remote node descriptors (link NLRI) |
| `prefix` | object | Prefix descriptors (prefix NLRI) |
| `tlvs` | array | Additional TLVs |

## Files to Modify

- `internal/plugin/inprocess.go` - register bgpls in internalPluginRunners
- `internal/plugin/bgp/nlri/nlri.go` - remove BGP-LS family constants
- `cmd/ze/bgp/bgp.go` - add `plugin bgpls` subcommand

## Files to Create

- `internal/plugin/bgpls/bgpls.go` - plugin main
- `internal/plugin/bgpls/parse.go` - wire parsing
- `internal/plugin/bgpls/encode.go` - wire encoding
- `internal/plugin/bgpls/json.go` - JSON conversion
- `internal/plugin/bgpls/nlri.go` - NLRI types
- `internal/plugin/bgpls/tlv.go` - TLV handling
- `internal/plugin/bgpls/node.go` - Node NLRI
- `internal/plugin/bgpls/link.go` - Link NLRI
- `internal/plugin/bgpls/prefix.go` - Prefix NLRI
- `internal/plugin/bgpls/bgpls_test.go` - unit tests
- `cmd/ze/bgp/plugin_bgpls.go` - CLI entry

## Files to Delete

- `internal/plugin/bgp/nlri/bgpls.go` - moved to plugin
- `internal/plugin/bgp/nlri/bgpls_test.go` - moved to plugin

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Create plugin directory** - `internal/plugin/bgpls/`
   → **Review:** Structure allows for complexity?

2. **Extract TLV handling** - Create `tlv.go`
   → **Review:** Generic TLV parser works for all types?

3. **Extract Node NLRI** - Create `node.go`
   → **Review:** Node descriptors parsed correctly?

4. **Extract Link NLRI** - Create `link.go`
   → **Review:** Local and remote descriptors?

5. **Extract Prefix NLRI** - Create `prefix.go`
   → **Review:** Both IPv4 and IPv6?

6. **Create parse.go** - Wire parsing entry point
   → **Review:** Dispatches to correct NLRI type?

7. **Create encode.go** - Wire encoding
   → **Review:** TLV ordering correct?

8. **Create json.go** - JSON conversion
   → **Review:** Nested TLVs represented?

9. **Create bgpls.go** - Plugin main
   → **Review:** Stage 1 declarations?

10. **Move tests** - Adapt for new package
    → **Review:** All NLRI types tested?

11. **Write decode mode tests** - Test protocol
    → **Review:** Complex TLV cases?

12. **Run tests** - Verify PASS
    → **Review:** Coverage adequate?

13. **Create CLI entry** - `cmd/ze/bgp/plugin_bgpls.go`
    → **Review:** Matches pattern?

14. **Register in inprocess.go** - Add runner
    → **Review:** Logger configured?

15. **Delete original files** - Remove from nlri/
    → **Review:** No broken imports?

16. **Create functional test** - `test/data/decode/bgpls-plugin.ci`
    → **Review:** Tests all NLRI types?

17. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero errors?

18. **Final self-review**
    - All 4 NLRI types work
    - TLV handling comprehensive
    - Complex topologies handled

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPLSDecodeMode` | `internal/plugin/bgpls/bgpls_test.go` | Decode mode protocol | |
| `TestBGPLSJSONRoundTrip` | `internal/plugin/bgpls/bgpls_test.go` | JSON ↔ struct | |
| `TestBGPLSWireRoundTrip` | `internal/plugin/bgpls/bgpls_test.go` | wire ↔ struct | |
| `TestBGPLSNodeNLRI` | `internal/plugin/bgpls/bgpls_test.go` | Node NLRI type 1 | |
| `TestBGPLSLinkNLRI` | `internal/plugin/bgpls/bgpls_test.go` | Link NLRI type 2 | |
| `TestBGPLSIPv4PrefixNLRI` | `internal/plugin/bgpls/bgpls_test.go` | IPv4 prefix type 3 | |
| `TestBGPLSIPv6PrefixNLRI` | `internal/plugin/bgpls/bgpls_test.go` | IPv6 prefix type 4 | |
| `TestBGPLSTLVParsing` | `internal/plugin/bgpls/bgpls_test.go` | TLV structures | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI type | 1-4 | 4 | 0 | 5 |
| Protocol ID | 1-7 | 7 | 0 | 8 |
| TLV type | 0-65535 | 65535 | N/A | N/A (16-bit) |
| TLV length | 0-65535 | 65535 | N/A | N/A (16-bit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgpls-decode` | `test/data/decode/bgpls-plugin.ci` | Decode BGP-LS NLRI via CLI | |
| `bgpls-node` | `test/data/encode/bgpls-node.ci` | Node NLRI roundtrip | |
| `bgpls-link` | `test/data/encode/bgpls-link.ci` | Link NLRI roundtrip | |

## Design Decisions

### TLV JSON Representation

| Option | Pros | Cons |
|--------|------|------|
| Flat array | Simple | Loses hierarchy |
| Nested objects | Preserves structure | Complex |
| Type-keyed map | Easy lookup | Duplicate handling |

**Decision:** Nested objects to preserve TLV hierarchy and sub-TLVs.

### Unknown TLV Handling

| Option | Behavior |
|--------|----------|
| Fail | Reject unknown TLVs |
| Skip | Ignore unknown TLVs |
| Passthrough | Include as opaque hex |

**Decision:** Passthrough - include unknown TLVs as hex to enable future compatibility.

## RFC Documentation

### Reference Comments
- RFC 7752 Section 3.2 - NLRI format
- RFC 7752 Section 3.2.1 - Node NLRI
- RFC 7752 Section 3.2.2 - Link NLRI
- RFC 7752 Section 3.2.3 - Prefix NLRI
- RFC 9085 - SR extensions
- RFC 9514 - SRv6 extensions

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Design Insights
- [To be filled]

### Deviations from Plan
- [To be filled]

## Checklist

### 🏗️ Design
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
