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

### Interoperability

Scenario `pppoe-pap-ze-ac` (`internal/le/interoplab/pppoe/check_pap.go`,
`checkZeAccessConcentratorPAP`) runs Ze's AC with `auth-method pap` against
pppd 2.5.1 dialing with `refuse-chap`. It requires `<auth pap>` in Ze's LCP
Configure-Request, the client's named Authenticate-Request and Ze's Ack in
pppd's trace with no CHAP, IPCP, ICMP, exactly one Request and one Ack with the
same Identifier on the captured wire, and PADT teardown. It then resends the
client's own captured Authenticate-Request with `tcpreplay` from inside the
client container after IPCP opened, and requires exactly one Ack carrying the
request's Identifier with the session still up. RFC1334-2.2.1-4 and -5 are
therefore exercised against a real peer as well as in the unit tests.

Forced red, 2026-10-09: `reanswerPAP` made to return `true` before its body
(silent discard), Ze image rebuilt by the lab run: FAIL "RFC 1334 Section
2.2.1: Ze answered the repeated Authenticate-Request with 0 Acks and 0 Naks,
want exactly one Ack". Restored and rebuilt: `interop: 1 passed, 0 failed`
(one intervening rerun failed environmentally on `docker run -d` deadline).

## Required Reading

- [ ] `rfc/full/rfc1334.txt` Section 2.2.1 -> Constraint: "Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate-Request packets after completing the Authentication phase."
- [ ] `docs/labs/pppoe-interop.md` -> Decision: interop proof runs in the PPPoE Docker lab against pppd 2.5.1.
- [ ] `docs/research/l2tpv2-ze-integration.md` -> Constraint: the design document `pap.go` declares; the shared PPP driver serves L2TP and PPPoE.

## Current Behavior

- [ ] `internal/component/l2tp/ppp/pap.go` -- `runPAPAuthPhase` caches the reply Code; `reanswerPAP` answers a later request with it.
- [ ] `internal/component/l2tp/ppp/session_run.go` -- `handleFrame` routes PAP frames after authentication to `reanswerPAP`.

## Data Flow

### Entry Point
A PAP Authenticate-Request reaches `pppSession.handleFrame` after the authentication phase completed.

### Transformation Path
`handleFrame` hands the payload to `reanswerPAP`, which validates it and writes the cached Code with the incoming Identifier.

### Boundaries Crossed
PPPoE (or L2TP) session socket to the shared PPP driver and back to the wire. No plugin or config boundary.

### Integration Points
The shared authenticator serves the L2TP LNS and the PPPoE AC; `auth-method pap` selects it.

## Wiring Test

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| `pppSession.handleFrame` -> `reanswerPAP` -> `TestPAPReanswersAfterAuthentication` |
| `ze start` with `pppoe { auth-method pap; }` -> `reanswerPAP` -> Docker lab `pppoe-pap-ze-ac` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Requirement |
|------|------|-------------|
| `TestPAPReanswersAfterAuthentication` | `internal/component/l2tp/ppp` | RFC1334-2.2.1-4, -5 |
| `TestPAPReanswerPreservesDecision` | `internal/component/l2tp/ppp` | RFC1334-2.2.1-5 |
| `TestPAPReanswerDiscardsMalformedRequest` | `internal/component/l2tp/ppp` | RFC1334-2.2.1-4 negative |

## Files to Modify

- `internal/component/l2tp/ppp/pap.go`, `ppp/session_run.go` (landed in `c9258b5fe6`).
- Closure: `internal/le/interoplab/pppoe/check_pap.go`, `checkers_l2tp.go`, `check_ac.go`, `test/interop-pppoe/scenarios/pppoe-pap-ze-ac/`, `test/interop-pppoe/Dockerfile.client`, `test/interop-pppoe/pppoe_parity_test.go`, `docs/labs/pppoe-interop.md`.

## Implementation Steps

1. Cache the decision and re-answer repeats (done, `c9258b5fe6`).
2. Discrimination records (done, `2961683671`).
3. Closure: interop scenario `pppoe-pap-ze-ac` and its forced red (2026-10-09).

## Checklist

- [ ] Tests written: the five PAP tests and the lab checker tests exist.
- [ ] Tests FAIL: unit tags observed red (`rfc/discrimination/rfc1334.json`); the lab observed red with `reanswerPAP` disabled.
- [ ] Tests PASS: `go test -race` over `ppp`; lab 1 passed.
- [ ] `./le verify worktree`: owed to the main thread; this closure agent may not run a whole-tree gate.

### Integration Checklist

| Item | Evidence |
|------|----------|
| Both PPPoE AC and L2TP LNS use the shared authenticator | `pap.go::runPAPAuthPhase` is the one PAP path in `ppp` |

## Deliverables Checklist

| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| Repeated requests answered after authentication | `TestPAPReanswersAfterAuthentication` | pass (race), discriminated |
| Decision preserved across changed credentials | `TestPAPReanswerPreservesDecision` | pass, discriminated |
| PAP interoperates and re-answers on the wire | `ZE_PPPOE_INTEROP_SCENARIO=pppoe-pap-ze-ac ./le test deployment docker-pppoe-accel-test` | 1 passed; red with `reanswerPAP` disabled |

## Security Review Checklist

| Concern | Check | Result |
|---------|-------|--------|
| A repeated request re-authenticating with new credentials | `reanswerPAP` returns the cached Code and never copies the credentials | closed (`TestPAPReanswerPreservesDecision`) |
| Malformed repeats provoking replies | `reanswerPAP` validates the packet before answering | silent (`TestPAPReanswerDiscardsMalformedRequest`) |
| Lab checker parsing captured frames | `observePAPFrames` checks frame length before every index | no overread (`TestObservePAPFramesRefusesCorruptFrames`) |

### Documentation Update Checklist

| Category | Update needed | Where |
|----------|---------------|-------|
| Test infrastructure | Yes, this closure | `docs/labs/pppoe-interop.md`: Running list and the `pppoe-pap-ze-ac` section |
| RFC compliance | No further edit | `rfc/short/rfc1334.md` updated in `2961683671` |
| Config, CLI, API, wire format | No | no YANG, `cmd/` or wire change; `auth-method pap` already existed |

---

## Implementation Summary

### What Was Implemented
- Product code and unit tests: `c9258b5fe6`; discrimination: `2961683671`.
- Closure: interop scenario `pppoe-pap-ze-ac` with an in-container replay, pinned in `test/interop-pppoe/pppoe_parity_test.go`.

### Bugs Found/Fixed
- None in the product. Lab: replay through `interoplab.SendFrameInNamespace` needs host privilege to enter the container namespace (permission denied on this host), so the PAP replay runs `tcpreplay` inside the client container (`Dockerfile.client` gains `tcpreplay`).

### Documentation Updates
- `docs/labs/pppoe-interop.md`.

### Deviations from Plan
- The spec expected the scenario could not lose an Ack on demand. It resends the request instead, which observes the re-answer on the wire.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RFC1334-2.2.1-4 | Done | `ppp/pap.go::runPAPAuthPhase`, `session_run.go::handleFrame`, `pap.go::reanswerPAP` | unit + interop |
| RFC1334-2.2.1-5 | Done | `ppp/pap.go::reanswerPAP` | unit + interop |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| the five rows under Implementation and verification | Done | the five named tests | race pass 2026-10-09 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the five PAP tests | Done | `internal/component/l2tp/ppp` | |
| `pppoe-pap-ze-ac` | Done | `internal/le/interoplab/pppoe/check_pap.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `ppp/pap.go`, `ppp/session_run.go` | Done | `c9258b5fe6` |

### Audit Summary
- **Total items:** 4
- **Done:** 4
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer recovers when its first PAP reply is lost: Ze answers a repeated Authenticate-Request after authentication | interop, forced red | `pppoe-pap-ze-ac` passed against pppd 2.5.1; with `reanswerPAP` disabled it failed with 0 Acks to the resent request |
| The same reply Code is returned | unit, discriminated | `TestPAPReanswerPreservesDecision`, `rfc/discrimination/rfc1334.json` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ppp-pap-reanswer-after-auth-12d06ccf-2460-42c7-a707-30bb0a427796.md` (16 files, verdict clean) |
| `./le spec review check` | run after recording, over the same 16 files |
| Rounds | 1 |
| Reviewer lenses used | wiring of the new checkers into `checkers()` and `wireCheckers`; removed-behavior audit of the `pppdDial` and `checkLCPAuthIPCP` split (CHAP argv and checks still pinned by `TestZeAccessConcentratorScenarioExercisesEveryStage`); frame-parse bounds; style pass over the changed Go (no `panic`, loops bounded by a finite capture). The lab code was reviewed by the agent that wrote it: an independent pass is owed by the main thread |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE. NOTE: `replayFrameInClient` has no unit test; the lab run and its forced red exercise it | | |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/interoplab/pppoe/check_pap.go` | yes | new |
| `test/interop-pppoe/scenarios/pppoe-pap-ze-ac/ze.conf` | yes | new, pinned |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| interop | PAP AC green | `pppoe-pap-ze-ac`: 1 passed (2026-10-09) |
| lab unit | pins and parsers | `go test -tags ze_l2tp ./internal/le/interoplab/pppoe/ ./test/interop-pppoe/`: ok |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze start` with `pppoe { auth-method pap; }` | Docker lab `pppoe-pap-ze-ac` | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| none declared | n/a | the spec lists no A-N rows |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/labs/pppoe-interop.md` `pppoe-pap-ze-ac` | `check_pap.go::checkZeAccessConcentratorPAP` | yes |
