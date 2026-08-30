# Goroutine Lifecycle

**When:** before writing `go func()` anywhere
**Severity:** blocking

## Directives

**Every goroutine MUST be a long-lived worker. A per-event goroutine in a hot path is forbidden.** The permitted shapes and the channel-plus-worker skeleton are in `docs/contributing/ze-go-style.md`, "Goroutines".

**`go func()` MAY be used for:** component startup (one-time), test helpers, `ProcessManager.Stop()` wait, `Process.Wait()` bridge, timers and scheduled tasks (dedicated goroutine that sleeps/selects on a timer, cancellable via context or channel).

**Before writing `go func()`:** Inside event loop? → MUST use channel + worker. Called per message? → MUST use channel + worker. One-time lifecycle? → MAY use `go func()`. Timer/scheduler? → MAY use `go func()` (dedicated goroutine with cancellation).
