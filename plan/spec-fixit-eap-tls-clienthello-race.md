# Spec: fixit-eap-tls-clienthello-race

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-eap-tls-clienthello-race.md` |
| Updated | 2026-07-31 |

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
| A-1 | The ClientHello is the only message affected | The failure is at the first exchange | Run scenario 04 past the Start and see where it stops next |
| R-1 | A bounded wait CAN hide a real stall as a slow start | Any timeout does | The bound must fail loudly with the stall named, never send an acknowledgement instead |

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
| `test/ipsec/ipsec-eap-tls-clienthello.ci` | AC-1 and AC-3 through the daemon, on every push |
| `test/ipsec-interop/scenarios/04-eap-tls` | AC-4, the only test that reproduces the race against a real peer |

The `.ci` proves the daemon answers a Start with handshake bytes. The interop scenario
proves strongSwan accepts them. Both are needed, and the `.ci` is the one that runs on every
push, because the `ipsec` suite now has a `run_suite` line (`mk/test-functional.mk`).

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/eap/peer.go` | Wait for handshake output, then answer a Start |
| `internal/component/ike/eap/eap_tls.go` | A blocking-with-deadline read beside the existing drain |

## Implementation Steps

1. Write the slow-client unit test, and record the red.
2. Add the bounded wait, and keep the acknowledgement branch for its real case.
3. Confirm green, then run scenario 04.

## Goal Gates

`make ze-verify`, and scenario 04 of `make ze-ipsec-interop-test`.

## Quality Gates

`make ze-lint-changed`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

RFC 5216 governs EAP-TLS and is not enrolled in `rfc/enrolled.txt`. Enrolling it needs an
extraction sign-off artifact, which is larger than this fix. Do not tag a row that does not
exist.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The Start is answered with a ClientHello
- [ ] The acknowledgement branch keeps its real case
- [ ] Tests PASS output pasted into the spec
- [ ] Scenario 04 completes the exchange
- [ ] `make ze-verify` green
