# Spec: AttributesWire - Wire-Canonical Attribute Storage

## Status: Ready for Implementation

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
    offset uint16  // Offset into packed
    length uint16  // Value length (excludes header)
    hdrLen uint8   // Header length (3 or 4)
}
```

### Memory Contract

**IMPORTANT:** `packed` is a reference to external memory (message buffer).
- Caller MUST ensure buffer lifetime exceeds AttributesWire lifetime
- Caller MUST NOT modify buffer contents after construction
- Future: Will transition to pool.Handle (see spec-pool-handle-migration.md)

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
func (a *AttributesWire) Has(code AttributeCode) bool

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

    bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
)

// attrIndex caches attribute location within packed bytes.
// Built lazily on first scan, reused for subsequent lookups.
type attrIndex struct {
    code   AttributeCode
    offset uint16
    length uint16
    hdrLen uint8
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
func (a *AttributesWire) Has(code AttributeCode) bool {
    a.mu.RLock()
    // Check parse cache first
    if a.parsed != nil {
        if _, ok := a.parsed[code]; ok {
            a.mu.RUnlock()
            return true
        }
    }

    // Check index if built
    if a.index != nil {
        for _, idx := range a.index {
            if idx.code == code {
                a.mu.RUnlock()
                return true
            }
        }
        a.mu.RUnlock()
        return false
    }
    a.mu.RUnlock()

    // Build index (upgrades to write lock)
    a.mu.Lock()
    defer a.mu.Unlock()

    if err := a.ensureIndexLocked(); err != nil {
        return false
    }

    for _, idx := range a.index {
        if idx.code == code {
            return true
        }
    }
    return false
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
func (a *AttributesWire) ensureIndexLocked() error {
    if a.index != nil {
        return nil
    }

    // Estimate capacity: typical UPDATE has 4-8 attributes
    a.index = make([]attrIndex, 0, 8)

    offset := 0
    for offset < len(a.packed) {
        _, code, length, hdrLen, err := ParseHeader(a.packed[offset:])
        if err != nil {
            return fmt.Errorf("parsing header at offset %d: %w", offset, err)
        }

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

    return parseAttributeValue(idx.code, valueBytes, srcCtx)
}

// packWithContext re-encodes all attributes with destination context.
func (a *AttributesWire) packWithContext(destCtx *bgpctx.EncodingContext) ([]byte, error) {
    attrs, err := a.All()
    if err != nil {
        return nil, err
    }

    srcCtx := bgpctx.Registry.Get(a.sourceCtxID)

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

// parseAttributeValue parses a single attribute value by code.
// This dispatches to the appropriate type-specific parser.
func parseAttributeValue(code AttributeCode, data []byte, ctx *bgpctx.EncodingContext) (Attribute, error) {
    fourByteAS := ctx == nil || ctx.ASN4

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
        return &AtomicAggregate{}, nil
    case AttrAggregator:
        return ParseAggregator(data, fourByteAS)
    case AttrCommunity:
        return ParseCommunities(data)
    case AttrClusterList:
        return ParseClusterList(data)
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
        // Unknown attribute - return as opaque
        return NewOpaqueAttribute(code, data), nil
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
// VALIDATES: Check existence without parsing value.
// PREVENTS: Parsing overhead for existence check.
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
```

## Checklist

- [ ] Tests written for Get() (success + not found)
- [ ] Tests written for Get() (error cases)
- [ ] Tests written for Has()
- [ ] Tests written for GetMultiple()
- [ ] Tests written for PackFor() (same context)
- [ ] Tests written for PackFor() (different context)
- [ ] Tests written for All()
- [ ] Tests written for concurrent access
- [ ] Tests written for index reuse
- [ ] Tests FAIL before implementation
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes

## Dependencies

- `pkg/bgp/context/` (EncodingContext, ContextID, Registry) - EXISTS
- `pkg/bgp/attribute/` (Attribute interface, ParseHeader, Parse* functions) - EXISTS

## Dependents

- `spec-route-id-forwarding.md` - Route uses AttributesWire
- `spec-api-attribute-filter.md` - API uses GetMultiple()

## Future Work

- `spec-pool-handle-migration.md` - Transition `packed []byte` to `pool.Handle`
  for memory deduplication across routes

---

**Created:** 2026-01-01
**Updated:** 2026-01-01 - Added thread safety, error handling, offset caching
