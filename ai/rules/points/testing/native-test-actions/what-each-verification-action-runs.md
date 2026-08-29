---
kind: table
level:
stage:
---
| Action | Purpose |
|--------|---------|
| `./le verify worktree` | Pre-commit gate: lint, changed-file wiring/doc/inventory, vet evidence, Linux/amd64 SCA (`govulncheck`), two-pass unit, functional, and ExaBGP |
| `./le verify worktree` | Changed-package lint/test plus wiring/doc/inventory, Linux/amd64 SCA (`govulncheck`), functional, and ExaBGP |
| `./le doc wiring` | Changed-file-aware wiring, documentation, command, and inventory gate |
| `./le test-unit` | All unit tests with `-race` under default-on feature tags, plus bare `ze_core` compile-out checks (~5 min) |
| `./le functional` | All 13 functional test suites |
| `./le verify lint run` | 26 linters |
| `./le verify current mode full` | Native lint, unit, functional, build, documentation, and structural stages |
| `./le fuzz run` | Every registered fuzz target |
| `./le functional exabgp-test` | ExaBGP compatibility |
| `./le verify worktree` | Full pre-commit proof over a detached committed tree |
| `./le functional editor` | Editor `.et` tests |
| `./le test-chaos` | Chaos simulator tests and checks |
| `go test -race -count=20 ./internal/component/bgp/reactor/...` | Required repeated reactor race proof |
| `go run github.com/sivchari/gomu/cmd/gomu run ...` | Advisory mutation execution |
| `./le mutation combine` | Combine native mutation reports |
| `./le mutation record-history` | Append package mutation scores to history |
| `./le test-sensitivity check` | Assert-nothing and tag-orphan ratchets (in `./le verify current mode full`, both modes) |
| `./le test-weakened check` | Selftests `internal/le/testweakened/testweakened.go`, then checks that `test/weakened.md` parses (in `./le verify current mode full`, both modes) |
| `./le test-health update` | Regenerate `docs/features/test-health.md` + `test/health/latest.json` |
| `./le test-health record` | Append one KPI sample to `test/health/history.ndjson` |
