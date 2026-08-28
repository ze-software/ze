---
kind: note
level:
stage:
---
Race coverage: `./le verify current mode full` runs `-race` on component groups with changed `.go` files. For reactor concurrency changes, also run `go test -race -count=20 ./internal/component/bgp/reactor/...`.
