---
kind: fence
level:
stage:
---
```go
// Local use: 0 alloc (buffer on stack, inline array, no string created)
var b textbuf.Buffer
w.Write(b.Reset().Addr(addr).Byte(':').Uint16(port).Bytes())

// Map lookup: 0 alloc (compiler elides string([]byte) for map/switch)
var b textbuf.Buffer
val := m[string(b.Reset().Str(key).Bytes())]

// Pool loop: 0 alloc (amortized, string consumed before next Reset)
b := textbuf.Get()
defer b.Release()
for _, peer := range peers {
    key := b.Reset().Addr(peer.Addr).Byte(':').Uint16(peer.Port).Slice()
    val := lookupMap[key]
}

// AppendTo: 0 alloc (caller-owned buffer, no pool needed)
func (p *Peer) AppendTo(dst []byte) []byte {
    dst = textbuf.Addr(dst, p.Addr)
    dst = append(dst, ':')
    dst = textbuf.Uint(dst, uint64(p.Port))
    return dst
}
```
