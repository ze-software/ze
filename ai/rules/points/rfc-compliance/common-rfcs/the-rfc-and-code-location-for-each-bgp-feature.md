---
kind: directive
level: MUST
stage:
---
**A change to one of these BGP features MUST be read against the RFC in its row, at the code its row names:**

| Feature | RFC | Location |
|---------|-----|----------|
| BGP-4 base | 4271 | `internal/component/bgp/message/`, `internal/component/bgp/reactor/` |
| MP-BGP | 4760 | `internal/component/bgp/reactor/received_update.go`, `internal/core/bgp/attribute/` |
| 4-byte ASN | 6793 | `internal/core/bgp/capability/capability.go` |
| Add-Path | 7911 | `internal/core/bgp/capability/capability.go` |
| GR | 4724 | `internal/core/bgp/capability/capability.go` |
| Revised error handling | 7606 | `internal/component/bgp/reactor/received_update.go` |
