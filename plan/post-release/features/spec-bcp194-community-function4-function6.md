# Spec: bcp194-community-function4-function6

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | General eBGP Function 4 and Function 6 actions are absent conventions, separate from existing community repairs. |
| Depends | `plan/release/fixes/spec-bcp194-1-communities.md` ; inherited: spec-bcp194-0-umbrella |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Implement original AC-9 and AC-10: general eBGP Function 4 destination suppression and Function 6 ASN prepend. Own the former phase 5, its egress wiring, two functional cases and interop scenarios, plus parameter bounds and unchanged-other-destination controls. Do not duplicate route-server rail repairs or the already recorded ingress mechanisms.

## Source and Evidence

Split from `plan/release/fixes/spec-bcp194-1-communities.md` under the release-scope and spec-split rules in `ai/META-DECISIONS.md`. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in `plan/release/fixes/spec-bcp194-1-communities.md`. Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-9 | Route carries `<localAS>:4:<X>`, forwarded to a general eBGP peer whose ASN is X | That peer does not receive the route, and a peer whose ASN is not X does receive it |
| AC-10 | Route carries `<localAS>:6:<X>`, forwarded to a general eBGP peer whose ASN is X | That peer receives the route with `<localAS>` prepended once, and a peer whose ASN is not X receives an unchanged AS_PATH |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-1 to AC-8 and AC-11 to AC-21; former phases 1 to 4 and 6, including retained delivered history | `plan/release/fixes/spec-bcp194-1-communities.md` |
| Original AC-9/AC-10 and former phase 5, with general-rail feature tests and interop | `plan/post-release/features/spec-bcp194-community-function4-function6.md` |

Work assigned to `plan/release/fixes/spec-bcp194-1-communities.md` is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.
