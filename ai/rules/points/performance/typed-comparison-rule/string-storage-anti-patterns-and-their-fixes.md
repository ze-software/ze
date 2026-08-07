---
kind: table
level:
stage:
---
| Anti-pattern | Fix |
|-------------|-----|
| `type Foo struct { Addr string }` then `net.ParseIP(a.Addr).Compare(...)` | `type Foo struct { Addr netip.Addr }` then `a.Addr.Compare(b.Addr)` |
| Formatting to string, storing, parsing back for comparison | Parse once at construction, store typed, format only for display |
| `compareAddrs(a.PeerAddr, b.PeerAddr)` with string parsing | `a.PeerIP.Compare(b.PeerIP)` with `netip.Addr` field |
