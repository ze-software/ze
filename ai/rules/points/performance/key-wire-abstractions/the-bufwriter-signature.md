---
kind: fence
level:
stage:
---
```go
type BufWriter interface {
    WriteTo(buf []byte, off int) int
    Len() int
}
```
