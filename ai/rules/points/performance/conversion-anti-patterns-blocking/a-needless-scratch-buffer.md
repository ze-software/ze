---
kind: fence
level:
stage:
---
```go
// BAD: scratch tb just to write into b
func render(b *textbuf.Buffer, name string) {
    var tb textbuf.Buffer
    b.Str(tb.Str("prefix:").Str(name).String())
}

// GOOD: write directly to the output buffer
func render(b *textbuf.Buffer, name string) {
    b.Str("prefix:").Str(name)
}
```
