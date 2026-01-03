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

```go
// Pool manages deduplicated byte storage
type Pool struct {
    // Per POOL_ARCHITECTURE.md
}

type Handle uint32  // MSB = buffer bit, lower 31 = slot index

// Route uses handles for deduplication
type Route struct {
    attrHandle    pool.Handle      // Deduplicated attributes
    nlriHandle    pool.Handle      // Deduplicated NLRI
    sourceCtxID   ContextID
}

// AttributesWire UNCHANGED - stays as zero-copy reference
type AttributesWire struct {
    packed      []byte             // Still refs message buffer (NO POOL)
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

**File:** `pkg/rib/route.go`

This is the ONLY place where data is copied into the pool.

```go
type Route struct {
    nlri        NLRI
    attrHandle  pool.Handle    // CHANGED from wireBytes []byte
    nlriHandle  pool.Handle    // CHANGED from nlriWireBytes []byte
    sourceCtxID ContextID
    // ...
}

// NewRouteWithWireCache - THE INTERNING POINT
// This is where zero-copy ends and pool dedup begins
func NewRouteWithWireCache(
    n NLRI,
    nextHop netip.Addr,
    attrs []attribute.Attribute,
    asPath *attribute.ASPath,
    wireBytes []byte,          // From AttributesWire.Packed()
    sourceCtxID ContextID,
) *Route {
    return &Route{
        nlri:        n,
        nextHop:     nextHop,
        attributes:  attrs,
        asPath:      asPath,
        attrHandle:  pool.Attributes.Intern(wireBytes),  // COPY HERE
        nlriHandle:  pool.InvalidHandle,
        sourceCtxID: sourceCtxID,
    }
}

func (r *Route) WireBytes() []byte {
    if r.attrHandle == pool.InvalidHandle {
        return nil
    }
    return pool.Attributes.Get(r.attrHandle)  // Zero-copy from pool
}

func (r *Route) PackAttributesFor(destCtxID ContextID) []byte {
    if r.sourceCtxID == destCtxID && r.attrHandle != pool.InvalidHandle {
        return pool.Attributes.Get(r.attrHandle)  // Zero-copy from pool
    }
    // Re-encode path...
}

func (r *Route) ReleasePoolHandles() {
    if r.attrHandle != pool.InvalidHandle {
        pool.Attributes.Release(r.attrHandle)
    }
    if r.nlriHandle != pool.InvalidHandle {
        pool.NLRI.Release(r.nlriHandle)
    }
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
