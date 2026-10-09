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

The code and tests landed in `c9258b5fe6` (2026-09-23). On 2026-10-09
`go test -race -count=1 ./internal/component/l2tp/ppp` passed (16.9s), and
`TestPAPInitialReplyWriteFailureDoesNotAuthenticate` runs in that package: an
initial failed or short reply write aborts and leaves no decision for a later
request to reuse.

Discrimination (2026-10-09, `rfc/discrimination/rfc1334.json`): six revert
records with `pap.go::reanswerPAP` disabled, each observed red. They cover
RFC1334-2.2.1-4 positive and RFC1334-2.2.1-5 positive, and RFC1334-2.3-3 positive
and negative, all on `TestPAPReanswersAfterAuthentication`. RFC1334-2.2.1-5
negative is on `TestPAPReanswerPreservesDecision`, and RFC1334-2.2.1-4 negative
on `TestPAPReanswerDiscardsMalformedRequest`.

### Owed before closure

No interoperability scenario exercises PAP. The PPPoE lab (`02-ze-ac-pppd-client`)
and the L2TP lab authenticate with CHAP-MD5, and `check_ac.go` passes
`refuse-pap` to pppd. `ai/rules/interop-and-goal-validation.md` requires a
scenario where none matches: a Ze-AC scenario with `auth-method pap` against
pppd (`refuse-chap`), asserting PAP Ack, IPCP and teardown. That scenario
cannot lose an Ack on demand, so the reanswer MUST itself stays proven by the
discriminated unit tests above; the scenario proves the changed authenticator
interoperates.
