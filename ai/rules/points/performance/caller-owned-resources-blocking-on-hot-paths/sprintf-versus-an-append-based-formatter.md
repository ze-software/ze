---
kind: fence
level:
stage:
---
```go
// BAD: format() allocates a string each call
func format(addr netip.Addr, port uint16) string {
    return fmt.Sprintf("%s:%d", addr, port)  // 3 allocations
}

// GOOD: caller owns the buffer
func appendFormat(buf []byte, addr netip.Addr, port uint16) []byte {
    buf = addr.AppendTo(buf)
    buf = append(buf, ':')
    buf = textbuf.Uint(buf, uint64(port))
    return buf
}

// Or with textbuf.Buffer when you need a string:
var b textbuf.Buffer
return b.Addr(addr).Byte(':').Uint16(port).String()  // 1 allocation
```
