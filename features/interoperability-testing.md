# Interoperability Testing

## Meta

| Field | Value |
|-------|-------|
| Name | Interoperability Testing |
| Page | docs/features/interoperability-testing.md |
| Kind | test-infra |
| Scope | complete |
| Level | experimental |
| Components | internal/le/interoplab, test/interop |
| Docs | docs/features/interoperability-testing.md |
| Doc review | 2026-10-07: 165 scenario directories under test/interop/scenarios, so 100+ holds |
| Defect review | 2026-10-07: plan/pre-release/spec-rfcgate-2-deferred-unrun-interop-trees.md ready (execution evidence unreviewed); journal 2026-09-04 interop image staleness rows unfixed |
| Extra criteria | supported: a dated record of each interop suite's pass count = test/interop/RESULTS.md |

## Description

100+ Docker-based interop scenarios against FRR, BIRD, and GoBGP; OpenBGPD, FreeRtr, and Rust implementations are exercised as performance-benchmark DUTs, not interop scenarios <!-- source: internal/le/rfc/actions.go -- Answer --> <!-- source: internal/le/test/health/actions.go -- Answer -->
