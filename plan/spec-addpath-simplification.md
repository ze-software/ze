# Spec: ADD-PATH Encoding Simplification

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

### Phase 1: Add New Methods (backward compatible)

1. Add `BaseLen() int` to all NLRI types - returns length WITHOUT path ID
2. Add `WritePayloadTo(buf, off) int` - writes payload only
3. Add `WriteNLRI()` helper function in nlri package
4. Keep old methods working

```go
// Temporary: both old and new methods coexist
func (i *INET) Len() int     { /* old behavior with hasPath */ }
func (i *INET) BaseLen() int { /* new behavior without pathID */ }
```

### Phase 2: Update Callers

1. Update `buildNLRIBytes` to use `WriteNLRI()`
2. Update `LenWithContext` to use `BaseLen()`
3. Update all message builders
4. Run tests to verify identical wire output

### Phase 3: Simplify NLRI Types

1. Change `Len()` to return base length (breaking change)
2. Change `WriteTo()` to write payload only
3. Remove `hasPath` field from all types
4. Remove `HasPathID()` from interface
5. Remove `Pack()` method (deprecated)

### Phase 4: Cleanup

1. Remove `BaseLen()` (now redundant with `Len()`)
2. Remove `WritePayloadTo()` (now redundant with `WriteTo()`)
3. Update documentation

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
- `pkg/bgp/nlri/nlri.go` - Add `WriteNLRI()` helper
- `pkg/bgp/nlri/inet.go` - Add `BaseLen()`, `WritePayloadTo()`
- `pkg/bgp/nlri/ipvpn.go` - Add `BaseLen()`, `WritePayloadTo()`
- `pkg/bgp/nlri/labeled.go` - Add `BaseLen()`, `WritePayloadTo()`
- `pkg/bgp/nlri/evpn.go` - Add `BaseLen()`, `WritePayloadTo()`, fix bugs
- (other NLRI types)

### Phase 2
- `pkg/rib/update.go` - Use `WriteNLRI()`
- `pkg/rib/commit.go` - Use new encoding
- `pkg/bgp/message/update_build.go` - Use new encoding

### Phase 3
- All NLRI types - Simplify `Len()`, `WriteTo()`, remove `hasPath`
- `pkg/bgp/nlri/nlri.go` - Update interface

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
