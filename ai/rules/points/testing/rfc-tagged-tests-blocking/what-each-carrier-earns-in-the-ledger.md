---
kind: table
level:
stage:
---
| Carrier | Cell in the ledger | Executed by | Tier |
|---------|--------------------|-------------|------|
| `*_test.go` | `unit/verify` | `./le test-unit` | runs on every push |
| `*.ci` | `functional/verify` | `./le functional` | runs on every push, but ONLY from a suite that target actually runs: the tier is derived per-suite from the functional run's own suite list (`GATING`, in `internal/le/functional/actions.go`), so a `.ci` in a suite outside it (traffic, vrrp, flow-export, static, vpp, chaos) earns no verify tier, and `test/draft/` is skipped entirely |
| `*.et` | `editor/verify` | `./le functional editor` | runs on every push, on the same earned-per-suite basis as `*.ci` |
| `internal/le/interoplab/bgp/*_test.go` | `interop/nightly` | `./le integration interop` | scheduled, advisory |
| `internal/le/interoplab/ipsec/*_test.go` | `interop/nightly` | `./le integration interop-ipsec` | scheduled, advisory |
