---
kind: table
level:
stage:
---
| Target | Purpose |
|--------|---------|
| `make ze-verify` | Pre-commit gate: lint, changed-file wiring/doc/inventory, vet evidence, two-pass unit, functional, and ExaBGP |
| `make ze-verify-changed` | Changed-package lint/test plus wiring/doc/inventory, functional, and ExaBGP |
| `make ze-verify-wiring-docs` | Changed-file-aware wiring, documentation, command, and inventory gate |
| `make ze-unit-test` | All unit tests with `-race` under default-on feature tags, plus bare `ze_core` compile-out checks (~5 min) |
| `make ze-functional-test` | All 13 functional test suites |
| `make ze-lint` | 26 linters |
| `make ze-ci` | lint + unit + build |
| `make ze-fuzz-test` | Fuzz tests (10s per target) |
| `make ze-exabgp-test` | ExaBGP compatibility via `ze-test exabgp --all` |
| `make ze-test` | All tests including fuzz |
| `make ze-editor-test` | Editor `.et` tests (headless TUI) |
| `make ze-chaos-test` | Chaos unit + functional + integration + web |
| `make ze-race-reactor` | Stress race-test reactor (`-race -count=20`) -- REQUIRED when touching reactor concurrency code |
| `make ze-mutation-test` | Mutation testing via gomu on all non-excluded packages (advisory, slow) |
| `make ze-mutation-changed` | Incremental mutation testing on changed files only (advisory, fast) |
| `make ze-mutation-report` | Mutation testing with HTML report to `tmp/mutation-report.html` |
| `make ze-test-sensitivity-check` | Assert-nothing and tag-orphan ratchets (in `ze-verify`, both modes) |
| `make ze-test-health` | Regenerate `docs/features/test-health.md` + `test/health/latest.json` |
| `make ze-test-health-record` | Append one KPI sample to `test/health/history.ndjson` |
