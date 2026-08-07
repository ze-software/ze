---
kind: fence
level:
stage:
---
```go
func (t *MyType) AppendTo(buf []byte) []byte {
    buf = append(buf, "prefix "...)
    buf = textbuf.Uint(buf, uint64(t.Field))
    buf = append(buf, ':')
    buf = textbuf.Addr(buf, t.Addr)
    return buf
}

func (t *MyType) String() string {
    var b textbuf.Buffer
    // Call AppendTo on Buffer's internal slice... or just chain:
    return textbuf.StringAddr(t.Addr)
}
```
