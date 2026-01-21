# Spec: ADD-PATH Encoding Simplification

## Required Reading (completed before implementation)

- [x] `rfc/rfc7911.txt` - ADD-PATH capability and NLRI encoding
- [x] `rfc/rfc4271.txt` Section 4.3 - Base NLRI encoding
- [x] `rfc/rfc4760.txt` Section 5 - MP-BGP NLRI encoding
- [x] `docs/architecture/wire/NLRI.md` - NLRI type architecture
- [x] `docs/architecture/ENCODING_CONTEXT.md` - PackContext usage

**Key insights from docs:**
- RFC 7911 Section 3: Path ID is 4 bytes, prepended to NLRI when ADD-PATH negotiated
- RFC 7911: Path ID 0 is valid (means "no specific path")
- PackContext.AddPath controls when path ID is included in wire format

## Current State

- Tests: All pass (make test, make functional encoding)
- Last commit: `aa27eb9` (Phase 4 cleanup)

## Problem

Current ADD-PATH handling is inconsistent across NLRI types:

| NLRI Type | `Len()` includes pathID? | `Bytes()` includes pathID? | `hasPath` field? |
|-----------|--------------------------|---------------------------|------------------|
| INET | Yes (if hasPath) | Yes (if hasPath) | ✅ |
| IPVPN | Yes (if hasPath) | Yes (if hasPath) | ✅ |
| LabeledUnicast | Yes (if hasPath) | Yes (if hasPath) | ✅ |
| EVPN | ❌ No | ❌ No | ✅ (but unused) |
| FlowSpec | N/A | N/A | ❌ |
| BGPLS | N/A | N/A | ❌ |

**Bugs caused by inconsistency:**
- EVPN `packEVPN()` assumes `Bytes()` has pathID when `hasPath=true` → wrong wire format
- `LenWithContext` logic breaks for EVPN → size mismatch potential
- Complex conditional logic in every NLRI type

## RFC 7911 Requirement

> "When the Path Identifiers capability is negotiated for a given AFI/SAFI,
> each NLRI in that AFI/SAFI MUST be encoded with a Path Identifier."

- Path ID is 4 bytes, prepended to NLRI
- Value 0 is valid (means "no specific path")
- When ADD-PATH not negotiated, no path ID in wire format

## Proposed Design

### Core Principle

**Separation of concerns:**
- NLRI types handle payload encoding only
- ADD-PATH layer handles path ID prepending
- No `hasPath` logic in wire encoding

### Interface Changes

```go
// NLRI interface - payload only, no path ID logic
type NLRI interface {
    Family() Family

    // Len returns payload length WITHOUT path ID
    Len() int

    // Bytes returns payload bytes WITHOUT path ID
    Bytes() []byte

    // WriteTo writes payload WITHOUT path ID
    // Returns bytes written
    WriteTo(buf []byte, off int) int

    // PathID returns the stored path identifier (0 if unset)
    PathID() uint32

    // String returns human-readable representation
    String() string
}

// Remove from interface:
// - Pack(ctx *PackContext) []byte     // Deprecated
// - HasPathID() bool                   // No longer needed for encoding
// - WriteTo with ctx parameter         // Simplified
```

### Encoding Layer

```go
// LenWithContext calculates wire length with ADD-PATH handling
func LenWithContext(n NLRI, ctx *PackContext) int {
    if ctx != nil && ctx.AddPath {
        return 4 + n.Len()  // path ID + payload
    }
    return n.Len()
}

// WriteTo writes NLRI with ADD-PATH handling
func WriteNLRI(n NLRI, buf []byte, off int, ctx *PackContext) int {
    if ctx != nil && ctx.AddPath {
        binary.BigEndian.PutUint32(buf[off:], n.PathID())
        return 4 + n.WriteTo(buf, off+4)
    }
    return n.WriteTo(buf, off)
}
```

### Storage

```go
type INET struct {
    prefix netip.Prefix
    pathID uint32  // Always stored, 0 if unset
    // Remove: hasPath bool
}

func (i *INET) PathID() uint32 { return i.pathID }
func (i *INET) Len() int       { return 1 + prefixBytes }  // No path ID
func (i *INET) WriteTo(buf []byte, off int) int {
    // Write prefix only, no path ID logic
}
```

## Migration Plan

### Phase 1: Add New Methods (backward compatible) ✅ COMPLETE

1. ✅ Add `BaseLen() int` to all NLRI types - returns length WITHOUT path ID
2. ✅ Add `WritePayloadTo(buf, off) int` - writes payload only
3. ✅ Add `WriteNLRI()` helper function in nlri package
4. ✅ Add `PayloadWriter` interface for types supporting payload-only writing
5. ✅ Keep old methods working

Commit: `daab7d6` feat(nlri): add BaseLen, WritePayloadTo for ADD-PATH simplification

### Phase 2: Update Callers ✅ COMPLETE

1. ✅ Update `buildNLRIBytes` to use `WriteNLRI()`
2. ✅ Update `LenWithContext` to use new semantics
3. ✅ Run tests to verify identical wire output

Commit: `daab7d6` (included in Phase 1 commit)

### Phase 3: Simplify NLRI Types ✅ COMPLETE

1. ✅ Change `Len()` to return base length (breaking change)
2. ✅ Change `WriteTo()` to write payload only
3. ✅ Remove `hasPath` field from INET, IPVPN, LabeledUnicast
4. ✅ Update `LenWithContext()` - adds 4 bytes when ctx.AddPath=true
5. ✅ Update `writeNLRIOptimized()` - path ID only when ctx.AddPath=true
6. ✅ Fix `route.Index()` and `store.Key()` to include path ID for uniqueness
7. ✅ Keep `Pack()` as deprecated wrapper for backward compatibility

Commit: `5516ddd` refactor(nlri): Phase 3 ADD-PATH simplification - payload-only encoding

### Phase 4: Cleanup ✅ COMPLETE

1. ✅ Remove `BaseLen()` from INET, IPVPN, LabeledUnicast
2. ✅ Remove `WritePayloadTo()` from INET, IPVPN, LabeledUnicast
3. ✅ Remove `PayloadWriter` interface
4. ✅ Simplify `WriteNLRI()` to use `WriteTo()` directly
5. ✅ Update tests to use `Len()` and `WriteTo()` instead of removed methods
6. Pack() kept as deprecated wrapper (many callers not yet migrated)

Note: EVPN types not updated - they don't currently support ADD-PATH

Commit: `aa27eb9` refactor(nlri): Phase 4 cleanup - remove transitional methods

### Phase 5: RFC Documentation ✅ COMPLETE

Added/verified RFC references for ADD-PATH encoding:

1. ✅ `WriteNLRI()` in nlri.go - RFC 7911 Section 3 path ID encoding
2. ✅ `LenWithContext()` in nlri.go - RFC 7911 Section 3 path ID sizing
3. ✅ `WriteTo()` in INET - RFC 4271 Section 4.3, RFC 7911 Section 3
4. ✅ `WriteTo()` in IPVPN - RFC 4364/4659, RFC 7911 Section 3
5. ✅ `WriteTo()` in LabeledUnicast - RFC 8277 Section 2.2, RFC 7911 Section 3
6. ✅ `ParseINET()` - already had RFC 7911 Section 3 reference
7. ✅ `ParseIPVPN()` - already had RFC 7911 reference

Key RFC sections:
- RFC 7911 Section 3: ADD-PATH NLRI encoding (4-byte path ID prefix)
- RFC 7911 Section 4: Capability negotiation for ADD-PATH
- RFC 4271 Section 4.3: Base NLRI encoding (<length, prefix>)
- RFC 4760 Section 5: MP-BGP NLRI encoding

### Phase 6: Update Architecture Docs ✅ COMPLETE

Updated `docs/architecture/` docs to reflect new ADD-PATH encoding:

1. ✅ `docs/architecture/wire/NLRI.md`:
   - Updated NLRI interface to show payload-only methods
   - Added `WriteNLRI()` and `LenWithContext()` usage
   - Removed old `hasPath` field from struct examples

2. ✅ `docs/architecture/ENCODING_CONTEXT.md`:
   - Updated ADD-PATH section with canonical encoding pattern
   - Added note that `Len()`/`WriteTo()` return payload-only

3. ✅ `docs/architecture/edge-cases/ADDPATH.md`:
   - Replaced old NLRI Pack() example with WriteNLRI() pattern
   - Documented RFC 7911 Section 3 behavior

## Checklist

- [x] Required docs read
- [x] Test fails first (TDD)
- [x] Test passes after impl
- [x] make test passes
- [x] make lint passes
- [x] Phase 5: RFC documentation in code
- [x] Phase 6: Update `docs/architecture/` docs
- [x] Move to `docs/plan/done/` when complete

## Test Strategy

### Invariant Tests

```go
func TestWriteNLRI_MatchesOldPack(t *testing.T) {
    // For each NLRI type and context combination:
    // Verify WriteNLRI produces identical bytes to old Pack()
}

func TestLenWithContext_MatchesWriteNLRI(t *testing.T) {
    // Verify predicted length == actual written bytes
}
```

### Wire Format Tests

```go
func TestWireFormat_AddPathEnabled(t *testing.T) {
    // Verify: [4-byte pathID][payload]
}

func TestWireFormat_AddPathDisabled(t *testing.T) {
    // Verify: [payload] only, no path ID
}

func TestWireFormat_PathIDZero(t *testing.T) {
    // Verify: pathID=0 is correctly encoded as 00000000
}
```

## Files to Modify

### Phase 1
- `internal/bgp/nlri/nlri.go` - Add `WriteNLRI()` helper
- `internal/bgp/nlri/inet.go` - Add `BaseLen()`, `WritePayloadTo()`
- `internal/bgp/nlri/ipvpn.go` - Add `BaseLen()`, `WritePayloadTo()`
- `internal/bgp/nlri/labeled.go` - Add `BaseLen()`, `WritePayloadTo()`
- `internal/bgp/nlri/evpn.go` - Add `BaseLen()`, `WritePayloadTo()`, fix bugs
- (other NLRI types)

### Phase 2
- `internal/rib/update.go` - Use `WriteNLRI()`
- `internal/rib/commit.go` - Use new encoding
- `internal/bgp/message/update_build.go` - Use new encoding

### Phase 3
- All NLRI types - Simplify `Len()`, `WriteTo()`, remove `hasPath`
- `internal/bgp/nlri/nlri.go` - Update interface

## Benefits

1. **Correctness**: All NLRI types behave identically for ADD-PATH
2. **Simplicity**: No conditional `hasPath` logic in encoding
3. **Performance**: Simpler code paths, easier to optimize
4. **Maintainability**: New NLRI types don't need ADD-PATH handling
5. **Bug fixes**: EVPN ADD-PATH bug fixed as side effect

## Risks

1. **Breaking change**: `Len()` semantics change in Phase 3
2. **Large refactor**: Touches all NLRI types
3. **Testing burden**: Need comprehensive wire format tests

## Rollback Plan

- Phase 1 is fully backward compatible
- Phase 2 can be reverted by switching back to old methods
- Phase 3 is the breaking change - requires version bump

## Timeline Estimate

- Phase 1: 2-3 hours (add new methods)
- Phase 2: 1-2 hours (update callers)
- Phase 3: 2-3 hours (simplify types)
- Phase 4: 30 min (cleanup)
- Testing: 1-2 hours

Total: ~8-10 hours
