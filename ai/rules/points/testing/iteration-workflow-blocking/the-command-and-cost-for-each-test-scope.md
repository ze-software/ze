---
kind: table
level:
stage:
---
| Scope | Command | Speed |
|-------|---------|-------|
| Single functional test | `ze-test bgp plugin N` or `ze-test ui N` | seconds |
| Resume functional suite | `ze-test bgp plugin --start N` or `ze-test ui --start N` | seconds to remaining suite |
| Single encode test | `ze-test bgp encode N` | seconds |
| Single editor test | `ze-test editor N` or `ze-test editor --pattern <name>` | seconds |
| Single ExaBGP compatibility test | `ze-test exabgp N` or `ze-test exabgp --start N` | seconds |
| Single Go test | `make ze-unit-pkg-test PKG=./pkg/... RUN=TestName` | seconds |
| Single package | `make ze-unit-pkg-test PKG=./internal/component/bgp/reactor/` | seconds |
| Component group | `make ze-unit-bgp-test` (or core, plugins, config, cli, rest) | 10s-1:30 |
| All unit tests | `make ze-unit-test` | ~5 min |
| All editor tests | `make ze-functional-editor-test` | ~30s |
| Pre-commit gate | `make ze-precommit-verify` | 4-10 min (see `tmp/.ze-verify-duration.txt`) |
