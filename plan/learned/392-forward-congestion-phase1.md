# 392 -- Forward Congestion Phase 1: Bounded Overflow Pool

## Context

The forward pool's per-worker overflow buffer (`[]fwdItem`) grew unboundedly via `append()` when a destination peer was slow. A single stuck peer could cause OOM, killing all peers. The design called for a four-layer congestion response; Phase 1 implements Layer 2: a global bounded overflow pool that tracks how many items are in overflow across all workers, with unbounded fallback when exhausted (Layer 3/4 handle escalation).

## Decisions

- Chose a **channel-based semaphore** (token acquire/release) over a pre-allocated backing array. The semaphore bounds the count of in-flight items without changing the existing `[]fwdItem` value-copy data flow. Pre-allocated storage can be added later without API changes.
- Chose **`pooled bool` on `fwdItem`** (zero-value false) over index-based tracking. Zero-value safety means all existing code creating `fwdItem{}` literals is unaffected.
- Chose **sentinel skip** (`item.peer == nil` bypasses pool acquire) over letting barrier items consume tokens. Barriers carry no route data and would waste tokens meant for real updates under congestion.
- Chose **rate-limited exhaustion logging** (thresholds 1/100/10K/100K via atomic counter) over per-event logging. Under sustained backpressure, per-event logging floods the log.
- Chose **10M max pool cap** in `newFwdPool` over no cap. Prevents misconfigured `ze.fwd.pool.size` from hanging the init loop or allocating excessive channel buffer.

## Consequences

- Forward pool overflow is now soft-bounded (default 100K items). Memory growth is visible via `overflowPool.available()` and exhaustion counter.
- Layer 3 (read throttling) can use pool exhaustion as the trigger signal -- the `exhausted` counter and `available()` method provide the inputs.
- The `pooled` field adds 8 bytes to every `fwdItem` (alignment padding). Modest cost (~800KB for 100K items).
- The unbounded fallback means the pool is NOT a hard bound. Hard bounding requires Layer 3/4 to reduce inflow or tear down sessions.

## Gotchas

- **Sentinel items must skip pool acquire.** The `peer == nil` check was added after deep-review found barrier sentinels consuming tokens. Any future code path that creates `fwdItem` with `peer == nil` and routes through `DispatchOverflow` will correctly skip the pool.
- **Pre-existing race in `drainOverflow`:** The worker's send loop (`w.ch <- item`) could panic if `Stop()` closed the channel concurrently. Fixed with a two-select pattern checking `stopCh` before each send. This was not introduced by the pool changes but was exposed during deep-review.
- **Token release happens in two places:** `safeBatchHandle` (normal processing and direct-process path) and `Stop()` (overflow cleanup). The `overflowMu` mutex prevents double-release by ensuring items are either in `w.overflow` (Stop handles) or already moved to `w.ch`/processed (safeBatchHandle handles), never both.
- **`available()` is a racy snapshot.** Fine for logging and test assertions but must never be used for control-flow decisions. Documented in godoc.

## Files

- `internal/component/bgp/reactor/forward_pool.go` -- `fwdOverflowPool` type, token lifecycle in `DispatchOverflow`/`safeBatchHandle`/`Stop`, `drainOverflow` race fix
- `internal/component/bgp/reactor/forward_pool_test.go` -- 7 new tests (3 AC tests + 4 deep-review gap tests)
- `internal/component/bgp/reactor/reactor.go` -- `ze.fwd.pool.size` env var registration and wiring
