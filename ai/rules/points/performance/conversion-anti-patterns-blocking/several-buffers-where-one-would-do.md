---
kind: fence
level:
stage:
---
```go
// BAD: tb2, tb3, tb4 waste stack space
var tb2 textbuf.Buffer
msg2 := tb2.Str("Deleted ").Join(path, " ").String()
var tb3 textbuf.Buffer
msg3 := tb3.Str("Renamed ").Str(name).String()

// GOOD: one buffer, Reset between uses
var tb textbuf.Buffer
msg2 := tb.Str("Deleted ").Join(path, " ").String()
msg3 := tb.Reset().Str("Renamed ").Str(name).String()
```
