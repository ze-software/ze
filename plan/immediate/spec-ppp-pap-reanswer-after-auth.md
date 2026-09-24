# Spec: ppp-pap-reanswer-after-auth

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

A peer must recover when its first PAP reply is lost. RFC 1334 Section 2.2.1
states, "Because the Authenticate-Ack might be lost, the authenticator MUST allow
repeated Authenticate-Request packets after completing the Authentication phase."

`internal/component/l2tp/ppp/pap.go::runPAPAuthPhase` caches the reply Code after
a successful write. `reanswerPAP` validates a later request without copying its
credentials and returns that Code with the incoming Identifier. The session
dispatcher handles these requests during NCP negotiation as well as the
established network phase. LCP teardown or restart clears the decision. Both
the L2TP LNS and PPPoE AC use this shared authenticator.

The rows carry `{superseded: dropped}` markers in `rfc/short/rfc1334.md` because
RFC 1994 obsoletes RFC 1334 without restating PAP; PAP stays implemented, so the
obligation stays owed and this spec is where it is scheduled.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC1334-2.2.1-4 | "Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate-Request packets after completing the Authentication phase." (Section 2.2.1) | `pap.go::runPAPAuthPhase` records the decision; `session_run.go::handleFrame` routes repeats to `pap.go::reanswerPAP` |
| RFC1334-2.2.1-5 | "Protocol phase MUST return the same reply Code returned when the Authentication phase completed (the message portion MAY be different)." (Section 2.2.1, wording present in the published text) | `pap.go::reanswerPAP` writes the cached Code with the incoming Identifier |

## Implementation and verification

| Acceptance criterion | Regression test |
|----------------------|-----------------|
| Repeated requests receive a reply during NCP negotiation and after Opened | `TestPAPReanswersAfterAuthentication` |
| The original Ack or Nak decision survives changed credentials and Identifier | `TestPAPReanswerPreservesDecision` |
| Malformed PAP packets and non-request Codes receive no reply | `TestPAPReanswerDiscardsMalformedRequest` |
| Failed or short reply writes terminate the session | `TestPAPReanswerWriteFailure` |
| Requests outside authentication and the post-authentication network phase remain silent | `TestPAPRequestOutsideAuthPhaseIsSilentlyDiscarded` |

The tests are written but unrun. The parent owns the session field, dispatcher,
cache reset and common short-write handling. The 2026-09-21 source batch explicitly
defers all validation, builds, formatting, lint, tests and discrimination to
integration. No owner approval, passing result or closure is recorded.

Owed package command:
`./le job run label ppp-pap-server quiet command go test -race ./internal/component/l2tp/ppp`.
RFC discrimination is owed for the new units covering RFC1334-2.2.1-4,
RFC1334-2.2.1-5 and RFC1334-2.3-3. The Identifier-copying proof is attached to
`TestPAPReanswersAfterAuthentication`, which checks the authenticator's emitted
Identifiers for the initial request and two repeats.

`TestPAPInitialReplyWriteFailureDoesNotAuthenticate` additionally checks that an
initial failed or short reply write aborts and leaves no decision for a later
request to reuse. This test is also unrun.
