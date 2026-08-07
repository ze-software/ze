---
kind: table
level:
stage:
---
| Artifact | Location |
|----------|----------|
| Implementation library | `component/<name>/` or `core/<name>/` |
| Config YANG (data model) | `component/<name>/yang/` |
| Command YANG (CLI tree) | `plugins/<name>/yang/` |
| RPC handlers | `plugins/<name>/cmd/` |
| Offline CLI registration | `plugins/<name>/register.go` |
| YANG embed + register | `plugins/<name>/yang/` (generated) |
| Blank imports | `all.go` (generated) |
