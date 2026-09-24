# Spec: pppoe-client-pap-retry

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/1 |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The PPPoE client must recover from a lost PAP request or reply without redialling.
RFC 1334 Section 2.2.1 states, "The Authenticate-Request packet MUST be repeated
until a valid reply packet is received, or an optional retry counter expires."
It also states, "The Identifier field MUST be changed each time an
Authenticate-Request packet is issued."

`internal/component/l2tp/pppoeclient/session.go::runClientAuth` now repeats the
request every three seconds, bounded to five requests and the existing overall
authentication timeout. Only a structurally valid reply carrying the latest
request's Identifier completes PAP. Initial and retry writes both abort on an
error or a short write.

The row carries a `{superseded: dropped}` marker in `rfc/short/rfc1334.md` because
RFC 1994 obsoletes RFC 1334 without restating PAP; PAP stays implemented, so the
obligation stays owed and this spec is where it is scheduled.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC1334-2.2.1-2 | "The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires." (Section 2.2.1) | `pppoeclient/session.go::runClientAuth` retries on its PAP timer until a matching reply or the request limit |

## Implementation and verification

| Acceptance criterion | Regression test |
|----------------------|-----------------|
| A lost exchange causes another request on the same transport; a matching Ack stops retries | `TestClientPAPRetriesUntilMatchingReply` |
| Retries change the Identifier and retain credentials | `TestClientPAPRetriesUntilMatchingReply` |
| Malformed, unmatched and stale replies do not authenticate | `TestClientPAPDiscardsInvalidReplies` |
| A silent peer cannot cause unbounded retries | `TestClientPAPRetryLimit` |
| Initial and retry write errors and short writes abort | `TestClientPAPWriteFailure`, `TestClientPAPRetryWriteFailure` |

The regression tests are written but unrun. The 2026-09-21 source batch explicitly
defers all validation, builds, formatting, lint, tests and discrimination to the
integrating parent. No owner approval, passing result or closure is recorded.
The spec remains in progress until those proofs run.

The parent also assigned client-side LCP reply correlation to this file.
`negotiateLCP` retains the sent request, calls `ppp.ValidateLCPReply`, preserves an
unanswered request's Identifier on timeout, changes it after a valid response and
removes rejected options. `lcp_reply_test.go` covers those paths; the existing
option-refusal fixture now acknowledges the actual sent options.

Owed package command:
`./le job run label ppp-pap-client quiet command go test -race ./internal/component/l2tp/pppoeclient`.
RFC discrimination is owed for the new units covering RFC1334-2.2.1-2,
RFC1334-2.2-1 and the LCP tags in `lcp_reply_test.go`.

The client now adopts MRU Naks between `ppp.MinFrameLen` and the configured MTU.
It redraws a fresh nonzero Magic-Number on a Magic Nak, aborts on entropy failure
and passes the final value to later phases. Independent rejection flags prevent
later suggestions from restoring a rejected option. Additional unrun coverage:
`TestClientLCPMRUNakBounds`, `TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue`
(RFC1661-6.4-7) and `TestClientPAPStopsOnLCPTermination`.

The integration pass also corrects the Ack-Rcvd duplicate-Ack/Nak/Reject
transitions and cancels prior peer acceptance when a later proposal is refused.
Echo requests remain silent until LCP has opened. Reader delivery is cancellable
for both frames and errors when its four-frame queue fills. Additional unrun
tests are `TestClientLCPRepliesInAckReceived`, `TestClientLCPRefusalRevokesPeerAck`,
`TestClientLCPEchoBeforeOpenDiscarded` (RFC1661-5.8-2) and
`TestSessionReaderStopsWithFullQueue`.
