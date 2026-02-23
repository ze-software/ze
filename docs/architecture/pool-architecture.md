# Ze Pool Architecture

> **Context:** This pool design is for **API programs** that implement RIB storage.
> The Ze engine does NOT use pools - it passes wire bytes to API programs.
> See `docs/architecture/core-design.md` for the canonical architecture reference.
> See `docs/architecture/rib-transition.md` for the overall architecture.

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Purpose** | Deduplicate attributes/NLRIs in API programs |
| **Location** | API program (Go: `internal/attrpool/`, Python/Rust: implement equivalent) |
| **Key Pattern** | Double-buffer with hybrid handles: `Handle = bufferBit(1) \| poolIdx(5) \| flags(2) \| slot(24)` |
| **Core Types** | `Handle`, `Pool`, `Scheduler` |
| **Key Functions** | `Pool.Intern()`, `Pool.Get()`, `Pool.Release()`, `Pool.MigrateBatch()` |
| **Input** | Base64-decoded wire bytes from engine events |

**When to read full doc:** Implementing RIB in Go, memory optimization, compaction.

**For other languages:** Implement simpler dedup (hash map) or skip dedup entirely.

---

Memory-efficient attribute and NLRI deduplication for API programs.

---

## Design Goals

1. **Memory efficiency**: Deduplicate identical attributes/NLRIs across all peers
2. **Non-blocking**: Incremental compaction, no stop-the-world pauses
3. **Scalable**: Handle millions of routes with bounded memory
4. **Simple API**: `Intern()`, `Get()`, `Release()` - easy to use
5. **Polyglot friendly**: Design can be implemented in any language

---

## Data Flow

The pool lives in the **API program**, not the engine. Wire bytes flow from engine to API:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Ze ENGINE                                       │
│                                                                             │
│   Network recv()                                                            │
│        │                                                                    │
│        ▼                                                                    │
│   ┌─────────────────────────────────────────────────────────────────┐      │
│   │  Parse UPDATE, extract wire bytes                                │      │
│   │  Assign msg-id, cache wire bytes                                 │      │
│   └─────────────────────────────────────────────────────────────────┘      │
│        │                                                                    │
│        │ JSON event with base64 wire bytes                                  │
│        ▼                                                                    │
└─────────────────────────────────────────────────────────────────────────────┘
                              │
════════════════════════════ PROCESS BOUNDARY ═════════════════════════════════
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API PROGRAM                                        │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────┐      │
│   │  Receive JSON event                                              │      │
│   │  Decode base64: attrBytes, nlriBytes                             │      │
│   └─────────────────────────────────────────────────────────────────┘      │
│        │                                                                    │
│        │ raw []byte                                                         │
│        ▼                                                                    │
│   ┌─────────────────────────────────────────────────────────────────┐      │
│   │  Pool.Intern(attrBytes) → Handle                                 │      │
│   │  Pool.Intern(nlriBytes) → Handle                                 │      │
│   │                                                                  │      │
│   │  Deduplication happens here:                                     │      │
│   │    - Identical attributes → same handle (no new allocation)     │      │
│   │    - New attributes → stored in pool buffer                      │      │
│   └─────────────────────────────────────────────────────────────────┘      │
│        │                                                                    │
│        ▼                                                                    │
│   ┌─────────────────────────────────────────────────────────────────┐      │
│   │  RIB Storage                                                     │      │
│   │    Route stores pool.Handle (4 bytes) + msg-id                  │      │
│   │    Multiple routes with same attrs → share storage              │      │
│   └─────────────────────────────────────────────────────────────────┘      │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Principles

| Component | Location | Purpose |
|-----------|----------|---------|
| Wire bytes | Engine → API (base64) | Raw BGP data |
| Pool | API program | Deduplication |
| RIB | API program | Route storage |
| msg-id cache | Engine | Zero-copy forwarding |

### API Program Usage

```go
func (s *Server) handleUpdate(event *Event) {
    // Decode base64 wire bytes from event
    attrBytes, _ := base64.StdEncoding.DecodeString(event.RawAttributes)
    nlriBytes, _ := base64.StdEncoding.DecodeString(event.RawNLRI)

    // Store in pool (deduplication)
    attrHandle := s.pool.Intern(attrBytes)
    nlriHandle := s.pool.Intern(nlriBytes)

    // Create route with handles
    route := &Route{
        AttrHandle: attrHandle,
        NLRIHandle: nlriHandle,
        MsgID:      event.MsgID,
    }
    s.rib.Insert(event.Peer, route)

    // Tell engine to retain msg-id
    s.send("msg-id %d retain", event.MsgID)
}
```

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

## Handle Design (Hybrid Layout)

Handles encode buffer bit, pool index, flags, and slot in a 32-bit value:

```
┌─────────┬─────────┬───────┬────────────────────────┐
│BufferBit│ PoolIdx │ Flags │        Slot            │
│ (1 bit) │ (5 bits)│(2 bit)│      (24 bits)         │
└─────────┴─────────┴───────┴────────────────────────┘
 31        30    26  25   24  23                    0
```

| Field | Bits | Range | Purpose |
|-------|------|-------|---------|
| BufferBit | 1 | 0-1 | Which buffer contains data |
| PoolIdx | 5 | 0-30 (31 reserved) | Pool validation |
| Flags | 2 | 0-3 | ADD-PATH support (bit 0 = hasPathID) |
| Slot | 24 | 0-16M | Entry index |

**Implementation** (`internal/attrpool/handle.go`):

```go
type Handle uint32

// InvalidHandle uses bufferBit=1, poolIdx=31, flags=3, slot=0xFFFFFF
const InvalidHandle Handle = 0xFFFFFFFF

// NewHandle creates handle with poolIdx, flags, slot (bufferBit defaults to 0)
func NewHandle(poolIdx uint8, flags uint8, slot uint32) Handle

// NewHandleWithBuffer creates handle with all fields
func NewHandleWithBuffer(bufferBit uint32, poolIdx uint8, flags uint8, slot uint32) Handle

// Accessors
func (h Handle) BufferBit() uint32  // Extract buffer bit (0 or 1)
func (h Handle) PoolIdx() uint8     // Extract pool index (0-30 valid, 31 invalid)
func (h Handle) Flags() uint8       // Extract flags (0-3)
func (h Handle) Slot() uint32       // Extract slot index (0-0xFFFFFF)
func (h Handle) HasPathID() bool    // True if ADD-PATH flag set
func (h Handle) IsValid() bool      // True if poolIdx < 31

// Modifiers
func (h Handle) WithFlags(flags uint8) Handle       // Change flags only
func (h Handle) WithBufferBit(bit uint32) Handle    // Change bufferBit only
```

### Handle Number Space

```
Buffer 0 handles: 0x00000000 - 0x7EFFFFFF (poolIdx < 31)
Buffer 1 handles: 0x80000000 - 0xFEFFFFFF (poolIdx < 31)

InvalidHandle:    0xFFFFFFFF (poolIdx = 31)
```

### Benefits of Hybrid Design

| Aspect | Benefit |
|--------|---------|
| Pool validation | Each pool validates handles belong to it via poolIdx |
| ADD-PATH support | Flags encode path-id presence for BGP |
| Buffer tracking | MSB distinguishes buffers during compaction |
| Capacity | 24-bit slot = 16.7M entries per pool |

**Trade-off:** Max pools reduced from 63 to 31. Sufficient for BGP use.

---

## Pool Structure

```go
type Pool struct {
    mu sync.RWMutex

    // Pool index for handle encoding (0-30, 31 reserved for InvalidHandle)
    idx uint8

    // Double buffer - alternates between compaction cycles
    buffers [2]buffer
    currentBit uint32  // 0 or 1 - which buffer is current

    // Slot table - indexed by handle.Slot()
    slots []slot

    // Free list for slot reuse
    freeSlots []uint32

    // Dedup index: data content → Handle (always points to current buffer)
    // Keys are unsafe.String pointing directly into buffer (zero-copy)
    index map[string]Handle

    // Compaction state
    state            PoolState
    compactCursor    uint32  // Migration progress (slot index)
    compactSlotCount uint32  // Slot count when compaction started

    // Activity tracking for scheduler
    lastActivity atomic.Int64

    // Metrics counters
    internTotal atomic.Int64  // total Intern() calls
    internHits  atomic.Int64  // deduplication hits

    // Shutdown state
    shutdown atomic.Bool
}

type buffer struct {
    data     []byte
    pos      int            // write cursor
    refCount atomic.Int32   // handles pointing here
}

type slot struct {
    offsets  [2]uint32  // offset in EACH buffer (both valid during compaction)
    length   uint16     // data length
    refCount int32      // reference count
    dead     bool       // marked for removal
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
// Intern stores data with deduplication. Returns handle to retrieve data.
// Panics on error. Use InternWithError for error returns.
func (p *Pool) Intern(data []byte) Handle

// InternWithError returns error instead of panic.
// Returns ErrPoolShutdown, ErrDataTooLarge, or ErrPoolFull.
func (p *Pool) InternWithError(data []byte) (Handle, error)
```

Behavior:
1. Check dedup index for existing entry
2. If found: increment refCount, return existing handle
3. If new: allocate slot, copy data to current buffer, index with zero-copy key
4. Handle encodes pool idx and current buffer bit

### Get (Read Data)

```go
// Get returns data for handle. Returns zero-copy slice into pool buffer.
// Returns error if handle invalid, wrong pool, or slot dead.
func (p *Pool) Get(h Handle) ([]byte, error)
```

Validates handle pool idx matches, slot in bounds, not dead.

### GetBySlot (Read Data by Slot Index)

Used when handles are stored normalized (slot only, no bufferBit).
Automatically selects the correct buffer based on compaction state.

```go
// GetBySlot returns data for normalized slot index.
// Auto-selects buffer: current if migrated, old if not yet migrated.
func (p *Pool) GetBySlot(slotIdx uint32) ([]byte, error)
```

### Handle Normalization

When storing handles in compound structures, you can normalize by
extracting just the slot. Use `GetBySlot()` to retrieve data:

```go
// Store normalized:
storedSlot := handle.Slot()  // Extract 24-bit slot only

// Retrieve later:
data, err := pool.GetBySlot(storedSlot)  // Auto-selects correct buffer
```

### Length (Get Data Length)

```go
// Length returns data length without copying data.
func (p *Pool) Length(h Handle) (int, error)
```

### AddRef (Share Reference)

```go
// AddRef increments refcount for handle sharing between owners.
// Returns error if handle invalid or wrong pool.
func (p *Pool) AddRef(h Handle) error
```

### Release (Decrement Reference)

```go
// Release decrements refcount. When refCount reaches 0, slot marked dead.
// Returns error if handle invalid, wrong pool, or already dead.
func (p *Pool) Release(h Handle) error
```

When refCount reaches 0:
- Slot marked dead
- Entry removed from dedup index
- Slot added to free list for reuse

### ReleaseBySlot (Release by Slot Index)

Used when handles are stored normalized (slot only).

```go
// ReleaseBySlot decrements refcount for normalized slot.
// Auto-selects correct buffer based on compaction state.
func (p *Pool) ReleaseBySlot(slotIdx uint32) error
```

---

## Incremental Compaction

### Start Compaction

```go
// StartCompaction begins incremental compaction.
// Allocates new buffer, sets state to PoolCompacting.
// Call MigrateBatch() repeatedly until it returns true.
func (p *Pool) StartCompaction()
```

Behavior:
1. Flip currentBit (0→1 or 1→0)
2. Allocate new buffer with liveBytes + 25% headroom
3. Set state to PoolCompacting, cursor to 0
4. Record slot count (don't migrate slots created during compaction)

### Migrate Batch

```go
// MigrateBatch migrates batchSize slots to new buffer.
// Returns true when migration complete.
// Call repeatedly until returns true, then call CheckOldBufferRelease.
func (p *Pool) MigrateBatch(batchSize int) bool

// CheckOldBufferRelease checks if old buffer can be freed.
// Call periodically after MigrateBatch returns true.
// Old buffer freed when its refCount reaches 0.
func (p *Pool) CheckOldBufferRelease()

// Compact performs stop-the-world compaction (legacy).
// No-op if incremental compaction in progress.
// Prefer StartCompaction/MigrateBatch for non-blocking.
func (p *Pool) Compact()

// State returns current compaction state.
func (p *Pool) State() PoolState
```

Behavior:
1. Copy live slots from old buffer to new buffer
2. Update slot offsets and dedup index
3. Skip slots created during compaction (compactSlotCount)
4. When cursor reaches end, return true
5. Old buffer freed when all handles released

---

## Global Compaction Scheduler

One pool compacts at a time. Pauses when activity detected. Round-robin prevents starvation.

```go
type Scheduler struct {
    pools  []*Pool
    config SchedulerConfig
    // ... internal state
}

type SchedulerConfig struct {
    QuietPeriod        time.Duration  // Default: 100ms
    CheckInterval      time.Duration  // Default: 50ms
    DeadRatioThreshold float64        // Default: 0.25 (25%)
    MigrateBatchSize   int            // Default: 100
}

func NewScheduler(pools []*Pool, config SchedulerConfig) *Scheduler

// Run starts scheduler loop. Blocks until context canceled.
func (s *Scheduler) Run(ctx context.Context)
```

Scheduler behavior:
1. Check if any pool has recent activity (within QuietPeriod)
2. If activity: pause compaction
3. If idle: continue active compaction or find next pool
4. Pool selected if dead ratio >= threshold
5. Round-robin prevents any pool from starvation

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

When buffer capacity is exceeded, the pool must:

1. Allocate larger buffer (2x growth)
2. Copy existing data
3. Rebuild dedup index (old keys reference deallocated memory)

**Index rebuild behavior:**
- Iterates all live slots
- Creates new index entries with keys pointing to new buffer memory
- Old buffer slice becomes eligible for GC

**Cost:** O(live slots) iteration, but only happens on buffer growth (rare in steady state).

**Implementation:** See `internal/attrpool/pool.go:rebuildIndex()`

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
| Handle layout | Hybrid: bufferBit(1) + poolIdx(5) + flags(2) + slot(24) | Pool validation, ADD-PATH flags, buffer tracking |
| InvalidHandle | 0xFFFFFFFF (poolIdx=31) | Reserved poolIdx ensures IsValid() = false |
| Buffer model | Alternating double-buffer | Both handles valid during compaction |
| Buffer lifetime | Per-buffer refCount | Safe release when no handles remain |
| Dedup index | `map[string]Handle` with `unsafe.String` | Zero-copy keys |
| Compaction | Incremental, non-blocking | Pause when activity detected |
| Pool coordination | Global scheduler, round-robin | Prevent starvation |
| Slot reuse | Free list | O(1) allocation after release |
| Error handling | Return errors (not panic) | Caller can handle gracefully |

---

## API Summary

```go
// Handle creation
func NewHandle(poolIdx uint8, flags uint8, slot uint32) Handle
func NewHandleWithBuffer(bufferBit uint32, poolIdx uint8, flags uint8, slot uint32) Handle

// Handle accessors
func (h Handle) BufferBit() uint32
func (h Handle) PoolIdx() uint8
func (h Handle) Flags() uint8
func (h Handle) Slot() uint32
func (h Handle) HasPathID() bool
func (h Handle) IsValid() bool

// Handle modifiers
func (h Handle) WithFlags(flags uint8) Handle
func (h Handle) WithBufferBit(bit uint32) Handle

// Pool creation
func New(initialCapacity int) *Pool
func NewWithIdx(idx uint8, initialCapacity int) *Pool

// Core operations
func (p *Pool) Intern(data []byte) Handle
func (p *Pool) InternWithError(data []byte) (Handle, error)
func (p *Pool) Get(h Handle) ([]byte, error)
func (p *Pool) Length(h Handle) (int, error)
func (p *Pool) AddRef(h Handle) error
func (p *Pool) Release(h Handle) error

// Normalized access (by slot)
func (p *Pool) GetBySlot(slotIdx uint32) ([]byte, error)
func (p *Pool) ReleaseBySlot(slotIdx uint32) error

// Compaction
func (p *Pool) StartCompaction()
func (p *Pool) MigrateBatch(batchSize int) bool
func (p *Pool) CheckOldBufferRelease()
func (p *Pool) Compact()
func (p *Pool) State() PoolState

// Lifecycle
func (p *Pool) Shutdown()
func (p *Pool) IsShutdown() bool
func (p *Pool) Metrics() Metrics

// Activity tracking
func (p *Pool) Touch()
func (p *Pool) IsIdle(d time.Duration) bool
```

---

## Global Pool Instances

Ze provides pre-configured global pools in `internal/plugins/bgp-rib/pool/attributes.go`:

### Per-Attribute-Type Pools

For fine-grained deduplication when routes share some but not all attributes:

| Pool | Index | Initial Size | Purpose |
|------|-------|--------------|---------|
| `Origin` | 2 | 64B | ORIGIN (3 values: IGP, EGP, INCOMPLETE) |
| `ASPath` | 3 | 256KB | AS_PATH (RFC 4271) |
| `LocalPref` | 4 | 4KB | LOCAL_PREF (RFC 4271) |
| `MED` | 5 | 16KB | MULTI_EXIT_DISC (RFC 4271) |
| `NextHop` | 6 | 16KB | NEXT_HOP (RFC 4271) |
| `Communities` | 7 | 64KB | COMMUNITIES (RFC 1997) |
| `LargeCommunities` | 8 | 16KB | LARGE_COMMUNITIES (RFC 8092) |
| `ExtCommunities` | 9 | 16KB | EXTENDED_COMMUNITIES (RFC 4360) |
| `ClusterList` | 10 | 4KB | CLUSTER_LIST (RFC 4456) |
| `OriginatorID` | 11 | 4KB | ORIGINATOR_ID (RFC 4456) |
| `AtomicAggregate` | 12 | 64B | ATOMIC_AGGREGATE (RFC 4271) |
| `Aggregator` | 13 | 4KB | AGGREGATOR (RFC 4271) |
| `OtherAttrs` | 14 | 64KB | Unknown/unhandled attributes |

### Usage Pattern

**Per-attribute** (fine-grained deduplication):
```go
entry, _ := storage.ParseAttributes(attrBytes)  // Parses into per-type handles
// entry.Origin, entry.ASPath, etc. are individual pool handles
// Access: data, _ := pool.Origin.Get(entry.Origin)
```

**Memory improvement:** Routes with identical ORIGIN/LOCAL_PREF but different MED share ORIGIN/LOCAL_PREF pool entries instead of duplicating the entire blob.

---

## Related Docs

- `docs/architecture/rib-transition.md` - Overall architecture (RIB in API)
- `internal/attrpool/` - Pool implementation
- `internal/plugin/rib/storage/` - RIB storage using pool
- `internal/plugin/rib/storage/familyrib_perattr.go` - Per-attribute RIB storage

---

## Polyglot Alternatives

For non-Go API programs, simpler approaches work:

### Python

```python
# Simple dict-based dedup
class Pool:
    def __init__(self):
        self.data = {}  # bytes -> handle
        self.handles = {}  # handle -> bytes
        self.next_handle = 0

    def intern(self, data: bytes) -> int:
        key = data
        if key in self.data:
            return self.data[key]
        handle = self.next_handle
        self.next_handle += 1
        self.data[key] = handle
        self.handles[handle] = data
        return handle

    def get(self, handle: int) -> bytes:
        return self.handles[handle]
```

### Rust

```rust
use std::collections::HashMap;

struct Pool {
    data: HashMap<Vec<u8>, u32>,
    handles: HashMap<u32, Vec<u8>>,
    next_handle: u32,
}

impl Pool {
    fn intern(&mut self, data: Vec<u8>) -> u32 {
        if let Some(&h) = self.data.get(&data) {
            return h;
        }
        let handle = self.next_handle;
        self.next_handle += 1;
        self.data.insert(data.clone(), handle);
        self.handles.insert(handle, data);
        handle
    }
}
```

### No Dedup

For simplicity, store raw bytes directly (higher memory, simpler code):

```python
# 1M routes × 200 bytes = ~200 MB
routes = {}  # (peer, prefix) -> {'attrs': bytes, 'nlri': bytes, 'msg_id': int}
```

---

**Last Updated:** 2026-01-30
