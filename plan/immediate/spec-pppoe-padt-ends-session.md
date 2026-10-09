# Spec: pppoe-padt-ends-session

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

The initial defect was that `pppoeclient/dialer.go::Dial` stopped reading
discovery after PADS, so a received PADT left PPP negotiation or keepalive
running. RFC 2516 Section 5.5 states: "Even normal PPP termination packets
MUST NOT be sent after sending or receiving a PADT."

The client now runs `watchPADT` during PPP negotiation and keepalive.
It checks the interface, session ID and both MAC addresses before closing
the session's PPP descriptors. `sessionLink` serializes writes and closure;
cleanup joins the discovery reader before releasing its socket.
`sendPADT` closes any established PPP transport before transmitting.

The AC's `handlePADT` closes its separately owned PPPoX descriptor before
waiting for the PPP driver. `handleSessionDown` closes it before sending
PADT. The PPP driver remains the owner of its channel and unit descriptors.

Owner requirement, 2026-09-21: "Valid received or sent PADT ends PPP
transmission/session use; irrelevant PADT cannot terminate another session."

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2516-5.5-4 | "Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT." (Section 5.5) | `pppoeclient/dialer.go::watchPADT`, `sessionLink`, `sendPADT`; `pppoe/server.go::handlePADT`, `handleSessionDown` |

## Regressions and verification

Client regressions: `TestRFC2516ClientPADTStopsOnlyMatchingSession`,
`TestRFC2516ClientLocalStopDisablesPPP`,
`TestRFC2516ClientSentPADTStopsPPPBeforeSend`.
AC regressions: `TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`,
`TestRFC2516ACSentPADTClosesTransportBeforeSend`. The AC tests use real
socket descriptors to observe closure at the discovery-send boundary.

The client code and tests landed in `d115e0a1d4`, the AC code and tests in
`c9258b5fe6`, and the AC docs and RFC rows in `82b495d9c3`. They pass under `go test -race` (2026-10-09). Each of the five
RFC2516-5.5-4 tags carries a revert discrimination record in
`rfc/discrimination/rfc2516.json`.

"Linux PPPoE runtime scenarios" names the Docker PPPoE lab
(`./le test deployment docker-pppoe-accel-test`, host-kernel PPPoE, see
`docs/labs/pppoe-interop.md`). Its `01-pppoe-chap-ipv4` stops the Ze client and
requires accel-ppp to drop the session (`check_client.go::checkZeClient`), and
`02-ze-ac-pppd-client` requires pppd's "Sent PADT" and an empty Ze session table
(`check_ac.go::checkTeardown`). Neither asserts silence after PADT, so they
prove interoperation of the changed teardown paths; the MUST NOT itself is
proven by the discriminated unit tests.

## Required Reading

- [ ] `rfc/full/rfc2516.txt` Section 5.5 -> Constraint: "Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT."
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` -> Decision: `watchPADT` owns discovery reads after PADS.
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` -> Decision: the AC closes its PPPoX descriptor before waiting on PPP or sending PADT.
- [ ] `docs/research/l2tpv2-implementation-guide.md` -> Constraint: design document the PPP sources declare.

## Current Behavior

- [ ] `internal/component/l2tp/pppoeclient/dialer.go` -- `watchPADT`, `sessionLink`, `sendPADT`.
- [ ] `internal/component/l2tp/pppoe/server.go` -- `handlePADT`, `handleSessionDown`.

## Data Flow

### Entry Point
A PADT on the client's discovery socket, or on the AC's discovery reader; a local stop on either side.

### Transformation Path
The PADT is matched on interface, session ID and both MACs; a match closes the PPP transport (`sessionLink.Close` on the client, the PPPoX descriptor on the AC) before any further PPP write can happen. A local stop closes PPP first, then sends PADT.

### Boundaries Crossed
Discovery socket (AF_PACKET) to the PPP session descriptors; on the AC, the PPP driver keeps its channel and unit descriptors.

### Integration Points
`pppoeclient.Dial` cleanup joins the discovery reader; the AC's session table `Remove`.

## Wiring Test

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| client discovery PADT -> `watchPADT` -> `TestRFC2516ClientPADTStopsOnlyMatchingSession` |
| client local stop -> `sendPADT` -> `TestRFC2516ClientSentPADTStopsPPPBeforeSend`, `TestRFC2516ClientLocalStopDisablesPPP` |
| AC discovery PADT -> `handlePADT` -> `TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport` |
| AC session down -> `handleSessionDown` -> `TestRFC2516ACSentPADTClosesTransportBeforeSend` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Requirement |
|------|------|-------------|
| `TestRFC2516ClientPADTStopsOnlyMatchingSession` | `internal/component/l2tp/pppoeclient/rfc2516_padt_lifetime_test.go` | RFC2516-5.5-4 positive and negative |
| `TestRFC2516ClientSentPADTStopsPPPBeforeSend` | same file | RFC2516-5.5-4 positive |
| `TestRFC2516ClientLocalStopDisablesPPP` | same file | local stop |
| `TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport` | `internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go` | RFC2516-5.5-4 positive and negative |
| `TestRFC2516ACSentPADTClosesTransportBeforeSend` | same file | RFC2516-5.5-4 positive |

## Files to Modify

- `internal/component/l2tp/pppoeclient/dialer.go`, `internal/component/l2tp/pppoe/server.go` (client in `d115e0a1d4`, AC in `c9258b5fe6`).

## Implementation Steps

1. Keep discovery reading after PADS and close PPP on a matching PADT (done).
2. Close PPP before sending PADT on both roles (done).
3. Closure: race tests, discrimination records, lab run, stale prose (2026-10-09).

## Checklist

- [ ] Tests written: the five tests above exist.
- [ ] Tests FAIL: each tagged unit observed red with its producer reverted (`rfc/discrimination/rfc2516.json`).
- [ ] Tests PASS: `go test -race` over `pppoeclient` and `pppoe`, 2026-10-09.
- [ ] `./le verify worktree`: owed to the main thread; this closure agent may not run a whole-tree gate.

### Integration Checklist

| Item | Evidence |
|------|----------|
| Client cleanup joins the discovery reader | `dialer.go` `Dial` cleanup, `sessionLink` |

## Deliverables Checklist

| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| Received matching PADT ends PPP use, client and AC | the two received-PADT tests | pass (race) |
| Irrelevant PADT cannot end another session | negative legs of the same tests | pass |
| Sent PADT is preceded by PPP closure | the two sent-PADT tests | pass |
| Discrimination for every RFC2516-5.5-4 tag | `./le rfc discriminate stem rfc2516`: none under `unproven`, `stale` empty | 5 records |
| Lab teardown with independent peers | `./le test deployment docker-pppoe-accel-test` | `01-pppoe-chap-ipv4` passed (13:50 full run), `02-ze-ac-pppd-client` passed (14:14 selected run), 2026-10-09 |

## Security Review Checklist

| Concern | Check | Result |
|---------|-------|--------|
| Spoofed PADT tearing down another subscriber | `watchPADT` and `handlePADT` match interface, session ID and both MACs; negative test legs | refused |
| Malformed discovery frame | client test feeds malformed frames; PPP stays usable | refused |
| Use-after-close race on the descriptor | `sessionLink` serializes writes and closure; cleanup joins the reader before releasing the socket | no race under `-race` |

### Documentation Update Checklist

| Category | Update needed | Where |
|----------|---------------|-------|
| Architecture | Yes, already made | `docs/architecture/l2tp/cpe-1-pppoe-client.md` lines 138-146, `docs/architecture/l2tp/bng-5-pppoe.md` lines 28-35 |
| User guide | Yes, already made | `docs/guide/pppoe.md` lines 17-23 |
| RFC compliance | Yes, this closure | `rfc/short/rfc2516.md` Section 5.5 prose and Meta: "unrun" replaced by the records |
| Config, CLI, API, wire format | No | no YANG or command change in `d115e0a1d4` |

---

## Implementation Summary

### What Was Implemented
- Client: `watchPADT`, `sessionLink`, `sendPADT` (`d115e0a1d4`). AC: `handlePADT` and `handleSessionDown` close the PPPoX descriptor first.
- Closure (2026-10-09): race tests, five RFC2516-5.5-4 records, PPPoE lab run, stale prose corrected.

### Bugs Found/Fixed
- The original defect (client stopped reading discovery after PADS) is covered by `TestRFC2516ClientPADTStopsOnlyMatchingSession`.

### Documentation Updates
- `rfc/short/rfc2516.md` Section 5.5 prose and Meta rows.

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
| RFC2516-5.5-4 client | Done | `pppoeclient/dialer.go::watchPADT`, `sendPADT` | |
| RFC2516-5.5-4 AC | Done | `pppoe/server.go::handlePADT`, `handleSessionDown` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| Valid received or sent PADT ends PPP use | Done | the four positive tests | |
| Irrelevant PADT cannot end another session | Done | negative legs | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| five tests in the Unit Tests table | Done | the two files named there | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `dialer.go`, `server.go` | Done | `d115e0a1d4`, `c9258b5fe6` |

### Audit Summary
- **Total items:** 9
- **Done:** 9
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Valid PADT ends PPP transmission | unit with real descriptors, discriminated | AC tests use real sockets and are observed red with `handlePADT` / `handleSessionDown` reverted; client tests red with `watchPADT` / `sendPADT` reverted (`rfc/discrimination/rfc2516.json`) |
| Irrelevant PADT cannot terminate another session | unit, discriminated | negative records on the same units |
| Teardown interoperates | interop | `./le test deployment docker-pppoe-accel-test`, 2026-10-09: `01-pppoe-chap-ipv4` (Ze client stops, accel-ppp drops the session, `check_client.go::checkZeClient`) passed in the 13:50 full run; `02-ze-ac-pppd-client` (pppd sends PADT, Ze session table empties, `check_ac.go::checkTeardown`) passed at 14:14, "interop: 1 passed, 0 failed". Two earlier 02 runs failed before Ze started (`docker run -d` past its 30 s deadline under disk iowait) and are environmental. Neither scenario asserts silence after PADT; the MUST NOT rests on the discriminated unit tests |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/pppoe-padt-ends-session-12d06ccf-2460-42c7-a707-30bb0a427796.md` |
| `./le spec review check` | clean (run after `record`, 2026-10-09) |
| Rounds | 1 |
| Scope reviewed | client `d115e0a1d4` (`Dial`, `sessionLink`, `watchPADT`, `sendPADT`, `superviseNetworkPhase`); AC `c9258b5fe6` (`handlePADT`, `handleSessionDown`, `detachTransport`); tests in both; records and `rfc/short/rfc2516.md` in `2961683671`; this spec |
| Reviewer lenses used | pre-checks: `./le repo check` and `./le commit audit base origin/main` report nothing under `internal/component/l2tp`, `rfc/` or this spec (every finding is in another session's files); RFC 2516 Section 5.5 text against the four producers; write paths (every client PPP write goes through `sessionLink.Write`, which refuses after `Close`; `chanFD` is used only for `ppp.Connect`); spoofed-PADT matching (interface, SID, both MACs on the client; SID, destination and source MAC on the per-interface AC); descriptor lifetime (cleanup closes the link, joins `watchPADT` via `discoveryDone`, then closes the discovery fd); functional coverage through the daemon (lab rows in Goal Validation); style pass over the changed Go (no peer-reachable `panic`; `watchPADT` bounded by `SO_RCVTIMEO` and the link lifetime, stated in its doc comment; MUST stated on both sides of `sessionLink.Close` and `watchPADT`'s done) |
| Result | 0 BLOCKER, 0 ISSUE, 3 NOTE |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE | | |

### Notes (non-blocking)
| # | Note |
|---|------|
| N1 | This spec said the AC code landed in `d115e0a1d4`; `git log -S` shows `c9258b5fe6` for `detachTransport` and the AC tests. Corrected in Regressions, Files to Modify, Implementation Summary and Files from Plan |
| N2 | The discrimination records replace the producer body with `panic`; the negative legs (irrelevant PADT) are proven by the assertions, which write through the link after every non-matching frame and fail on the first premature close |
| N3 | `handlePADT` logs a source-MAC mismatch at Warn for every such frame, so an access-side host can drive log volume (behavior older than this spec); journal row in `plan/journal/peer-refusal-logged-at-warn.md` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go` | yes | carries both AC tests |
| `internal/component/l2tp/pppoeclient/rfc2516_padt_lifetime_test.go` | yes | carries the three client tests |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| PADT ends PPP | client and AC | `go test -race -count=1` over `l2tp/pppoeclient` and `l2tp/pppoe`: ok 1.1s, 4.7s |
| discriminated | 5 tags | `./le rfc discriminate stem rfc2516`: no RFC2516-5.5-4 row under `unproven`; `stale` empty |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| PPPoE teardown through the daemon | none; Docker lab scenarios 01 and 02 | yes, both passed 2026-10-09 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| none declared | n/a | the spec lists no A-N rows |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `cpe-1-pppoe-client.md` 138-146 | `dialer.go::watchPADT`, `sendPADT` | yes |
| `bng-5-pppoe.md` 28-35 | `server.go::handlePADT`, `handleSessionDown` | yes |
