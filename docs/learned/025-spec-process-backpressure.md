# 025 — Process Backpressure and Respawn Limits

## Objective

Implement write queue backpressure for API processes (drop events when queue full, resume when drained) and respawn limits (disable a process after too many crashes in a time window).

## Decisions

- HIGH_WATER=1000, LOW_WATER=100 — events are dropped (not blocked) when the write queue exceeds 1000 items; writing resumes when it drains to 100. Dropping prevents the engine from blocking on a slow consumer.
- Respawn limit: 5 respawns per ~63 second window (ExaBGP uses `respawn_timemask = 0xFFFFFF - 0b111111`). Exceeded limit → process disabled until reload.
- `queueDropped atomic.Uint64` counter for monitoring; `QueueSize()` and `QueueDropped()` methods expose stats.
- `RespawnEnabled bool` in `ProcessConfig` — opt-in, not default.
- Dropped events logged as warnings — not silently discarded.

## Patterns

- Write queue: `writeQueue chan []byte` on Process; `WriteEvent()` uses non-blocking send (`select { case q <- data: default: drop }`).
- Respawn tracking: `respawnTimes map[string][]time.Time` in ProcessManager, pruned to the time window on each respawn attempt.

## Gotchas

- None documented.

## Files

- `internal/component/plugin/process.go` — writeQueue, queueDropped, QueueSize(), QueueDropped(), WriteEvent() backpressure
- `internal/component/plugin/types.go` — ProcessConfig.RespawnEnabled
- `internal/component/plugin/` — ProcessManager respawn tracking, disabled map
