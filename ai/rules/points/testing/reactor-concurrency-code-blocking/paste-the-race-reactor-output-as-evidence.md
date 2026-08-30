---
kind: directive
level: MUST NOT
stage:
---
**A passing `./le test-unit` MUST NOT be offered as proof that a reactor
concurrency change is race-free.** MUST paste the admitted
`go test -race -count=20 ./internal/component/bgp/reactor/...` output as the
evidence.
