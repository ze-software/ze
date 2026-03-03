# 269 — RR Serial Forward Worker

## Objective

Fix a ~98% route loss bug in the bgp-rr plugin under full-table load by replacing per-UPDATE goroutines with a single serialized worker goroutine, preserving FIFO message-ID order required by the engine's recent-cache ack protocol.

## Decisions

- Single buffered channel (capacity 4096) per RouteServer drains all forward/release work items — chosen over unbounded goroutines to prevent 522K concurrent goroutines racing to the engine.
- `forwardWork` struct carries msgID, sourcePeer, families, and a release flag — minimal struct, one type for both forward and release operations.
- Channel closed and worker drained after `p.Run()` returns — shutdown is clean with no dropped items.
- `go` calls in `handleStateDown`/`handleStateUp`/`handleRefresh` were intentionally left as-is — they send withdrawals/refresh requests, not cache forward/release commands, so their ordering relative to the cache is irrelevant.

## Patterns

- Channel + single worker is the correct pattern whenever FIFO ordering of SDK RPCs must be preserved.
- `OnEvent` callback must return promptly (it is a synchronous RPC) — the channel send is non-blocking from the callback's perspective, all blocking work moves to the worker.
- Test replaced the bug-demonstrating `TestForwardOrdering_ConcurrentGoroutines` with `TestForwardWorker_OrderPreserved` — only the fix is tested in the final codebase, not the bug.

## Gotchas

- Engine FIFO cache acks N implicitly acks 1..N-1. With concurrent goroutines, a later ID arriving first causes implicit eviction of earlier entries — this is the root cause of ~98% route loss at 522K routes.
- Channel buffer of 4096 was chosen to handle burst load; under sustained 522K routes it still backs up, but acts as flow control rather than spawning unbounded goroutines.

## Files

- `internal/plugins/bgp-rr/server.go` — `forwardWork` type, `workCh`, `runForwardWorker`, modified `handleUpdate` and `RunRouteServer`
- `internal/plugins/bgp-rr/propagation_test.go` — 3 new worker tests, 1 replaced ordering test
