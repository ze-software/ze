# 278 — RR Flow Control (End-to-End Backpressure)

## Objective

Wire end-to-end flow control so the route reflector achieves 100% route reflection under asymmetric load by connecting existing but unconnected building blocks: backpressure detection, peer pause gate, and forward pool blocking.

## Decisions

- Hysteresis is mandatory for pause/resume: pause at 75% channel capacity, resume at 25%. Without hysteresis, a channel oscillating near a single threshold generates a flood of RPCs.
- Pause is per-source-peer, not global: each source peer's backpressure is independent. If only one peer floods, only that peer gets paused — others continue at full speed.
- Forward pool backpressure is implicit, not signaled: when `fwdPool.Dispatch` blocks → `ForwardUpdate` RPC blocks → RR `processForward` blocks → RR worker channel fills → backpressure detected → source paused. The existing blocking chain becomes the signal path — no new signaling needed.
- Env vars for pool sizing (`ZE_RR_CHAN_SIZE`, `ZE_FWD_CHAN_SIZE`, `ZE_CACHE_SAFETY_VALVE`), not auto-scaling: dynamic channel resizing is impossible in Go (channels are fixed-size at creation). Env vars are explicit, tunable per deployment, and the existing `chanSize <= 0 → 64` guard provides safe defaults.
- Unbounded `EventBuffer` instead of ring buffer for chaos simulator: user chose no dropped events over bounded memory. `EventDroppedEvents` constant kept for backwards compat but never emitted.
- Shutdown cleanup: `Stop()` calls `resumeAllPaused()` to resume all paused peers. Without this, peers remain paused after the RR exits, blocking subsequent sessions.

## Patterns

- Low-water callback on worker pool: `onLowWater(key)` fires when channel drops below 25% AND was previously in backpressure. Clear-on-read semantics for `BackpressureDetected` prevent duplicate signals per transition.
- `PausePeer`/`ResumePeer` RPC handlers use existing `bgp peer pause/resume <addr>` command path via `UpdateRoute` RPC — same path as all other peer commands. No new YANG schema needed.

## Gotchas

- Data race in chaos simulator: the `Drain` goroutine outlived the `readLoop`, causing concurrent access to the events channel. Fixed by joining `Drain` before `readLoop` returns via child context.
- Handler RPC count test (`TestBgpHandlerRPCs`) expected 22 but became 24 after adding pause/resume handlers. Always update the count test when adding handlers.
- Functional test for full pause→drain→resume→multi-peer verification is deferred to spec-inprocess-chaos. The `rr-backpressure.ci` test covers env var parsing and single-peer lifecycle only.

## Files

- `internal/component/bgp/handler/bgp.go` — `bgp peer pause/resume` handlers + `peerFlowControl` shared impl
- `internal/component/bgp/plugins/rs/worker.go` — `onLowWater`, `inBackpressure`, configurable chanSize
- `internal/component/bgp/plugins/rs/server.go` — `wireFlowControl`, `resumeAllPaused`, dispatch backpressure, `ZE_RR_CHAN_SIZE`
- `internal/component/bgp/reactor/recent_cache.go` — `SetSafetyValveDuration`
- `cmd/ze-chaos/peer/ringbuf.go` — `EventBuffer` (unbounded push/drain)
- `cmd/ze-chaos/peer/simulator.go` — `EventBuffer` in readLoop with Drain goroutine lifecycle
- `internal/component/bgp/reactor/reactor.go` — `ZE_CACHE_SAFETY_VALVE` + `ZE_FWD_CHAN_SIZE` env vars
- `test/plugin/rr-backpressure.ci` — functional test: single-peer RR with `ZE_RR_CHAN_SIZE=1`
