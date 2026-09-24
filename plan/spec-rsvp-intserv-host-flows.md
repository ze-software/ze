# Spec: RSVP IntServ flow reservations for host applications (RFC 2205)

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

Per-flow IntServ reservations (UDP/TCP port classification, the IPv4/GPI SESSION with Protocol Id and ports, link-layer QoS negotiation, the service-dependent flowspec routines of RFC 2210, the host application API and Local_Only path state) are a feature Ze does not offer today: Ze's session is LSP_TUNNEL_IPv4 (`internal/plugins/rsvpte/wire.go::decodeSessionIPv4`, C-Type 7) with no ports, data is classified by MPLS label (`fib.go`), and the sender is a configured tunnel (`register.go`). The owner can decline the feature in one word; until then every MUST below is owed.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-1-1 | "Because the UDP/TCP port numbers are used for packet classification, each router must be able to examine these fields." (§1) | absent: no classifier. Ze's session is LSP_TUNNEL_IPv4 (`wire.go::decodeSessionIPv4`, C-Type 7) with no ports; data is classified by MPLS label (`fib.go`) |
| RFC2205-1-2 | "If the link-layer technology implements its own QoS management capability, then RSVP must negotiate with the link layer to obtain the requested QoS." (§1) | absent: `admission.go::reserveSession` is a bandwidth counter, no link-layer QoS negotiation |
| RFC2205-3-4 | "Path messages must satisfy the rules on SrcPort and DstPort in Section 3.2." (§3) | absent: LSP_TUNNEL_IPv4 session and sender template carry no ports (`wire.go::decodeSessionIPv4`, `decodeSenderTemplate`) |
| RFC2205-3-32 | "The RSVP process must be aware of the default, and if an application sets a specific interface, it must also pass that information to RSVP." (§3) | host application API; Ze's sender is the configured tunnel (`register.go`) |
| RFC2205-3-34 | "When Path and PathTear messages are forwarded, path state marked \"Local_Only\" must be ignored." (§3) | absent: no local application path state |
| RFC2205-3-43 | "In order to manipulate these objects, RSVP process must have available to it the following service-dependent routines." (§3) | absent: no IntServ service modules; `wire.go::FlowSpec` is the RFC 2210 token bucket read as a bandwidth number |
| RFC2205-4-2 | "This field must be non-zero." (§4, SESSION Protocol Id) | absent: LSP_TUNNEL_IPv4 SESSION (RFC 3209 4.6.1.1) has no Protocol Id field (`wire.go::encodeSessionIPv4` writes reserved zero at offsets 8-9) |
