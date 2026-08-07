---
kind: fence
level:
stage:
---
```go
// BAD: allocates N times inside the loop
for _, attr := range attributes {
    packed := attr.Pack()         // make([]byte, attr.Len()) inside
    copy(buf[off:], packed)       // copy into caller's buffer
    off += len(packed)
}

// GOOD: zero allocations
for _, attr := range attributes {
    off += attr.WriteTo(buf, off) // writes directly into caller's buffer
}
```
