# Spec: RSVP refresh timing: immediate forward on change, jitter, lifetime and slew (RFC 2205 Sections 2.1, 3.7)

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

The started implementation now forwards changed transit PATH state on receipt, sends a changed route-recording reservation immediately and refreshes locally repaired and merged PATH state. That does not close all timing obligations: `runRefreshLoop` still uses a fixed ticker, stored-state expiry uses the configured multiplier and `adoptedRefreshPeriod` adopts a changed period directly. Jitter, the RFC lifetime floor and period slew remain backlog. Main must validate the implemented immediate-forwarding paths without claiming these separate gaps are complete.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-2-5 | "If this update results in modification of state to be forwarded in refresh messages, these refresh messages must be generated and forwarded immediately, so that state changes can be propagated end-to-end without delay." (§2) | `engine.go::handlePathTransit` rebuilds and sends received PATH state immediately, including repaired PATH and route-recording withdrawal; `reservation.go::acceptReservation` propagates accepted reservations. Current behavioral verification remains required |
| RFC2205-3-26 | "Since RSVP sends periodic refresh messages, it must avoid message synchronization and ensure that any synchronization that may occur is not stable." (§3) | Remaining gap: `register.go::runRefreshLoop` uses a fixed `time.Ticker`; row 3.7-1's SHOULD jitter is also open |
| RFC2205-3-27 | "To avoid premature loss of state, L must satisfy L >= (K + 0.5)*1.5*R, where K is a small integer." (§3) | `register.go::cleanupTick` uses K*R; `plan/journal/bound-too-small-for-its-own-burst.md` row exists |
| RFC2205-3-28 | "Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max." (§3) | `register.go::adoptedRefreshPeriod` adopts a committed period in one step; `plan/journal/setting-changed-faster-than-its-consumer-allows.md` |
