# Spec: RSVP message validation and state matching rules (RFC 2205 Sections 3.1, 3.2, 3.10)

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

The started implementation now validates object placement, flow descriptors, session destinations and matching path/reservation state. `DecodeMessage` invokes `message_validation.go`; `engine.go::handlePacket` checks the PATH envelope and dispatches reservation errors and tears; `reservation.go` matches descriptors against sender state before changing forwarding. The table records source coverage, not completed RFC verification. Main must exercise the malformed-message and state-preservation cases, refresh native discrimination records and complete review before closing this spec. Unsupported RSVP roles remain backlog rather than part of this completion boundary.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-1-4 | "In an explicit sender-selection reservation, each filter spec must match exactly one sender, while in a wildcard sender-selection no filter spec is needed." (§1) | `reservation.go::acceptReservation` requires matching sender state for each explicit descriptor; wildcard reservations remain outside the implemented RSVP-TE role |
| RFC2205-3-2 | "The IP source address of a Path message must be an address of the sender it describes, while the destination address must be the DestAddress for the session." (§3) | `engine.go::pathRoute` and `transport_linux.go::SendPath` retain the sender source and session destination independently of the forwarding next hop; `handlePacket` checks the received envelope |
| RFC2205-3-3 | "If the INTEGRITY object is present, it must immediately follow the common header." (§3) | `message_validation.go::checkObjectPlacement` rejects a later INTEGRITY object; this does not claim cryptographic integrity support |
| RFC2205-3-7 | "The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF given below." (§3) | `message_validation.go::checkObjectPlacement`, `appendFlowSpec` and `appendFilter` enforce descriptor placement and order |
| RFC2205-3-8 | "A FLOWSPEC object can be omitted if it is identical to the most recent such object that appeared in the list; the first FF flow descriptor must contain a FLOWSPEC." (§3) | `message_validation.go::appendFilter` requires the first FF FLOWSPEC and carries it to subsequent descriptors |
| RFC2205-3-10 | "Matching state must have match the SESSION, SENDER_TEMPLATE, and PHOP objects." (§3) | `engine.go::handlePathTear` compares the matched sender's stored RSVP_HOP before removing state |
| RFC2205-3-11 | "A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP." (§3) | `engine.go::handlePathTear` returns without forwarding or removal when the stored hop differs |
| RFC2205-3-12 | "A PathTear message must be routed exactly like the corresponding Path message." (§3) | `engine.go::pathTearLocked` and `transport_linux.go::SendPath` use the stored PATH route and protocol identities |
| RFC2205-3-16 | "Matching reservation state must match the SESSION, STYLE, and FILTER_SPEC objects as well as the LIH in the RSVP_HOP object." (§3) | `reservation.go::handleResvTear` compares the matched descriptor's style, hop and peer before removing the reservation |
| RFC2205-3-35 | "The original order of such unknown-class objects need not be retained; however, the message that is forwarded must obey the general order requirements for its message type." (§3) | Forwardable unknown objects are retained separately from the ordered flow descriptors; receive validation and the message builders must be verified together |
| RFC2205-4-1 | "This field must be non-zero." (Appendix A.1, SESSION DestAddress; stable historical ID) | `wire.go::decodeSessionIPv4` rejects an unspecified endpoint |
