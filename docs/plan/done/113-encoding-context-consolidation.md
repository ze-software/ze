# Spec: encoding-context-consolidation

## Task

Consolidate WireContext and EncodingContext into a single type named `EncodingContext`.
Keep the WireContext implementation (references sub-components), use the EncodingContext name.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/encoding-context.md` - Current design, WireContext section
- [ ] `docs/architecture/core-design.md` - How contexts are used

### RFC Summaries
- [ ] `docs/rfc/rfc7911.md` - ADD-PATH (direction-dependent)

**Key insights:**
- WireContext references `*PeerIdentity` + `*EncodingCaps` (zero copy)
- EncodingContext copies data into flat struct
- Both have equivalent methods, different access patterns
- Direction is explicit in WireContext, implicit in old EncodingContext

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing `TestEncodingContext*` | `pkg/bgp/context/context_test.go` | Update to new API | |
| Existing `TestFromNegotiated*` | `pkg/bgp/context/negotiated_test.go` | Update to new API | |
| Existing `TestWireContext*` | `pkg/bgp/context/wire_test.go` | Rename to EncodingContext | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| All existing | `test/data/` | Verify no regression | |

## Files to Modify

### Phase 1: Rename WireContext → EncodingContext
- `pkg/bgp/context/wire.go` → `pkg/bgp/context/context.go` (merge/replace)
  - Rename `WireContext` → `EncodingContext`
  - Rename `NewWireContext` → `NewEncodingContext`
  - Keep `Direction` type and constants

### Phase 2: Update factories
- `pkg/bgp/context/negotiated.go`
  - Remove `FromNegotiatedRecv/Send` (old API)
  - Rename `FromNegotiatedRecvWire` → `FromNegotiatedRecv`
  - Rename `FromNegotiatedSendWire` → `FromNegotiatedSend`

### Phase 3: Update consumers (field → method)
- `pkg/reactor/peer.go` - `ctx.ASN4` → `ctx.ASN4()`
- `pkg/reactor/reactor.go` - Same pattern
- `pkg/rib/commit.go` - Same pattern
- `pkg/rib/route.go` - Same pattern
- `pkg/plugin/wire_update_split.go` - Same pattern
- `pkg/plugin/filter.go` - Same pattern
- `pkg/plugin/text.go` - Same pattern
- `pkg/plugin/json.go` - Same pattern
- `pkg/plugin/decode.go` - Same pattern

### Phase 4: Update tests
- `pkg/bgp/context/wire_test.go` → merge into `context_test.go`
- `pkg/bgp/context/negotiated_test.go` - Update API calls
- `pkg/reactor/peer_test.go` - Update API calls
- Other test files using EncodingContext

### Phase 5: Cleanup
- Remove duplicate hash functions (`hashFamilyBoolMap` vs `hashFamilyBoolMapWire`)
- Remove old `context.go` content (replaced by wire.go content)
- Update `docs/architecture/encoding-context.md`

## Files to Delete
- None (merging, not deleting)

## API Changes

### Before → After
```go
// Type
*EncodingContext (flat)     → *EncodingContext (refs sub-components)

// Creation
FromNegotiatedRecv(neg)     → FromNegotiatedRecv(neg)  // same name, new impl
FromNegotiatedSend(neg)     → FromNegotiatedSend(neg)  // same name, new impl

// Field access → Method calls
ctx.ASN4                    → ctx.ASN4()
ctx.IsIBGP                  → ctx.IsIBGP()
ctx.LocalAS                 → ctx.LocalASN()
ctx.PeerAS                  → ctx.PeerASN()
ctx.AddPath[f]              → ctx.AddPath(f)
ctx.ExtendedNextHop[f]      → ctx.ExtendedNextHopFor(f)

// Already methods (unchanged)
ctx.AddPathFor(f)           → ctx.AddPath(f)  // slight rename
ctx.ExtendedNextHopFor(f)   → ctx.ExtendedNextHopFor(f)
ctx.ToPackContext(f)        → ctx.ToPackContext(f)
ctx.Hash()                  → ctx.Hash()

// New (from WireContext)
(none)                      → ctx.Direction()
(none)                      → ctx.Identity()
(none)                      → ctx.Encoding()
(none)                      → ctx.Families()
```

### Inline creation pattern
```go
// Before (some places create inline)
ctx := &EncodingContext{ASN4: true}

// After (use constructor or nil-safe methods)
ctx := NewEncodingContext(nil, &capability.EncodingCaps{ASN4: true}, DirectionSend)
// OR for simple cases, methods handle nil gracefully
```

## Implementation Steps

1. **Write migration tests** - Tests that verify both old and new API work
2. **Run tests** - Verify FAIL for new API (doesn't exist yet)
3. **Rename WireContext → EncodingContext** in wire.go
4. **Update negotiated.go factories** - Rename Wire variants, remove old
5. **Update context.go** - Replace old struct with renamed wire.go content
6. **Merge hash functions** - Keep one set
7. **Update consumers** - Field → method access
8. **Update tests** - Merge wire_test.go into context_test.go
9. **Run tests** - Verify PASS
10. **Verify all** - `make lint && make test && make functional`

## Consumer Update Checklist

| File | Changes | Status |
|------|---------|--------|
| `pkg/reactor/peer.go` | `recvCtx/sendCtx` field access → methods | |
| `pkg/reactor/reactor.go` | Context creation/access | |
| `pkg/reactor/session.go` | If uses context | |
| `pkg/rib/commit.go` | `dstCtx.ASN4` → `dstCtx.ASN4()` | |
| `pkg/rib/route.go` | Context param usage | |
| `pkg/plugin/wire_update_split.go` | `srcCtx` param usage | |
| `pkg/plugin/filter.go` | Context param usage | |
| `pkg/plugin/text.go` | `encCtx` usage | |
| `pkg/plugin/json.go` | Context usage | |
| `pkg/plugin/decode.go` | Context usage | |
| `pkg/plugin/server.go` | If uses context | |

## Risk Assessment

- **Low risk:** Mechanical refactor, same semantics
- **Testing:** Extensive existing tests catch regressions
- **Rollback:** Git revert if issues

## Implementation Summary

### What Was Implemented

1. **Renamed `WireContext` → `EncodingContext`** in `pkg/bgp/context/context.go`
2. **Removed old flat `EncodingContext`** (was a struct with public fields)
3. **Updated factory functions:**
   - `FromNegotiatedRecvWire` → `FromNegotiatedRecv`
   - `FromNegotiatedSendWire` → `FromNegotiatedSend`
4. **Added helper constructors:**
   - `EncodingContextForASN4(bool)` - for simple ASN4-only contexts
   - `EncodingContextWithAddPath(bool, map)` - for contexts with ADD-PATH
5. **Updated all consumers** (field → method access):
   - `ctx.ASN4` → `ctx.ASN4()`
   - `ctx.IsIBGP` → `ctx.IsIBGP()`
   - `ctx.LocalAS` → `ctx.LocalASN()`
   - `ctx.PeerAS` → `ctx.PeerASN()`
6. **Merged `wire.go` into `context.go`** and deleted `wire.go`
7. **Merged `wire_test.go` into `context_test.go`** and deleted `wire_test.go`
8. **Updated `docs/architecture/encoding-context.md`** to remove WireContext section
9. **Renamed hash functions** (removed "Wire" suffix)

### Files Changed

| Category | Files |
|----------|-------|
| Context package | `context.go`, `negotiated.go`, `api.go` |
| Context tests | `context_test.go`, `api_test.go`, `negotiated_test.go`, `registry_test.go`, `encoding_test.go` |
| Attribute encoding | `aspath.go`, `simple.go`, `wire.go` |
| Attribute tests | `len_test.go`, `len_writeto_test.go`, `opaque_test.go`, `pack_context_test.go`, `wire_test.go` |
| Plugin | `filter.go` |
| Plugin tests | `filter_test.go`, `text_test.go`, `wire_update_split_test.go` |
| Reactor | `peer.go`, `reactor.go` |
| Reactor tests | `forward_split_test.go`, `peer_test.go` |
| RIB | `commit.go` |
| RIB tests | `route_test.go` |
| Docs | `docs/architecture/encoding-context.md` |
| Deleted | `pkg/bgp/context/wire.go`, `pkg/bgp/context/wire_test.go` |

### Bugs Found/Fixed
- None

### Deviations from Plan
- Added `AddPathFor()` as alias for `AddPath()` (API compatibility)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC references in code

### Completion
- [x] Dead code check performed
- [x] Test coverage integrated
- [x] Architecture docs updated
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/`
- [ ] All files committed together
