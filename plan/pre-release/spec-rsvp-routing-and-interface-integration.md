# Spec: RSVP routing and interface integration (RFC 2205 Sections 2.8, 3.8, 3.9, 3.10, 3.11)

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

The started Linux transport now queries native routes and interface addresses, receives Router Alert traffic with source, destination, interface and TTL metadata, and sends PATH/PathTear with separate protocol identities and next-hop selection. `engine.go::handlePacket` derives the previous RSVP hop from RSVP_HOP rather than the end-to-end IP source. These source changes still require Main's native forwarding scenarios. The table distinguishes implemented carriage from remaining protocol gaps; it does not authorize implementing every listed RFC requirement in this completion phase.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-2-11 | "RSVP must therefore provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary \"cloud\" of non-RSVP routers." (§2) | `engine.go::handlePacket` uses RSVP_HOP for the previous signaling hop; arbitrary non-RSVP-cloud operation remains unverified and non-RSVP-hop detection remains backlog |
| RFC2205-2-12 | "If the destination address does not match any local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node." (§2) | Remaining gap: `engine.go::handlePacket` has no general non-local-message forwarding branch |
| RFC2205-3-6 | "RSVP must not forward (according to the rules of Section 3.9) Path messages that arrive on an incoming interface different from that provided by routing." (§3) | Remaining gap: `Packet.IfIndex` is available, but the engine does not compare it with the expected incoming route |
| RFC2205-3-21 | "However, this must not trigger sending a message out the interface through which M arrived (which could happen if the implementation simply triggered an immediate refresh of all state for the session)." (§3) | Remaining gap: the general session/interface loop-prevention rule is not established by the unicast next-hop path |
| RFC2205-3-23 | "Forwarding of RSVP messages must avoid looping." (§3) | Transit PATH processing bounds Send_TTL; general interface-based loop prevention remains a separate gap |
| RFC2205-3-33 | "The RSVP process must determine which case holds by examining the path state, to decide which incoming interface to use for sending Resv messages." (§3) | `engine.go::sendResv` selects the previous hop from path state; `transport_linux.go::Send` queries its native output route. General multicast/interface selection is not claimed |
| RFC2205-3-37 | "To forward Path and PathTear messages, an RSVP process must be able to query the routing process(s) for routes." (§3) | `routing.go::resolveExplicitPath` and `resolveImplicitPath` call `Transport.ResolveRoute`; `transport_linux.go::ResolveRoute` queries the native FIB |
| RFC2205-3-38 | "RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses." (§3) | `transport_linux.go::LocalAddresses` reads native addresses, link state and MTU; the engine refreshes this view before signaling |
| RFC2205-3-39 | "Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded." (§3) | `transport_linux.go::openSockets` registers a wildcard protocol-46 socket with IP_ROUTER_ALERT; native transit diversion must be exercised |
| RFC2205-3-40 | "On a router or multi-homed host, the identity of the interface (real or virtual) on which a diverted message is received, as well as the IP source address and IP TTL with which it arrived, must also be available to the RSVP process." (§3) | `transport_linux.go::receivedIPv4Packet` decodes source, destination, TTL and IP_PKTINFO into `Packet`; missing interface metadata is rejected |
| RFC2205-3-42 | "RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages." (§3) | `transport_linux.go::SendPath` writes source and Send_TTL into the IPv4 header and pins the selected output interface |
