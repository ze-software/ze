# Spec: fib-p4-acceptance-and-state-disposition

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | Resolve acceptance and installed-state claims without overriding the explicit deferral of P4 backend implementation. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Establish and record the release disposition for existing `fib/p4` acceptance and the `show fib p4` installed-state presentation. Keep the owner decision deferring switch programming intact. Check whether the documented noop visibility is distinct from a claim that routes were installed; record an explicit owner disposition and concrete repair ownership for any misleading acceptance or state. Do not infer that help text alone proves the output honest, or commission a P4Runtime backend.

## Source and Evidence

Split from `plan/post-release/features/spec-fib-p4-leaves-reach-a-noop-backend.md` under owner decisions M30-M33. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in `plan/post-release/features/spec-fib-p4-leaves-reach-a-noop-backend.md`. Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-V1 | Read current config verification, help and installed-state producers | Record what is accepted and what the operator is actually told, distinguishing desired state from programmed state |
| AC-V2 | A state or acceptance promise exceeds the noop behavior | Record the explicit owner disposition and release repair owner without hiding the discrepancy as a feature deferral |
| AC-V3 | P4 backend remains deferred | The 2026-09-06 owner decision and all backend obligations remain with the feature spec |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Real P4Runtime backend, target/device/flush behavior and dependency decision, still owner-deferred | `plan/post-release/features/spec-fib-p4-leaves-reach-a-noop-backend.md` |
| Release proof and owner disposition of accepting noop configuration and displaying installed state | `plan/release/validation/spec-fib-p4-acceptance-and-state-disposition.md` |

Work assigned to `plan/post-release/features/spec-fib-p4-leaves-reach-a-noop-backend.md` is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.
