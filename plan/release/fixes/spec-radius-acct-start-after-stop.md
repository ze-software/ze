# Spec: radius-acct-start-after-stop

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped-daemon defect an operator's RADIUS server meets: a subscriber session torn down just after it gets its address can send the Accounting-Stop before the Accounting-Start, and the server is left with an open accounting record for a session that ended. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` (same file, `acct.go`; its open question 3 moved the Accounting-Stop onto the session's own accounting task, which is the fix this spec needed) |
| Phase | Met by `93ff938cdb`, which landed under `spec-l2tp-session-identity-and-publish-stall` (its open question 3); the wire order for the exact Down-before-Start case is asserted since `938cf958c1`. See [Resolution by 93ff938cdb](#resolution-by-93ff938cdb). Closing |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

One defect met on 2026-11-12 while
`spec-single-owner-lifecycle-5-session-end-locals` was implemented. That spec
orders the L2TP session events; this race is inside the RADIUS accounting
reader, so the event order does not close it. It has a journal row (Owned
journal rows). The fix landed under another spec (Resolution by 93ff938cdb),
so this spec records the evidence and closes.

| Part | Producer | What the operator met | Why it is owed |
|------|----------|-----------------------|----------------|
| Accounting-Start can follow the Stop | `onSessionIPAssigned`, `runAccounting`, `onSessionDown` (`internal/component/l2tp/plugins/authradius/acct.go`) | Before `93ff938cdb`, `onSessionIPAssigned` started `runAccounting` with `go`, and it sent the Start without checking its context. `onSessionDown` cancelled that context and sent the Stop at once, without waiting for the task. A teardown just after the address was assigned could put the Stop on the wire first | The RADIUS server keeps an open record for a session that ended, so the operator's billing and session counts are wrong |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: a goroutine is a long-lived worker on a channel, never one per event.
- [ ] `ai/doors/access-security.md` - L2TP and RADIUS accounting
  → Constraint: a RADIUS server is an outside party; the billing records it keeps are its contract with Ze.
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - the RADIUS accounting paragraph (source anchor `onSessionDown, runAccounting`)
  → Constraint: the publisher of a session's Down never waits for a RADIUS server.
- [ ] `docs/research/l2tpv2-ze-integration.md` - section 8.4, "End-to-end flow: subscriber goes down", the `// Design:` page of `acct.go`
  → Constraint: a session-down leads to the RADIUS Accounting-Stop; the page names no sender goroutine, so `93ff938cdb` made it no less accurate.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc2866.txt` Section 2, quoted in `TestTeardownDuringIPAssignedLeavesNoAccountingSession` and above `runAccounting`: "At the end of service delivery the client will generate an Accounting Stop packet describing the type of service that was delivered and optionally statistics such as elapsed time, input and output octets, or input and output packets." Section 5.5, quoted in `onSessionIPAssigned`: "The start and stop records for a given session MUST have the same Acct-Session-Id."

**Key insights:**
- The Stop was the one accounting record sent from outside the session's own accounting task. Moving it onto that task makes the order structural.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - at HEAD, `onSessionIPAssigned` stores the accounting session and starts `runAccounting` on one goroutine for the session's lifetime. `runAccounting` calls `sendAcctStart`, then `interimLoop` until the session's context ends, then waits on `sess.ended` and calls `sendAcctStop`. `onSessionDown` removes the session from the live map, adds it to `a.ending`, and calls `sess.end` with the Down's cause; it sends nothing. `sendAcctStop` has one caller, `runAccounting`.
- [ ] `internal/test/fixture/tunnel_fixture_l2tp_session_clear.go` - `checkClearedAccounting` counts one Start and one Stop under one Acct-Session-Id after an operator clear; it read the records once, right after the clear answered, and its comments, with `awaitAccountingStart`'s, still described the pre-`93ff938cdb` race.

**Behavior to preserve:**
- One Accounting-Stop for each Accounting-Start, with the terminate cause the teardown named.
- The Acct-Session-Id that Start and Stop share.

**Behavior to change:**
- No Accounting-Start reaches the server after its session's Stop. Done by `93ff938cdb`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-session accounting task | `internal/component/l2tp/plugins/authradius/acct.go` `runAccounting` | One goroutine per accounting session, sending Start, Interims, then Stop | Extended by `93ff938cdb` (under the identity spec): the Stop joined the task, which closes this defect. Nothing further here |
| Session end hand-over | `acct.go` `onSessionDown`, `acctSession.end` | Claims the session record and hands the end of service to the task without waiting for RADIUS | Extended by `93ff938cdb`: replaced the direct `sendAcctStop` call. Nothing further here |
| Per-tunnel session notice order | `internal/component/l2tp` `sessionNoticeTicket` (2026-10-14 row of `plan/journal/late-write-lands-on-the-successor.md`) | Keeps a SessionDown from overtaking its SessionIPAssigned | Does not apply: it orders the events, and this race was inside the reader |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The L2TP engine bus: a SessionIPAssigned event, then a SessionDown event for the same session (an operator `clear l2tp session`, a timeout, a CoA Disconnect, a peer's CDN).

### Transformation Path
1. `onSessionIPAssigned` builds the `acctSession` and starts `runAccounting`.
2. `runAccounting` sends the Start and waits in `sendAcctPacket` for the acknowledgement or `acctRecordBound`.
3. `onSessionDown` calls `sess.end(cause)`, closing `sess.ended` and cancelling the task's context.
4. `runAccounting` leaves `interimLoop`, waits on `sess.ended`, and sends the Stop.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine bus to the authradius plugin | `radiusAcct.subscribeEventBus` | Yes: `TestTeardownDuringIPAssignedLeavesNoAccountingSession` subscribes through it |
| Ze to the RADIUS server | Signed UDP Accounting-Request | Yes: the ordered server in `acct_original_task_test.go` captures every request on a real socket |

### Integration Points
- `subscribeEventBus` (`acct.go`) - the reader the engine bus delivers both events to.

### Changed-Wire Census

The packet format did not change; `93ff938cdb` changed only the order in which
one session's Accounting-Requests leave Ze.

**Search evidence:** `git grep -ln "AcctStatusStop\|accountingStatusStop" -- '*.go'` over the whole tree, 2026-11-19: the one production writer is `acct.go` (`sendAcctStop`, called only from `runAccounting`); `exclude.go` classifies a status and sends nothing; every other hit is a test reader. Known control: `sendAcctStop` found.

| Wire | Role (writer / reader) | Carrier (file + symbol, or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|------------------------------------------|-----------------|---------------------------------------|
| RADIUS Accounting-Request order of one session | writer | `acct.go` `runAccounting` through `sendAcctStart`, `sendAcctStop` | read at HEAD | Changed by `93ff938cdb`: Start, Interims, then Stop from one task |
| RADIUS Accounting-Request order of one session | reader | The operator's RADIUS server (outside the repository; FreeRADIUS in `l2tp-many-sessions-kl2tpd`) | RFC 2866 Section 2 and 5.5 | Preserved contract: it now always sees the Start first |
| RADIUS Accounting-Request order of one session | reader | `acct_original_task_test.go` `assertOrderedAccountingWire` | read at HEAD | Asserts the order |
| RADIUS Accounting-Request order of one session | reader | `tunnel_fixture_l2tp_session_clear.go` `checkClearedAccounting` | read at HEAD | Counts records, does not read the order; now waits, bounded, for the Stop the task sends after the clear answered |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SessionIPAssigned and SessionDown published back to back on the engine bus | → | `onSessionIPAssigned`, `onSessionDown`, `runAccounting` | `TestTeardownDuringIPAssignedLeavesNoAccountingSession` (`acct_order_test.go`) |
| Operator `clear l2tp session` on a live session, through the daemon | → | the same three, over a RADIUS mock | `test/l2tp/session-clear-leaves-no-subscriber.ci` (counts one Start and one Stop; the wire order is the unit tests') |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Stop can reach the server before the Start | Read 2026-11-12 in `acct.go` | The row closes with the evidence | A unit test that ends the session before the accounting task sends its Start | confirmed on the pre-`93ff938cdb` code: `TestAccountingDownReturnsWhileStartAwaitsRadius` was committed red in `113944fea8` (a Stop competing with a held, unacknowledged Start) and passes after `93ff938cdb`. The exact "Down before the Start is sent" case is asserted since `938cf958c1` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A session ends right after its address is assigned, before its accounting task has sent the Start | The server receives exactly two records for the session, the Accounting-Start then the Accounting-Stop, both with the same Acct-Session-Id and the Stop carrying the Down's terminate cause; no Start arrives after the Stop | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAccountingDownReturnsWhileStartAwaitsRadius` | `internal/component/l2tp/plugins/authradius/acct_original_task_test.go` | AC-1: with the Start held unacknowledged on a real signed UDP exchange, the Down returns at once, and the wire then carries exactly Start then Stop with the same Acct-Session-Id and the Down's cause (`assertOrderedAccountingWire` also fails on any Stop sent before the held acknowledgement) | red in `113944fea8`, green after `93ff938cdb` |
| `TestTeardownDuringIPAssignedLeavesNoAccountingSession` | `internal/component/l2tp/plugins/authradius/acct_order_test.go` | AC-1, the exact condition: IP-assigned and Down published back to back on the engine bus, before the task can send its Start; after `acct.Stop` joins the task, `assertOrderedAccountingWire` requires exactly Start then Stop under the Start's Acct-Session-Id, and no session is left | wire-order assertion added in `938cf958c1`; its MUTATION (`runAccounting` sends the Stop before the Start) was run red on the claude server, as the main thread reported, and green restored |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-many-sessions-kl2tpd` | `test/interop-l2tp/scenarios/l2tp-many-sessions-kl2tpd/` | FreeRADIUS, kl2tpd, pppd | Subscriber sessions through the task-owned accounting reach a real RADIUS server and tear down; it does not assert the order of one session's records | passed unchanged at `938cf958c1`, after `93ff938cdb` (recorded in the identity spec) |

## Files to Modify
- `internal/component/l2tp/plugins/authradius/acct.go` - done by `93ff938cdb`.
- `internal/component/l2tp/plugins/authradius/acct_order_test.go` - done by `938cf958c1`.
- `internal/test/fixture/tunnel_fixture_l2tp_session_clear.go` - `checkClearedAccounting` waits, within `sessionClearWaitShare`, for the Stop before judging the records, and the comments of it, `awaitAccountingStart`, `tunnelL2TPSessionClear` and `sessionClearWaitShare` describe the task-sent Stop; both stale since `93ff938cdb` (closure).

## Files to Create
- None.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- done under the identity spec: the Stop moved onto `runAccounting` (`93ff938cdb`), and the engine-bus test asserts the wire order (`938cf958c1`). Closure repairs the daemon fixture the change left stale: its comments, and a single read that could precede the task-sent Stop.

## Resolution by 93ff938cdb

The defect was fixed inside `spec-l2tp-session-identity-and-publish-stall`
(its open question 3: the Stop moved onto the session's own accounting task).
Producer read at HEAD, `internal/component/l2tp/plugins/authradius/acct.go`:

| Site | What it does now |
|------|------------------|
| `runAccounting` | The one sender of a session's records. It sends the Start (`sendAcctStart`, which waits in `sendAcctPacket` for an acknowledgement or the 10 s `acctRecordBound`), runs `interimLoop` until the session's context ends, then waits on `sess.ended` and only then sends the Stop |
| `onSessionDown` | Removes the session from the live map, marks it ending and calls `sess.end` with the Down's cause. It sends nothing itself and does not wait for a RADIUS server |
| Order consequence | The Start is the task's first action and is never skipped, so a Down that arrives before the Start was sent still gets Start then Stop on the wire, the Start first, with the same Acct-Session-Id |

| AC | Verdict | Evidence |
|----|---------|----------|
| AC-1 | Met | Structure: one goroutine sends Start, Interims, then Stop (`runAccounting`), and nothing else calls `sendAcctStop`. Tests: `TestAccountingDownReturnsWhileStartAwaitsRadius` asserts Start then Stop against a held Start and refuses an early Stop (red in `113944fea8`, green after `93ff938cdb`); `TestTeardownDuringIPAssignedLeavesNoAccountingSession` asserts Start then Stop for the exact back-to-back condition since `938cf958c1` |

Behavior to preserve holds: one Stop per Start, carrying the Down's terminate
cause (asserted in `TestAccountingDownReturnsWhileStartAwaitsRadius`), and the
shared Acct-Session-Id (both tests).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/late-write-lands-on-the-successor.md` | 2026-11-12 | authradius Accounting-Start sent by `runAccounting` can follow the Stop |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Close against `93ff938cdb` rather than design a second fix | Waiting in `onSessionDown` for the task before sending the Stop | The identity spec's fix already makes the order structural, and a wait in `onSessionDown` would stall the reactor that publishes the Down on a silent RADIUS server |

## Known Limitations
- None for this AC. The daemon-level `.ci` counts records and leaves their order to the unit tests, which drive the real reader and a real UDP server.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 demonstrated (Resolution by 93ff938cdb)
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/component/l2tp/plugins/authradius/` (owed command). No agent runs any `./le verify` command

### TDD
- [ ] Tests written: `TestAccountingDownReturnsWhileStartAwaitsRadius` (`113944fea8`), the wire-order assertion of `TestTeardownDuringIPAssignedLeavesNoAccountingSession` (`938cf958c1`)
- [ ] Tests FAIL: `TestAccountingDownReturnsWhileStartAwaitsRadius` red at `113944fea8`; the `938cf958c1` mutation red ("record 0 status=2, want 1") on the claude server, as the main thread reported
- [ ] Tests PASS: both green after `93ff938cdb`; the rerun at HEAD is an owed command

## Deliverables Checklist

| Deliverable | Verification method | Evidence |
|-------------|---------------------|----------|
| The Stop is sent only by the session's accounting task | `grep -n sendAcctStop internal/component/l2tp/plugins/authradius/*.go` (non-test) | One call, inside `runAccounting`, after `<-sess.ended` |
| AC-1 wire-order tests exist | `gopls symbols` on `acct_original_task_test.go`; `git show 938cf958c1` | `TestAccountingDownReturnsWhileStartAwaitsRadius` and `assertOrderedAccountingWire` exist; `TestTeardownDuringIPAssignedLeavesNoAccountingSession` calls `assertOrderedAccountingWire` |
| The tests pass at HEAD | `./le test unit pkg path ./internal/component/l2tp/plugins/authradius/ run 'TestAccountingDownReturnsWhileStartAwaitsRadius\|TestTeardownDuringIPAssignedLeavesNoAccountingSession'` | owed command, run by the main thread |
| Journal row rewritten to `fixed:` | read `plan/journal/late-write-lands-on-the-successor.md` | the 2026-11-12 row, in commit A |

## Security Review Checklist

| Concern | Check | Result |
|---------|-------|--------|
| A silent RADIUS server stalls the reactor | `onSessionDown` sends nothing and waits for nothing; each record is bounded by `acctRecordBound` in the task | No stall |
| Goroutine leak per session | `runAccounting` closes `sess.finished` and releases the endpoint on exit; `stopDetached` and `acct.Stop` join it (doc anchor in `subscriber-session-model.md`) | One bounded task per session lifetime |
| Untrusted input | The closure changes a test fixture only: comments and a bounded wait over the RADIUS mock's log | Not applicable |

## Documentation Update Checklist

| Category | Update needed | Evidence |
|----------|---------------|----------|
| Architecture design | No: already done by `93ff938cdb` | `docs/architecture/l2tp/subscriber-session-model.md`, the RADIUS accounting paragraph ("That one task sends the session's Start, its Interim-Updates and then its Stop"), anchor `acct.go -- onSessionDown, runAccounting` |
| User guide, config, CLI, API, plugin SDK, wire format | No | No surface changed: `git show --stat 93ff938cdb` touches no YANG, command or guide file |
| RFC compliance | No | No requirement row or tag changed by this closure; `93ff938cdb` re-took its RFC 2869 records itself |
| Feature list, comparison | No | Accounting already shipped; the fix corrects behavior and moves no feature scope |
| Test infrastructure | Yes | `internal/test/fixture/tunnel_fixture_l2tp_session_clear.go` `checkClearedAccounting` and `awaitAccountingStart` comments described the pre-fix Stop and one cited this spec's path; rewritten in commit A, with the bounded wait for the Stop |

---

## Implementation Summary

### What Was Implemented
- Nothing new under this spec: `93ff938cdb` (identity spec) moved the Accounting-Stop onto `runAccounting`, and `938cf958c1` made `TestTeardownDuringIPAssignedLeavesNoAccountingSession` assert the wire order.
- Closure: the daemon fixture's stale comments, its bounded wait for the task-sent Stop, the journal row, and this record.

### Bugs Found/Fixed
- Stale comment in `checkClearedAccounting` (`internal/test/fixture/tunnel_fixture_l2tp_session_clear.go`): it still said the Stop is sent at once on teardown and can reach the server first. Rewritten to name `runAccounting` and the two tests; `awaitAccountingStart`'s comment likewise.
- Latent false red in `checkClearedAccounting`: it read the RADIUS records once, right after the clear answered, while since `93ff938cdb` the Stop leaves from the task after `onSessionDown` returned, so a run could see 0 Stops. It now polls through `link.await` (every `sessionClearDelay`, within `sessionClearWaitShare` of the budget) until a Stop is recorded, then judges that read with every earlier assertion: exactly one Start and one Stop, one Acct-Session-Id, the Admin Reset cause. A test-robustness change, not a weakening: no assertion is removed or loosened, and the wait ends on state, not on time.

### Documentation Updates
- None needed beyond the fixture comment: `subscriber-session-model.md` was updated by `93ff938cdb`.

### Deviations from Plan
- The fix came from another spec, so this spec carries no implementation commit of its own.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `93ff938cdb` changed the accounting order and left the `checkClearedAccounting` comment describing the old race | The comment is a citer of the behavior, outside the package the fix changed | Closure citer search for this spec's path | Rewritten in commit A |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| No Start after its session's Stop | Done | `acct.go` `runAccounting` | by `93ff938cdb` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAccountingDownReturnsWhileStartAwaitsRadius`, `TestTeardownDuringIPAssignedLeavesNoAccountingSession` | the second covers the exact Down-before-Start condition |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAccountingDownReturnsWhileStartAwaitsRadius` | Done | `acct_original_task_test.go` | red `113944fea8`, green after `93ff938cdb` |
| `TestTeardownDuringIPAssignedLeavesNoAccountingSession` | Done | `acct_order_test.go` | order asserted since `938cf958c1` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `acct.go` | Done | `93ff938cdb` |
| `acct_order_test.go` | Done | `938cf958c1` |
| `tunnel_fixture_l2tp_session_clear.go` | Done | comments and bounded Stop wait, commit A |

### Audit Summary
- **Total items:** 7
- **Done:** 7
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The RADIUS server never receives a Start after its session's Stop | data correctness, on the wire | `assertOrderedAccountingWire` over a real signed UDP capture in both AC-1 tests: exactly Start then Stop under one Acct-Session-Id; the mutation that sends the Stop first fails with "record 0 status=2, want 1" (`938cf958c1`, run red on the claude server as the main thread reported) |
| Sessions with task-owned accounting work against a real RADIUS server | interop | `l2tp-many-sessions-kl2tpd` passed at `938cf958c1` against FreeRADIUS (identity spec) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | AC-1 is met | `plan/release/fixes/spec-radius-acct-start-after-stop.md` (this spec, closing) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | written by the independent reviewer's `./le spec review record spec spec-radius-acct-start-after-stop` over this spec, the journal file, the fixture and the handover; its path is in the main thread's report, because naming it here would change the hash it pins |
| `./le spec review check` | run by the main thread right after the record |
| Rounds | 2: round 1, an independent ze-review, found the two ISSUEs below; round 2 reviews their fixes |
| Reviewer lenses used | logic and wiring of the AC-1 producer (`runAccounting`, `onSessionDown`), stale comments and citers, journal row shape, the fixture's wait |

### Run 1
- `./le test weakened check` and `./le repo check`: no finding on this diff's files (the findings are IKE tests and a plugin-process file of other sessions).
- NOTE: the interop directory was named wrongly; fixed to `test/interop-l2tp/scenarios/l2tp-many-sessions-kl2tpd/`.
- NOTE: `awaitAccountingStart`'s comment still described the Start-after-Stop race, and `checkClearedAccounting` read the records once while the Stop leaves from the task after the clear answered; both fixed (Bugs Found/Fixed).
- NOTE: `docs/research/l2tpv2-ze-integration.md` section 8.4 says only that a session-down leads to the Accounting-Stop; still accurate.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `checkClearedAccounting` comment described the pre-`93ff938cdb` race and cited this spec's path, which commit B removes | `internal/test/fixture/tunnel_fixture_l2tp_session_clear.go` | comment rewritten in commit A |
| 2 | ISSUE | The rewritten 2026-11-12 journal row was 993 characters, over the row cap | `plan/journal/late-write-lands-on-the-successor.md` | row shortened; `./le spec journal check` valid |
| 3 | ISSUE | Citers of this spec's path remained in the identity spec and handover 48 | `plan/handover/48-l2tp-identity-land-l2tp-chunk-then-close.md`; the identity spec's row | handover restated with the bare stem; the identity spec's row is left to its own closure |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/l2tp/session-clear-leaves-no-subscriber.ci` | Yes | `ls` 2026-11-19: 5.9K |
| `test/interop-l2tp/scenarios/l2tp-many-sessions-kl2tpd/` | Yes | `ls` 2026-11-19 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Only `runAccounting` sends a Stop, after the Start | `grep sendAcctStop` non-test: one call, in `runAccounting` after `<-sess.ended`; the scoped test run is an owed command |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Engine bus SessionIPAssigned then SessionDown | unit `TestTeardownDuringIPAssignedLeavesNoAccountingSession` via `subscribeEventBus` | Yes, read `git show 938cf958c1` |
| `clear l2tp session` through the daemon | `test/l2tp/session-clear-leaves-no-subscriber.ci` | Yes, counts one Start and one Stop per Acct-Session-Id (`checkClearedAccounting`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | red `113944fea8` before `93ff938cdb` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `subscriber-session-model.md`: one task sends Start, Interims, then Stop | `acct.go` `runAccounting` | Yes |
| Fixture comment names `runAccounting` and the two tests | `acct.go` `runAccounting`, `acct_original_task_test.go`, `acct_order_test.go` | Yes |
