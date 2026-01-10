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

### Writer Interface

```go
// BufWriter writes directly into a pre-allocated buffer.
// Returns number of bytes written.
// Caller guarantees buf has sufficient capacity.
type BufWriter interface {
    // WriteTo writes the wire representation into buf starting at offset.
    // Returns the number of bytes written.
    WriteTo(buf []byte, offset int) int
}
```

All wire types implement this:
- `Attribute` types (Origin, ASPath, NextHop, etc.)
- `NLRI` types (IPv4, IPv6, VPN, EVPN, etc.)
- `Message` types (Open, Update, Notification, Keepalive)

### Example Implementation

```go
// Current (allocates)
func (o Origin) Pack() []byte {
    return []byte{byte(o)}
}

// New (zero-alloc)
func (o Origin) WriteTo(buf []byte, off int) int {
    buf[off] = byte(o)
    return 1
}

// Attribute with header
func (o Origin) WriteAttrTo(buf []byte, off int) int {
    // Header: flags (1) + code (1) + len (1) = 3 bytes
    buf[off] = 0x40   // Transitive, Well-known
    buf[off+1] = 1    // ORIGIN type code
    buf[off+2] = 1    // Length = 1
    buf[off+3] = byte(o)
    return 4
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

func (sb *SessionBuffer) Write(w BufWriter) {
    sb.offset += w.WriteTo(sb.buf, sb.offset)
}

func (sb *SessionBuffer) Bytes() []byte {
    return sb.buf[:sb.offset]
}
```

### Building UPDATE Message

```go
func (sb *SessionBuffer) BuildUpdate(attrs []Attribute, nlris []NLRI) []byte {
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

    // Write attributes directly
    for _, attr := range attrs {
        sb.offset += attr.WriteAttrTo(sb.buf, sb.offset)
    }
    attrsLen := sb.offset - attrsStart

    // Write NLRI directly
    for _, nlri := range nlris {
        sb.offset += nlri.WriteTo(sb.buf, sb.offset)
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

1. Add `BufWriter` interface alongside existing `Pack()` methods
2. Implement `WriteTo()` on all wire types
3. Add `SessionBuffer` to peer session
4. Convert message builders to use `SessionBuffer`
5. Remove old `Pack()` methods once migration complete

### Compatibility

- Interface is additive - old code continues to work
- Can migrate incrementally per message type
- Tests can verify old vs new produce identical bytes
