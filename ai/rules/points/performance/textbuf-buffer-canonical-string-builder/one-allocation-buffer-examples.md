---
kind: fence
level:
stage:
---
```go
// var + String(): 1 alloc (string copy, buffer stays on stack)
var b textbuf.Buffer
return b.Reset().Str("0:").Uint16(asn).Byte(':').Uint32(assigned).String()

// New() + Slice(): 1 alloc (Buffer struct ~160B, zero-copy extract)
b := textbuf.New()
return b.Str("0:").Uint16(asn).Byte(':').Uint32(assigned).Slice()

// Pool + String(): 1 alloc (string copy, buffer returns to pool)
b := textbuf.Get()
defer b.Release()
return b.Str("0:").Uint16(asn).Byte(':').Uint32(assigned).String()
```
