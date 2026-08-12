# Spec: fixit-eap-tls-clienthello-race

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; nothing deferred; the spec has no Known Limitations and no out-of-scope decision. Create `plan/deferrals/fixit-eap-tls-clienthello-race.md` on the first deferral) |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-31. The EAP-TLS interop scenario started for the first time.
It had never run before. The lab built a ze binary without the `ze_ike` tag.
The scenario config also named a PKI key by file path, where the schema takes
inline base64 DER. Both are fixed, so scenario 04 now reaches the EAP exchange.

## Task

**Ze answers the EAP-TLS Start with a fragment acknowledgement instead of a ClientHello, so
strongSwan fails the method immediately.**

strongSwan sends a 67-octet `EAP/REQ/TLS` Start. Ze replies with 67 octets. A ClientHello is
several hundred octets. strongSwan then logs `EAP method EAP_TLS failed`.

`readAndSendTLS` (`internal/component/ike/eap/peer.go`) calls
`ps.tlsTransport.readServerData()`, which is a non-blocking drain of a buffer under a mutex
(`eap_tls.go`). When that drain returns empty, line 415 sends `TypeData: []byte{0}`,
a bare flags octet, which is the fragment-acknowledgement form.

It is reached from `handleTLSRequest:286`, immediately after `startTLSClient()` at `:283`.
**Nothing waits for the TLS client goroutine to produce the ClientHello.** So Ze
acknowledges the Start rather than answering it.

Ze's own log carries the sequence: `sending EAP response code=2 type=13 msgID=3`, then
`EAP received code=4`, which is Failure.

## Why no test caught it

The EAP-TLS unit tests drive the handshake through an in-memory transport. There the
ClientHello is already buffered when the drain runs. The race needs a real goroutine
scheduling boundary, which only the interop lab supplies. Scenario 04 is that test, and
it was unable to start until today.

## Required Reading

| Document | Why |
|----------|-----|
| RFC 5216 Section 2.1 | The EAP-TLS message flow, and what answers a Start |
| `internal/component/ike/eap/peer.go` | `readAndSendTLS` and `handleTLSRequest` |
| `internal/component/ike/eap/eap_tls.go` | The buffered transport and its non-blocking drain |
| `ai/rules/evidence.md` | An empty drain is a zero value that reads as a valid answer |

## Current Behavior (MANDATORY)

Source files read on 2026-07-31:

- [ ] `internal/component/ike/eap/peer.go`
- [ ] `internal/component/ike/eap/eap_tls.go`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`handleTLSRequest` (`eap/peer.go`), on the inbound `EAP/REQ/TLS` Start.

### Transformation Path

`startTLSClient()` starts the TLS client goroutine. `readAndSendTLS` immediately drains the
outbound buffer. The drain is non-blocking, so it returns empty while the goroutine is still
producing the ClientHello. The empty result takes the fragment-acknowledgement branch, and
Ze sends one flags octet.

The fix must make the send wait for the handshake bytes, bounded, and must distinguish
"nothing yet" from "genuinely nothing to send".

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| TLS client goroutine -> transport buffer | in | The ClientHello |
| transport buffer -> EAP peer | in | Whatever has arrived, which is the defect |
| EAP peer -> IKE | out | The EAP response, today an acknowledgement |

### Integration Points

Scenario 04 of `test/ipsec-interop` is the gate. It currently fails at this point.

## Key Design Decisions

| Decision | Over | Because |
|----------|------|---------|
| Wait for handshake output, then answer a Start | Retrying on the next EAP round | The peer fails the method on the first wrong answer, so there is no next round |
| Keep the acknowledgement branch for genuine fragment ACKs | Removing it | It is the correct answer mid-fragmentation. The defect is reaching it at the Start |
| Bound the wait and fail with a named error | Waiting without a bound | A stalled TLS client must not hang the IKE exchange |

## Blast Radius

`internal/component/ike/eap/`. EAP-TLS only. EAP-MSCHAPv2 does not use this path, and
scenario 03 passes.

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | The ClientHello is the only message affected | The failure is at the first exchange | Run scenario 04 past the Start and see where it stops next. **confirmed 2026-08-10**: scenario 04 runs the whole EAP-TLS exchange and reports `EAP-TLS tunnel established with certificate authentication`, so nothing after the Start needed a second fix |
| R-1 | A bounded wait CAN hide a real stall as a slow start | Any timeout does | The bound must fail loudly with the stall named, never send an acknowledgement instead. **closed 2026-08-10**: `readAndSendTLS` reports `errTLSClientStalled` when the engine settles empty with the handshake incomplete |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | Ze answers an EAP-TLS Start with a ClientHello |
| AC-2 | A genuine fragment acknowledgement is still sent when that is the correct answer |
| AC-3 | A TLS client that produces nothing fails with a named error rather than an acknowledgement |
| AC-4 | Scenario 04 completes the EAP-TLS exchange |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| inbound EAP-TLS Start (`peer.go`) | -> | the handshake-output wait | a unit test with a deliberately slow TLS client |
| scenario 04 against strongSwan | -> | the whole EAP-TLS path | `test/ipsec-interop/scenarios/04-eap-tls` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| A slow TLS client still yields a ClientHello | AC-1, AC-3 |
| A mid-fragmentation empty drain still acknowledges | AC-2 |

### Functional Tests

| Test | Role |
|------|------|
| `test/ipsec/ipsec-eap-tls-clienthello.ci` | The coupled wait-plus-guard pair through two daemons, on every push. It does NOT prove AC-1's wire form |
| `test/ipsec-interop/scenarios/04-eap-tls` | AC-1 and AC-4 on the wire, the only test that reproduces the race against a real peer |

The `.ci` proves the daemon completes the EAP-TLS exchange with the wait in place, and it
runs on every push because the `ipsec` suite has a `run_suite ipsec` line in
`mk/test-functional.mk`. It cannot see which octets Ze puts on the wire. The interop
scenario is what proves strongSwan accepts a ClientHello there. Both are needed, and the
measurement that separates them is in "What the per-push test cannot see" below.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/eap/peer.go` | Wait for handshake output, then answer a Start |
| `internal/component/ike/eap/eap_tls.go` | A blocking-with-deadline read beside the existing drain |

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The fix sits inside the EAP-TLS peer state machine. No leaf changed |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | No command surface touched |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | No | No RPC added. `test/ipsec/ipsec-eap-tls-clienthello.ci` covers the daemon path |
| Pipe completeness | N-A | The change produces no CLI output |
| Env var registration | No | `eapTLSSettleBackstop` (`eap_tls.go`) is a compile-time constant, not an operator knob |
| Doctor check for runtime dependencies | No | No new file path, socket, port, kernel module, binary, or certificate. The wait uses the transport that already exists |
| Prometheus counters/metrics | No | A stall ends the EAP method and is reported by name through the existing error path |
| BGP family surface (new SAFI / capability / attribute) | N-A | IKEv2 and EAP only |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The "IPsec EAP Authentication" row of `docs/features.md` already claims EAP-TLS. This makes its first exchange work |
| 2 | Config syntax changed? | No | No leaf touched |
| 3 | CLI command added/changed? | No | No command touched |
| 4 | API/RPC added/changed? | No | No RPC touched |
| 5 | Plugin added/changed? | No | `internal/component/ike` is a component, and its registration is unchanged |
| 6 | Has a user guide page? | No | `docs/guide/ipsec.md` documents operator config. This change alters no config and no output |
| 7 | Wire format changed? | No | The EAP-TLS packet layout is unchanged. The defect was WHICH packet the peer sent |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5216.md` gains `[RFC5216-2.1.1-2]`, tagged in both polarities. The RFC 5216 row of `docs/features/rfc-status.md` needs no edit: its support claim covers the full peer-side handshake, and its Remaining cell still names the same three gaps |
| 10 | Test infrastructure changed? | No | The `.ci` runs in the existing `ipsec` suite |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no EAP-TLS claim this changes |
| 12 | Internal architecture changed? | No | `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` anchors `handleTLSRequest` and `readAndSendTLS`, and its prose covers the Section 2.1.3 two-round alert delivery, which this change leaves alone |
| 13 | Route metadata keys added/changed? | N-A | No route metadata on this path |
| 14 | Prometheus counters added/changed? | No | No counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers differently |
| 16 | Any changed source file referenced by existing doc source anchors? | No | `grep -rn "component/ike/eap" docs/` names anchors in `ipsec-9-ikev2-eap-nat.md`, `ipsec-11-interop-eap.md`, `docs/guide/ipsec.md` and `docs/features.md`. None states the empty-drain behavior, so none went stale |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config or CLI surface changed |

## Implementation Steps

1. Write the slow-client unit test, and record the red.
2. Add the bounded wait, and keep the acknowledgement branch for its real case.
3. Confirm green, then run scenario 04.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC has a producer. AC-1 and AC-2 are the two exits of `readAndSendTLS` (`peer.go`), AC-3 is its `tlsDone` branch returning `errTLSClientStalled`, AC-4 is scenario 04 |
| Zero value as a valid answer | An empty drain must never reach the wire as a bare flags octet while the handshake runs. The branch reads `tlsDone`, never the buffer length alone (`ai/rules/evidence.md`) |
| The wait is bounded | `waitServerData` (`eap_tls.go`) returns when the engine parks (`readIdle`, `finished`, `closed`, or an error), and otherwise when `eapTLSSettleBackstop` fires. No path blocks the IKE exchange without a bound |
| Both halves judged apart | The peer half reports the stall. The authenticator half (`tlsMethod.Process`) keeps its bare ACK, because the same guard there makes `eapTLSMaxPeerBuffered` unreachable. `eap_tls.go` records that reason beside the branch |
| The tests discriminate | Reverting the wait alone turns the `.ci` red. Reverting the guard alone leaves it green. Reverting both leaves it green, which is why scenario 04 stays the proof of AC-1 (`ai/rules/interop-and-goal-validation.md`) |
| The acknowledgement keeps its case | `TestEAPTLSPeerAcknowledgesOnlyAfterTheHandshakeFinishes` and `TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess` fail if the guard becomes unconditional |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The bounded wait | `grep -n "func (t \*eapTLSTransport) waitServerData" internal/component/ike/eap/eap_tls.go` |
| The named stall error, gated on `tlsDone` | `grep -n "errTLSClientStalled" internal/component/ike/eap/peer.go`: the declaration and the one `return` |
| Unit proof of all three branches | `make ze-test-pkg PKG=./internal/component/ike/eap`: `ok github.com/ze-software/ze/internal/component/ike/eap` under `-race` |
| The per-push functional test | `ls test/ipsec/ipsec-eap-tls-clienthello.ci`, and the `run_suite ipsec` line of `mk/test-functional.mk` |
| The interop proof | `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=04-eap-tls`: `EAP-TLS tunnel established with certificate authentication` |
| The RFC row and its two tags | `python3 scripts/dev/rfc_requirements.py --check` reports no RFC 5216 requirement, and `ai/RFC-REQUIREMENTS.md` lists `RFC5216-2.1.1-2` with a positive and a negative test |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | The wait is bounded by `eapTLSSettleBackstop`, two seconds per EAP round. A wedged TLS engine cannot hold the IKE exchange open |
| Unbounded buffering | `eapTLSMaxReassembly` and `eapTLSMaxPeerBuffered` still bound the peer's backlog. `TestEAPTLSProcessRefusesUnboundedPeerBuffer` is what proves the second one stays reachable, and it is why the authenticator half keeps its bare ACK |
| Fail closed | An empty settle with the handshake incomplete sets `peerStateFailed` and returns no packet, rather than a packet that reads as a valid answer |
| Error leakage | `errTLSClientStalled` names a local condition. It carries no key material, no certificate content and no peer identity |
| Authentication bypass | No route reaches an MSK without a completed handshake: `exportEAPTLSMSK` (`eap_tls.go`) refuses when `HandshakeComplete` is false, and `deriveTLSMSK` (`peer.go`) reports the handshake's own error first |

## Evidence (2026-08-10)

The bounded wait was already in the tree when this phase started: `waitServerData`
and `eapTLSSettleBackstop` (`eap_tls.go`), read by `readAndSendTLS` (`peer.go`)
and by `tlsMethod.Process`. AC-1, AC-2 and AC-4 held on arrival. AC-3 did not:
the empty branch still answered with `TypeData: []byte{0}`.

| AC | Evidence |
|----|----------|
| AC-1 | `TestEAPTLSPeerFirstResponseCarriesClientHello` asserts a real ClientHello record answers the Start, and scenario 04 proves strongSwan accepts it. The per-push `.ci` is NOT evidence here: see "What the per-push test cannot see" |
| AC-2 | `TestEAPTLSPeerAcknowledgesOnlyAfterTheHandshakeFinishes` (the branch) and `TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess` (end to end) |
| AC-3 | `TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging` asserts `errTLSClientStalled` and no response |
| AC-4 | `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=04-eap-tls`: `EAP-TLS tunnel established with certificate authentication`, PASS |

Tests FAIL, before the guard:

```
--- FAIL: TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging (2.00s)
    eap_tls_flight_test.go:228: a peer whose TLS client produced nothing reported
    no error and replied &{Code:2 Identifier:7 Type:13 TypeData:[0]}
```

Tests PASS, after it: `ok github.com/ze-software/ze/internal/component/ike/eap 3.485s`
(`-race`), `golangci-lint run ./internal/component/ike/eap`: 0 issues.

The guard discriminates in both directions. Removed, the stall test goes red on
`TypeData:[0]`. Made unconditional, the boundary test and six committed tests go
red, `TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess` among them.

**The authenticator half keeps its bare ACK, deliberately.** The same guard in
`tlsMethod.Process` makes `eapTLSMaxPeerBuffered` unreachable: `feedPeerData`
runs before `waitServerData`, so the error ends the exchange on the first
message and no second one crosses the backlog ceiling.
`TestEAPTLSProcessRefusesUnboundedPeerBuffer` caught it. `eap_tls.go` records
the reason beside the branch.

That same branch also answers an empty message the peer sends in reply to the Start,
which RFC 5216 Section 2.1.5 sanctions only in reply to an M-flagged message. The RFC
states no obligation to refuse it, so this is hardening rather than a deviation, and it
is filed as `plan/future/spec-eap-tls-authenticator-empty-message-guard.md`.

**Still owed: nothing.** `test/ipsec/ipsec-eap-tls-clienthello.ci` now exists. It
runs under `make ze-ipsec-test` and inside the `run_suite ipsec` line of
`mk/test-functional.mk`, so it runs on every push, and it passed 15 of 15
assertions on eight consecutive runs.

### What the per-push test cannot see

The `.ci` guards the coupled wait-plus-guard pair. It does not guard AC-1's wire
form. Measured on 2026-08-10 by building the daemon twice and comparing:

| Reverted | Result |
|----------|--------|
| `waitServerData` back to a non-blocking snapshot drain | RED in 34.7s. The initiator logs `eap-tls: the TLS client produced no handshake data and its handshake is not complete`, the responder times out, and no SA comes up |
| the `errTLSClientStalled` guard alone, wait left in place | GREEN. The wait never returns empty in a healthy exchange, so that branch is not reached |
| BOTH, which puts the original defect on the wire | GREEN in 3.5s |

The third row is the one that matters. `tlsMethod.Process` (`eap_tls.go`) answers a
bare fragment acknowledgement with a bare fragment acknowledgement, so the peer half
sends its ClientHello on the next round and the exchange recovers. Two Ze daemons
therefore always recover from the defect, and no assertion added to this `.ci` would
change that. strongSwan refuses the same answer and fails the method.

So AC-1 is proven by `TestEAPTLSPeerFirstResponseCarriesClientHello` and by
`test/ipsec-interop/scenarios/04-eap-tls`, which stays the only test that can see
the 67-octet answer to a 67-octet Start. The `.ci` header records the same
measurement, so a reader of either file gets it.

The general lesson has a journal row: `plan/journal/shared-leniency-hides-the-defect.md`.

## Goal Gates

`make ze-verify`, and scenario 04 of `make ze-ipsec-interop-test`.

## Quality Gates

`make ze-lint-changed`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

RFC 5216 governs EAP-TLS and IS enrolled in `rfc/enrolled.txt`. The sentence here said the
opposite until 2026-08-10; it was wrong, and the correction is recorded rather than deleted.

Until 2026-08-10 no requirement id covered the behavior this spec fixed. Section 2.1.1
states the peer's answer to the Start in the indicative register: "The EAP-TLS
conversation will then begin, with the peer sending an EAP-Response packet with
EAP-Type=EAP-TLS. The data field of that packet will encapsulate one or more TLS records
in TLS record layer format, containing a TLS client_hello handshake message." It carries no
RFC 2119 keyword, so the summary's checklist held no row for it and the proof was not
machine-checked.

**Extracted and tagged, 2026-08-10.** `rfc/short/rfc5216.md` now carries
`[RFC5216-2.1.1-2] [MUST]` quoting that sentence and naming its register.
`ai/rules/rfc-compliance.md` settles the route: making Ze better proven needs no
permission, and the question is owed only before doing LESS. The row is met in both
polarities on arrival, so it adds coverage and drops nothing.

| Polarity | Test | What it pins |
|----------|------|--------------|
| positive | `TestEAPTLSPeerFirstResponseCarriesClientHello` | The answer to a Start carries a TLS handshake record whose first message is a client_hello |
| negative | `TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging` | With no handshake data the peer sends nothing. The bare flags octet, the only other packet that branch can produce, is refused |

`rfc/extraction/` holds no sign-off artifact for rfc5216, and stays that way: rfc5216 is
one of the grandfathered summaries, and `check_extraction_ratchet` compares a stem against
its own HEAD row, so an unsigned stem gains no obligation from a new row
(`rfc/extraction/README.md`). Scenario 04 proves the same sentence against strongSwan and
carries no tag, because the evidence ratchet keys on kind and tier and an interop tag is a
permanent obligation. Adding one is available and was not taken in this round.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The Start is answered with a ClientHello
- [ ] The acknowledgement branch keeps its real case
- [ ] Tests PASS output pasted into the spec
- [ ] Scenario 04 completes the exchange
- [ ] `make ze-verify` green

---

## Implementation Summary

### What Was Implemented
- `internal/component/ike/eap/eap_tls.go`: `waitServerData` and
  `eapTLSSettleBackstop`, the bounded settle the peer half reads instead of the
  non-blocking snapshot drain. Plus the comment on `tlsMethod.Process`'s empty
  branch recording why the AUTHENTICATOR half keeps its bare ACK.
- `internal/component/ike/eap/peer.go`: `errTLSClientStalled`, and the
  `if !ps.tlsDone.Load()` guard in `readAndSendTLS`. An empty settle with the
  handshake still running sets `peerStateFailed` and returns no packet. An empty
  settle with the handshake FINISHED still sends the no-data response RFC 5216
  Section 2.1.3 requires.
- `internal/component/ike/eap/eap_tls_flight_test.go`: the stall test, the
  boundary test, and the `RFC5216-2.1.1-2` tags in both polarities.
- `test/ipsec/ipsec-eap-tls-clienthello.ci` (new): the coupled wait-plus-guard
  pair through two daemons, on every push.
- `rfc/short/rfc5216.md`: the `RFC5216-2.1.1-2` checklist row, its section-index
  row, and a `5.5 Packet Modification Attacks` row that had fallen out of its
  table to the bottom of the file.

### Bugs Found/Fixed
- **The defect itself.** `readAndSendTLS` drained a buffer the TLS client
  goroutine had not filled yet, and the empty result took the
  fragment-acknowledgement branch. Ze answered a 67-octet Start with 67 octets
  and strongSwan failed the method.
- **The empty branch could not tell two states apart.** An empty buffer means
  "finished with nothing left" or "never produced anything", and the branch read
  it as the first. The guard reads `tlsDone`, never the length alone.

### Documentation Updates
- None owed. `grep -rn "component/ike/eap" docs/` names anchors in
  `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`,
  `docs/architecture/ike/ipsec-11-interop-eap.md`, `docs/guide/ipsec.md` and
  `docs/features.md`. None of them states the empty-drain behaviour, so none went
  stale. The RFC 5216 row of `docs/features/rfc-status.md` needs no edit: its
  support claim already covers the full peer-side handshake and its Remaining
  cell names the same three gaps.
- `make ze-doc-test` NOT RUN: this session was instructed to run no suite.

### Deviations from Plan
- The bounded wait was already in the tree when the closing phase started, so
  AC-1, AC-2 and AC-4 held on arrival and only AC-3 was implemented here.
- The plan named no `.ci`. One was added, and the measurement that says what it
  can and cannot see is in "What the per-push test cannot see".

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The `.ci` was expected to guard AC-1, the wire form of the answer to a Start | A functional test whose two ends are both Ze cannot see a leniency both ends share. `tlsMethod.Process` answers a bare ACK with a bare ACK, so two Ze daemons recover from the defect and the test stays green in 3.5s with the defect on the wire | measured by building the daemon twice, with the change reverted and restored | the `.ci` header and this spec both record it, and the interop scenario stays the evidence for the wire form. The class has a journal row: `plan/journal/shared-leniency-hides-the-defect.md` |
| assumption | No requirement id covered the answer to the Start, so the fix looked unprovable by the RFC gate | RFC 5216 Section 2.1.1 states it in the indicative register, which is extractable at MUST level exactly as RFC 5301 section 3 was | re-read of the section during the closing phase | `RFC5216-2.1.1-2` extracted and tagged in both polarities. It adds coverage and drops nothing |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The Start is answered with a ClientHello, not an ACK | Done | `readAndSendTLS`, `internal/component/ike/eap/peer.go` | reads `waitServerData`, not the snapshot drain |
| The wait is bounded | Done | `waitServerData`, `internal/component/ike/eap/eap_tls.go` | returns when the engine parks, or when `eapTLSSettleBackstop` fires |
| A stall is named, never sent | Done | `errTLSClientStalled`, `peer.go` | `peerStateFailed`, and no packet |
| The genuine ACK keeps its case | Done | the same branch, gated on `ps.tlsDone.Load()` | RFC 5216 Section 2.1.3's no-data response still goes out |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestEAPTLSPeerFirstResponseCarriesClientHello`; `test/ipsec-interop/scenarios/04-eap-tls` | the `.ci` is NOT evidence here, and the spec says why |
| AC-2 | Done | `TestEAPTLSPeerAcknowledgesOnlyAfterTheHandshakeFinishes`, `TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess` | both go red if the guard becomes unconditional |
| AC-3 | Done | `TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging` | red before the guard, on `TypeData:[0]` |
| AC-4 | Done | `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=04-eap-tls` on 2026-08-10: `EAP-TLS tunnel established with certificate authentication` | NOT re-run in the closing session: no Docker |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| A slow TLS client still yields a ClientHello | Done | `eap_tls_flight_test.go` | `TestEAPTLSPeerFirstResponseCarriesClientHello`, tagged `RFC5216-2.1.1-2 positive` |
| A mid-fragmentation empty drain still acknowledges | Done | `eap_tls_flight_test.go` | `TestEAPTLSPeerAcknowledgesOnlyAfterTheHandshakeFinishes` |
| The stall is reported | Done | `eap_tls_flight_test.go` | `TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging`, tagged `RFC5216-2.1.1-2 negative` |
| `ipsec-eap-tls-clienthello` | Done | `test/ipsec/ipsec-eap-tls-clienthello.ci` | 15 of 15 assertions on eight consecutive runs, 2026-08-10. NOT re-run in the closing session |
| scenario 04 | Done | `test/ipsec-interop/scenarios/04-eap-tls` | unchanged by this spec; it is the gate the fix was written against |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/eap/peer.go` | Done | |
| `internal/component/ike/eap/eap_tls.go` | Done | the wait, plus the comment on the authenticator half |
| `internal/component/ike/eap/eap_tls_flight_test.go` | Changed | not in the plan's file list; it carries every unit proof |
| `test/ipsec/ipsec-eap-tls-clienthello.ci` | Changed | not in the plan; added with the measurement of what it cannot see |
| `rfc/short/rfc5216.md` | Changed | not in the plan; the requirement was extracted during the closing phase |

### Audit Summary
- **Total items:** 18
- **Done:** 15
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (the three files the plan did not name), recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Ze answers an EAP-TLS Start with a ClientHello, so strongSwan does not fail the method | interop | `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=04-eap-tls` on 2026-08-10: `EAP-TLS tunnel established with certificate authentication`. Proven to discriminate by reverting `waitServerData` to the snapshot drain, which turns the scenario RED in 34.7s with the initiator logging the stall and no SA coming up |
| A genuine fragment acknowledgement is still sent | unit | `TestEAPTLSPeerAcknowledgesOnlyAfterTheHandshakeFinishes` and `TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`. Making the guard unconditional turns both red, along with six committed tests |
| A stalled TLS client fails by name rather than by an acknowledgement | unit | `TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging`. Red before the guard: `a peer whose TLS client produced nothing reported no error and replied &{Code:2 Identifier:7 Type:13 TypeData:[0]}` |
| The behaviour is machine-checked against the RFC | gate | `rfc/short/rfc5216.md` carries `[RFC5216-2.1.1-2]`, and `rfc/requirements/rfc5216.md` lists a positive and a negative test for it. `make ze-rfc-check` exits 0 |
| The fix runs on every push, not only in the interop lab | functional | `test/ipsec/ipsec-eap-tls-clienthello.ci`, inside the `run_suite ipsec` line of `mk/test-functional.mk` |

**What the per-push test cannot prove, said plainly.** It guards the coupled
wait-plus-guard pair and NOT AC-1's wire form. Reverting both changes, which puts
the original defect on the wire, leaves it GREEN in 3.5s, because
`tlsMethod.Process` answers a bare ACK with a bare ACK and two Ze daemons always
recover. Scenario 04 is the only test that sees the 67-octet answer to a 67-octet
Start, and it did not run in the closing session: no Docker.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| the Deferral shard field is `-`; no shard was ever created | done | nothing was deferred |
| The authenticator half answers an empty message in reply to a Start, which RFC 5216 Section 2.1.5 sanctions only in reply to an M-flagged message | homed | `plan/future/spec-eap-tls-authenticator-empty-message-guard.md`. The RFC states no obligation to refuse it, so this is hardening rather than a deviation. The spec is written and not commissioned: it runs when Thomas says so |
| `scripts/dev/stress-repro.py` cannot stress any `test/ipsec` test, because the `registerCIRoot` suites take a deterministic port and never reserve it | recorded | `plan/journal/parallel-copies-collide-on-a-deterministic-port.md`, already committed. Pre-existing and suite-wide: it reproduces on the committed `ipsec-sa-installed` too, so it is not this spec's to fix |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-eap-tls-clienthello-race-640fa955-f03a-45e8-a58f-4b367f5859e6.md`, `verdict=clean rounds=1` |
| `review_gate.py check` | `review_gate: OK (clean, hashes match)` |
| Rounds | 1. The round returned no BLOCKER and no ISSUE |
| Reviewer lenses used | logic and guard, comment sweep, closure readiness |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | | the round recorded no BLOCKER and no ISSUE | | |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ipsec/ipsec-eap-tls-clienthello.ci` | Yes | on disk, listed by `git status` as a new file in this commit |
| `plan/future/spec-eap-tls-authenticator-empty-message-guard.md` | Yes | on disk, new in this commit |
| `plan/journal/shared-leniency-hides-the-defect.md` | Yes | on disk, new in this commit |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the answer to a Start carries a client_hello | `ok github.com/ze-software/ze/internal/component/ike/eap` (2026-08-12), carrying `TestEAPTLSPeerFirstResponseCarriesClientHello` |
| AC-2 | the genuine ACK survives | same run, `TestEAPTLSPeerAcknowledgesOnlyAfterTheHandshakeFinishes` and `TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess` |
| AC-3 | a stall is named | same run, `TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging`; `errTLSClientStalled` has one declaration and one `return`, both in `peer.go` |
| AC-4 | scenario 04 completes | `EAP-TLS tunnel established with certificate authentication`, 2026-08-10. NOT re-run: no Docker in the closing session |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| inbound EAP-TLS Start | `test/ipsec/ipsec-eap-tls-clienthello.ci` | Read: two ze daemons complete the EAP-TLS exchange. Its header states what it cannot see, and that measurement is repeated in this spec |
| scenario 04 against strongSwan | `test/ipsec-interop/scenarios/04-eap-tls` | the only test that reproduces the race against a foreign peer, and the only proof of AC-1's wire form |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | scenario 04 runs the whole EAP-TLS exchange, so nothing after the Start needed a second fix |
| R-1 | closed | the bound fails loudly: `errTLSClientStalled`, `peerStateFailed`, and no packet |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| no doc states the empty-drain behaviour | `grep -rn "component/ike/eap" docs/` names four files, none of which describes that branch | Yes |
| the RFC 5216 row of `docs/features/rfc-status.md` needs no edit | its support claim covers the peer-side handshake and its Remaining cell names the same three gaps, all of which are still `{gap}` in `rfc/short/rfc5216.md` | Yes |
| `rfc/short/rfc5216.md` carries the new row and its section index | both are in the diff, and `rfc/requirements/rfc5216.md` lists `RFC5216-2.1.1-2` with a positive and a negative test | Yes |
| `mk/test-functional.mk` runs the new `.ci` on every push | its `run_suite ipsec` line covers `test/ipsec/` | Yes |

## Core Insight

A functional test whose two ends are the same implementation cannot see a
leniency both ends share. Ze's authenticator answered a bare fragment ACK with a
bare fragment ACK, so two Ze daemons recovered from a defect strongSwan refuses,
and a `.ci` written against that pair stayed green in 3.5s with the defect on the
wire. A wire-form claim needs a foreign peer, never a second copy of Ze.
