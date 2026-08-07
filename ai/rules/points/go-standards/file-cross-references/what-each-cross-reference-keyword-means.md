---
kind: table
level:
stage:
---
| Keyword | Direction | Meaning | Example |
|---------|-----------|---------|---------|
| `// Detail:` | Hub -> Leaf | "details of this topic are in X" | `reactor.go` -> `reactor_api.go` |
| `// Overview:` | Leaf -> Hub | "broader context is in X" | `reactor_api.go` -> `reactor.go` |
| `// Related:` | Peer <-> Peer | "sibling at same level" | `reactor_api_batch.go` <-> `reactor_api_forward.go` |
