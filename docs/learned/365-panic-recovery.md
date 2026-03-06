# 365 — Panic Recovery

## Objective

Add panic recovery boundaries to all goroutines in Ze's BGP subsystem, matching ExaBGP's failure domain model: per-peer faults cause session teardown and re-establishment, never daemon crash.

## Decisions

- Recovery wraps `runOnce()` via `safeRunOnce()` — panic becomes error, feeds into existing backoff loop
- Delivery goroutine recovery exits the loop (drops remaining items) — correct because session tears down anyway
- `sendInitialRoutes` recovery clears the atomic flag to prevent stuck peer state
- Listener uses `safeHandle()` per-connection wrapper — accept loop survives handler panics
- Signal handler and gap scan use safe wrapper functions for per-invocation recovery
- All recovery points log with `runtime.Stack()` for post-mortem debugging
- Used `rec` instead of `r` for recover variable in `monitor()` to avoid shadowing the `*Reactor` receiver

## Patterns

- Recovery pattern: `defer func() { if r := recover(); r != nil { log + stack + cleanup } }()`
- Two styles: wrapper function (`safeRunOnce`, `safeHandle`, `safeHandleSignal`, `safeRunGapScan`) for loop-continues cases; inline defer for exit-cleanly cases (delivery goroutine)
- Stack trace: `buf := make([]byte, 4096); n := runtime.Stack(buf, false)` — 4KB captures relevant frames
- Existing pattern in codebase: `safeBatchHandle` (forward_pool) and `safeHandle` (bgp-rs worker)

## Gotchas

- Delivery goroutine recovery exits the `for range` loop — remaining buffered items are dropped, not reprocessed. This is correct because the session tears down after `deliveryDone` closes.
- `sendInitialRoutes` has multiple code paths that clear the flag — the recovery defer's `Store(0)` is idempotent with all of them
- Session cancel goroutine recovery calls `closeConn()` which could theoretically re-panic — accepted as minimal risk since `closeConn` is a simple wrapper
- Variable shadowing trap: `func (r *Reactor) monitor()` + `if r := recover()` shadows receiver — use different name

## Files

- `internal/component/bgp/reactor/peer.go` — `safeRunOnce()`, delivery goroutine recovery
- `internal/component/bgp/reactor/peer_initial_sync.go` — `sendInitialRoutes()` recovery
- `internal/component/bgp/reactor/session.go` — cancel goroutine recovery
- `internal/component/bgp/reactor/listener.go` — `safeHandle()`, cancel goroutine recovery
- `internal/component/bgp/reactor/reactor.go` — `monitor()` recovery
- `internal/component/bgp/reactor/signal.go` — `safeHandleSignal()`
- `internal/component/bgp/reactor/recent_cache.go` — `safeRunGapScan()`
- `internal/component/bgp/plugins/bgp-rs/worker.go` — `drainLoop()` recovery
- `internal/component/bgp/reactor/panic_recovery_test.go` — 6 tests
