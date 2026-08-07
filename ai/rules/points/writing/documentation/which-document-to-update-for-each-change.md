---
kind: table
level:
stage:
---
| # | Category | Location | When to update |
|---|----------|----------|----------------|
| 1 | Feature list | `docs/features.md` | New user-facing feature |
| 2 | User guide | `docs/guide/<topic>.md` | Feature with usage instructions |
| 3 | Config syntax | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` | Config format changes |
| 4 | CLI reference | `docs/guide/command-reference.md` | New/changed CLI commands |
| 5 | API/RPC docs | `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md` | New/changed RPCs or event types |
| 6 | Plugin guide | `docs/guide/plugins.md`, `docs/plugin-development/` | Plugin SDK or lifecycle changes |
| 7 | Wire format | `docs/architecture/wire/*.md` | Encoding/decoding changes |
| 8 | Plugin SDK rules | `ai/rules/plugins.md` | Registration fields, protocol changes |
| 9 | RFC compliance | `rfc/short/rfcNNNN.md` | New RFC implementation |
| 10 | Test infrastructure | `docs/functional-tests.md`, `docs/architecture/testing/` | New test tools or patterns |
| 11 | Comparison | `docs/comparison.md` | Feature parity with other daemons |
| 12 | Architecture | `docs/architecture/core-design.md` or subsystem doc | Structural design changes |
| 13 | Route metadata | `docs/architecture/meta/README.md` + `docs/architecture/meta/<plugin>.md` | Plugin sets or reads route metadata keys |
| 14 | Prometheus counters | `docs/plugin-development/metrics.md` or subsystem telemetry doc | Counters/gauges added or changed |
| 15 | Agent discovery | `ai/rules/repo-maintenance.md`, `ai/INDEX.md` | New features, tools, self-checks, verification gates, test infrastructure, or agent workflows |
