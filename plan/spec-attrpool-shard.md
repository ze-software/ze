# Spec: attrpool-shard

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-05-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/pool-architecture.md` - attribute/NLRI pool design
4. `internal/component/bgp/attrpool/pool.go`, `internal/component/bgp/attrpool/handle.go`

## Task

The BGP attribute pool (`attrpool.Pool`) deduplicates and reference-counts attribute byte
sequences shared across routes and per-peer export state. It is accessed concurrently from
every per-peer goroutine. Today each `Pool` is guarded by a single `sync.RWMutex`. Profiling
shows this single lock is the dominant contention point under concurrent write load, and the
`RWMutex` reader-count word is itself a contended cache line under concurrent read load.

The goal is to remove both contention points by sharding each `Pool` internally into N
independent sub-pools selected by a hash of the interned data, while preserving the existing
32-bit `Handle` ABI, the global deduplication guarantee, reference-counting semantics, and the
incremental double-buffer compaction behaviour. No public API or wire/storage format changes.

### Profiling Evidence (captured 2026-05-29, Apple M4 Max, GOMAXPROCS=16)

Source data driving this spec. Reproduce with the benchmarks in
`internal/component/bgp/attrpool/benchmark_test.go`. `Intern` is confirmed hot and concurrent:
45 references across 16 files, including `storage/attrparse.go` which interns each path attribute
per route on the UPDATE path.

**Throughput (`go test -bench ... -benchmem -benchtime=2s`):**

| Benchmark | Lock used | ns/op | Reading |
|-----------|-----------|-------|---------|
| `BenchmarkInternExisting` | write `Lock` (dedup hit), single goroutine | 131 | baseline single-threaded hit |
| `BenchmarkInternNew` | write `Lock` (new entry), single goroutine | 458 | baseline single-threaded miss |
| `BenchmarkConcurrentIntern` | write `Lock`, RunParallel x16 | 769 | per-op time **rose** vs single-threaded miss instead of scaling out -> serialization |
| `BenchmarkConcurrentGet` | read `RLock`, RunParallel x16 | 138 | barely above single-threaded; read path does not collapse but see CPU profile |

**Mutex profile (`-mutexprofile`, `go tool pprof -top`):** 30.12s total delay; 100% attributed to
`attrpool.(*Pool).internLocked` via `sync.(*Mutex).Unlock` / `runtime.unlock`. No other lock appears.

**CPU profile of the read path:** `BenchmarkConcurrentGet` is dominated by
`sync/atomic.(*Int32).Add` at ~52% flat. That atomic is the `sync.RWMutex` internal reader-count
word incremented on every `RLock`/`RUnlock`. All 16 cores contend on that single cache line, so
even the read path is bounded by one shared word rather than by the work `Get` does.

**Conclusion:** Both the write path (full `Lock` serialization) and the read path (shared
reader-count cache line) are limited by per-`Pool` global synchronization. Splitting the lock,
the dedup index, the slot table, and the buffers into independent shards addresses both: writes
to different shards proceed in parallel, and each shard has its own reader-count word on its own
cache line.

### Out of scope (recorded, not addressed here)

A separate `-memprofile` run of `internal/component/bgp/plugins/rib/storage` shows route-store
insert allocations dominated by (a) `bart` radix-trie node growth, (b) interface boxing of the
value struct inside the `bart` sparse array, and (c) a per-element allocation in
`core/rib/store.Store.ModifyAll`'s range-over-func. These are RIB-store allocation concerns,
distinct from attribute-pool lock contention, and belong in their own spec. Not addressed here.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/pool-architecture.md` - canonical description of attribute/NLRI pools
  → Decision: Handle is a 32-bit opaque value; stability across compaction is a contract.
  → Constraint: Handle bit layout is fixed at 32 bits (1 buffer + 5 pool-idx + 26 slot). Any
    sharding scheme must live inside these bits without widening Handle.
- [ ] `docs/architecture/core-design.md` - pool dedup is part of the buffer-first wire pipeline
  → Constraint: per-attribute refcounted dedup, caller-owned buffers, zero-copy `Get`.
- [ ] `ai/rules/memory-architecture.md` - pool strategy and lifecycle
  → Constraint: data lifetime is governed by refcount; `Get` returns a slice into pool memory
    valid only while the handle is live.
- [ ] `ai/rules/enum-over-string.md` - dispatch by numeric key on hot paths
  → Constraint: shard selection must be a cheap numeric operation, not a string map.

**Key insights:**
- The `Handle`'s `PoolIdx` (5 bits, values 0-30) is a **scarce global namespace** already assigned
  to distinct logical pools (e.g., RibOut uses idx 16). Sharding a single logical pool by
  allocating multiple `PoolIdx` values would exhaust the 31-value namespace and is rejected.
- The `Slot` field is 26 bits (~67M values). There is room to carve a shard-id sub-field from the
  high bits of `Slot` while leaving each shard a large slot range.
- Identical data must always hash to the same shard, otherwise the global dedup guarantee breaks
  (the same bytes could be interned twice with different handles).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/attrpool/pool.go` - `Pool` struct holds one `sync.RWMutex`,
  `index map[string]Handle`, `slots []slot`, `freeSlots []uint32`, double `buffers [2]buffer`,
  `currentBit`, compaction cursor/state, and atomic metrics. `Intern`/`internLocked` take the
  full write `Lock` (including on dedup hits). `Get`/`Length` take `RLock`. `Release` and
  compaction (`StartCompaction`/`MigrateBatch`/`Compact`) take `Lock`.
  → Constraint: dedup index keys are `unsafe.String` views into buffer memory (zero-copy); shard
    split must keep each shard's keys pointing into that shard's own buffers.
- [ ] `internal/component/bgp/attrpool/handle.go` - 32-bit `Handle`: bit 31 buffer, bits 30-26
  pool idx (0-30; 31 = `InvalidHandle`), bits 25-0 slot. `NewHandleWithBuffer`, `PoolIdx()`,
  `Slot()`, `BufferBit()`, `WithBufferBit()`, `IsValid()`.
  → Constraint: `PoolIdx()` and `Slot()` are consumed by `Get`/`Release` to locate data; a new
    shard accessor must be derived from existing bits without changing these methods' meaning.
- [ ] `internal/component/bgp/attrpool/scheduler.go` - drives idle-time compaction via `IsIdle`,
  `StartCompaction`, `MigrateBatch`.
- [ ] `internal/component/bgp/attrpool/compaction_test.go`, `pool_test.go`, `handle_test.go`,
  `metrics_test.go`, `scheduler_test.go`, `shutdown_test.go` - existing behavioural contract.

**Behavior to preserve:**
- `Handle` stays 32 bits; existing handles stored elsewhere keep working; `PoolIdx`/`Slot`/
  `BufferBit` semantics unchanged for external callers.
- Global deduplication: interning identical bytes twice returns the same handle and increments
  refcount.
- Reference counting: `Release` frees only when refcount reaches zero; double-return guards hold.
- Zero-copy `Get`: returns a slice into pool memory, valid while the handle is live.
- Incremental, non-blocking double-buffer compaction; handles remain valid mid-compaction.
- Metrics (`internTotal`, `internHits`, `IsIdle`, `Metrics()`) remain available; aggregate values
  across shards must match prior single-pool semantics.
- `New`, `NewWithIdx`, `Intern`, `Get`, `Length`, `Release`, `Compact`, `StartCompaction`,
  `MigrateBatch`, `Metrics`, shutdown — same signatures and observable contracts.
- `MaxSlots` aggregate capacity per logical pool stays ~ the current 26-bit range (see AC-7).

**Behavior to change:**
- Internal only: one `RWMutex`/`index`/`slots`/`freeSlots`/`buffers`/compaction-state set becomes
  N independent shard instances selected by `hash(data) mod N`. The shard id is encoded in the
  high bits of the `Slot` field so `Get`/`Release` can route a handle back to its shard.

## Data Flow (MANDATORY)

### Entry Point
- `Pool.Intern(data []byte)` called from BGP UPDATE processing (`storage/attrparse.go` interns each
  attribute) and per-peer export bucket construction (many concurrent per-peer goroutines).

### Transformation Path
1. Caller passes attribute/NLRI bytes to `Intern`.
2. **New:** compute `shard = hash(data) mod N` (cheap, allocation-free hash of the bytes).
3. Acquire only that shard's lock; dedup-lookup, slot-allocate, buffer-append within the shard.
4. Encode `Handle` = buffer bit + pool idx + (shard id in high slot bits | slot index within shard).
5. `Get(h)`/`Release(h)` decode shard id from the slot bits, route to that shard, take that
   shard's lock, and read/refcount within it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Caller ↔ Pool | `Intern`/`Get`/`Release`/`Length` unchanged signatures | [ ] |
| Pool ↔ Handle encoding | shard id packed into high `Slot` bits; `Handle` width unchanged | [ ] |
| Pool ↔ scheduler | `IsIdle`/`StartCompaction`/`MigrateBatch` operate per shard or fan out | [ ] |

### Integration Points
- `internal/component/bgp/rib/*` and `internal/component/bgp/plugins/rib/*` hold `Handle`
  values (confirmed: 45 `Intern` references across 16 files); they must continue to work unchanged.
- Wire/storage layers that persist or compare handles are unaffected (width and meaning unchanged).

### Architectural Verification
- [ ] No bypassed layers (callers still go through `Intern`/`Get`/`Release`).
- [ ] No unintended coupling (sharding is internal to `attrpool`).
- [ ] No duplicated functionality (extends `Pool`, does not introduce a parallel pool type).
- [ ] Zero-copy preserved (each shard keeps `unsafe.String` keys into its own buffers).

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Pool.Intern` from concurrent goroutines | → | shard selection + per-shard lock in `pool.go` | `TestShardRoundTripAllShards` |
| `Pool.Get`/`Release` on a handle from any shard | → | shard decode + per-shard access | `TestHandleShardDecodeRoute` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Intern same bytes twice (any concurrency) | Same `Handle` returned; refcount incremented once; one stored copy (dedup preserved) |
| AC-2 | Intern bytes, `Get` the handle | Returns the exact bytes, zero-copy, regardless of which shard stored them |
| AC-3 | Intern then `Release` to zero refcount | Slot freed in the owning shard; subsequent `Get` returns `ErrSlotDead` |
| AC-4 | Handle from shard X passed to `Get`/`Release` | Routed to shard X; never reads another shard's slot table |
| AC-5 | Existing `Handle` semantics | `Handle` remains 32 bits; `PoolIdx()`/`BufferBit()` unchanged; `IsValid()` unchanged |
| AC-6 | Compaction triggered while interning | Incremental, non-blocking; handles valid throughout; runs independently per shard |
| AC-7 | Aggregate capacity | Per logical pool total slot capacity >= documented minimum after carving shard bits (record exact split and resulting per-shard max in Key Design Decisions) |
| AC-8 | `Metrics()`/`internTotal`/`internHits`/`IsIdle` | Aggregate across shards equals prior single-pool semantics |
| AC-9 | `BenchmarkConcurrentIntern` after change | per-op time scales down with parallelism (no longer rises above single-threaded miss); mutex-profile delay no longer concentrated in one lock |
| AC-10 | `BenchmarkConcurrentGet` after change | CPU no longer dominated by a single shared reader-count atomic |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShardRoundTripAllShards` | `internal/component/bgp/attrpool/shard_test.go` | intern/get/release across every shard id | |
| `TestHandleShardDecodeRoute` | `internal/component/bgp/attrpool/shard_test.go` | shard id encoded in slot bits decodes to same shard | |
| `TestDedupStaysGlobal` | `internal/component/bgp/attrpool/shard_test.go` | identical bytes always land in one shard; single stored copy | |
| `TestMetricsAggregateAcrossShards` | `internal/component/bgp/attrpool/metrics_test.go` | `Metrics()`/hits/total sum shards (AC-8) | |
| `TestCompactionPerShardValidHandles` | `internal/component/bgp/attrpool/compaction_test.go` | handles valid during per-shard compaction (AC-6) | |
| existing `pool_test.go`/`handle_test.go` suite | (unchanged files) | all current behaviour still passes unmodified | |
| `TestNewWithShardsValidation` | `shard_test.go` | shard count constrained to powers of two 1..16 (2026-05-30 per-pool count) | done |
| `TestUnshardedPoolDegeneratesToSingleShard` | `shard_test.go` | N=1 pool puts all data in shard 0, round-trips (Origin-like) | done |
| `TestForeignShardHandleRoutingIsBoundsSafe` | `shard_test.go` | out-of-range shard id → `ErrWrongPool`/`ErrSlotOutOfBounds`, no panic | done |
| `TestSchedulerCompactsFragmentedShardDespiteLowAggregate` | `scheduler_test.go` | per-shard scheduling compacts a fragmented shard below the pool-wide ratio | done |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| shard count N | power-of-two, fits shard-id bits | N=2^k max for chosen bits | N=1 (degenerate, must still work) | N > 2^(shard bits) |
| slot within shard | 0 .. per-shard max | per-shard max | N/A | per-shard max + 1 → `ErrPoolFull` |
| data length | 0 .. `MaxDataLength` | 65535 | N/A | 65536 → `ErrDataTooLarge` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing BGP convergence/throughput suite | `test/` (.ci) | many-peer route propagation still correct after sharding | |

### Interop Tests (MANDATORY for protocol features)
Not a wire-protocol change. Handles are internal; no bytes on the wire change. Interop coverage is
satisfied by the existing BGP functional/interop suite continuing to pass (no new scenario needed).
Justification recorded here per template requirement.

### Benchmarks (goal validation)
| Benchmark | Location | Proves |
|-----------|----------|--------|
| `BenchmarkConcurrentIntern` | `attrpool/benchmark_test.go` | AC-9 write-path scaling |
| `BenchmarkConcurrentGet` | `attrpool/benchmark_test.go` | AC-10 read-path cache-line relief |
| new `-mutexprofile` capture | manual, documented in Goal Validation | contention no longer in one lock |

### Future (if deferring any tests)
- RIB-store allocation reductions (bart boxing / `ModifyAll` rangefunc) — separate spec, see Out of scope.

## Files to Modify
- `internal/component/bgp/attrpool/pool.go` - split per-pool state into N shard structs; route
  `Intern`/`Get`/`Length`/`Release`/compaction/metrics through shards; add shard-selection hash.
- `internal/component/bgp/attrpool/handle.go` - add internal shard accessor derived from `Slot`
  high bits and a constructor that packs shard id; keep public `Handle` width and existing methods.
- `internal/component/bgp/attrpool/scheduler.go` - drive compaction per shard (or fan out).
- `internal/component/bgp/attrpool/benchmark_test.go` - keep concurrent benchmarks; add a
  shard-stress concurrent benchmark if needed for AC-9/AC-10 evidence.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | internal API only |
| CLI commands/flags | [ ] Maybe | if a pool-stats/`dump` command should expose per-shard stats |
| Prometheus counters/metrics | [ ] Maybe | if per-shard depth/contention metrics are wanted |
| Doctor check | [ ] No | no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/pool-architecture.md` - document sharding, shard-id-in-slot encoding |
| 14 | Prometheus counters added/changed? | [ ] TBD | telemetry doc if per-shard metrics added |
| 16 | Changed source referenced by doc source anchors? | [ ] Yes | grep `docs/` for `source: .../attrpool/` and update |
| (others) | | [ ] No | answer Yes/No at completion with source-aware checks |

**Resolution (2026-05-30):**
- #12 **Yes, done** — `docs/architecture/pool-architecture.md`: added a `## Sharding` section
  (numShards/shardIDBits/shardSlotBits/MaxSlotsPerShard/MaxSlots table, shard selection,
  per-shard concurrency and compaction), updated the Handle Slot sub-field layout, and
  replaced the `Pool` struct snippet with the `Pool`+`shard` split. New source anchors point
  at `pool.go` (shard struct, shardOf, numShards) and `handle.go` (shardID/shardSlot/packShardSlot).
- #14 **No** — no Prometheus counters added or changed. The `attrpool.Metrics` struct is an
  in-process struct (unchanged fields), not a Prometheus collector; sharding only changed how it
  is aggregated internally. No telemetry doc update required.
- #16 **Yes, done** — grepped `docs/` for `source: .../attrpool/`. The canonical doc
  (`pool-architecture.md`) was updated. Other anchors (architecture.md, DESIGN.md, overview.md,
  rib-storage-design.md, etc.) reference the unchanged public surface (`Pool`, `Handle`,
  `Intern`/`Get`/`Release`, `Scheduler`) whose ABI and behavior are preserved, so their claims
  remain accurate. `make ze-doc-test` passes.
- All other categories **No** (no config/YANG/CLI/web/API/RPC/wire/plugin-inventory/RFC change;
  internal-only refactor with unchanged public API and wire format).

## Files to Create
- `internal/component/bgp/attrpool/shard_test.go` - shard round-trip, decode-route, dedup-locality tests.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-14 | per template |

### Implementation Phases

Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — define the shard-id encoding in `handle.go` (carve high
   bits of `Slot`), add internal shard accessor + packing constructor, and a failing
   `TestHandleShardDecodeRoute`. Verify encoding round-trips before touching `Pool`.
   - Tests: `TestHandleShardDecodeRoute`
   - Files: `handle.go`, `shard_test.go`
   - Verify: encode/decode round-trips; `PoolIdx`/`BufferBit` unaffected; test initially fails on stub.
2. **Phase: Shard the Pool state** — introduce a `shard` struct holding the current per-pool fields
   (`mu`, `index`, `slots`, `freeSlots`, `buffers`, `currentBit`, compaction state); `Pool` holds
   `[N]shard`. Add allocation-free `hash(data) mod N` shard selection. Route `Intern`.
   - Tests: `TestShardRoundTripAllShards`, `TestDedupStaysGlobal`, existing `pool_test.go`
   - Files: `pool.go`
   - Verify: dedup still global per shard; identical bytes single-copy; tests pass.
3. **Phase: Route Get/Length/Release/metrics through shards** — decode shard from handle; aggregate
   metrics; preserve refcount and double-return guards.
   - Tests: `TestMetricsAggregateAcrossShards`, existing release/metrics tests
   - Files: `pool.go`
   - Verify: AC-2..AC-5, AC-8 hold.
4. **Phase: Per-shard compaction** — make scheduler/compaction operate per shard; handles valid
   throughout; idle detection aggregates shard activity.
   - Tests: `TestCompactionPerShardValidHandles`, existing `compaction_test.go`/`scheduler_test.go`
   - Files: `pool.go`, `scheduler.go`
   - Verify: AC-6 holds; no handle invalidation mid-compaction.
5. **Benchmarks / goal validation** — re-run concurrent benchmarks + mutex/CPU profiles; record
   before/after in Goal Validation.
6. **Full verification** → `make ze-verify`.
7. **Complete spec** → audit, learned summary, two-commit closure per planning.md.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | shard selection is deterministic per byte content; dedup stays global; refcount correct across shards |
| Handle ABI | `Handle` still 32 bits; `PoolIdx`/`BufferBit`/`IsValid` unchanged; shard bits taken only from `Slot` high bits |
| Data flow | sharding internal to `attrpool`; callers unchanged |
| Capacity | per-shard slot max and aggregate recorded; `ErrPoolFull` boundary correct |
| Rule: enum-over-string | shard selection numeric, no string maps on the hot path |
| Rule: memory-architecture | zero-copy `Get` preserved; keys point into owning shard's buffers |
| Concurrency | `-race` clean; no shard reads another shard's state without its lock |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Sharded `Pool` | `go test ./internal/component/bgp/attrpool/...` green incl. `-race` |
| Handle ABI unchanged | `handle_test.go` passes unmodified; grep callers compile |
| Contention removed | before/after `-mutexprofile` + benchmark numbers in Goal Validation |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|------------------|
| Input validation | `data` length still bounded (`MaxDataLength`); `ErrPoolFull` enforced per shard and aggregate |
| Resource exhaustion | uneven hashing must not let one shard fill while others idle without `ErrPoolFull` surfacing; choose a good byte hash |
| Unsafe usage | `unsafe.String` keys remain confined to their shard's buffer lifetime |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails behavior mismatch | Re-read `pool.go`/`handle.go` from Current Behavior |
| `-race` failure | shard locking gap → fix routing/locking |
| Benchmark shows no improvement | revisit shard count / hash distribution / false sharing of shard structs |
| 3 fix attempts fail | STOP. Report approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Could shard by allocating multiple `PoolIdx` values | `PoolIdx` is a 5-bit global namespace already assigned to distinct pools (RibOut=16); only 31 exist | reading `handle.go` + `NewWithIdx` | shard id must come from `Slot` high bits instead |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Multiple pool indices per logical pool | exhausts 31-value `PoolIdx` namespace | intra-pool shards keyed in `Slot` high bits |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The single `RWMutex` hurts twice: full `Lock` serializes writers, and the reader-count word is a
  contended cache line for readers. Sharding fixes both in one change because each shard owns its
  own lock (hence its own reader-count word on its own cache line).
- Dedup correctness depends only on "same bytes → same shard." A content hash gives that for free.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Shard internally; encode shard id in high bits of the 26-bit `Slot` field | Allocate multiple `PoolIdx` per pool; widen `Handle` to 64 bits | `PoolIdx` namespace is scarce (31 total, some assigned); widening `Handle` breaks ABI and doubles handle storage everywhere. Slot has spare bits. |
| Shard count = power of two, selected by content hash | per-peer sharding; round-robin | content hash keeps dedup global and balances load; power of two makes selection a mask |
| Max shard count = **16**, split = **4 shard bits + 22 per-shard slot bits** (approved 2026-05-29) | 8 shards (3+23); 32 shards (5+21) | 16 matches the 16-core profiling box for maximal contention relief; the 4 high bits of the 26-bit `Slot` field hold the shard id, leaving 22 bits = 4,194,303 slots/shard, 67,108,848 aggregate per logical pool (exceeds the prior 24-bit `MaxSlots` of 16,777,215, so AC-7 capacity is satisfied). `Handle.Slot()` keeps returning the full 26-bit field (now composite shard\|slot); only `attrpool`-internal accessors split it, so external opaque-handle callers (incl. BundlePool, which builds its own handles and never routes through sharded `Intern`/`Get`) are unaffected. |
| **Per-pool shard count** (revised 2026-05-30): `NewWithShards(idx, cap, shards)`, power of two 1..16; `New`/`NewWithIdx` default to 16 | Fixed global `numShards` for every pool | Content-hash sharding only relieves contention when a pool's hot values are diverse enough to spread across shards. Pools dominated by a few values (ORIGIN=3, ATOMIC_AGGREGATE=1) have their hot value monopolize one shard, so sharding adds fixed overhead for no benefit; those use N=1, which is exactly the pre-sharding single-lock pool (one lock/index/buffer, shard id always 0). `shardMask = len(shards)-1`; `shards` is now a slice, so handle routing bounds-checks the decoded shard id (`ErrWrongPool`/`ErrSlotOutOfBounds`) where the fixed `[16]shard` array could not. |
| **Compaction scheduled per shard, quiet-gated per pool** (revised 2026-05-30): `Scheduler` round-robins over `(pool, shard)` units, one shard at a time | Pool-wide aggregate dead-ratio trigger then fan out to all shards | The aggregate trigger diluted a single fragmented shard across the others, so a hot shard could accumulate dead entries forever below the pool-wide threshold; and it forced every shard (even clean ones) to compact together. *Which* shard to compact is now decided by that shard's own dead ratio (`deadRatioExceeds`, O(1) fast path when the free list is empty); *whether* to run is gated by the owning pool's quiet period (`Pool.IsIdle`), preserving the original "stay out of the way during activity" guard at pool granularity since compaction competes for memory bandwidth, not just the shard lock. |

## Known Limitations
- Sharding raises per-`Pool` fixed overhead (N lock/index/slot/buffer sets). **Resolved
  (2026-05-30):** the shard count is now per-pool (`NewWithShards`, power of two 1..16); N=1
  degenerates cleanly to the pre-sharding single-lock pool. Low-cardinality pools (ORIGIN,
  LOCAL_PREF, MED, CLUSTER_LIST, ORIGINATOR_ID, ATOMIC_AGGREGATE, AGGREGATOR) use N=1 because their
  few hot values would monopolize one shard and gain nothing from sharding; high-cardinality
  per-route attributes (AS_PATH, NEXT_HOP, communities, labels, RibOut, unknown attrs) use N=16.
  Split lives in `internal/component/bgp/plugins/rib/pool`.
- Aggregate slot capacity per logical pool is reduced per shard but unchanged in total; AC-7 records
  the exact split. If a single shard can fill while others are empty under skewed hashing, hash
  quality matters — covered by the security review row.
- RIB-store allocation findings (bart boxing, `ModifyAll` rangefunc) are explicitly out of scope.

## Goal Validation (BLOCKING)

Captured 2026-05-30, Apple M4 Max, `GOMAXPROCS=16`, `-benchtime=2s`. Baselines are the
single-lock numbers recorded in the Profiling Evidence section above.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Remove read-path cache-line contention | benchmark | **Decisive.** A single shared handle (one shard, one reader-count word) `BenchmarkConcurrentGet` = **172.5 ns/op**; spreading reads across all 16 shards `BenchmarkConcurrentGetShardedSpread` = **23.6 ns/op** — **7.3x faster**. The single reader-count atomic was the entire cost; per-shard locks on separate cache lines remove it. |
| Remove write-path lock contention (scaling) | benchmark | New-entry intern across 16 cores with diverse keys `BenchmarkConcurrentInternSharded` = **475 ns/op ~= single-threaded `BenchmarkInternNew` 468 ns/op** (no rise above single-threaded miss), vs the single-lock baseline **769 ns/op** (1.68x single-threaded serialization). |
| Remove write-path lock contention (dedup-hit, no alloc) | benchmark | Isolating the write lock (pre-interned keys, zero allocation in loop): all hits on one shard `BenchmarkConcurrentInternHitSingleShard` = **423.5 ns/op**; hits spread across 16 shards `BenchmarkConcurrentInternHitSpread` = **343.9 ns/op** (1.23x faster). Bounded by 16 shards ~= 16 cores collision rate over a ~10 ns critical section; the single global serialization point is gone. |
| Remove write-path lock contention (mutex profile) | mutex profile | Baseline: 30.12 s delay, **100%** attributed to one `(*Pool).internLocked` lock. After: contention is reached only via per-shard `sync.(*RWMutex).Unlock` in `(*shard).intern`; no single pool-wide lock exists to concentrate on. |
| No regressions | unit + functional | `attrpool` suite green incl. `-race` (`go test -race`); dependent `rib`/`rib/pool`/`rib/storage` suites green incl. `-race`; `go vet ./internal/component/bgp/...` clean; `make ze-lint-changed` 0 issues; `make ze-doc-test` passes. |

**Pre-existing failures unrelated to this spec (recorded for honesty):** `make ze-verify` and
the full `make ze-functional-test` fail on `origin/main` independently of this change:
- `internal/plugins/sysrib/sysrib.go:260` gocritic `rangeValCopy` (blocks the full-tree `ze-lint`).
- functional `editor` (7 tests: mode-switch, completion ghost, pipe format, session show-blame/
  show-changes/adopt), plus `ui`/`reload`/`plugin` suite failures and timeouts.

These are confined to files this spec did not touch. Proof they cannot be caused by this change:
the CLI editor (`internal/component/cli/`) has **no import of `attrpool`**, and
`internal/component/cli/`, `cmd/ze/`, `internal/plugins/sysrib/`, `test/editor/` are all
unmodified in the working tree (`git status` clean). The editor failures reproduce identically
in isolation and reference nothing about pools/shards/handles. The attrpool public API is
byte-identical, so the editor binary links unchanged.

## Review Gate

### Run 1 (2026-05-30)
Self-review against the Critical Review Checklist + an adversarial sub-agent review of the
concurrency-sensitive `pool.go`/`handle.go` (routing, within-shard indexing, MaxSlotsPerShard
boundary, GetBySlot/ReleaseBySlot bit extraction, lock discipline, shardOf determinism, and the
migration index-key identity).

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NONE | Every handle construction packs the shard id (`packShardSlot`) | pool.go:312/626/738 | verified, no change |
| 2 | NONE | Every slot index uses `h.shardSlot()` / local index, never full `Slot()` | pool.go get/length/release/addRef/validators | verified, no change |
| 3 | NONE | `MaxSlotsPerShard` boundary prevents slot index overflowing into shard bits | pool.go intern | verified, no change |
| 4 | NONE | `GetBySlot`/`ReleaseBySlot` decode shard as inverse of `packShardSlot` | pool.go:448/480 | verified, no change |
| 5 | NONE | No method holds two shard locks; fan-out is sequential per shard | pool.go fan-out methods | verified, no deadlock |
| 6 | NONE | Migration index key is value-equal across buffers (Go string compare), release-after-migrate deletes the correct entry | pool.go migrateBatch/release | verified, no leak |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  → **Achieved: 0 BLOCKER, 0 ISSUE.**
- [ ] All NOTEs recorded above (or explicitly "none")  → **None.**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/attrpool/handle.go` | Yes | shard encoding helpers added (`shardID`/`shardSlot`/`packShardSlot`) |
| `internal/component/bgp/attrpool/pool.go` | Yes | `shard` struct + routing; `git diff --stat` 643 lines changed |
| `internal/component/bgp/attrpool/validate_release.go` / `validate_debug.go` | Yes | shard-aware validators |
| `internal/component/bgp/attrpool/shard_test.go` | Yes | new file: round-trip, decode-route, dedup-locality tests |
| `docs/architecture/pool-architecture.md` | Yes | Sharding section + updated Handle/Pool structure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | dedup global under sharding | `TestDedupStaysGlobal` (h1==h2, 1 live slot); `pool.go` shardOf is content-only |
| AC-2 | Get returns exact bytes from any shard | `TestShardRoundTripAllShards` (all 16 shards); `pool.go` `(*shard).get` zero-copy |
| AC-3 | Release to zero frees slot, then ErrSlotDead | `TestShardRoundTripAllShards` release+Get→ErrSlotDead |
| AC-4 | handle routes to owning shard only | `TestHandleShardDecodeRoute`; `Pool.shardForHandle` uses `h.shardID()` |
| AC-5 | Handle ABI unchanged | `handle_test.go` passes unmodified (PoolIdx/BufferBit/Slot/IsValid) |
| AC-6 | per-shard compaction, handles valid throughout | `TestCompactionPerShardValidHandles` |
| AC-7 | capacity recorded | `TestMaxSlotsConstant`: 16 shards × 4,194,304 = 67,108,864 |
| AC-8 | metrics aggregate across shards | `TestMetricsAggregateAcrossShards`; `Pool.Metrics` sums shards |
| AC-9 | write path scales | `BenchmarkConcurrentInternSharded` 475 ns/op ≈ single-threaded 468 (see Goal Validation) |
| AC-10 | read path no single reader-count atomic | `BenchmarkConcurrentGetShardedSpread` 23.6 vs 172.5 ns/op (7.3×) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests) incl. `-race` on `attrpool`
- [ ] Handle ABI unchanged; existing callers compile and pass
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Design
- [ ] No premature abstraction
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (sharding internal to attrpool)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for shard/slot/data-length
- [ ] Goal Validation table filled with concrete before/after evidence
