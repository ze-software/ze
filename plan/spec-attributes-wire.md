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
    packed      []byte                         // Canonical wire bytes
    sourceCtxID bgpctx.ContextID               // Encoding context
    parsed      map[AttributeCode]Attribute    // Lazy cache (sparse)
}
```

## API

```go
// Constructor
func NewAttributesWire(packed []byte, ctxID bgpctx.ContextID) *AttributesWire

// Zero-copy access
func (a *AttributesWire) Packed() []byte

// Lazy single-attribute access (parse on demand, cache result)
func (a *AttributesWire) Get(code AttributeCode) (Attribute, bool)

// Check existence without parsing value
func (a *AttributesWire) Has(code AttributeCode) bool

// Parse subset (for API output)
func (a *AttributesWire) GetMultiple(codes []AttributeCode) map[AttributeCode]Attribute

// Full parse (when needed)
func (a *AttributesWire) All() []Attribute

// Context-aware forwarding
func (a *AttributesWire) PackFor(destCtxID bgpctx.ContextID) []byte
```

## Implementation

### File: `pkg/bgp/attribute/wire.go`

```go
package attribute

import (
    bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
)

// AttributesWire stores path attributes in wire format with lazy parsing.
//
// Wire bytes are the canonical representation. Parsed attributes are
// cached on demand and discarded when no longer needed.
type AttributesWire struct {
    packed      []byte
    sourceCtxID bgpctx.ContextID
    parsed      map[AttributeCode]Attribute
}

// NewAttributesWire creates from raw packed bytes.
func NewAttributesWire(packed []byte, ctxID bgpctx.ContextID) *AttributesWire {
    return &AttributesWire{
        packed:      packed,
        sourceCtxID: ctxID,
    }
}

// Packed returns raw wire bytes for transmission.
func (a *AttributesWire) Packed() []byte {
    return a.packed
}

// Get returns a specific attribute by code (lazy parse).
func (a *AttributesWire) Get(code AttributeCode) (Attribute, bool) {
    // Check cache first
    if a.parsed != nil {
        if attr, ok := a.parsed[code]; ok {
            return attr, true
        }
    }

    // Scan wire bytes for this attribute
    attr, found := a.parseOne(code)
    if found {
        if a.parsed == nil {
            a.parsed = make(map[AttributeCode]Attribute)
        }
        a.parsed[code] = attr
    }
    return attr, found
}

// Has checks if attribute exists without parsing value.
func (a *AttributesWire) Has(code AttributeCode) bool {
    // Check cache first
    if a.parsed != nil {
        if _, ok := a.parsed[code]; ok {
            return true
        }
    }

    // Scan headers only
    offset := 0
    for offset < len(a.packed) {
        _, attrCode, length, hdrLen, err := ParseHeader(a.packed[offset:])
        if err != nil {
            return false
        }
        if attrCode == code {
            return true
        }
        offset += hdrLen + int(length)
    }
    return false
}

// GetMultiple returns multiple attributes (for API output).
func (a *AttributesWire) GetMultiple(codes []AttributeCode) map[AttributeCode]Attribute {
    result := make(map[AttributeCode]Attribute, len(codes))
    for _, code := range codes {
        if attr, ok := a.Get(code); ok {
            result[code] = attr
        }
    }
    return result
}

// All returns all attributes (full parse).
func (a *AttributesWire) All() []Attribute {
    // ... parse all attributes from wire
}

// PackFor returns packed bytes for destination context.
// Zero-copy if contexts match, otherwise re-encode.
func (a *AttributesWire) PackFor(destCtxID bgpctx.ContextID) []byte {
    if a.sourceCtxID == destCtxID {
        return a.packed
    }
    // Slow path: re-encode with destination context
    destCtx := bgpctx.Registry.Get(destCtxID)
    return a.packWithContext(destCtx)
}

// parseOne scans for and parses a single attribute.
func (a *AttributesWire) parseOne(code AttributeCode) (Attribute, bool) {
    // ... scan and parse single attribute
}

// packWithContext re-encodes all attributes with destination context.
func (a *AttributesWire) packWithContext(ctx *bgpctx.EncodingContext) []byte {
    // ... full parse and re-encode
}
```

## Test Plan

```go
// TestAttributesWireGet verifies lazy single-attribute parsing.
// VALIDATES: Only requested attribute is parsed, cached for reuse.
// PREVENTS: Full parse on single attribute access.
func TestAttributesWireGet(t *testing.T)

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

// TestAttributesWireAll verifies full parse.
// VALIDATES: All attributes returned in order.
// PREVENTS: Missing or duplicated attributes.
func TestAttributesWireAll(t *testing.T)
```

## Checklist

- [ ] Tests written for Get()
- [ ] Tests written for Has()
- [ ] Tests written for GetMultiple()
- [ ] Tests written for PackFor()
- [ ] Tests written for All()
- [ ] Tests FAIL before implementation
- [ ] Implementation makes tests pass
- [ ] `make test` passes
- [ ] `make lint` passes

## Dependencies

- `pkg/bgp/context/` (EncodingContext, ContextID, Registry) - EXISTS
- `pkg/bgp/attribute/` (Attribute interface, ParseHeader) - EXISTS

## Dependents

- `spec-route-id-forwarding.md` - Route uses AttributesWire
- `spec-api-attribute-filter.md` - API uses GetMultiple()

---

**Created:** 2026-01-01
