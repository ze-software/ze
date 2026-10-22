# Spec: ike transform support claims

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | The existing features page still advertises unimplemented cryptographic primitives; truthful release support does not wait for adding them. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Reconcile the IKEv2 Cryptographic Primitives row of docs/features.md with the current transform registry and the already-corrected IPsec guide. Preserve the missing-transform feature obligation without claiming it is implemented.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: the feature row names only implemented and proven transforms, with missing transforms described as unavailable.
- AC-2: guide and feature support claims agree with current registry/negotiation evidence.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| New transform implementation and its acceptance criteria; truthful newly earned support claims. | `plan/post-release/features/spec-ike-missing-transforms.md` |
| Current unsupported cryptographic-primitives claim in Known Limitations; no new transform code. | `plan/release/validation/spec-ike-transform-support-claims.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| New transform implementation and its acceptance criteria; truthful newly earned support claims. | `plan/post-release/features/spec-ike-missing-transforms.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-ike-missing-transforms.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-ike-missing-transforms.md`. This child claims no new test result or completion.
