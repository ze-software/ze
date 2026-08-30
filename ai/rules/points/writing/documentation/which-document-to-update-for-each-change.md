---
kind: directive
level: MUST
stage:
---
**A change MUST update the document its row names when it changes user or agent behavior, changes an architecture contract, invariant or documented data flow, makes existing documentation stale, or adds a surface users or agents have to discover.** A private implementation change that meets none of those triggers needs no prose. Name the file and the section and describe the change: "update documentation" is not an instruction. Every spec carries the Documentation Update Checklist from `plan/TEMPLATE.md`, each row answered Yes or No, and each Yes naming the file and what to add.

| # | Category | Location | When to update |
|---|----------|----------|----------------|
| 1 | Feature list | `docs/features.md` | New user-facing feature |
| 2 | User guide | `docs/guide/<topic>.md` | Feature with usage instructions |
| 3 | Config syntax | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` | Config format changes |
| 4 | CLI reference | `docs/guide/command-reference.md` | New or changed CLI commands |
| 5 | API and RPC docs | `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md` | New or changed RPCs or event types |
| 6 | Plugin guide | `docs/guide/plugins.md`, `docs/plugin-development/` | Plugin SDK or lifecycle changes |
| 7 | Wire format | `docs/architecture/wire/*.md` | Encoding or decoding changes |
| 8 | Plugin SDK rules | `ai/rules/plugins.md` | Registration fields, protocol changes |
| 9 | RFC compliance | `rfc/short/rfcNNNN.md` | New RFC implementation |
| 10 | Test infrastructure | `docs/functional-tests.md`, `docs/architecture/testing/` | New test tools or patterns |
| 11 | Comparison | `docs/comparison.md` | Feature parity with other daemons |
| 12 | Architecture | `docs/architecture/core-design.md` or the subsystem doc | Structural design changes |
| 13 | Route metadata | `docs/architecture/meta/README.md` and `docs/architecture/meta/<plugin>.md` | A plugin sets or reads route metadata keys |
| 14 | Prometheus counters | `docs/plugin-development/metrics.md` or the subsystem telemetry doc | Counters or gauges added or changed |
| 15 | Agent discovery | `ai/rules/repo-maintenance.md`, `ai/INDEX.md` | New features, tools, self-checks, verification gates, test infrastructure, or agent workflows |
