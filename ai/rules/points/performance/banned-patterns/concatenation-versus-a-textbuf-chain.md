---
kind: fence
level:
stage:
---
```go
// BAD: 2 allocations (Uint result + concat)
return "peer:" + textbuf.StringUint32(asn)

// GOOD: 1 allocation (String())
var b textbuf.Buffer
return b.Str("peer:").Uint32(asn).String()

// BAD: N*2 + 1 allocations
parts := make([]string, len(items))
for i, m := range items {
    parts[i] = ">" + textbuf.StringUint(m.Value)
}
return strings.Join(parts, " ")

// GOOD: 1 allocation (String())
var b textbuf.Buffer
for _, m := range items {
    b.Byte(' ').Byte('>').Uint(m.Value)
}
return b.String()
```
