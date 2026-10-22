# Spec: ldp dynamic capability receive proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | The dynamic-capability unknown-TLV receive MUST is independent of optional local advertisement; the existing unknown-TLV path needs exact evidence. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Prove that an unknown Dynamic Capability Announcement Parameter with its U-bit set is silently ignored and the remaining Initialization message continues to be processed. The capability-status spec records the general unknown-TLV path as implemented; use that evidence rather than infer a new defect from missing send encoders.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: RFC5561-9-1 receive behavior is proven with a valid following parameter that is actually consumed.
- AC-2: the negative case distinguishes ignoring this permitted unknown TLV from terminating or skipping the rest of the message; general unknown-TLV handling remains intact.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| All original send/negotiated-capability requirements; RFC5561-9-1 transmit half only. | `plan/post-release/features/spec-ldp-capability-advertisement.md` |
| RFC5561-9-1 receiver half only; no capability advertisement or dynamic message sender. | `plan/release/validation/spec-ldp-dynamic-capability-receive-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| All original send/negotiated-capability requirements; RFC5561-9-1 transmit half only. | `plan/post-release/features/spec-ldp-capability-advertisement.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-ldp-capability-advertisement.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-ldp-capability-advertisement.md`. This child claims no new test result or completion.
