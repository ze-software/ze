# Spec: ike-responder-auth-malformed-request-notify

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | An authenticated IKE_AUTH request whose encrypted content does not parse ends the responder's handshake with no answer, where RFC 7296 Section 2.21.2 requires an UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX notification. |
| Scope | protocol |
| RFC gaps | RFC7296-2.21.2-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

During IKE_AUTH, the responder ends the handshake (`StateDead`) and sends nothing
when a request passes its integrity check but its encrypted content does not parse
(`errInnerParse`). Three handlers share the shape: `handleAuthRequest`
(`internal/component/ike/engine/responder.go`, the first IKE_AUTH),
`handleResponderEAP` (`responder_eap.go`, each EAP round) and
`handleResponderAuthRound` (`multiple_auth.go`, the RFC 4739 second exchange).

RFC 7296 Section 2.21.2: "Note, however, that request messages that contain an
unsupported critical payload, or where the whole message is malformed (rather than
just bad payload contents), MUST be rejected in their entirety, and MUST only lead to
an UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX Notification sent as a response."
The same section: "In an IKE_AUTH exchange, or in the INFORMATIONAL exchange
immediately following it (in case an error happened when processing a response to
IKE_AUTH), the UNSUPPORTED_CRITICAL_PAYLOAD, INVALID_SYNTAX, and AUTHENTICATION_FAILED
notifications are the only ones to cause the IKE SA to be deleted or not created,
without a Delete payload."

The goal: each of the three sites answers with the protected notify, then ends the
handshake.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - the unauthenticated-input paragraph names this gap
  → Constraint: an unprotected request stays silent and counted (`discardUnauthenticatedHandshakeRequest`)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - RFC7296-2.21.2-1, RFC7296-3.10.1-1
  → Constraint: INVALID_SYNTAX goes out only when the Message ID and the checksum are valid (Section 3.10.1)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/responder.go` - `handleAuthRequest` sets `StateDead` on `errInnerParse`; `respondAuthError` sends one notify with no data to the observed source
- [ ] `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP`, the same shape
- [ ] `internal/component/ike/engine/multiple_auth.go` - `handleResponderAuthRound`, the same shape
- [ ] `internal/component/ike/engine/inbound.go` - `respondInnerParseError` answers an established SA through `respondError`

**Behavior to preserve:** the silent, counted drop of an unprotected request at all three sites.

**Behavior to change:** the authenticated malformed request draws the notify before the handshake ends.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `errInnerParse` | `inbound.go`, set by `decryptAndParse` and `decryptSKPayload` | Marks a failure after the integrity check passed | does not apply: already the condition this spec branches on |
| `respondInnerParseError` | `inbound.go` | Picks UNSUPPORTED_CRITICAL_PAYLOAD (payload type from `wire.CriticalPayloadType`) or INVALID_SYNTAX, sends through `respondError` to the configured peer | extends: the choice is reused with the handshake's reply destination, never copied |
| `respondAuthError` | `responder.go` | `buildErrorNotifyResponse`, then `sendReplyFrom` to the observed source (RFC 7296 Section 2.11), no notification data | extends: carries the one-octet data UNSUPPORTED_CRITICAL_PAYLOAD needs |
| `discardUnauthenticatedHandshakeRequest` | `responder_unauthenticated.go` | Drops and counts an unprotected request | does not apply: unchanged |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An IKE_AUTH datagram on the IKE socket, `dispatchInbound` then `handleInbound`

### Transformation Path
1. `handleResponderInbound` routes by state to one of the three handlers
2. `decryptAndParse` answers `errInnerParse`
3. The handler answers through the IKE_AUTH reply path, then sets `StateDead`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to IKE peer | One SK-protected IKE_AUTH response | No |

### Changed-Wire Census

**Search evidence:** to be filled at design; the only writer is `buildErrorNotifyResponse`, the reader is the peer's IKE_AUTH response handler (strongSwan, Ze's `handleAuthResponse`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| IKE_AUTH error response | writer | `notify_error.go` `buildErrorNotifyResponse` | to be filled at design | to be filled at design |

### Integration Points
- `respondInnerParseError`, `respondAuthError`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | design |
| No unintended coupling (components stay isolated) | No | design |
| No duplicated functionality (extends existing, does not recreate) | No | design |
| Zero-copy preserved where applicable (refs, not copies) | No | design |
| Registration over hardcoding, outbound | No | design |
| Registration over hardcoding, inbound | No | design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan ends its IKE SA on INVALID_SYNTAX in an IKE_AUTH response | RFC 7296 Section 2.21.2 | the peer retransmits until timeout | read charon's source at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A notify to the configured peer rather than the observed source misses a NATed initiator | interop red | use the IKE_AUTH reply path |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | One handshake that already ends would end with a wrong or missing notify |
| How is it reverted? | single commit revert |
| Who else touches this path? | `plan/release/fixes/spec-single-owner-lifecycle.md` AC-8 (initiator side, `fsm.go`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH datagram through `handleInbound` | → | the three handlers' `errInnerParse` branch | `TestResponderEAPEndsOnAuthenticatedMalformedRound`, `TestResponderSecondAuthRoundEndsOnAuthenticatedMalformedRequest`, plus a first-IKE_AUTH sibling |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | First IKE_AUTH, EAP round or second-exchange request sealed under the peer's keys whose inner chain does not parse | One protected IKE_AUTH response carrying INVALID_SYNTAX goes to the observed source, then `StateDead` | none |
| AC-2 | The same with an unrecognized critical inner payload | UNSUPPORTED_CRITICAL_PAYLOAD whose data is the one-octet payload type, then `StateDead` | none |
| AC-3 | An unprotected request at any of the three sites | The SA keeps its state and the drop is counted, as today | silence is owed: no notify may answer an unprotected request |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResponderEAPEndsOnAuthenticatedMalformedRound` | `internal/component/ike/engine/rfc7296_unprotected_handshake_test.go` | AC-1, gains the notify assertion; tagged RFC7296-2.4-3, so its record is re-recorded | |
| `TestResponderSecondAuthRoundEndsOnAuthenticatedMalformedRequest` | same | AC-1, gains the notify assertion | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the change takes no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be named at design | `test/ipsec/` | a genuine EAP exchange still establishes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| to be named at design | `test/interop-ipsec/scenarios/` | strongSwan | the peer ends its half on the notify | |

## Files to Modify
- `internal/component/ike/engine/responder.go` - `handleAuthRequest`, `respondAuthError`
- `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP`
- `internal/component/ike/engine/multiple_auth.go` - `handleResponderAuthRound`
- `internal/component/ike/engine/inbound.go` - `respondInnerParseError`

## Files to Create
- none expected

## Implementation Steps

1. **Phase: Wiring** -- the two positive tests gain the notify assertion and go red
   - Tests: the Wiring Test row
   - Files: `rfc7296_unprotected_handshake_test.go`
   - Verify: red, because no datagram goes out
2. **Phase: Notify** -- the three `errInnerParse` branches answer through the IKE_AUTH reply path
   - Tests: as above, plus a first-IKE_AUTH sibling
   - Files: the Files to Modify
   - Verify: tests fail, implement, tests pass

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Interop tests for protocol features (or N-A with a reason)
- [ ] The engine package tests under the full feature tag set, `./le test functional ipsec`, and `./le test integration interop-ipsec` for the named scenario

### Closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/silent-fall-through.md` | 2026-11-10 | IKE responder IKE_AUTH errInnerParse sends no notify |

## Known Limitations
- The initiator side (`fsm.go`) is owned by `plan/release/fixes/spec-single-owner-lifecycle.md` AC-8, including the undecryptable IKE_AUTH and EAP datagrams.
