# Spec: ike-responder-eap-preserves-state-on-unauthenticated-input

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Keep an IKE responder's EAP handshake alive when an inbound IKE_AUTH datagram
fails cryptographic validation. This spec owns the remaining obligation recorded
on 2026-08-14 in `plan/journal/unprotected-message-changes-sa-state.md`. The
cached-message-ID repair described there remains separate. This ownership was
assigned during planning reconciliation on 2026-09-19; no implementation or
runtime reproduction is recorded here.

RFC 7296 Section 2.4 (`rfc/full/rfc7296.txt`) states that an endpoint MUST NOT
conclude that its peer failed from IKE messages that arrive without cryptographic
protection. The same section requires implementations to limit the rate of
actions based on unprotected messages, recorded as `RFC7296-2.4-12` in
`rfc/short/rfc7296.md`. Both obligations apply to this repair.

Source inspection found that `handleResponderInbound`
(`internal/component/ike/engine/responder.go`) routes an IKE_AUTH request in
`StateEAPInProgress` through the cached-response guard and then to
`handleResponderEAP` (`responder_eap.go` in the same package). That handler sets
`sa.State = StateDead` on every error from `decryptAndParse`.
`decryptAndParse` (`inbound.go`) returns errors for a missing SK payload and for
decryption failure. It separately marks an inner-payload parse error after a
successful integrity check with `errInnerParse`. A repair must retain that
distinction rather than discard every error through one branch.

The design must preserve the SA and its valid EAP progress when a datagram lacks
valid cryptographic protection. It must retain the protocol-required handling of
authenticated malformed payloads and authentication failures. Valid EAP and final
AUTH exchanges must still complete, and authenticated rejection must not become
acceptance. Cached retransmissions must retain their existing replay behavior.
Normal handshake timeouts and retransmission limits must remain effective;
unprotected input must not keep a handshake alive indefinitely. Actions taken in
response to such input must satisfy the existing Section 2.4 rate-limit obligation.

Before implementation, complete the design and acceptance criteria using
`plan/TEMPLATE.md`. Read `docs/architecture/ike/ipsec-14-responder.md` and trace
the daemon's wire dispatch through these producers. Require discriminating
proof through that daemon path: inject unprotected input during an EAP exchange,
then show that the legitimate peer can complete it. The proof must fail under
the recorded StateDead transition and must also distinguish an authenticated
error from an unauthenticated one. Require interop evidence with an independent
EAP peer, including a valid control exchange, and evidence for bounded actions
under repeated unprotected input. A direct helper test or a self-consistent
exchange between two Ze endpoints cannot replace those proofs.

This spec does not own remote-access admission, virtual-address assignment, or
a broader EAP feature expansion. Update the responder architecture description
and assess the existing RFC evidence when the behavior changes. This skeleton
changes no RFC ledger, tag, support claim, or publication status.
