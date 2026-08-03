# Spec: fixit-ike-request-window

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; nothing deferred; window-size negotiation was never in scope and is conditional on a peer that does not exist. Create `plan/deferrals/fixit-ike-request-window.md` on the first deferral) |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 by phase 2b of `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. An agent
tried to write tagged tests for `RFC7296-2.3-2` and `RFC7296-2.3-4`. The agent
refused to tag either row. A test bound to any one guard stays green while the
endpoint exceeds the window through the other paths. The supervising session then read
every producer named below and confirmed the trace. Owner ruling taken the same day:
fix it in this named spec, and implement it now.

## Task

**Ze holds more than one IKE request outstanding, and it never negotiates a window that
would allow that.** RFC 7296 Section 2.3 carries two obligations, and Ze meets neither.

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
