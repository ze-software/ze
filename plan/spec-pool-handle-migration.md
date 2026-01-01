# Spec: Pool Handle Migration

## Status: Design Phase

## Overview

Migrate from direct `[]byte` storage to `pool.Handle` references for memory
deduplication across routes. This enables efficient memory usage when many
routes share identical attributes.

## Current State

### Design Doc
- `POOL_ARCHITECTURE.md` describes the pool system design
- Double-buffer with MSB handles, incremental compaction
- NOT YET IMPLEMENTED

### Current Code
```go
// Route stores wire bytes directly
type Route struct {
    wireBytes     []byte           // Direct ownership
    nlriWireBytes []byte           // Direct ownership
    sourceCtxID   ContextID
}

// AttributesWire references external buffer (spec-attributes-wire.md)
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

// AttributesWire wraps a pool handle
type AttributesWire struct {
    handle      pool.Handle        // Pool owns the bytes
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

### Phase 2: Attribute Pool

**File:** `pkg/pool/attributes.go`

```go
// Global attribute pool
var Attributes = NewPool(PoolConfig{
    InitialBufferSize: 1 << 20,  // 1MB
    ExpectedEntries:   10000,
})
```

**Integration point:** Update receive path to intern attributes.

### Phase 3: Update AttributesWire

**File:** `pkg/bgp/attribute/wire.go`

```go
type AttributesWire struct {
    handle      pool.Handle        // CHANGED from []byte
    sourceCtxID ContextID
    index       []attrIndex
    parsed      map[AttributeCode]Attribute
    mu          sync.RWMutex
}

func NewAttributesWire(packed []byte, ctxID ContextID) *AttributesWire {
    h := pool.Attributes.Intern(packed)  // Dedup + copy
    return &AttributesWire{
        handle:      h,
        sourceCtxID: ctxID,
    }
}

func (a *AttributesWire) Packed() []byte {
    return pool.Attributes.Get(a.handle)
}

func (a *AttributesWire) Release() {
    pool.Attributes.Release(a.handle)
}
```

### Phase 4: Update Route

**File:** `pkg/rib/route.go`

```go
type Route struct {
    nlri        NLRI
    attrHandle  pool.Handle    // CHANGED from wireBytes []byte
    nlriHandle  pool.Handle    // CHANGED from nlriWireBytes []byte
    sourceCtxID ContextID
    // ...
}

func (r *Route) PackAttributesFor(destCtxID ContextID) []byte {
    if r.sourceCtxID == destCtxID {
        return pool.Attributes.Get(r.attrHandle)  // Zero-copy from pool
    }
    // Re-encode path...
}

func (r *Route) Release() {
    pool.Attributes.Release(r.attrHandle)
    pool.NLRI.Release(r.nlriHandle)
}
```

### Phase 5: Compaction Integration

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
| `NewAttributesWire([]byte, ContextID)` | Same signature, but now interns |
| `AttributesWire.Packed() []byte` | Returns pool-managed bytes |
| N/A | `AttributesWire.Release()` MUST be called |

### New Requirements

1. **Lifecycle management:** Routes/AttributesWire MUST call `Release()` when done
2. **No modification:** Bytes from `Packed()`/`Get()` MUST NOT be modified
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

### Phase 1: Pool Core
- [ ] Implement `pkg/pool/pool.go`
- [ ] Implement `pkg/pool/handle.go`
- [ ] Tests for Intern/Get/AddRef/Release
- [ ] Tests for deduplication
- [ ] Tests for concurrent access

### Phase 2: Attribute Pool
- [ ] Create global Attributes pool
- [ ] Integrate with receive path

### Phase 3: AttributesWire Migration
- [ ] Change `packed []byte` to `handle Handle`
- [ ] Update `NewAttributesWire` to intern
- [ ] Update `Packed()` to use `pool.Get()`
- [ ] Add `Release()` method
- [ ] Update all callers to call Release()

### Phase 4: Route Migration
- [ ] Change `wireBytes` to `attrHandle`
- [ ] Change `nlriWireBytes` to `nlriHandle`
- [ ] Update constructors
- [ ] Add `Release()` method
- [ ] Update RIB to call Release() on route removal

### Phase 5: Compaction
- [ ] Implement CompactionScheduler
- [ ] Integrate with reactor
- [ ] Test incremental compaction
- [ ] Test activity-based pausing

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
