---
kind: fence
level:
stage:
---
```go
// Slice: zero-copy, consumed immediately by ParsePrefix
entry, _ := netip.ParsePrefix(b.Reset().Addr(addr).Byte('/').Int(int64(bits)).Slice())

// String: result stored in a struct field
peer.Label = b.Reset().Str("AS").Uint32(asn).String()
```
