# Plan: Shard the Loc-RIB by prefix hash (Phase 4)

Working design document. Precedes a formal spec.

## Why

`internal/core/rib/locrib/manager.go` protects its per-family stores with
one `sync.RWMutex`. Every Insert/Remove serialises through that lock,
and the BGP RIB has the same shape: `internal/component/bgp/plugins/rib/
rib.go:246` guards `bestPrev` with a single `sync.RWMutex`. A single
full-feed peer drives all writers onto that lock; show-commands, best-
path recompute, and event emission all queue up behind it.

Sharding by prefix hash splits the prefix space across N worker-owned
shards, each with its own lock. Work on unrelated prefixes no longer
contends. This is where BIRD (per-table workers) and Juniper KRT have
landed; FRR's threaded RIB is moving the same direction.

## Constraints

- Each prefix must always land on the same shard. `hash(prefix) % N`
  is deterministic and stable per prefix.
- Best-path for a given prefix is computed within its shard; no cross-
  shard coordination for normal churn.
- OnChange callbacks (Phase 3c) fire from whichever shard's writer
  produced the change. Callbacks are already non-blocking and do not
  re-enter the RIB, so running them under a per-shard lock is safe.
- ADD-PATH (pathSet) semantics stay inside each shard's values; no
  change to the value types.
- Store under `-tags maprib` (map backend) must still work; sharding is
  above `store.Store[T]` and wraps N of them.

## Shape

A sharded RIB per family: replace `map[family.Family]*store.Store[
PathGroup]` with `map[family.Family]*familyShards`.

```
familyShards struct {
    n      int
    shards []shard   // len == n; indexed by hash(prefix) % n
}

shard struct {
    mu    sync.RWMutex
    store *store.Store[PathGroup]
}
```

Insert / Remove: compute shard index once, take that shard's write
lock, call into `store.Store`. No global lock held.

Lookup / Best: read lock on one shard.

Families(): global snapshot of known families; needs a minimal outer
lock (only read on bookkeeping paths, not hot).

Iterate(fam, fn): iterate shards in order; each yields its own prefixes
under its read lock. Order is no longer sorted within a family -- if
callers depend on sorted order they pay for a merge step.

## Hash function

`netip.Prefix` is comparable and already hashable via `maphash`. Use
`maphash.Comparable[netip.Prefix]` (Go 1.21+) with a package-level
seed. The seed is fixed per process so the same prefix always hashes
to the same shard.

Shard count N: configurable, default `runtime.GOMAXPROCS(0)`, clamped
to `[1, 64]`. One writer per core is a reasonable starting point;
profiling can refine.

## OnChange dispatch

Today `RIB.Insert / Remove` fires handlers synchronously under the
single write lock. With shards, handlers fire under the per-shard
write lock instead. Handler contract is unchanged: cheap, non-
blocking, no re-entry. Subscribers that span multiple shards (e.g.
sysrib) already tolerate out-of-order delivery between prefixes.

## Migration steps

1. **Add `familyShards` alongside the existing single store.** Guard
   behind a `sharded bool` flag on RIB so tests can opt in per case.
2. **Port methods incrementally.** Insert / Remove / Lookup / Best /
   Iterate / Len / Families. Each gets a sharded path next to the
   single-store path.
3. **Delete the single-store path.** `rules/no-layering.md` applies;
   no "old mode" kept as fallback. Flip the default before deleting.
4. **Benchmark.** Add `BenchmarkShardedInsert` to
   `internal/core/rib/locrib/` with 1M prefixes and N workers. Compare
   against single-lock baseline.

Step order matters: step 1-2 is additive (old tests still pass), step
3 is the behavior change.

## Files touched

- `internal/core/rib/locrib/manager.go` -- RIB struct gains
  familyShards; methods dispatch.
- `internal/core/rib/locrib/shard.go` -- new; `familyShards` type plus
  hash helper.
- `internal/core/rib/locrib/locrib_test.go` -- existing tests still
  pass; add parallel-Insert test that asserts no lost writes.
- `internal/core/rib/locrib/shard_bench_test.go` -- new; sharded vs
  single benchmark.

## Deferred to a later phase

- Sharding the BGP-plugin-internal `bestPrev` store
  (`rib_bestchange.go:247-249`) is a separate change with the same
  shape. Do after locrib sharding lands so the pattern is proven.
- Per-shard metrics (insertions/shard, queue depth/shard) are
  observability, not correctness; add when debugging a hot shard.

## Open questions

1. Should the OnChange subscriber list be per-shard too? A per-shard
   copy-on-write list avoids one atomic load per dispatch but a subscriber
   registered after one shard's Insert but before another's would see
   inconsistent state. Start with a single shared list; revisit if the
   atomic load shows up in profiles.
2. Iterate order. Some callers (replay, show) expect sorted output
   today. Document the ordering change explicitly in each caller; if a
   caller genuinely needs sorted order, sort at the caller not in
   locrib.
3. Family creation races. Two goroutines inserting into a previously-
   absent family must cooperate. The outer map keyed by family.Family
   needs one writer at a time -- smallest mutex covers it; families
   are created O(few) per process lifetime.
