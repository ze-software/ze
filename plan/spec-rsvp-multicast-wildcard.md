# Spec: RSVP multicast sessions, wildcard filter style and reservation merging (RFC 2205)

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

Ze signals point-to-point IPv4 LSP tunnels with explicit FF and SE descriptors. Reservation errors, descriptor blockade state and FRR merge-point refresh now exist, but do not implement multicast sessions, Wildcard-Filter style, SCOPE processing or multicast output-interface selection. This spec retains those distinct general-RSVP obligations; it must not treat the current unicast reservation implementation as multicast support.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-1-3 | "More importantly, reservations from different downstream branches of the multicast tree(s) from the same sender (or set of senders) must be \" merged\" as reservations travel upstream." (§1) | Multicast reservation merging is absent; `reservation.go` handles explicit unicast LSP descriptors, and FRR branch merging does not establish this multicast rule |
| RFC2205-2-6 | "Conversely, state that is forwarded out interface I* must be computed using only state that arrived on interfaces different from I*." (§2) | absent: multicast/WF merge rule; Ze holds one incoming and one outgoing hop per LSP (`engine.go::handlePathTransit`) |
| RFC2205-2-9 | "The blockade state in each downstream router must not remove the state or prevent its immediate refresh." (§2) | `reservation.go::handleResvErr` now retains descriptor blockade state and reservation state for the implemented unicast path; general multicast blockade behavior remains unverified |
| RFC2205-3-5 | "Multicast routing allows a stable distribution tree in which Path messages from the same sender arrive from more than one PHOP, and RSVP must be prepared to maintain all such path state." (§3) | `engine.go::handlePathEgress`/`handlePathTransit` overwrite `PrevHop` per LSP key; one PHOP per (session, sender) |
| RFC2205-3-9 | "Whenever a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (see Section 3.4 below); in this case, the scope for forwarding the reservation is constrained to just the sender IP addresses explicitly listed in the SCOPE object." (§3) | absent: no WF style (`wire.go` ClassScope comment: "WF style only; ze signals FF and SE LSPs") |
| RFC2205-3-24 | "If reservation state from some NHOP does not contain a SCOPE object, a substitute sender list must be created and included in the union." (§3) | absent: WF/SCOPE |
| RFC2205-3-25 | "However, the ResvErr message forwarded out OI must contain a SCOPE object derived from L by including only those senders that route to OI." (§3) | Wildcard/SCOPE forwarding remains absent; the existing ResvErr handler covers explicit descriptors only |
| RFC2205-3-29 | "RSVP knows where such points occur and must so indicate to the traffic control mechanism." (§3) | General multicast merge-point notification to traffic control is absent; the FRR merge-point implementation serves a different role |
| RFC2205-3-36 | "At each such replication point, RSVP must merge reservation requests from the corresponding next hops by computing the \"maximum\" of their flowspecs." (§3) | absent: multicast |
| RFC2205-3-41 | "RSVP must be able to force a (multicast) datagram to be sent on a specific outgoing real or virtual link, bypassing the normal routing mechanism." (§3) | The Linux transport pins unicast output links with IP_PKTINFO; it does not implement multicast output-interface selection |
| RFC2205-4-3 | "The addresses must be listed in ascending numerical order." (§4, SCOPE) | absent: SCOPE never emitted or decoded (`wire.go` ClassScope) |
