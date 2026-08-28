---
kind: fence
level:
stage:
---
```bash
go test -race ./internal/component/bgp/message/... -v  # Single package
go test -race ./... -run TestName -v          # Single test
go test -race -cover ./...                    # Coverage
FUZZ=FuzzName TIME=30s ./le fuzz run          # Single fuzz target
```
