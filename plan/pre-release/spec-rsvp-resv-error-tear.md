# Spec: RSVP ResvErr, ResvTear and per-descriptor error reporting (RFC 2205)

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

The started implementation now builds and dispatches ResvErr and ResvTear, processes explicit flow descriptors independently and preserves an existing reservation when an attempted increase fails. The implementation is in `internal/plugins/rsvpte/reservation.go`, with native label/forwarding removal preceding reservation-state removal. Main must verify receiver-directed errors, mismatched tears, failed increases and forwarding cleanup before closure. General multicast and wildcard reservation support remain backlog.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-2-8 | "Since a request that fails may be the result of merging a number of requests, a reservation error must be reported to all of the responsible receivers." (§2) | `reservation.go::handleResvErr` handles the implemented explicit-descriptor path; general multicast merge/fan-out is not claimed |
| RFC2205-3-13 | "A PathTear message may include a SENDER_TSPEC or ADSPEC object in its sender descriptor, but these must be ignored." (§3) | `engine.go::handlePathTear` removes matched state without using those optional objects; the quotation is a PathTear rule, not a ResvTear requirement |
| RFC2205-3-17 | "A ResvTear message must be routed like the corresponding Resv message, and its IP destination address will be the unicast address of a previous hop." (§3) | `reservation.go::handleResvTear` and `removeReservation` match downstream reservation state and relay toward the stored previous hop |
| RFC2205-3-18 | "Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error." (§3) | `reservation.go::acceptReservation` admits each descriptor independently; failed descriptors produce separate errors without removing accepted siblings |
| RFC2205-3-19 | "This ResvErr message must contain the information required to define the error and to route the error message in later hops." (§3) | The reservation error builder retains the session, error, style and failing descriptor for downstream processing |
| RFC2205-3-20 | "If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message." (§3) | `reservation.go::acceptReservation` retains existing reservation/forwarding state on failed admission or installation and reports InPlace when an old reservation exists |
