# Spec: ipsec fragment receive and kernel proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | Existing exact-port IPsec behavior and required receive rejection need real-kernel proof independently of optional fragment transmission. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Establish in the QEMU lab whether the existing exact-port XFRM policies transmit non-initial fragments and observe the drop promised by the boundary tests. Prove required receive rejection (absence of an echoed notify when fragments are not accepted). Ignoring the notify is not by itself a demonstrated violation because RFC 7296 defines rejection by absence. If existing transmission triggers RFC4301-7.3-1, retain its repair in release scope here rather than moving it to the optional sender.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: RFC4301-7.3-2 has tagged positive and negative proof distinguishing acceptance from rejection.
- AC-2: a real-kernel test observes the drop described by RFC4301-7.3-3, -4 and -5, rather than only the programmed selector.
- AC-3: observed existing transmission determines RFC4301-7.3-1 applicability; any triggered notification requirement is met and proven before this release obligation closes.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Optional future transmission capability; RFC4301-7.3-1 only where the current path does not already trigger it. | `plan/post-release/features/spec-ipsec-non-first-fragments-also.md` |
| RFC4301-7.3-2 receive obligation, real-kernel 7.3-3..5 proof, and current-sender applicability/repair for 7.3-1. | `plan/release/validation/spec-ipsec-fragment-receive-and-kernel-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Optional future transmission capability; RFC4301-7.3-1 only where the current path does not already trigger it. | `plan/post-release/features/spec-ipsec-non-first-fragments-also.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-ipsec-non-first-fragments-also.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-ipsec-non-first-fragments-also.md`. This child claims no new test result or completion.
