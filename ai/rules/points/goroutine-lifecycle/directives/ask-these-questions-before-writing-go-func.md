---
kind: directive
level:
stage:
---
**Before writing `go func()`:** Inside event loop? → channel + worker. Called per message? → channel + worker. One-time lifecycle? → OK. Timer/scheduler? → OK (dedicated goroutine with cancellation).
