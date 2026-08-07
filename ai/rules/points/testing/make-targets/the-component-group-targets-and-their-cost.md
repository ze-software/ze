---
kind: table
level:
stage:
---
| Target | Scope | Approx time |
|--------|-------|-------------|
| `make ze-test-bgp` | `./internal/component/bgp/...` (96 pkgs) | ~1:30 |
| `make ze-test-core` | `./internal/core/...` (26 pkgs) | ~30s |
| `make ze-test-plugins` | `./internal/plugins/...` (44 pkgs) | ~40s |
| `make ze-test-config` | `./internal/component/config/...` (13 pkgs) | ~20s |
| `make ze-test-cli` | `./internal/component/cli/...` (3 pkgs) | ~10s |
| `make ze-test-rest` | Everything not in a named group (~70 pkgs) | ~1:00 |
| `make ze-test-pkg PKG=<pattern>` | ONE package, or any pattern. `RUN=<regexp>` narrows, `RACE=0` drops `-race` while iterating | seconds |
