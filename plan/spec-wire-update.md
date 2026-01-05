# Spec: Wire Update Types

## Status: Phase 1 & 2 Complete

API-only mode. No pool, no RIB. Concrete wire types, no interface indirection.

## Task

Implementation plan: `/Users/thomas/.claude/plans/sleepy-drifting-bachman.md`

## Required Reading

- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - Zero-copy pattern, ContextID comparison
- [x] `.claude/zebgp/UPDATE_BUILDING.md` - Forward path vs Build path (WireUpdate is Forward path)
- [x] `pkg/bgp/attribute/wire.go` - AttributesWire pattern to follow
- [x] `pkg/api/mpwire.go` - MPReachWire/MPUnreachWire already implemented

**Key insights:**
- Zero-copy rule: `sourceCtxID == destCtxID` → return original bytes
- WireUpdate is receive-side (Forward path), not for building UPDATEs
- AttributesWire already exists with lazy parsing + sync.Once pattern
- MPReachWire/MPUnreachWire are `[]byte` type aliases with accessor methods

---

## Overview

Parse BGP UPDATE messages using concrete wire types. Direct method calls, no interface overhead. API receives updates, handles storage/resend on session restart. GC manages memory.

## Memory Model

```
┌─────────────────────────────────────────────────────────────────────────┐
│  SIMPLE: Allocate per read, GC frees when done                          │
│                                                                         │
│  buf := make([]byte, msgLen) ──► conn.Read(buf) ──► WireUpdate{buf}    │
│                                                           │             │
│                                                     API callback        │
│                                                           │             │
│                                                     GC frees when       │
│                                                     unreferenced        │
└─────────────────────────────────────────────────────────────────────────┘
```

No pool. No interning. No reference counting. No interfaces.

---

## RFC 4271 UPDATE Format

```
UPDATE Message (Section 4.3):
┌────────────────────────────────────────────────────────┐
│  Withdrawn Routes Length      (2 octets)               │
├────────────────────────────────────────────────────────┤
│  Withdrawn Routes             (variable)               │
├────────────────────────────────────────────────────────┤
│  Total Path Attribute Length  (2 octets)               │
├────────────────────────────────────────────────────────┤
│  Path Attributes              (variable)               │
├────────────────────────────────────────────────────────┤
│  Network Layer Reachability Information (variable)     │
└────────────────────────────────────────────────────────┘
```

All positions derivable from the two length fields.

---

## Concrete Types

### WireUpdate

```go
// WireUpdate holds UPDATE bytes. GC manages lifetime.
// All methods return concrete types, no interface indirection.
// Thread-safe for concurrent read access.
type WireUpdate struct {
    payload     []byte
    sourceCtxID ContextID

    // Cached AttributesWire - lazily initialized, thread-safe
    attrsOnce sync.Once
    attrs     *attribute.AttributesWire
}

func NewWireUpdate(payload []byte, ctxID ContextID) *WireUpdate

func (u *WireUpdate) Withdrawn() []byte          // Returns nil if empty/malformed
func (u *WireUpdate) Attrs() *AttributesWire     // Cached - same instance per WireUpdate
func (u *WireUpdate) NLRI() []byte               // Returns nil if empty/malformed
func (u *WireUpdate) MPReach() MPReachWire       // Uses cached Attrs()
func (u *WireUpdate) MPUnreach() MPUnreachWire   // Uses cached Attrs()
func (u *WireUpdate) SourceCtxID() ContextID
func (u *WireUpdate) Payload() []byte
```

### AttributesWire

```go
// AttributesWire wraps path attribute bytes.
// Concrete type, no interface.
type AttributesWire struct {
    packed      []byte
    sourceCtxID ContextID
    index       []attrIndex
    parsed      map[AttributeCode]Attribute
    mu          sync.RWMutex
}

func (a *AttributesWire) GetRaw(code AttributeCode) ([]byte, bool)
func (a *AttributesWire) Get(code AttributeCode) (Attribute, error)
func (a *AttributesWire) Has(code AttributeCode) bool
func (a *AttributesWire) Packed() []byte
func (a *AttributesWire) Codes() []AttributeCode
```

### MPReachWire (RFC 4760)

```go
// MPReachWire wraps MP_REACH_NLRI attribute (code 14)
// Format: [AFI:2][SAFI:1][NH_Len:1][NextHop:var][Reserved:1][NLRI:var]
// Type alias for []byte with accessor methods.
type MPReachWire []byte

func (m MPReachWire) AFI() uint16
func (m MPReachWire) SAFI() uint8
func (m MPReachWire) Family() nlri.Family
func (m MPReachWire) NextHop() netip.Addr
func (m MPReachWire) NLRI() []byte
```

### MPUnreachWire (RFC 4760)

```go
// MPUnreachWire wraps MP_UNREACH_NLRI attribute (code 15)
// Format: [AFI:2][SAFI:1][Withdrawn:var]
// Type alias for []byte with accessor methods.
type MPUnreachWire []byte

func (m MPUnreachWire) AFI() uint16
func (m MPUnreachWire) SAFI() uint8
func (m MPUnreachWire) Family() nlri.Family
func (m MPUnreachWire) Withdrawn() []byte
```

---

## Implementation

### Derived Accessors

Use `uint32` for offset calculations to avoid overflow with extended messages (RFC 8654).

```go
// deriveWithdrawn extracts Withdrawn Routes from UPDATE payload
func deriveWithdrawn(buf []byte) []byte {
    if len(buf) < 2 {
        return nil
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    if wdLen == 0 {
        return nil
    }
    if uint32(len(buf)) < 2+wdLen {
        return nil
    }
    return buf[2 : 2+wdLen]
}

// deriveAttrs extracts Path Attributes as *AttributesWire
func deriveAttrs(buf []byte, ctxID ContextID) *AttributesWire {
    if len(buf) < 2 {
        return nil
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    attrLenOffset := 2 + wdLen
    if uint32(len(buf)) < attrLenOffset+2 {
        return nil
    }
    attrLen := uint32(binary.BigEndian.Uint16(buf[attrLenOffset:]))
    if attrLen == 0 {
        return nil
    }
    attrStart := attrLenOffset + 2
    if uint32(len(buf)) < attrStart+attrLen {
        return nil
    }
    return NewAttributesWire(buf[attrStart:attrStart+attrLen], ctxID)
}

// deriveNLRI extracts NLRI from UPDATE payload
func deriveNLRI(buf []byte) []byte {
    if len(buf) < 2 {
        return nil
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    attrLenOffset := 2 + wdLen
    if uint32(len(buf)) < attrLenOffset+2 {
        return nil
    }
    attrLen := uint32(binary.BigEndian.Uint16(buf[attrLenOffset:]))
    nlriStart := attrLenOffset + 2 + attrLen
    if nlriStart >= uint32(len(buf)) {
        return nil
    }
    return buf[nlriStart:]
}

// deriveMPReach extracts MP_REACH_NLRI as *MPReachWire
func deriveMPReach(buf []byte, ctxID ContextID) *MPReachWire {
    attrs := deriveAttrs(buf, ctxID)
    if attrs == nil {
        return nil
    }
    raw, ok := attrs.GetRaw(14)  // MP_REACH_NLRI
    if !ok || len(raw) < 5 {
        return nil
    }
    return &MPReachWire{raw: raw}
}

// deriveMPUnreach extracts MP_UNREACH_NLRI as *MPUnreachWire
func deriveMPUnreach(buf []byte, ctxID ContextID) *MPUnreachWire {
    attrs := deriveAttrs(buf, ctxID)
    if attrs == nil {
        return nil
    }
    raw, ok := attrs.GetRaw(15)  // MP_UNREACH_NLRI
    if !ok || len(raw) < 3 {
        return nil
    }
    return &MPUnreachWire{raw: raw}
}
```

### MPReachWire Methods

```go
func (m *MPReachWire) AFI() uint16 {
    if len(m.raw) < 2 {
        return 0
    }
    return binary.BigEndian.Uint16(m.raw[0:2])
}

func (m *MPReachWire) SAFI() uint8 {
    if len(m.raw) < 3 {
        return 0
    }
    return m.raw[2]
}

func (m *MPReachWire) NextHop() []byte {
    if len(m.raw) < 4 {
        return nil
    }
    nhLen := m.raw[3]
    if len(m.raw) < 4+int(nhLen) {
        return nil
    }
    return m.raw[4 : 4+nhLen]
}

func (m *MPReachWire) NLRI() []byte {
    if len(m.raw) < 4 {
        return nil
    }
    nhLen := m.raw[3]
    // Skip: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(nhLen) + Reserved(1)
    nlriStart := 4 + int(nhLen) + 1
    if nlriStart >= len(m.raw) {
        return nil
    }
    return m.raw[nlriStart:]
}

func (m *MPReachWire) Raw() []byte { return m.raw }
```

### MPUnreachWire Methods

```go
func (m *MPUnreachWire) AFI() uint16 {
    if len(m.raw) < 2 {
        return 0
    }
    return binary.BigEndian.Uint16(m.raw[0:2])
}

func (m *MPUnreachWire) SAFI() uint8 {
    if len(m.raw) < 3 {
        return 0
    }
    return m.raw[2]
}

func (m *MPUnreachWire) Withdrawn() []byte {
    if len(m.raw) < 3 {
        return nil
    }
    return m.raw[3:]
}

func (m *MPUnreachWire) Raw() []byte { return m.raw }
```

---

## Usage

### Receiving Updates

```go
func (c *Connection) readUpdate() (*WireUpdate, error) {
    // Allocate buffer for this message
    buf := make([]byte, msgLen)
    _, err := io.ReadFull(c.conn, buf)
    if err != nil {
        return nil, err
    }
    return NewWireUpdate(buf, c.ctxID), nil
}
```

### Processing Updates

```go
func (peer *Peer) handleUpdate(update *WireUpdate) {
    // Process withdrawals
    if wd := update.Withdrawn(); wd != nil {
        // Parse IPv4 withdrawn prefixes from wd
    }
    if mpu := update.MPUnreach(); mpu != nil {
        // Parse MP withdrawn from mpu.Withdrawn()
    }

    // Process announcements
    if nlri := update.NLRI(); nlri != nil {
        attrs := update.Attrs()
        // Parse IPv4 prefixes, pass to API with attrs
    }
    if mpr := update.MPReach(); mpr != nil {
        attrs := update.Attrs()
        // Parse MP NLRI, pass to API with attrs
    }

    // Pass to API callback
    peer.api.OnUpdate(update)

    // When function returns, GC can free update if API doesn't hold reference
}
```

---

## Checklist

### Phase 1: Wire Types
- [x] Implement `WireUpdate` struct and methods (`pkg/api/wire_update.go`)
- [x] Implement `MPReachWire` struct and methods (already in `pkg/api/mpwire.go`)
- [x] Implement `MPUnreachWire` struct and methods (already in `pkg/api/mpwire.go`)
- [x] Implement derived accessors with bounds checking
- [x] Ensure `AttributesWire` exists (already in `pkg/bgp/attribute/wire.go`)
- [x] Unit tests: `pkg/api/wire_update_test.go` (7 tests pass)

### Phase 2: Integration
- [x] Update connection read loop to create `*WireUpdate` (reactor.go:notifyMessageReceiver)
- [x] Add `WireUpdate` field to `RawMessage` (types.go)
- [x] Derive `AttrsWire` from `WireUpdate.Attrs()` for backward compat
- [x] Cache `AttributesWire` in `WireUpdate` with `sync.Once` for efficiency
- [x] Integration test: `TestNotifyMessageReceiverWireUpdate`
- [x] Remove redundant `ExtractAttributeBytes` (replaced by `WireUpdate.Attrs()`)
- [x] Tests pass (existing tests + WireUpdate tests)

### Phase 3: ReceivedUpdate Migration (Future)
- [ ] Update `ReceivedUpdate` struct to use `*WireUpdate` instead of `RawBytes + Attrs`
- [ ] Update `ReceivedUpdate.ConvertToRoutes()` to use `WireUpdate`
- [ ] Update cache operations to use `WireUpdate`

---

## What's NOT in This Spec

| Feature | Reason |
|---------|--------|
| Interfaces | Concrete types only, no indirection |
| Pool | No deduplication needed, GC handles memory |
| RIB | API manages route state |
| Reference counting | GC handles lifetime |
| Compaction | No pool, no fragmentation |
| Mode selection | Always API mode |

---

**Created:** 2025-01-03
