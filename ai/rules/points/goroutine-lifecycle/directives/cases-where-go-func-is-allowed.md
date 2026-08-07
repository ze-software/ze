---
kind: directive
level:
stage:
---
**`go func()` IS OK for:** component startup (one-time), test helpers, `ProcessManager.Stop()` wait, `Process.Wait()` bridge, timers and scheduled tasks (dedicated goroutine that sleeps/selects on a timer, cancellable via context or channel).
