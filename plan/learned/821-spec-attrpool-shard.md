# 821 -- attrpool-shard

## Context

The BGP attribute pool (`attrpool.Pool`) deduplicates and refcounts attribute byte
sequences, accessed concurrently from every per-peer goroutine and once per path
attribute on the UPDATE path. Each `Pool` was guarded by one `sync.RWMutex`. Profiling
on a 16-core M4 Max showed the write path fully serialized on that single lock (mutex
profile: 30.12s delay, 100% in `internLocked`) and the read path bottlenecked on the
`RWMutex` reader-count word (one contended cache line, ~52% CPU in `atomic.Int32.Add`).
Goal: remove both contention points without changing the 32-bit `Handle` ABI, the global
dedup guarantee, refcount semantics, or the double-buffer compaction behavior.

## Decisions

- Shard each `Pool` into 16 independent sub-pools selected by `hash(data) mod 16`, over
  (a) allocating multiple `PoolIdx` values per logical pool (the 5-bit PoolIdx is a scarce
  31-value global namespace, some already assigned, e.g. RibOut=16) and (b) widening
  `Handle` to 64 bits (breaks ABI, doubles handle storage everywhere).
- Carve the shard id from the **high 4 bits of the existing 26-bit `Slot` field**
  (4 shard bits + 22 per-shard slot bits). `Handle.Slot()` still returns the full 26-bit
  composite; only package-private `shardID()`/`shardSlot()`/`packShardSlot()` split it, so
  every external opaque-handle caller is untouched.
- Fixed global `numShards = 1 << shardIDBits` (16), over per-pool configurability: keeps the
  hot-path shard mask trivial and the `New`/`NewWithIdx` signatures unchanged. 16 matches the
  profiling box's core count.
- Shard selection is allocation-free FNV-1a over the bytes (`shardOf`), over per-peer or
  round-robin sharding: content hashing is the only scheme that keeps "same bytes -> same
  shard", which is what preserves global dedup.
- Compaction is **fan-out**, not per-(pool,shard) scheduling: `StartCompaction`/`MigrateBatch`/
  `Compact`/`CheckOldBufferRelease` iterate the shards, each under its own lock; `State()` is
  `PoolCompacting` if any shard is. This left `scheduler.go` completely unchanged (it only
  calls Pool-level methods).
- Per-shard `lastActivity`/`internTotal`/`internHits`; `Metrics()` sums shards, `IsIdle` is
  true only when every shard is idle, `Touch` marks all. `idx` and `shutdown` stay Pool-level.

## Consequences

- New per-pool capacity: 16 x 4,194,304 = 67,108,864 slots (was a 24-bit `MaxSlots` of
  16,777,215). `MaxSlots` redefined as the aggregate; `MaxSlotsPerShard = 1<<22` added.
- Read path with reads spread across shards: 23.6 ns/op vs 172.5 for a single shared handle
  (7.3x). Write path new-entry intern under 16-way concurrency: ~475 ns/op, matching the
  single-threaded miss (468) instead of the single-lock baseline's 769.
- Sharding raises per-pool fixed overhead (16 lock/index/slot/buffer sets). Bounded and
  acceptable; `New(cap)` divides `cap` across shards (min 64 each), so aggregate buffer
  capacity is preserved (e.g. New(1024) -> 16 x 64).
- Any future persistent owner that stores a handle across compaction must use the normalized
  `GetBySlot`/`ReleaseBySlot` (buffer auto-select); a raw handle with an explicit BufferBit
  pins its shard's old buffer until released, keeping that shard `Compacting`.

## Gotchas

- `BundlePool` (storage/bundle.go) shares pool idx 15 with the `Labels` attrpool but builds
  its own handles via `NewHandle(15, slot)` and uses `h.Slot()` as a dense index into its own
  array -- it never routes through the sharded `Intern`/`Get`, so it is unaffected. Do not
  assume `h.Slot()` is a small dense integer for attrpool handles; it is `shardID<<22 | slot`.
- White-box tests asserting the pre-sharding slot layout had to change: `h.Slot()==0` for the
  first entry became `h.shardSlot()==0` (Slot is now composite); free-list-reuse tests need
  two payloads in the *same* shard (added a `differentSameShard` test helper) because free
  lists are per-shard; `p.currentBit` became `p.shards[h.shardID()].currentBit`.
- The micro-benchmark `BenchmarkConcurrentIntern` uses an overlapping key sequence across
  goroutines, which collapses onto shared shards and understates the win; representative
  diverse-key benchmarks were added. Allocation (`fmt.Appendf`) in the loop also masks the
  lock cost on the write path -- the dedup-hit, zero-alloc benchmark isolates it.
- Pre-existing, out of scope: `debug_test.go` does not compile under `-tags debug`
  (`h := p.Intern(...)` ignores the 2nd return) and its `require.Panics` never matched the
  error-returning validators. Not touched.

## Files

- `internal/component/bgp/attrpool/handle.go` -- shard sub-field constants + `shardID`/`shardSlot`/`packShardSlot`.
- `internal/component/bgp/attrpool/pool.go` -- `shard` struct, `shardOf`, routing, fan-out compaction, aggregated metrics.
- `internal/component/bgp/attrpool/validate_release.go` / `validate_debug.go` -- shard-aware validators.
- `internal/component/bgp/attrpool/shard_test.go` -- new: round-trip, decode-route, dedup-locality, `differentSameShard` helper.
- `internal/component/bgp/attrpool/{metrics,compaction,benchmark,pool}_test.go` -- aggregate-metrics test, per-shard compaction test, sharded benchmarks, adapted white-box tests.
- `docs/architecture/pool-architecture.md` -- Sharding section, updated Handle/Pool structure.
