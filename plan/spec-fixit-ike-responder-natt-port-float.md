# Spec: fixit-ike-responder-natt-port-float

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/1 (defect 2 only; defect 1 landed in RFC 7296 pilot WP-8) |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; nothing deferred; the spec records no out-of-scope decision. Create `plan/deferrals/fixit-ike-responder-natt-port-float.md` on the first deferral) |
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-31. It is the first time the responder EAP interop scenario was able to start.

## Task

**NARROWED 2026-07-31 to defect 2. Defect 1 landed in RFC 7296 pilot WP-8.**

~~**Defect 1: Ze gates the NAT-T reply form on its OWN NAT verdict, not the peer's.**~~
Landed in the rfcgate-1b RFC 7296 pilot spec, WP-8. The rows are `RFC7296-2.11-2`,
`RFC7296-2.11-3` and `RFC7296-2.23-8`.

`sendWithNATT` is gone. `sendReply` (`internal/component/ike/engine/eap_auth.go`) now
derives the reply form from the socket the request ARRIVED on. It sends to the observed
source verbatim. `sendRaw` (`engine/established.go`) picks the SA's own socket from its
float verdict.

**The claim about `dispatchNATTInbound` above was STALE, and a reader must not act on it.**
It was re-read on 2026-07-31. `dispatchNATTInbound` is launched with `trNATT`
(`engine/register.go`, `go dispatchNATTInbound(trNATT, table, log)`). It passes that same
transport to `tryResponderSAInit` and to `routeInbound`. The pre-adoption responder path
therefore always held the 4500 socket.

The real loss was in `routeInbound`. It dropped the arrival transport when it handed a
packet to an OWNED SA's owner loop. The session's own transport is always the port-500
one. WP-8 carries the arrival role on `transport.Packet` instead.

**Defect 2: a retransmitted IKE_AUTH kills the SA instead of replaying the cached response.**
STILL OPEN, and it is why this spec stays. `handleResponderEAP`
(`engine/responder_eap.go`, the `eapPayload == nil` branch that logs
`ike: EAP round missing EAP payload`) finds no EAP payload and no AUTH payload on the
retransmit, and sets `StateDead`. RFC 7296 Section 2.1 makes a retransmission normal, and
the responder is required to be able to answer it.

Defect 2 is a defect on its own terms, whatever causes the retransmit.

WP-8 did not absorb it. It is an RFC 7296 Section 2.1 retransmission obligation. It
touches none of WP-8's producers. It has no row among the pilot's 113, so absorbing it
would have shipped an untagged protocol fix. That choice is recorded as OR-WP8-3.

## Observed sequence

Verified facts from the scenario run:

| Step | Evidence |
|------|----------|
| Ze answers IKE_SA_INIT | strongSwan parses it and selects the proposal |
| Ze reaches `responder EAP started` | `engine/responder_eap.go`, reachable only after IKE_AUTH #1 arrives and IDr, CERT, AUTH and the EAP request are sent |
| strongSwan never reports parsing that response | its log stops after `sending cert request` |
| 4.001 seconds later Ze logs `EAP round missing EAP payload` | `responder_eap.go`, then `StateDead` |
| later packets are dropped | `no SA for NAT-T packet`, `engine/register.go` |
| `swanctl --list-sas` | both endpoints on port 4500, SA stuck `CONNECTING` |

**Control experiment.** Scenario 03, same containers and the same PKI, with Ze as the
INITIATOR: `swanctl --list-sas` shows both endpoints on port 500, and the scenario passes.
So the float is specific to Ze's responder path.

## The mechanism reading, labelled as a reading

The producer was read. Packets were NOT captured, so this is a reading rather than a
measurement, and the fix must confirm it.

A peer that has floated receives, on 4500, a datagram whose first four octets are the IKE
SPI rather than zeros. It classifies that as ESP and drops it. It writes no log line. That matches
strongSwan's silence exactly. Its retransmit of IKE_AUTH #1 then reaches defect 2.

## Required Reading

| Document | Why |
|----------|-----|
| RFC 3948 Section 2.2 | The non-ESP marker, and when it is required |
| RFC 7296 Sections 2.1 and 2.23 | Retransmission, and NAT traversal |
| `internal/component/ike/engine/eap_auth.go` | `sendWithNATT` and the gate |
| `internal/component/ike/engine/register.go` | `dispatchNATTInbound` and the transport it hands on |

## Current Behavior (MANDATORY)

Source files read on 2026-07-31:

- [ ] `internal/component/ike/engine/eap_auth.go`
- [ ] `internal/component/ike/engine/register.go`
- [ ] `internal/component/ike/engine/responder_eap.go`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An IKE_AUTH request arriving on UDP 4500, dispatched by `dispatchNATTInbound`
(`register.go`).

### Transformation Path

`dispatchNATTInbound` accepts the datagram and passes the handler a transport bound to 500.
The handler builds its response and calls `sendWithNATT`, which reads `sa.NATDetected`.
That is false, so no non-ESP marker is written and the port-500 transport is used. The peer,
which floated, never recognises the reply.

The reply form must follow the port the request ARRIVED on, and the peer's float, rather
than Ze's own NAT verdict.

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| peer -> `dispatchNATTInbound` | in | An IKE_AUTH on 4500, with the non-ESP marker |
| handler -> `sendWithNATT` | out | The response, today without the marker and from the wrong socket |

### Integration Points

Scenario 08 of `test/ipsec-interop` is the gate. Scenario 03 is the control and passes.

## Key Design Decisions

| Decision | Over | Because |
|----------|------|---------|
| Derive the reply form from the received datagram | Reading `sa.NATDetected` | The peer's float is a fact about the peer. Ze's NAT verdict answers a different question |
| Replay the cached response for a retransmit | Setting `StateDead` | RFC 7296 Section 2.1 makes a retransmission normal, and the responder must answer it |
| Confirm with a packet capture | Trusting the reading above | The mechanism is inferred from logs and code. A capture settles it |

## Blast Radius

The responder NAT-T path and the responder EAP retransmit path. The initiator path is
unaffected, which the scenario 03 control shows.

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | The peer floats and Ze does not follow | `swanctl` shows 4500 both ends where the control shows 500 | A capture on the peer side |
| A-2 | The silent drop is the peer classifying the reply as ESP | strongSwan logs nothing at all for that datagram | The same capture |
| R-1 | Fixing the reply form CAN change the non-NAT path, which works today | Both paths share `sendWithNATT` | Scenario 03 must keep passing |

A-1, A-2 and R-1 are all defect 1's, and defect 1 landed in the RFC 7296 pilot WP-8.
A-1 and A-2 need no capture now. The charon log of scenario 08 carries the whole IKE_AUTH
exchange on 4500 in both directions, which is the float they describe, and `sendReply`
(`engine/eap_auth.go`) follows the arrival socket. R-1 is validated: scenario 03 passes.

| Id | Statement | Basis | Status |
|----|-----------|-------|--------|
| A-3 | The cached-response machinery is already populated on the EAP path, so only the CONSULT is missing | `cacheResponse` is called by both `startResponderEAP` and `sendResponderEAP` (`engine/responder_eap.go`) | confirmed |
| A-4 | `sendRaw` cannot serve the mid-EAP replay, because the SA holds no endpoint yet | `adoptAuthenticatedEndpoint` is the only writer of `sa.peerEndpoint` (`engine/sa.go`), and `handleAuthRequest` (`engine/responder.go`) returns at its `startResponderEAP` call before it reaches that write | confirmed |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| ~~AC-1~~ | ~~A reply to a request received on 4500 carries the non-ESP marker and leaves from 4500~~ -- landed in RFC 7296 pilot WP-8, proven by `TestNattReplyLeavesFromTheArrivalSocket` |
| ~~AC-2~~ | ~~The reply form follows the received datagram, never Ze's own NAT verdict~~ -- landed in RFC 7296 pilot WP-8. `sendReply` reads the arrival socket |
| AC-3 | A retransmitted IKE_AUTH is answered with the cached response, and does not set `StateDead` |
| AC-4 | Scenario 08 establishes, and scenario 03 still passes -- KEPT, because defect 2 alone still blocks scenario 08 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| ~~IKE_AUTH on 4500~~ | -> | ~~the reply-form decision~~ | landed in RFC 7296 pilot WP-8 |
| a retransmitted IKE_AUTH (`handleResponderInbound`, `engine/responder.go`) | -> | `replayCachedResponse` | `TestEapRtxResponderReplaysCachedResponseMidEAP` (`engine/rfc7296_eap_retransmit_test.go`), plus interop scenario 08 through the running daemon |

**The `.ci` named here until 2026-08-14 was `test/ipsec/ipsec-responder-retransmit.ci`, and it
is deliberately not written.** The skill permits the wiring test to be a `.ci` or a `_test.go`,
and the substitution is the stronger proof rather than the cheaper one.

Driving a mid-EAP retransmit ze-to-ze needs a LOST response, and the IKE test seams are
`ze.test.ike.port` and `ze.test.ike.dataplane` (`engine/testport.go`). Neither drops a
datagram. A `.ci` would therefore have needed a new fault-injection seam in the transport,
built to obtain evidence weaker than what already exists:
`TestEapRtxResponderReplaysCachedResponseMidEAP` drives the real entry point,
`ps.handleResponderInbound`, with the real IKE_AUTH bytes a real initiator sent, and interop
scenario 08 proves the same path through the running daemon against strongSwan, which no
ze-to-ze `.ci` can do. Adding a product seam to weaken the proof is the wrong trade.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| ~~The reply form follows the received port, both ways~~ | ~~AC-1, AC-2~~ -- landed in RFC 7296 pilot WP-8 |
| `TestEapRtxResponderReplaysCachedResponseMidEAP` | AC-3. The retransmit is answered from cache and the SA lives |
| `TestEapRtxMidEAPReplayRefusesUnprotected` | AC-3's guard: a forged bare header draws no answer and does not kill the SA |
| `TestEapRtxMidEAPReplayIsRateLimited` | AC-3's second guard: the burst bound reaches the mid-EAP site too |

All three are in `internal/component/ike/engine/rfc7296_eap_retransmit_test.go` and all three
call `ps.handleResponderInbound` directly, so they drive the entry point rather than the
helper (`ai/rules/evidence.md`).

### Functional Tests

| Test | Role |
|------|------|
| ~~`test/ipsec/ipsec-responder-natt-reply.ci`~~ | ~~AC-1 and AC-2 through the daemon~~ -- superseded by the WP-8 unit pair in `internal/component/ike/engine/rfc7296_natt_test.go` |
| ~~`test/ipsec/ipsec-responder-retransmit.ci`~~ | ~~AC-3 through the daemon~~ -- not written. See the Wiring Test note above: there is no drop seam to lose a response with, and scenario 08 proves the same path against a real peer |
| `test/ipsec-interop/scenarios/08-responder-eap-mschapv2` | AC-3 and AC-4 against a real peer |

The interop scenario is the one that proves strongSwan agrees, and it is the only tier that
can: it needs charon to decide on its own to retransmit into a live EAP exchange.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/engine/eap_auth.go` | Derive the reply form from the received datagram |
| `internal/component/ike/engine/register.go` | Hand the handler the transport the request arrived on |
| `internal/component/ike/engine/responder_eap.go` | Replay the cached response for a retransmit |

## Implementation Steps

1. Capture the packets first, and confirm or refute the reading above. Record which.
2. Write the reply-form tests, and record the red.
3. Derive the form from the received datagram.
4. Write the retransmit test, and record the red.
5. Replay the cached response.
6. Run scenarios 08 and 03.

## Evidence (defect 2)

The fix is one call. `handleResponderInbound` (`engine/responder.go`) now runs
`replayCachedResponse` before `handleResponderEAP` in the `StateEAPInProgress` arm, and
the `StateEstablished` arm calls the same function instead of its own inline copy.

Unit, with the fix reverted:

```
--- FAIL: TestEapRtxResponderReplaysCachedResponseMidEAP (0.02s)
    rfc7296_eap_retransmit_test.go:121: a retransmitted IKE_AUTH killed the IKE SA mid-EAP
--- FAIL: TestEapRtxMidEAPReplayRefusesUnprotected (0.02s)
    rfc7296_eap_retransmit_test.go:174: state after the forgery = dead, want EAP in progress
--- FAIL: TestEapRtxMidEAPReplayIsRateLimited (0.02s)
    rfc7296_eap_retransmit_test.go:218: a mid-EAP duplicate drew no answer at all, so the bound below proves nothing
```

Unit, with the fix in place:

```
ok  	github.com/ze-software/ze/internal/component/ike/engine	24.493s
```

Interop, scenario 08 with the fix reverted:

```
✗ FAIL: ze re-processed the retransmitted IKE_AUTH ('EAP round missing EAP payload')
  instead of replaying its cached response, against RFC 7296 Section 2.1
```

Interop, scenario 08 with the fix in place:

```
✓ the IKE SA survived the mid-EAP retransmissions and established (RFC 7296 Section 2.1)
✓ strongSwan retransmitted its IKE_AUTH into the live EAP exchange (2 retransmitted requests, was 1)
✓ PASS
```

Scenario 08 needed a second correction before it could reach the retransmit at all. Its
`swanctl.conf` set `eap_id` and no `id`, so charon asserted its IP in IDi while ze's
`remote-id` named `testuser`. `checkRemoteIdentity` (`engine/remote_id.go`) compares
`remote-id` against the IDi payload, which is what RFC 7296 Section 3.5 governs, so ze was
right and the fixture was inconsistent. The fixture now sets `id = testuser`.

## Goal Gates

`make ze-verify`, plus scenarios 03 and 08 of `make ze-ipsec-interop-test`.

## Quality Gates

`make ze-lint-changed`, `make ze-ipsec-test`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

`RFC7296-2.23-*` covers NAT traversal and `RFC7296-2.1-*` covers retransmission. Check
`rfc/short/rfc7296.md` for an existing id first, and remember
`check_id_allocation` refuses an ordinal at or below a section's high-water mark.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The packet capture confirms or refutes the mechanism reading
- [ ] The reply form follows the received datagram
- [ ] A retransmit is answered rather than fatal
- [ ] Tests PASS output pasted into the spec
- [ ] Scenario 08 establishes and scenario 03 still passes
- [ ] `make ze-verify` green

---

## Implementation Summary

### What Was Implemented

Defect 2 only. Defect 1 landed in the RFC 7296 pilot WP-8 and its struck-through text
above is history, not open work.

- `replayCachedResponse` (`internal/component/ike/engine/responder.go`) is new. It answers
  a retransmitted request from `sa.lastResponse` on the pre-adoption dispatch path, and it
  reports whether the message id names the cached response. A true result means the caller
  MUST NOT process the message again, whether or not a datagram went back.
- `handleResponderInbound` (same file) calls it from BOTH pre-adoption arms.
  `StateEAPInProgress` had no guard at all and now returns early on true, before
  `handleResponderEAP` can decrypt a duplicate and kill the SA. `StateEstablished` had an
  inline copy and now calls the same function.
- No new mechanism. `cacheResponse` (`engine/msgid.go`) was ALREADY called on the EAP path
  by `startResponderEAP` and `sendResponderEAP` (`engine/responder_eap.go`). Only the
  consult was missing, which is why the fix is one call rather than a cache.
- The comment at the sibling replay site in `handleOwnedInbound` (`engine/inbound.go`) was
  repointed at the lifted function.

### Bugs Found/Fixed

- **The defect itself**: a retransmitted IKE_AUTH mid-EAP reached `handleResponderEAP`,
  found neither an EAP nor an AUTH payload, and took one of six paths to `StateDead`.
  Covered by `TestEapRtxResponderReplaysCachedResponseMidEAP`.
- **A forged 28-byte header killed a live IKE SA**, which is worse than the amplification
  the established arm's guards were written for: `handleResponderEAP` tried to decrypt the
  forgery, failed, and set `StateDead`. Both guards now cover the EAP arm. Covered by
  `TestEapRtxMidEAPReplayRefusesUnprotected`.
- **Scenario 08 fixture defect**: `swanctl.conf` set `eap_id` and no `id`, so charon
  asserted its IP in IDi while ze's `remote-id` named `testuser`. `checkRemoteIdentity`
  (`engine/remote_id.go`) compares `remote-id` against the IDi payload, which is what
  RFC 7296 Section 3.5 governs, so ze was right and the fixture was inconsistent. The
  fixture now sets `id = testuser`. Without this the scenario never reached the retransmit.

### Documentation Updates

None required, and the greps that prove it are in Pre-Commit Verification below. The change
adds no config leaf, no CLI surface, no metric and no runtime dependency. It corrects a
state-machine arm behind an existing documented behavior.

Two source comments citing this spec by path were restated inline before closure, because
commit B removes the file they name: `internal/component/ike/engine/sa.go` (`localPort`) and
`internal/component/ike/engine/child.go` (the `NATDetected`/`localPort` split).

### Deviations from Plan

| Deviation | Why |
|-----------|-----|
| `test/ipsec/ipsec-responder-retransmit.ci` was not written | No drop seam exists to lose a response with (`engine/testport.go` exposes `ze.test.ike.port` and `ze.test.ike.dataplane` only). The unit test drives the real entry point with real handshake bytes and scenario 08 proves the path against strongSwan. Building a product seam to obtain weaker evidence is the wrong trade. Recorded in the Wiring Test section |
| Implementation Step 1 (capture packets first) was not done | Defect 1 owned that step, and defect 1 landed in WP-8. A-1 and A-2 are validated from the scenario 08 charon log instead, which carries the whole IKE_AUTH exchange on 4500 in both directions |
| `sendRaw` was not used for the replay | A-4: the SA holds no endpoint mid-EAP, so `sa.remoteUDPAddr` falls back to the CONFIGURED remote address, which for a NATed peer is never the mapped port |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spec's Wiring Test row assumed a `.ci` could drive a mid-EAP retransmit ze-to-ze | A retransmit needs a LOST response, and the IKE test transport has no drop seam | Writing the test | Substituted the entry-point unit test plus interop scenario 08, and recorded the reason in the Wiring Test section rather than leaving the row naming a file nobody wrote |
| approach | The fix was first read as needing its own cache on the EAP path | `cacheResponse` was already called by `startResponderEAP` and `sendResponderEAP`. Only the CONSULT was missing | Reading the producers before writing code (A-3) | The fix is one call. A second cache would have been a mechanism where an extension was enough |
| escalation | A sentence was reflowed to satisfy the STE run-on check, and the reflow INVENTED a claim: that `localPort` records what the peer did and `NATDetected` records ze's own verdict | `detectResponderNAT` (`engine/responder.go`) and two `fsm.go` branches set `NATDetected` and float the port on the SAME line, so the port is set from ze's own verdict too. The true relation is one-way implication | Review round 3, by enumerating every writer of `sa.localPort` | Corrected at both sites. **The general lesson: a reflow is a REWRITE.** An edit made for style still restates a claim, and a restated claim needs its producer read again. Style tooling makes that edit feel free, and it is not: this one put a false statement into shipped code and cost a review round |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Defect 1: reply form follows the peer's float, not ze's NAT verdict | Changed | `sendReply` (`engine/eap_auth.go`), `sendRaw` (`engine/established.go`) | Landed in RFC 7296 pilot WP-8 as `RFC7296-2.11-2`, `RFC7296-2.11-3`, `RFC7296-2.23-8`. Struck through in the Task section. NOT this spec's deliverable |
| Defect 2: a retransmitted IKE_AUTH is answered, not fatal | Done | `replayCachedResponse`, and the `StateEAPInProgress` arm of `handleResponderInbound` (`engine/responder.go`) | The whole of this spec |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Changed | `TestNattReplyLeavesFromTheArrivalSocket` | Landed in WP-8, struck through |
| AC-2 | Changed | `sendReply` reads the arrival socket | Landed in WP-8, struck through |
| AC-3 | Done | `TestEapRtxResponderReplaysCachedResponseMidEAP` asserts `resp.State != StateDead` and `bytes.Equal(replay, first)` | The byte-equality is what separates a REPLAY from a rebuild: every build draws a fresh random CBC IV (`engine/auth.go`), so a rebuilt response cannot match |
| AC-4 | Done | Scenario 08 passes; scenario 03 still passes | Scenario 03 is the control and is unchanged by this work |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| A retransmit replays rather than kills | Done | `TestEapRtxResponderReplaysCachedResponseMidEAP` | Drives `ps.handleResponderInbound` |
| (added) a forgery draws no answer | Done | `TestEapRtxMidEAPReplayRefusesUnprotected` | Not in the original plan. The forgery path was worse than amplification here |
| (added) the burst bound reaches the mid-EAP site | Done | `TestEapRtxMidEAPReplayIsRateLimited` | Carries a floor so the ceiling cannot pass vacuously |
| `test/ipsec/ipsec-responder-retransmit.ci` | Changed | -- | Substituted; see Deviations and the Wiring Test note |
| Scenario 08 | Done | `test/ipsec-interop/scenarios/08-responder-eap-mschapv2/check.py` | Passes for the first time |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/eap_auth.go` | Changed | Defect 1's file. Landed in WP-8, untouched here |
| `internal/component/ike/engine/register.go` | Changed | Defect 1's file. WP-8 carries the arrival role on `transport.Packet` instead |
| `internal/component/ike/engine/responder_eap.go` | Changed | Not edited. The producer read confirmed A-3: the cache was already populated, so the fix landed at the CONSULT in `responder.go` |
| `internal/component/ike/engine/responder.go` | Done | Not in the original list. It is where the consult belongs |
| `internal/component/ike/engine/inbound.go` | Done | Not in the original list. Comment repointed at the lifted function |

### Audit Summary

- **Total items:** 16
- **Done:** 10
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 6 (five are defect 1's, landed in WP-8 and struck through in the Task section before implementation began; one is the `.ci` substitution, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A retransmitted IKE_AUTH does not kill the IKE SA (RFC 7296 Section 2.1) | Unit, discriminating | `TestEapRtxResponderReplaysCachedResponseMidEAP`. Reverted, it reports `a retransmitted IKE_AUTH killed the IKE SA mid-EAP` |
| The response is REPLAYED, not rebuilt | Unit, discriminating | Same test, `bytes.Equal(replay, first)`. A rebuild cannot match: fresh random CBC IV per build (`engine/auth.go`) |
| The replay is not a spoofable amplifier | Unit, discriminating | `TestEapRtxMidEAPReplayRefusesUnprotected`. Reverted, it reports `state after the forgery = dead, want EAP in progress`. Guards: `carriesSKPayload` then `cachedReplayAllowed`, both in `engine/notify_error.go`, at `cachedReplayRate` 1/s and `cachedReplayBurst` 3 |
| The bound reaches the mid-EAP site, not just the established one | Unit, discriminating | `TestEapRtxMidEAPReplayIsRateLimited`. Reverted, it reports `a mid-EAP duplicate drew no answer at all, so the bound below proves nothing` |
| A real peer's retransmit is served (interop) | Interop, discriminating | Scenario 08: `the IKE SA survived the mid-EAP retransmissions and established (RFC 7296 Section 2.1)` and `strongSwan retransmitted its IKE_AUTH into the live EAP exchange (2 retransmitted requests, was 1)`. Reverted, it reports `ze re-processed the retransmitted IKE_AUTH ... instead of replaying its cached response` |
| The initiator path is unaffected (R-1) | Interop, control | Scenario 03 still passes, unchanged |

Every row's evidence is a test that FAILS when the behavior is removed. The revert output is
pasted in the Evidence section above, per `ai/rules/interop-and-goal-validation.md` "Prove
the test discriminates".

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| (none) | n/a | The spec metadata names no shard, and `plan/deferrals/fixit-ike-responder-natt-port-float.md` does not exist (`ls`, 2026-08-14). The metadata line was corrected on 2026-08-03: the row had named a shard that never existed. Nothing was deferred, so no shard is created and none is removed |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ike-responder-natt-port-float-ca112cd4-8337-4992-b4e1-e0d7bbff5820.md` |
| `review_gate.py check` | OK, clean, hashes match |
| Rounds | 5. Each round reviewed only what the previous round's fixes changed. Rounds 4 and 5 were each earned by a PRODUCT defect, not a record one: finding 9 (a false claim about `localPort` that a round-3 reflow put into shipped code) and finding 12 (an inverted implication in `ChildSA.UDPEncap` that fixing finding 9 exposed) |
| Reviewer lenses used | Round 1: logic + wiring + removed-behavior + RFC compliance, and separately security + test discrimination + edge cases + simplicity. Round 2: the round-1 fixes. Round 3: the four files changed after round 2. Round 4: the three comment corrections round 3 drove. Round 5: the two corrections round 4 drove |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `rfc/requirements/rfc7296.md` was committed STALE, so a verify-tier gate was red at HEAD. The `RFC7296-2.1-3` row cited line 147 for the negative-polarity tag, which sits at line 150. `rfc_requirements.py --check-fresh` exited 1, and `TestRFCLedgerFresh` runs exactly that | `rfc/requirements/rfc7296.md`, produced by `scripts/dev/rfc_requirements.py` | `make ze-rfc-index`. The diff is that one line. `--check-fresh` now reports 177 shards up to date, and `make ze-rfc-check` exits 0 |
| 2 | ISSUE | A FALSE RFC claim in shipped code. `replayCachedResponse` justified its guards with "RFC 7296 Section 2.21.4 MUST NOT: an unprotected message draws no response". Section 2.21.4 says the opposite for an unprotected REQUEST: the node "MAY send a response". Its MUST NOTs govern a message marked as a RESPONSE and a peer receiving an unprotected INVALID_IKE_SPI Notify, neither of which is this case | `replayCachedResponse` (`engine/responder.go`), and the same over-generalization at the sibling site in `handleOwnedInbound` (`engine/inbound.go`) | Both comments now cite `RFC7296-2.4-12` MUST, which is the obligation both guards actually serve, and both say plainly that refusing is permitted rather than compelled. Behavior unchanged: refusing was always conformant. Fixed at BOTH sites, because the sibling carried the same defect |
| 3 | ISSUE | The interop VACUITY GUARD could itself pass vacuously. `check()` differences two whole-log `"retransmit 1 of request"` counts taken either side of the blackout, and charon block-buffers stdio, so a part-one retransmit not yet flushed at the baseline read would surface in the after-read and satisfy the guard with a duplicate that predates the window | `check()` (`test/ipsec-interop/scenarios/08-responder-eap-mschapv2/check.py`) | New `strongswan.conf` for the scenario sets `charon.filelog.stderr.flush_line = yes`, so a count taken at any moment reflects what charon has logged. The two comments that justified the old reasoning were corrected rather than left stale |

| 4 | ISSUE | Round 2. The round-1 RFC correction stopped at TWO of FOUR sites, and it left `handleOwnedInbound` contradicting itself twelve lines apart: the new text said "Section 2.21.4 is NOT the obligation here" while the token-bucket guard below still opened with "RFC 7296 Section 2.21.4 asks for a rate limit" | `handleOwnedInbound` (`engine/inbound.go`), and the same sentence in `TestEapRtxMidEAPReplayIsRateLimited` (`engine/rfc7296_eap_retransmit_test.go`) | Both now cite `RFC7296-2.4-12` and state why: Section 2.21.4 opens with "A node needs to limit the rate ...", which is not RFC 2119 language, so Section 2.4 carries the only obligation |
| 5 | ISSUE | Round 2. A PUBLIC conformance claim the review had just falsified. `docs/features/rfc-status.md` stated "An unprotected datagram draws no cached response and changes no SA state", and this spec's own Documentation Verified row affirmed it and called the diff "MORE true". The general reading is false | `docs/features/rfc-status.md`, RFC 7296 row, falsified by `handleResponderEAP` (`engine/responder_eap.go`) | The sentence is now bounded to the CACHED message id, names both replay sites, and names the path that DOES change state. The spec's Documentation Verified row is corrected and says its first draft was wrong |
| 6 | NOTE | Round 2. "one of five paths to `StateDead`" is SIX: decrypt failure, AUTH-verify failure, response-build failure, missing EAP payload, missing EAP session, EAP Failure | `handleResponderEAP` (`engine/responder_eap.go`) | Corrected in this spec and in the journal row. The commit message of `c293e9a8e` carries the wrong count and cannot be edited |
| 7 | NOTE | Round 2. `ZE_HANDSHAKE_TIMEOUT` lost its only reader when the round-1 comment rewrite replaced the name with the Go symbol | `check()` (`test/ipsec-interop/scenarios/08-responder-eap-mschapv2/check.py`) | The bound comment names both the constant and its Go source again |
| 8 | ISSUE | Found by the main thread sweeping for the round-2 class, not by a reviewer. A FIFTH site carried the same overstated claim: the `cachedReplayLimiter` field doc said "RFC 7296 Section 2.21.4 requires a rate limit". That section says a node "needs to" limit the rate, which is not RFC 2119 language | `SA.cachedReplayLimiter` (`engine/sa.go`) | Now cites `RFC7296-2.4-12` and says why 2.21.4 is not the obligation. A full sweep of `Section 2.21.4` across `internal/component/ike/engine/` then confirmed every OTHER mention is accurate: the MUST NOTs at `sendInvalidIKESPI` (`engine/notify_error.go`) are the real ones, and `register.go` correctly calls the request case a MAY |
| 9 | ISSUE | Round 3. A sentence reflow, made to satisfy the STE run-on check, introduced a FALSE claim into shipped code. The `localPort` doc asserted "the port is what the PEER observably did" and "NATDetected is ze's own verdict". The producers contradict it: `detectResponderNAT` (`engine/responder.go`) and two `fsm.go` branches set `NATDetected` and call `floatToNATTPort` on the same lines, so the port is ALSO set from ze's own verdict | `SA.localPort` (`engine/sa.go`) and its twin `ChildSA.UDPEncap` (`engine/child.go`) | Both now state the true relation: the implication runs ONE way. Every site that sets `NATDetected` floats the port, and `adoptAuthenticatedEndpoint` is the only site that floats WITHOUT it, which is exactly the case that makes the `NATDetected \|\| localPort == NATTPort` disjunct non-redundant. Verified by enumerating every writer of `sa.localPort` |
| 10 | NOTE | Round 3. "Its MUST NOTs govern two other cases" undercounts: Section 2.21.4 carries `RFC7296-2.21.4-1`, `-3`, `-5`, `-6` and `-7`. The paragraph's conclusion held; the enumeration did not | `replayCachedResponse` (`engine/responder.go`) | Reworded to "govern other cases, among them ..." |
| 11 | NOTE | Round 3. The same reflow dropped a correct number: "28-byte header" became "bare header", which left "raises the cost of a forgery from a bare header to about forty octets" with no baseline | `replayCachedResponse` (`engine/responder.go`) and `TestEapRtxMidEAPReplayIsRateLimited` (`engine/rfc7296_eap_retransmit_test.go`) | "28-byte header" restored at both sites |

| 12 | BLOCKER | Round 4. The `UDPEncap` doc introduces `NATDetected` FIRST and `localPort` SECOND, then said "In production the second implies the first". That asserts `localPort` implies `NATDetected`, which is the direction the paragraph below it refutes by name, and which the sentence's OWN justification clause contradicts ("every site that sets NATDetected also floats the SA") | `ChildSA.UDPEncap` (`engine/child.go`) | One word: "the FIRST implies the second". The sentence was pre-existing and wrong before this spec; round 3's correction landed a right paragraph beside it and made the contradiction visible. The twin in `SA.localPort` (`engine/sa.go`) already stated it correctly |
| 13 | NOTE | Round 4. The `localPort` enumeration was imprecise and partial: it said the writers set `NATDetected` and float "on the same line" (a comment and a `BehindNAT` assignment sit between, in two of them), and it missed the two `rekey.go` sites, which copy `NATDetected` into the replacement SA and carry `localPort` across through `inheritSendPath` | `SA.localPort` (`engine/sa.go`) | Now says "in the same branch", and names the rekey pair. The universal claim was true throughout; only the list was short |

| 14 | NOTE | Round 5. "holds at every writer of NATDetected" is unqualified, and two TEST writers falsify it literally: `bfmInstall` (`engine/rfc7296_natt_bothforms_test.go`) exists to build the NAT-detected-but-not-floated combination, and `TestChildSANATTEncapPorts` (`engine/child_test.go`) sets the flag without floating | `SA.localPort` (`engine/sa.go`) | NOT changed. NOTEs do not block, and a round whose findings are all NOTEs is the last round (`ai/rules/planning.md`). The sentence is scoped by the enumeration that immediately follows it, which names production sites only, and the twin in `child.go` says "In production". The reviewer graded it cosmetic. Recorded so the next editor of that comment adds the two words |
| 15 | NOTE | Round 5. "the one this leaf exists to catch" calls a Go struct field a leaf. `UDPEncap` is a field, and `leaf` is this repo's YANG word | `ChildSA.UDPEncap` (`engine/child.go`) | NOT changed, same reason. Recorded |

Round 5 reported 0 BLOCKER and 0 ISSUE, which is what satisfies this gate. It also verified
the complete writer sets independently: `sa.NATDetected` has four production writers plus two
`rekey.go` SA literals, and `sa.localPort` has exactly two, `floatToNATTPort` and
`inheritSendPath`. No writer either earlier round had missed.

**Rounds 3 and 4 are why this spec ran five rather than three, and both extra rounds were
earned by a PRODUCT defect rather than a record one.** Finding 9 and finding 12 are false
statements in SHIPPED CODE that the producing functions contradict, which
`ai/rules/evidence.md` treats as the shield that stops the next reader checking.

Finding 9 was introduced by an edit made to satisfy the STE run-on check, and finding 12
was uncovered by the fix for finding 9. That is the lesson worth carrying out of this
closure: **a reflow is a rewrite.** An edit made for style still restates a claim, style
tooling makes such an edit feel free, and a restated claim needs its producer read again.

Findings 2, 3, 4 and 5 are all the record-level form of the product defect this spec fixed:
a claim or a guard that is right at the site somebody examined and wrong at its sibling.
Round 1 found it in one comment. Round 2 found that fixing that comment had produced the
same asymmetry one level up. Each was then fixed at EVERY site, not the reported one.

### Raised and NOT fixed: owner decision required

**Two tests tag `RFC7296-2.21.4-5` and prove something else.** That requirement is "A peer
receiving such an unprotected Notify payload MUST NOT respond".
`TestErrResponderWindowDoesNotReflectToObservedSource` and
`TestErrUnprotectedMessageDrawsNoCachedResponse`
(`internal/component/ike/engine/rfc7296_errornotify_test.go`) both drive a forged IKE_AUTH
REQUEST at the cached message id, which Section 2.21.4 does not govern. The ledger derives
2.21.4-5's coverage from those tag lines, so a Notify obligation is credited with proof
from a request test. The governing MUST is `RFC7296-2.4-12`.

It is NOT fixed here because `.claude/hooks/pretool-writeedit.py` refuses an edit that
changes an `RFC requirement:` tag and routes the decision to Thomas. The tests predate this
spec and AC-3 does not depend on them. Recorded in
`plan/journal/reference-checked-claim-unchecked.md`, with the ratchet analysis.

A pre-existing defect found during review is also NOT in the findings table, for the same
reason of scope: `handleResponderEAP` (`engine/responder_eap.go`) sets `StateDead` when
`decryptAndParse` fails, so an unprotected spoofed datagram at a NON-cached message id
still kills a live pre-auth SA (RFC 7296 Section 2.4). AC-3 does not depend on it. Recorded
in `plan/journal/unprotected-message-changes-sa-state.md` per `ai/rules/completion.md`.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/responder.go` | yes | `gopls symbols` lists `replayCachedResponse` as a Function in it |
| `internal/component/ike/engine/rfc7296_eap_retransmit_test.go` | yes | `grep -n '^func Test'` returns the three `TestEapRtx*` tests |
| `test/ipsec-interop/scenarios/08-responder-eap-mschapv2/check.py` | yes | `ls` shows check.py, swanctl.conf, ze.conf |
| `test/ipsec/ipsec-responder-retransmit.ci` | NO, deliberately | `ls test/ipsec/` lists 15 `.ci` files and not this one. Substitution recorded in Wiring Test and Deviations |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-3 | A retransmit is answered from cache and does not set `StateDead` | Read at the producer: the `StateEAPInProgress` arm of `handleResponderInbound` calls `replayCachedResponse` and returns on true, BEFORE it reaches `ps.handleResponderEAP`. The test asserts `resp.State != StateDead` after a second delivery of the same bytes |
| AC-3 | Both guards carry to the new site | Read at the producer: `replayCachedResponse` runs `carriesSKPayload(msg)` then `sa.cachedReplayAllowed()`, in that order, both before its `sendReply` call |
| AC-4 | Scenario 08 establishes, 03 still passes | Scenario 08 output pasted in the Evidence section; 03 is the unchanged control |
| (regression) | The `StateEstablished` arm's behavior is unchanged | Diffed condition by condition against the deleted inline code in `git show c293e9a8e`: same four early-outs, same two guards in the same order, same `sendReply`. The only difference is a bool return the established arm discards |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a retransmitted IKE_AUTH | none, by decision | Verified instead at the entry point in Go: all three tests call `ps.handleResponderInbound` with the real IKE_AUTH bytes a real initiator produced (`eaprtxResponderMidExchange`), and deliver the SAME slice twice. Not a helper-only test |
| a retransmitted IKE_AUTH, through the daemon | `test/ipsec-interop/scenarios/08-responder-eap-mschapv2` | The scenario drops ze's port 4500 for eight seconds so charon retransmits of its own accord, then asserts the SA established AND that charon's retransmit count rose. The second assertion is what stops the first passing vacuously |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Defect 1's. The scenario 08 charon log carries the whole IKE_AUTH exchange on 4500 in both directions, which is the float A-1 describes |
| A-2 | confirmed | Defect 1's. Same log. `sendReply` (`engine/eap_auth.go`) follows the arrival socket |
| A-3 | confirmed | `cacheResponse` (`engine/msgid.go`) is called by both `startResponderEAP` and `sendResponderEAP` (`engine/responder_eap.go`). The fix is one call because of this |
| A-4 | confirmed | `adoptAuthenticatedEndpoint` (`engine/sa.go`) is the only writer of `sa.peerEndpoint`, and `handleAuthRequest` (`engine/responder.go`) returns at its `startResponderEAP` call before reaching that write |
| R-1 | validated | Scenario 03 passes. The initiator path is untouched by this diff |

None left `unvalidated`. No assumption was broken, so the Mistake Log carries no `assumption` row about A-1..A-4.

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Anchored claims naming the changed files | `grep -rn 'engine/responder.go\|engine/inbound.go' docs/ ai/` returns 6 hits: `docs/features.md`, `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-14-responder.md` (twice), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, and two generated index rows | None is stale. Each describes the responder's ROLE and handshake surface (`handleResponderInbound`, `handleSAInitRequest`, `handleAuthRequest`, ESP selection). This diff adds no symbol to those lists and changes no described behavior: it closes an arm of an existing state machine |
| RFC status page | `docs/features/rfc-status.md` RFC 7296 row stated that "An unprotected datagram draws no cached response and changes no SA state" | EDITED, and the first draft of this row was WRONG. It said no edit was needed because the diff made the claim "MORE true". Review round 2 falsified the general reading: at a message id the responder is still expecting, an undecryptable datagram reaches `handleResponderEAP` (`engine/responder_eap.go`) and sets `StateDead`. The claim holds only at the CACHED message id. The sentence now says so, names both replay sites, and names the path that does change state |
| Config / CLI / API / metrics / plugin inventory | No YANG leaf, no `ze:command`, no env var, no metric, no registration in the diff (`git show c293e9a8e --stat`: 7 files, all engine, test, interop fixture or spec) | No category applies |
| Runtime dependency / `ze doctor` | The diff adds no file path, socket, listen port, kernel module, external binary or cert | No doctor check owed |
| `ai/INDEX.md` discovery | The diff adds no feature, tool, make target or gate | No row owed |

### Gates Run At Closure

A SHARED CHECKOUT cannot give a clean full `ze-verify` (`ai/rules/git-safety.md`), so the
evidence is scoped to the surfaces this work changed, and every red is attributed.

| Gate | Result | Attribution |
|------|--------|-------------|
| `golangci-lint run ./internal/component/ike/engine ./internal/component/ike/cmd` | 0 issues | ours, green |
| `make ze-test-pkg PKG=./internal/component/ike/engine` | ok, 24.4s | ours, green |
| `make ze-rfc-check` | exit 0, 2967 gated MUSTs | ours, green after `make ze-rfc-index` |
| `rfc_requirements.py --check-fresh` | 177 shards up to date | ours, green after the BLOCKER fix |
| `make ze-spec-citation-check` | OK, 221 specs | ours, green. WARNs are foreign |
| `make ze-journal` | exit 0 | ours, green |
| `audit-test-relaxation.py origin/main` | 0 deleted, 0 weakened, 4 relaxed | all four foreign: `internal/component/lg/` (two), `internal/component/web/golden_test.go`, `test/plugin/mup4.ci`. No test of this spec's lost an assertion |
| `make ze-doc-test` after the `rfc-status.md` edit | see the `ze-doc-test` row below | the RFC-status edit is prose inside an existing row. It changes no Status cell and no gap count, and `make ze-rfc-check` covers `check_status_completeness`, `check_status_agreement` and `check_gap_count_agreement`. All exit 0 |
| `make ze-lint-changed` | red | FOREIGN. `internal/component/web/view_test.go` typecheck, another session's templ migration mid-edit |
| `make ze-validate` | red | FOREIGN. Three unwired exports in `internal/component/vpp/config.go`, another session's unexport refactor. The file was clean at this session's first `git status` and modified later |
| `make ze-doc-test` | red, twice, on the same two foreign causes | FOREIGN, and re-run after the `rfc-status.md` edit to prove that edit added nothing. Cause 1: `ai/DOCS-TO-CODE.md` is stale, and its uncommitted diff is BGP `forward-rails.md` plus `internal/component/lg/`. Cause 2: a source anchor in `docs/guide/web-interface.md` names `internal/component/web/templates/input/text.html`, which the templ migration deleted. No file this closure touched carries a `// Design:` header edit, and this closure adds no source anchor |
| `make ze-ipsec-test` | 14/15 | The one red is machine-dependent, not ours: `ipsec-show-dataplane.ci` needs `10.0.0.1` and this host cannot bind it. Recorded in `plan/journal/gate-verdict-depends-on-the-machine.md` |
| `ste_check.py --check` | one flag | `e.g.` inside a verbatim RFC 7296 Section 2.4 quote. `ai/rules/writing.md` never edits quoted external text |

## Core Insight

**A cache with two consult sites gets its guards on the site whose failure was noticed.**

The established arm carried `carriesSKPayload` and a token bucket because somebody had
reasoned about amplification THERE. The EAP arm sat four lines above it, held the same cache
open for far longer, and had no guard at all. The asymmetry was invisible because both arms
read correctly on their own: one has guards, one calls a handler.

What makes it a class rather than an oversight: the EAP arm's exposure is strictly WORSE
than the arm that got the attention. It holds the window open across several round trips
instead of one narrow pre-adoption moment, and its unguarded path did not merely amplify,
it reached `StateDead`. A 28-byte forged header killed a live IKE SA. The site with the
longer exposure and the worse failure is the one nobody guarded, because the guard was
written where the bug was FOUND rather than where the mechanism is CONSULTED.

The fix shape follows: lift the guarded body into one function and call it from every
consult site, so the next site added inherits the guards instead of re-deriving them.
