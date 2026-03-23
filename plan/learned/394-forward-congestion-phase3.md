# 394 -- Forward Congestion Phase 3: Read Throttle

## Context

Phases 1 and 2 added a bounded overflow pool and Prometheus metrics. Without read throttling, a slow destination peer causes unbounded overflow growth (or pool exhaustion) because source peers continue reading at full speed. Phase 3 adds a `ReadThrottle` type that computes a sleep duration proportional to pool fill level and per-source overflow ratio, inserting it between TCP reads to reduce inflow from the peers causing the most pressure.

## Decisions

- Chose **closure-based dependency injection** (`poolFillRatio func() float64`, `sourceRatio func(string) float64`) over direct `fwdPool` reference. This keeps `ReadThrottle` testable in isolation without reactor dependencies, and maps directly to Phase 2's `PoolUsedRatio()` and `SourceOverflowRatios()`.
- Chose **proportional sleep table** (4 bands: 0-25%, 25-50%, 50-75%, 75-100%) over a simple linear ramp. Low-ratio sources are spared at moderate fill, only throttled at critical fill. This prevents innocent peers from being penalized for one bad source.
- Chose **keepalive/6 clamp** (AC-9) over keepalive/3 or keepalive/2. Six intervals per keepalive gives enough headroom for the source to deliver at least one message (UPDATE or keepalive) within the hold time, preventing false hold timer expiry.
- Chose **context-interruptible sleep** (`ThrottleSleep` checks `ctx.Done()`) over plain `time.Sleep`. Ensures clean shutdown without waiting for throttle timers to expire.
- Deferred **session wiring** (calling `ThrottleSleep` from the read loop) due to pre-existing peer.go compilation break. The throttle logic is complete and tested; wiring is mechanical.

## Consequences

- The throttle type is ready to wire into the session read loop. When the peer.go break is fixed, add `readThrottle *ReadThrottle` to `Session`, call `ThrottleSleep(ctx, sourceAddr, keepaliveInterval)` after each message read.
- Layer 4 (teardown) can use the same `poolFillRatio` signal to decide when to tear down a destination peer.
- The worktree agent approach produced usable code but with massive drift from HEAD. Manual adaptation was needed. For future worktree agents: ensure the worktree is created from HEAD with all uncommitted changes excluded.

## Gotchas

- **Worktree drift:** The worktree agent started from a clean HEAD but the working tree had 40+ uncommitted changes. The worktree's diff showed 144 files and 5000+ deletions -- mostly the "absence" of those uncommitted changes. Only the new throttle files were usable; all other changes had to be discarded.
- **Go test cache hides compilation breaks.** This bit us again in Phase 3 (same as Phase 2). Modifying any file in a package invalidates the cache and exposes broken dependencies.
- **Hold time 0 edge case.** RFC 4271 Section 4.4 allows hold time 0 (no timers). With keepalive=0, there is no safe sleep budget, so throttling is disabled entirely. This prevents division by zero in the keepalive/6 clamp.

## Files

- `internal/component/bgp/reactor/forward_pool_throttle.go` (NEW) -- `ReadThrottle` type, `ComputeSleep`, `ThrottleSleep`
- `internal/component/bgp/reactor/forward_pool_throttle_test.go` (NEW) -- 11 tests covering throttle table, clamping, easing, interruptibility
