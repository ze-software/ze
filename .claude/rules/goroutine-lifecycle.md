# Goroutine Lifecycle

**BLOCKING:** All goroutines MUST be long-lived workers. Never per-event goroutines in hot paths.
Rationale: `.claude/rationale/goroutine-lifecycle.md`

## Rules

| Pattern | Status |
|---------|--------|
| Long-lived goroutine reading from channel | Required |
| Goroutine per lifecycle (process, session, peer) | OK |
| Goroutine per event in hot path | Forbidden |
| `go func()` inside `for range` on events | Forbidden |

Pattern: channel + worker goroutine. Component start → create chan + start worker. Hot path → enqueue. Stop → close chan.

## go func() IS OK For

Component startup (one-time), test helpers, `ProcessManager.Stop()` wait, `Process.Wait()` bridge.

## Before Writing `go func()`

1. Inside event loop? → channel + worker
2. Called per message? → channel + worker
3. One-time lifecycle? → OK
