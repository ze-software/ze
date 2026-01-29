# Buffer-First Architecture

**Status:** Target Architecture (all development should follow this pattern)
**Date:** 2026-01-10

> **See also:** `docs/architecture/core-design.md` for the canonical architecture reference
> covering WireUpdate, RIB storage model, factory pattern, and type consolidation.

## Implementation Progress

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1 | ✅ Done | Core iterator types (`NLRIIterator`, `AttrIterator`, `ASPathIterator`) |
| Phase 2 | ✅ Done | WireUpdate integration (iterator methods) |
| Phase 3 | ✅ Done | Direct formatting functions (FormatPrefixFromBytes, FormatASPathJSON, etc.) |
| Phase 4 | ✅ Done | RIB migration (Route.AttrIterator, Route.ASPathIterator) |
| Phase 5 | ✅ Done | Deprecate parsed types (PathAttributes, RouteUpdate, UpdateInfo) |
| Phase 6 | ⚠️ Partial | RouteJSON, Builder done; PathAttributes removal deferred (see `spec-pathattributes-removal.md`) |

See `docs/plan/spec-buffer-first-migration.md` for detailed implementation plan.

---

## Executive Summary

Ze uses a **buffer-first** architecture where BGP messages are represented as byte buffers with iterators and partial parsers. This eliminates duplication between wire format and parsed representations, enables zero-copy operations, and provides a single source of truth.

**Core principle:** One representation (bytes). Everything else is views/iterators.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Message Buffer                          │
│  ┌──────────┬───────────┬──────────────┬─────────────────┐  │
│  │  Header  │ Withdrawn │  Attributes  │      NLRI       │  │
│  │ 19 bytes │   (var)   │    (var)     │     (var)       │  │
│  └──────────┴───────────┴──────────────┴─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
            ┌─────────────────┼─────────────────┐
            ▼                 ▼                 ▼
      ┌───────────┐    ┌───────────┐     ┌───────────┐
      │ AttrIter  │    │ NLRIIter  │     │ASPathIter │
      │ (offset)  │    │ (offset)  │     │ (offset)  │
      └───────────┘    └───────────┘     └───────────┘
            │                 │                 │
            ▼                 ▼                 ▼
      ┌───────────┐    ┌───────────┐     ┌───────────┐
      │  Accessor │    │  Accessor │     │  Accessor │
      │ (no alloc)│    │ (no alloc)│     │ (no alloc)│
      └───────────┘    └───────────┘     └───────────┘
```

---

## Design Principles

### 1. Bytes Are the Source of Truth

- Store wire bytes, not parsed structs
- Parse on demand via iterators
- Never duplicate data in different representations

### 2. Iterators Instead of Slices

```go
// ❌ OLD: Allocates slice
func (u *Update) NLRIs() []nlri.NLRI

// ✅ NEW: Iterator, zero allocation
func (u *Update) NLRIIterator() *NLRIIterator

type NLRIIterator struct {
    data    []byte
    offset  int
    family  Family
    addPath bool
}

func (it *NLRIIterator) Next() (prefix []byte, pathID uint32, ok bool)
```

### 3. Partial Parsers (Stateless Functions)

```go
// Parse only what you need, where you need it
func ParseNLRI(buf []byte, off int, addPath bool) (prefix []byte, pathID uint32, nextOff int, err error)
func ParseASPathSegment(buf []byte, off int) (segType uint8, asns []byte, nextOff int, err error)

// Iterators return []byte views directly - no intermediate Span type
```

### 4. Context-Aware Parsing

Parsing depends on negotiated capabilities (ADD-PATH, ASN4):

```go
type ParseContext struct {
    AddPath bool   // NLRI includes 4-byte path-id
    ASN4    bool   // AS numbers are 4 bytes (not 2)
}

func (it *NLRIIterator) WithContext(ctx ParseContext) *NLRIIterator
func (it *ASPathIterator) WithContext(ctx ParseContext) *ASPathIterator
```

### 5. Direct Formatting (No Intermediate Structs)

```go
// ❌ OLD: Parse to struct, then marshal
attrs := parseAttributes(buf)
json.Marshal(attrs)

// ✅ NEW: Format directly from buffer
func FormatAttributesJSON(buf []byte, ctx ParseContext, w io.Writer) error {
    iter := NewAttrIterator(buf)
    for typeCode, value, ok := iter.Next(); ok; typeCode, value, ok = iter.Next() {
        switch typeCode {
        case ORIGIN:
            fmt.Fprintf(w, `"origin":%d`, value[0])
        case AS_PATH:
            formatASPathJSON(value, ctx, w)
        // ...
        }
    }
}
```

---

## Component Design

### Message Buffer

The core type wrapping raw BGP message bytes:

```go
// WireUpdate wraps UPDATE message payload (after BGP header)
type WireUpdate struct {
    payload     []byte
    sourceCtxID ContextID  // For zero-copy decisions
    messageID   uint64     // Unique identifier
}

// Existing section accessors (return raw bytes)
func (u *WireUpdate) Withdrawn() ([]byte, error)
func (u *WireUpdate) Attrs() (*AttributesWire, error)
func (u *WireUpdate) NLRI() ([]byte, error)

// Iterator accessors (Phase 2 - implemented)
func (u *WireUpdate) WithdrawnIterator(addPath bool) (*nlri.NLRIIterator, error)
func (u *WireUpdate) AttrIterator() (*attribute.AttrIterator, error)
func (u *WireUpdate) NLRIIterator(addPath bool) (*nlri.NLRIIterator, error)
```

### Attribute Iterator

```go
type AttrIterator struct {
    data   []byte
    offset int
}

func NewAttrIterator(data []byte) *AttrIterator

// Next returns the next attribute
// Returns (0, 0, nil, false) when exhausted
func (it *AttrIterator) Next() (typeCode uint8, flags uint8, value []byte, ok bool)

// Convenience: find specific attribute
func (it *AttrIterator) Find(typeCode uint8) ([]byte, bool)
```

### NLRI Iterator

```go
type NLRIIterator struct {
    data    []byte
    offset  int
    addPath bool
}

func NewNLRIIterator(data []byte, addPath bool) *NLRIIterator

// Next returns next NLRI
// prefix is a view into the buffer (not a copy)
// pathID is 0 if addPath is false
// Returns (nil, 0, false) when exhausted
func (it *NLRIIterator) Next() (prefix []byte, pathID uint32, ok bool)
```

### AS-PATH Iterator

```go
type ASPathIterator struct {
    data   []byte
    offset int
    asn4   bool
}

func NewASPathIterator(data []byte, asn4 bool) *ASPathIterator

// Next returns next segment
// asns is a view into buffer (raw bytes, 2 or 4 bytes per ASN)
// Returns (0, nil, false) when exhausted
func (it *ASPathIterator) Next() (segType uint8, asns []byte, ok bool)

// Convenience: iterate ASNs within current segment
func (it *ASPathIterator) ASNIterator(asns []byte) *ASNIterator
```

### Update Builder (For Creating Messages)

```go
type UpdateBuilder struct {
    buf       []byte
    attrsOff  int
    nlriOff   int
    ctx       BuildContext
}

type BuildContext struct {
    ASN4      bool
    AddPath   bool
    MaxSize   int  // 4096 or 65535
}

func NewUpdateBuilder(ctx BuildContext) *UpdateBuilder

// Attribute writers
func (b *UpdateBuilder) WriteOrigin(origin uint8) error
func (b *UpdateBuilder) WriteASPath(segments []ASPathSegment) error
func (b *UpdateBuilder) WriteNextHop(addr netip.Addr) error
func (b *UpdateBuilder) WriteMED(med uint32) error
func (b *UpdateBuilder) WriteLocalPref(pref uint32) error
func (b *UpdateBuilder) WriteCommunities(comms []uint32) error

// NLRI writers
func (b *UpdateBuilder) WriteNLRI(prefix netip.Prefix, pathID uint32) error
func (b *UpdateBuilder) WriteWithdrawn(prefix netip.Prefix, pathID uint32) error

// Finalize
func (b *UpdateBuilder) Build() ([]byte, error)
func (b *UpdateBuilder) Reset()
```

---

## RIB Storage

Routes store wire bytes as source of truth:

```go
type Route struct {
    // Wire bytes (source of truth)
    attrBytes     []byte
    nlriBytes     []byte
    sourceCtxID   ContextID

    // Cached offsets (optional optimization)
    asPathOffset  int16  // -1 = not cached, else offset into attrBytes
    nextHopOffset int16  // -1 = not cached

    // Reference counting
    refCount      atomic.Int32
}

// Access via iterators - parse on demand
func (r *Route) AttrIterator() *AttrIterator {
    return NewAttrIterator(r.attrBytes)
}

func (r *Route) ASPathIterator(asn4 bool) *ASPathIterator {
    // Find AS_PATH attribute
    iter := r.AttrIterator()
    for typeCode, _, value, ok := iter.Next(); ok; typeCode, _, value, ok = iter.Next() {
        if typeCode == AS_PATH_TYPE {
            return NewASPathIterator(value, asn4)
        }
    }
    return nil
}

// Zero-copy forwarding
func (r *Route) CanForwardDirect(destCtxID ContextID) bool {
    return r.sourceCtxID == destCtxID
}

func (r *Route) AttrBytes() []byte {
    return r.attrBytes
}
```

---

## API Layer

### JSON Formatting (Direct from Buffer)

```go
// Format UPDATE event directly to JSON writer
func FormatUpdateEventJSON(u *WireUpdate, ctx ParseContext, w io.Writer) error {
    w.Write([]byte(`{"type":"update"`))

    // Announce section
    w.Write([]byte(`,"announce":{`))
    nlriIter := u.NLRIIterator(ctx)
    first := true
    for prefix, pathID, ok := nlriIter.Next(); ok; prefix, pathID, ok = nlriIter.Next() {
        if !first {
            w.Write([]byte(`,`))
        }
        formatPrefixJSON(prefix, pathID, w)
        first = false
    }
    w.Write([]byte(`}`))

    // Attributes
    w.Write([]byte(`,"attributes":{`))
    iter, _ := u.AttrIterator()
    formatAttributesJSON(iter, ctx, w)
    w.Write([]byte(`}}`))

    return nil
}
```

### Text Formatting

```go
func FormatUpdateText(u *WireUpdate, ctx ParseContext, w io.Writer) error {
    // "update text as-path set [65001 65002] nhop set 192.168.1.1 nlri ipv4/unicast add 10.0.0.0/24"
    // Format directly from buffer bytes
}
```

---

## Migration Path

### Phase 1: Add Iterators (Non-Breaking)

Add iterator types alongside existing slice-returning methods:

```go
// Keep existing (deprecated)
func (u *Update) NLRIs() []nlri.NLRI

// Add new
func (u *Update) NLRIIterator() *NLRIIterator
```

### Phase 2: Migrate Internal Code

Update internal consumers to use iterators:
- RIB storage
- Route forwarding
- UPDATE building

### Phase 3: Migrate API Layer

Update API formatting to use direct buffer access:
- JSON encoder
- Text encoder

### Phase 4: Remove Parsed Types

Once all consumers migrated:
- Remove `PathAttributes` struct
- Remove `RouteUpdate` struct
- Remove slice-returning methods

---

## What Gets Removed

| Current Type | Replacement |
|--------------|-------------|
| `plugin.PathAttributes` | `AttrIterator` over buffer |
| `plugin.RouteUpdate` | Direct formatting from buffer |
| `[]attribute.Attribute` | `AttrIterator` |
| `[]nlri.NLRI` | `NLRIIterator` |
| `[]uint32` (AS-PATH) | `ASPathIterator` |
| `rr.UpdateInfo` | `WireUpdate` + iterators |
| `plugin.RawMessage` | Simplified to buffer ref |

---

## Benefits

| Benefit | Description |
|---------|-------------|
| **Zero-copy passthrough** | Route reflection = memcpy of buffer |
| **Single source of truth** | No sync between wire/parsed representations |
| **Parse on demand** | Only parse attributes API actually needs |
| **Memory efficient** | No slice allocations for AS-PATH, communities |
| **Consistent** | API and wire code use identical primitives |
| **Simpler code** | One way to do things, not three |

---

## Guidelines for New Code

1. **Never store parsed slices** - Store wire bytes, iterate on demand
2. **Never return slices from iterators** - Return views (subslices) or format directly
3. **Use builders for construction** - `UpdateBuilder` for new messages
4. **Pass ParseContext** - Context-dependent parsing (ADD-PATH, ASN4)
5. **Format directly to Writer** - No intermediate JSON structs

---

## Related Documentation

- `docs/architecture/encoding-context.md` - Context-dependent encoding
- `docs/architecture/update-building.md` - Wire format construction
- `docs/architecture/rib-transition.md` - RIB ownership model
- `docs/plan/spec-buffer-first-migration.md` - Implementation spec

---

**Last Updated:** 2026-01-10
