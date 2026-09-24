# Spec: RSVP-TE per-sender policing (RFC 3209 Section 4.1.1.1)

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

Policing individual senders to a session is a feature Ze does not offer today: `internal/plugins/rsvpte/admission.go` reserves bandwidth per interface and no per-sender policer exists, so the conditional MUST (unique labels for policed senders) has a false antecedent. `fsm.go::AllocateLabel` hands out a distinct label per LSP anyway. The owner can decline the feature in one word; until then the MUST below is owed with its policer.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.1.1.1-2 | "Note that if a node intends to police individual senders to a session, it MUST assign unique labels to those senders." (S4.1.1.1) | Ze polices no sender: `admission.go` reserves bandwidth per interface, no per-sender policer exists. `fsm.go::AllocateLabel` hands out a distinct label per LSP anyway |
