---
kind: directive
level: MUST
stage:
---
**A change to reactor concurrency code MUST also be run under `go test -race -count=20 ./internal/component/bgp/reactor/...`.** `./le verify current mode full` already race-instruments the component groups your changed `.go` files reach.
