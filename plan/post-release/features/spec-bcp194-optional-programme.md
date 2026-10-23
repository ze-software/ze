# Spec: bcp194-optional-programme

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Optional BCP recommendations retain programme completion ownership without making absent features release prerequisites. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Retain the full BCP recommendation programme and all-child inventory/completion bookkeeping. Coordinate optional role-derived policy, prefix defaults, general community Functions 4/6, remaining RFC 8195 conventions and the separately owned damping decision. Release fixes retain their independent owners and do not wait for this programme. No new protocol behavior is authorized by moving the programme.

## Source and Evidence

Split from `plan/release/validation/spec-bcp194-0-umbrella.md` under the release-scope and spec-split rules in `ai/META-DECISIONS.md`. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in `plan/release/validation/spec-bcp194-0-umbrella.md`. Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The set is complete | Every row of the Gap Inventory names exactly one owning child, and every child file exists in `plan/` |
| AC-6 | A child is closed | Its inventory rows are marked done here, and any row it could not close has a deferral row with a destination spec that exists on disk |
| AC-P1 | A child is optional or already delivered | Its actual state is preserved; relocating or splitting is not implementation or closure |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-2/AC-3/AC-4/AC-5/AC-7; accurate recommendation/default evidence, ledger and shipped RFC 7999 claims | `plan/release/validation/spec-bcp194-0-umbrella.md` |
| Original AC-1/AC-6; full programme inventory and all-child completion, including optional children | `plan/post-release/features/spec-bcp194-optional-programme.md` |

Work assigned to `plan/release/validation/spec-bcp194-0-umbrella.md` is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.
