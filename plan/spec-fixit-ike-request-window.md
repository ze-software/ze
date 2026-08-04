# Spec: fixit-ike-request-window

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; nothing deferred; window-size negotiation was never in scope and is conditional on a peer that does not exist. Create `plan/deferrals/fixit-ike-request-window.md` on the first deferral) |
| Updated | 2026-08-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 by phase 2b of `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. An agent
tried to write tagged tests for `RFC7296-2.3-2` and `RFC7296-2.3-4`. The agent
refused to tag either row. A test bound to any one guard stays green while the
endpoint exceeds the window through the other paths. The supervising session then read
every producer named below and confirmed the trace. Owner ruling taken the same day:
fix it in this named spec, and implement it now.

## Task

**Corrected 2026-08-04. The text below described the tree of 2026-07-30, before the
window was implemented. It is rewritten here to describe the tree at HEAD.** The
paragraphs that named four unguarded emitters are kept under "The tree of 2026-07-30",
because the Acceptance Criteria and the Tests FAIL output still refer to them.

### The state at HEAD

`SA.reserveRequestWindow` (`internal/component/ike/engine/msgid.go`) refuses when
`sa.requestOutstanding || sa.msgIDExhausted`, and every one of the seven request
emitters reserves before it builds. RFC 7296 Section 2.3's "one outstanding request"
obligation, row `RFC7296-2.3-2`, is met. The effective send window is the constant 1,
so row `RFC7296-2.3-4` cannot be exceeded either.

Ze DOES parse a peer SET_WINDOW_SIZE. `wire.ParseSetWindowSize` reaches
`recordPeerWindowSize` (`msgid.go`), which stores `sa.PeerWindowSize`. Ze never SENDS
one, and `reserveRequestWindow` never reads `PeerWindowSize`. The stored value is
reported by `show vpn ipsec sa` and bounds nothing. The "never parses it" sentence
below is therefore wrong at HEAD.

### What was left to fix

Three defects were named when this session opened. The independent review of their
fix named three more, and the "Findings fixed" table below carries all six.

| # | Defect | Producer |
|---|--------|----------|
| 1 | A Delete is DROPPED when the window is held, and is never remembered. Row `RFC7296-2.1-5` [MUST] asks the initiator to remember each request until its response arrives | `sendDeleteIKE`, `sendDeleteESP` (`engine/delete.go`) |
| 2 | A refused reservation spends a Message ID, because `advanceMsgID` runs before `reserveRequestWindow` | `sendIKESATeardown` (`engine/delete.go`) |
| 3 | `TestNegSharedSecretRefusesWrongLength` is tagged `RFC7296-1.3-2`, a row about the responder's INVALID_KE_PAYLOAD answer. The test builds no message and emits no Notify, so it cannot prove that row. It proves `RFC7296-3.4-1` | `internal/component/ike/engine/rfc7296_negotiation_test.go` |

Defect 1 is the one that diverges state with a peer. A dropped Delete leaves the peer
holding a Child SA or an IKE SA that Ze has torn down, until its own lifetime ends it.

### The tree of 2026-07-30

RFC 7296 Section 2.3 carries two obligations, and Ze met neither.

`RFC7296-2.3-2`, level MUST. Section 2.3 states:

<!-- ste: ignore -->
> An IKE endpoint MUST wait for a response to each of its messages before sending a subsequent message

`RFC7296-2.3-4`, level MUST NOT. Section 2.3 states:

<!-- ste: ignore -->
> An IKE endpoint MUST NOT exceed the peer's stated window size for transmitted IKE requests

Both quotations are verbatim, because a changed quotation is false evidence.

Four code paths emit a self-initiated request. Each one takes the next value of the
single counter `sa.NextMsgID` (`internal/component/ike/engine/sa.go`). No shared
slot couples them.

| # | Path | Producer | Guard |
|---|------|----------|-------|
| 1 | Dead peer detection | `engine/dpd.go`, `:94` | `dpdState.awaitReply` only (`engine/dpd.go`) |
| 2 | Child SA rekey | `engine/rekey.go`, `:86` | `ps.pendingRekey == nil` only (`engine/established.go`) |
| 3 | IKE SA rekey | `engine/rekey.go`, `:329` | `ps.pendingRekey == nil` only (`engine/established.go`) |
| 4 | Delete IKE and Delete ESP | `engine/inbound.go`, `:242`, `:304`, `:309` | **none**, beyond `tr == nil` |

**The guards never consult each other.** One `maintainSA` tick runs the DPD branch at
`engine/established.go` and then the rekey branch at `:197` or `:229`. The first
sets `awaitReply` and the second reads `pendingRekey`, so both fire. The DPD probe takes
message id N and the rekey request takes N+1, with no response to either yet.

**Path 4 is the worst case, because it has no guard at all.** `sendDeleteIKE`
(`engine/inbound.go`) and `sendDeleteESP` (`engine/inbound.go`) check
only that the transport is present. `sendDeleteESP` is called from `engine/inbound.go`
while a DPD probe CAN be outstanding.

**The window is permanently 1, so the RFC's own escape never applies.** Section 2.3
allows a larger window when the peer sends a SET_WINDOW_SIZE Notify.
`NotifySetWindowSize` exists as a bare constant (`internal/component/ike/wire/payload_notify.go`).
A tree-wide search finds no other use, so Ze never sends it and never parses it.
**Void at HEAD: Ze parses it. See "The state at HEAD" above.**

**One outstanding-request slot already exists, and it is not general.** `classifyInbound`
(`engine/msgid.go`) tracks exactly one, `pendingRekey`. A Delete never occupies it.

## Required Reading

| Document | Why |
|----------|-----|
| `rfc/short/rfc7296.md` | The checklist rows this spec unblocks |
| `rfc/full/rfc7296.txt` Section 2.3 | The two obligations, verbatim. The file is line-wrapped |
| `ai/rules/rfc-compliance.md` | Conformance is not negotiable, and who decides a deviation |
| `ai/rules/evidence.md` | Cite the producing function, never the caller |
| `internal/component/ike/engine/established.go` | The maintain tick that emits paths 1 to 3 |
| `internal/component/ike/engine/msgid.go` | The existing single-slot tracking |

## Current Behavior (MANDATORY)

Source files read on 2026-07-30, with every `file:line` confirmed against the tree:

- [ ] `internal/component/ike/engine/sa.go`
- [ ] `internal/component/ike/engine/msgid.go`
- [ ] `internal/component/ike/engine/established.go`
- [ ] `internal/component/ike/engine/dpd.go`
- [ ] `internal/component/ike/engine/rekey.go`
- [ ] `internal/component/ike/engine/inbound.go`
- [ ] `internal/component/ike/wire/payload_notify.go`

`dpdState.shouldSend` returns false only while `awaitReply` is set. The two rekey branches
return early only while `ps.pendingRekey` is non-nil. The two Delete senders build a
message, increment `sa.NextMsgID`, and call `sendRaw` unconditionally.

A peer therefore CAN receive two requests with consecutive message ids and no intervening
response. A conforming peer answers only within its window, so the second request goes
unanswered until the first completes. Ze reads that as a lost message and retransmits.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

The `maintainSA` tick (`internal/component/ike/engine/established.go`) and the
inbound handlers that send a Delete (`internal/component/ike/engine/inbound.go`,
`:263`). Format at entry is a decision to send, not yet any bytes.

### Transformation Path

A path decides to send. It reads and increments `sa.NextMsgID`
(`internal/component/ike/engine/sa.go`). A build function encodes the request.
`sendRaw` hands the bytes to `transport.UDPTransport`.

The response returns through `transport` to `classifyInbound`
(`internal/component/ike/engine/msgid.go`), which routes it to the owning handler.
That classifier knows about `pendingRekey` alone, so a response to a Delete or to a DPD
probe carries no slot to release.

The fix inserts one gate between "a path decides to send" and "the counter is read", and
one release between "a response is classified" and "the handler runs".

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| engine -> transport | out | The encoded request bytes, through `sendRaw` |
| transport -> engine | in | The peer's response, through `classifyInbound` |

No wire format change, and no configuration surface change.

### Integration Points

The rfcgate-1b pilot, now closed (`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`), owned at its phase 2b the rows `RFC7296-2.3-2`
and `RFC7296-2.3-4`. Both stay untagged until this spec closes.

## What other implementations do (evidence, not assumption)

The design below is not invented. It is what the two reference implementations do.

**strongSwan.** `src/libcharon/sa/ikev2/task_manager_v2.c` holds ONE shared queue for
every self-initiated task: `queued_tasks` and `active_tasks`. DPD, rekey, delete and
MOBIKE all compete for it. `initiate()` refuses to start a second exchange with
`if (this->initiating.type != EXCHANGE_TYPE_UNDEFINED)`. It then logs "delaying task
initiation". The request is QUEUED, never dropped and never sent anyway.

`process_response()` clears the type and calls `initiate()` again to drain the queue.
strongSwan models no window size at all, and it implements no SET_WINDOW_SIZE.

**libreswan.** Version 3.30 logs "received unsupported NOTIFY v2N_SET_WINDOW_SIZE", so it
does not implement the notify either.

**The conclusion for Ze.** A single shared slot with queue-and-defer is the conformant
behaviour that both references implement. SET_WINDOW_SIZE is implemented by neither, so
Ze gains no interoperability from it. This spec implements the slot, and does NOT
implement SET_WINDOW_SIZE.

## Key Design Decisions

**Superseded 2026-07-30 by the owner.** The row below that puts the release in
`classifyInbound` is void. That classifier runs before the message is authenticated,
so a forged datagram freed the window. That datagram named the right SPIs and the
exact outstanding message id. strongSwan does not allow it: `process_message`
compares the message id, then `parse_body` verifies the message. Only then does
`process_response` clear its slot.

The release now sits at two post-decrypt sites in `handleOwnedInbound`
(`inbound.go`), and both call one method, `SA.answerAuthenticatedResponse`. The
single-site property is traded for authentication. The method's doc comment carries
the two-site contract.

| Decision | Over | Because |
|----------|------|---------|
| One shared outstanding-request slot on the SA | Per-mechanism guards that consult each other | Four guards that each know about three others is the shape that produced this defect. One slot has one owner |
| Queue and defer a blocked request | Drop it | strongSwan defers. A dropped DPD probe delays liveness detection, and a dropped Delete leaks peer state |
| The slot is released when the response is classified | Release in each handler | `classifyInbound` (`engine/msgid.go`) already sees every inbound message and already owns `pendingRekey`. One release site cannot be forgotten by a new caller |
| No SET_WINDOW_SIZE | A negotiated window larger than 1 | Neither reference implementation supports it, so it buys no interoperability. Ze's window stays 1, which the RFC permits |
| A teardown Delete keeps its best-effort character | Blocking teardown on a free slot | `sendDeleteIKE` runs while the SA is being torn down. It waits for the slot when one is expected soon, and never blocks shutdown |

## Blast Radius

`internal/component/ike/engine` only. No wire format changes, so no interoperability risk
from the encoding. The observable change is that Ze sends fewer concurrent requests, which
a conforming peer already required.

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | Every self-initiated request reads `sa.NextMsgID` | A tree-wide grep on 2026-07-30 found exactly the four paths in the Task table | Re-run the grep at implementation, and gate on it in a test |
| A-2 | `classifyInbound` sees every response | It is the single classifier called from the inbound path | Read the callers first, then rely on it |
| R-1 | A deferred DPD probe delays liveness detection | The probe waits for the slot rather than sending | Bound the wait, and prove the probe still sends after the slot frees |
| R-2 | A deferred Delete on teardown is never sent | Shutdown does not wait for a slot | Keep the Delete best-effort, and prove teardown does not hang |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | A single outstanding-request slot exists on the SA, and every one of the four paths takes it, and only then reads `sa.NextMsgID` |
| AC-2 | A second self-initiated request, raised while the slot is held, is deferred and NOT sent |
| AC-3 | The deferred request is sent after the response to the first arrives |
| AC-4 | The slot is released by the inbound classifier, for a response to any request kind |
| AC-5 | Teardown does not hang when the slot is held. A Delete stays best-effort |
| AC-6 | `RFC7296-2.3-2` carries a positive AND a negative tagged test |
| AC-7 | `RFC7296-2.3-4` carries a positive AND a negative tagged test |
| AC-8 | A test proves the DPD-then-rekey tick from the Task table now emits ONE request, and it fails against the code as it stands today |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| `maintainSA` tick, DPD due AND lifetime soft-expired (`established.go`) | -> | the shared slot take | `TestWinOneRequestPerTick` |
| Inbound Delete while a probe is outstanding (`inbound.go`) | -> | the slot take on the unguarded path | `TestWinDeleteDefersWhileProbeOutstanding` |
| Response classified (`msgid.go`) | -> | the slot release | `TestWinResponseReleasesSlot` |

## 🧪 TDD Test Plan

Each test is written first and must fail against the current tree.

### Unit Tests

| Test | Proves |
|------|--------|
| `TestWinOneRequestPerTick` | AC-2, AC-8 |
| `TestWinResponseReleasesSlot` | AC-3, AC-4 |
| `TestWinDeleteDefersWhileProbeOutstanding` | AC-2 for the unguarded path |
| `TestWinTeardownDoesNotHang` | AC-5 |

### Functional Tests

The IPsec functional suite covers an established SA with rekey. It stays as the
regression net. A `.ci` test is the wrong instrument here. The property is which datagram
one tick emits, and no user-facing surface exposes that. The proof is a Go test that
drives `maintainSA` directly. The `.ci` suite proves the daemon still establishes and
rekeys with the slot in place.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/engine/sa.go` | The slot field |
| `internal/component/ike/engine/msgid.go` | Release the slot in `classifyInbound` |
| `internal/component/ike/engine/established.go` | The DPD and rekey branches take the slot |
| `internal/component/ike/engine/dpd.go` | The probe takes the slot |
| `internal/component/ike/engine/rekey.go` | Both rekey initiators take the slot |
| `internal/component/ike/engine/inbound.go` | Both Delete senders take the slot |

## Files to Create

| File | Holds |
|------|-------|
| `internal/component/ike/engine/rfc7296_window_test.go` | The tagged pairs for `RFC7296-2.3-2` and `RFC7296-2.3-4` |

## Implementation Steps

1. Write the failing tests from the TDD plan, and record the red output.
2. Add the slot to the SA, with a take and a release.
3. Release it in `classifyInbound`.
4. Route all four paths through the take.
5. Confirm each test goes green, then mutation-verify both tagged rows.
6. Add the two checklist rows to `rfc/short/rfc7296.md` with their tags in the same commit.

## Integration Points

The rfcgate-1b pilot, now closed (`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`), owned at its phase 2b the rows `RFC7296-2.3-2`
and `RFC7296-2.3-4`. Both stay untagged until this spec closes.

## Boundaries Crossed

Engine to transport only. No wire format change, and no configuration surface change.

## Entry Point

The `maintainSA` tick (`internal/component/ike/engine/established.go`) and the inbound
handlers that send a Delete (`internal/component/ike/engine/inbound.go`).

## Goal Gates

`make ze-verify`.

## Quality Gates

`make ze-lint-changed`, `make ze-rfc-check`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

`rfc/short/rfc7296.md` gains the two rows named in AC-6 and AC-7. No other summary changes.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The slot exists and all four paths take it
- [ ] The classifier releases it
- [ ] Tests PASS output pasted into the spec
- [ ] Both rows tagged and mutation-verified
- [ ] Rows added to `rfc/short/rfc7296.md` in the same commit as their tests
- [ ] `make ze-verify` green

### Tests FAIL, before the window existed

`go test -race -run TestWin ./internal/component/ike/engine/` against the tree of
2026-07-30, with the four tests present and no other change:

```
--- FAIL: TestWinOneRequestPerTick (1.04s)
    rfc7296_window_test.go:123: one tick that raised a probe and a rekey: ze wrote 208 unexpected bytes before the sentinel
--- FAIL: TestWinResponseReleasesSlot (0.04s)
    rfc7296_window_test.go:176: an ESP Delete while the probe is unanswered: ze wrote 80 unexpected bytes before the sentinel
--- FAIL: TestWinDeleteDefersWhileProbeOutstanding (0.04s)
    rfc7296_window_test.go:253: an IKE Delete while the probe is unanswered: ze wrote 80 unexpected bytes before the sentinel
--- FAIL: TestWinTeardownDoesNotHang (0.07s)
    --- FAIL: TestWinTeardownDoesNotHang/window_held_by_a_probe (0.04s)
        rfc7296_window_test.go:338: a graceful stop while a request is outstanding: ze wrote 80 unexpected bytes before the sentinel
FAIL
FAIL	github.com/ze-software/ze/internal/component/ike/engine	1.248s
```

Every failure is behavioral. The 208 bytes of the first test are the Child SA rekey
that rode out beside the DPD probe of one tick.

### Tests PASS, with the window in place

```
ok  	github.com/ze-software/ze/internal/component/ike/engine	2.246s
```

Whole component, `go test -race ./internal/component/ike/...`:

```
ok  	github.com/ze-software/ze/internal/component/ike/cmd	1.048s
ok  	github.com/ze-software/ze/internal/component/ike/crypto	(cached)
ok  	github.com/ze-software/ze/internal/component/ike/dataplane	(cached)
ok  	github.com/ze-software/ze/internal/component/ike/eap	(cached)
ok  	github.com/ze-software/ze/internal/component/ike/engine	7.030s
ok  	github.com/ze-software/ze/internal/component/ike/ipsec	(cached)
ok  	github.com/ze-software/ze/internal/component/ike/transport	(cached)
ok  	github.com/ze-software/ze/internal/component/ike/wire	(cached)
ok  	github.com/ze-software/ze/internal/component/ike/yang	(cached)
```

### Mutation verification of both rows

Mutation A neuters the reserve, which restores the behavior of the old tree.
`reserveRequestWindow` returns true always:

```
--- FAIL: TestWinOneRequestPerTick (1.03s)
    rfc7296_window_test.go:123: one tick that raised a probe and a rekey: ze wrote 208 unexpected bytes before the sentinel
--- FAIL: TestWinResponseReleasesSlot (0.03s)
    rfc7296_window_test.go:176: an ESP Delete while the probe is unanswered: ze wrote 80 unexpected bytes before the sentinel
--- FAIL: TestWinDeleteDefersWhileProbeOutstanding (0.03s)
--- FAIL: TestWinTeardownDoesNotHang (0.05s)
```

Mutation B removes the release. `classifyInbound` no longer calls
`answerRequestWindow`:

```
--- FAIL: TestWinOneRequestPerTick (1.03s)
    rfc7296_window_test.go:138: the rekey branch refused again after the window freed
--- FAIL: TestWinResponseReleasesSlot (5.03s)
    rfc7296_window_test.go:197: the deferred Delete never went out after the window freed
--- FAIL: TestWinDeleteDefersWhileProbeOutstanding (5.04s)
    rfc7296_window_test.go:287: the IKE Delete never went out after the window freed
```

`TestWinOneRequestPerTick` carries `RFC7296-2.3-2` and `TestWinResponseReleasesSlot`
carries `RFC7296-2.3-4`. Both rows fail under both mutations, so the positive half
needs the reserve and the negative half needs the release. Both mutations were
reverted, and the suite is green again.

### The release moved after authentication

The owner voided the `classifyInbound` release site on 2026-07-30. A test was added
first for the property it lost. It forges the answer to the outstanding probe. The
SPIs and the message id are right, and the last byte is flipped. The integrity check
therefore fails. Against the pre-authentication release the forgery freed the window:

```
--- FAIL: TestWinResponseReleasesSlot (0.04s)
    rfc7296_window_test.go:213: an ESP Delete after a forged answer: ze wrote 80 unexpected bytes before the sentinel
```

The release then moved to the two post-decrypt sites, and the suite went green.
The three mutations were re-run against the new shape:

| Mutation | Result |
|----------|--------|
| A, `reserveRequestWindow` never refuses | 5 tests red, both tagged rows among them |
| B, `answerAuthenticatedResponse` never frees | 3 tests red, both tagged rows among them |
| C, the release put back in `classifyInbound` | exactly 1 red, the forged-answer assertion |

Mutation C is the interesting one. Only the new assertion sees it. Nothing else in
the suite covered the difference between a real answer and a datagram that resembles
one. Every mutation was reverted.

## Known Limitations

Ze's window stays 1. A peer that offers a larger window with SET_WINDOW_SIZE gains nothing,
which matches strongSwan and libreswan. If a future peer makes that costly, the negotiation
is a separate spec and is not scoped here.

---

## Implementation Summary

### What Was Implemented
- One outstanding-request slot on the SA. `SA.reserveRequestWindow` and
  `SA.releaseRequestWindow` (`internal/component/ike/engine/msgid.go`) take and free it, and
  `reserveRequestWindow` also refuses an SA whose Message ID is exhausted.
- Seven non-test emitter sites take the slot before they read `sa.NextMsgID`:
  `startChildRekey` and `startIKERekey` (`established.go`), `sendDPD` (`dpd.go`), the three
  Delete senders (`delete.go`), and the INVALID_MESSAGE_ID emitter
  (`notify_invalid_msgid.go`). Every one of them defers rather than drops, and every one
  releases the slot when its build fails.
- `SA.answerAuthenticatedResponse` (`msgid.go`) frees the slot, and only when the answer
  names the outstanding request's Message ID. It is called at two POST-DECRYPT sites in
  `handleOwnedInbound` (`inbound.go`).
- `SA.armRequestRetransmit` / `shouldRetransmitRequest` / `noteRequestRetransmit` and
  `requestWindowStale` bound how long a slot can be held, so a lost answer frees it instead
  of silencing the SA forever.
- `rfc/short/rfc7296.md` gained rows `RFC7296-2.3-2` and `RFC7296-2.3-4`.
- `internal/component/ike/engine/rfc7296_window_test.go` holds both tagged pairs.

### Bugs Found/Fixed
- One `maintainSA` tick emitted a DPD probe AND a rekey request, at consecutive Message IDs
  with no answer to either. Covered by `TestWinOneRequestPerTick`, whose red output measured
  208 unexpected bytes: the rekey that rode out beside the probe.
- The Delete senders had no guard at all beyond `tr == nil`. Covered by
  `TestWinDeleteDefersWhileProbeOutstanding`.
- **A forged datagram freed the window.** The first shape released the slot in
  `classifyInbound`, which runs BEFORE authentication. A datagram naming the right SPIs and
  the exact outstanding Message ID, with one byte flipped, freed it. Found by the owner on
  2026-07-30 and fixed by moving the release to the two post-decrypt sites. Covered by the
  forged-answer case of `TestWinResponseReleasesSlot`, which was the ONLY assertion in the
  suite that saw mutation C.

### Documentation Updates
- `docs/features/rfc-status.md`'s RFC 7296 row already states the behaviour: "Section 2.3
  window: Ze declares a window of one and accepts exactly one request id... A peer request
  that crosses one of ours is accepted and answered, and a request outside the window is
  never acknowledged."
- No `docs/` claim is anchored to the changed producers:
  `grep -rln "source: internal/component/ike/engine/msgid.go" docs/` and the same for
  `delete.go` and `inbound.go` return nothing.

### Deviations from Plan
- **AC-4's release site MOVED, by owner ruling on 2026-07-30.** The spec's Key Design
  Decisions table names the release site as `classifyInbound`; that row is marked superseded
  in this spec. The release now sits at two post-decrypt sites in `handleOwnedInbound` and
  both call `SA.answerAuthenticatedResponse`. The single-site property was traded for
  authentication, and the method's doc comment carries the two-site contract so a third
  response path cannot silently skip it.
- The slot is taken at SEVEN sites, not the four the Task table enumerates. `delete.go`
  carries three senders rather than the two the table counted, and
  `notify_invalid_msgid.go` is an emitter the Task table predates.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The release was put in `classifyInbound`, which "already sees every inbound message" | `classifyInbound` runs before the message is authenticated, so a forged datagram naming the right SPIs and Message ID freed the window | The owner read the design and voided the row on 2026-07-30 | A test was written FIRST for the property being lost, then the release moved to two post-decrypt sites |
| assumption | The Task table said four emitter paths | Seven non-test sites read `sa.NextMsgID` for a self-initiated request | The tree-wide grep at implementation time (A-1's own validation) | All seven route through `reserveRequestWindow` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `RFC7296-2.3-2`: wait for a response before sending a subsequent message | Done | `msgid.go` `SA.reserveRequestWindow`, taken at 7 sites | |
| `RFC7296-2.3-4`: never exceed the peer's stated window size | Done | same slot; the window is 1 and is never widened | SET_WINDOW_SIZE is deliberately not implemented, matching strongSwan and libreswan |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `grep -rn "reserveRequestWindow" --include=*.go internal/component/ike/ \| grep -v _test.go` returns the definition plus 7 call sites, each immediately before the build that reads `sa.NextMsgID` | |
| AC-2 | Done | `TestWinOneRequestPerTick`, `TestWinDeleteDefersWhileProbeOutstanding` | Both assert the second request writes NOTHING, using a sentinel-byte probe rather than a bare absence |
| AC-3 | Done | `TestWinResponseReleasesSlot`: the deferred request goes out after the answer | Mutation B (remove the release) reddens exactly this |
| AC-4 | **Changed** | `SA.answerAuthenticatedResponse` (`msgid.go`), called at `inbound.go` lines 114 and 186, both after a successful `decryptAndParse` | The AC text says "released by the inbound classifier". Owner ruling of 2026-07-30 superseded it; see Deviations. The SUBSTANCE of AC-4 (a response to ANY request kind frees the slot) holds |
| AC-5 | Done | `TestWinTeardownDoesNotHang` | The Delete keeps its best-effort character |
| AC-6 | Done | `rfc7296_window_test.go` carries `RFC7296-2.3-2 positive` and `negative` | Mutations A and B both redden it |
| AC-7 | Done | same file carries `RFC7296-2.3-4 positive` and two `negative` tags | The second negative is the forged-answer case |
| AC-8 | Done | `TestWinOneRequestPerTick` failed against the pre-fix tree with "208 unexpected bytes" | Recorded verbatim under Tests FAIL |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestWinOneRequestPerTick` | Done | `internal/component/ike/engine/rfc7296_window_test.go` | |
| `TestWinResponseReleasesSlot` | Done | same file | Carries the forged-answer assertion |
| `TestWinDeleteDefersWhileProbeOutstanding` | Done | same file | |
| `TestWinTeardownDoesNotHang` | Done | same file | |
| `TestWinStaleWindowIsFreed` | Done (beyond plan) | same file | Covers `requestWindowStale`, added with the retransmit bound |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/sa.go` | Done | The slot fields and the exhaustion flag |
| `internal/component/ike/engine/msgid.go` | Done | Take, release, answer, retransmit bound |
| `internal/component/ike/engine/established.go` | Done | Both rekey branches, plus `serviceRequestWindow` |
| `internal/component/ike/engine/dpd.go` | Done | The probe takes the slot |
| `internal/component/ike/engine/rekey.go` | Done | Both initiators are reached through the `established.go` takes |
| `internal/component/ike/engine/inbound.go` | Done | The two release sites |
| `internal/component/ike/engine/delete.go` | Done (beyond plan) | The Delete senders moved out of `inbound.go` into their own file; all three take the slot |
| `internal/component/ike/engine/notify_invalid_msgid.go` | Done (beyond plan) | A seventh emitter the Task table predated |
| `internal/component/ike/engine/rfc7296_window_test.go` | Done | Created |

### Audit Summary
- **Total items:** 20 (2 requirements, 8 ACs, 5 tests, 9 files, counting the beyond-plan rows)
- **Done:** 19
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-4's release site, by owner ruling, recorded in Deviations and in the spec's Key Design Decisions)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One `maintainSA` tick must emit ONE request, never two | functional (Go, sentinel-byte transport) | `TestWinOneRequestPerTick`. Against the pre-fix tree it reported "ze wrote 208 unexpected bytes before the sentinel"; it passes now. Mutation A (never refuse) reddens it |
| The unguarded Delete path must respect the window | functional | `TestWinDeleteDefersWhileProbeOutstanding`. Pre-fix: "an IKE Delete while the probe is unanswered: ze wrote 80 unexpected bytes" |
| A deferred request must still go out, so the window is not a mute | functional | `TestWinResponseReleasesSlot`. Mutation B (never free) reddens it with "the deferred Delete never went out after the window freed" |
| Only an AUTHENTICATED answer may free the window | functional, negative | The forged-answer case of `TestWinResponseReleasesSlot`. Mutation C (release back in `classifyInbound`) reddens exactly one assertion in the whole suite, and it is this one |
| Teardown must not hang on a held slot | functional | `TestWinTeardownDoesNotHang`, both sub-cases |
| The daemon still establishes and rekeys with the slot in place | functional (`.ci`) | The `ipsec` suite, 8/8, recorded above. `ipsec-child-rekey` drives a real CREATE_CHILD_SA through a taken slot |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| (no shard) | n/a | The metadata row named `plan/deferrals/fixit-ike-request-window.md`, which never existed. Corrected to `-` on 2026-08-03. This spec deferred nothing |
| SET_WINDOW_SIZE negotiation | cancelled | Never in scope. Neither strongSwan (`task_manager_v2.c` models no window size) nor libreswan 3.30 ("received unsupported NOTIFY v2N_SET_WINDOW_SIZE") implements it, so it buys no interoperability. Recorded in Known Limitations as conditional on a peer that does not exist |

No shard is removed by this closure, because none exists. No FOREIGN shard was emptied by it.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | **none recorded -- the gate is NOT satisfied** |
| `review_gate.py check` | not run. The round produced a BLOCKER, so there is no clean pass to record |
| Reviewer lenses used | Three independent subagents through `/ze-review`: logic+wiring+removed-behaviour; security+edge-cases+test-vacuity; RFC 7296 conformance+interop |

**Round 1 scope, written before the round ran:** the whole five-spec IKE changeset at HEAD
(`git log --oneline -14 -- internal/component/ike/`), including the sibling call sites of
every changed function.

### Findings fixed
<!-- Only BLOCKER and ISSUE. Nothing here is fixed yet: see "CLOSURE REFUSED
     2026-08-03" below for the full text of each finding and the RFC sentence it
     rests on. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | A Delete is never remembered (`armRequestRetransmit` is not called) and is dropped outright when the window is held, against enrolled row `RFC7296-2.1-5` [MUST]. This spec created the drop | `internal/component/ike/engine/delete.go` `sendDeleteIKE`, `sendDeleteESP` | **Fixed 2026-08-04.** `writeDelete` (`delete.go`) arms the window's retransmit slot. The one caller that can meet a held window waits for the answer first (`sayGoodbye`, `established.go`) |
| 2 | NOTE | `sendIKESATeardown` spends a Message ID before it reserves the window | `internal/component/ike/engine/delete.go` `sendIKESATeardown` | **Fixed 2026-08-04.** `SA.requestWindowAvailable` (`msgid.go`) tests the window before `advanceMsgID`, and the reservation still runs after it |
| 3 | NOTE | The goodbye Delete is dropped whenever a liveness probe is in flight | `internal/component/ike/engine/delete.go` `sendDeleteIKE` | **Fixed 2026-08-04.** `sayGoodbye` (`established.go`) waits `goodbyeWindowWait` for the answer, then sends. It still sends nothing when the answer never comes, because RFC 7296 Section 2.3 forbids the second request |
| 4 | NOTE | `pendingIKESwap` is not cleared on `runEstablished` return, so an unconfirmed SA and its `SK_*` survive into the next reconnect cycle (RFC 7296 Section 2.12) | `internal/component/ike/engine/established.go` `runEstablished` | Not fixed. Not this spec's producer |
| 5 | BLOCKER | Found by the independent review OF the first fix. Abandoning the outstanding request before the goodbye is itself an RFC 7296 Section 2.3 violation: local bookkeeping does not make a second unanswered request legal | `internal/component/ike/engine/established.go` `maintainSA` stop branch | **Fixed 2026-08-04** by `sayGoodbye`. See the design note below |
| 6 | BLOCKER | Found by the same review. After `maxRequestRetransmits` the request is FORGOTTEN while the SA runs on. RFC 7296 Section 2.1 offers two exits and that is neither | `internal/component/ike/engine/established.go` `serviceRequestWindow` | **Fixed 2026-08-04.** The SA is deemed failed, which is the section's second exit |
| 7 | BLOCKER | Found by the same review. The deferral queue added by the first fix can NEVER fire: every non-test caller reaches a Delete sender with the window free | `internal/component/ike/engine/delete.go` `deferDelete` | **Fixed 2026-08-04** by deleting the queue. A mechanism that claims reliability and cannot deliver is worse than none |

### The design, after the review (2026-08-04)

The first attempt queued a refused Delete and freed the window before the goodbye. Two
independent reviewers refused both halves, and re-reading the RFC confirmed them.

RFC 7296 Section 2.3, `rfc/full/rfc7296.txt:1463`, verbatim:

<!-- ste: ignore -->
> An IKE endpoint MUST wait for a response to each of its messages before sending a subsequent message unless it has received a SET_WINDOW_SIZE Notify message from its peer

RFC 7296 Section 2.1, `rfc/full/rfc7296.txt:1359`, verbatim:

<!-- ste: ignore -->
> IKE is a reliable protocol: the initiator MUST retransmit a request until it either receives a corresponding response or deems the IKE SA to have failed.

Three consequences settle the design.

| Question | Answer |
|----------|--------|
| CAN the goodbye ride out beside an unanswered probe? | No. §2.3 binds the SENDER. `sa.releaseRequestWindow()` changes Ze's bookkeeping and not what Ze has received |
| What does the goodbye do instead? | Wait for the answer, bounded, then send. `sayGoodbye` waits `goodbyeWindowWait` (2s) over `ps.inbound`. A peer that has not answered in that time is the case DPD exists for, and a Delete it cannot answer is worth nothing |
| What happens when a request is never answered? | The SA is deemed failed (§2.1's second exit), not the request forgotten |

The deferral queue is gone. Its only refusal path in production was an exhausted Message
ID space. There `maintainSA` marks the SA dead before any drain runs, so the queue
fills and never empties.

### Fixed 2026-08-04

| Fix | Producer | Evidence |
|-----|----------|----------|
| The goodbye waits for the outstanding answer, then sends. It is not sent when that answer never comes | `PeerSession.sayGoodbye`, `PeerSession.waitForRequestWindow` (`established.go`) | `TestWinTeardownDoesNotHang`, three cases. Removing the wait reddens "window held and the peer answers" with "a graceful stop wrote no goodbye Delete". Sending regardless of the window reddens "window held and the peer stays silent" with "ze wrote 80 unexpected bytes before the sentinel" |
| Every Delete is remembered and repeated until the peer answers it (`RFC7296-2.1-5`) | `PeerSession.writeDelete` (`delete.go`), through `SA.armRequestRetransmit` | `TestWinDeleteIsRememberedUntilAnswered`. Removing the arm reddens it with "the SA remembered 0 bytes, want the 80 it sent" |
| An unanswered request fails the SA rather than being forgotten (`RFC7296-2.1-7`) | `PeerSession.serviceRequestWindow` (`established.go`) | `TestWinUnansweredRequestFailsTheSA`. Freeing the window without `StateDead` reddens it with "the unanswered request left the SA in state established, want StateDead" |
| A refused teardown notify spends no Message ID | `SA.requestWindowAvailable` (`msgid.go`), read by `sendIKESATeardown` (`delete.go`) | `TestWinRefusedTeardownSpendsNoMessageID`. Restoring the advance-first order reddens it with "the refused teardown moved NextMsgID from 3 to 4" |
| The window rule holds on a real wire against strongSwan | `startChildRekey` (`established.go`), `SA.reserveRequestWindow` (`msgid.go`) | `test/ipsec-interop/scenarios/24-delete-while-window-held`. With a probe of Ze's own unanswered, the Child SA soft trigger passes and Ze raises NO CREATE_CHILD_SA. Once the window frees, the same rekey goes out and its Delete follows. Removing the reservation reddens it with "Ze sent a CREATE_CHILD_SA rekey while its own liveness probe was unanswered" |
| `TestNegSharedSecretRefusesWrongLength` is tagged `RFC7296-3.4-1`, not `RFC7296-1.3-2` | `internal/component/ike/engine/rfc7296_negotiation_test.go` | The test calls `crypto.NewDHExchange(...).SharedSecret` alone. It builds no message and emits no Notify, so it cannot prove a row about the responder's INVALID_KE_PAYLOAD answer. `TestNegRekeyRejectsMismatchedKEGroup` keeps both polarities of `RFC7296-1.3-2`, asserting `NotifyInvalidKEPayload` and the two octets naming the group. Section 3.4 binds the SENDER, and this test proves the same length rule on RECEIPT. **One reviewer dissents** and would leave the test untagged, because §3.4 is a sender obligation whose real proof is `crypto/rfc7296_dh_test.go`. Thomas's instruction named `RFC7296-3.4-1`, so that is what is recorded. The dissent is his to settle |

**Not proven at interop: the goodbye's bounded wait.** Any construction against a live
strongSwan either frees the window before the clear reaches the engine, or holds it past
the two-second bound. The three cases are proven in Go with the mutation output above.

Both test edits carry the `rfc-test-change-approved: 2026-08-04` marker. Thomas's standing
authorisation of that date covers an edit that makes a test prove its RFC MORE faithfully.
Neither edit reduces what a test proves: the teardown case moved from asserting silence to
asserting the goodbye arrives and carries an IKE Delete payload.

**One defect outside this spec was fixed on the way, because the interop proof needed it.**
`ze cli` had never worked in the IPsec interop lab. No container ran `ze init`, and no
scenario's `ze.conf` started an SSH listener, so scenario `10-clear-reestablish` failed at
its first command with "no credentials for 127.0.0.1:2222". `lab.py` now carries `ze_cli`,
and scenarios 10 and 24 carry the account and the listener. Scenario 10 passes for the
first time.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/rfc7296_window_test.go` | Yes | `ls -la`; `grep -n "^func TestWin"` returns five tests |
| `internal/component/ike/engine/msgid.go` | Yes | Read on 2026-08-03; `reserveRequestWindow` at the `requestOutstanding \|\| msgIDExhausted` guard |
| `internal/component/ike/engine/delete.go` | Yes | Three `reserveRequestWindow` call sites read on 2026-08-03 |
| `internal/component/ike/engine/notify_invalid_msgid.go` | Yes | One `reserveRequestWindow` call site |
| `test/ipsec/ipsec-child-rekey.ci` | Yes | `ls test/ipsec/` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Every emitter takes the slot before reading `NextMsgID` | `grep -rn "reserveRequestWindow" --include=*.go internal/component/ike/ \| grep -v _test.go` -> 7 call sites (`established.go` 401, 426; `dpd.go` 132; `delete.go` 49, 84, 154; `notify_invalid_msgid.go` 91) plus the definition at `msgid.go` 142. The remaining hits are comments |
| AC-2, AC-8 | One tick emits one request | `--- PASS: TestWinOneRequestPerTick (1.04s)` |
| AC-3, AC-4 | The answer frees the slot and the deferred request goes | `--- PASS: TestWinResponseReleasesSlot (0.04s)` |
| AC-2 (Delete path) | The unguarded path now defers | `--- PASS: TestWinDeleteDefersWhileProbeOutstanding (0.04s)` |
| AC-5 | Teardown does not hang | `--- PASS: TestWinTeardownDoesNotHang (0.07s)` |
| AC-4 (post-decrypt) | Both release sites sit after authentication | Read `inbound.go`: line 114 is inside `if _, err := decryptAndParse(...); err == nil`, line 186 is after the main `decryptAndParse` succeeded and is guarded by `if isResponse` |
| AC-6, AC-7 | Both rows carry both polarities | `grep -rn "RFC7296-2.3-2 \|RFC7296-2.3-4 " --include=*.go` -> five tags in `rfc7296_window_test.go` |
| (bound) | A lost answer frees the slot rather than silencing the SA | `--- PASS: TestWinStaleWindowIsFreed (0.04s)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `maintainSA` tick, DPD due AND lifetime soft-expired -> the shared slot take | `test/ipsec/ipsec-child-rekey.ci` | Yes for the rekey half: the `.ci` drives a real CREATE_CHILD_SA between two daemons through `startChildRekey`, which now reserves the slot first. The DPD-and-rekey COLLISION itself is proven in Go by `TestWinOneRequestPerTick`, because no user-facing surface exposes which datagram one tick emitted |
| Inbound Delete while a probe is outstanding -> the slot take on the unguarded path | `test/ipsec/ipsec-clear-reestablish.ci` | Yes for the Delete path reaching the wire; the collision is proven in Go by `TestWinDeleteDefersWhileProbeOutstanding` |
| Response classified -> the slot release | `test/ipsec/ipsec-sa-installed.ci` | Read the file: it establishes and holds an SA, which requires every answered request to have freed the slot. The forged-answer half is proven in Go, since no `.ci` can forge an integrity failure |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | **broken, then corrected** | The grep found SEVEN sites, not four. `delete.go` carries three senders and `notify_invalid_msgid.go` is an eighth emitter the Task table predates. All seven take the slot. Recorded in Deviations and in the Mistake Log |
| A-2 | broken | `classifyInbound` does see every response, but it sees them BEFORE authentication, so it was the wrong place to release. Owner ruling 2026-07-30. Recorded in Deviations and in the Mistake Log |
| R-1 | confirmed and bounded | A deferred probe waits rather than sending. `dpd.lastSent` keeps its value so the next tick raises it again, and `requestWindowStale` bounds the wait |
| R-2 | confirmed | `TestWinTeardownDoesNotHang` proves shutdown does not wait for a slot; the Delete stays best-effort |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC compliance disclosure for Section 2.3 | `docs/features/rfc-status.md` RFC 7296 row: "Section 2.3 window: Ze declares a window of one and accepts exactly one request id" | Yes |
| No anchored `docs/` claim points at a changed producer | `grep -rln "source: internal/component/ike/engine/msgid.go" docs/`, and the same for `delete.go`, `inbound.go`, `dpd.go` -> nothing | Yes |
| Config surface | No change. The window is not configurable and SET_WINDOW_SIZE is deliberately absent | Yes |
| CLI / API / plugin SDK / wire format / comparison table / test infrastructure / architecture | No change. `show vpn ipsec sa`'s `peer-window-size` field predates this spec and reports what the PEER declared, which this spec does not touch | Yes |

## CLOSURE REFUSED 2026-08-03 -- the window made a Delete droppable, against an enrolled MUST

`/ze-close` re-verified AC-1 through AC-8 against their producing functions and put the
changeset to three independent reviewer subagents. **This spec is NOT closed.** Everything
recorded above holds: the slot exists, all seven emitters take it, both release sites sit
after authentication, and every AC has a passing tagged test. One finding below is a
MUST-level non-conformance on a path this spec created, and it must be settled first.

### BLOCKER 1 -- a Delete is neither remembered nor retransmitted, and is dropped outright

Row `RFC7296-2.1-5`, already enrolled at MUST level in `rfc/short/rfc7296.md`:

<!-- ste: ignore -->
> The initiator MUST remember each request until it receives the corresponding response.

`sendDeleteIKE` and `sendDeleteESP` (`internal/component/ike/engine/delete.go`) both reserve
the window, build, `sendRaw`, and never call `sa.armRequestRetransmit`. Its only non-test
caller in the tree is `notify_invalid_msgid.go`. Both also `return` silently when the window
is held:

```
if !sa.reserveRequestWindow() {
        log.Debug("ike: ESP delete dropped, a request is outstanding", "peer", ps.peerName)
        return
}
```

**Before this spec, `sendDeleteESP` sent unconditionally.** The drop is new, and this spec
created it. A make-before-break ESP Delete lost this way leaves the peer's retired Child SA
and its policy installed until its own lifetime expires.

The justification on record is the fifth Key Design Decision row, "A teardown Delete keeps
its best-effort character", and the comment in `msgid.go` calling a Delete "never resent".
Per `ai/rules/rfc-compliance.md` a comment is its author's belief and not a decision record,
and no owner ruling was taken on this against the enrolled MUST. Row `RFC7296-2.1-5`'s
tagged test (`rfc7296_retransmit_test.go`) covers the generic window slot, so the Delete
path is untagged and unproven against the requirement it is supposed to meet.

Two fixes are on the table and the choice is the owner's: arm the retransmit slot for the
Delete so the window's own bounded retransmission carries it, or queue the Delete behind the
held window instead of dropping it, as strongSwan's `queued_tasks` does. Dropping it is not
one of them.

### NOTE 2 -- `sendIKESATeardown` spends a Message ID before it reserves

`internal/component/ike/engine/delete.go` `sendIKESATeardown` calls `sa.advanceMsgID()` and
then `reserveRequestWindow()`. A refused reservation returns with the id spent and nothing
sent. Latent today: both callers sit in `handleAuthResponse`, where no window is ever held,
and the SA dies immediately after. Reserve first, then advance.

### NOTE 3 -- the goodbye Delete is dropped whenever a liveness probe is in flight

`sendDeleteIKE` refuses on a held window, and `TestWinTeardownDoesNotHang` pins that as
intended. The SA is being destroyed, so the probe's answer is worthless:
`sa.retireRequest(dpd.probeMsgID)` before the goodbye would keep the window rule and still
say goodbye. Related to BLOCKER 1 and probably settled by the same decision.

### NOTE 4 -- `pendingIKESwap` outlives its session

`runEstablished` (`internal/component/ike/engine/established.go`) clears `ownedSA` and
`pendingRekey` on return but never `ps.pendingIKESwap`. `PeerSession` is reused across
reconnect cycles (`reconcile.go`), so an SA built by `respondIKERekey` and never confirmed
survives into the next cycle, and its `SK_*` survive with it, against RFC 7296 Section 2.12.
`handleInformationalOwned` would then answer the next peer IKE Delete by swapping the owner
loop onto an SA from the previous session. Reported by the logic lens; not this spec's
producer, and recorded here because this spec is the one still open over that file.

## Core Insight

Four guards that each knew about three others was the shape that produced the defect, and
it is the shape a fifth emitter silently re-creates. One slot with one owner cannot be
forgotten by a new caller in the same way, but it moved the risk rather than removing it:
the new failure mode is a response path that does not free the slot. That is why
`answerAuthenticatedResponse`'s doc comment names the two-site contract explicitly and says
what a third path owes.

The sharper lesson is about WHERE a slot may be freed. "The one place that sees every
message" and "the one place that has authenticated the message" are different places, and
the first is the tempting one because it is single. Releasing a security-relevant resource
before authentication hands the attacker the resource. Only one assertion in the whole
suite could see the difference between a real answer and a datagram that resembled one.
