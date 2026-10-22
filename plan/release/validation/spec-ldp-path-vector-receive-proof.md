# Spec: ldp path vector receive proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | Unqualified received Path Vector requirements need role/applicability evidence independently of optional configured loop-detection origination. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Resolve applicability of RFC5036-3.4.5.1.1-1..3 and RFC5036-3.4.5.1.2-1..3 for the current LDP receiver. Read their surrounding optional-mechanism context rather than treating the isolated MUST or absent configuration as sufficient. For required reachable behavior, implement/prove loop rejection, notification and processing termination; if a requirement truly binds only an unoffered role, record the explicit grounded classification rather than waive it. Consume the release label-request owner for DU request plumbing.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: each of the six receive rows has a documented current-role applicability result with RFC context and producer.
- AC-2: every applicable received Path Vector case has discriminating positive/negative evidence for loop detection, rejection and notification.
- AC-3: no configured-origination feature is required merely to exercise an unconditional receive obligation.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Configured Loop Detection and origination: RFC5036-2.8.1-* / 2.8.2-* / 3.4.4.1-3. | `plan/post-release/features/spec-ldp-loop-detection.md` |
| RFC5036-3.4.5.1.1-1..3 and 3.4.5.1.2-1..3 receive applicability/proof/repair; no new configuration capability. | `plan/release/validation/spec-ldp-path-vector-receive-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Configured Loop Detection and origination: RFC5036-2.8.1-* / 2.8.2-* / 3.4.4.1-3. | `plan/post-release/features/spec-ldp-loop-detection.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-ldp-loop-detection.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-ldp-loop-detection.md`. This child claims no new test result or completion.
