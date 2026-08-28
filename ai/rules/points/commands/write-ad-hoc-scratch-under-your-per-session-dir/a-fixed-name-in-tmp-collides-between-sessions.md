---
kind: note
level:
stage:
---
`tmp/` is shared by every concurrent session in this checkout (it is keyed
per-checkout, not per-session -- `internal/le/scratch/scratch.go`). A fixed name at
the `tmp/` root -- `tmp/out.log`, `tmp/stdout`, `tmp/gotest.log` -- collides with
a sibling session writing the same name, and is never cleaned when your session
ends.

**A file at the `tmp/` root is REFUSED, on both surfaces that create one**:
`bashScratch` and the Write/Edit path check in `internal/le/hookruntime` answer
alike. A path carrying a directory component passes, including a session's
private scratch path and a producer-owned subdirectory.
Session-keyed names and producer-owned root artifacts are explicit exceptions in
`internal/le/hookruntime.IsAdHocScratch`.
