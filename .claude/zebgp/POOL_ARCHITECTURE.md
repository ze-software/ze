# ZeBGP Pool Architecture

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Purpose** | Deduplicate attributes/NLRIs across peers, zero-copy forwarding |
| **Key Pattern** | Double-buffer with MSB handles: `Handle = (bufferBit << 31) \| slotIndex` |
| **Core Types** | `Handle`, `Pool`, `Slot`, `CompactionScheduler` |
| **Key Functions** | `Pool.Intern()`, `Pool.Get()`, `Pool.AddRef()`, `Pool.Release()` |
| **Zero-Copy** | Slice into message buffer; copy only when storing to pool |

**When to read full doc:** Pool storage, memory issues, compaction debugging, pass-through optimization.

---

Memory-efficient, zero-copy attribute and NLRI deduplication.

---

## Design Goals

1. **Memory efficiency**: Deduplicate identical attributes/NLRIs across all peers
2. **Zero-copy parsing**: Slice into message buffer, don't copy until storing
3. **Non-blocking**: Incremental compaction, no stop-the-world pauses
4. **Pass-through optimization**: Forward unchanged messages without parsing
5. **Scalable**: Handle millions of routes with bounded memory

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    Global Compaction Scheduler                   │
│  • One pool compacts at a time                                  │
│  • Triggers on: memory pressure + low activity                  │
│  • Pauses when activity resumes                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
           ┌──────────────────┼──────────────────┐
           ▼                  ▼                  ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
│  Attribute Pools │ │  Attribute Pools │ │   NLRI Pools     │
│  ┌────────────┐  │ │  ┌────────────┐  │ │  ┌────────────┐  │
│  │  ORIGIN    │  │ │  │  AS_PATH   │  │ │  │ IPv4 Ucast │  │
│  │  Pool      │  │ │  │  Pool      │  │ │  │ Pool       │  │
│  └────────────┘  │ │  └────────────┘  │ │  └────────────┘  │
│  ┌────────────┐  │ │  ┌────────────┐  │ │  ┌────────────┐  │
│  │ COMMUNITIES│  │ │  │ NEXT_HOP   │  │ │  │ IPv6 Ucast │  │
│  │  Pool      │  │ │  │  Pool      │  │ │  │ Pool       │  │
│  └────────────┘  │ │  └────────────┘  │ │  └────────────┘  │
│       ...        │ │       ...        │ │       ...        │
└──────────────────┘ └──────────────────┘ └──────────────────┘
```

---

## Reference Chain

```
┌─────────────────────────────────────────────────────────────────┐
│                            RIB                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ RIB Entry                                                  │  │
│  │   nlriHandle ─────────────────────────┐                    │  │
│  │   originHandle ───────────────────────┼──┐                 │  │
│  │   communitiesHandle ──────────────────┼──┼──┐              │  │
│  │   nextHopHandle ──────────────────────┼──┼──┼──┐           │  │
│  │   ...                                 │  │  │  │           │  │
│  └───────────────────────────────────────┼──┼──┼──┼───────────┘  │
└──────────────────────────────────────────┼──┼──┼──┼──────────────┘
                                           │  │  │  │
              ┌────────────────────────────┘  │  │  │
              ▼                               │  │  │
┌─────────────────────────┐                   │  │  │
│  NLRI Pool (per-family) │                   │  │  │
│  ┌───────────────────┐  │                   │  │  │
│  │ Slot              │  │                   │  │  │
│  │  offsets[2]       │  │                   │  │  │
│  │  refCount: 3      │  │                   │  │  │
│  │  asPathRef ───────┼──┼───┐               │  │  │
│  └───────────────────┘  │   │               │  │  │
└─────────────────────────┘   │               │  │  │
                              ▼               ▼  ▼  ▼
                    ┌─────────────────────────────────────┐
                    │         Attribute Pools              │
                    │  AS_PATH, ORIGIN, COMMUNITIES, etc.  │
                    └─────────────────────────────────────┘
```

**Key insight**: NLRI entries reference AS_PATH (per AS-PATH-as-NLRI-extension design).
When NLRI is released, it cascades to release its AS_PATH reference.

---

## Handle Design (MSB Buffer Bit)

Handles use the **most significant bit** to indicate which buffer contains the data.
This creates two distinct number spaces:

```go
type Handle uint32

const (
    InvalidHandle   Handle = 0x7FFFFFFF  // Max slot index, invalid
    BufferBitMask   Handle = 0x80000000  // Bit 31
    SlotIndexMask   Handle = 0x7FFFFFFF  // Bits 0-30
)

func MakeHandle(slotIdx uint32, bufferBit uint32) Handle {
    return Handle(slotIdx&uint32(SlotIndexMask)) | Handle(bufferBit<<31)
}

func (h Handle) SlotIndex() uint32 { return uint32(h & SlotIndexMask) }
func (h Handle) BufferBit() uint32 { return uint32(h >> 31) }

// Visual checks
func (h Handle) IsLowerHalf() bool { return h < BufferBitMask }  // buffer 0
func (h Handle) IsUpperHalf() bool { return h >= BufferBitMask } // buffer 1
```

### Handle Number Space

```
0x00000000 ─┬─ Buffer 0 (lower half)
            │  0x00000000 = slot 0, buffer 0
            │  0x00000001 = slot 1, buffer 0
            │  0x00000002 = slot 2, buffer 0
            │  ...
0x7FFFFFFF ─┘  (InvalidHandle)

0x80000000 ─┬─ Buffer 1 (upper half)
            │  0x80000000 = slot 0, buffer 1
            │  0x80000001 = slot 1, buffer 1
            │  0x80000002 = slot 2, buffer 1
            │  ...
0xFFFFFFFF ─┘
```

### Benefits of MSB Design

| Aspect | Benefit |
|--------|---------|
| Slot index preserved | Slot 5 → `0x00000005` or `0x80000005` |
| Visual debugging | Upper half handles clearly distinct |
| Simple extraction | `slotIdx = h & 0x7FFFFFFF` (mask, no shift) |
| Range check | `h >= 0x80000000` means buffer 1 |

---

## Pool Structure

```go
type Pool struct {
    mu sync.RWMutex

    // Double buffer - alternates between compaction cycles
    buffers [2]struct {
        data     []byte
        pos      int            // write cursor
        refCount atomic.Int32   // handles pointing here
    }
    currentBit uint32  // 0 or 1 - which buffer is current

    // Slot table - indexed by handle.SlotIndex()
    slots []Slot

    // Dedup index: data content → Handle (always points to current buffer)
    // Keys are unsafe.String pointing directly into buffer (zero-copy)
    index map[string]Handle

    // Compaction state
    state         PoolState
    compactCursor uint32

    // Metrics
    liveBytes int64
    liveCount int32
    deadCount int32

    // Activity tracking
    lastActivityNano atomic.Int64

    // Configuration
    config PoolConfig
}

type Slot struct {
    // Offset in each buffer (both valid during compaction)
    offsets [2]uint32
    length  uint16

    // Reference counting (per-slot, across both handles)
    refCount int32

    // GC state
    dead bool
}

type PoolState int

const (
    PoolNormal PoolState = iota
    PoolCompacting
)
```

---

## Alternating Buffer Model

The buffer bit alternates each compaction cycle. During compaction, **both handles are valid**.

### Compaction Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│  Cycle 0: currentBit = 0                                        │
│                                                                 │
│  buffers[0]: [████████████]  ← all data here                   │
│  buffers[1]: nil                                                │
│                                                                 │
│  All handles in lower half: 0x00000000, 0x00000001, ...        │
└─────────────────────────────────────────────────────────────────┘
                              │
                        Start Compaction
                        currentBit = 1
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  During Compaction 1                                            │
│                                                                 │
│  buffers[0]: [████████████]  ← old data (being migrated from)  │
│  buffers[1]: [████░░░░░░░░]  ← new data (migration target)     │
│                                                                 │
│  Old handles (lower half): 0x00000005 → buffers[0] ✓           │
│  New handles (upper half): 0x80000005 → buffers[1] ✓           │
│  Both valid simultaneously!                                     │
│                                                                 │
│  New Intern() creates upper half handles                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                        Compaction Complete
                        (when buffers[0].refCount == 0)
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Cycle 1: currentBit = 1                                        │
│                                                                 │
│  buffers[0]: nil (freed)                                        │
│  buffers[1]: [████████████]  ← all data here                   │
│                                                                 │
│  All handles in upper half: 0x80000000, 0x80000001, ...        │
└─────────────────────────────────────────────────────────────────┘
                              │
                        Start Compaction
                        currentBit = 0
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  During Compaction 2                                            │
│                                                                 │
│  buffers[0]: [████░░░░░░░░]  ← new data (migration target)     │
│  buffers[1]: [████████████]  ← old data (being migrated from)  │
│                                                                 │
│  Old handles (upper half): 0x80000005 → buffers[1] ✓           │
│  New handles (lower half): 0x00000005 → buffers[0] ✓           │
│  Both valid simultaneously!                                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                        ... alternates forever
```

---

## Operations

### Intern (Deduplicate and Store)

```go
func (p *Pool) Intern(data []byte) Handle {
    lookupKey := bytesToString(data)

    p.mu.Lock()
    defer p.mu.Unlock()

    p.touchActivity()

    // Check for existing (deduplication)
    // Index always contains handles with currentBit
    if h, ok := p.index[lookupKey]; ok {
        slot := &p.slots[h.SlotIndex()]
        if !slot.dead && slot.refCount > 0 {
            slot.refCount++
            p.buffers[h.BufferBit()].refCount.Add(1)
            return h
        }
    }

    // Allocate new entry in current buffer
    bufIdx := p.currentBit
    buf := &p.buffers[bufIdx]

    p.ensureCapacity(bufIdx, len(data))
    offset := uint32(buf.pos)
    copy(buf.data[buf.pos:], data)
    buf.pos += len(data)

    // Allocate slot
    slotIdx := p.allocSlot()
    slot := &p.slots[slotIdx]
    slot.offsets[bufIdx] = offset
    slot.length = uint16(len(data))
    slot.refCount = 1
    slot.dead = false

    // Create handle with current buffer bit
    h := MakeHandle(slotIdx, bufIdx)

    // Track buffer reference
    buf.refCount.Add(1)

    // Index with key pointing to buffer memory
    bufferKey := bytesToString(buf.data[offset : offset+uint32(len(data))])
    p.index[bufferKey] = h

    p.liveBytes += int64(len(data))
    p.liveCount++

    return h
}
```

### Get (Read Data)

```go
func (p *Pool) Get(h Handle) []byte {
    p.mu.RLock()
    defer p.mu.RUnlock()

    bufIdx := h.BufferBit()
    slot := &p.slots[h.SlotIndex()]

    offset := slot.offsets[bufIdx]
    return p.buffers[bufIdx].data[offset : offset+uint32(slot.length)]
}
```

### WriteTo (Write Data to Output)

```go
func (p *Pool) WriteTo(h Handle, w io.Writer) (int64, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()

    bufIdx := h.BufferBit()
    slot := &p.slots[h.SlotIndex()]

    offset := slot.offsets[bufIdx]
    data := p.buffers[bufIdx].data[offset : offset+uint32(slot.length)]

    n, err := w.Write(data)
    return int64(n), err
}
```

### AddRef (Share Reference)

```go
func (p *Pool) AddRef(h Handle) {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.slots[h.SlotIndex()].refCount++
    p.buffers[h.BufferBit()].refCount.Add(1)
}
```

### Release (Decrement Reference)

```go
func (p *Pool) Release(h Handle) {
    p.mu.Lock()
    defer p.mu.Unlock()

    bufIdx := h.BufferBit()
    slotIdx := h.SlotIndex()
    slot := &p.slots[slotIdx]

    slot.refCount--
    p.buffers[bufIdx].refCount.Add(-1)

    if slot.refCount <= 0 {
        slot.dead = true
        p.deadCount++
        p.liveCount--
        p.liveBytes -= int64(slot.length)

        // Remove from index if this is the current handle
        if bufIdx == p.currentBit {
            offset := slot.offsets[bufIdx]
            bufferKey := bytesToString(p.buffers[bufIdx].data[offset : offset+uint32(slot.length)])
            delete(p.index, bufferKey)
        }
    }
}
```

---

## Incremental Compaction

### Start Compaction

```go
func (p *Pool) startCompaction() {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Flip to new buffer
    oldBit := p.currentBit
    newBit := 1 - oldBit
    p.currentBit = newBit

    // Allocate new buffer (live data + headroom)
    newSize := p.liveBytes + p.liveBytes/4
    p.buffers[newBit].data = make([]byte, newSize)
    p.buffers[newBit].pos = 0
    p.buffers[newBit].refCount.Store(0)

    p.state = PoolCompacting
    p.compactCursor = 0
}
```

### Migrate Batch

```go
func (p *Pool) MigrateBatch(batchSize int) bool {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.state != PoolCompacting {
        return true
    }

    oldBit := 1 - p.currentBit
    newBit := p.currentBit
    oldBuf := &p.buffers[oldBit]
    newBuf := &p.buffers[newBit]

    migrated := 0
    for p.compactCursor < uint32(len(p.slots)) && migrated < batchSize {
        slot := &p.slots[p.compactCursor]

        if !slot.dead && slot.refCount > 0 {
            // Copy data from old buffer to new buffer
            oldOffset := slot.offsets[oldBit]
            oldData := oldBuf.data[oldOffset : oldOffset+uint32(slot.length)]

            newOffset := uint32(newBuf.pos)
            p.ensureCapacity(newBit, int(slot.length))
            copy(newBuf.data[newBuf.pos:], oldData)
            newBuf.pos += int(slot.length)

            slot.offsets[newBit] = newOffset

            // Update index: old key → new key with new handle
            oldKey := bytesToString(oldData)
            delete(p.index, oldKey)

            newKey := bytesToString(newBuf.data[newOffset : newOffset+uint32(slot.length)])
            newHandle := MakeHandle(p.compactCursor, newBit)
            p.index[newKey] = newHandle

            // Note: old handle still valid (oldBuf still exists)
            // New handle also valid (data now in newBuf)

            migrated++
        }

        p.compactCursor++
    }

    // Check if migration complete
    if p.compactCursor >= uint32(len(p.slots)) {
        // All entries migrated, but old buffer may still have references
        // It will be freed when buffers[oldBit].refCount == 0
        if p.buffers[oldBit].refCount.Load() == 0 {
            p.finishCompaction(oldBit)
        }
        return true
    }

    return false
}

func (p *Pool) finishCompaction(oldBit uint32) {
    p.buffers[oldBit].data = nil
    p.buffers[oldBit].pos = 0
    p.deadCount = 0
    p.state = PoolNormal
}

func (p *Pool) checkOldBufferRelease() {
    if p.state != PoolCompacting {
        return
    }
    oldBit := 1 - p.currentBit
    if p.buffers[oldBit].refCount.Load() == 0 {
        p.finishCompaction(oldBit)
    }
}
```

---

## Global Compaction Scheduler

One pool compacts at a time. Pauses when activity detected. Round-robin prevents starvation.

```go
type CompactionScheduler struct {
    pools     []*Pool
    mu        sync.Mutex
    lastIndex int  // round-robin cursor

    // Current compaction
    activePool *Pool
    paused     bool

    // Configuration
    idleThreshold    time.Duration
    checkInterval    time.Duration
    migrateBatchSize int
}

func (s *CompactionScheduler) Run(ctx context.Context) {
    ticker := time.NewTicker(s.checkInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.tick()
        }
    }
}

func (s *CompactionScheduler) tick() {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Check if any pool has activity
    anyActive := false
    for _, p := range s.pools {
        if !p.isIdle(s.idleThreshold) {
            anyActive = true
            break
        }
    }

    if anyActive {
        s.paused = true
        return
    }

    s.paused = false

    // Continue or start compaction
    if s.activePool != nil {
        done := s.activePool.MigrateBatch(s.migrateBatchSize)
        if done {
            s.activePool = nil
        }
        return
    }

    // Find pool needing compaction (round-robin)
    n := len(s.pools)
    for i := 0; i < n; i++ {
        idx := (s.lastIndex + 1 + i) % n
        p := s.pools[idx]
        if p.shouldCompact() {
            s.lastIndex = idx
            p.startCompaction()
            s.activePool = p
            return
        }
    }
}
```

---

## Pass-Through Buffer Management

When forwarding unchanged messages to multiple peers:

```go
type PassthroughBuffer struct {
    data     []byte
    refCount atomic.Int32
    pool     *BufferPool
}

func (b *PassthroughBuffer) Acquire() {
    b.refCount.Add(1)
}

func (b *PassthroughBuffer) Release() {
    if b.refCount.Add(-1) == 0 {
        b.pool.Return(b)
    }
}
```

---

## Capability Mismatch Handling

When peers have different capabilities (ADD-PATH, ASN4, message size):

```go
type CapabilitySet struct {
    AddPath         bool
    ASN4            bool
    ExtendedMessage bool
}

type PackedMessageCache struct {
    mu    sync.RWMutex
    cache map[CapabilitySet][]byte
}

func (c *PackedMessageCache) GetOrPack(
    caps CapabilitySet,
    pack func() []byte,
) []byte {
    c.mu.RLock()
    if data, ok := c.cache[caps]; ok {
        c.mu.RUnlock()
        return data
    }
    c.mu.RUnlock()

    c.mu.Lock()
    defer c.mu.Unlock()

    if data, ok := c.cache[caps]; ok {
        return data
    }

    data := pack()
    c.cache[caps] = data
    return data
}
```

---

## Memory Analysis

### Normal Operation

| Component | Memory |
|-----------|--------|
| Active buffer | Live data |
| Slots | ~16 bytes × entries |
| Index | ~40 bytes × entries |

### During Compaction

| Phase | Old Buffer | New Buffer | Peak |
|-------|------------|------------|------|
| Start | 100% | ~0% | 100% |
| Mid | 100% | ~50% | 150% |
| End | 100% | ~75% | 175% |
| After | 0% | 75% | 75% |

**Peak overhead:** ~75% during compaction
**Net result:** Memory reduction (dead data removed)

---

## Buffer Growth and Index Rebuild

When `ensureCapacity()` reallocates a buffer, index keys pointing to that buffer become stale
(they reference old memory, preventing GC). We rebuild affected index entries.

```go
func (p *Pool) ensureCapacity(bufIdx uint32, needed int) {
    buf := &p.buffers[bufIdx]
    required := buf.pos + needed

    if required <= cap(buf.data) {
        if required > len(buf.data) {
            buf.data = buf.data[:required]
        }
        return
    }

    // Need to grow - allocate new and copy
    newCap := cap(buf.data) * 2
    if newCap < required {
        newCap = required
    }

    oldData := buf.data
    buf.data = make([]byte, newCap)
    copy(buf.data, oldData[:buf.pos])

    // Rebuild index entries pointing to this buffer
    p.rebuildIndexForBuffer(bufIdx)
}

func (p *Pool) rebuildIndexForBuffer(bufIdx uint32) {
    buf := &p.buffers[bufIdx]

    // Find and update all index entries pointing to this buffer
    // We need to rebuild because old keys reference old memory
    newIndex := make(map[string]Handle, len(p.index))

    for _, h := range p.index {
        if h.BufferBit() == bufIdx {
            // This entry points to the reallocated buffer - recreate key
            slot := &p.slots[h.SlotIndex()]
            offset := slot.offsets[bufIdx]
            newKey := bytesToString(buf.data[offset : offset+uint32(slot.length)])
            newIndex[newKey] = h
        } else {
            // Entry points to other buffer - copy key as-is
            // (we iterate values, need to get key differently)
        }
    }

    // Alternative: iterate slots directly
    p.index = make(map[string]Handle, len(p.slots))
    for i := range p.slots {
        slot := &p.slots[i]
        if !slot.dead && slot.refCount > 0 {
            // Determine which buffer this slot's current handle points to
            // Use currentBit to create the handle
            h := MakeHandle(uint32(i), p.currentBit)
            offset := slot.offsets[p.currentBit]
            key := bytesToString(p.buffers[p.currentBit].data[offset : offset+uint32(slot.length)])
            p.index[key] = h
        }
    }
}
```

**When rebuild occurs:**
- `ensureCapacity()` reallocates a buffer
- After reallocation, all index entries are recreated pointing to new memory
- Old buffer slice becomes eligible for GC (no more references)

**Cost:** O(live slots) iteration, but only happens on buffer growth (rare in steady state).

---

## Configuration

```go
type PoolConfig struct {
    InitialBufferSize int
    ExpectedEntries   int
    GrowthFactor      float64

    DeadRatioThreshold  float64
    MemoryPressureRatio float64

    IdleThreshold time.Duration
}

type SchedulerConfig struct {
    CheckInterval    time.Duration
    MigrateBatchSize int
}
```

---

## Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Handle buffer bit | MSB (bit 31) | Clear separation, slot index preserved |
| Buffer model | Alternating double-buffer | Both handles valid during compaction |
| Buffer lifetime | Per-buffer refCount | Safe release when no handles remain |
| Dedup index | `map[string]Handle` with `unsafe.String` | Zero-copy keys |
| Compaction | Incremental, non-blocking | Pause when activity detected |
| Pool coordination | Global scheduler, round-robin | Prevent starvation |
| Slot reuse | Leave as holes | Simple, reclaim under pressure |

---

## API Summary

```go
// Handle operations
func MakeHandle(slotIdx, bufferBit uint32) Handle
func (h Handle) SlotIndex() uint32
func (h Handle) BufferBit() uint32

// Pool operations
func (p *Pool) Intern(data []byte) Handle
func (p *Pool) Get(h Handle) []byte
func (p *Pool) WriteTo(h Handle, w io.Writer) (int64, error)
func (p *Pool) AddRef(h Handle)
func (p *Pool) Release(h Handle)
func (p *Pool) Length(h Handle) int
```

---

## Related Specs

- `plan/spec-pool-handle-migration.md` - Implementation plan for pool integration
- `plan/spec-attributes-wire.md` - AttributesWire (will use pool handles)

---

**Last Updated:** 2026-01-01
