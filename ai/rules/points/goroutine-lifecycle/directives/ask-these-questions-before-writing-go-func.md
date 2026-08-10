---
kind: directive
level: MUST
stage:
---
**Before writing `go func()`:** Inside event loop? → MUST use channel + worker. Called per message? → MUST use channel + worker. One-time lifecycle? → MAY use `go func()`. Timer/scheduler? → MAY use `go func()` (dedicated goroutine with cancellation).
