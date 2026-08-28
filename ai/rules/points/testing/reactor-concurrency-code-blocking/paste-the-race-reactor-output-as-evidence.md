---
kind: note
level:
stage:
---
A passing `./le test-unit` action is NOT proof that a reactor concurrency change is
race-free. Paste the admitted `go test -race -count=20 ./internal/component/bgp/reactor/...` output as evidence.
