# Spec: 04 - VPN Family Plugin (VPNv4/VPNv6)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-01-family-plugin-infrastructure.md` - infrastructure
4. `docs/plan/spec-02-flowspec-plugin.md` - reference for plugin pattern
5. `internal/plugin/bgp/nlri/ipvpn.go` - current VPN implementation

## Task

Move VPNv4/VPNv6 NLRI implementation from `internal/plugin/bgp/nlri/ipvpn.go` to a standalone family plugin at `internal/plugin/vpn/`.

**Prerequisites:** spec-01-family-plugin-infrastructure must be complete.

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-01-family-plugin-infrastructure.md` - infrastructure
- [ ] `internal/plugin/bgp/nlri/ipvpn.go` - current implementation
- [ ] `internal/plugin/hostname/hostname.go` - plugin pattern
- [ ] `docs/architecture/wire/nlri.md` - NLRI wire formats

### RFC Summaries
- [ ] `rfc/short/rfc4364.md` - BGP/MPLS IP VPNs
- [ ] `rfc/short/rfc4659.md` - BGP-MPLS IP VPN Extension for IPv6 VPN

**Key insights:**
- VPN NLRI = Route Distinguisher (8 bytes) + MPLS Label(s) + Prefix
- Route Distinguisher types: 0 (2+4), 1 (4+2), 2 (4+4)
- MPLS label stack (20-bit labels, 3 bytes each with S-bit)
- 6PE/6VPE uses IPv6 prefix with IPv4 next-hop

## VPN Families

| Family | AFI | SAFI | Description |
|--------|-----|------|-------------|
| `ipv4/mpls-vpn` | 1 | 128 | VPNv4 |
| `ipv6/mpls-vpn` | 2 | 128 | VPNv6 |

## Route Distinguisher Types

| Type | Format | Example |
|------|--------|---------|
| 0 | 2-byte ASN : 4-byte value | `65000:1000` |
| 1 | 4-byte IP : 2-byte value | `192.0.2.1:100` |
| 2 | 4-byte ASN : 2-byte value | `4200000001:100` |

## Target State

### Plugin Structure

| File | Purpose |
|------|---------|
| `internal/plugin/vpn/vpn.go` | Plugin main, decode mode handler |
| `internal/plugin/vpn/parse.go` | Wire bytes → VPN struct |
| `internal/plugin/vpn/encode.go` | VPN struct → wire bytes |
| `internal/plugin/vpn/json.go` | VPN ↔ JSON conversion |
| `internal/plugin/vpn/rd.go` | Route Distinguisher types |
| `internal/plugin/vpn/label.go` | MPLS label handling |
| `internal/plugin/vpn/vpn_test.go` | Unit tests |
| `cmd/ze/bgp/plugin_vpn.go` | CLI entry point |

### Plugin Registration

| Declaration | Purpose |
|-------------|---------|
| `declare family ipv4 mpls-vpn decode` | Claim VPNv4 decoding |
| `declare family ipv6 mpls-vpn decode` | Claim VPNv6 decoding |
| `declare rfc 4364` | RFC reference |
| `declare rfc 4659` | RFC reference |
| `declare encoding hex` | Wire encoding |

### JSON Format

| Field | Type | Description |
|-------|------|-------------|
| `family` | string | `"ipv4/mpls-vpn"` or `"ipv6/mpls-vpn"` |
| `rd` | string | Route Distinguisher (formatted) |
| `rd-type` | integer | 0, 1, or 2 |
| `labels` | array | MPLS label stack |
| `prefix` | string | IP prefix |

## Files to Modify

- `internal/plugin/inprocess.go` - register vpn in internalPluginRunners
- `internal/plugin/bgp/nlri/nlri.go` - remove VPN family constants
- `cmd/ze/bgp/bgp.go` - add `plugin vpn` subcommand

## Files to Create

- `internal/plugin/vpn/vpn.go` - plugin main
- `internal/plugin/vpn/parse.go` - wire parsing
- `internal/plugin/vpn/encode.go` - wire encoding
- `internal/plugin/vpn/json.go` - JSON conversion
- `internal/plugin/vpn/rd.go` - Route Distinguisher
- `internal/plugin/vpn/label.go` - MPLS labels
- `internal/plugin/vpn/vpn_test.go` - unit tests
- `cmd/ze/bgp/plugin_vpn.go` - CLI entry

## Files to Delete

- `internal/plugin/bgp/nlri/ipvpn.go` - moved to plugin
- `internal/plugin/bgp/nlri/ipvpn_test.go` - moved to plugin

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Create plugin directory** - `internal/plugin/vpn/`
   → **Review:** Directory structure matches pattern?

2. **Extract RD types** - Create `rd.go`
   → **Review:** All 3 RD types handled?

3. **Extract label handling** - Create `label.go`
   → **Review:** Label stack encoding correct? S-bit handled?

4. **Create parse.go** - Extract parsing functions
   → **Review:** Both VPNv4 and VPNv6?

5. **Create encode.go** - Extract encoding functions
   → **Review:** WriteTo methods work?

6. **Create json.go** - Add JSON marshal/unmarshal
   → **Review:** RD formatted correctly?

7. **Create vpn.go** - Plugin main with decode mode
   → **Review:** Both families declared?

8. **Move tests** - Adapt for new package
   → **Review:** VPNv4 and VPNv6 covered?

9. **Write decode mode tests** - Test decode nlri protocol
   → **Review:** All RD types tested?

10. **Run tests** - Verify PASS
    → **Review:** Coverage maintained?

11. **Create CLI entry** - `cmd/ze/bgp/plugin_vpn.go`
    → **Review:** Matches pattern?

12. **Register in inprocess.go** - Add to internalPluginRunners
    → **Review:** Logger configured?

13. **Delete original files** - Remove from nlri/
    → **Review:** No broken imports?

14. **Create functional test** - `test/data/decode/vpn-plugin.ci`
    → **Review:** Tests both families?

15. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero errors?

16. **Final self-review**
    - VPNv4 and VPNv6 both work
    - All RD types supported
    - Label stack handling correct

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPNDecodeMode` | `internal/plugin/vpn/vpn_test.go` | Decode mode protocol | |
| `TestVPNJSONRoundTrip` | `internal/plugin/vpn/vpn_test.go` | JSON ↔ struct | |
| `TestVPNWireRoundTrip` | `internal/plugin/vpn/vpn_test.go` | wire ↔ struct | |
| `TestVPNv4` | `internal/plugin/vpn/vpn_test.go` | VPNv4 specific | |
| `TestVPNv6` | `internal/plugin/vpn/vpn_test.go` | VPNv6 specific | |
| `TestRDAllTypes` | `internal/plugin/vpn/vpn_test.go` | All 3 RD types | |
| `TestLabelStack` | `internal/plugin/vpn/vpn_test.go` | Multi-label stacks | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RD type | 0-2 | 2 | N/A | 3 |
| MPLS label | 0-0xFFFFF | 0xFFFFF | N/A | 0x100000 |
| Prefix len IPv4 | 0-32 | 32 | N/A | 33 |
| Prefix len IPv6 | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpn-decode` | `test/data/decode/vpn-plugin.ci` | Decode VPN NLRI via CLI | |
| `vpnv4-roundtrip` | `test/data/encode/vpnv4-plugin.ci` | VPNv4 encode/decode | |
| `vpnv6-roundtrip` | `test/data/encode/vpnv6-plugin.ci` | VPNv6 encode/decode | |

## Design Decisions

### RD String Format

| Type | Format | Example |
|------|--------|---------|
| 0 | `<2-byte-asn>:<4-byte-value>` | `65000:1000` |
| 1 | `<ip>:<2-byte-value>` | `192.0.2.1:100` |
| 2 | `<4-byte-asn>:<2-byte-value>` | `4200000001:100` |

**Decision:** Use colon-separated format matching industry convention.

### Label Stack Representation

| Option | Format | Pros | Cons |
|--------|--------|------|------|
| Array | `[100, 200]` | Clear | Verbose |
| String | `100/200` | Compact | Parsing needed |

**Decision:** Array of integers for clarity and JSON compatibility.

## RFC Documentation

### Reference Comments
- RFC 4364 Section 4.1 - VPN-IPv4 address format
- RFC 4364 Section 4.2 - Route Distinguisher types
- RFC 4659 Section 3 - VPN-IPv6 address format
- RFC 3032 Section 2.1 - MPLS label stack encoding

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
