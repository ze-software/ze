# 618 -- rib-bestpath-pack

## Context

The 1M-prefix stress profile from `plan/learned/607-rib-bart-bestprev.md` flagged
`bart.NewFringeNode[bestPathRecord]` at 56.5 MB (33% of inuse heap) with
GC mark work at ~27% of CPU. Each record was a 72-byte struct holding five
GC-scannable pointers (`PeerAddr string`, `NextHop netip.Addr` string zone,
`ProtocolType string`, plus two more via BART's node metadata). At 1M entries
the mark phase had to scan 5M pointer words every cycle. The goal was to
eliminate the pointer scan entirely by packing the stored value into a scalar.

## Decisions

- **Named `uint64` over a compact struct.** Four 16-bit fields packed by shift
  + OR: `[63:48] MetricIdx | [47:32] PeerIdx | [31:16] NextHopIdx | [15:0] Flags`.
  Chose this over "8-byte struct with accessors" because the generic
  `bart.Table[T]` specialises cleanly on a scalar and the same-best compare
  becomes a single `cmpq`.
- **Shared interner over per-family.** One `bestPrevInterner` on `RIBManager`
  across every family, rather than a per-family interner inside `bestPrevStore`.
  Realistic deployments peak around 2k peers (largest Internet IXP) and low
  hundreds of unique next-hops / metrics -- sharing the dedup maps across
  families amortises the map overhead to sub-kilobyte scale.
- **`Priority` + `ProtocolType` collapse into Flags bit 0.** Both are derived
  from `PeerASN != LocalASN`; storing a single `isEBGP` bit and deriving 20/200
  + "ebgp"/"ibgp" at emission time removes redundancy. Rejected "keep both
  fields packed" because the two-way derivation is already documented in
  `ebgpLabel` / `protocolTypeEBGP`.
- **Overflow handler returns `(0, false)`, interner logs once per table.**
  Rejected `panic`: the cap is architecturally unreachable (uint16 = 65536
  entries, 30x above the largest observed cardinality), but a mis-deployment
  must degrade gracefully rather than crash the daemon. Rejected "log on every
  saturated call" after the first `/ze-review` pass flagged it as a log-flood
  vector: first saturation per table flips a latch on `bestPrevInterner` and
  emits one `slog.Error`; subsequent saturated calls are silent. The stored
  `prev` record is retained so consumers keep seeing the pre-saturation best
  path rather than a spurious withdraw; recovery requires a restart.
- **Drop `adminDistance` method.** It existed to emit 20/200 given a Candidate;
  with `resolve()` inlining the mapping from Flags bit 0, the helper is dead.
- **Same-best short-circuit compares raw values, not packed records.** Reverse
  table lookups (`peerAt/nextHopAt/metricAt`) resolve `prev`'s indices back to
  strings/addresses/uints and compare to the winner's raw fields BEFORE any
  `intern*` call. Steady-state update has zero interner mutation, no
  `wirePrefixToString` allocation, and no packing -- three slice reads + value
  compares. Also avoids "first malformed NLRI grows the interner" (the
  previous design interned then bailed on an empty prefix).
- **`NewRIBManager` is the sole constructor.** A `RIBManager{}` literal has
  nil maps and panics on the first intern call; routing everything through
  `NewRIBManager(plugin)` makes that invariant mechanical. Callers that want
  a plugin-less manager (tests with closed pipes) pass `nil`.
- **`ze_rib_bestpath_interner_size{table}` Prometheus gauge.** Added so
  operators can alert on approach to the uint16 cap before saturation hits.
  Populated by `updateMetrics` under the existing RLock.

## Consequences

- BART fringe nodes storing `bestPathRecord` are now opaque 8-byte scalars.
  The GC mark phase touches zero pointers per entry (down from 5), removing
  the 5M pointer-trace workload at 1M prefixes.
- `Store[bestPathRecord]` per-entry cost drops from ~72 bytes (struct) to
  ~8 bytes (uint64) for the payload, plus BART node metadata unchanged.
  Go benchmark (`BenchmarkBestPathRecordHeapFootprint`) shows 27.78 MB total
  heap for 1M entries; Phase-4b baseline was 56.5 MB for the fringe-node
  allocations alone.
- The interner's reverse tables grow with unique attribute values, not with
  prefix count: ~2k peers * (string header + avg address) + 256 next-hops +
  16 metrics is sub-megabyte at any realistic scale.
- Emission path now goes through `resolve()` in one place (instead of being
  duplicated in `checkBestPathChange` and `replayBestPaths`). Any future
  payload-shape change lives in one function.
- AC-1 / AC-10 stress numbers (fringe-node heap and GC share) still need a
  privileged stress run (`make ze-stress-profile`) for empirical confirmation;
  the in-memory benchmark gives a lower bound without root/netns.

## Gotchas

- `net/netip.Addr` is a comparable struct (16-byte value), so it maps keys
  cleanly -- but the zero value is also interned and round-trips through
  `nextHopString()` to empty. Overlooking this would make a "missing
  next-hop" emit as a spurious change.
- The overflow path is tested both at the bare interner and through
  `checkBestPathChange`. The RIB-level test must saturate the PEER interner,
  not the metric interner, because the hot-path intern order is peer first.
  An earlier attempt saturated metrics and confused the test with the
  same-best short-circuit.
- The `internerCap` constant is `1 << 16 = 65536` (distinct indices
  `0..65535`). The check is `len(table) >= internerCap`; `== 65536` would
  be equivalent but the `>=` form catches any accidental double-increment
  defensively.
- Benchmark uses `HeapAlloc` (live heap objects), not `HeapInuse` (span-level
  bytes). `HeapInuse` can shrink when map buckets are released during resize
  and wrap the subtraction into a garbage uint64; `HeapAlloc` is stable for
  the production BART path (27.53 bytes/entry at 1M). The maprib variant
  still reports 0 in the Go benchmark harness due to bucket-release
  scheduling -- a measurement artefact, not a correctness issue (the tests
  pass under both tags).
- The spec prescribed `resolve()` as a method on `bestPathRecord` taking an
  explicit `*bestPrevInterner`. Kept that signature rather than moving it
  onto `RIBManager` so the record stays a self-describing primitive and the
  hot path can inline the call.

## Files

- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- rewritten:
  `type bestPathRecord uint64`, `packBestPath`, accessors, `bestPrevInterner`
  with per-table one-shot overflow logging, bounds-safe reverse accessors
  (`peerAt/nextHopAt/metricAt`), bounds-safe `resolve`, rewired
  `checkBestPathChange` (raw-value same-best short-circuit) +
  `replayBestPaths`.
- `internal/component/bgp/plugins/rib/rib.go` -- added `bestPathInterner`
  field on `RIBManager`, `NewRIBManager(plugin)` constructor, and
  `bestpathInternerSize` Prometheus gauge populated in `updateMetrics`.
- `internal/component/bgp/plugins/rib/rib_test.go` -- routed through
  `NewRIBManager`.
- `internal/component/bgp/plugins/rib/rib_bestchange_test.go` -- constructor
  routed through `NewRIBManager`; 7 new unit tests (pack/unpack, equality,
  interner dedup, interner reverse, overflow including log-emission
  assertion, resolve).
- `internal/component/bgp/plugins/rib/rib_bestchange_bench_test.go` -- new
  `BenchmarkBestPathRecordHeapFootprint` using `HeapAlloc` for AC-1
  mechanical evidence.
- `docs/architecture/plugin/rib-storage-design.md` -- paragraph + source
  anchor on packed uint64 + shared interner.
