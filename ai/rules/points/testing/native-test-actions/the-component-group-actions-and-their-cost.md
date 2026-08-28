---
kind: table
level:
stage:
---
| Action | Scope | Approx time |
|--------|-------|-------------|
| `./le test-unit bgp` | `./internal/component/bgp/...` (96 pkgs) | ~1:30 |
| `./le test-unit core` | `./internal/core/...` (26 pkgs) | ~30s |
| `./le test-unit plugins` | `./internal/plugins/...` (44 pkgs) | ~40s |
| `./le test-unit config` | `./internal/component/config/...` (13 pkgs) | ~20s |
| `./le test-unit cli` | `./internal/component/cli/...` (3 pkgs) | ~10s |
| `go test -race <package-pattern>` | A package or pattern outside the five component groups | varies |
| `./le job run label unit-pkg command go test <package-pattern>` | One admitted focused Go test job | seconds |
