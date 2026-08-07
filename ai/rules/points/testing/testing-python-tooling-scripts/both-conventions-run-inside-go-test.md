---
kind: note
level:
stage:
---
Both land inside `go test`, so `make ze-unit-test` covers them via `go list ./...`
and no make target is needed. `scripts/dev` and `scripts/evidence` are test-only Go
packages that exist for exactly this.
