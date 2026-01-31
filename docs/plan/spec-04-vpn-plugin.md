# Spec: 04 - VPN Family Plugin (VPNv4/VPNv6)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/bgp/nlri/ipvpn.go` - current VPN implementation
4. `internal/plugin/flowspec/types.go` - reference for correct dependency pattern
5. `docs/plan/spec-03-evpn-plugin.md` - lessons learned from EVPN migration

## Task

Create a VPN family plugin at `internal/plugin/vpn/` to handle VPNv4/VPNv6 decoding.

**Current State:** No plugin exists yet. VPN types are in `internal/plugin/bgp/nlri/ipvpn.go`.

**Critical Note:** `RouteDistinguisher` and `RDType` are **shared types** used by EVPN, FlowSpec, and other families. These MUST stay in `nlri/ipvpn.go`. Only the `IPVPN` struct and its parsing should move to the plugin.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/bgp/nlri/ipvpn.go` - IPVPN struct, RouteDistinguisher, label parsing
- [ ] `internal/plugin/flowspec/types.go` - reference for correct dependency pattern

**Behavior to preserve:**
- `RouteDistinguisher` type and parsing (stays in nlri - shared)
- `RDType` constants (stays in nlri - shared)
- `ParseLabelStack`, `EncodeLabelStack` (stays in nlri - shared)
- `IPVPN` struct wire format and encoding
- Label stack handling with S-bit

**Behavior to change:**
- Move `IPVPN` struct to `internal/plugin/vpn/`
- Consumers use `vpn.IPVPN` instead of `nlri.IPVPN`

## The Shared Types Problem

### What STAYS in nlri (shared by multiple families)

| Type | Used By |
|------|---------|
| `RouteDistinguisher` | IPVPN, EVPN, FlowSpecVPN, MVPN, VPLS, MUP |
| `RDType` (0, 1, 2) | All RD-using families |
| `ParseRouteDistinguisher` | All RD-using families |
| `ParseLabelStack` | IPVPN, EVPN, Labeled Unicast |
| `EncodeLabelStack` | IPVPN, EVPN, Labeled Unicast |

### What MOVES to vpn plugin

| Type | Description |
|------|-------------|
| `IPVPN` struct | VPNv4/VPNv6 NLRI |
| `ParseIPVPN` | Wire parsing for VPN |
| `NewIPVPN` | Constructor |

### Dependency Direction (Like FlowSpec)

```
internal/plugin/bgp/nlri
    ├── RouteDistinguisher (SHARED - stays here)
    ├── ParseLabelStack (SHARED - stays here)
    └── does NOT import vpn

internal/plugin/vpn
    imports → nlri (for RouteDistinguisher, labels)
    exports → IPVPN, ParseIPVPN, NewIPVPN

Consumers (decode.go, encode.go, etc.):
    import → nlri (for RouteDistinguisher when needed)
    import → vpn (for IPVPN type)
```

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/plugin-modes.md` - **CRITICAL**: Plugin CLI/Engine mode interface spec
- [ ] `internal/plugin/bgp/nlri/ipvpn.go` - current implementation
- [ ] `internal/plugin/flowspec/types.go` - correct dependency pattern
- [ ] `docs/plan/spec-03-evpn-plugin.md` - import cycle lessons

### RFC Summaries
- [ ] `rfc/short/rfc4364.md` - BGP/MPLS IP VPNs
- [ ] `rfc/short/rfc4659.md` - BGP-MPLS IP VPN Extension for IPv6 VPN

**Key insights:**
- VPN NLRI = Route Distinguisher (8 bytes) + MPLS Label(s) + Prefix
- Route Distinguisher types: 0 (2+4), 1 (4+2), 2 (4+4)
- MPLS label stack (20-bit labels, 3 bytes each with S-bit)
- RouteDistinguisher is SHARED - do not move it

## Files to Modify

- `cmd/ze/bgp/encode.go` - change `nlri.IPVPN*` → `vpn.IPVPN*`, add vpn import
- `internal/plugin/update_text.go` - change `nlri.IPVPN*` → `vpn.IPVPN*`
- `internal/plugin/update_text_test.go` - change `nlri.IPVPN*` → `vpn.IPVPN*`
- `internal/plugin/text.go` - change `nlri.IPVPN*` → `vpn.IPVPN*`
- `internal/plugin/json_test.go` - change `nlri.IPVPN*` → `vpn.IPVPN*`
- `internal/plugin/bgp/reactor/reactor.go` - change `nlri.IPVPN*` → `vpn.IPVPN*`
- `internal/plugin/inprocess.go` - register vpn in internalPluginRunners
- `cmd/ze/bgp/bgp.go` - add `plugin vpn` subcommand

## Files to Create

- `internal/plugin/vpn/vpn.go` - plugin main with decode mode
- `internal/plugin/vpn/types.go` - IPVPN struct (imports nlri for RD/labels)
- `internal/plugin/vpn/vpn_test.go` - unit tests
- `cmd/ze/bgp/plugin_vpn.go` - CLI entry point

## Files to Delete

- None initially. After migration verified:
  - Remove `IPVPN` struct from `nlri/ipvpn.go` (keep RouteDistinguisher, labels)
  - Remove IPVPN tests from `nlri/ipvpn_test.go` (keep RD/label tests)

## VPN Families

| Family | AFI | SAFI | Description |
|--------|-----|------|-------------|
| `ipv4/vpn` | 1 | 128 | VPNv4 (RFC 4364) |
| `ipv6/vpn` | 2 | 128 | VPNv6 (RFC 4659) |

## CLI Mode Interface (per plugin-modes.md)

### Invocation

```bash
# CLI Mode - JSON output (default)
ze bgp plugin vpn --nlri 0001000100000001...

# CLI Mode - Text output
ze bgp plugin vpn --nlri 0001000100000001... --text

# CLI Mode - From stdin
echo "0001000100000001..." | ze bgp plugin vpn --nlri -

# CLI Mode - With family context (VPNv4 vs VPNv6)
ze bgp plugin vpn --nlri 0001000100000001... --family ipv4/vpn

# Query supported features
ze bgp plugin vpn --features

# Engine Decode Mode - Protocol commands on stdin
ze bgp plugin vpn --decode

# Engine Mode - Full protocol loop
ze bgp plugin vpn
```

### CLI Flags

| Flag | Type | Description |
|------|------|-------------|
| `--nlri <hex\|->` | string | Decode NLRI hex, output JSON (use `-` for stdin) |
| `--text` | bool | Output human-readable text instead of JSON |
| `--family <family>` | string | Address family context (`ipv4/vpn` or `ipv6/vpn`) |
| `--features` | bool | List supported decode features |
| `--decode` | bool | Engine decode protocol mode |
| `--log-level` | string | Log level (disabled, debug, info, warn, err) |
| `--yang` | bool | Output YANG schema and exit |

### Output Formats

**JSON (default):**
```json
[{"rd":"1:1","prefix":"10.0.0.0/24","labels":[[100]]}]
```

**Text (`--text`):**
```
VPNv4 rd=1:1 prefix=10.0.0.0/24 label=100
```

### Engine Decode Mode Protocol

```
# Engine calls plugin with --decode flag
ze bgp plugin vpn --decode
# Plugin reads from stdin: decode nlri ipv4/vpn 0001000100000001...
# Plugin responds: decoded json [{"rd":"1:1","prefix":"10.0.0.0/24","labels":[[100]]}]
```

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Create Plugin (New Code)

1. **Create `internal/plugin/vpn/types.go`**
   - Import `nlri` for `RouteDistinguisher`, `Family`, label functions
   - Define `IPVPN` struct (copy from nlri/ipvpn.go)
   - Define `ParseIPVPN` function
   - Define `NewIPVPN` constructor
   - Test: `go build ./internal/plugin/vpn/...`
   → **Review:** Compiles? Uses nlri types correctly?

2. **Create `internal/plugin/vpn/vpn.go`**
   - Plugin main with decode mode handler
   - Startup protocol: declare both families
   - Event loop for decode requests
   - Test: `go build ./internal/plugin/vpn/...`
   → **Review:** Declarations for both VPNv4 and VPNv6?

3. **Create `internal/plugin/vpn/vpn_test.go`**
   - Wire roundtrip tests for VPNv4 and VPNv6
   - All 3 RD types tested
   - Label stack tests
   - Test: `go test ./internal/plugin/vpn/... -v`
   → **Review:** Tests pass?

4. **Create `cmd/ze/bgp/plugin_vpn.go`** (per `plugin-modes.md`)
   - CLI entry point with three-mode support:
     - `--nlri <hex|->` - decode NLRI hex (use `-` for stdin)
     - `--text` - output text instead of JSON
     - `--features` - list supported features (`nlri yang`)
     - `--decode` - engine decode protocol mode
     - `--family <family>` - address family context
     - `--log-level` - logger configuration
     - `--yang` - output YANG schema
   - Mode detection: `--nlri` → CLI mode, `--decode` → engine decode, else → engine mode
   - Test: `go build ./cmd/ze/bgp/...`
   → **Review:** Follows plugin-modes.md pattern? All three modes work?

5. **Register in `internal/plugin/inprocess.go`**
   - Add vpn to internalPluginRunners
   - Add to familyToPlugin map for ipv4/vpn and ipv6/vpn
   - Test: `go build ./internal/plugin/...`
   → **Review:** Registered correctly?

### Phase 2: Update Consumers

6. **Update `cmd/ze/bgp/encode.go`**
   - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/vpn"`
   - Replace `nlri.IPVPN` with `vpn.IPVPN`
   - Replace `nlri.NewIPVPN` with `vpn.NewIPVPN`
   - Test: `go build ./cmd/ze/bgp/...`
   → **Review:** Compiles?

7. **Update `internal/plugin/update_text.go`**
   - Add vpn import
   - Replace IPVPN references
   - Test: `go build ./internal/plugin/...`
   → **Review:** Compiles?

8. **Update `internal/plugin/update_text_test.go`**
   - Add vpn import
   - Replace IPVPN references
   - Test: `go test ./internal/plugin/... -run VPN`
   → **Review:** Tests pass?

9. **Update `internal/plugin/text.go`**
   - Add vpn import
   - Replace IPVPN references
   - Test: `go build ./internal/plugin/...`
   → **Review:** Compiles?

10. **Update `internal/plugin/json_test.go`**
    - Add vpn import
    - Replace IPVPN references
    - Test: `go test ./internal/plugin/... -run VPN`
    → **Review:** Tests pass?

11. **Update `internal/plugin/bgp/reactor/reactor.go`**
    - Add vpn import
    - Replace IPVPN references
    - Test: `go build ./internal/plugin/bgp/reactor/...`
    → **Review:** Compiles?

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

16. **Remove IPVPN from nlri/ipvpn.go**
    - Keep RouteDistinguisher, RDType, label functions
    - Remove IPVPN struct, ParseIPVPN, NewIPVPN
    - Test: `go build ./...`
    → **Review:** No broken imports? RouteDistinguisher still works?

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPNDecodeMode` | `internal/plugin/vpn/vpn_test.go` | Decode mode protocol | |
| `TestVPNv4WireRoundTrip` | `internal/plugin/vpn/vpn_test.go` | VPNv4 wire format | |
| `TestVPNv6WireRoundTrip` | `internal/plugin/vpn/vpn_test.go` | VPNv6 wire format | |
| `TestVPNJSONRoundTrip` | `internal/plugin/vpn/vpn_test.go` | JSON ↔ struct | |
| `TestVPNAllRDTypes` | `internal/plugin/vpn/vpn_test.go` | RD types 0, 1, 2 | |
| `TestVPNLabelStack` | `internal/plugin/vpn/vpn_test.go` | Multi-label handling | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RD type | 0-2 | 2 | N/A (0 valid) | 3 (unknown) |
| MPLS label | 0-0xFFFFF | 0xFFFFF (20-bit max) | N/A | 0x100000 |
| Prefix len IPv4 | 0-32 | 32 | N/A | 33 |
| Prefix len IPv6 | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpn-decode` | `test/decode/vpn-*.ci` | Decode VPN NLRI via CLI | |

## Design Decisions

### Why Keep RouteDistinguisher in nlri?

`RouteDistinguisher` is used by 6+ NLRI families. Moving it to the vpn plugin would require all other families to import vpn, creating unnecessary coupling.

**Decision:** Keep shared types in nlri. Plugin-specific types move to plugin.

### Plugin Registration

```
declare family ipv4 vpn decode
declare family ipv6 vpn decode
declare rfc 4364
declare rfc 4659
declare encoding hex
declare done
```

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
- [ ] No premature abstraction (following existing flowspec pattern)
- [ ] No speculative features (only decode mode initially)
- [ ] Single responsibility (vpn package owns IPVPN type)
- [ ] Explicit behavior (direct imports, no re-export magic)
- [ ] Minimal coupling (shared types stay in nlri)
- [ ] Next-developer test (follows existing flowspec pattern)

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
