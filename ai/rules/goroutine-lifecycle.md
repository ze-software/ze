# Goroutine Lifecycle

**When:** before writing `go func()` anywhere
**Severity:** blocking

## Directives

All goroutines MUST be long-lived workers. Never per-event goroutines in hot paths.
Rationale: `ai/rationale/goroutine-lifecycle.md`

| Pattern | Status |
|---------|--------|
| Long-lived goroutine reading from channel | Required |
| Goroutine per lifecycle (process, session, peer) | OK |
| Goroutine per event in hot path | Forbidden |
| `go func()` inside `for range` on events | Forbidden |

Pattern: channel + worker. Start → create chan + start worker. Hot path → enqueue. Stop → close chan.

**`go func()` IS OK for:** component startup (one-time), test helpers, `ProcessManager.Stop()` wait, `Process.Wait()` bridge, timers and scheduled tasks (dedicated goroutine that sleeps/selects on a timer, cancellable via context or channel).

**Before writing `go func()`:** Inside event loop? → channel + worker. Called per message? → channel + worker. One-time lifecycle? → OK. Timer/scheduler? → OK (dedicated goroutine with cancellation).
