# Spec: API UPDATE Builder Bounds Safety

## Task
Ensure UpdateBuilder methods respect maxSize limits when generating UPDATEs from API/config input.

## Scope: Build Path Only

| Path | Use Case | Code | This Spec |
|------|----------|------|-----------|
| **Wire** | Forward received UPDATE to peer | `SplitUpdateWithAddPath` | ❌ NO (done: 093-writeto-bounds-safety.md) |
| **API** | Generate UPDATE from API input | `UpdateBuilder.Build*()` | ✅ YES |

**Call sites**: `pkg/reactor/peer.go` conversion functions, API command handlers.

## Problem
- `BuildGroupedUnicastWithLimit` already handles size limits for IPv4 unicast
- Other `Build*()` methods have no size limiting:
  - Single-route builders may produce oversized UPDATEs (attributes + NLRI > maxSize)
  - Grouped builders (`BuildGroupedUnicast`, `BuildMVPN`) may overflow
- Must respect negotiated Extended Message capability (4096 vs 65535)

## Current State Analysis

### Methods WITH Size Limiting
| Method | Limit Handling |
|--------|----------------|
| `BuildGroupedUnicastWithLimit` | ✅ Takes maxSize, returns `[]*Update` |

### Methods WITHOUT Size Limiting
| Method | Risk | Notes |
|--------|------|-------|
| `BuildUnicast` | Low | Single route, unlikely to exceed 4096 |
| `BuildVPN` | Low | Single VPN route |
| `BuildLabeledUnicast` | Low | Single labeled route |
| `BuildVPLS` | Low | Single VPLS route |
| `BuildFlowSpec` | **Medium** | FlowSpec rules can be large (up to 4095 bytes) |
| `BuildEVPN` | Low | Single EVPN route |
| `BuildMUP` | Low | Single MUP route |
| ~~`BuildGroupedUnicast`~~ | **Removed** | Replaced by `BuildGroupedUnicastWithLimit` |
| `BuildMVPN` | **Medium** | Takes `[]MVPNParams`, no limit |

## Design: Size-Aware API Path

### Approach 1: Add `WithLimit` Variants (Recommended)

Add `*WithLimit` variants for methods that need size control:

```go
// Grouped unicast already has this
func (ub *UpdateBuilder) BuildGroupedUnicastWithLimit(routes []UnicastParams, maxSize int) ([]*Update, error)

// Add for other grouped/risky methods
func (ub *UpdateBuilder) BuildFlowSpecWithLimit(p FlowSpecParams, maxSize int) ([]*Update, error)
func (ub *UpdateBuilder) BuildMVPNWithLimit(routes []MVPNParams, maxSize int) ([]*Update, error)
```

### Approach 2: Caller-Side Split (Alternative)

Caller builds single UPDATE, then splits if needed:

```go
update := ub.BuildFlowSpec(params)
updates, err := SplitUpdate(update, maxSize)
```

**Pros:** Simpler builder code
**Cons:** Less efficient (build then split vs build-with-limit)

### Decision: Hybrid

1. **Single-route builders**: Return error if UPDATE > maxSize (caller must reduce attributes)
2. **Grouped builders**: Add `*WithLimit` variants that split automatically
3. **FlowSpec**: Special case - single rule can be 4095 bytes, needs careful handling

## Required Reading

### Architecture Docs
- [ ] `.claude/zebgp/UPDATE_BUILDING.md` - Build path architecture
- [ ] `.claude/zebgp/ENCODING_CONTEXT.md` - Context-dependent encoding

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4271 Section 4.3 - UPDATE message format, max 4096 bytes
- [ ] RFC 8654 - Extended Message raises to 65535 bytes
- [ ] RFC 5575 - FlowSpec NLRI max 4095 bytes

**Key insights:**
- Build path is low-volume (config/API) vs high-volume wire forwarding
- Pre-splitting at build time avoids post-build splitting overhead
- FlowSpec is the only single-route type that could legitimately approach 4K

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestBuildFlowSpecWithLimit_Split` | `pkg/bgp/message/update_build_test.go` | FlowSpec splits when > maxSize |
| `TestBuildFlowSpecWithLimit_SingleTooLarge` | `pkg/bgp/message/update_build_test.go` | Error if single FlowSpec rule + attrs > maxSize |
| `TestBuildMVPNWithLimit_Split` | `pkg/bgp/message/update_build_test.go` | MVPN splits across multiple UPDATEs |
| `TestBuildGroupedUnicast_ErrorIfNoLimit` | `pkg/bgp/message/update_build_test.go` | Document that BuildGroupedUnicast has no limit |
| `TestBuildUnicast_ErrorIfTooLarge` | `pkg/bgp/message/update_build_test.go` | Single unicast route returns error if > maxSize |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | API bounds covered by unit tests; integration tests cover full API flow |

## Files to Modify
- `pkg/bgp/message/update_build.go` - Add `*WithLimit` methods
- `pkg/bgp/message/update_build_test.go` - Add bounds tests
- `pkg/reactor/peer.go` - Use `*WithLimit` methods with peer's maxUpdateSize

## Implementation Steps
1. **Write tests** - Test limit behavior for each builder type
2. **Run tests** - Verify FAIL (paste output)
3. **Add BuildFlowSpecWithLimit** - Handle FlowSpec splitting
4. **Add BuildMVPNWithLimit** - Handle MVPN batch splitting
5. **Add size checks to single-route builders** - Return error if oversized
6. **Run tests** - Verify PASS (paste output)
7. **Verify all** - `make lint && make test && make functional`
8. **RFC refs** - Add RFC reference comments
9. **RFC constraints** - Add constraint comments

## Design Decisions
- **Hybrid approach**: `*WithLimit` for grouped, error for oversized single
- **No silent truncation**: Error clearly if single route can't fit
- **Caller provides maxSize**: Builder doesn't know peer's Extended Message state
- **FlowSpec special case**: Single rule can be large, needs split capability

## RFC Documentation

### Reference Comments
```go
// RFC 4271 Section 4.3 - UPDATE max 4096 bytes (standard).
// RFC 8654 - Extended Message raises max to 65535 bytes.
// Caller must provide maxSize based on negotiated capabilities.
func (ub *UpdateBuilder) BuildFlowSpecWithLimit(p FlowSpecParams, maxSize int) ([]*Update, error)
```

### Constraint Comments
```go
// RFC 5575 Section 4: FlowSpec NLRI max 4095 bytes.
// Single FlowSpec rule CAN exceed standard 4096-byte message when
// combined with attributes. MUST split or return error.
```

## Open Questions

1. **Should single-route builders take maxSize?**
   - Option A: Add maxSize param, return error if exceeded
   - Option B: Assume caller knows route fits, panic/error on overflow at send time
   - Recommendation: Option A for safety

2. ~~**Deprecate BuildGroupedUnicast?**~~ **DONE**
   - Removed in 094-deprecated-code-removal.md

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added
- [ ] `.claude/zebgp/` updated if schema changed

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
