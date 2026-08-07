---
kind: table
level:
stage:
---
| Carrier | Cell in the ledger | Executed by | Tier |
|---------|--------------------|-------------|------|
| `*_test.go` | `unit/verify` | `make ze-unit-test` | runs on every push |
| `*.ci` | `functional/verify` | `make ze-functional-test` | runs on every push, but ONLY from a suite that target actually runs: the tier is derived per-suite from `mk/test-functional.mk`'s own `all_suites=` line, so a `.ci` in a suite outside it (traffic, vrrp, ipsec, flow-export, static, vpp, chaos) earns no verify tier, and `test/draft/` is skipped entirely |
| `*.et` | `editor/verify` | `make ze-editor-test` | runs on every push, on the same earned-per-suite basis as `*.ci` |
| `test/interop/scenarios/*/check.py` | `interop/nightly` | `make ze-interop-test` | scheduled, ADVISORY |
| `test/ipsec-interop/scenarios/*/check.py` | `interop/nightly` | `make ze-ipsec-interop-test` | scheduled, ADVISORY |
