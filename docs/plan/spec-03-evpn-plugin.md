# Spec: 03 - EVPN Family Plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-01-family-plugin-infrastructure.md` - infrastructure
4. `docs/plan/spec-02-flowspec-plugin.md` - reference for plugin pattern
5. `internal/plugin/bgp/nlri/evpn.go` - current EVPN implementation

## Task

Move EVPN NLRI implementation from `internal/plugin/bgp/nlri/evpn.go` to a standalone family plugin at `internal/plugin/evpn/`.

**Prerequisites:** spec-01-family-plugin-infrastructure must be complete.

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-01-family-plugin-infrastructure.md` - infrastructure
- [ ] `internal/plugin/bgp/nlri/evpn.go` - current implementation
- [ ] `internal/plugin/hostname/hostname.go` - plugin pattern
- [ ] `docs/architecture/wire/nlri.md` - NLRI wire formats
- [ ] `docs/architecture/wire/nlri-evpn.md` - EVPN specific (if exists)

### RFC Summaries
- [ ] `rfc/short/rfc7432.md` - EVPN base
- [ ] `rfc/short/rfc9136.md` - EVPN updates

**Key insights:**
- EVPN has 5 route types (MAC/IP, IMET, Ethernet Segment, etc.)
- Each route type has different NLRI format
- EVPN uses Route Distinguisher (8 bytes)
- ESI (Ethernet Segment Identifier) is 10 bytes

## EVPN Route Types

| Type | Name | RFC Section |
|------|------|-------------|
| 1 | Ethernet Auto-Discovery | RFC 7432 Section 7.1 |
| 2 | MAC/IP Advertisement | RFC 7432 Section 7.2 |
| 3 | Inclusive Multicast Ethernet Tag | RFC 7432 Section 7.3 |
| 4 | Ethernet Segment | RFC 7432 Section 7.4 |
| 5 | IP Prefix | RFC 9136 |

## EVPN Families

| Family | AFI | SAFI | Description |
|--------|-----|------|-------------|
| `l2vpn/evpn` | 25 | 70 | L2VPN EVPN |

## Target State

### Plugin Structure

| File | Purpose |
|------|---------|
| `internal/plugin/evpn/evpn.go` | Plugin main, decode mode handler |
| `internal/plugin/evpn/parse.go` | Wire bytes → EVPN struct |
| `internal/plugin/evpn/encode.go` | EVPN struct → wire bytes |
| `internal/plugin/evpn/json.go` | EVPN ↔ JSON conversion |
| `internal/plugin/evpn/routes.go` | Route type definitions |
| `internal/plugin/evpn/evpn_test.go` | Unit tests |
| `cmd/ze/bgp/plugin_evpn.go` | CLI entry point |

### Plugin Registration

| Declaration | Purpose |
|-------------|---------|
| `declare family l2vpn evpn decode` | Claim EVPN decoding |
| `declare rfc 7432` | RFC reference |
| `declare rfc 9136` | RFC reference |
| `declare encoding hex` | Wire encoding |

### JSON Format

| Field | Type | Description |
|-------|------|-------------|
| `family` | string | `"l2vpn/evpn"` |
| `route-type` | integer | 1-5 |
| `route-type-name` | string | Human readable name |
| `rd` | string | Route Distinguisher |
| `esi` | string | Ethernet Segment ID (hex) |
| `ethernet-tag` | integer | Ethernet Tag ID |
| `mac` | string | MAC address |
| `ip` | string | IP address (optional) |
| `label` | integer | MPLS label |

## Files to Modify

- `internal/plugin/inprocess.go` - register evpn in internalPluginRunners
- `internal/plugin/bgp/nlri/nlri.go` - remove EVPN family constants
- `cmd/ze/bgp/bgp.go` - add `plugin evpn` subcommand

## Files to Create

- `internal/plugin/evpn/evpn.go` - plugin main
- `internal/plugin/evpn/parse.go` - wire parsing
- `internal/plugin/evpn/encode.go` - wire encoding
- `internal/plugin/evpn/json.go` - JSON conversion
- `internal/plugin/evpn/routes.go` - route types
- `internal/plugin/evpn/evpn_test.go` - unit tests
- `cmd/ze/bgp/plugin_evpn.go` - CLI entry

## Files to Delete

- `internal/plugin/bgp/nlri/evpn.go` - moved to plugin
- `internal/plugin/bgp/nlri/evpn_test.go` - moved to plugin

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Create plugin directory** - `internal/plugin/evpn/`
   → **Review:** Directory structure matches flowspec plugin?

2. **Move route types** - Extract to `routes.go`
   → **Review:** All 5 route types defined?

3. **Create parse.go** - Extract parsing functions
   → **Review:** Each route type has parser?

4. **Create encode.go** - Extract encoding functions
   → **Review:** WriteTo methods work?

5. **Create json.go** - Add JSON marshal/unmarshal
   → **Review:** All route type fields mapped?

6. **Create evpn.go** - Plugin main with decode mode
   → **Review:** Stage 1 declarations correct?

7. **Move tests** - Adapt for new package
   → **Review:** All route types tested?

8. **Write decode mode tests** - Test decode nlri protocol
   → **Review:** Round-trip for all route types?

9. **Run tests** - Verify PASS
   → **Review:** Coverage maintained?

10. **Create CLI entry** - `cmd/ze/bgp/plugin_evpn.go`
    → **Review:** Matches flowspec pattern?

11. **Register in inprocess.go** - Add to internalPluginRunners
    → **Review:** Logger configured?

12. **Delete original files** - Remove from nlri/
    → **Review:** No broken imports?

13. **Fix import errors** - Update references
    → **Review:** All imports fixed?

14. **Create functional test** - `test/data/decode/evpn-plugin.ci`
    → **Review:** Tests CLI decode?

15. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero errors?

16. **Final self-review**
    - All route types preserved
    - No code duplication
    - Tests comprehensive

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEVPNDecodeMode` | `internal/plugin/evpn/evpn_test.go` | Decode mode protocol | |
| `TestEVPNJSONRoundTrip` | `internal/plugin/evpn/evpn_test.go` | JSON ↔ struct | |
| `TestEVPNWireRoundTrip` | `internal/plugin/evpn/evpn_test.go` | wire ↔ struct | |
| `TestEVPNAllRouteTypes` | `internal/plugin/evpn/evpn_test.go` | All 5 route types | |
| `TestEVPNType2MACIP` | `internal/plugin/evpn/evpn_test.go` | MAC/IP with variations | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Route type | 1-5 | 5 | 0 | 6 |
| Ethernet tag | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A (32-bit) |
| MPLS label | 0-0xFFFFF | 0xFFFFF | N/A | 0x100000 |
| ESI length | 10 bytes | 10 | 9 | 11 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `evpn-decode` | `test/data/decode/evpn-plugin.ci` | Decode EVPN NLRI via CLI | |
| `evpn-type2` | `test/data/encode/evpn-type2.ci` | MAC/IP route encode/decode | |

## Design Decisions

### JSON Field Naming

| Option | Example | Pros | Cons |
|--------|---------|------|------|
| RFC names | `ethernet-tag-id` | Matches RFC | Verbose |
| Short names | `etag` | Compact | Less clear |
| CamelCase | `ethernetTagId` | Go convention | Not JSON convention |

**Decision:** Hyphenated RFC names for clarity.

## RFC Documentation

### Reference Comments
- RFC 7432 Section 7 - NLRI format
- RFC 7432 Section 7.1 - Route type 1
- RFC 7432 Section 7.2 - Route type 2
- RFC 9136 - IP Prefix route type 5

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
