# Ze Pool Architecture

> **Context:** This pool design is for **API programs** that implement RIB storage.
> The Ze engine does NOT use pools - it passes wire bytes to API programs.
> See `docs/architecture/core-design.md` for the canonical architecture reference.
> See `docs/architecture/rib-transition.md` for the overall architecture.

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Purpose** | Deduplicate attributes/NLRIs in API programs |
| **Location** | API program (Go: `internal/component/bgp/attrpool/`, Python/Rust: implement equivalent) |
| **Key Pattern** | Double-buffer with hybrid handles: `Handle = bufferBit(1) \| poolIdx(5) \| flags(2) \| slot(24)` |
| **Core Types** | `Handle`, `Pool`, `Scheduler` |
| **Key Functions** | `Pool.Intern(data) (Handle, error)`, `Pool.Get()`, `Pool.Release()`, `Pool.MigrateBatch()` |
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
4. **Simple API**: `Intern(data) (Handle, error)`, `Get()`, `Release()` - easy to use
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
│        │ StructuredEvent or formatted JSON event with cached msg-id          │
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
│   │  Receive StructuredEvent or formatted JSON event                  │      │
│   │  Read raw UPDATE sections when the binding includes them          │      │
│   └─────────────────────────────────────────────────────────────────┘      │
│        │                                                                    │
│        │ raw []byte                                                         │
│        ▼                                                                    │
│   ┌─────────────────────────────────────────────────────────────────┐      │
│   │  Pool.Intern(attrBytes) → (Handle, error)                        │      │
│   │  Pool.Intern(nlriBytes) → (Handle, error)                        │      │
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
| Wire bytes | Engine -> API (`StructuredEvent` or raw JSON fields) | Raw BGP data |
| Pool | API program | Deduplication |
| RIB | API program | Route storage |
| msg-id cache | Engine | Zero-copy forwarding |

### API Program Usage

```go
func (s *Server) handleUpdate(event *Event) error {
    // Read raw UPDATE sections from StructuredEvent.RawMessage or from
    // configured raw JSON fields on external process bindings.
    attrBytes := event.RawAttributes
    nlriBytes := event.RawNLRI

    // Store in pool (deduplication)
    attrHandle, err := s.pool.Intern(attrBytes)
    if err != nil {
        return err
    }
    nlriHandle, err := s.pool.Intern(nlriBytes)
    if err != nil {
        _ = s.pool.Release(attrHandle)
        return err
    }

    // Create route with handles
    route := &Route{
        AttrHandle: attrHandle,
        NLRIHandle: nlriHandle,
        MsgID:      event.MsgID,
    }
    s.rib.Insert(event.Peer, route)

    // Tell engine to retain msg-id
    s.send("bgp cache retain %d", event.MsgID)
    return nil
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

Handles encode buffer bit, pool index, and slot in a 32-bit value:

```
┌─────────┬─────────┬──────────────────────────────┐
│BufferBit│ PoolIdx │            Slot              │
│ (1 bit) │ (5 bits)│          (26 bits)           │
└─────────┴─────────┴──────────────────────────────┘
 31        30    26  25                            0
```

| Field | Bits | Range | Purpose |
|-------|------|-------|---------|
| BufferBit | 1 | 0-1 | Which buffer contains data |
| PoolIdx | 5 | 0-30 (31 reserved) | Pool validation |
| Slot | 26 | 0-67M | Entry index (shard id + per-shard slot, see Sharding) |

The 26-bit Slot field is itself split: the high 4 bits hold a **shard id** (0-15) and
the low 22 bits hold the slot index within that shard. `Handle.Slot()` still returns the
full 26-bit composite value, so external callers treat the handle as opaque; only the
`attrpool`-internal `shardID()`/`shardSlot()` helpers split it. See [Sharding](#sharding).

```
Slot (26 bits) = ┌ shardID (4 bits) ┬ per-shard slot (22 bits) ┐
                  25              22  21                       0
```

**Implementation** (`internal/component/bgp/attrpool/handle.go`):
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle type -->

```go
type Handle uint32

// InvalidHandle uses bufferBit=1, poolIdx=31, slot=0x3FFFFFF
const InvalidHandle Handle = 0xFFFFFFFF

// NewHandle creates handle with poolIdx and slot (bufferBit defaults to 0)
func NewHandle(poolIdx uint8, slot uint32) Handle

// NewHandleWithBuffer creates handle with all fields
func NewHandleWithBuffer(bufferBit uint32, poolIdx uint8, slot uint32) Handle

// Accessors
func (h Handle) BufferBit() uint32  // Extract buffer bit (0 or 1)
func (h Handle) PoolIdx() uint8     // Extract pool index (0-30 valid, 31 invalid)
func (h Handle) Slot() uint32       // Extract slot index (0-0x3FFFFFF)
func (h Handle) IsValid() bool      // True if poolIdx < 31

// Modifiers
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
| Buffer tracking | MSB distinguishes buffers during compaction |
| Capacity | 26-bit slot = 67M entries per pool |

**Trade-off:** Max pools reduced from 63 to 31. Sufficient for BGP use.

---

## Pool Structure

A `Pool` is sharded into a per-pool number of independent sub-pools, chosen at
construction in the range `1..numShards` (16) (see [Sharding](#sharding)). The per-pool
state that used to live directly on `Pool` (lock, dedup index, slot table, free list,
double buffer, compaction cursor, activity and metrics counters) now lives on each `shard`.
`Pool` keeps the immutable pool index, the shared shutdown flag, and the shard mask used
to select a shard from a content hash.

```go
type Pool struct {
    // Pool index for handle encoding (0-30, 31 reserved for InvalidHandle)
    idx uint8

    // Shutdown state (shared by all shards)
    shutdown atomic.Bool

    // shardMask = len(shards)-1; selects a shard with a single AND
    shardMask uint32

    // Independent sub-pools selected by shardOf(data, shardMask)
    shards []shard
}

type shard struct {
    mu sync.RWMutex

    id       uint32       // shard index, packed into handle Slot high bits
    idx      uint8        // owning pool index, for handle encoding/validation
    shutdown *atomic.Bool // owning pool's shutdown flag (shared across shards)

    // Double buffer - alternates between compaction cycles
    buffers [2]buffer
    currentBit uint32  // 0 or 1 - which buffer is current

    // Slot table - indexed by handle.shardSlot() (low 22 bits of Slot)
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
    internTotal atomic.Int64  // total Intern() calls routed here
    internHits  atomic.Int64  // deduplication hits
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
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool struct -->

---

## Sharding

`Intern` is called concurrently from every per-peer goroutine (and once per path
attribute on the UPDATE path). With a single `sync.RWMutex` per pool, profiling on a
16-core box showed the write path fully serialized on that one lock and the read path
bottlenecked on the `RWMutex` reader-count word (one contended cache line). To remove
both, each `Pool` is split into a per-pool number of independent shards.

| Constant | Value | Meaning |
|----------|-------|---------|
| `numShards` | 16 | maximum/default shard count per logical pool (`1 << shardIDBits`) |
| `shardIDBits` | 4 | high bits of the 26-bit Slot field that hold the shard id |
| `shardSlotBits` | 22 | low bits of Slot indexing within a shard |
| `MaxSlotsPerShard` | 4,194,304 | per-shard slot capacity (`1 << shardSlotBits`) |
| `MaxSlots` | 67,108,864 | aggregate at the max shard count (`numShards * MaxSlotsPerShard`) |

**Per-pool shard count.** The shard count is fixed at construction:
`NewWithShards(idx, capacity, shards)` accepts any power of two in `1..numShards`;
`New`/`NewWithIdx` default to `numShards`. Content-hash sharding only relieves `Intern`
lock contention when a pool's *hot* values are diverse enough to spread across shards. A
pool dominated by a few values (ORIGIN has three; ATOMIC_AGGREGATE is a single zero-length
flag) has its hot value monopolize one shard, gaining nothing while paying fixed per-shard
overhead, so those pools are created with **1 shard**. A 1-shard pool is exactly the
pre-sharding single-lock pool: one lock, one index, one buffer, shard id always 0. High
cardinality, per-route attributes (AS_PATH, communities, MED, next-hop, labels, the wire
RibOut blobs) use the full `numShards`. The per-attribute split lives in
`internal/component/bgp/plugins/rib/pool`.

**Shard selection.** `shardOf(data, mask)` is an allocation-free FNV-1a hash of the bytes,
xor-folded (high half into low) and masked with the pool's `shardMask` (`len(shards)-1`).
The fold matters because FNV-1a's final step is a multiply, whose low bits carry the
weakest mixing; masking them directly would skew distribution. Because selection depends
only on content, identical bytes always map to the same shard, which is what preserves the
global deduplication guarantee. The shard id is then packed into the high 4 bits of the
handle's Slot field, so `Get`/`Release` decode it (`Handle.shardID()`) to route a handle
back to the shard that owns it; within the shard the low 22 bits (`Handle.shardSlot()`)
index the slot table. Routing bounds-checks the decoded shard id against `len(shards)`, so
a stray or foreign handle returns `ErrWrongPool`/`ErrSlotOutOfBounds` instead of indexing
out of range (a fixed `[16]shard` array gave this for free; a variable-length slice does not).

**Concurrency.** Each shard owns its own `RWMutex`, dedup index, slot table and double
buffer, so writes to different shards proceed in parallel and each shard's reader-count
word sits on its own cache line. `Metrics()` sums per-shard counters; `IsIdle` is true
only when every shard is idle; `Touch` marks all shards.

**Compaction is decided per shard, gated per pool.** The `Scheduler` round-robins over
`(pool, shard)` units, not whole pools, and compacts one shard at a time. *Which* shard is
chosen is decided by that shard's own dead ratio (`deadRatioExceeds`), so a single
heavily-fragmented shard is never starved by a low pool-wide average. *Whether* to run is
gated by the owning pool's quiet period (`Pool.IsIdle`): compaction copies bytes, so it
stays out of the way during a pool's activity burst rather than only when the one shard it
is migrating is quiet. The dead-ratio check has an O(1) fast path (an empty free list means
no dead slots) so the per-tick scan over many shards stays cheap.
`StartCompaction`/`MigrateBatch`/`Compact`/`CheckOldBufferRelease` still exist as pool-level
fan-outs (each shard takes only its own lock); `State()` reports `PoolCompacting` if any
shard is compacting. Handles stay valid throughout because a shard frees an old buffer only
once its reference count reaches zero.

The Handle ABI is unchanged (still 32 bits; `PoolIdx`/`BufferBit`/`Slot`/`IsValid` keep
their meaning), so external callers, the wire/storage layers, and `BundlePool` (which
builds its own handles via `NewHandle` and never routes through the sharded `Intern`) are
all unaffected.

<!-- source: internal/component/bgp/attrpool/pool.go -- shard struct, shardOf, numShards -->
<!-- source: internal/component/bgp/attrpool/handle.go -- shardID, shardSlot, packShardSlot -->

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
// Intern stores data with deduplication.
// Returns ErrPoolShutdown, ErrDataTooLarge, or ErrPoolFull on failure.
func (p *Pool) Intern(data []byte) (Handle, error)
```
<!-- source: internal/component/bgp/attrpool/pool.go -- Intern -->

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
storedSlot := handle.Slot()  // Extract 26-bit slot only

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
<!-- source: internal/component/bgp/attrpool/scheduler.go -- Scheduler, SchedulerConfig -->

Scheduler behavior:
1. Check if any pool has recent activity (within QuietPeriod)
2. If activity: pause compaction
3. If idle: continue active compaction or find next pool
4. Pool selected if dead ratio >= threshold
5. Round-robin prevents any pool from starvation

### RIB Plugin Lifecycle Wiring

The scheduler is wired into the RIB plugin via `OnStarted`:

| Lifecycle Event | Action |
|----------------|--------|
| Plugin startup (OnStarted) | `go runCompaction(ctx, pool.AllPools())` |
| Route churn | Dead bytes accumulate, scheduler triggers compaction |
| Plugin shutdown (context cancel) | Scheduler exits, goroutine stops |

Implementation: `internal/component/bgp/plugins/rib/compaction.go` (thin wiring), `rib.go` (OnStarted callback).

`pool.AllPools()` in `internal/component/bgp/plugins/rib/pool/attributes.go` returns all 13 attribute pools.
<!-- source: internal/component/bgp/plugins/rib/ -- compaction wiring, AllPools -->

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

**Implementation:** See `internal/component/bgp/attrpool/pool.go:rebuildIndex()`
<!-- source: internal/component/bgp/attrpool/pool.go -- rebuildIndex -->

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
func NewHandle(poolIdx uint8, slot uint32) Handle
func NewHandleWithBuffer(bufferBit uint32, poolIdx uint8, slot uint32) Handle

// Handle accessors
func (h Handle) BufferBit() uint32
func (h Handle) PoolIdx() uint8
func (h Handle) Slot() uint32
func (h Handle) IsValid() bool

// Handle modifiers
func (h Handle) WithBufferBit(bit uint32) Handle

// Pool creation
func New(initialCapacity int) *Pool                                    // idx 0, default shard count
func NewWithIdx(idx uint8, initialCapacity int) (*Pool, error)         // default shard count
func NewWithShards(idx uint8, initialCapacity, shards int) (*Pool, error) // shards: power of two, 1..16

// Core operations
func (p *Pool) Intern(data []byte) (Handle, error)
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

Ze provides pre-configured global pools in `internal/component/bgp/plugins/rib/pool/attributes.go`:

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
<!-- source: internal/component/bgp/plugins/rib/pool/attributes.go -- per-attribute pool instances -->

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
- `internal/component/bgp/attrpool/` - Pool implementation
- `internal/component/bgp/plugins/rib/storage/` - RIB storage using pool
- `internal/component/bgp/plugins/rib/storage/familyrib_perattr.go` - Per-attribute RIB storage

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

## Memory Profile (Measured 2026-05-25, ribOut optimized 2026-05-26)

Three storage layers hold route data. Each has different memory
characteristics. Measured at 100K IPv4/32 routes, Apple M4 Max, Go 1.26.

### Per-Route Memory by Layer

| Layer | Location | Struct | Measured | Allocs | Per-peer? |
|-------|----------|--------|----------|--------|-----------|
| Plugin RIB (adj-rib-in) | `plugins/rib/storage/` | 32 B | **69 B** | 1.0 | No |
| Engine OutgoingRIB | `bgp/rib/outgoing.go` | 160 B | **478 B** | 10.0 | Yes (test-only, no production callers) |
| Plugin ribOut (before) | `bgp/route.go` in `rib.go` | 288 B | **385-741 B** | 6-10 | Yes |
| Plugin ribOut (after) | `plugins/rib/ribout_entry.go` | 16 B | **~16 B** + shared pool | 0 | Yes (entry) / No (pool) |

"Measured" = TotalAlloc / routes. Includes struct, backing data (strings,
slices, wire bytes), and map overhead. "Per-peer?" means the data is
duplicated for every peer.

### ribOut Compact Storage (pool.RibOut, idx 16)

The plugin ribOut now stores a 16 B `ribOutEntry` per peer per route:
`MsgID` (8 B) + `AttrHandle` (4 B) + `StaleLevel` (1 B) + padding (3 B).
Wire attribute bytes are deduplicated in `pool.RibOut`: the same UPDATE
sent to N peers stores one pool copy and N 4-byte handles.

Full `*Route` is reconstructed on demand (cold paths only: replay, show,
refresh) by parsing wire bytes from the pool via `reconstructRoute()`.

Source peer tracking uses a separate refcounted map (`ribOutSource`)
with one entry per unique route, not per destination peer.

### Scaling Impact

| Scenario | Plugin RIB | Plugin ribOut | Shared Pool | Total |
|----------|-----------|---------------|-------------|-------|
| 100K routes, 1 peer | 7 MB | 2 MB | ~7 MB | 16 MB |
| 100K routes, 10 peers | 7 MB | 15 MB | ~7 MB | 29 MB |
| 1M routes, 10 peers | 66 MB | 153 MB | ~70 MB | 289 MB |
| 1M routes, 50 peers | 66 MB | 763 MB | ~70 MB | 899 MB |

Engine OutgoingRIB is excluded: it has no production callers (test-only).
Previous totals included it at 478 B/route/peer, inflating projections.

### Where the Bytes Go

**Plugin ribOutEntry (16 B):** MsgID (uint64, 8 B) + AttrHandle (uint32,
4 B) + StaleLevel (uint8, 1 B) + padding (3 B). Zero heap allocations
per entry. Map overhead adds ~50 B/entry.

**Plugin RIB RouteEntry (69 B):** 32 B struct (Bundle handle + ASPath handle +
fingerprint + stale level) + 37 B BART trie node overhead. Pool attribute
data is shared across routes, not counted per-route.

**Shared pool (pool.RibOut):** Wire attribute bytes stored once. Typical
UPDATE attributes are 40-200 bytes. Deduplication ratio is high when
the same UPDATE is forwarded to multiple peers.

---

**Last Updated:** 2026-05-26
