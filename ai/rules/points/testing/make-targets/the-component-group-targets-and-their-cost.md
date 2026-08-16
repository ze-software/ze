---
kind: table
level:
stage:
---
| Target | Scope | Approx time |
|--------|-------|-------------|
| `make ze-unit-bgp-test` | `./internal/component/bgp/...` (96 pkgs) | ~1:30 |
| `make ze-unit-core-test` | `./internal/core/...` (26 pkgs) | ~30s |
| `make ze-unit-plugins-test` | `./internal/plugins/...` (44 pkgs) | ~40s |
| `make ze-unit-config-test` | `./internal/component/config/...` (13 pkgs) | ~20s |
| `make ze-unit-cli-test` | `./internal/component/cli/...` (3 pkgs) | ~10s |
| `make ze-unit-rest-test` | Everything not in a named group (~70 pkgs) | ~1:00 |
| `make ze-unit-pkg-test PKG=<pattern>` | ONE package, or any pattern. `RUN=<regexp>` narrows, `RACE=0` drops `-race` while iterating | seconds |
