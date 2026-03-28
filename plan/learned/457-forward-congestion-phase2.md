# 457 -- Forward Congestion Phase 2: Overflow Metrics

## Context

Phase 1 added a bounded overflow pool but it was invisible to operators and to the planned Layer 3 (read throttling). Without metrics, there was no way to know the pool was filling, which source peers were causing pressure, or which destination peers had deep overflow queues. Phase 2 adds three Prometheus metrics to make congestion observable before it becomes critical.

## Decisions

- Chose **per-source-peer overflow ratio** (AC-16) over per-destination ratio, because Layer 3 needs to throttle the sources whose traffic fills the pool, not the slow destinations (which are already visible via the congestion callbacks). The ratio is `overflowed / (forwarded + overflowed)` per source peer.
- Chose **atomic counters** (`atomic.Int64`) for the hot-path forwarded/overflowed tracking over mutex-protected counters. `RecordForwarded`/`RecordOverflowed` are called from `ForwardUpdate` on every dispatch, so lock-free updates are essential.
- Chose **10-second polling** in `updatePeriodicMetrics` over event-driven metric updates. Consistent with the existing reactor metrics pattern (uptime, cache entries, worker count). Avoids metric update storms during burst traffic.
- Chose **`OverflowDepths()` returning `map[string]int`** snapshot over per-item gauge updates. The overflow depth changes rapidly during congestion; polling every 10 seconds is sufficient for dashboards and alerting without adding overhead to the dispatch path.
- Deferred **AC-10 auto-sizing** (PeeringDB, zefs) to a separate spec since it depends on prefix-limit and persistence infrastructure not yet built.

## Consequences

- Layer 3 (Phase 3) can use `ze_bgp_overflow_ratio` to identify which source peers to throttle and `ze_bgp_pool_used_ratio` as the trigger threshold.
- Operators can set Prometheus alerts on `ze_bgp_pool_used_ratio > 0.8` to detect congestion before pool exhaustion.
- The `srcStats` map grows monotonically (one entry per source peer ever seen). For typical deployments (<1000 peers), this is negligible. Long-running route servers with dynamic peers may want periodic cleanup, but that is future work.

## Gotchas

- **`overflowItems` and `overflowRatio` use different label semantics.** `overflowItems{peer=X}` is destination X's overflow depth. `overflowRatio{source=X}` is source X's overflow fraction. Deep-review caught that both originally used `"peer"` label, which would produce nonsense joins. Renamed `overflowRatio` label to `"source"`.
- **`srcStats` must be cleaned on peer removal.** Without cleanup, the map grows monotonically with every source peer ever seen. Added `RemoveSourceStats()` called from `reactor_peers.go` alongside existing metric label deletion.
- **`OverflowDepths()` acquires `fp.mu` then each worker's `overflowMu`** in a loop. Lock ordering is consistent (fp.mu -> overflowMu), same as the idle timeout path. But under very high worker counts, the metrics poll briefly blocks dispatches. Acceptable for 10-second polling.
- **Go test cache hid a pre-existing build break.** Changes to `reactor_metrics.go` invalidated the cache for the reactor package, exposing a broken dependency on `peer.go`/`types.go` from concurrent work. Lesson: a green `make ze-verify` after modifying only non-reactor files does not prove reactor compiles -- the cache may be stale.
- **Overflow ratio is cumulative, not windowed.** The counters track total forwarded/overflowed since startup (reset on peer disconnect via `RemoveSourceStats`), so the ratio reflects per-session behavior. Phase 3 may need a sliding window for responsive throttling decisions.
- **Hoist loop-invariant `String()` calls.** `update.SourcePeerIP.String()` was inside the per-destination-peer loop despite being constant. Hoisted to avoid N redundant allocations per ForwardUpdate.

## Files

- `internal/component/bgp/reactor/forward_pool.go` -- `fwdSourceStats`, `RecordForwarded`/`RecordOverflowed`, `RemoveSourceStats`, `OverflowDepths`, `PoolUsedRatio`, `SourceOverflowRatios`
- `internal/component/bgp/reactor/reactor_metrics.go` -- `ze_bgp_pool_used_ratio`, `ze_bgp_overflow_items`, `ze_bgp_overflow_ratio` registration and polling
- `internal/component/bgp/reactor/reactor_api_forward.go` -- counter wiring in `ForwardUpdate` (hoisted srcAddr)
- `internal/component/bgp/reactor/reactor_peers.go` -- overflow metric + srcStats cleanup on peer removal
- `internal/component/bgp/reactor/forward_pool_test.go` -- 5 new tests (depths with drain, ratio, concurrent safety)
