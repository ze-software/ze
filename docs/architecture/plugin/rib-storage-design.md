# Plugin RIB Storage Design

**Status:** Design Reference for API Programs
**Location:** Derived from `plan/learned/059-spec-pool-handle-migration.md`

This document describes the pool-based RIB storage design for API programs (plugins).
The Ze engine does NOT implement this - it belongs in API programs like `ze plugin bgp-rs`.

**See also:**
- `docs/architecture/rib-transition.md` - Architecture overview
- `plan/spec-plugin-rs.md` - Route Server plugin spec
- `docs/architecture/pool-architecture.md` - Pool system design

---

## Overview

This is the **implementation reference** for Pool + Wire design in API programs:
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool struct -->
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle type -->
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate struct -->

| This Spec Implements | Design Goal |
|---------------------|-------------|
| `Update` interface | Unified access to UPDATE parts |
| `WireUpdate` / `PooledUpdate` | Mode-specific storage |
| Pool with `Intern()` / `Get()` | Memory deduplication (RIB mode) |
| RIB keyed by attribute handle | Efficient route grouping |

### Supersedes

| Spec | Status |
|------|--------|
| `buildRIBRouteUpdate` conversion | **SKIP** - pool forwarding replaces it |

### Enables

| Spec | When |
|------|------|
| `spec-unified-handle-nlri.md` | After Phase 2 |
| Zero-copy forwarding | After Phase 3 |

---

## Memory Management Design

### Mode-Specific Read Strategies

Different buffer strategies optimize each mode:

```
┌─────────────────────────────────────────────────────────────────────────┐
│  RIB MODE - Reusable buffer + Intern                                    │
│                                                                         │
│  conn.Read(reusableBuf) ──► pool.Intern(buf[:n]) ──► reuse buffer      │
│                                    │                                    │
│                              Intern COPIES into pool                    │
│                              1 copy total                               │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  API MODE - Allocate per read, no copy                                  │
│                                                                         │
│  buf := make([]byte, msgLen) ──► conn.Read(buf) ──► WireUpdate owns buf│
│                                                           │             │
│                                                     0 copies            │
│                                                     GC frees when done  │
└─────────────────────────────────────────────────────────────────────────┘
```

### RIB Mode Implementation

```go
type Connection struct {
    conn net.Conn
    buf  []byte  // Reusable buffer (65535 bytes)
}

func (c *Connection) readUpdateRIB() (*PooledUpdate, error) {
    n, err := c.conn.Read(c.buf)
    if err != nil {
        return nil, err
    }
    // Intern copies into pool, buf can be reused immediately
    return NewPooledUpdate(c.buf[:n], c.ctxID), nil
}
```

### API Mode Implementation

```go
func (c *Connection) readUpdateAPI() (*WireUpdate, error) {
    // Allocate fresh buffer for each message
    buf := make([]byte, msgLen)
    _, err := io.ReadFull(c.conn, buf)
    if err != nil {
        return nil, err
    }
    // WireUpdate takes ownership, no copy needed
    return &WireUpdate{payload: buf, sourceCtxID: c.ctxID}, nil
}
```

### Key Principles

1. **RIB mode:** Reusable connection buffer
   - `pool.Intern()` copies into pool buffer
   - Connection buffer reused immediately
   - 1 copy per message

2. **API mode:** Allocate per read
   - Fresh buffer each message
   - WireUpdate takes ownership directly
   - 0 copies per message (just allocation)
   - GC frees when WireUpdate unreferenced

3. **Pool owns its data** - `Intern()` copies bytes into pool's internal buffer
   - Handle points to pool's copy, NOT original
   - No dangling references

### Summary

| Mode | Allocation | Copies | Buffer Strategy |
|------|------------|--------|-----------------|
| RIB (PooledUpdate) | 0 per msg | 1 (into pool) | Reusable buffer |
| API (WireUpdate) | 1 per msg | 0 | Allocate per read |

Both modes efficient. RIB optimizes for memory dedup. API optimizes for simplicity.
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate (API mode) -->

---

## Overview

Migrate from direct `[]byte` storage to single `pool.Handle` reference.
Route stores ONE handle to the UPDATE payload; all parts (withdrawn, attrs,
NLRI) are derived on access. CPU derives offsets; storage is minimal.

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
<!-- source: internal/component/bgp/wireu/wire_update.go -- ensureParsed, derived accessors -->

## Current State

### Design Doc
- `POOL_ARCHITECTURE.md` describes the pool system design
- Double-buffer with MSB handles, incremental compaction

### Current Code
```go
// Route stores wire bytes directly (to be changed)
type Route struct {
    wireBytes     []byte           // Direct ownership
    nlriWireBytes []byte           // Direct ownership
    sourceCtxID   ContextID
}

// AttributesWire references external buffer - UNCHANGED
// (per spec-attributes-wire.md, zero-copy reference)
type AttributesWire struct {
    packed      []byte             // NOT owned, refs message buffer
    sourceCtxID ContextID
    // ...
}
```

## Target State

### Two Modes

| Mode | When | Storage | Intern |
|------|------|---------|--------|
| **API-controlled** | Adj-RIB disabled | Raw wire bytes | Never |
| **RIB-enabled** | Adj-RIB on any peer | Pool handle | On RIB insert |

Both expose the same `Update` interface. API mode uses wire bytes directly;
RIB mode uses pooled handle. On session restart in API mode, API resends routes.

### Single Global Pool (RIB Mode)

```go
// Global pool shared across all peers (only used when RIB enabled)
// Stores: attribute bytes, NLRI bytes (for large NLRIs)
// Deduplication: same bytes = same handle across all peers
var Pool = pool.NewPool(PoolConfig{
    InitialBufferSize: 1 << 20,  // 1MB
    ExpectedEntries:   100000,
})

// Convenience accessors (all use the same pool)
var Attrs = Pool   // Attribute bytes
var NLRIs = Pool   // NLRI bytes (IPv6+, VPN, etc.)
```

One pool. Shared across peers. Deduplication works globally.

### Attributes Interface

```go
// Attributes provides access to path attributes.
// Implemented by AttributesWire.
type Attributes interface {
    GetRaw(code AttributeCode) ([]byte, bool)
    Get(code AttributeCode) (Attribute, error)
    Has(code AttributeCode) bool
    Packed() []byte
    Codes() []AttributeCode
}
```

### MP Interfaces (RFC 4760)

```go
// MPReach wraps MP_REACH_NLRI attribute (code 14)
// Format: [AFI:2][SAFI:1][NH_Len:1][NextHop:var][Reserved:1][NLRI:var]
type MPReach interface {
    AFI() uint16
    SAFI() uint8
    NextHop() []byte
    NLRI() []byte
    Raw() []byte  // Full attribute bytes
}

// MPUnreach wraps MP_UNREACH_NLRI attribute (code 15)
// Format: [AFI:2][SAFI:1][Withdrawn:var]
type MPUnreach interface {
    AFI() uint16
    SAFI() uint8
    Withdrawn() []byte
    Raw() []byte  // Full attribute bytes
}
```

### Update Interface

```go
// Update provides access to UPDATE message parts.
// Same interface for both wire and pooled backing.
type Update interface {
    Attrs() Attributes      // Path attributes (interface, not concrete type)
    NLRI() []byte           // IPv4 unicast NLRI (message body)
    Withdrawn() []byte      // IPv4 unicast withdrawn (message body)
    MPReach() MPReach       // MP_REACH_NLRI attr (nil if not present)
    MPUnreach() MPUnreach   // MP_UNREACH_NLRI attr (nil if not present)
    SourceCtxID() ContextID
    Release()
}
```

### WireUpdate (API Mode)

```go
// WireUpdate holds raw UPDATE bytes. No pool interaction.
// Used when Adj-RIB disabled, API controls route lifecycle.
type WireUpdate struct {
    payload     []byte      // Owned buffer (allocated per read, no copy)
    sourceCtxID ContextID
}

func (u *WireUpdate) Release() {} // No-op, GC handles it
```

### PooledUpdate (RIB Mode)

```go
// PooledUpdate references decomposed UPDATE components.
// Used when Adj-RIB enabled for deduplication.
// Reconstructed from RIB data, not stored directly.
type PooledUpdate struct {
    attrHandle  pool.Handle   // Interned attrs in global pool
    nlriSet     NLRISet       // Contains family internally
    sourceCtxID ContextID
}

func (u *PooledUpdate) Release() {
    // Note: RIB owns the attrHandle ref, not PooledUpdate
    // This is called when done iterating, not for refcount
}
```

**Storage per attr set in RIB:** 4 bytes (handle) + NLRISet overhead

### Derived Accessors (shared implementation)

Both `WireUpdate` and `PooledUpdate` use these helpers on their payload.

**Note:** Use `uint32` for offset calculations to avoid overflow with extended messages (RFC 8654).

```go
// deriveWithdrawn extracts Withdrawn Routes from UPDATE payload
func deriveWithdrawn(buf []byte) []byte {
    if len(buf) < 2 {
        return nil  // Malformed: too short for withdrawn length
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    if wdLen == 0 {
        return nil
    }
    if uint32(len(buf)) < 2+wdLen {
        return nil  // Malformed: withdrawn length exceeds buffer
    }
    return buf[2 : 2+wdLen]
}

// deriveAttrs extracts Path Attributes as Attributes interface
func deriveAttrs(buf []byte, ctxID ContextID) Attributes {
    if len(buf) < 2 {
        return nil  // Malformed: too short for withdrawn length
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    attrLenOffset := 2 + wdLen
    if uint32(len(buf)) < attrLenOffset+2 {
        return nil  // Malformed: too short for attr length
    }
    attrLen := uint32(binary.BigEndian.Uint16(buf[attrLenOffset:]))
    if attrLen == 0 {
        return nil
    }
    attrStart := attrLenOffset + 2
    if uint32(len(buf)) < attrStart+attrLen {
        return nil  // Malformed: attr length exceeds buffer
    }
    return NewAttributesWire(buf[attrStart:attrStart+attrLen], ctxID)
}

// deriveNLRI extracts NLRI from UPDATE payload
func deriveNLRI(buf []byte) []byte {
    if len(buf) < 2 {
        return nil  // Malformed: too short for withdrawn length
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    attrLenOffset := 2 + wdLen
    if uint32(len(buf)) < attrLenOffset+2 {
        return nil  // Malformed: too short for attr length
    }
    attrLen := uint32(binary.BigEndian.Uint16(buf[attrLenOffset:]))
    nlriStart := attrLenOffset + 2 + attrLen
    if nlriStart >= uint32(len(buf)) {
        return nil  // No NLRI present
    }
    return buf[nlriStart:]
}

// MP-BGP types (RFC 4760)

type mpReachWire struct {
    raw []byte
}

func (m *mpReachWire) AFI() uint16      { return binary.BigEndian.Uint16(m.raw[0:2]) }
func (m *mpReachWire) SAFI() uint8      { return m.raw[2] }
func (m *mpReachWire) NextHop() []byte  {
    nhLen := m.raw[3]
    return m.raw[4 : 4+nhLen]
}
func (m *mpReachWire) NLRI() []byte {
    nhLen := m.raw[3]
    // Skip: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(nhLen) + Reserved(1)
    return m.raw[4+nhLen+1:]
}
func (m *mpReachWire) Raw() []byte { return m.raw }

type mpUnreachWire struct {
    raw []byte
}

func (m *mpUnreachWire) AFI() uint16       { return binary.BigEndian.Uint16(m.raw[0:2]) }
func (m *mpUnreachWire) SAFI() uint8       { return m.raw[2] }
func (m *mpUnreachWire) Withdrawn() []byte { return m.raw[3:] }
func (m *mpUnreachWire) Raw() []byte       { return m.raw }

func deriveMPReach(buf []byte, ctxID ContextID) MPReach {
    attrs := deriveAttrs(buf, ctxID)
    if attrs == nil {
        return nil
    }
    raw, ok := attrs.GetRaw(14)  // MP_REACH_NLRI
    if !ok || len(raw) < 5 {
        return nil
    }
    return &mpReachWire{raw: raw}
}

func deriveMPUnreach(buf []byte, ctxID ContextID) MPUnreach {
    attrs := deriveAttrs(buf, ctxID)
    if attrs == nil {
        return nil
    }
    raw, ok := attrs.GetRaw(15)  // MP_UNREACH_NLRI
    if !ok || len(raw) < 3 {
        return nil
    }
    return &mpUnreachWire{raw: raw}
}
```

### WireUpdate Methods

```go
func (u *WireUpdate) Withdrawn() []byte      { return deriveWithdrawn(u.payload) }
func (u *WireUpdate) Attrs() Attributes      { return deriveAttrs(u.payload, u.sourceCtxID) }
func (u *WireUpdate) NLRI() []byte           { return deriveNLRI(u.payload) }
func (u *WireUpdate) MPReach() MPReach       { return deriveMPReach(u.payload, u.sourceCtxID) }
func (u *WireUpdate) MPUnreach() MPUnreach   { return deriveMPUnreach(u.payload, u.sourceCtxID) }
func (u *WireUpdate) SourceCtxID() ContextID { return u.sourceCtxID }
func (u *WireUpdate) Release()               {} // No-op
```

### PooledUpdate Methods

```go
// PooledUpdate is reconstructed from RIB data, not derived from wire bytes.
// It provides access to decomposed components stored in RIB.

func (u *PooledUpdate) Attrs() Attributes {
    attrBytes := pool.Attrs.Get(u.attrHandle)
    return NewAttributesWire(attrBytes, u.sourceCtxID)
}

func (u *PooledUpdate) NLRI() []byte {
    // For IPv4 unicast, return concatenated NLRIs from DirectNLRISet
    if direct, ok := u.nlriSet.(*DirectNLRISet); ok {
        return direct.data
    }
    return nil  // Non-IPv4 uses MPReach, or caller iterates nlriSet
}

func (u *PooledUpdate) Withdrawn() []byte {
    return nil  // RIB doesn't store withdrawals - they're processed immediately
}

func (u *PooledUpdate) MPReach() MPReach {
    // MP_REACH_NLRI is stored in attrs, extract it
    attrs := u.Attrs()
    if attrs == nil {
        return nil
    }
    raw, ok := attrs.GetRaw(14)
    if !ok || len(raw) < 5 {
        return nil
    }
    return &mpReachWire{raw: raw}
}

func (u *PooledUpdate) MPUnreach() MPUnreach {
    return nil  // Unreachable routes not stored in RIB
}

func (u *PooledUpdate) SourceCtxID() ContextID { return u.sourceCtxID }

func (u *PooledUpdate) Release() {
    // No-op: RIB owns the attrHandle reference
}
```

### AttributesWire UNCHANGED

```go
// Wraps path attribute bytes - NO POOL interaction
// Implements Attributes interface
// References either:
//   - GC-managed copy (during processing)
//   - Pooled buffer slice (from PooledUpdate.Attrs())
type AttributesWire struct {
    packed      []byte             // Refs external buffer
    sourceCtxID ContextID
    index       []attrIndex
    parsed      map[AttributeCode]Attribute
    mu          sync.RWMutex
}
```

---

## RIB Storage Model

An UPDATE contains multiple NLRIs sharing the same attributes. RIB stores routes keyed by attributes, with NLRI lists as values.

### Address Families

BGP supports many AFI/SAFI combinations:

| AFI | SAFI | Family | NLRI Size |
|-----|------|--------|-----------|
| 1 | 1 | IPv4 Unicast | 1-5 bytes |
| 1 | 2 | IPv4 Multicast | 1-5 bytes |
| 1 | 128 | VPNv4 | 12+ bytes |
| 2 | 1 | IPv6 Unicast | 2-18 bytes |
| 2 | 2 | IPv6 Multicast | 2-18 bytes |
| 2 | 128 | VPNv6 | 24+ bytes |
| 1 | 133 | FlowSpec IPv4 | Variable |
| 2 | 133 | FlowSpec IPv6 | Variable |
| 25 | 65 | L2VPN/EVPN | Variable |

### Storage Strategy

| NLRI Size | Storage | Reason |
|-----------|---------|--------|
| ≤ 4 bytes | Wire format `[]byte` | Smaller than handle overhead |
| > 4 bytes | `pool.Handle` | Handle (4 bytes) saves space |

```go
// Family-specific threshold
func shouldPoolNLRI(afi uint16, safi uint8) bool {
    switch {
    case afi == 1 && safi == 1:   // IPv4 Unicast
        return false              // 1-5 bytes, inline is fine
    case afi == 1 && safi == 2:   // IPv4 Multicast
        return false              // 1-5 bytes
    default:
        return true               // All others: pool
    }
}
```

### RIB Structure

```go
// FamilyKey identifies an address family
type FamilyKey struct {
    AFI  uint16
    SAFI uint8
}

// PeerRIB is the Adj-RIB-In for one peer
// Each peer has its own RIB, but all share the global pool
type PeerRIB struct {
    peerID   PeerID
    mu       sync.RWMutex
    families map[FamilyKey]*FamilyRIB
}

// FamilyRIB stores routes for one AFI/SAFI
type FamilyRIB struct {
    afi      uint16
    safi     uint8
    poolNLRI bool  // true if NLRI stored as handles
    addPath  bool  // true if ADD-PATH enabled for this family

    // Forward index: attribute handle → NLRI set
    entries map[pool.Handle]NLRISet

    // Reverse index: prefix → attribute handle (for lookups/withdrawals)
    // Key is NLRI bytes (includes path-id if ADD-PATH enabled)
    prefixIndex map[string]pool.Handle
}
```

### ADD-PATH Support (RFC 7911)

When ADD-PATH is negotiated, path ID is part of prefix identity:

```
Without ADD-PATH:
  [prefix-len:1][prefix-bytes:0-4]

With ADD-PATH:
  [path-id:4][prefix-len:1][prefix-bytes:0-4]

The path-id + prefix together form the unique key.
Same IP prefix with different path-ids = different routes.
```

See `DirectNLRISet.nlriLen()` for parsing implementation.

### Reverse Index

For efficient prefix lookup and withdrawal:

```go
// Insert adds NLRI to RIB, managing pool refcount internally.
// Caller passes raw attr bytes, RIB manages all pool references.
// Key invariant: each unique attrHandle in r.entries has exactly ONE pool ref.
func (r *FamilyRIB) Insert(attrBytes []byte, nlri []byte) {
    h := pool.Attrs.Intern(attrBytes)  // refcount++ (1 for new, +1 for existing)

    // Check reverse index for implicit withdraw
    nlriKey := string(nlri)  // NLRI bytes as key (includes path-id if ADD-PATH)
    if oldHandle, exists := r.prefixIndex[nlriKey]; exists {
        if oldHandle != h {
            // Implicit withdraw: remove from old attr set
            r.removeFromSet(oldHandle, nlri)
        } else {
            // Same prefix, same attrs → no-op (route refresh)
            pool.Attrs.Release(h)  // Undo Intern's ref
            return
        }
    }

    // Add to forward index
    set, exists := r.entries[h]
    if exists {
        pool.Attrs.Release(h)  // Already own a ref for this attr set
    } else {
        // New attr set in RIB - keep the ref from Intern()
        set = NewNLRISet(r.family, r.addPath)
        r.entries[h] = set
    }
    set.Add(nlri)

    // Update reverse index
    r.prefixIndex[nlriKey] = h
}

// Lookup by prefix
func (r *FamilyRIB) Lookup(nlri []byte) (pool.Handle, bool) {
    handle, exists := r.prefixIndex[string(nlri)]
    return handle, exists
}

// Remove removes NLRI from RIB, releasing pool ref when attr set empty.
func (r *FamilyRIB) Remove(nlri []byte) bool {
    nlriKey := string(nlri)
    h, exists := r.prefixIndex[nlriKey]
    if !exists {
        return false
    }

    set := r.entries[h]
    set.Remove(nlri)
    delete(r.prefixIndex, nlriKey)

    // Last NLRI removed → release RIB's ref to attrs
    if set.Len() == 0 {
        set.Release()              // Release NLRI handles if pooled
        delete(r.entries, h)
        pool.Attrs.Release(h)      // Release RIB's single ref
    }
    return true
}
```

### NLRISet Interface

```go
// NLRISet stores NLRIs for one attribute set
type NLRISet interface {
    // Add appends NLRI to the set
    Add(nlri []byte)

    // Remove removes NLRI from the set, returns true if found
    Remove(nlri []byte) bool

    // Contains checks if NLRI exists
    Contains(nlri []byte) bool

    // Iterate calls fn for each NLRI (wire bytes)
    Iterate(fn func(nlri []byte) bool)

    // Len returns number of NLRIs
    Len() int

    // Release frees any pool handles (no-op for direct)
    Release()
}
```

### DirectNLRISet (IPv4 Unicast/Multicast)

```go
// DirectNLRISet stores small NLRIs as concatenated wire bytes
// Used for IPv4 where NLRI (1-5 bytes) < handle overhead
type DirectNLRISet struct {
    data    []byte     // Concatenated wire-format NLRIs
    count   int        // Number of NLRIs (avoid re-parsing for Len())
    family  FamilyKey  // AFI + SAFI
    addPath bool       // If true, NLRIs have 4-byte path-id prefix
}

// nlriLen returns the wire length of an NLRI at offset
func (s *DirectNLRISet) nlriLen(offset int) int {
    if offset >= len(s.data) {
        return 0
    }
    if s.addPath {
        // ADD-PATH: [path-id:4][prefix-len:1][prefix-bytes]
        if offset+4 >= len(s.data) {
            return 0
        }
        prefixLen := s.data[offset+4]
        return 4 + 1 + (int(prefixLen)+7)/8
    }
    // Standard: [prefix-len:1][prefix-bytes]
    prefixLen := s.data[offset]
    return 1 + (int(prefixLen)+7)/8
}

func (s *DirectNLRISet) Add(nlri []byte) {
    s.data = append(s.data, nlri...)
    s.count++
}

func (s *DirectNLRISet) Remove(nlri []byte) bool {
    offset := 0
    for offset < len(s.data) {
        length := s.nlriLen(offset)
        if length == 0 {
            break
        }
        if bytes.Equal(s.data[offset:offset+length], nlri) {
            // Found - remove by shifting remaining data
            copy(s.data[offset:], s.data[offset+length:])
            s.data = s.data[:len(s.data)-length]
            s.count--
            return true
        }
        offset += length
    }
    return false
}

func (s *DirectNLRISet) Contains(nlri []byte) bool {
    offset := 0
    for offset < len(s.data) {
        length := s.nlriLen(offset)
        if length == 0 {
            break
        }
        if bytes.Equal(s.data[offset:offset+length], nlri) {
            return true
        }
        offset += length
    }
    return false
}

func (s *DirectNLRISet) Iterate(fn func(nlri []byte) bool) {
    offset := 0
    for offset < len(s.data) {
        length := s.nlriLen(offset)
        if length == 0 {
            break
        }
        if !fn(s.data[offset : offset+length]) {
            return
        }
        offset += length
    }
}

func (s *DirectNLRISet) Len() int {
    return s.count
}

func (s *DirectNLRISet) Release() {} // No-op, GC handles data
```

### PooledNLRISet (IPv6, VPN, EVPN, FlowSpec)

```go
// PooledNLRISet stores large NLRIs as pool handles
// Used for IPv6+ where NLRI > handle size (4 bytes)
type PooledNLRISet struct {
    handles []pool.Handle
    family  FamilyKey  // AFI + SAFI
    addPath bool       // If true, NLRIs have 4-byte path-id prefix
}

func (s *PooledNLRISet) Add(nlri []byte) {
    h := pool.NLRIs.Intern(nlri)
    s.handles = append(s.handles, h)
}

func (s *PooledNLRISet) Remove(nlri []byte) bool {
    // Lookup without modifying refcount
    targetHandle, exists := pool.NLRIs.Lookup(nlri)
    if !exists {
        return false
    }

    for i, h := range s.handles {
        if h == targetHandle {
            // Found - remove by swap with last
            pool.NLRIs.Release(h)  // Release our ref
            s.handles[i] = s.handles[len(s.handles)-1]
            s.handles = s.handles[:len(s.handles)-1]
            return true
        }
    }
    return false
}

func (s *PooledNLRISet) Contains(nlri []byte) bool {
    // Lookup without modifying refcount
    targetHandle, exists := pool.NLRIs.Lookup(nlri)
    if !exists {
        return false
    }

    for _, h := range s.handles {
        if h == targetHandle {
            return true
        }
    }
    return false
}

func (s *PooledNLRISet) Iterate(fn func(nlri []byte) bool) {
    for _, h := range s.handles {
        if !fn(pool.NLRIs.Get(h)) {
            return
        }
    }
}

func (s *PooledNLRISet) Len() int {
    return len(s.handles)
}

func (s *PooledNLRISet) Release() {
    for _, h := range s.handles {
        pool.NLRIs.Release(h)
    }
    s.handles = nil
}
```

### Factory

```go
// NewNLRISet creates appropriate implementation for family
func NewNLRISet(family FamilyKey, addPath bool) NLRISet {
    if shouldPoolNLRI(family.AFI, family.SAFI) {
        return &PooledNLRISet{family: family, addPath: addPath}
    }
    return &DirectNLRISet{family: family, addPath: addPath}
}
```

### Why Key by Attributes?

1. **Deduplication:** Many routes share same AS_PATH, communities, etc.
2. **Efficient lookup:** Find all routes with same attributes
3. **UPDATE building:** Group routes by attrs for efficient encoding

### Stale Route Tracking (Graceful Restart)

RFC 4724 requires the Receiving Speaker to mark routes as stale when a GR-capable peer
drops, retain them during the restart window, and selectively purge only stale routes
when EOR arrives — preserving fresh routes received during the GR window.

**Design:** `Stale bool` on `RouteEntry` — per-route metadata, NOT a pooled attribute.
Stale state is not shared via dedup because each route instance has independent stale status.

| Operation | Method | Scope |
|-----------|--------|-------|
| Mark stale | `FamilyRIB.MarkStale()` | All routes in one family |
| Purge stale | `FamilyRIB.PurgeStale()` | Only stale routes in one family |
| Stale count | `FamilyRIB.StaleCount()` | Count stale routes |
| Implicit unstale | `FamilyRIB.Insert()` | New entry always has `Stale = false` |

`PeerRIB` provides aggregate methods: `MarkAllStale`, `PurgeFamilyStale`, `PurgeAllStale`, `StaleCount`.

**RIB commands:** `rib mark-stale <peer> <restart-time>` and `rib purge-stale <peer> [family]`.
Called by bgp-gr plugin via `DispatchCommand`. The RIB stores per-peer GR state
(`StaleAt`, `RestartTime`, `ExpiresAt`) and starts a safety-net expiry timer
(restart-time + 5s margin) that auto-purges stale routes if bgp-gr never sends
`purge-stale` or `release-routes`.

**Show output:** `rib show in` includes `"stale": true` on stale routes.
`rib status` includes aggregate `stale-routes` count and per-peer GR state with
`stale-at` and `expires-at` absolute times.

---

## Update → RIB Flow

The RIB decomposes incoming Updates, interns components, and can reconstruct pooled Updates.

### Decomposition and Interning

```
┌─────────────────────────────────────────────────────────────────────────┐
│  RECEIVE                                                                 │
│                                                                         │
│  conn.Read(buf) ──► Parse UPDATE ──► Temporary access to parts         │
│                          │                                              │
│                    [withdrawn][attrs][nlri][mp_reach][mp_unreach]       │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  INTERN INTO GLOBAL POOL                                                 │
│                                                                         │
│  attrs bytes ──► pool.Attrs.Intern() ──► attrHandle                    │
│                                              │                          │
│  (Same attrs from any peer = same handle)    │                          │
│                                              ▼                          │
│  nlri bytes ──► RIB.Insert(attrHandle, nlri)                           │
│                       │                                                 │
│                 ┌─────┴─────┐                                          │
│                 │           │                                          │
│           IPv4: direct   IPv6+: pool.NLRIs.Intern()                    │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  STORE IN PER-PEER RIB                                                  │
│                                                                         │
│  PeerRIB[family].entries[attrHandle] ──► NLRISet                       │
│  PeerRIB[family].prefixIndex[nlri] ──► attrHandle                      │
│                                                                         │
│  (Each peer has own RIB, but handles point to shared pool)              │
└─────────────────────────────────────────────────────────────────────────┘
```

### Reconstructing Update from Pool

When we need an Update interface from stored data, PooledUpdate is constructed
from RIB entries. See "PooledUpdate (RIB Mode)" section for struct definition
and "PooledUpdate Methods" section for accessor implementations.

**Note:** Family information is stored in NLRISet, not in PooledUpdate directly.

### Processing Flow Summary

```go
func (peer *Peer) processUpdate(buf []byte) error {
    // 1. Parse into temporary accessors
    wdLen := binary.BigEndian.Uint16(buf[0:2])
    // ... derive all parts
    attrBytes := buf[attrStart:attrEnd]

    // 2. Process withdrawals (no pool interaction)
    for each withdrawn nlri {
        peer.rib.Remove(family, nlri)
    }

    // 3. Store announcements - RIB manages pool refs internally
    for each announced nlri {
        peer.rib.Insert(family, attrBytes, nlri)  // RIB calls Intern()
    }

    // 4. Original buffer can be reused immediately
    return nil
}
```

**Key:** Caller never touches pool directly for RIB operations. RIB.Insert() takes raw bytes, handles all refcounting.

### Lifecycle Management

RIB manages pool handle lifecycle. See "Reverse Index" section above for full Insert/Remove implementations.

**Key invariant:** Each unique `attrHandle` in `r.entries` has exactly ONE pool reference, regardless of how many NLRIs share those attributes.

### Reference Counting Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│  INSERT (first NLRI for new attr set)                                    │
│                                                                         │
│  RIB.Insert(attrBytes, nlri)                                            │
│        │                                                                │
│        ├──► pool.Intern(attrBytes) ──► handle (refcount = 1)           │
│        │                                                                │
│        ├──► entries[handle] not found                                   │
│        │        └──► keep the ref, create new NLRISet                  │
│        │                                                                │
│        └──► set.Add(nlri), prefixIndex[nlri] = handle                  │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  INSERT (additional NLRI for existing attr set)                          │
│                                                                         │
│  RIB.Insert(attrBytes, nlri)                                            │
│        │                                                                │
│        ├──► pool.Intern(attrBytes) ──► same handle (refcount = 2)      │
│        │                                                                │
│        ├──► entries[handle] found                                       │
│        │        └──► pool.Release(handle) ──► refcount = 1             │
│        │                                                                │
│        └──► set.Add(nlri), prefixIndex[nlri] = handle                  │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  REMOVE (last NLRI for attr set)                                         │
│                                                                         │
│  RIB.Remove(nlri)                                                       │
│        │                                                                │
│        ├──► handle = prefixIndex[nlri]                                  │
│        ├──► set.Remove(nlri)                                            │
│        │                                                                │
│        └──► set.Len() == 0                                              │
│                  ├──► set.Release() (NLRI handles if pooled)            │
│                  ├──► delete(entries, handle)                           │
│                  └──► pool.Release(handle) ──► refcount = 0 ──► freed  │
└─────────────────────────────────────────────────────────────────────────┘
```

**RIB owns the lifecycle.** Callers pass bytes, never handles. Pool refcount matches RIB entries exactly.
<!-- source: internal/component/bgp/attrpool/pool.go -- Intern, Release refcounting -->

### Example: IPv4 Unicast

```
Received UPDATE:
  Attrs: AS_PATH=[65001], NEXT_HOP=10.0.0.1
  NLRI: [192.168.1.0/24, 192.168.2.0/24, 192.168.3.0/24]

RIB stores (IPv4 unicast, inline wire bytes):
  families[{1,1}].entries[attrHandle] = &DirectNLRISet{
      data:  [0x18 0xC0 0xA8 0x01][0x18 0xC0 0xA8 0x02][0x18 0xC0 0xA8 0x03]
      count: 3
  }
```

### Example: VPNv4

```
Received UPDATE:
  Attrs: AS_PATH=[65001], NEXT_HOP=10.0.0.1, RT=65001:100
  MP_REACH_NLRI: VPNv4 [RD:label:prefix, RD:label:prefix]

RIB stores (VPNv4, pooled handles):
  families[{1,128}].entries[attrHandle] = &PooledNLRISet{
      handles: [handle1, handle2]
  }
```

---

## Mode Selection

### Global Mode Decision

```go
// Mode is determined globally at startup and when peers change
type UpdateMode int

const (
    ModeWire   UpdateMode = iota  // WireUpdate - no pooling
    ModePooled                     // PooledUpdate - with pooling
)

func DetermineMode(peers []PeerConfig) UpdateMode {
    for _, p := range peers {
        if p.AdjRIBEnabled() {
            slog.Info("RIB enabled on peer, using pooled mode", "peer", p.Name)
            return ModePooled
        }
    }
    slog.Info("No peer has RIB enabled, using wire mode (API-controlled)")
    return ModeWire
}
```

### Behavior

| Condition | Mode | Log Message |
|-----------|------|-------------|
| No peer has Adj-RIB | `ModeWire` | "No peer has RIB enabled, using wire mode (API-controlled)" |
| Any peer has Adj-RIB | `ModePooled` | "RIB enabled on peer, using pooled mode" |

### Runtime Considerations

- Mode is set at startup
- If peer config changes (add peer with RIB), mode switches to `ModePooled`
- Once in `ModePooled`, stays there (no downgrade to `ModeWire`)

## Migration Phases

### Phase 1: Pool Core ✅

**File:** `internal/component/bgp/attrpool/pool.go`
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool, Intern, Get, Release -->
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle layout -->

> **Note:** The implementation uses a hybrid handle layout. See `docs/architecture/pool-architecture.md` for current design.

```go
package pool

type Handle uint32

// Handle layout: bufferBit(1) | poolIdx(5) | flags(2) | slot(24)
const InvalidHandle Handle = 0xFFFFFFFF  // poolIdx=31 (reserved)

// Core operations (all return errors for validation)
func (p *Pool) Intern(data []byte) Handle
func (p *Pool) InternWithError(data []byte) (Handle, error)
func (p *Pool) Get(h Handle) ([]byte, error)
func (p *Pool) AddRef(h Handle) error
func (p *Pool) Release(h Handle) error
func (p *Pool) Length(h Handle) (int, error)

// Normalized access (by slot index)
func (p *Pool) GetBySlot(slotIdx uint32) ([]byte, error)
func (p *Pool) ReleaseBySlot(slotIdx uint32) error
```

**Tests:**
- Intern returns same handle for identical data
- Get returns correct bytes
- AddRef/Release reference counting
- Compaction preserves data integrity
- Fuzz tests for handle encoding

### Phase 2: Global Pool

**File:** `internal/component/bgp/attrpool/global.go`

```go
// Global pool shared across all peers
// Stores attribute bytes and large NLRIs
// Deduplication: same bytes = same handle
var Pool = NewPool(PoolConfig{
    InitialBufferSize: 1 << 20,  // 1MB
    ExpectedEntries:   100000,
})

// Convenience aliases (same pool, for code clarity)
var Attrs = Pool
var NLRIs = Pool
```

Single pool. Shared globally. Deduplicates identical bytes across all peers.

### Phase 3: Update Types

**File:** `internal/component/bgp/message/update.go`

```go
// Update interface - same API for both modes
type Update interface {
    Attrs() Attributes
    NLRI() []byte
    Withdrawn() []byte
    MPReach() MPReach
    MPUnreach() MPUnreach
    SourceCtxID() ContextID
    Release()
}

// WireUpdate - API mode (no RIB)
// Takes ownership of payload buffer (allocated per read)
type WireUpdate struct {
    payload     []byte
    sourceCtxID ContextID
}

func NewWireUpdate(payload []byte, ctxID ContextID) *WireUpdate {
    // Takes ownership - caller allocated buffer, we own it now
    // No copy needed - buffer was allocated for this message
    return &WireUpdate{payload: payload, sourceCtxID: ctxID}
}

// PooledUpdate - RIB mode (reconstructed from RIB entries)
// Family info stored in nlriSet, not here
type PooledUpdate struct {
    attrHandle  pool.Handle   // Interned attrs
    nlriSet     NLRISet       // Contains family internally
    sourceCtxID ContextID
}

// Note: No NewPooledUpdate constructor - RIB constructs these
// from stored entries when iterating routes
```

### Phase 4: RIB Storage Model

**File:** `internal/component/bgp/rib/rib.go`

```go
// NLRISet interface with two implementations
type NLRISet interface {
    Add(nlri []byte)
    Remove(nlri []byte) bool
    Contains(nlri []byte) bool
    Iterate(fn func(nlri []byte) bool)
    Len() int
    Release()
}

// DirectNLRISet - IPv4 (wire bytes, no pool)
type DirectNLRISet struct {
    data    []byte
    count   int
    family  FamilyKey  // AFI + SAFI
    addPath bool
}

// PooledNLRISet - IPv6+, VPN, EVPN (pool handles)
type PooledNLRISet struct {
    handles []pool.Handle
    family  FamilyKey  // AFI + SAFI
    addPath bool
}

// FamilyRIB stores routes for one AFI/SAFI
type FamilyRIB struct {
    afi      uint16
    safi     uint8
    entries  map[pool.Handle]NLRISet  // attrHandle → NLRIs
}
```

### Phase 5: Compaction (Future/Optional)

**Status:** Not needed for MVP. Pool grows without reclaiming. Add when memory pressure matters.

**File:** `internal/component/bgp/attrpool/scheduler.go`
<!-- source: internal/component/bgp/attrpool/scheduler.go -- Scheduler, Run -->

#### The Problem: Fragmentation

```
Pool buffer over time with Release() calls:

Initial:  [entry1][entry2][entry3][entry4][entry5][...........]
                                                    ↑ free space

After releases:
          [entry1][freed][entry3][freed][entry5][...........]
                    ↑            ↑
                  holes - wasted space, cannot be reused
                  (entries are variable size)
```

Without compaction, pool grows indefinitely even if most entries are released.

#### The Solution: Double-Buffer Compaction

```
Buffer A (current):  [e1][--][e3][--][e5][--][e7]  ← fragmented
Buffer B (target):   [e1][e3][e5][e7][............]  ← compact

1. Copy live entries from A → B (incrementally)
2. Update slot offsets to point to B
3. Flip: B becomes current, A becomes target
4. A is now empty, ready for next compaction cycle
```

#### Handle Design Supports This

> **Note:** Current implementation uses hybrid layout. See `docs/architecture/pool-architecture.md`.

```
Handle layout: bufferBit(1) | poolIdx(5) | flags(2) | slot(24)

Handle 0x80000005 → bufferBit=1, poolIdx=0, flags=0, slot=5
Handle 0x00000005 → bufferBit=0, poolIdx=0, flags=0, slot=5
```

- Handle encodes buffer bit + pool index + flags + slot
- Slot stores offset in BOTH buffers
- During compaction, slot is valid in both buffers
- After compaction, flip the buffer bit for new allocations

#### Incremental Compaction

```go
type CompactionScheduler struct {
    pool          *Pool
    state         CompactionState
    cursor        uint32          // Current slot being migrated
    batchSize     int             // Slots per iteration
    idleThreshold time.Duration   // Wait for idle before compacting
}

type CompactionState int

const (
    Idle       CompactionState = iota
    Compacting
    Flipping
)

func (s *CompactionScheduler) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-time.After(s.idleThreshold):
            if s.pool.ShouldCompact() {
                s.compactBatch()
            }
        }
    }
}

func (s *CompactionScheduler) compactBatch() {
    // Move batchSize slots from old buffer to new
    // Pause if BGP activity detected
    // Resume on next idle period
}
```

#### When to Compact

| Trigger | Threshold |
|---------|-----------|
| Fragmentation ratio | > 30% of buffer is freed slots |
| Memory pressure | Pool size > configured limit |
| Idle time | No BGP activity for N seconds |

#### Activity-Based Pausing

```go
func (s *CompactionScheduler) compactBatch() {
    for i := 0; i < s.batchSize; i++ {
        if s.pool.HasRecentActivity() {
            // BGP traffic - pause compaction
            return
        }
        s.migrateSlot(s.cursor)
        s.cursor++
    }
}
```

Compaction yields to BGP processing - never blocks message handling.

#### MVP: Skip Compaction

For initial implementation:
- Pool grows without bound
- No compaction scheduler
- Acceptable for short-lived sessions or small route tables
- Add compaction when memory profiling shows need

```go
type CompactionScheduler struct {
    pool *Pool
    // ... per POOL_ARCHITECTURE.md
}

func (s *CompactionScheduler) Run(ctx context.Context)
```

**Integration:** Start scheduler in reactor initialization (when implemented).

## API Changes

### New: Update Interface

| Type | Mode | Storage |
|------|------|---------|
| `WireUpdate` | API-controlled (no RIB) | Owned `[]byte` copy |
| `PooledUpdate` | RIB-enabled | `pool.Handle` |

Both implement `Update` interface with same methods.

### Breaking Changes

| Before | After |
|--------|-------|
| Direct `[]byte` storage | `Update` interface |
| N/A | `Update.Release()` MUST be called (no-op for WireUpdate) |

### Unchanged (Zero-Copy Preserved)

| Component | Why Unchanged |
|-----------|---------------|
| `AttributesWire` | Keeps zero-copy reference (to wire or pooled buffer) |
| `NewAttributesWire()` | No pool interaction |
| `AttributesWire.Packed()` | Returns reference, not pool data |

### New Requirements

1. **Mode selection:** Use `WireUpdate` when Adj-RIB disabled, `PooledUpdate` when enabled
2. **Lifecycle management:** Always call `Release()` (safe no-op for WireUpdate)
3. **No modification:** Bytes from `Update.NLRI()` etc. MUST NOT be modified
4. **Thread safety:** Pool operations are thread-safe

## Memory Analysis

### Without Pool (Current)

| Routes | Memory per Route | Total |
|--------|------------------|-------|
| 1M | ~200 bytes (attrs+nlri copy) | ~200MB |

### With Pool (Target)

| Routes | Memory per Route | Total |
|--------|------------------|-------|
| 1M | 8 bytes (handle+ctxID) | 8MB + pooled data |

**Deduplication:** Identical UPDATE payloads share storage.
For route reflectors forwarding same UPDATE to many peers → significant savings.

## Checklist

### Phase 1: Pool Core ✅
- [x] Implement `internal/component/bgp/attrpool/pool.go`
- [x] Implement `internal/component/bgp/attrpool/handle.go`
- [x] Implement `pool.Lookup()` for read-only handle lookup
- [x] Tests for Intern/Get/Lookup/AddRef/Release
- [x] Tests for deduplication
- [x] Tests for concurrent access

### Phase 2: Global Pool
- [ ] Create `internal/component/bgp/attrpool/global.go` with single global pool
- [ ] Add `Attrs` and `NLRIs` aliases pointing to same pool
- [ ] Remove old separate pools if they exist

### Phase 3: Update Types
- [ ] Define `Attributes` interface
- [ ] Make `AttributesWire` implement `Attributes`
- [ ] Define `MPReach` and `MPUnreach` interfaces
- [ ] Implement `mpReachWire` and `mpUnreachWire` structs
- [ ] Define `Update` interface
- [ ] Implement `WireUpdate` (API mode)
- [ ] Implement `PooledUpdate` (RIB mode)
- [ ] Implement derived accessors: `Withdrawn()`, `Attrs()`, `NLRI()`
- [ ] Implement MP-BGP accessors: `MPReach()`, `MPUnreach()`
- [ ] Implement mode selection (`DetermineMode()`)
- [ ] Add logging for mode selection
- [ ] Tests for all types
- [ ] Update message processing to use Update interface

### Phase 4: RIB Storage Model
- [ ] Define `NLRISet` interface (with `family` field)
- [ ] Implement `DirectNLRISet` (IPv4: wire bytes)
- [ ] Implement `PooledNLRISet` (IPv6+: pool handles, uses `Lookup()`)
- [ ] Implement `NewNLRISet()` factory with ADD-PATH support
- [ ] Implement `FamilyKey`, `PeerRIB`, `FamilyRIB` types
- [ ] Implement reverse index (prefix → attrHandle)
- [ ] Implement RIB-bound refcounting (RIB.Insert takes bytes, not handles)
- [ ] Implement `shouldPoolNLRI()` for family-specific storage
- [ ] Implement implicit withdraw (same prefix, new attrs)
- [ ] Implement ADD-PATH aware NLRI parsing
- [ ] Implement Update → RIB decomposition flow
- [ ] Implement PooledUpdate reconstruction from RIB data
- [ ] Tests for all address families
- [ ] Tests for ADD-PATH scenarios
- [ ] Tests for refcount invariant (1 ref per entries key)

### Phase 5: Compaction (Future)
- [ ] Implement CompactionScheduler
- [ ] Integrate with reactor
- [ ] Test incremental compaction
- [ ] Test activity-based pausing

**Note:** AttributesWire is NOT modified - it remains a zero-copy view.

## Dependencies

- `POOL_ARCHITECTURE.md` - Design specification
- `spec-attributes-wire.md` - AttributesWire current design

## Risks

| Risk | Mitigation |
|------|------------|
| Forgetting Release() | Static analysis, runtime leak detection |
| Pool contention | Sharding if needed |
| Compaction latency | Incremental, pause on activity |
| Lower dedup ratio | Full UPDATE less likely to match than attrs alone |

## Deduplication Granularity

### Three Levels of Deduplication

| Level | What's Deduplicated | Storage/Route | Dedup Ratio | Status |
|-------|---------------------|---------------|-------------|--------|
| **1. UPDATE blob** | Entire UPDATE message | 8 bytes | Low (exact match only) | ❌ Not used |
| **2. Attribute blob** | All attrs as one blob | handle + NLRI | Medium (same attr set shared) | ✅ **Current** |
| **3. Per-attribute** | Each attr type separately | ~10 handles | High (partial sharing) | 📋 Planned |

### Current Implementation (Level 2)

```
Route A: ORIGIN=IGP, AS_PATH=[65001], LP=100, MED=0
Route B: ORIGIN=IGP, AS_PATH=[65001], LP=100, MED=50
         ↓
pool.Attributes.Intern(attrBytes)  → TWO different blobs (MED differs)
```

**Limitation:** Routes sharing ORIGIN, AS_PATH, LOCAL_PREF but differing in MED get NO sharing.

### Target Implementation (Level 3 - Phase 6)

```
Route A: ORIGIN=IGP, AS_PATH=[65001], LP=100, MED=0
Route B: ORIGIN=IGP, AS_PATH=[65001], LP=100, MED=50
         ↓
RouteEntry {
    Origin:    pool.Origin.Intern([IGP])      → SHARED (same handle)
    ASPath:    pool.ASPath.Intern([65001])    → SHARED (same handle)
    LocalPref: pool.LocalPref.Intern([100])   → SHARED (same handle)
    MED:       pool.MED.Intern([0 or 50])     → DIFFERENT handles
}
```

**Benefit:** 1M routes with same ORIGIN/LP but different MED → ~3 ORIGIN refs + ~100 LP refs vs 1M blobs.

### Memory Impact

| Scenario | Level 2 (blob) | Level 3 (per-attr) |
|----------|----------------|-------------------|
| 1M routes, same ORIGIN/LP/AS_PATH | Good (if identical) | Same |
| 1M routes, same except MED | 1M × ~50B = 50MB | ~1MB (shared + MED pool) |
| Route reflector (identical attrs) | Excellent | Same |

### Phase 6 Implementation Plan

See `plan/spec-plugin-rib-pool-storage.md` § "Phase 6: Per-Attribute Deduplication" for:
- Per-attribute pools (Origin, ASPath, LocalPref, MED, etc.)
- RouteEntry struct with per-attr handles
- Attribute parser using existing `AttrIterator`
- Wire reconstruction for route resend

**Dependencies:**
- `internal/component/bgp/attribute/iterator.go` - `AttrIterator` (exists, reuse)
- `internal/component/bgp/attrpool/pool.go` - Pool infrastructure (exists, extend)
<!-- source: internal/component/bgp/attribute/ -- AttrIterator -->
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool with Intern/Get/Release -->

---

## Current Decision

**Level 2 (blob dedup)** is implemented. Good dedup when attribute sets are identical.

**Level 3 (per-attr dedup)** planned for Phase 6. Better dedup when routes share
most attributes but differ in one (e.g., MED varies, rest identical).

---

**Created:** 2026-01-01
**Updated:** 2026-01-03 - Fixed design: decomposed attrs+NLRIs, RIB-bound refcounting, pool.Lookup()
