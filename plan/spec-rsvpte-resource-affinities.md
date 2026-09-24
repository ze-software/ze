# Spec: RSVP-TE resource affinities (RFC 3209 Section 4.7.4)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
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

**Backlog boundary:** This skeleton records an unstarted capability or conditional RFC obligation. The owner's authorization to finish already-started work does not select this feature. Its requirements are not completed or waived.

Resource affinities (SESSION_ATTRIBUTE C-Type 1 Exclude-any, Include-any and Include-all masks, per-link resource classes and the three link validation tests of Section 4.7.4) are a feature Ze does not offer today: `internal/plugins/rsvpte/frr.go::decodeSessionAttr` skips the three masks, Ze emits C-Type 7, and no interface carries a resource class. The owner can decline the feature in one word; until then every MUST below is owed.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.7.4-1 | "In order to be validated a link MUST pass the three tests below." (S4.7.4) | absent: `frr.go::decodeSessionAttr` skips the Exclude-any / Include-any / Include-all masks of C-Type 1 and Ze emits C-Type 7 (no affinities); no link validation exists |
| RFC3209-4.7.4-2 | "When a node is choosing links in order to extend a loose node of an ERO, the node MUST validate the resource classes of those links against the resource affinities." (S4.7.4) | `routing.go::resolveExplicitPath` expands loose hops using native routes, but has no resource-class or affinity inputs |
