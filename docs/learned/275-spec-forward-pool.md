# 275 — Forward Pool (Async Per-Peer Forwarding)

## Objective

Add per-destination-peer long-lived worker goroutines to the reactor's `ForwardUpdate` path so that a slow TCP write to one peer does not block forwarding to other peers in the same batch.

## Decisions

- Chose per-destination-peer workers over a shared pool: FIFO ordering per peer is guaranteed by the channel, and the bgp-rr `workerPool` was the ready-made pattern to follow (keyed by destination peer address instead of source peer).
- Pre-compute all send operations (split/re-encode decisions) synchronously in `ForwardUpdate`; workers only do TCP writes. This keeps complex context-dependent logic in one place.
- Fire-and-forget error semantics for TCP write failures: workers log errors but do not propagate them to the RPC caller. TCP failures trigger FSM state transitions independently — the plugin learns via a state event, not a send error.
- Added `dispatchWG sync.WaitGroup` (not in original spec): `Dispatch` is called from multiple RPC goroutines concurrently; the WaitGroup prevents Stop() from closing channels while a Dispatch is in progress. This was a required race fix not anticipated in the plan.
- Used `fp.clock.NewTimer()` instead of `time.NewTimer()`: reactor code must use the sim.Clock interface to allow deterministic testing (enforced by `sim.Clock` audit test).

## Patterns

- Channel + long-lived worker per key is the standard ze pattern for hot-path async delivery — same as `bgp-rr/worker.go`.
- Cache `Retain()`/`Release()` for buffer lifetime during async sends — avoids copying 4KB payloads. Retain before dispatch, Release in done callback. `Ack` (defer in ForwardUpdate) and Retain are independent refcount axes on `RecentUpdateCache`.
- `safeHandle` with panic recovery + guaranteed done callback: worker errors should never kill the worker goroutine.

## Gotchas

- DATA RACE: `w.ch <- item` (Dispatch) racing with `close(w.ch)` (Stop). Fixed with `dispatchWG` tracking in-flight dispatches — Stop waits for all to drain before closing channels. This was found by the race detector during testing.
- The idle timer must use `fp.clock.NewTimer()`, not `time.NewTimer()`. The reactor's sim.Clock audit test enforces this and will fail if real time is used.
- `defer Ack` fires when `ForwardUpdate` returns — before workers finish. This is correct: `totalConsumers() = pendingConsumers + retainCount`, so Ack decrements pendingConsumers while Retain keeps retainCount > 0, preventing premature buffer eviction.

## Files

- `internal/plugins/bgp/reactor/forward_pool.go` — `fwdPool` type, per-peer workers, `fwdHandler`, `fwdItem`
- `internal/plugins/bgp/reactor/forward_pool_test.go` — 10 pool lifecycle tests
- `internal/plugins/bgp/reactor/forward_update_test.go` — 3 integration tests for ForwardUpdate
- `internal/plugins/bgp/reactor/reactor.go` — `fwdPool` field, `New()`, `cleanup()` Phase 1, `ForwardUpdate` refactor
