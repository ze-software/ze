# Spec: API UPDATE Builder Bounds Safety

## Task

Ensure UpdateBuilder methods respect maxSize limits when generating UPDATEs from API/config input.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/UPDATE_BUILDING.md` - Build vs Forward paths, size-aware building
- [ ] `docs/architecture/ENCODING_CONTEXT.md` - Context-dependent encoding, ASN4/ADD-PATH

### RFC Summaries (MUST for protocol work)
- [ ] `docs/rfc/rfc4271.md` - UPDATE message format, max 4096 bytes
- [ ] `docs/rfc/rfc8654.md` - Extended Message capability raises max to 65535 bytes
- [ ] `docs/rfc/rfc5575.md` - FlowSpec NLRI format, max 4095 bytes per rule

**Key insights:**
- Build path is low-volume (config/API) vs high-volume wire forwarding
- Single-route builders return error if too large (cannot split atomic route)
- Multi-route builders (`*WithLimit`) split across multiple UPDATEs
- FlowSpec single rule is atomic - if too large, must error (cannot split)

## Scope: Build Path Only

| Path | Use Case | Code | This Spec |
|------|----------|------|-----------|
| **Wire** | Forward received UPDATE to peer | `SplitUpdateWithAddPath` | ❌ NO (done: 093-writeto-bounds-safety.md) |
| **API** | Generate UPDATE from API input | `UpdateBuilder.Build*()` | ✅ YES |

**Call sites**: `internal/reactor/peer.go` conversion functions, API command handlers.

## Problem

- `BuildGroupedUnicastWithLimit` already handles size limits for IPv4/IPv6 unicast
- Other `Build*()` methods have no size limiting:
  - Single-route builders may produce oversized UPDATEs (attributes + NLRI > maxSize)
  - Multi-route builders (`BuildMVPN`) may overflow
- Must respect negotiated Extended Message capability (4096 vs 65535)

## Current State Analysis

### Methods WITH Size Limiting
| Method | Limit Handling |
|--------|----------------|
| `BuildGroupedUnicastWithLimit` | ✅ Takes maxSize, returns `[]*Update` |

### Methods WITHOUT Size Limiting

**Single-route builders (atomic - cannot split):**
| Method | Risk | Notes |
|--------|------|-------|
| `BuildUnicast` | Low | Single route, unlikely to exceed 4096 |
| `BuildVPN` | Low | Single VPN route |
| `BuildLabeledUnicast` | Low | Single labeled route |
| `BuildVPLS` | Low | Single VPLS route |
| `BuildFlowSpec` | **Medium** | Single rule up to 4095 bytes + attrs |
| `BuildEVPN` | Low | Single EVPN route |
| `BuildMUP` | Low | Single MUP route |

**Multi-route builders (can split):**
| Method | Risk | Notes |
|--------|------|-------|
| `BuildMVPN` | **Medium** | Takes `[]MVPNParams`, no limit |

## Design: Size-Aware API Path

### Approach: Error on Overflow (Single) + Split (Multi)

**Single-route builders:** Add `*WithMaxSize` variant, return error if exceeded.
A single route is atomic and cannot be split - if it doesn't fit, caller must reduce attributes or use Extended Message.

```go
// Single-route: error if too large (cannot split atomic route)
func (ub *UpdateBuilder) BuildUnicastWithMaxSize(p UnicastParams, maxSize int) (*Update, error)
func (ub *UpdateBuilder) BuildFlowSpecWithMaxSize(p FlowSpecParams, maxSize int) (*Update, error)
func (ub *UpdateBuilder) BuildVPNWithMaxSize(p VPNParams, maxSize int) (*Update, error)
func (ub *UpdateBuilder) BuildLabeledUnicastWithMaxSize(p LabeledUnicastParams, maxSize int) (*Update, error)
func (ub *UpdateBuilder) BuildVPLSWithMaxSize(p VPLSParams, maxSize int) (*Update, error)
func (ub *UpdateBuilder) BuildEVPNWithMaxSize(p EVPNParams, maxSize int) (*Update, error)
func (ub *UpdateBuilder) BuildMUPWithMaxSize(p MUPParams, maxSize int) (*Update, error)
```

**Multi-route builders:** Add `*WithLimit` variants that split automatically.

```go
// Multi-route: split across UPDATEs if needed
func (ub *UpdateBuilder) BuildGroupedUnicastWithLimit(routes []UnicastParams, maxSize int) ([]*Update, error)  // EXISTS
func (ub *UpdateBuilder) BuildMVPNWithLimit(routes []MVPNParams, maxSize int) ([]*Update, error)
```

### Design Decisions

1. **Single-route = error on overflow**: Cannot split atomic route
2. **Multi-route = split**: `*WithLimit` variants return `[]*Update`
3. **No silent truncation**: Error clearly if route can't fit
4. **Caller provides maxSize**: Builder doesn't know peer's Extended Message state
5. **FlowSpec**: Single rule is atomic - error if rule + attrs > maxSize

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestBuildFlowSpec_MaxSize_Fits` | `internal/bgp/message/update_build_test.go` | FlowSpec within limit succeeds |
| `TestBuildFlowSpec_MaxSize_TooLarge` | `internal/bgp/message/update_build_test.go` | Error if FlowSpec rule + attrs > maxSize |
| `TestBuildMVPNWithLimit_Split` | `internal/bgp/message/update_build_test.go` | MVPN splits across multiple UPDATEs |
| `TestBuildMVPNWithLimit_AllFit` | `internal/bgp/message/update_build_test.go` | MVPN returns single UPDATE when all fit |
| `TestBuildUnicast_MaxSize_Fits` | `internal/bgp/message/update_build_test.go` | Unicast within limit succeeds |
| `TestBuildUnicast_MaxSize_TooLarge` | `internal/bgp/message/update_build_test.go` | Error if unicast + attrs > maxSize |
| `TestBuildVPN_MaxSize_Fits` | `internal/bgp/message/update_build_test.go` | VPN within limit succeeds |
| `TestBuildVPN_MaxSize_TooLarge` | `internal/bgp/message/update_build_test.go` | Error if VPN + attrs > maxSize |
| `TestBuildLabeledUnicast_MaxSize_Fits` | `internal/bgp/message/update_build_test.go` | Labeled unicast within limit succeeds |
| `TestBuildLabeledUnicast_MaxSize_TooLarge` | `internal/bgp/message/update_build_test.go` | Error if labeled unicast > maxSize |
| `TestBuildVPLS_MaxSize_Fits` | `internal/bgp/message/update_build_test.go` | VPLS within limit succeeds |
| `TestBuildVPLS_MaxSize_TooLarge` | `internal/bgp/message/update_build_test.go` | Error if VPLS + attrs > maxSize |
| `TestBuildEVPN_MaxSize_Fits` | `internal/bgp/message/update_build_test.go` | EVPN within limit succeeds |
| `TestBuildEVPN_MaxSize_TooLarge` | `internal/bgp/message/update_build_test.go` | Error if EVPN + attrs > maxSize |
| `TestBuildMUP_MaxSize_Fits` | `internal/bgp/message/update_build_test.go` | MUP within limit succeeds |
| `TestBuildMUP_MaxSize_TooLarge` | `internal/bgp/message/update_build_test.go` | Error if MUP + attrs > maxSize |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | API bounds covered by unit tests; integration tests cover full API flow |

## Files to Modify
- `internal/bgp/message/update_build.go` - Add maxSize to single-route builders, add `BuildMVPNWithLimit`
- `internal/bgp/message/update_build_test.go` - Add bounds tests
- `internal/reactor/peer.go` - Pass peer's maxUpdateSize to builders

## Implementation Steps
1. **Write tests** - Test limit behavior for each builder type
2. **Run tests** - Verify FAIL (paste output)
3. **Add maxSize to BuildFlowSpec** - Return error if too large
4. **Add BuildMVPNWithLimit** - Handle MVPN batch splitting
5. **Add maxSize to other single-route builders** - Return error if oversized
6. **Run tests** - Verify PASS (paste output)
7. **Verify all** - `make lint && make test && make functional`
8. **RFC refs** - Add RFC reference comments
9. **RFC constraints** - Add constraint comments

## RFC Documentation

### Reference Comments
```go
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
// Caller must provide maxSize based on negotiated capabilities.
// Returns error if single route + attributes exceeds maxSize.
func (ub *UpdateBuilder) BuildFlowSpec(p FlowSpecParams, maxSize int) (*Update, error)
```

### Constraint Comments
```go
// RFC 5575 Section 4: FlowSpec NLRI max 4095 bytes.
// Single FlowSpec rule is atomic - cannot be split across UPDATEs.
// If rule + attributes > maxSize, MUST return error.
if updateSize > maxSize {
    return nil, ErrUpdateTooLarge
}
```

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)

#### Tests FAIL output:
```
internal/bgp/message/update_build_test.go:1642:20: ub.BuildFlowSpecWithMaxSize undefined
internal/bgp/message/update_build_test.go:1671:12: undefined: ErrUpdateTooLarge
internal/bgp/message/update_build_test.go:1706:21: ub.BuildMVPNWithLimit undefined
internal/bgp/message/update_build_test.go:1770:15: ub.BuildUnicastWithMaxSize undefined
FAIL	codeberg.org/thomas-mangin/ze/internal/bgp/message [build failed]
```

#### Tests PASS output:
```
=== RUN   TestBuildFlowSpec_MaxSize_Fits
--- PASS: TestBuildFlowSpec_MaxSize_Fits (0.00s)
=== RUN   TestBuildFlowSpec_MaxSize_TooLarge
--- PASS: TestBuildFlowSpec_MaxSize_TooLarge (0.00s)
=== RUN   TestBuildMVPNWithLimit_AllFit
--- PASS: TestBuildMVPNWithLimit_AllFit (0.00s)
=== RUN   TestBuildMVPNWithLimit_Split
--- PASS: TestBuildMVPNWithLimit_Split (0.00s)
=== RUN   TestBuildUnicast_MaxSize_TooLarge
--- PASS: TestBuildUnicast_MaxSize_TooLarge (0.00s)
=== RUN   TestBuildUnicast_MaxSize_Fits
--- PASS: TestBuildUnicast_MaxSize_Fits (0.00s)
=== RUN   TestBuildVPN_MaxSize_Fits
--- PASS: TestBuildVPN_MaxSize_Fits (0.00s)
=== RUN   TestBuildVPN_MaxSize_TooLarge
--- PASS: TestBuildVPN_MaxSize_TooLarge (0.00s)
=== RUN   TestBuildLabeledUnicast_MaxSize_Fits
--- PASS: TestBuildLabeledUnicast_MaxSize_Fits (0.00s)
=== RUN   TestBuildLabeledUnicast_MaxSize_TooLarge
--- PASS: TestBuildLabeledUnicast_MaxSize_TooLarge (0.00s)
=== RUN   TestBuildVPLS_MaxSize_Fits
--- PASS: TestBuildVPLS_MaxSize_Fits (0.00s)
=== RUN   TestBuildVPLS_MaxSize_TooLarge
--- PASS: TestBuildVPLS_MaxSize_TooLarge (0.00s)
=== RUN   TestBuildEVPN_MaxSize_Fits
--- PASS: TestBuildEVPN_MaxSize_Fits (0.00s)
=== RUN   TestBuildEVPN_MaxSize_TooLarge
--- PASS: TestBuildEVPN_MaxSize_TooLarge (0.00s)
=== RUN   TestBuildMUP_MaxSize_Fits
--- PASS: TestBuildMUP_MaxSize_Fits (0.00s)
=== RUN   TestBuildMUP_MaxSize_TooLarge
--- PASS: TestBuildMUP_MaxSize_TooLarge (0.00s)
PASS
```

### Verification
- [x] `make lint` passes (pre-existing issues unrelated to this change)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC summaries read (all referenced RFCs)
- [x] RFC references added to code
- [x] RFC constraint comments added
- [ ] `docs/` updated if schema changed (N/A - no schema changes)

### Completion
- [x] Spec moved to `docs/plan/done/097-api-bounds-safety.md`
