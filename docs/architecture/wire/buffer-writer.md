# Buffer Writer Architecture

## Problem

Current code uses `append()` extensively:
- Each append may check capacity, reallocate, copy
- Temporary slices created and discarded
- GC pressure from short-lived allocations
- Cache unfriendly allocation patterns

## Solution: Fixed Session Buffer + Writer Interface

### Core Concept

```
┌─────────────────────────────────────────────────┐
│ Session Buffer (allocated once at session start) │
│ Size: MaxBGPMessageSize (4096 or 65535 extended) │
│                                                   │
│  ┌─────┬────────┬──────────┬──────┬─────────┐   │
│  │ Hdr │ Attrs  │ MP_REACH │ NLRI │ (unused)│   │
│  └─────┴────────┴──────────┴──────┴─────────┘   │
│  0     19       off1       off2    off3     cap  │
└─────────────────────────────────────────────────┘
```

- One buffer per peer session, allocated at connection setup
- Never reallocated during session lifetime
- All message building uses `copy()` into this buffer
- Buffer reset (offset = 0) between messages

### WireWriter Interface

```go
// WireWriter is in internal/component/bgp/context/context.go (not wire package due to import cycle).
// Implemented by all wire types (Message, Attribute, NLRI).
// Uses EncodingContext for capability-dependent encoding (ASN4, ADD-PATH, etc.)
type WireWriter interface {
    // Len returns wire size in bytes.
    // Pass nil for context-independent types.
    Len(ctx *EncodingContext) int

    // WriteTo writes the wire representation into buf starting at offset.
    // Returns the number of bytes written.
    // Caller guarantees buf has sufficient capacity.
    // Pass nil for context-independent types.
    WriteTo(buf []byte, off int, ctx *EncodingContext) int
}
```

**Note:** Attribute interface has separate methods (`Len()`, `WriteTo()`, `WriteToWithContext()`) rather than embedding WireWriter directly. See `internal/component/bgp/attribute/attribute.go`.

All wire types implement WireWriter:
- `Attribute` types (Origin, ASPath, NextHop, etc.)
- `NLRI` types (IPv4, IPv6, VPN, EVPN, etc.)
- `Message` types (Open, Update, Notification, Keepalive)

### Example Implementation

```go
// Context-independent (most types) - ignore context
func (o Origin) Len(_ *context.EncodingContext) int {
    return 1
}

func (o Origin) WriteTo(buf []byte, off int, _ *context.EncodingContext) int {
    buf[off] = byte(o)
    return 1
}

// Context-dependent (AS_PATH) - use context for ASN4
func (p *ASPath) Len(ctx *context.EncodingContext) int {
    if ctx == nil || ctx.ASN4() {
        return p.len4byte()
    }
    return p.len2byte()
}

func (p *ASPath) WriteTo(buf []byte, off int, ctx *context.EncodingContext) int {
    if ctx == nil || ctx.ASN4() {
        return p.writeTo4byte(buf, off)
    }
    return p.writeTo2byte(buf, off)
}
```

### Session Buffer Management

```go
type SessionBuffer struct {
    buf    []byte  // Fixed allocation
    offset int     // Current write position
}

func NewSessionBuffer(extended bool) *SessionBuffer {
    size := 4096
    if extended {
        size = 65535
    }
    return &SessionBuffer{
        buf: make([]byte, size),
    }
}

func (sb *SessionBuffer) Reset() {
    sb.offset = 0
}

func (sb *SessionBuffer) Write(w WireWriter, ctx *context.EncodingContext) {
    sb.offset += w.WriteTo(sb.buf, sb.offset, ctx)
}

func (sb *SessionBuffer) Bytes() []byte {
    return sb.buf[:sb.offset]
}
```

### Building UPDATE Message

```go
func (sb *SessionBuffer) BuildUpdate(attrs []Attribute, nlris []NLRI, ctx *context.EncodingContext) []byte {
    sb.Reset()

    // Skip header (19 bytes) - fill later
    sb.offset = 19

    // Withdrawn routes length placeholder
    withdrawnStart := sb.offset
    sb.offset += 2

    // (write withdrawals here)
    withdrawnLen := sb.offset - withdrawnStart - 2

    // Path attributes length placeholder
    attrsLenPos := sb.offset
    sb.offset += 2
    attrsStart := sb.offset

    // Write attributes directly with encoding context
    for _, attr := range attrs {
        sb.offset += writeAttrTo(attr, sb.buf, sb.offset, ctx)
    }
    attrsLen := sb.offset - attrsStart

    // Write NLRI directly with encoding context
    packCtx := ctx.ToPackContext(family)
    for _, nlri := range nlris {
        sb.offset += nlri.WriteTo(sb.buf, sb.offset, packCtx)
    }

    // Fill in lengths
    binary.BigEndian.PutUint16(sb.buf[withdrawnStart:], uint16(withdrawnLen))
    binary.BigEndian.PutUint16(sb.buf[attrsLenPos:], uint16(attrsLen))

    // Fill BGP header
    sb.writeHeader(TypeUpdate)

    return sb.buf[:sb.offset]
}
```

### Benefits

| Aspect | Current | New |
|--------|---------|-----|
| Allocations per UPDATE | 10-50+ | 0 |
| GC pressure | High | None |
| Cache locality | Poor | Excellent |
| Copy operations | append checks + copy | copy only |
| Buffer reuse | None | 100% |

### Migration Path

1. Add `WireWriter` interface with `Len(ctx)` and `WriteTo(buf, off, ctx)`
2. Update Message interface to embed WireWriter (remove Pack)
3. Update Attribute interface to embed WireWriter (remove Pack, PackWithContext)
4. Implement `Len(ctx)` and `WriteTo(buf, off, ctx)` on all wire types
5. Update callers to use EncodingContext instead of message.Negotiated
6. Delete message.Negotiated (ephemeral conversion shim)

### No Backwards Compatibility

Ze has never been released. No backwards compatibility needed:
- Change interface directly
- Fix all implementations
- Fix all callers
- Delete old methods

---

## CheckedWriteTo: Safe Path with Error Returns

### Motivation

`WriteTo()` assumes the caller guarantees buffer capacity. While efficient, this can lead to:
- Buffer overflows if capacity calculation is wrong
- Undefined behavior on undersized buffers
- Silent data corruption

`CheckedWriteTo()` provides a safe path with explicit error handling:

```go
// CheckedBufWriter (internal/component/bgp/wire/writer.go)
type CheckedBufWriter interface {
    BufWriter
    CheckedWriteTo(buf []byte, off int) (int, error)
    Len() int
}
```

**Note:** The actual interface is `CheckedBufWriter` (context-free), not context-dependent.

### Error Types

```go
// internal/component/bgp/wire/errors.go
var ErrBufferTooSmall = errors.New("wire: buffer too small")
```

### Implementation Pattern

```go
// CheckedWriteTo validates capacity before delegating to WriteTo.
func (x *Foo) CheckedWriteTo(buf []byte, off int) (int, error) {
    needed := x.Len()
    if len(buf) < off+needed {
        return 0, wire.ErrBufferTooSmall
    }
    return x.WriteTo(buf, off), nil
}

// WriteTo unchanged - caller guarantees capacity.
func (x *Foo) WriteTo(buf []byte, off int) int {
    buf[off] = x.value
    return 1
}
```

**Note:** For context-dependent types (AS_PATH, Aggregator), use the context-aware methods in the Attribute interface.

### Usage Guidelines

| Use Case | Method | Rationale |
|----------|--------|-----------|
| Internal iteration | `WriteTo()` | Capacity pre-validated at top level |
| Entry points | `CheckedWriteTo()` | Validates before committing to write |
| Performance critical | `WriteTo()` | Skip redundant checks |
| Defensive code | `CheckedWriteTo()` | Safety over speed |

### Composite Types

Composite types validate total capacity once, then use `WriteTo()` internally:

```go
func (c *Composite) CheckedWriteTo(buf []byte, off int) (int, error) {
    needed := c.Len()  // Sum of all children
    if len(buf) < off+needed {
        return 0, wire.ErrBufferTooSmall
    }
    return c.WriteTo(buf, off), nil
}

func (c *Composite) WriteTo(buf []byte, off int) int {
    pos := off
    pos += c.child1.WriteTo(buf, pos)  // Unchecked - capacity guaranteed
    pos += c.child2.WriteTo(buf, pos)
    return pos - off
}
```

### Context-Dependent Attributes

Some attribute types have different wire lengths depending on encoding context (e.g., ASN4 capability).
These use the Attribute interface's context-aware methods (`LenWithContext`, `WriteToWithContext`):

```go
// Aggregator: 8 bytes with ASN4, 6 bytes with legacy 2-byte ASN
func (a *Aggregator) LenWithContext(srcCtx, dstCtx *context.EncodingContext) int {
    if dstCtx == nil || dstCtx.ASN4() {
        return 8
    }
    return 6
}
```

**Types with context-dependent lengths:**
- `Aggregator` - 6 or 8 bytes based on ASN4
- `ASPath` - 2 or 4 bytes per ASN based on ASN4
- NLRI types - vary based on ADD-PATH context (path-id added/stripped)

### WireNLRI Zero-Allocation Pattern

`WireNLRI` adapts raw bytes for ADD-PATH context. The pattern ensures no allocation in `WriteTo`:

```go
// LenWithContext calculates length without allocation
func (w *WireNLRI) LenWithContext(ctx *PackContext) int {
    targetAddPath := ctx != nil && ctx.AddPath
    if w.hasAddPath && !targetAddPath {
        return len(w.data) - 4  // Strip path-id
    }
    if !w.hasAddPath && targetAddPath {
        return len(w.data) + 4  // Add path-id
    }
    return len(w.data)
}

// WriteTo writes directly without allocation
func (w *WireNLRI) WriteTo(buf []byte, off int, ctx *PackContext) int {
    // ... writes directly to buf
}

// Pack allocates and calls WriteTo (for convenience)
func (w *WireNLRI) Pack(ctx *PackContext) []byte {
    buf := make([]byte, w.LenWithContext(ctx))
    w.WriteTo(buf, 0, ctx)
    return buf
}
```

---

**Last Updated: 2026-01-30
