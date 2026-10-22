# Spec: policy existing actions proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | The umbrella still owes producer/evidence reconciliation for shipped policy actions and dry-run testing; absent named sets or live tracing must not hide that proof. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Map current producers and discriminating evidence to the action and `show policy test` dry-run criteria formerly owned by closed children. Do not reimplement those features solely because the umbrella remains open. Any observed supported-path defect stays in release scope. New named sets, route-server defaults and the inherited live `show policy explain` trace buffer remain with the feature parent.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-4 | modify filter with increment local-preference 50 on route with LP 100 | Route exits with local-preference 150 |
| AC-5 | modify filter with decrement med 30 on route with MED 20 | Route exits with MED 0 (floor, not underflow) |
| AC-6 | modify filter with community-add 65000:200 | Route gains community 65000:200, existing communities preserved |
| AC-7 | modify filter with community-remove 65000:100 | Community 65000:100 removed, others preserved |
| AC-8 | modify filter with large-community-add 65000:100:200 | Large community added to route |
| AC-9 | remove-private-as filter in export chain, AS_PATH contains 64512 | 64512 stripped from AS_PATH |
| AC-10 | remove-private-as with replace-with peer-as, peer AS is 65001 | Private ASN replaced with 65001 |
| AC-11 | as-path-length filter with max 30, route has 35-hop path | Route rejected |
| AC-14 | show policy test peer X import prefix 10.0.0.0/24 | JSON output with per-filter verdict |

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Absent named sets/reference validation and route-server defaults: AC-1..3,12,13,15,16; inherited live trace-buffer scope made explicit as AC-17. | `plan/post-release/features/spec-pol-0-umbrella.md` |
| Shipped policy actions AC-4..11 and dry-run `show policy test` AC-14 evidence reconciliation. | `plan/release/validation/spec-policy-existing-actions-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Absent named sets/reference validation and route-server defaults: AC-1..3,12,13,15,16; inherited live trace-buffer scope made explicit as AC-17. | `plan/post-release/features/spec-pol-0-umbrella.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-pol-0-umbrella.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-pol-0-umbrella.md`. This child claims no new test result or completion.
