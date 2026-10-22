# Spec: ipsec vpp umbrella integration

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | VPP IPsec activation is an additional integration; the recorded XFRM-only release boundary does not defer native IKE/XFRM correctness. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Integrate the umbrella VPP acceptance criterion after the separately owned dataplane-selector, policy-interface and readback capabilities are ready. Those specs retain their implementation ownership; this child owns cross-feature umbrella acceptance only. The historical wording XFRM SAs via VPP means the VPP SA/SP interface, not Linux XFRM state inside VPP.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-12 | VPP backend active | XFRM SAs installed via VPP API instead of kernel netlink; traffic encrypted/decrypted by VPP |

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Native IKE/XFRM umbrella acceptance AC-1..11 and historical child/evidence reconciliation. | `plan/release/validation/spec-ipsec-0-umbrella.md` |
| Umbrella AC-12 integration only; underlying code remains with spec-ipsec-dataplane-selector, spec-ipsec-vpp-policy-interface and spec-ipsec-vpp-dataplane-readback. | `plan/post-release/features/spec-ipsec-vpp-umbrella-integration.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Native IKE/XFRM umbrella acceptance AC-1..11 and historical child/evidence reconciliation. | `plan/release/validation/spec-ipsec-0-umbrella.md` |

## Provenance and retained evidence

Split from `plan/release/validation/spec-ipsec-0-umbrella.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/release/validation/spec-ipsec-0-umbrella.md`. This child claims no new test result or completion.
