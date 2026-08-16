---
kind: table
level:
stage:
---
| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `go test ./...` for verification | `make ze-precommit-verify` (two-pass + functional + exabgp) | `ai/rules/testing.md` | 349 packages; cached full + race on changed groups |
| Unit tests prove correctness | Unit tests + `.ci` functional tests (both required) | `ai/rules/completion.md` | Unit proves algorithm; `.ci` proves user can reach the feature |
| `testify/assert` | Standard library `testing` | (convention) | No test framework dependencies |
| `go test -race` once | `make ze-unit-reactor-test-race` (`-race -count=20`) for reactor code | `ai/rules/testing.md` | Rare schedules need repeated runs to surface |
