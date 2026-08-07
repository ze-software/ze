---
kind: table
level:
stage:
---
| New code in | Must be called from |
|-------------|---------------------|
| `internal/component/host/` | `cmd/ze/hub/main.go`, `loader_create.go`, `internal/component/cmd/show/system.go`, or `web/page_system.go` |
| `internal/component/config/system/` | `cmd/ze/hub/main.go` (startup + reload) |
| Any new metrics registration | `loader_create.go` telemetry block |
| Any new report bus emission | Verified via `show warnings` / `show errors` |
