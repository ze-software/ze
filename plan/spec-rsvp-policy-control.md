# Spec: RSVP policy control (RFC 2205 Section 2.2)

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

The current RSVP-TE implementation applies local sender/endpoint policy before signaling and reservation admission. That address policy is not a POLICY_DATA authorization module and does not establish the identity-based policy-control role described here. This backlog concerns that additional role, not removal or replacement of the existing local admission checks.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-2-10 | "When a new reservation is requested, each node must answer two questions: \"Are enough resources available to meet this request?\" and \"Is this user allowed to make this reservation?\" These two decisions are termed the \"admission control\" decision and the \"policy control\" decision, respectively, and both must be favorable in order for RSVP to make a reservation." (§2) | `reservation.go::acceptReservation` checks local sender/endpoint policy and bandwidth admission before installation; identity-based POLICY_DATA authorization is not implemented |
