# Spec: rsvp-multicast-interface-selection

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `spec-rsvp-routing-and-interface-integration.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |
| Priority | P3 |
| Release reason | Retained later capability or corpus expansion; shipped correctness and required release proof stay with the companion. |

## Task

Own the general multicast/interface-selection portion of RFC2205-3-33 that the source explicitly does not claim. This is backlog ownership only: the source does not authorize implementing its other listed requirements in the current completion phase, and this relocation does not do so either. Unicast route selection, transit diversion, non-local forwarding, ingress-route checks and loop prevention remain required release work in the companion.

## Acceptance Criteria

Original criterion ids from `plan/release/fixes/spec-rsvp-routing-and-interface-integration.md` are retained for this slice only; read the full inherited design and dated evidence in its Historical source record. Where a criterion spans both slices, the ownership table below is the exact boundary.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | General multicast RSVP interface selection is designed and implemented under its own approval | Path state chooses the correct incoming interface for Resv under RFC2205-3-33, with observable forwarding proof; no unicast proof is misrepresented as multicast proof. |

## Implementation Steps

Complete the original design/authorization boundary before implementation, then prove the general multicast path. Reuse the release owner’s native transport; do not duplicate or defer its fixes.

## Work Not Done

The companion retains all other obligations and the complete historical source record. This split is neither closure nor a claim of missing implementation; original approvals, dependencies, artifact requirements and proof standards remain in force.

| Original obligation | Implementation owner | Boundary |
|---------------------|----------------------|----------|
| RFC2205-2-11/2-12, 3-6/3-21/3-23, unicast 3-33 and 3-37/3-38/3-39/3-40/3-42 | `plan/release/fixes/spec-rsvp-routing-and-interface-integration.md` | Shipped routing/interface repairs and native proof |
| RFC2205-3-33: general multicast/interface-selection extension | `plan/post-release/features/spec-rsvp-multicast-interface-selection.md` | Absent extension, with original authorization limit retained |

## Required Reading

- `plan/release/fixes/spec-rsvp-routing-and-interface-integration.md`: full original source record, design, tests, decisions and evidence; apply them to this file’s owned slice only.
