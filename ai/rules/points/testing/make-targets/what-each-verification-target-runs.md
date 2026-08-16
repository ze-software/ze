---
kind: table
level:
stage:
---
| Target | Purpose |
|--------|---------|
| `make ze-precommit-verify` | Pre-commit gate: lint, changed-file wiring/doc/inventory, vet evidence, Linux/amd64 SCA (`govulncheck`), two-pass unit, functional, and ExaBGP |
| `make ze-precommit-verify-changed` | Changed-package lint/test plus wiring/doc/inventory, Linux/amd64 SCA (`govulncheck`), functional, and ExaBGP |
| `make ze-wiring-docs-check` | Changed-file-aware wiring, documentation, command, and inventory gate |
| `make ze-unit-test` | All unit tests with `-race` under default-on feature tags, plus bare `ze_core` compile-out checks (~5 min) |
| `make ze-functional-test` | All 13 functional test suites |
| `make ze-lint` | 26 linters |
| `make ze-ci-verify` | lint + unit + build |
| `make ze-fuzz-test` | Fuzz tests (10s per target) |
| `make ze-functional-exabgp-test` | ExaBGP compatibility via `ze-test exabgp --all` |
| `make ze-standard-test` | All tests including fuzz |
| `make ze-functional-editor-test` | Editor `.et` tests (headless TUI) |
| `make ze-chaos-test` | Chaos unit + functional + integration + web |
| `make ze-unit-reactor-test-race` | Stress race-test reactor (`-race -count=20`) -- REQUIRED when touching reactor concurrency code |
| `make ze-mutation-test` | Mutation testing via gomu on all non-excluded packages (advisory, slow) |
| `make ze-mutation-test-changed` | Incremental mutation testing on changed files only (advisory, fast) |
| `make ze-mutation-report` | Mutation testing with HTML report to `tmp/mutation-report.html` |
| `make ze-test-sensitivity-check` | Assert-nothing and tag-orphan ratchets (in `ze-precommit-verify`, both modes) |
| `make ze-weakened-check` | Selftests `scripts/dev/check_weakened_tests.py`, then checks that `test/weakened.md` parses (in `ze-precommit-verify`, both modes) |
| `make ze-test-health-update` | Regenerate `docs/features/test-health.md` + `test/health/latest.json` |
| `make ze-test-health-record` | Append one KPI sample to `test/health/history.ndjson` |
