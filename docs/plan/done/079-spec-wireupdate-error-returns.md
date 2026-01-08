# Spec: WireUpdate Error Returns

## Task

Change WireUpdate accessor methods from `[]byte` to `([]byte, error)` to distinguish:
- Empty/valid: `nil, nil`
- Has data: `data, nil`
- Malformed: `nil, err`

## Required Reading

- [x] `.claude/zebgp/UPDATE_BUILDING.md` - WireUpdate accessor signatures, receive path
- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - Zero-copy context, WireUpdate structure

**Key insights:**
- WireUpdate holds UPDATE payload for zero-copy lazy parsing
- Malformed UPDATE → session teardown → no need to cache errors
- AttributesWire is cheap (slice wrapper), caching unnecessary

## Problem

Current methods return `nil` for both valid empty and malformed:

```go
func (u *WireUpdate) Withdrawn() []byte {
    if wdLen == 0 { return nil }      // Valid empty
    if truncated { return nil }        // Error - indistinguishable!
}
```

## Methods Changed

| Method | Before | After |
|--------|--------|-------|
| `Withdrawn()` | `[]byte` | `([]byte, error)` |
| `Attrs()` | `*AttributesWire` | `(*AttributesWire, error)` |
| `deriveAttrs()` | `*AttributesWire` | `(*AttributesWire, error)` |
| `NLRI()` | `[]byte` | `([]byte, error)` |
| `MPReach()` | `MPReachWire` | `(MPReachWire, error)` |
| `MPUnreach()` | `MPUnreachWire` | `(MPUnreachWire, error)` |

## Design Decisions

### Attrs() - No Caching (Simplified)

**Deviation from original plan:** Removed `sync.Once` caching entirely instead of keeping it for valid cases.

`AttributesWire` is cheap to create (just a slice wrapper with context ID).
Each call derives fresh. Cost is negligible vs complexity of error-aware caching.

```go
// BEFORE: cached with sync.Once
type WireUpdate struct {
    payload     []byte
    sourceCtxID bgpctx.ContextID
    messageID   uint64
    attrsOnce   sync.Once              // REMOVED
    attrs       *attribute.AttributesWire // REMOVED
}

// AFTER: no caching
type WireUpdate struct {
    payload     []byte
    sourceCtxID bgpctx.ContextID
    messageID   uint64
}

func (u *WireUpdate) Attrs() (*attribute.AttributesWire, error) {
    return u.deriveAttrs()  // Fresh each call
}
```

**Rationale:** Avoids sync.Once bug where second call after error returns `nil, nil` instead of error.

### Error Context with fmt.Errorf

```go
return nil, fmt.Errorf("withdrawn: %w", ErrUpdateTruncated)
return nil, fmt.Errorf("mp_reach: %w", ErrUpdateMalformed)
```

Base errors in `pkg/api/errors.go`:
```go
var (
    ErrUpdateTruncated = errors.New("UPDATE payload truncated")
    ErrUpdateMalformed = errors.New("UPDATE malformed")
)
```

### Return Semantics

| Method | Condition | Return |
|--------|-----------|--------|
| `Withdrawn()` | wdLen == 0 | `nil, nil` |
| `Withdrawn()` | payload truncated | `nil, fmt.Errorf("withdrawn: %w", ErrUpdateTruncated)` |
| `Attrs()` | attrLen == 0 | `nil, nil` |
| `Attrs()` | payload truncated | `nil, fmt.Errorf("attrs: %w", ErrUpdateTruncated)` |
| `NLRI()` | no trailing bytes | `nil, nil` |
| `NLRI()` | payload truncated | `nil, fmt.Errorf("nlri: %w", ErrUpdateTruncated)` |
| `MPReach()` | attr not present | `nil, nil` |
| `MPReach()` | GetRaw() error | `nil, fmt.Errorf("mp_reach: %w", err)` |
| `MPReach()` | len < 5 | `nil, fmt.Errorf("mp_reach: %w", ErrUpdateMalformed)` |
| `MPUnreach()` | attr not present | `nil, nil` |
| `MPUnreach()` | GetRaw() error | `nil, fmt.Errorf("mp_unreach: %w", err)` |
| `MPUnreach()` | len < 3 | `nil, fmt.Errorf("mp_unreach: %w", ErrUpdateMalformed)` |

## Files Modified

### Core Changes
- `pkg/api/wire_update.go` - Changed signatures, removed `attrsOnce`/`attrs` fields
- `pkg/api/errors.go` - NEW: Define base error types

### Test Updates
- `pkg/api/wire_update_test.go` - Added error tests, updated existing tests
- `pkg/api/wire_update_split_test.go` - Handle error returns
- `pkg/api/filter_test.go` - Handle Attrs() error
- `pkg/api/text_test.go` - Handle Attrs() error

### Caller Updates
- `pkg/reactor/reactor.go:2424` - Ignore Attrs() error in callback (observes, doesn't validate)
- `pkg/reactor/reactor_test.go` - Handle error returns
- `pkg/reactor/received_update_test.go` - Handle error returns

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestWireUpdate_Withdrawn_Error` | `pkg/api/wire_update_test.go` | Truncated returns error |
| `TestWireUpdate_Withdrawn_Empty` | `pkg/api/wire_update_test.go` | wdLen=0 returns nil,nil |
| `TestWireUpdate_Attrs_Error` | `pkg/api/wire_update_test.go` | Truncated returns error |
| `TestWireUpdate_Attrs_Empty` | `pkg/api/wire_update_test.go` | attrLen=0 returns nil,nil |
| `TestWireUpdate_NLRI_Error` | `pkg/api/wire_update_test.go` | Truncated returns error |
| `TestWireUpdate_NLRI_Empty` | `pkg/api/wire_update_test.go` | No trailing bytes returns nil,nil |
| `TestWireUpdate_MPReach_NotPresent` | `pkg/api/wire_update_test.go` | Missing attr returns nil,nil |
| `TestWireUpdate_MPReach_Malformed` | `pkg/api/wire_update_test.go` | len < 5 returns error |
| `TestWireUpdate_MPReach_AttrsError` | `pkg/api/wire_update_test.go` | Propagates Attrs() error |
| `TestWireUpdate_MPUnreach_NotPresent` | `pkg/api/wire_update_test.go` | Missing attr returns nil,nil |
| `TestWireUpdate_MPUnreach_Malformed` | `pkg/api/wire_update_test.go` | len < 3 returns error |
| `TestWireUpdate_MPUnreach_AttrsError` | `pkg/api/wire_update_test.go` | Propagates Attrs() error |
| `TestWireUpdate_AttrsConsistent` | `pkg/api/wire_update_test.go` | Multiple calls return same data |
| `TestSplitWireUpdate_OutputChunksAccessible` | `pkg/api/wire_update_split_test.go` | All split chunks parse without error |
| `TestSplitWireUpdate_BaseAttrsInAllChunks` | `pkg/api/wire_update_split_test.go` | Base attrs replicated in all chunks |
| `TestSplitWireUpdate_MixedIPv4AndMP` | `pkg/api/wire_update_split_test.go` | Mixed IPv4 + MP_REACH preserved |

### Functional Tests

N/A - Unit tests sufficient for this refactor (no new protocol behavior).

## Known Test Gaps - RESOLVED

Edge cases now covered:

1. ✅ `TestWireUpdate_WithdrawnOK_AttrsMissing` - Withdrawn succeeds, Attrs/NLRI fail
2. ✅ `TestWireUpdate_Attrs_TruncatedByOne` - Attrs truncated at boundary
3. ✅ `TestWireUpdate_NLRI_WithEmptyAttrs` - NLRI with empty attrs
4. ✅ `TestWireUpdate_NLRI_SingleByte` - Single byte NLRI

### SplitWireUpdate Edge Cases (added 2026-01-06)

5. ✅ `TestSplitWireUpdate_OutputChunksAccessible` - All output chunks parse without error
6. ✅ `TestSplitWireUpdate_BaseAttrsInAllChunks` - Base attrs replicated in all chunks
7. ✅ `TestSplitWireUpdate_MixedIPv4AndMP` - IPv4 + MP_REACH together

## Implementation Steps

1. **Write tests** - Create error expectation tests ✅
2. **Run tests** - Verify FAIL ✅
3. **Implement** - Add error returns to all accessors ✅
4. **Run tests** - Verify PASS ✅
5. **Verify all** - `make lint && make test && make functional` ✅
6. **RFC refs** - Added RFC 4271/4760 comments to code ✅

## RFC Documentation

- RFC 4271 Section 4.3 - UPDATE Message Format (Withdrawn, Attrs, NLRI structure)
- RFC 4760 Section 3 - MP_REACH_NLRI (minimum 5 bytes: AFI+SAFI+NHLen+Reserved)
- RFC 4760 Section 4 - MP_UNREACH_NLRI (minimum 3 bytes: AFI+SAFI)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL before implementation
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC references added
- [x] `.claude/zebgp/UPDATE_BUILDING.md` updated
- [x] `.claude/zebgp/ENCODING_CONTEXT.md` updated

### Completion
- [x] Spec moved to `docs/plan/done/079-spec-wireupdate-error-returns.md`

## Summary

✅ **Complete** - All methods return errors, callers updated, tests pass.

⚠️ **Deviation** - Removed sync.Once caching entirely (simpler, avoids error-caching bug).

---

## Follow-up: SplitWireUpdate Improvements

Identified during critical review. Should be done in separate spec.

### Issue #1: O(n²) NLRI Splitting 🔴 High Priority

**Problem:** `splitNLRIs` calls `ChunkMPNLRI` to split into N chunks, then recombines chunks 1..N back into `remaining`. Next iteration splits `remaining` again.

```go
// Current: O(n²)
chunks, _ := message.ChunkMPNLRI(data, afi, safi, addPath, maxBytes)
var rest []byte
for i := 1; i < len(chunks); i++ {
    rest = append(rest, chunks[i]...)  // Recombine = wasteful
}
return chunks[0], rest, nil
```

**Fix:** Return all chunks at once, consume them directly:
```go
func splitNLRIsAll(...) ([][]byte, error) {
    return message.ChunkMPNLRI(...)
}
```

### Issue #2: Duplicate Validation Logic 🟡 Medium Priority

**Problem:** Lines 32-45 in `SplitWireUpdate` duplicate the validation in `WireUpdate.Withdrawn()`, `Attrs()`, `NLRI()`.

```go
// SplitWireUpdate - manual parsing
if len(payload) < 4 {
    return nil, fmt.Errorf("UPDATE too short: %d bytes", len(payload))
}
withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
```

**Fix Options:**
1. Extract shared `parseUpdateStructure()` helper
2. Use WireUpdate methods and extract raw bytes from results

### Issue #3: Error Type Inconsistency 🟡 Medium Priority

| Location | Error Style |
|----------|-------------|
| `WireUpdate.Withdrawn()` | `fmt.Errorf("withdrawn: %w", ErrUpdateTruncated)` |
| `SplitWireUpdate` | `fmt.Errorf("UPDATE too short: %d bytes", len)` |
| `separateMPAttributes` | `fmt.Errorf("truncated attribute at %d", pos)` |

**Fix:** Use `ErrUpdateTruncated`/`ErrUpdateMalformed` consistently.

### Issue #4: Magic Numbers 🟢 Low Priority

```go
total := 4 + len(ipv4W) + len(mpUnreach)  // What is 4?
```

**Fix:**
```go
const updateLengthFieldsSize = 4  // 2 bytes withdrawn len + 2 bytes attr len
```

### Issue #5: splitMPReach/splitMPUnreach Re-parse Header 🟡 Medium

Each split call re-parses attribute header. Better to parse once, return all chunks:

```go
func splitMPReachAll(attr []byte, maxBytes int, ctx *bgpctx.EncodingContext) ([][]byte, error)
```

### Issue #6: Undocumented Behavior 🟢 Low

IPv4 routes only included in first iteration when multiple MP_REACH/MP_UNREACH exist:
```go
if i == 0 {
    useIPv4W = remIPv4W
    useIPv4A = remIPv4A
}
```
Should be documented in function comment.

### Issue #7: No Output Validation 🟢 Low

Split chunks are trusted to be valid. Could add defensive check:
```go
for _, chunk := range results {
    if _, err := chunk.Withdrawn(); err != nil {
        return nil, fmt.Errorf("internal: produced invalid chunk: %w", err)
    }
}
```

### Recommended Implementation Order

1. **Phase 1 (High Priority):** Fix O(n²) in splitNLRIs ✅
2. **Phase 2 (Medium Priority):** Consistent error types, extract shared validation ✅
3. **Phase 3 (Low Priority):** Named constants, documentation, output validation ✅

**Status:** Implemented in `docs/plan/spec-splitwireupdate-improvements.md`
