---
kind: table
level:
stage:
---
| Scope | Command | Speed |
|-------|---------|-------|
| Single functional behavior | Run the owning compiled fixture's focused Go test, then the complete `./le functional <suite>` action | seconds plus suite |
| Functional suite | `./le functional <suite>` | suite budget |
| Encode or decode behavior | Focused Go test in the owning package, then `./le functional encode` or `./le functional decode` | seconds plus suite |
| Single editor behavior | Focused Go test under `internal/component/cli/testing`, then `./le functional editor` | seconds plus suite |
| ExaBGP compatibility | `./le functional exabgp-test` | suite budget |
| Single Go test | `./le job run label unit-pkg command go test PKG=./pkg/... RUN=TestName` | seconds |
| Single package | `./le job run label unit-pkg command go test PKG=./internal/component/bgp/reactor/` | seconds |
| Component group | `./le test-unit bgp` (or core, plugins, config, cli, rest) | 10s-1:30 |
| All unit tests | `./le test-unit` | ~5 min |
| All editor tests | `./le functional editor` | ~30s |
| Pre-commit gate | `./le verify worktree` | 4-10 min (see `tmp/.ze-verify-duration.txt`) |
