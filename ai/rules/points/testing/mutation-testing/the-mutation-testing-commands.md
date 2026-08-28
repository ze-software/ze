---
kind: table
level:
stage:
---
| Command | Purpose |
|---------|---------|
| `go run github.com/sivchari/gomu/cmd/gomu run --output json --incremental=false --fail-on-gate=false` | Full advisory mutation run |
| `go run github.com/sivchari/gomu/cmd/gomu run --output json --incremental --base-branch=main --fail-on-gate=false` | Changed-file advisory mutation run |
| `./le mutation combine` | Combine per-package JSON reports |
