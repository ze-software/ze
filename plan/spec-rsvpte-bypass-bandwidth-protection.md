# Spec: RSVP-TE bypass with guaranteed bandwidth (RFC 4090 Sections 4.4, 6)

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

A bypass tunnel that guarantees the protected LSP's bandwidth is a feature Ze does not offer today: `internal/plugins/rsvpte/register.go::setupBypass` reserves no bandwidth of its own, so the "bandwidth protection" RRO flag can never be set and the MUST below has no positive input. RFC 4090 Section 6 says "if this is not feasible, the PLR SHOULD then try to provide a backup without a guarantee of the full bandwidth", which is what Ze does. The owner can decline the feature in one word; until then the MUST below is owed with the bandwidth-reserving bypass.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC4090-4.4-5 | "the PLR MUST set this flag when the desired bandwidth is guaranteed and the 'bandwidth protection desired' flag was set in the SESSION_ATTRIBUTE object." (S4.4) | register.go::setupBypass ("reserves no bandwidth of its own") |
