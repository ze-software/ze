# Spec: ldp downstream on demand mode

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Full configurable Downstream-on-Demand operation is additional to required request handling in the shipped DU mode. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Add full Downstream-on-Demand operation only as a separately admitted capability. The original request/release/abort and Hop Count requirements are not transferred out of release scope merely because the original title named DoD. This child owns only triggers introduced by enabling the optional distribution mode.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: a selected DoD mode originates and manages requests, releases and aborts as its RFC state machine requires.
- AC-2: all newly reachable conditions of the original RFC5036 requirement list have discriminating tagged evidence.
- AC-3: DU receive behavior and NH.13 handling remain correct with DoD disabled.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| All listed RFC5036 requirements for DU/unconditional receive and NH.13 behavior, including their response encoders and Hop Count receive addendum. | `plan/release/fixes/spec-ldp-label-request-path.md` |
| Optional DoD selection and mode-specific origination; no ownership of unconditional or DU receive requirements. | `plan/post-release/features/spec-ldp-downstream-on-demand-mode.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| All listed RFC5036 requirements for DU/unconditional receive and NH.13 behavior, including their response encoders and Hop Count receive addendum. | `plan/release/fixes/spec-ldp-label-request-path.md` |

## Provenance and retained evidence

Split from `plan/release/fixes/spec-ldp-label-request-path.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/release/fixes/spec-ldp-label-request-path.md`. This child claims no new test result or completion.
