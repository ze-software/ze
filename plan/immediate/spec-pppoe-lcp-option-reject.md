# Spec: pppoe-lcp-option-reject

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

The initial defect was that `ppp/lcp_options.go::negotiatePeerOption`
acknowledged well-formed ACCM and ACFC on PPPoE, contrary to RFC 2516
Section 7. `LCPNegPolicy.PPPoE` now rejects ACCM (Type 2), ACFC (Type 8)
and FCS Alternatives (Type 9). Both the AC and client set this policy.
Neither producer requests these options. L2TP leaves the policy unset and
retains its existing negotiation. PFC is NOT RECOMMENDED rather than
prohibited by Section 7, so this change preserves its existing behaviour.

Owner requirement, 2026-09-21: "Both PPPoE roles reject mandatory forbidden
options, unrelated L2TP negotiation preserved."

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2516-7-2 | "An implementation MUST NOT request any of the following options, and MUST reject a request for such an option:" FCS-Alternatives, ACFC, ACCM (Section 7) | `lcp_options.go::negotiatePeerOption`, `session_run.go::negPolicy`, `pppoeclient/session.go::clientLCPPolicy` |

## Regressions and verification

`TestRFC2516ACRejectsForbiddenLCPOptions` exercises the PPP session frame
handler, checks each Configure-Reject and the local Configure-Request,
then checks permitted PPPoE options and the unchanged L2TP path.
`TestRFC2516ClientRejectsForbiddenLCPOptions` checks the corresponding
client wire replies and emitted request.

Both tests landed in `82b495d9c3` (AC) and `d115e0a1d4` (client). They pass
under `go test -race` (2026-10-09). Each of the four RFC2516-7-2 tags carries a
revert discrimination record in `rfc/discrimination/rfc2516.json`.

## Required Reading

- [ ] `rfc/full/rfc2516.txt` Section 7 -> Constraint: "An implementation MUST NOT request any of the following options, and MUST reject a request for such an option:" FCS Alternatives, ACFC, ACCM. PFC is only NOT RECOMMENDED.
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` -> Decision: the AC sets `StartSession.PPPoE`; L2TP leaves it unset.
- [ ] `docs/research/l2tpv2-implementation-guide.md` -> Constraint: the design document the PPP sources declare; the shared LCP negotiator serves L2TP and PPPoE.

## Current Behavior

- [ ] `internal/component/l2tp/ppp/lcp_options.go` -- `negotiatePeerOption` rejects Types 2, 8, 9 when `LCPNegPolicy.PPPoE` is set.
- [ ] `internal/component/l2tp/ppp/session_run.go` -- `negPolicy` copies `s.pppoe` into the policy.
- [ ] `internal/component/l2tp/pppoe/server.go` -- the AC starts PPP with `PPPoE: true`.
- [ ] `internal/component/l2tp/pppoeclient/session.go` -- `clientLCPPolicy` sets `PPPoE: true`.

## Data Flow

### Entry Point
A peer LCP Configure-Request reaches `pppSession.handleFrame` (AC) or the client's `negotiateLCP` loop.

### Transformation Path
The option walker yields each option; `negotiatePeerOption` returns `negReject` with the option's own data for Types 2, 8, 9 under the PPPoE policy; the reply builder emits Configure-Reject.

### Boundaries Crossed
PPPoE session socket to the PPP negotiator and back to the wire. No plugin or config boundary.

### Integration Points
`LCPNegPolicy` (shared by L2TP and PPPoE); `StartSession.PPPoE`.

## Wiring Test

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| `pppSession.handleFrame` -> `negotiatePeerOption` PPPoE branch -> `TestRFC2516ACRejectsForbiddenLCPOptions` |
| client `negotiateLCP` loop -> `clientLCPPolicy` -> `TestRFC2516ClientRejectsForbiddenLCPOptions` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Requirement |
|------|------|-------------|
| `TestRFC2516ACRejectsForbiddenLCPOptions` | `internal/component/l2tp/ppp/rfc2516_options_test.go` | RFC2516-7-2 positive and negative |
| `TestRFC2516ClientRejectsForbiddenLCPOptions` | `internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go` | RFC2516-7-2 positive and negative |

## Files to Modify

- `internal/component/l2tp/ppp/lcp_options.go`, `ppp/session_run.go`, `ppp/start_session.go`, `pppoe/server.go` (landed in `c9258b5fe6`), `pppoeclient/session.go` (`clientLCPPolicy`, landed in `d115e0a1d4`).

## Implementation Steps

1. Add `LCPNegPolicy.PPPoE` and the reject branch (done, `c9258b5fe6`).
2. Set the policy in both PPPoE roles (done).
3. Closure: race tests, discrimination records, stale prose (2026-10-09).

## Checklist

- [ ] Tests written: both RFC2516 tests exist.
- [ ] Tests FAIL: each observed red with its producer reverted (`rfc/discrimination/rfc2516.json`).
- [ ] Tests PASS: `go test -race` over `ppp` and `pppoeclient`, 2026-10-09.
- [ ] `./le verify worktree`: owed to the main thread; this closure agent may not run a whole-tree gate.

### Integration Checklist

| Item | Evidence |
|------|----------|
| Both roles set the policy | `pppoe/server.go` `PPPoE: true`; `clientLCPPolicy` |

## Deliverables Checklist

| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| AC rejects ACCM, ACFC, FCS Alternatives on PPPoE | `TestRFC2516ACRejectsForbiddenLCPOptions` | pass (race) |
| Client rejects the same three and requests none | `TestRFC2516ClientRejectsForbiddenLCPOptions` | pass (race) |
| L2TP negotiation unchanged | L2TP leg of `TestRFC2516ACRejectsForbiddenLCPOptions` acknowledges ACCM and ACFC | pass |
| Discrimination for every RFC2516-7-2 tag | `./le rfc discriminate stem rfc2516` lists none under `unproven`; `stale` empty | 4 records |

## Security Review Checklist

| Concern | Check | Result |
|---------|-------|--------|
| Peer option bytes echoed in Configure-Reject | `negotiatePeerOption` returns `opt.Data`, bounded by the option walker's Length check | no overread |
| A peer forcing ACCM/ACFC framing on PPPoE | the reject precedes the option switch, so no Ack or Nak path exists for Types 2, 8, 9 | closed |
| Policy leaking into L2TP | only `pppoe/server.go` and `clientLCPPolicy` set `PPPoE` | L2TP unaffected |

### Documentation Update Checklist

| Category | Update needed | Where |
|----------|---------------|-------|
| User guide | Yes, already made | `docs/guide/pppoe.md` lines 15-17 (`82b495d9c3`) |
| Architecture | Yes, already made | `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/architecture/l2tp/cpe-1-pppoe-client.md` (`82b495d9c3`) |
| RFC compliance | Yes, this closure | `rfc/short/rfc2516.md` Meta coverage and remaining: "not run" replaced by the records |
| Config syntax, CLI, API, plugin SDK, wire format | No | `git show --stat 82b495d9c3` touches no `*.yang` and no `cmd/` file |

---

## Implementation Summary

### What Was Implemented
- `LCPNegPolicy.PPPoE` and its reject branch in `negotiatePeerOption`, set by the AC and by `clientLCPPolicy`.
- Closure (2026-10-09): race package tests, four RFC2516-7-2 discrimination records, stale prose corrected here and in `rfc/short/rfc2516.md`.

### Bugs Found/Fixed
- The original defect (ACCM and ACFC acknowledged on PPPoE) is covered by both regression tests.

### Documentation Updates
- `rfc/short/rfc2516.md`: Meta coverage and remaining cite the records instead of "not run".

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
| RFC2516-7-2, AC | Done | `ppp/lcp_options.go::negotiatePeerOption`, `ppp/session_run.go::negPolicy` | |
| RFC2516-7-2, client | Done | `pppoeclient/session.go::clientLCPPolicy` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| Both PPPoE roles reject the forbidden options | Done | both RFC2516 tests | |
| L2TP negotiation preserved | Done | L2TP leg of the AC test | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRFC2516ACRejectsForbiddenLCPOptions` | Done | `ppp/rfc2516_options_test.go` | |
| `TestRFC2516ClientRejectsForbiddenLCPOptions` | Done | `pppoeclient/rfc1661_client_negotiation_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| the five files under Files to Modify | Done | `c9258b5fe6`, `d115e0a1d4` |

### Audit Summary
- **Total items:** 6
- **Done:** 6
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Both PPPoE roles reject mandatory forbidden options | unit over the real frame handler, discriminated | `TestRFC2516ACRejectsForbiddenLCPOptions` enters `pppSession.handleFrame` and asserts each Configure-Reject; observed red with `negotiatePeerOption` reverted. The client test was observed red with `clientLCPPolicy` reverted (`rfc/discrimination/rfc2516.json`) |
| Unrelated L2TP negotiation preserved | unit | L2TP leg of the AC test |
| LCP still interoperates under the policy | interop | `./le test deployment docker-pppoe-accel-test`, 2026-10-09: `02-ze-ac-pppd-client` (Ze AC, policy set, against pppd) passed at 14:14 with `ZE_PPPOE_INTEROP_SCENARIO=02-ze-ac-pppd-client`, "interop: 1 passed, 0 failed"; `01-pppoe-chap-ipv4` (Ze client, policy set, against accel-ppp) passed in the 13:50 full run. Two earlier 02 runs failed before Ze started (`docker run -d` past its 30 s deadline, disk iowait 19%) and are environmental |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/pppoe-lcp-option-reject-12d06ccf-2460-42c7-a707-30bb0a427796.md` |
| `./le spec review check` | clean (run after `record`, 2026-10-09) |
| Rounds | 1 |
| Scope reviewed | producers `c9258b5fe6` (`negotiatePeerOption` PPPoE branch, `negPolicy`, `StartSession.PPPoE`, `pppoe/server.go` start) and `d115e0a1d4` (`clientLCPPolicy`, `sendLCPConfigRequest`); tests `82b495d9c3`, `d115e0a1d4`; records and `rfc/short/rfc2516.md` in `2961683671`; this spec |
| Reviewer lenses used | pre-checks: `./le repo check` and `./le commit audit base origin/main` report nothing under `internal/component/l2tp`, `rfc/` or this spec (every finding is in another session's `cmd/ze`, `command`, `plugin/server`, `bgp` files); wiring (both roles set `PPPoE`, L2TP never does: `manager.go` copies `start.PPPoE`); RFC 2516 Section 7 text against the reject branch; local requests (`sendLCPConfigRequest` and `session_run.go` build MRU and Magic only, so no forbidden option is requested); functional coverage through the daemon (lab rows in Goal Validation); security (`opt.Data` echoed is bounded by the option walker); style pass over the changed Go |
| Result | 0 BLOCKER, 0 ISSUE, 3 NOTE |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE | | |

### Notes (non-blocking)
| # | Note |
|---|------|
| N1 | This spec said the producers landed in `82b495d9c3`; `git log -S` shows `c9258b5fe6` (ppp, AC) and `d115e0a1d4` (client). Corrected in Files to Modify, Implementation Steps and Files from Plan |
| N2 | `negotiatePeerOption` matches FCS Alternatives as the literal `9` beside the named `LCPOptACCM` and `LCPOptACFC`; the comment above names it, a constant would carry it in the name |
| N3 | The four discrimination records replace the producer body with `panic`, which proves the test reaches the producer; the narrower claim (the L2TP leg and the permitted-option legs) rests on the assertions, which were read and check each Configure-Reject and Ack individually |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/ppp/rfc2516_options_test.go` | yes | carries the AC test, 2026-10-09 |
| `rfc/discrimination/rfc2516.json` | yes | four RFC2516-7-2 records added 2026-10-09 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| reject forbidden options | both roles | `go test -race -count=1` over `l2tp/ppp`, `l2tp/pppoeclient`, `l2tp/pppoe`: ok 16.9s, 1.1s, 4.7s |
| discriminated | 4 tags | `./le rfc discriminate stem rfc2516`: no RFC2516-7-2 row under `unproven`; `stale` empty |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `pppSession.handleFrame` | none; the unit test enters the real handler, and the interop row covers the daemon | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| none declared | n/a | the spec lists no A-N rows |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/pppoe.md` 15-17 | `negotiatePeerOption` PPPoE branch | yes |
| `rfc/short/rfc2516.md` Meta | `rfc/discrimination/rfc2516.json` | yes |
