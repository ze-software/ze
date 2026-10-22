# Spec: bgp-import-prepend-guards

| Field | Value |
|-------|-------|
| Status | deferred |
| Scope | protocol |
| Priority | P1 |
| Release reason | Existing import as-path-prepend must not silently modify route-server paths or contradict accepted no-prepend configuration. |
| Depends | - (spec-bgp-local-as-options closed 2026-10-09; no dependency on the deferred migration recipe) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Own AC-3 and AC-4 and the existing-policy safeguard work split from
`plan/post-release/features/spec-bgp-local-as-inbound-append.md`. The existing
import chain can prepend through `as-path-prepend` today. Refuse its combination
with `rs-client` or `local-options no-prepend` at config load. Neither refusal
requires adding automatic append or publishing the ASN-migration recipe.

The owner deferred that recipe on September 6. Its automatic inbound SHOULD
remains unimplemented; this split neither changes that decision nor changes
an RFC conformance verdict. This spec inherits `deferred` status and the
unscheduled state. Release ownership is not permission to lift an owner pause.

## Reciprocal ownership

| Original obligation | Owner |
|---------------------|-------|
| AC-3/4, implementation Phase 3, both refusal wiring rows/unit tests and route-server functional test/user story 3 | This spec |
| A-1 (whole config available to plugin), A-3 (route-server receives rewritten path), R-4 (new refusal diagnostics), R-6 (contradictory settings), guard-specific review and documentation | This spec |
| AC-1/2/5/6/7/8, recipe and width regression tests, other user stories, FRR interop, Phases 1/2/5, recipe portion of Phase 4, remaining assumptions/risks/boundaries and documentation | `plan/post-release/features/spec-bgp-local-as-inbound-append.md` |

The feature sibling retains the full original text, dated evidence and design
choices. Its guard references are historical/non-owning; publication depends
on these safeguards, not the reverse. Its existing width-aware producer is
preserved, not reclassified as a current mixed-width defect.

## Constraints and data flow

- Read the original Required Reading, Current Behavior and Key Design Decisions
  retained in the sibling. RFC 7947 Section 2.2.2.1's route-server transparency
  rule covers modifying AS_PATH, not only the egress prepend.
- The import chain rewrites received bytes before cache, RIB/plugin dispatch and
  forward rails. Stripping the edit later on the RS rail silently disagrees
  with configuration and is not the planned repair.
- The rs plugin's `OnConfigure` owns the `rs-client` refusal; the filter-modify
  plugin's `OnConfigure` owns the `no-prepend` refusal. A leaf `ValidateFn`
  cannot compare sibling settings. Resolve original A-1 by reading the delivered
  `bgp` subtree before implementing the checks; if it lacks the needed context,
  the original fallback is a whole-tree config validator, not a leaf check.
- A-3 remains unvalidated: prove that a second client receives the original
  path. Even if inspection changes its diagnosis to defence in depth, the
  original decision retains the refusal.
- Error messages name the peer, definition and conflicting leaf/settings and
  say what to remove. Warning-only acceptance or choosing a silent winner is
  not allowed (R-4/R-6).
- Do not reject all inbound prepend or peers lacking a local-as override;
  that is a legitimate general policy action and the original design explicitly
  did not specify that refusal. Keep outbound `secondaryPrependAS` behavior,
  ingress loop-detection order, ASN-width handling and existing schema ranges.

## Acceptance Criteria

| ID | Input / Condition | Expected behavior |
|----|-------------------|-------------------|
| AC-3 | A peer with import `as-path-prepend` is declared `rs-client` | Config load fails, naming the peer, the modify definition, the `as-path-prepend` leaf and RFC 7947 Section 2.2.2.1, and no client receives a modified path |
| AC-4 | A peer with import `as-path-prepend` also declares `local-options no-prepend` | Config load fails, naming both settings and stating that one of the two must go |

These are the original criteria for the existing import-policy combinations;
the original migrated-peer setup remains the fixture context, not a new feature
prerequisite or an exemption for other route-server import configurations.

## Implementation and proof obligations

1. Implement the original Phase 3 in
   `internal/component/bgp/plugins/rs/server.go` and
   `internal/component/bgp/plugins/filter_modify/config.go` at config load.
2. Own `TestRouteServerRefusesInboundASPathPrepend` in
   `internal/component/bgp/plugins/rs/config_refusal_test.go` and
   `TestNoPrependRefusesInboundASPathPrepend` in
   `internal/component/bgp/plugins/filter_modify/config_refusal_test.go`.
   Drive both through config entry, demonstrate each fails when its guard is
   removed, and preserve that configurations without the combination load.
3. Own `test/plugin/bgp-local-as-inbound-route-server.ci`: a second client
   receives the first client's path byte-identically, while config that would
   modify it is refused. This is the original user story 3 and A-3 proof.
4. Own guard-specific help, diagnostics and documentation in the existing
   filter-modify/BGP YANG texts, configuration guide, plugin guide and generated
   config reference. Recipe publication and width-rule explanation remain with
   the feature sibling. Preserve the original unaffected architecture-page
   declarations unless an implementation changes the described structure.

## Files to Modify

- `internal/component/bgp/plugins/rs/server.go` - route-server import-prepend refusal at config load (AC-3).
- `internal/component/bgp/plugins/filter_modify/config.go` - no-prepend/import-prepend conflict refusal at config load (AC-4).

The recipe and width-aware reactor producer remain with the feature sibling.
These are planned repair ownership paths, not implementation evidence.

## Remaining state

No refusal implementation or proof is claimed by this planning split. The
original source inspection and width tests marked "present, no fresh run" do
not satisfy these criteria. No new RFC tags, ledger changes or automatic-append
capability are part of this work.
