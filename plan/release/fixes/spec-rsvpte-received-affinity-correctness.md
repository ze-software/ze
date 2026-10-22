# Spec: rsvpte received affinity correctness

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | Existing RSVP-TE loose-hop expansion must not silently accept received affinity constraints it does not honor. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Establish current reachability of received C-Type 1 masks through decodeSessionAttr and resolveExplicitPath. For accepted constraints, meet the required three link-validation tests when choosing links to extend a loose ERO hop, or apply the refusal the RFC requires for unsupported input. Do not assert a current misroute solely from the historical Task; retain applicability evidence and avoid commissioning optional affinity origination.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: current received affinity masks and loose-hop selection are traced through the real packet entry.
- AC-2: every applicable current-path condition of RFC3209-4.7.4-1 and -2 is enforced and proven, rather than silently ignoring masks.
- AC-3: unsupported conditions have the required explicit handling and do not create state violating the requested constraints.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| New resource-class configuration, affinity origination and newly enabled selection conditions. | `plan/post-release/features/spec-rsvpte-resource-affinities.md` |
| RFC3209-4.7.4-1/-2 for current received constraints and existing loose-hop expansion; no new origination/config capability. | `plan/release/fixes/spec-rsvpte-received-affinity-correctness.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| New resource-class configuration, affinity origination and newly enabled selection conditions. | `plan/post-release/features/spec-rsvpte-resource-affinities.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-rsvpte-resource-affinities.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-rsvpte-resource-affinities.md`. This child claims no new test result or completion.
