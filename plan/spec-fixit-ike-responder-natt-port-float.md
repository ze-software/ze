# Spec: fixit-ike-responder-natt-port-float

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-ike-responder-natt-port-float.md` |
| Updated | 2026-07-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-31. It is the first time the responder EAP interop scenario was able to start.

## Task

**Two defects, and the second one turns the first into a dead session.**

**Defect 1: Ze gates the NAT-T reply form on its OWN NAT verdict, not the peer's.**
`sendWithNATT` (`internal/component/ike/engine/eap_auth.go:65-71`) gates both the non-ESP
marker and the destination port on `sa.NATDetected`. That field is Ze's own verdict. When
Ze sees no NAT it replies with no marker, even to a peer that has already floated to 4500.

`dispatchNATTInbound` (`engine/register.go:481-499`) accepts the request on 4500 and hands
the port-500 transport to the handler, so the reply leaves from the wrong socket.

**Defect 2: a retransmitted IKE_AUTH kills the SA instead of replaying the cached response.**
`handleResponderEAP` (`engine/responder_eap.go:230-234`) finds no EAP payload and no AUTH
payload on the retransmit, and sets `StateDead`. RFC 7296 Section 2.1 makes a retransmission
normal, and the responder is required to be able to answer it.

Defect 2 is a defect on its own terms, whatever causes the retransmit.

## Observed sequence

Verified facts from the scenario run:

| Step | Evidence |
|------|----------|
| Ze answers IKE_SA_INIT | strongSwan parses it and selects the proposal |
| Ze reaches `responder EAP started` | `engine/responder_eap.go:187`, reachable only after IKE_AUTH #1 arrives and IDr, CERT, AUTH and the EAP request are sent |
| strongSwan never reports parsing that response | its log stops after `sending cert request` |
| 4.001 seconds later Ze logs `EAP round missing EAP payload` | `responder_eap.go:231`, then `StateDead` |
| later packets are dropped | `no SA for NAT-T packet`, `engine/register.go:495` |
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

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point

An IKE_AUTH request arriving on UDP 4500, dispatched by `dispatchNATTInbound`
(`register.go:481`).

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

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | A reply to a request received on 4500 carries the non-ESP marker and leaves from 4500 |
| AC-2 | The reply form follows the received datagram, never Ze's own NAT verdict |
| AC-3 | A retransmitted IKE_AUTH is answered with the cached response, and does not set `StateDead` |
| AC-4 | Scenario 08 establishes, and scenario 03 still passes |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH on 4500 (`register.go:481`) | -> | the reply-form decision | `test/ipsec/ipsec-responder-natt-reply.ci` |
| a retransmitted IKE_AUTH (`responder_eap.go:230`) | -> | the cached-response replay | `test/ipsec/ipsec-responder-retransmit.ci` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| The reply form follows the received port, both ways | AC-1, AC-2 |
| A retransmit replays rather than kills | AC-3 |

### Functional Tests

| Test | Role |
|------|------|
| `test/ipsec/ipsec-responder-natt-reply.ci` | AC-1 and AC-2 through the daemon, on every push |
| `test/ipsec/ipsec-responder-retransmit.ci` | AC-3 through the daemon |
| `test/ipsec-interop/scenarios/08-responder-eap-mschapv2` | AC-4 against a real peer |

The two `.ci` tests run on every push, because the `ipsec` suite now has a `run_suite` line
(`mk/test-functional.mk`). The interop scenario is the one that proves strongSwan agrees.

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
