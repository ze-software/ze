---
kind: fence
level:
stage:
---
```go
var b textbuf.Buffer
for _, peer := range peers {
    key := b.Reset().Addr(peer.Addr).Byte(':').Uint16(peer.Port).Slice()
    lookupMap[key] = peer  // Slice valid until next Reset
}
```
