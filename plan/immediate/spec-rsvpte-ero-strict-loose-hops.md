# Spec: RSVP-TE strict-hop adjacency and loose-hop expansion (RFC 3209 Sections 4.3.3.1, 4.3.4.2)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-23 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The started implementation replaces the old ERO-only next-hop selection with `internal/plugins/rsvpte/routing.go::resolveExplicitPath` and the transport's native route query. Strict hops must resolve to an adjacent member of their abstract node. Loose hops insert the resolved adjacent next hop before the remaining loose subobject; expansion is bounded. `resolveReceivedPath` validates and removes local subobjects before forwarding. This is source coverage awaiting Main's strict/loose wire scenarios and native RFC discrimination, not a claim of CSPF or full RFC 3209 support.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.3.3.1-1 | "The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node." (§4.3.3.1) | `routing.go::resolveExplicitPath` rejects a strict subobject whose resolved next hop is outside that abstract node |
| RFC3209-4.3.4.2-1 | "Each subobject in this series MUST denote an abstract node that is a subset of the current abstract node." (§4.3.4.2) | `routing.go::resolveReceivedPath` removes locally satisfied subobjects; `resolveExplicitPath` expands the remaining route using native adjacency information; source and error paths require validation |
