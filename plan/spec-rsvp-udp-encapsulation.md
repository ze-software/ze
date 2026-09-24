# Spec: RSVP UDP encapsulation for hosts without raw network I/O (RFC 2205 Appendix C)

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

A host that cannot do raw network I/O is a role Ze does not fill today: Ze opens a raw protocol-46 socket (`internal/plugins/rsvpte/transport_linux.go::openRawTransport`) and `doctor.go` refuses to run without it, so the condition of the MUST below is false and no UDP encapsulation exists. The owner can decline the role in one word; until then the MUST below is owed.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-4-5 | "To use RSVP, such hosts must encapsulate RSVP messages in UDP." (§4) | absent: Ze does raw network I/O (`transport_linux.go::openRawTransport`, protocol 46) and the `doctor.go` check refuses to run without it |
