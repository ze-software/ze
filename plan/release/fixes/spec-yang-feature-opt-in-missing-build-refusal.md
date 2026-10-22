# Spec: yang feature opt in missing build refusal

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | BFD and LDP opt-ins survive builds without their implementation and can silently lose the requested liveness or synchronization behavior. |
| Scope | config |
| Depends | `spec-bgp-live-receive-binding-and-bfd-keep-session`, closed 2026-10-13 (it edits the BGP `connection bfd` readers and the BFD client commit refusal; this spec moves the schema those readers read, so it lands after that spec closes) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Make V-1..V-4 settings belong to their implementing feature and refuse them in builds lacking that feature. Keep the closed BGP-live-receive/BFD prerequisite from the original Depends row. This owns absent-schema admission, while spec-plugin-optional-dependency-refusal owns runtime dependency refusal when schema is present; neither is a substitute for the other.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-2 | `ze-bgp-conf.yang`, `ze-ospf-conf.yang`, `ze-static-conf.yang` after the change | contain no node named `bfd` or `bfd-profile`, and no description that names BFD as a setting they define |
| AC-3 | Build with `ze_bgp` and without `ze_bfd`; config sets `bgp peer <p> connection bfd` | parse refuses it with the unknown-key error naming `bfd`, exit non-zero |
| AC-4 | Same for OSPF (`ze_ospf`, no `ze_bfd`) interface `bfd`, and for static `bfd-profile` (no `ze_bfd`) | refused with the unknown-key error |
| AC-5 | Build with `ze_bfd` and without `ze_bgp` | links no BGP schema package (`go list -deps` shows no `internal/component/bgp/yang`) and loads its schema |
| AC-6 | Build with `ze_ospf` and without `ze_ldp`; config sets OSPF `ldp-sync` | refused with the unknown-key error |
| AC-7 | `ze-ospf-conf.yang` after the change | contains no `ldp-sync` node |

AC-1 and AC-10 apply here to V-1..V-4: full-tag BFD/LDP configurations parse and dump identically and every moved node preserves help. Apply the ownership rule from AC-13 to this repair without waiting for the placement-only sweep.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| V-5..V-8 placement; AC-8,9,11,12,13 and AC-1/10 for those nodes; durable ownership rule. | `plan/post-release/engineering/spec-yang-feature-opt-ins-owned-by-feature.md` |
| V-1..V-4; AC-2..7 and AC-1/10 for BFD/LDP nodes; repair-specific ownership documentation. | `plan/release/fixes/spec-yang-feature-opt-in-missing-build-refusal.md` |

## Files to Modify

- `internal/component/bgp/yang/ze-bgp-conf.yang` - move only `connection bfd` ownership for V-1.
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - move the BFD and LDP-sync nodes for V-2 and V-4.
- `internal/plugins/static/yang/ze-static-conf.yang` - move `bfd-profile` for V-3.
- `internal/component/bfd/clients/bgp/yang/`, `internal/component/bfd/clients/ospf/yang/`, `internal/component/bfd/clients/static/yang/` - planned BFD-owned augment packages with the original compound build gates; the BGP directory remains subject to original A-2.
- `internal/plugins/ldp/clients/ospf/yang/` - planned LDP-owned OSPF augment package.
- `feature-gates.txt`, `internal/component/plugin/all/all*.go` - gate declarations and generated imports for these BFD/LDP augment packages only.

This list includes planned augment destinations, not a claim they exist or
that the repair landed. Watchdog, route-refresh, interface and RIB placement
work remains with the engineering sibling.

## Work Not Done

| Scope | Current owner |
|---|---|
| V-5..V-8 placement; AC-8,9,11,12,13 and AC-1/10 for those nodes; durable ownership rule. | `plan/post-release/engineering/spec-yang-feature-opt-ins-owned-by-feature.md` |

## Provenance and retained evidence

Split from `plan/post-release/engineering/spec-yang-feature-opt-ins-owned-by-feature.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/engineering/spec-yang-feature-opt-ins-owned-by-feature.md`. This child claims no new test result or completion.
