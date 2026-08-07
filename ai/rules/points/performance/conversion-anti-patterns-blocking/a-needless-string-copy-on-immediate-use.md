---
kind: fence
level:
stage:
---
```go
// BAD: String() copies when the value is consumed immediately
emitLine(b, tb.Reset().Str(prefix).Str(name).String())

// GOOD: Slice() is zero-copy; valid until next Reset()
emitLine(b, tb.Reset().Str(prefix).Str(name).Slice())
```
