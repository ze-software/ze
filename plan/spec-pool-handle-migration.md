# Spec: Pool Handle Migration

## Status: Design Phase (PRIMARY IMPLEMENTATION SPEC)

**See:** `plan/DESIGN_TRANSITION.md` for overall architecture direction.

---

## Design Transition Alignment

This spec is the **primary implementation spec** for the Pool + Wire design:

| This Spec Implements | Design Goal |
|---------------------|-------------|
| Pool core with Handle encoding | Memory deduplication |
| Route stores `attrHandle` | Wire-canonical storage |
| `Release()` lifecycle | Reference counting |

### Supersedes

| Spec | Status |
|------|--------|
| `spec-pool-integration.md` | **SKIP** - don't implement factory methods |
| `buildRIBRouteUpdate` conversion | **SKIP** - pool forwarding replaces it |

### Enables

| Spec | When |
|------|------|
| `spec-unified-handle-nlri.md` | After Phase 2 |
| Zero-copy forwarding | After Phase 3 |

---

## Zero-Copy Design (CRITICAL)

The path from peer receive to RIB storage MUST be zero-copy until interning:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        ZERO-COPY PATH                                   │
│                                                                         │
│  Network recv() ──► buffer[]byte ──► AttributesWire(ref) ──► Process   │
│       │                  │                  │                    │      │
│       │              (owned by           (slice                (API     │
│       │              connection)         reference,            filters, │
│       │                                  NO COPY)              etc.)    │
│       │                                                          │      │
│       └──────────────────────────────────────────────────────────┘      │
│                                                                         │
│  ═══════════════════════ COPY BOUNDARY ════════════════════════════     │
│                                                                         │
│  Route creation ──► pool.Intern(wireBytes) ──► RIB storage             │
│       │                    │                        │                   │
│       │              (COPY into pool,          (pool handles,           │
│       │               deduplication)            long-lived)             │
└─────────────────────────────────────────────────────────────────────────┘
```

### Key Principles

1. **AttributesWire** = Temporary wrapper, stores REFERENCE to received bytes
   - NO pool interaction
   - NO copying
   - Lifetime tied to message processing

2. **Route** = Long-lived RIB entry, stores POOL HANDLES
   - `NewRouteWithWireCache()` calls `pool.Intern()`
   - Interning is the ONLY copy point
   - Deduplication happens here

3. **Copy happens exactly once** - when storing in RIB

---

## Overview

Migrate from direct `[]byte` storage to `pool.Handle` references for memory
deduplication across routes. This enables efficient memory usage when many
routes share identical attributes.

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

### Two-Level Attribute Pooling

```go
// Level 1: Individual attribute bytes
var AttrPool = pool.NewPool(...)

// Level 2: Attribute sets (bitmap + handle list)
var AttrSetPool = pool.NewPool(...)
```

### AttrSet Encoding Format

```
[bitmap:4 bytes][handle:4][handle:4][handle:4]...
     │              │         │         │
     │              └─────────┴─────────┘
     │               handles for set bits only
     │               in ascending code order
     │
 bit N = 1 if code N present
 (excludes codes 3, 14, 15 - handled separately)
```

Example: Route has ORIGIN(1), AS_PATH(2), LOCAL_PREF(5)
- Bitmap: `0x00000026` (bits 1, 2, 5 set)
- Bytes: `[00 00 00 26][origin_h][asPath_h][localPref_h]`
- Total: 4 + 3×4 = 16 bytes

### Route Structure

```go
type Route struct {
    attrSetHandle pool.Handle  // → AttrSetPool → [bitmap][h][h]...
    nlri          nlri.NLRI
    nextHop       netip.Addr   // Inline (code 3, per-peer)
    sourceCtxID   ContextID
}
```

### Deduplication

| Level | What's Deduped |
|-------|----------------|
| AttrPool | Individual attr bytes (same AS_PATH → same handle) |
| AttrSetPool | Handle combinations (same attr set → same setHandle) |

### AttributesWire UNCHANGED

```go
// Zero-copy reference - NO POOL interaction
type AttributesWire struct {
    packed      []byte             // Refs message buffer
    sourceCtxID ContextID
    index       []attrIndex
    parsed      map[AttributeCode]Attribute
    mu          sync.RWMutex
}
```

## Migration Phases

### Phase 1: Implement Pool Core

**File:** `pkg/pool/pool.go`

```go
package pool

type Handle uint32

const (
    InvalidHandle Handle = 0x7FFFFFFF
    BufferBitMask Handle = 0x80000000
    SlotIndexMask Handle = 0x7FFFFFFF
)

type Pool struct {
    mu         sync.RWMutex
    buffers    [2]buffer
    currentBit uint32
    slots      []Slot
    index      map[string]Handle  // dedup index
    // ... per POOL_ARCHITECTURE.md
}

// Core operations
func (p *Pool) Intern(data []byte) Handle
func (p *Pool) Get(h Handle) []byte
func (p *Pool) AddRef(h Handle)
func (p *Pool) Release(h Handle)
func (p *Pool) Length(h Handle) int
```

**Tests:**
- Intern returns same handle for identical data
- Get returns correct bytes
- AddRef/Release reference counting
- Compaction preserves data integrity

### Phase 2: Global Pools

**File:** `pkg/pool/attributes.go`

```go
// Global attribute pool
var Attributes = NewPool(PoolConfig{
    InitialBufferSize: 1 << 20,  // 1MB
    ExpectedEntries:   10000,
})

// Global NLRI pool
var NLRI = NewPool(PoolConfig{
    InitialBufferSize: 1 << 18,  // 256KB
    ExpectedEntries:   50000,
})
```

**Note:** AttributesWire is NOT modified. It remains a zero-copy reference.

### Phase 3: Update Route (INTERNING POINT)

**Files:** `pkg/pool/attrset.go`, `pkg/rib/route.go`

This is the ONLY place where data is copied into the pool.

---

#### Code Mapping (Bitmap Position → Attribute Code)

Bitmap bits don't map 1:1 to attribute codes (codes have gaps, some excluded).

```go
// Ordered list of pooled attribute codes
// Excludes: 3 (NEXT_HOP), 14 (MP_REACH), 15 (MP_UNREACH)
var pooledCodes = []uint8{
    1,  // bit 0  = ORIGIN
    2,  // bit 1  = AS_PATH
    4,  // bit 2  = MED
    5,  // bit 3  = LOCAL_PREF
    6,  // bit 4  = ATOMIC_AGGREGATE
    7,  // bit 5  = AGGREGATOR
    8,  // bit 6  = COMMUNITIES
    9,  // bit 7  = ORIGINATOR_ID
    10, // bit 8  = CLUSTER_LIST
    16, // bit 9  = EXTENDED_COMMUNITIES
    17, // bit 10 = AS4_PATH
    18, // bit 11 = AS4_AGGREGATOR
    22, // bit 12 = PMSI_TUNNEL
    32, // bit 13 = LARGE_COMMUNITIES
    // ... up to 30 known codes (bit 31 reserved)
}

// Reverse lookup: code → bit position (-1 if not mapped)
var codeToBit [256]int8  // Initialized from pooledCodes
```

---

#### Bitmap Format

```
┌─────────────────────────────────────────────────────────────┐
│                    Bitmap (32 bits)                         │
│                                                             │
│  bit 31    │ bits 0-30                                     │
│  ────────  │ ─────────────────────────────────────────     │
│  HasUnknown│ Known attr presence (via pooledCodes mapping) │
│  flag      │                                                │
└─────────────────────────────────────────────────────────────┘

const HasUnknownBit = uint32(1 << 31)
```

---

#### AttrSet Encoding Format

```
Known attrs only:
[bitmap:4][slot:4][slot:4]...

With unknown attrs:
[bitmap:4][slot:4]...[unknown_count:1][(code:1)(slot:4)]...
     │                      │              │
     │                      │              └─ unknown attrs (5 bytes each)
     │                      └─ count of unknown attrs
     └─ bit 31 set indicates unknowns present
```

**Handle Normalization:** Slots are stored with buffer bit cleared (bit 0).
This ensures dedup works regardless of compaction state.
Both `0x00000005` and `0x80000005` normalize to `0x00000005`.

---

#### Pool Access: GetBySlot

Since we store normalized slots (bit 0), we need a method that
accesses data via slot index using the current valid buffer:

```go
// GetBySlot retrieves data using slot index, not full handle.
// Uses whichever buffer is currently valid for the slot.
func (p *Pool) GetBySlot(slotIdx uint32) []byte {
    p.mu.RLock()
    defer p.mu.RUnlock()

    slot := &p.slots[slotIdx]
    bufIdx := p.currentBit

    // During compaction, slot might not be migrated yet
    if p.state == PoolCompacting && slotIdx >= p.compactCursor {
        bufIdx = 1 - p.currentBit  // Use old buffer
    }

    offset := slot.offsets[bufIdx]
    return p.buffers[bufIdx].data[offset : offset+uint32(slot.length)]
}
```

---

#### InternAttrSet

```go
func InternAttrSet(attrs *attribute.AttributesWire) Handle {
    var bitmap uint32
    var knownSlots []uint32
    var unknownAttrs []struct{ code uint8; slot uint32 }

    for _, entry := range attrs.Index() {
        code := uint8(entry.Code)

        // Skip per-route attrs
        if code == 3 || code == 14 || code == 15 {
            continue
        }

        raw, _ := attrs.GetRaw(entry.Code)
        h := Attributes.Intern(raw)              // Level 1: intern
        slot := h.SlotIndex()                    // Normalize (clear MSB)

        if bit := CodeToBit(code); bit >= 0 {
            bitmap |= 1 << bit
            knownSlots = append(knownSlots, slot)
        } else {
            unknownAttrs = append(unknownAttrs, struct{ code uint8; slot uint32 }{code, slot})
        }
    }

    // Set unknown flag
    if len(unknownAttrs) > 0 {
        bitmap |= HasUnknownBit
    }

    // Encode: [bitmap:4][known slots...][unknown_count:1][(code:1)(slot:4)]...
    size := 4 + len(knownSlots)*4
    if len(unknownAttrs) > 0 {
        size += 1 + len(unknownAttrs)*5
    }
    buf := make([]byte, size)

    binary.BigEndian.PutUint32(buf[:4], bitmap)
    offset := 4
    for _, s := range knownSlots {
        binary.BigEndian.PutUint32(buf[offset:], s)
        offset += 4
    }
    if len(unknownAttrs) > 0 {
        buf[offset] = uint8(len(unknownAttrs))
        offset++
        for _, u := range unknownAttrs {
            buf[offset] = u.code
            binary.BigEndian.PutUint32(buf[offset+1:], u.slot)
            offset += 5
        }
    }

    return AttrSets.Intern(buf)  // Level 2: intern handle list
}
```

---

#### GetAttrBytes

```go
func GetAttrBytes(setHandle Handle, code uint8) []byte {
    setBytes := AttrSets.Get(setHandle)
    bitmap := binary.BigEndian.Uint32(setBytes[:4])

    // Check known attrs
    if bit := CodeToBit(code); bit >= 0 {
        if bitmap&(1<<bit) == 0 {
            return nil  // Not present
        }
        idx := bits.OnesCount32(bitmap & ((1 << bit) - 1))
        slot := binary.BigEndian.Uint32(setBytes[4+idx*4:])
        return Attributes.GetBySlot(slot)
    }

    // Check unknown attrs (only if flag set)
    if bitmap&HasUnknownBit == 0 {
        return nil  // No unknowns, fast path
    }

    knownCount := bits.OnesCount32(bitmap &^ HasUnknownBit)
    unknownStart := 4 + knownCount*4
    unknownCount := int(setBytes[unknownStart])

    for i := 0; i < unknownCount; i++ {
        off := unknownStart + 1 + i*5
        if setBytes[off] == code {
            slot := binary.BigEndian.Uint32(setBytes[off+1:])
            return Attributes.GetBySlot(slot)
        }
    }

    return nil
}
```

---

#### ReleaseAttrSet

```go
func ReleaseAttrSet(setHandle Handle) {
    setBytes := AttrSets.Get(setHandle)
    bitmap := binary.BigEndian.Uint32(setBytes[:4])

    // Release known attrs
    knownCount := bits.OnesCount32(bitmap &^ HasUnknownBit)
    for i := 0; i < knownCount; i++ {
        slot := binary.BigEndian.Uint32(setBytes[4+i*4:])
        Attributes.ReleaseBySlot(slot)
    }

    // Release unknown attrs
    if bitmap&HasUnknownBit != 0 {
        unknownStart := 4 + knownCount*4
        unknownCount := int(setBytes[unknownStart])
        for i := 0; i < unknownCount; i++ {
            off := unknownStart + 1 + i*5
            slot := binary.BigEndian.Uint32(setBytes[off+1:])
            Attributes.ReleaseBySlot(slot)
        }
    }

    AttrSets.Release(setHandle)
}
```

---

#### Route Structure

```go
type Route struct {
    nlri          nlri.NLRI
    attrSetHandle pool.Handle  // → AttrSetPool → [bitmap][slots...]
    nextHop       netip.Addr   // Inline (per-peer, not pooled)
    sourceCtxID   ContextID
}

func (r *Route) GetAttr(code uint8) []byte {
    return pool.GetAttrBytes(r.attrSetHandle, code)
}

func (r *Route) ReleasePoolHandles() {
    pool.ReleaseAttrSet(r.attrSetHandle)
}
```

### Phase 4: Compaction Integration

**File:** `pkg/pool/scheduler.go`

```go
type CompactionScheduler struct {
    pools         []*Pool
    activePool    *Pool
    // ... per POOL_ARCHITECTURE.md
}

func (s *CompactionScheduler) Run(ctx context.Context)
```

**Integration:** Start scheduler in reactor initialization.

## API Changes

### Breaking Changes

| Before | After |
|--------|-------|
| `Route.wireBytes []byte` | `Route.attrHandle pool.Handle` |
| `Route.nlriWireBytes []byte` | `Route.nlriHandle pool.Handle` |
| N/A | `Route.ReleasePoolHandles()` MUST be called |

### Unchanged (Zero-Copy Preserved)

| Component | Why Unchanged |
|-----------|---------------|
| `AttributesWire` | Keeps zero-copy reference to received bytes |
| `NewAttributesWire()` | No pool interaction |
| `AttributesWire.Packed()` | Returns reference, not pool data |

### New Requirements

1. **Lifecycle management:** Routes MUST call `ReleasePoolHandles()` when removed from RIB
2. **No modification:** Bytes from `pool.Get()` / `WireBytes()` MUST NOT be modified
3. **Thread safety:** Pool operations are thread-safe

## Memory Analysis

### Without Pool (Current)

| Routes | Unique Attrs | Memory |
|--------|--------------|--------|
| 1M | 100K | 1M * avgAttrSize (wasteful) |

### With Pool (Target)

| Routes | Unique Attrs | Memory |
|--------|--------------|--------|
| 1M | 100K | 100K * avgAttrSize (deduplicated) |

**Typical savings:** 80-90% for route reflectors (many routes share attributes)

## Checklist

### Phase 1: Pool Core ✅
- [x] Implement `pkg/pool/pool.go`
- [x] Implement `pkg/pool/handle.go`
- [x] Tests for Intern/Get/AddRef/Release
- [x] Tests for deduplication
- [x] Tests for concurrent access

### Phase 2: Global Pools ✅
- [x] Create global Attributes pool
- [x] Create global NLRI pool

### Phase 3: Route Migration (INTERNING POINT)
- [ ] Change `wireBytes` to `attrHandle`
- [ ] Change `nlriWireBytes` to `nlriHandle`
- [ ] Update constructors to call `pool.Intern()`
- [ ] Add `ReleasePoolHandles()` method
- [ ] Update `WireBytes()` / `NLRIWireBytes()` to use `pool.Get()`
- [ ] Update RIB to call `ReleasePoolHandles()` on route removal

### Phase 4: Compaction
- [ ] Implement CompactionScheduler
- [ ] Integrate with reactor
- [ ] Test incremental compaction
- [ ] Test activity-based pausing

**Note:** AttributesWire is NOT modified - it remains a zero-copy reference.

## Dependencies

- `POOL_ARCHITECTURE.md` - Design specification
- `spec-attributes-wire.md` - AttributesWire current design

## Risks

| Risk | Mitigation |
|------|------------|
| Forgetting Release() | Static analysis, runtime leak detection |
| Pool contention | Per-family pools, sharding if needed |
| Compaction latency | Incremental, pause on activity |

---

**Created:** 2026-01-01
