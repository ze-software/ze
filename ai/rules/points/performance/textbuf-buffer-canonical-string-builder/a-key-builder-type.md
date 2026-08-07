---
kind: fence
level:
stage:
---
```go
type keyBuilder struct{ strings.Builder }
func (b *keyBuilder) Sep()                 { b.WriteByte('|') }
func (b *keyBuilder) Uint(v uint64)        { var buf [20]byte; b.Write(textbuf.Uint(buf[:0], v)) }
func (b *keyBuilder) Addr(addr netip.Addr) { var buf [39]byte; b.Write(textbuf.Addr(buf[:0], addr)) }
```
