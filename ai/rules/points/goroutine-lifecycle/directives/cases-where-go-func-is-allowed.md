---
kind: directive
level: MAY
stage:
---
**`go func()` MAY be used for:** component startup (one-time), test helpers, `ProcessManager.Stop()` wait, `Process.Wait()` bridge, timers and scheduled tasks (dedicated goroutine that sleeps/selects on a timer, cancellable via context or channel).
