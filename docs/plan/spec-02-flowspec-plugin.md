# Spec: 02 - FlowSpec Family Plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-01-family-plugin-infrastructure.md` - prerequisite spec
4. `internal/plugin/bgp/nlri/flowspec.go` - current FlowSpec implementation
5. `internal/plugin/hostname/hostname.go` - reference plugin pattern

## Task

Move FlowSpec NLRI implementation from `internal/plugin/bgp/nlri/flowspec.go` to a standalone family plugin at `internal/plugin/flowspec/`. This is the first family plugin using the infrastructure from spec-01.

**Prerequisites:** spec-01-family-plugin-infrastructure must be complete.

**Key decisions from user:**
- MOVE code (not copy) - delete from original location
- NO per-message caching
- Plugin registers families with `decode` keyword

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-01-family-plugin-infrastructure.md` - infrastructure this builds on
- [ ] `internal/plugin/bgp/nlri/flowspec.go` - current implementation (2700+ lines)
- [ ] `internal/plugin/hostname/hostname.go` - plugin pattern to follow
- [ ] `docs/architecture/wire/nlri.md` - NLRI wire formats

### RFC Summaries
- [ ] `rfc/short/rfc8955.md` - FlowSpec IPv4
- [ ] `rfc/short/rfc8956.md` - FlowSpec IPv6

**Key insights:**
- FlowSpec has 13 component types (destination, source, protocol, ports, etc.)
- FlowSpec VPN adds Route Distinguisher prefix
- FlowSpec does NOT support ADD-PATH (RFC 8955)
- Current implementation is complete and well-tested

## Current Implementation Analysis

### Files to Move

| Source | Lines | Content |
|--------|-------|---------|
| `internal/plugin/bgp/nlri/flowspec.go` | ~2700 | All FlowSpec types and parsing |
| `internal/plugin/bgp/nlri/flowspec_test.go` | ~1080 | Comprehensive tests |

### FlowSpec Families

| Family | AFI | SAFI | Description |
|--------|-----|------|-------------|
| `ipv4/flowspec` | 1 | 133 | IPv4 FlowSpec |
| `ipv6/flowspec` | 2 | 133 | IPv6 FlowSpec |
| `ipv4/flowspec-vpn` | 1 | 134 | IPv4 FlowSpec VPN |
| `ipv6/flowspec-vpn` | 2 | 134 | IPv6 FlowSpec VPN |

### Component Types (RFC 8955 Section 4)

| Type | Name | Description |
|------|------|-------------|
| 1 | Destination Prefix | IPv4/IPv6 destination |
| 2 | Source Prefix | IPv4/IPv6 source |
| 3 | IP Protocol | Protocol number |
| 4 | Port | Any port (src or dst) |
| 5 | Destination Port | Destination port |
| 6 | Source Port | Source port |
| 7 | ICMP Type | ICMP type |
| 8 | ICMP Code | ICMP code |
| 9 | TCP Flags | TCP flags bitmask |
| 10 | Packet Length | Total packet length |
| 11 | DSCP | DiffServ code point |
| 12 | Fragment | Fragment flags |
| 13 | Flow Label | IPv6 flow label |

## Target State

### Plugin Structure

| File | Purpose |
|------|---------|
| `internal/plugin/flowspec/flowspec.go` | Plugin main, decode mode handler |
| `internal/plugin/flowspec/parse.go` | Wire bytes → FlowSpec struct |
| `internal/plugin/flowspec/encode.go` | FlowSpec struct → wire bytes |
| `internal/plugin/flowspec/json.go` | FlowSpec ↔ JSON conversion |
| `internal/plugin/flowspec/components.go` | Component type definitions |
| `internal/plugin/flowspec/flowspec_test.go` | Unit tests (moved + new) |
| `cmd/ze/bgp/plugin_flowspec.go` | CLI entry point |

### Plugin Registration

Stage 1 declarations:

| Declaration | Purpose |
|-------------|---------|
| `declare family ipv4 flowspec decode` | Claim IPv4 FlowSpec decoding |
| `declare family ipv6 flowspec decode` | Claim IPv6 FlowSpec decoding |
| `declare family ipv4 flowspec-vpn decode` | Claim IPv4 FlowSpec VPN decoding |
| `declare family ipv6 flowspec-vpn decode` | Claim IPv6 FlowSpec VPN decoding |
| `declare rfc 8955` | RFC reference |
| `declare rfc 8956` | RFC reference |
| `declare encoding hex` | Wire encoding |

### JSON Format

FlowSpec NLRI as JSON:

| Field | Type | Description |
|-------|------|-------------|
| `family` | string | e.g., `"ipv4/flowspec"` |
| `components` | array | List of component objects |

Component object:

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Component type name |
| `prefix` | string | For destination/source (e.g., `"10.0.0.0/24"`) |
| `values` | array | For numeric components |
| `operators` | array | Operator specifications |

Example JSON:

| Input (text) | JSON Output |
|--------------|-------------|
| `destination 10.0.0.0/24 protocol 6` | `{"family":"ipv4/flowspec","components":[{"type":"destination","prefix":"10.0.0.0/24"},{"type":"protocol","values":[6]}]}` |

### Decode Mode Protocol

| Direction | Format | Example |
|-----------|--------|---------|
| Request | `decode nlri <family> <hex>` | `decode nlri ipv4/flowspec 0701180a0000` |
| Response | `decoded json <json>` | `decoded json {"family":"ipv4/flowspec",...}` |

### CLI Integration

| Command | Purpose |
|---------|---------|
| `ze bgp plugin flowspec` | Run plugin (normal mode) |
| `ze bgp plugin flowspec --decode` | Run in decode mode |
| `ze bgp plugin flowspec --yang` | Output YANG schema |
| `ze bgp decode --plugin flowspec --nlri ipv4/flowspec <hex>` | Decode via CLI |

## Files to Modify

- `internal/plugin/inprocess.go` - register flowspec in internalPluginRunners
- `internal/plugin/bgp/nlri/nlri.go` - remove FlowSpec family constants (or keep as aliases)
- `internal/plugin/bgp/nlri/other.go` - remove FlowSpec SAFI if present
- `cmd/ze/bgp/bgp.go` - add `plugin flowspec` subcommand

## Files to Create

- `internal/plugin/flowspec/flowspec.go` - plugin main
- `internal/plugin/flowspec/parse.go` - wire parsing
- `internal/plugin/flowspec/encode.go` - wire encoding
- `internal/plugin/flowspec/json.go` - JSON conversion
- `internal/plugin/flowspec/components.go` - component types
- `internal/plugin/flowspec/flowspec_test.go` - unit tests
- `cmd/ze/bgp/plugin_flowspec.go` - CLI entry

## Files to Delete

- `internal/plugin/bgp/nlri/flowspec.go` - moved to plugin
- `internal/plugin/bgp/nlri/flowspec_test.go` - moved to plugin

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Create plugin directory** - `internal/plugin/flowspec/`
   → **Review:** Directory structure matches hostname plugin?

2. **Move flowspec.go** - Copy to `internal/plugin/flowspec/components.go`
   → **Review:** All types and constants moved?

3. **Create parse.go** - Extract parsing functions
   → **Review:** Public API clean? No circular deps?

4. **Create encode.go** - Extract encoding functions
   → **Review:** WriteTo methods work standalone?

5. **Create json.go** - Add JSON marshal/unmarshal
   → **Review:** JSON format documented? Round-trip works?

6. **Create flowspec.go** - Plugin main with decode mode
   → **Review:** Follows hostname pattern? Stage 1 declarations correct?

7. **Move tests** - Copy flowspec_test.go, adapt imports
   → **Review:** All tests still pass with new package?

8. **Write decode mode tests** - Test decode nlri protocol
   → **Review:** All 13 component types tested?

9. **Run tests** - Verify PASS
   → **Review:** Coverage maintained?

10. **Create CLI entry** - `cmd/ze/bgp/plugin_flowspec.go`
    → **Review:** Matches hostname pattern? Flags correct?

11. **Register in inprocess.go** - Add to internalPluginRunners
    → **Review:** Logger configured? Name correct?

12. **Delete original files** - Remove from nlri/
    → **Review:** No remaining references? No broken imports?

13. **Fix import errors** - Update any code that imported flowspec from nlri
    → **Review:** All imports updated? No cycles?

14. **Create functional test** - `test/data/decode/flowspec-plugin.ci`
    → **Review:** Tests CLI decode path?

15. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero errors? No regressions?

16. **Final self-review** - Before claiming done:
    - All FlowSpec functionality preserved
    - No code duplication
    - Tests comprehensive
    - JSON format documented

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFlowSpecDecodeMode` | `internal/plugin/flowspec/flowspec_test.go` | Decode mode protocol | |
| `TestFlowSpecJSONRoundTrip` | `internal/plugin/flowspec/flowspec_test.go` | JSON ↔ struct | |
| `TestFlowSpecWireRoundTrip` | `internal/plugin/flowspec/flowspec_test.go` | wire ↔ struct | |
| `TestFlowSpecAllComponents` | `internal/plugin/flowspec/flowspec_test.go` | All 13 component types | |
| `TestFlowSpecVPN` | `internal/plugin/flowspec/flowspec_test.go` | VPN variant with RD | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Component type | 1-13 | 13 | 0 | 14 |
| Prefix length IPv4 | 0-32 | 32 | N/A | 33 |
| Prefix length IPv6 | 0-128 | 128 | N/A | 129 |
| Port | 0-65535 | 65535 | N/A | N/A (16-bit) |
| DSCP | 0-63 | 63 | N/A | 64 |
| Protocol | 0-255 | 255 | N/A | N/A (8-bit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `flowspec-decode` | `test/data/decode/flowspec-plugin.ci` | Decode FlowSpec NLRI via CLI | |
| `flowspec-roundtrip` | `test/data/encode/flowspec-plugin.ci` | Encode then decode FlowSpec | |

## Design Decisions

### Why Move (Not Copy)?

| Option | Pros | Cons |
|--------|------|------|
| Copy then delete | Safe transition | Temporary duplication |
| Move directly | Clean, no duplication | Must fix all refs at once |

**Decision:** Move directly. FlowSpec is self-contained, no partial migration needed.

### JSON Format Design

| Option | Pros | Cons |
|--------|------|------|
| Flat key-value | Simple | Loses structure |
| Nested components | Preserves structure | More complex |
| Wire-like format | Close to RFC | Not human friendly |

**Decision:** Nested components. Matches RFC structure, human readable.

## RFC Documentation

### Reference Comments
- RFC 8955 Section 4 - FlowSpec NLRI encoding
- RFC 8955 Section 4.2 - Component ordering requirement
- RFC 8955 Section 4.2.1 - Numeric operator format
- RFC 8956 Section 3 - IPv6 FlowSpec extensions

### Constraint Comments
- Component ordering MUST be by type (RFC 8955 Section 4.2.2)
- ADD-PATH not supported for FlowSpec families
- IPv6 offset field only valid for type 1 and 2 components

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
