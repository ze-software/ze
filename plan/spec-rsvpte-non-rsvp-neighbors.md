# Spec: RSVP-TE knowledge of non-RSVP neighbors and hops (RFC 3209 Section 4.2.5, RFC 2205 Section 3.8)

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

The transport now supplies the received IP TTL and queries native routes, but that does not identify RSVP capability along the route. This backlog covers comparing IP TTL with Send_TTL, reporting non-RSVP hops to traffic control and suppressing LABEL_REQUEST across a known non-RSVP router. `internal/plugins/rsvpte/build.go::buildPath` still emits LABEL_REQUEST for the implemented RSVP-TE role; general non-RSVP-cloud detection is not claimed.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.2.5-1 | "This means that if a router has a neighbor that is known to not be RSVP capable, the router MUST NOT advertise the LABEL_REQUEST object when sending messages that pass through the non-RSVP routers." (S4.2.5) | absent: Ze holds no RSVP-capability knowledge of a neighbor; `build.go::buildPath` appends LABEL_REQUEST unconditionally |
| RFC2205-3-30 | "RSVP must also test for the presence of non-RSVP hops in the path and pass this information to traffic control." (§3) | The transport supplies `Packet.TTL`, but the engine does not compare it with Send_TTL or report non-RSVP hops to traffic control |
| RFC2205-3-31 | "For example, if the routing protocol uses IP encapsulating tunnels, then the routing protocol must inform RSVP when non-RSVP hops are included." (§3) | Native route lookup supplies next-hop/interface information, not RSVP-capability information about the route |
