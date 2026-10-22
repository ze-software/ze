# Spec: RSVP-TE resource affinities (RFC 3209 Section 4.7.4)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-23 |

## Task

Add configurable link resource classes, resource-affinity origination and the complete resource-affinity capability. The historical source also records existing loose-hop expansion and ignored received masks; release applicability/handling of those accepted inputs belongs to the child. New capability design cannot waive required behavior of the existing receiver.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: resource classes and affinity masks are configurable and encoded by the new capability.
- AC-2: newly enabled link selection meets RFC3209-4.7.4-1 and RFC3209-4.7.4-2 with discriminating proof.
- AC-3: the release receive/selection guarantees remain intact with affinity origination disabled.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| New resource-class configuration, affinity origination and newly enabled selection conditions. | `plan/post-release/features/spec-rsvpte-resource-affinities.md` |
| RFC3209-4.7.4-1/-2 for current received constraints and existing loose-hop expansion; no new origination/config capability. | `plan/release/fixes/spec-rsvpte-received-affinity-correctness.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| RFC3209-4.7.4-1/-2 for current received constraints and existing loose-hop expansion; no new origination/config capability. | `plan/release/fixes/spec-rsvpte-received-affinity-correctness.md` |

## Historical planning record

The following is the complete pre-migration planning body. Its old Task, criteria, implementation steps and bucket statements are retained as history, not a second current implementation contract. The current sections above assign every obligation; original owner decisions, constraints, evidence and pauses remain authoritative. Heading levels alone are demoted for that distinction.

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

#### Task

**Backlog boundary:** This skeleton records an unstarted capability or conditional RFC obligation. The owner's authorization to finish already-started work does not select this feature. Its requirements are not completed or waived.

Resource affinities (SESSION_ATTRIBUTE C-Type 1 Exclude-any, Include-any and Include-all masks, per-link resource classes and the three link validation tests of Section 4.7.4) are a feature Ze does not offer today: `internal/plugins/rsvpte/frr.go::decodeSessionAttr` skips the three masks, Ze emits C-Type 7, and no interface carries a resource class. The owner can decline the feature in one word; until then every MUST below is owed.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.7.4-1 | "In order to be validated a link MUST pass the three tests below." (S4.7.4) | absent: `frr.go::decodeSessionAttr` skips the Exclude-any / Include-any / Include-all masks of C-Type 1 and Ze emits C-Type 7 (no affinities); no link validation exists |
| RFC3209-4.7.4-2 | "When a node is choosing links in order to extend a loose node of an ERO, the node MUST validate the resource classes of those links against the resource affinities." (S4.7.4) | `routing.go::resolveExplicitPath` expands loose hops using native routes, but has no resource-class or affinity inputs |
