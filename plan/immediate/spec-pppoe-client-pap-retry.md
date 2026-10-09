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

The code and tests landed in `d115e0a1d4` (2026-09-24). On 2026-10-09
`go test -race -count=1 ./internal/component/l2tp/pppoeclient` passed.
Discrimination (2026-10-09): four revert records on
`TestClientPAPRetriesUntilMatchingReply` with `session.go::runClientAuth`
disabled (RFC1334-2.2.1-2 and RFC1334-2.2-1, both polarities) in
`rfc/discrimination/rfc1334.json`, and fourteen revert records on the
`rfc1661_lcp_reply_test.go` tags with `session.go::negotiateLCP` disabled in
`rfc/discrimination/rfc1661.json`.

Interoperability: scenario `pppoe-pap-ze-client`
(`internal/le/interoplab/pppoe/check_client.go`, `checkZeClientPAP`) dials
accel-ppp loading `auth_pap` and no CHAP module. It runs the whole
`01-pppoe-chap-ipv4` proof (one `pppN`, address 10.11.0.2 peer 10.11.0.1,
route, accel-ppp session, ICMP, session removed after Ze stops), then requires
`recv [PAP AuthReq` and `send [PAP AuthAck` in accel-ppp's own trace with no
CHAP. Forced red, 2026-10-09: `writeClientPAPRequest` made to return `nil`
before writing, Ze image rebuilt by the lab run: FAIL "ppp0 address mismatch
... got \"\"" (no IPCP without PAP). Restored and rebuilt:
`interop: 1 passed, 0 failed`. The lab cannot drop a frame on demand, so the
retry itself stays proven by the discriminated unit tests above.

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
`TestSessionReaderStopsWithFullQueue`. These and the coverage named in the
previous paragraph are in `pppoeclient` and ran in the 2026-10-09
`go test -race` pass of that package.

## Required Reading

- [ ] `rfc/full/rfc1334.txt` Section 2.2.1 -> Constraint: "The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires."
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` -> Decision: the design document `session.go` declares; client PPP negotiation runs in `pppoeclient`.
- [ ] `docs/labs/pppoe-interop.md` -> Decision: interop proof runs against accel-ppp in the PPPoE Docker lab.

## Current Behavior

- [ ] `internal/component/l2tp/pppoeclient/session.go` -- `runClientAuth` sends the PAP request, repeats it every `papRetryInterval` up to `papRequestsMax`, and accepts only a valid reply carrying the latest Identifier.

## Data Flow

### Entry Point
`interface { pppoe-client <name> { authentication { ... } } }` dials; the AC demands PAP in LCP.

### Transformation Path
`negotiateLCP` records the auth protocol; `runClientAuth` builds the request with `buildPAPAuthRequest`, writes it with `writeClientPAPRequest`, and retries on its timer until a matching Ack or Nak.

### Boundaries Crossed
Config to the client session, then the PPPoE session socket to the AC.

### Integration Points
The client's LCP negotiation (`negotiateLCP`) chooses PAP when the AC asks for it.

## Wiring Test

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| `runClientAuth` -> PAP retry timer -> `TestClientPAPRetriesUntilMatchingReply` |
| `ze start` with a `pppoe-client` interface against a PAP-only AC -> `runClientAuth` -> Docker lab `pppoe-pap-ze-client` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Requirement |
|------|------|-------------|
| `TestClientPAPRetriesUntilMatchingReply` | `internal/component/l2tp/pppoeclient` | RFC1334-2.2.1-2 |
| `TestClientPAPDiscardsInvalidReplies` | `internal/component/l2tp/pppoeclient` | reply validation |
| `TestClientPAPRetryLimit` | `internal/component/l2tp/pppoeclient` | bounded retry |

## Files to Modify

- `internal/component/l2tp/pppoeclient/session.go` (landed in `d115e0a1d4`).
- Closure: `internal/le/interoplab/pppoe/check_client.go`, `pppoe.go`, `test/interop-pppoe/scenarios/pppoe-pap-ze-client/`, `test/interop-pppoe/pppoe_parity_test.go`, `docs/labs/pppoe-interop.md`.

## Implementation Steps

1. PAP retry and reply correlation in `runClientAuth` (done, `d115e0a1d4`).
2. Discrimination records (done, `2961683671`).
3. Closure: interop scenario `pppoe-pap-ze-client` and its forced red (2026-10-09).

## Checklist

- [ ] Tests written: the PAP and LCP client tests and the lab checker tests exist.
- [ ] Tests FAIL: unit tags observed red (`rfc/discrimination/rfc1334.json`, `rfc1661.json`); the lab observed red with the client PAP write disabled.
- [ ] Tests PASS: `go test -race` over `pppoeclient`; lab 1 passed.
- [ ] `./le verify worktree`: owed to the main thread; this closure agent may not run a whole-tree gate.

### Integration Checklist

| Item | Evidence |
|------|----------|
| The client selects PAP when the AC demands it | `negotiateLCP` records `authProto`; the lab session came up against an `auth_pap`-only accel-ppp |

## Deliverables Checklist

| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| Lost exchange retried, Identifier changed | `TestClientPAPRetriesUntilMatchingReply` | pass (race), discriminated |
| Retry bounded | `TestClientPAPRetryLimit` | pass |
| Ze's client authenticates with PAP against accel-ppp | `ZE_PPPOE_INTEROP_SCENARIO=pppoe-pap-ze-client ./le test deployment docker-pppoe-accel-test` | 1 passed; red with the PAP write disabled |

## Security Review Checklist

| Concern | Check | Result |
|---------|-------|--------|
| A silent or hostile AC causing unbounded retries | `papRequestsMax` and the overall auth timeout | bounded (`TestClientPAPRetryLimit`) |
| A stale or forged reply authenticating | Identifier must match the latest request | refused (`TestClientPAPDiscardsInvalidReplies`) |
| Password in the lab log | accel-ppp logs the user name only | no secret in the checked trace |

### Documentation Update Checklist

| Category | Update needed | Where |
|----------|---------------|-------|
| Test infrastructure | Yes, this closure | `docs/labs/pppoe-interop.md`: Running list and the `pppoe-pap-ze-client` section |
| RFC compliance | No further edit | `rfc/short/rfc1334.md` updated in `2961683671` |
| Config, CLI, API, wire format | No | no YANG, `cmd/` or wire change |

---

## Implementation Summary

### What Was Implemented
- Product code and unit tests: `d115e0a1d4`; discrimination: `2961683671`.
- Closure: interop scenario `pppoe-pap-ze-client`, pinned in `test/interop-pppoe/pppoe_parity_test.go`.

### Bugs Found/Fixed
- None in the product. Lab: accel-ppp writes session messages (LCP, PAP) to its general log only with `[log] copy=1`, which the scenario's `accel-ppp.conf` sets.

### Documentation Updates
- `docs/labs/pppoe-interop.md`.

### Deviations from Plan
- None.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RFC1334-2.2.1-2 | Done | `pppoeclient/session.go::runClientAuth` | unit + interop |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| the five rows under Implementation and verification | Done | the named tests | race pass 2026-10-09 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the PAP client tests | Done | `internal/component/l2tp/pppoeclient` | |
| `pppoe-pap-ze-client` | Done | `internal/le/interoplab/pppoe/check_client.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pppoeclient/session.go` | Done | `d115e0a1d4` |

### Audit Summary
- **Total items:** 3
- **Done:** 3
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The client recovers from a lost PAP request or reply without redialling | unit, discriminated | `TestClientPAPRetriesUntilMatchingReply`, observed red with `runClientAuth` reverted (`rfc/discrimination/rfc1334.json`) |
| The changed PAP client interoperates | interop, forced red | `pppoe-pap-ze-client` passed against accel-ppp `auth_pap`; with `writeClientPAPRequest` disabled the session got no address |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/pppoe-client-pap-retry-12d06ccf-2460-42c7-a707-30bb0a427796.md` (16 files, verdict clean) |
| `./le spec review check` | run after recording, over the same 16 files |
| Rounds | 1 |
| Reviewer lenses used | wiring of `checkZeClientPAP` into `checkers()`; evidence strength (accel-ppp's own trace, not Ze's log); bounded log read (`accelTraceLines`); style pass over the changed Go. The lab code was reviewed by the agent that wrote it: an independent pass is owed by the main thread |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE | | |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/interop-pppoe/scenarios/pppoe-pap-ze-client/accel-ppp.conf` | yes | new, pinned |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| interop | PAP client green | `pppoe-pap-ze-client`: 1 passed (2026-10-09) |
| lab unit | pins and evidence parsers | `go test -tags ze_l2tp ./internal/le/interoplab/pppoe/ ./test/interop-pppoe/`: ok |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze start` with a `pppoe-client` interface | Docker lab `pppoe-pap-ze-client` | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| none declared | n/a | the spec lists no A-N rows |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/labs/pppoe-interop.md` `pppoe-pap-ze-client` | `check_client.go::checkZeClientPAP` | yes |
