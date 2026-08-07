---
kind: fence
level:
stage:
---
```go
// BAD: buildPayload allocates its own scratch buffer
func buildPayload(attrs []Attribute) []byte {
    buf := make([]byte, totalLen(attrs))   // ALLOCATION
    off := 0
    for _, a := range attrs {
        off += a.WriteTo(buf, off)
    }
    return buf
}

// GOOD: caller passes its buffer
func writePayload(buf []byte, off int, attrs []Attribute) int {
    start := off
    for _, a := range attrs {
        off += a.WriteTo(buf, off)
    }
    return off - start
}
```
