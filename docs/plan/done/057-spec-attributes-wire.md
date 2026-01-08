# Spec: AttributesWire - Wire-Canonical Attribute Storage

## Status: Done

> **📍 LOCATION CHANGE:** Wire-canonical storage now lives in **API programs**
> via base64-encoded wire bytes in JSON events. The design principles remain valid.
> See `docs/architecture/rib-transition.md` for the overall architecture.

## Problem

Current Route stores both parsed attributes AND wire cache:
```go
type Route struct {
    attributes []attribute.Attribute  // Parsed form
    wireBytes  []byte                 // Wire cache
}
```

Issues:
- Memory duplication (both forms stored)
- Full parsing on receive even if not needed
- No partial attribute access

## Solution

`AttributesWire` - wire bytes as canonical storage with lazy parsing:

```go
type AttributesWire struct {
    mu          sync.RWMutex                     // Thread safety
    packed      []byte                           // Canonical wire bytes (NOT OWNED)
    sourceCtxID bgpctx.ContextID                 // Encoding context
    index       []attrIndex                      // Offset cache (built on first scan)
    parsed      map[AttributeCode]Attribute      // Lazy parse cache (sparse)
}

type attrIndex struct {
    code   AttributeCode
    offset uint16  // Offset into packed (points to value, after header)
    length uint16  // Value length (excludes header)
    hdrLen uint8   // Header length (3 or 4) - used to locate flags for unknown attrs
}
```

### Memory Contract

**IMPORTANT:** `packed` is a reference to external memory (message buffer).

**Lifetime Rule:** AttributesWire lifetime is tied to the Message that created it.
- Message owns the buffer; AttributesWire borrows it
- When Message is released to pool, all derived AttributesWire become invalid
- Route must not outlive its source Message (enforced by RIB design)

**Accepted Risk:** This is a use-after-free footgun if misused. We accept this
risk for zero-copy performance. The code path from Message → Route → AttributesWire
must maintain this invariant.

**Caller Contract:**
- MUST NOT modify buffer contents after construction
- MUST NOT use AttributesWire after Message is released

**Future:** Will transition to pool.Handle (see spec-pool-handle-migration.md)
for explicit lifetime tracking.

## API

```go
// Constructor - packed is NOT copied, caller retains ownership
func NewAttributesWire(packed []byte, ctxID bgpctx.ContextID) *AttributesWire

// Zero-copy access - returns reference to internal buffer
// WARNING: Do not modify returned slice
func (a *AttributesWire) Packed() []byte

// Lazy single-attribute access (parse on demand, cache result)
func (a *AttributesWire) Get(code AttributeCode) (Attribute, error)

// Check existence without parsing value
// Returns error if wire bytes are malformed
func (a *AttributesWire) Has(code AttributeCode) (bool, error)

// Parse subset (for API output)
func (a *AttributesWire) GetMultiple(codes []AttributeCode) (map[AttributeCode]Attribute, error)

// Full parse (when needed)
func (a *AttributesWire) All() ([]Attribute, error)

// Context-aware forwarding
func (a *AttributesWire) PackFor(destCtxID bgpctx.ContextID) ([]byte, error)

// SourceContext returns the encoding context ID
func (a *AttributesWire) SourceContext() bgpctx.ContextID
```

## Implementation

### File: `pkg/bgp/attribute/wire.go`

```go
package attribute

import (
    "fmt"
    "sync"

    bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
)

// attrIndex caches attribute location within packed bytes.
// Built lazily on first scan, reused for subsequent lookups.
// hdrLen is retained to locate original flags for unknown attributes.
type attrIndex struct {
    code   AttributeCode
    offset uint16  // Points to value (after header)
    length uint16
    hdrLen uint8   // 3 or 4; flags at packed[offset-hdrLen]
}

// AttributesWire stores path attributes in wire format with lazy parsing.
//
// Wire bytes are the canonical representation. Parsed attributes are
// cached on demand. Thread-safe for concurrent read access.
//
// Memory contract: packed is NOT owned by AttributesWire. Caller must
// ensure the underlying buffer outlives this struct and is not modified.
type AttributesWire struct {
    mu          sync.RWMutex
    packed      []byte
    sourceCtxID bgpctx.ContextID
    index       []attrIndex                  // nil until first scan
    parsed      map[AttributeCode]Attribute  // nil until first parse
}

// NewAttributesWire creates from raw packed bytes.
// WARNING: packed is NOT copied. Caller retains ownership and must not modify.
func NewAttributesWire(packed []byte, ctxID bgpctx.ContextID) *AttributesWire {
    return &AttributesWire{
        packed:      packed,
        sourceCtxID: ctxID,
    }
}

// Packed returns raw wire bytes for transmission.
// WARNING: Do not modify the returned slice.
func (a *AttributesWire) Packed() []byte {
    return a.packed
}

// SourceContext returns the encoding context ID.
func (a *AttributesWire) SourceContext() bgpctx.ContextID {
    return a.sourceCtxID
}

// Get returns a specific attribute by code (lazy parse).
func (a *AttributesWire) Get(code AttributeCode) (Attribute, error) {
    a.mu.RLock()
    // Check parse cache
    if a.parsed != nil {
        if attr, ok := a.parsed[code]; ok {
            a.mu.RUnlock()
            return attr, nil
        }
    }
    a.mu.RUnlock()

    a.mu.Lock()
    defer a.mu.Unlock()

    // Double-check after acquiring write lock
    if a.parsed != nil {
        if attr, ok := a.parsed[code]; ok {
            return attr, nil
        }
    }

    // Build index if needed
    if err := a.ensureIndexLocked(); err != nil {
        return nil, err
    }

    // Find attribute in index
    for _, idx := range a.index {
        if idx.code == code {
            attr, err := a.parseAtLocked(idx)
            if err != nil {
                return nil, err
            }
            if a.parsed == nil {
                a.parsed = make(map[AttributeCode]Attribute)
            }
            a.parsed[code] = attr
            return attr, nil
        }
    }

    return nil, nil // Not found (not an error)
}

// Has checks if attribute exists without parsing value.
// Returns error if wire bytes are malformed.
func (a *AttributesWire) Has(code AttributeCode) (bool, error) {
    a.mu.RLock()
    // Check parse cache first
    if a.parsed != nil {
        if _, ok := a.parsed[code]; ok {
            a.mu.RUnlock()
            return true, nil
        }
    }

    // Check index if built
    if a.index != nil {
        for _, idx := range a.index {
            if idx.code == code {
                a.mu.RUnlock()
                return true, nil
            }
        }
        a.mu.RUnlock()
        return false, nil
    }
    a.mu.RUnlock()

    // Build index (upgrades to write lock)
    a.mu.Lock()
    defer a.mu.Unlock()

    if err := a.ensureIndexLocked(); err != nil {
        return false, err
    }

    for _, idx := range a.index {
        if idx.code == code {
            return true, nil
        }
    }
    return false, nil
}

// GetMultiple returns multiple attributes (for API output).
func (a *AttributesWire) GetMultiple(codes []AttributeCode) (map[AttributeCode]Attribute, error) {
    result := make(map[AttributeCode]Attribute, len(codes))
    for _, code := range codes {
        attr, err := a.Get(code)
        if err != nil {
            return nil, fmt.Errorf("getting %s: %w", code, err)
        }
        if attr != nil {
            result[code] = attr
        }
    }
    return result, nil
}

// All returns all attributes (full parse).
// Attributes are returned in wire order.
func (a *AttributesWire) All() ([]Attribute, error) {
    a.mu.Lock()
    defer a.mu.Unlock()

    if err := a.ensureIndexLocked(); err != nil {
        return nil, err
    }

    result := make([]Attribute, 0, len(a.index))
    for _, idx := range a.index {
        // Check cache first
        if a.parsed != nil {
            if attr, ok := a.parsed[idx.code]; ok {
                result = append(result, attr)
                continue
            }
        }

        attr, err := a.parseAtLocked(idx)
        if err != nil {
            return nil, err
        }

        // Cache it
        if a.parsed == nil {
            a.parsed = make(map[AttributeCode]Attribute)
        }
        a.parsed[idx.code] = attr
        result = append(result, attr)
    }

    return result, nil
}

// PackFor returns packed bytes for destination context.
// Zero-copy if contexts match, otherwise re-encode.
func (a *AttributesWire) PackFor(destCtxID bgpctx.ContextID) ([]byte, error) {
    if a.sourceCtxID == destCtxID {
        return a.packed, nil
    }

    // Slow path: re-encode with destination context
    destCtx := bgpctx.Registry.Get(destCtxID)
    if destCtx == nil {
        return nil, fmt.Errorf("unknown context ID: %d", destCtxID)
    }

    return a.packWithContext(destCtx)
}

// ensureIndexLocked builds the attribute index if not already built.
// Caller must hold write lock.
// RFC 4271: Duplicate attributes are a Malformed Attribute List error.
func (a *AttributesWire) ensureIndexLocked() error {
    if a.index != nil {
        return nil
    }

    // Estimate capacity: typical UPDATE has 4-8 attributes
    a.index = make([]attrIndex, 0, 8)
    seen := make(map[AttributeCode]bool, 8)

    offset := 0
    for offset < len(a.packed) {
        _, code, length, hdrLen, err := ParseHeader(a.packed[offset:])
        if err != nil {
            return fmt.Errorf("parsing header at offset %d: %w", offset, err)
        }

        // RFC 4271: duplicate attributes are malformed
        if seen[code] {
            return fmt.Errorf("duplicate attribute %s at offset %d", code, offset)
        }
        seen[code] = true

        // Validate we have enough data
        if offset+hdrLen+int(length) > len(a.packed) {
            return fmt.Errorf("attribute %s truncated at offset %d", code, offset)
        }

        a.index = append(a.index, attrIndex{
            code:   code,
            offset: uint16(offset + hdrLen), // Points to value, not header
            length: length,
            hdrLen: uint8(hdrLen),
        })

        offset += hdrLen + int(length)
    }

    return nil
}

// parseAtLocked parses the attribute at the given index.
// Caller must hold lock.
func (a *AttributesWire) parseAtLocked(idx attrIndex) (Attribute, error) {
    valueBytes := a.packed[idx.offset : idx.offset+idx.length]

    // Get source context for context-dependent parsing (e.g., ASN4)
    srcCtx := bgpctx.Registry.Get(a.sourceCtxID)
    if srcCtx == nil {
        return nil, fmt.Errorf("unknown source context ID: %d", a.sourceCtxID)
    }

    // Try known attribute parsers first
    attr, err := parseKnownAttribute(idx.code, valueBytes, srcCtx)
    if err != nil {
        return nil, err
    }
    if attr != nil {
        return attr, nil
    }

    // Unknown attribute: read original flags from header for preservation
    // Flags are at the start of the header: packed[offset - hdrLen]
    flags := AttributeFlags(a.packed[idx.offset-uint16(idx.hdrLen)])
    return NewOpaqueAttribute(flags, idx.code, valueBytes), nil
}

// packWithContext re-encodes all attributes with destination context.
func (a *AttributesWire) packWithContext(destCtx *bgpctx.EncodingContext) ([]byte, error) {
    attrs, err := a.All()
    if err != nil {
        return nil, err
    }

    srcCtx := bgpctx.Registry.Get(a.sourceCtxID)
    if srcCtx == nil {
        return nil, fmt.Errorf("unknown source context ID: %d", a.sourceCtxID)
    }

    // Estimate size
    buf := make([]byte, 0, len(a.packed))

    for _, attr := range attrs {
        packed := attr.PackWithContext(srcCtx, destCtx)
        hdr := PackHeader(attr.Flags(), attr.Code(), uint16(len(packed)))
        buf = append(buf, hdr...)
        buf = append(buf, packed...)
    }

    return buf, nil
}

// parseKnownAttribute parses a known attribute value by code.
// Returns (nil, nil) for unknown attribute codes - caller handles as OpaqueAttribute.
// Known attributes derive their flags from type; only OpaqueAttribute needs stored flags.
// REQUIRES: ctx != nil (caller must validate context exists)
func parseKnownAttribute(code AttributeCode, data []byte, ctx *bgpctx.EncodingContext) (Attribute, error) {
    if ctx == nil {
        return nil, fmt.Errorf("nil encoding context")
    }
    fourByteAS := ctx.ASN4

    switch code {
    case AttrOrigin:
        return ParseOrigin(data)
    case AttrASPath:
        return ParseASPath(data, fourByteAS)
    case AttrNextHop:
        return ParseNextHop(data)
    case AttrMED:
        return ParseMED(data)
    case AttrLocalPref:
        return ParseLocalPref(data)
    case AttrAtomicAggregate:
        // RFC 4271: ATOMIC_AGGREGATE has length 0
        if len(data) != 0 {
            return nil, fmt.Errorf("ATOMIC_AGGREGATE must be empty, got %d bytes", len(data))
        }
        return &AtomicAggregate{}, nil
    case AttrAggregator:
        return ParseAggregator(data, fourByteAS)
    case AttrOriginatorID:
        return ParseOriginatorID(data)
    case AttrClusterList:
        return ParseClusterList(data)
    case AttrCommunity:
        return ParseCommunities(data)
    case AttrMPReachNLRI:
        return ParseMPReachNLRI(data)
    case AttrMPUnreachNLRI:
        return ParseMPUnreachNLRI(data)
    case AttrExtCommunity:
        return ParseExtendedCommunities(data)
    case AttrAS4Path:
        return ParseAS4Path(data)
    case AttrAS4Aggregator:
        return ParseAS4Aggregator(data)
    case AttrLargeCommunity:
        return ParseLargeCommunities(data)
    case AttrIPv6ExtCommunity:
        return ParseIPv6ExtendedCommunities(data)
    default:
        // Unknown - caller will create OpaqueAttribute with preserved flags
        return nil, nil
    }
}
```

## Test Plan

```go
// TestAttributesWireGet verifies lazy single-attribute parsing.
// VALIDATES: Only requested attribute is parsed, cached for reuse.
// PREVENTS: Full parse on single attribute access.
func TestAttributesWireGet(t *testing.T)

// TestAttributesWireGetError verifies error handling for malformed data.
// VALIDATES: Errors returned with context, not swallowed.
// PREVENTS: Silent failures on corrupt wire bytes.
func TestAttributesWireGetError(t *testing.T)

// TestAttributesWireHas verifies header-only scanning.
// VALIDATES: Check existence without parsing value, returns error on malformed data.
// PREVENTS: Parsing overhead for existence check, silent failures.
func TestAttributesWireHas(t *testing.T)

// TestAttributesWireGetMultiple verifies partial parsing.
// VALIDATES: Only requested attributes are parsed.
// PREVENTS: Parsing unrequested attributes.
func TestAttributesWireGetMultiple(t *testing.T)

// TestAttributesWirePackFor verifies zero-copy forwarding.
// VALIDATES: Same context returns original bytes.
// PREVENTS: Unnecessary re-encoding.
func TestAttributesWirePackFor(t *testing.T)

// TestAttributesWirePackForDifferentContext verifies re-encoding.
// VALIDATES: Different context triggers re-encode.
// PREVENTS: Sending wrong encoding to peer.
func TestAttributesWirePackForDifferentContext(t *testing.T)

// TestAttributesWireAll verifies full parse.
// VALIDATES: All attributes returned in wire order.
// PREVENTS: Missing or duplicated attributes.
func TestAttributesWireAll(t *testing.T)

// TestAttributesWireConcurrentAccess verifies thread safety.
// VALIDATES: Concurrent Get() calls don't race.
// PREVENTS: Data races on parsed cache.
func TestAttributesWireConcurrentAccess(t *testing.T)

// TestAttributesWireIndexReuse verifies index caching.
// VALIDATES: Second Get() reuses index, doesn't rescan.
// PREVENTS: O(n^2) scanning for multiple Gets.
func TestAttributesWireIndexReuse(t *testing.T)

// TestAttributesWireDuplicateAttribute verifies RFC 4271 compliance.
// VALIDATES: Duplicate attributes return error.
// PREVENTS: Silent acceptance of malformed UPDATE messages.
func TestAttributesWireDuplicateAttribute(t *testing.T)

// TestAttributesWireEmptyPacked verifies edge case handling.
// VALIDATES: Empty packed bytes returns empty results, not error.
// PREVENTS: Nil pointer dereference on empty input.
func TestAttributesWireEmptyPacked(t *testing.T)

// TestAttributesWireUnknownAttribute verifies opaque handling.
// VALIDATES: Unknown attribute codes return OpaqueAttribute.
// PREVENTS: Errors on vendor-specific or future attributes.
func TestAttributesWireUnknownAttribute(t *testing.T)

// TestAttributesWireInvalidContext verifies context validation.
// VALIDATES: Invalid context ID returns error.
// PREVENTS: Nil pointer dereference on missing context.
func TestAttributesWireInvalidContext(t *testing.T)

// TestAttributesWirePreservesFlags verifies flag preservation for unknown attributes.
// VALIDATES: Unknown transitive attributes retain original flags including Partial bit.
// PREVENTS: Incorrect flag reconstruction during forwarding (RFC 4271 violation).
func TestAttributesWirePreservesFlags(t *testing.T)

// TestAttributesWireAtomicAggregateValidation verifies length validation.
// VALIDATES: ATOMIC_AGGREGATE with non-zero length returns error.
// PREVENTS: Silent acceptance of malformed ATOMIC_AGGREGATE.
func TestAttributesWireAtomicAggregateValidation(t *testing.T)

// TestAttributesWireOriginatorID verifies route reflection attribute parsing.
// VALIDATES: ORIGINATOR_ID is correctly parsed.
// PREVENTS: Route reflection failures.
func TestAttributesWireOriginatorID(t *testing.T)
```

## Checklist

- [x] Tests written for Get() (success + not found)
- [x] Tests written for Get() (error cases)
- [x] Tests written for Has() (success + error)
- [x] Tests written for GetMultiple()
- [x] Tests written for PackFor() (same context)
- [x] Tests written for PackFor() (different context)
- [x] Tests written for All()
- [x] Tests written for concurrent access
- [x] Tests written for index reuse
- [x] Tests written for duplicate attribute detection
- [x] Tests written for empty packed bytes
- [x] Tests written for unknown attribute codes
- [x] Tests written for invalid context ID
- [x] Tests written for flag preservation (unknown attrs)
- [x] Tests written for ATOMIC_AGGREGATE validation
- [x] Tests written for ORIGINATOR_ID parsing
- [x] Tests FAIL before implementation
- [x] Implementation makes tests pass
- [x] `make test` passes
- [x] `make lint` passes

## Dependencies

- `pkg/bgp/context/` (EncodingContext, ContextID, Registry) - EXISTS
  - **Requirement:** `Registry.Get()` must be thread-safe (concurrent reads)
- `pkg/bgp/attribute/` (Attribute interface, ParseHeader, Parse* functions) - EXISTS
  - **Attribute interface:**
    ```go
    type Attribute interface {
        Code() AttributeCode
        Flags() AttributeFlags  // Base flags only (Optional, Transitive, Partial)
        PackWithContext(src, dst *EncodingContext) []byte
    }
    ```
  - **ParseHeader signature:** `func ParseHeader(data []byte) (flags AttributeFlags, code AttributeCode, length uint16, hdrLen int, err error)`
  - **PackHeader signature:** `func PackHeader(flags AttributeFlags, code AttributeCode, length uint16) []byte`
    - RFC 4271 Section 4.3: Extended Length bit is wire-format only
    - `PackHeader` determines Extended Length **solely from length parameter**:
      - Sets Extended Length if `length > 255`, clears otherwise
      - Ignores any Extended Length bit in input flags
      - Zeroes lower 4 bits of flags (RFC 4271: "MUST be zero when sent")
    - This ensures correct encoding for both known attrs and OpaqueAttribute
  - **NewOpaqueAttribute signature:** `func NewOpaqueAttribute(flags AttributeFlags, code AttributeCode, data []byte) *OpaqueAttribute`
    - `flags` preserved for forwarding (only OpaqueAttribute needs stored flags)
    - `data` is NOT copied (follows memory contract)
    - `PackWithContext()` returns data unchanged (can't re-encode unknown structure)
  - **AttributeCode:** Must implement `fmt.Stringer` for error messages

## Dependents

- `spec-route-id-forwarding.md` - Route uses AttributesWire
- `spec-api-attribute-filter.md` - API uses GetMultiple()

## Future Work

- `spec-pool-handle-migration.md` - Transition `packed []byte` to `pool.Handle`
  for memory deduplication across routes

## Notes

### RFC 7606 Consideration

RFC 7606 defines "treat-as-withdraw" for certain malformed attributes instead of
session reset. AttributesWire returns errors up the stack; callers (UPDATE parser)
must decide whether to:
1. Send NOTIFICATION and reset session (traditional)
2. Treat as withdrawal (RFC 7606)

This decision is outside AttributesWire's scope but callers should be aware.

---

**Created:** 2026-01-01
**Updated:** 2026-01-01 - Added thread safety, error handling, offset caching
**Updated:** 2026-01-01 - Review fixes: Has() returns error, duplicate detection, nil context handling, expanded tests
**Updated:** 2026-01-01 - Review v2 fixes: flags preservation, ATOMIC_AGGREGATE validation, ORIGINATOR_ID, RFC 7606 note
**Updated:** 2026-01-01 - Simplified: flags only stored for OpaqueAttribute (known attrs derive from type)
**Updated:** 2026-01-01 - Documented: Attribute interface, PackHeader sets Extended Length per RFC 4271 §4.3
**Updated:** 2026-01-01 - Clarified: PackHeader ignores input Extended Length, zeroes lower 4 flag bits
**Completed:** 2026-01-01 - Implementation done: wire.go, opaque.go, ParseOriginatorID, 33 tests
