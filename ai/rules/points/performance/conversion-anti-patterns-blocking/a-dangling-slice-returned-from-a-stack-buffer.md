---
kind: fence
level:
stage:
---
```go
// BAD: Slice points into stack buffer; dangling after return
var b textbuf.Buffer
return b.Reset().Str("peer:").Uint32(asn).Slice()

// GOOD: String copies data out; safe to return
var b textbuf.Buffer
return b.Reset().Str("peer:").Uint32(asn).String()

// ALSO GOOD: heap Buffer, Slice is safe (GC traces interior pointer)
b := textbuf.New()
return b.Str("peer:").Uint32(asn).Slice()
```
