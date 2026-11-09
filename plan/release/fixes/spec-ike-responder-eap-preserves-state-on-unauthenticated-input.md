# Spec: ike-responder-eap-preserves-state-on-unauthenticated-input

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Unauthenticated IKE input must not destroy valid EAP handshake state. |
| Scope | protocol |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Keep an IKE responder's EAP handshake alive when an inbound IKE_AUTH datagram
fails cryptographic validation. This spec owns the remaining obligation recorded
on 2026-08-14 in `plan/journal/unprotected-message-changes-sa-state.md`. The
cached-message-ID repair described there remains separate.

RFC 7296 Section 2.4 (`rfc/full/rfc7296.txt`) states that an endpoint MUST NOT
conclude that its peer failed from IKE messages that arrive without cryptographic
protection. The same section requires implementations to limit the rate of
actions based on unprotected messages, recorded as `RFC7296-2.4-12` in
`rfc/short/rfc7296.md`. Both obligations apply to this repair.

`handleResponderInbound` (`internal/component/ike/engine/responder.go`) routes an
IKE_AUTH request in `StateEAPInProgress` through the cached-response guard and
then to `handleResponderEAP` (`responder_eap.go`), which set `sa.State =
StateDead` on every error from `decryptAndParse`. `decryptAndParse` (`inbound.go`)
returns errors for a missing SK payload and for decryption failure, and marks an
inner-payload parse error after a successful integrity check with
`errInnerParse`. The repair keeps that distinction.

The design preserves the SA and its EAP progress when a datagram lacks valid
cryptographic protection, keeps the handling of authenticated malformed payloads
and authentication failures, lets valid EAP and final AUTH exchanges complete,
keeps cached retransmission replay, and keeps the half-open timeout effective, so
unprotected input cannot keep a handshake alive indefinitely. Proof runs through
the daemon's wire dispatch (`handleInbound`), fails under the recorded StateDead
transition, distinguishes an authenticated error from an unauthenticated one, and
includes interop evidence with strongSwan as an independent EAP peer, with a valid
control exchange and a count of bounded actions under repeated unprotected input.

This spec does not own remote-access admission, virtual-address assignment, a
broader EAP feature expansion, or the initiator half (`handleAuthResponse`,
`handleEAPResponse` in `fsm.go`), which the 2026-09-29 journal row assigns to
`plan/release/validation/spec-rfc-verdict-fix-ike-eap.md`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - the responder role, its half-open reaper, its proof
  → Decision: the deterministic proof is the in-process handshake driven through `handleInbound`; interop against strongSwan catches what a self-consistent pair cannot
  → Constraint: `reapStaleHandshake` keys on `CreatedAt`, so nothing the drop does may refresh it
- [ ] `ai/doors/access-security.md` - IKE door, its open trap row for this defect
  → Constraint: the trap row is updated in the same change
- [ ] `docs/architecture/testing/interop.md` - scenario discovery, the strongSwan lab, the vacuity traps
  → Constraint: the scenario is proven red with the fix reverted and the image rebuilt before it counts
  → Decision: the scenario is named, never numbered, and its checker registers in the lab's checker table
- [ ] `docs/guide/ipsec.md` "Health and metrics" - `ze_ipsec_error_notify_suppressed_total{reason}`
  → Decision: the drop is counted under that existing series with a new reason, not a new metric

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - `RFC7296-2.4-3` (MUST NOT conclude failure from unprotected messages), `RFC7296-2.4-12` (MUST limit the rate of actions)
  → Constraint: RFC 7296 Section 2.4: "an endpoint MUST NOT conclude that the other endpoint has failed based on any routing information (e.g., ICMP messages) or IKE messages that arrive without cryptographic protection (e.g., Notify messages complaining about unknown SPIs)."

**Key insights:**
- `handleResponderAuthRound` (`multiple_auth.go`) already drops an undecryptable RFC 4739 round and keeps the SA; the EAP and first-AUTH handlers were the outliers.
- The inbound token bucket in `dispatchInbound` (`register.go`, 100 per second, burst 200) already bounds every action on inbound datagrams, so the drop needs no limiter of its own.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/responder.go` - `handleResponderInbound` dispatch per state; `handleAuthRequest` set StateDead on any decrypt error
- [ ] `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP` set StateDead on any decrypt error after the RFC 3748 length discard
- [ ] `internal/component/ike/engine/inbound.go` - `decryptAndParse`, `errInnerParse`
- [ ] `internal/component/ike/engine/fsm.go` - `handleInbound` (the dispatch entry), `reapStaleHandshake`
- [ ] `internal/component/ike/engine/multiple_auth.go` - `handleResponderAuthRound`, the precedent that drops and keeps
- [ ] `internal/component/ike/engine/metrics.go` - `countErrorNotifySuppressed`, published as `ze_ipsec_error_notify_suppressed_total`

**Behavior to preserve:**
- `replayCachedResponse` answers a retransmission at the cached id before either handler runs.
- `eapMessageDiscarded` (RFC 3748 Section 4 length discard) keeps its own WARN and its own return.
- An authenticated inner chain that fails to parse (`errInnerParse`) ends the handshake.
- Authentication failures and refusals after a successful decrypt are unchanged.

**Behavior to change:**
- A missing SK payload or a failed integrity check in `handleAuthRequest` and `handleResponderEAP` no longer sets StateDead: the request is dropped, counted and logged at debug, with no datagram and no SA field changed.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Inner-parse marker | `inbound.go` `errInnerParse`, `decryptAndParse` | Separates an authenticated parse failure from a decrypt failure, for RFC 7296 Section 3.10.1 | extends: the new helper branches on it, so no second classifier exists |
| RFC 4739 round drop | `multiple_auth.go` `handleResponderAuthRound` | Drops an undecryptable second round and keeps the SA | does not apply as code (it drops every error, errInnerParse included); it is the precedent the two handlers now follow |
| Suppressed-notify counter | `metrics.go` `countErrorNotifySuppressed`, `ze_ipsec_error_notify_suppressed_total{reason}` | Counts an error notification a guard stopped, by guard name | extends: new reason `handshake-unauthenticated`; RFC 7296 Section 2.21.2 returns an IKE_AUTH error only in a protected response, so this is a notification the drop withholds |
| Inbound token bucket | `register.go` `dispatchInbound`, `newInboundRateLimiter` | Bounds every inbound datagram before dispatch | extends unchanged: it is the RFC7296-2.4-12 bound on the drop |
| Half-open reaper | `fsm.go` `reapStaleHandshake`, `responderHandshakeTimeout` | Ends a pre-established responder handshake 30 s after `CreatedAt` | extends unchanged: the drop never touches `CreatedAt` |
| Cached replay | `responder.go` `replayCachedResponse` | Answers a retransmission at the cached id, refuses an unprotected one | does not apply: runs before the handlers and is untouched |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A UDP datagram on 500 or 4500, read by `dispatchInbound`, matched by SPI to a responder SA.

### Transformation Path
1. `dispatchInbound` admits it through the token bucket and calls `handleInbound`.
2. `handleInbound` routes a responder SA to `handleResponderInbound`.
3. By state: `StateSAInitReceived` reaches `handleAuthRequest`; `StateEAPInProgress` reaches `replayCachedResponse`, then `handleResponderEAP`.
4. Each calls `decryptAndParse`; on an error that is not `errInnerParse`, `discardUnauthenticatedHandshakeRequest` counts, logs at debug and returns.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire to engine | IKE datagram through `handleInbound` | Yes: unit tests deliver through `handleInbound`; interop through the daemon socket |
| Engine to Prometheus | `countErrorNotifySuppressed` read at scrape | Yes: interop reads the series |

### Changed-Wire Census

**Search evidence:** no wire changes, so no rows. The change removes a state transition taken on receipt: it adds no datagram, removes none a conforming peer receives (a forgery drew no answer before either, since the old path set StateDead and returned), and alters no encoder or decoder. Searched every `decryptAndParse(` caller in `internal/component/ike/engine`: `handleAuthRequest`, `handleResponderEAP`, `handleResponderAuthRound` and three sites in `inbound.go`; only the first two changed, both after the decoder returned. Positive control: `handleResponderAuthRound` already drops on the same error.

### Integration Points
- `countErrorNotifySuppressed` - new reason string, published by the existing `ze_ipsec_error_notify_suppressed_total` gauge vector.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the helper is called inside the two existing handlers, after `decryptAndParse` |
| No unintended coupling (components stay isolated) | Yes | one new file in `ike/engine`, no new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `errInnerParse`, the suppressed counter, the token bucket and the reaper |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no buffer handling changed |
| Registration over hardcoding, outbound | Yes | no command, family or handler added |
| Registration over hardcoding, inbound | Yes | the reason string is a free label of an existing vector; searched `countErrorNotifySuppressed(` callers, no enumeration of reasons exists |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every `decryptAndParse` error except `errInnerParse` carries no proof of origin | `inbound.go` `decryptAndParse`: the only other errors are "no SK payload" and `decryptSKPayload`'s | a forged message could still kill, or an authenticated one be ignored | `TestResponderEAPEndsOnAuthenticatedMalformedRound`, `TestResponderEAPSurvivesUnprotectedIKEAuth` | unvalidated |
| A-2 | strongSwan, holding no answer, retransmits IKE_AUTH 1 and Ze replays its cached EAP request, continuing the exchange | `checkResponderEAPMSCHAPv2` uses the same hold and passes | the interop checker cannot complete the exchange | scenario `responder-eap-survives-unprotected-auth` | unvalidated |
| A-3 | The forgeries reach Ze's handshake (same SPIs, port 4500 with non-ESP marker) | `suspicious.go` `injectOutOfSA` reaches Ze from strongSwan's address and port | the counter does not rise for the right reason | the exact-rise assertion in the scenario | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A flood of forgeries keeps the handshake slot busy | `CreatedAt` refreshed | `TestUnprotectedInputDoesNotExtendTheHandshake` holds `CreatedAt` and the reaper |
| R-2 | A WARN per forgery lets an off-path sender write the log | log volume under attack | debug log plus a counter; the counter is the record (`ze-go-style.md`, "Error dropped below WARN" accepts a counter step) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a responder handshake either dies on forgeries (old behavior) or ignores a malformed authenticated request |
| How is it reverted? | single commit revert |
| Who else touches this path? | `spec-rfc-verdict-fix-ike-eap` (initiator half in `fsm.go`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH datagram through `handleInbound`, `StateEAPInProgress` | → | `handleResponderEAP` → `discardUnauthenticatedHandshakeRequest` | `TestResponderEAPSurvivesUnprotectedIKEAuth` |
| IKE_AUTH datagram through `handleInbound`, `StateSAInitReceived` | → | `handleAuthRequest` → `discardUnauthenticatedHandshakeRequest` | `TestResponderAuthSurvivesUnprotectedIKEAuth` |
| Two daemons over loopback, Ze responder as EAP authenticator | → | `handleAuthRequest`, `handleResponderEAP` on genuine rounds | `test/ipsec/ipsec-eap-md5-challenge.ci` |
| Forged IKE_AUTH on Ze's UDP 4500 from strongSwan's address | → | daemon dispatch → `handleResponderEAP` | interop `responder-eap-survives-unprotected-auth` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Mid-EAP, an IKE_AUTH at the next Message ID with no SK payload, or with a failing checksum | Responder stays in `StateEAPInProgress` at the same next Message ID, sends nothing, counts one `handshake-unauthenticated` per datagram; the peer's real request then completes the exchange to established on both sides | none: no answer is owed to an unprotected request |
| AC-2 | Before the first genuine IKE_AUTH, the same two forgeries | Responder stays in `StateSAInitReceived`, sends nothing, counts each; the real IKE_AUTH then completes the exchange | none, as AC-1 |
| AC-3 | Mid-EAP, a request sealed under the peer's keys whose inner chain fails to parse | Responder ends the handshake (`StateDead`); the counter does not move | the existing StateDead on an authenticated malformed request stays, because the message is the peer's own |
| AC-4 | Fifty forgeries mid-EAP, then the handshake aged past `responderHandshakeTimeout` | `CreatedAt` and the next Message ID are unchanged after the flood; the reaper then ends the handshake | the reaper's teardown is the intended end of an abandoned handshake |
| AC-5 | Against strongSwan: control exchange, then twenty forgeries while Ze waits mid-EAP | Counter rises by exactly twenty; after the hold lifts the tunnel establishes and passes traffic | none |
| AC-6 | A genuine EAP exchange between two daemons, and a retransmission at the cached id mid-EAP | The exchange establishes; the retransmission is replayed from cache as before | existing `TestEapRtx*` tests and `ipsec-eap-md5-challenge.ci` unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An EAP-MSCHAPv2 client connects while an attacker injects forged IKE_AUTH | wire → `dispatchInbound` → `handleInbound` → `handleResponderEAP` → drop, then genuine rounds → established | interop `responder-eap-survives-unprotected-auth` |
| 2 | An operator configures EAP on both Ze ends and the tunnel establishes | daemon config → `handleAuthRequest` → `handleResponderEAP` → established | `test/ipsec/ipsec-eap-md5-challenge.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResponderEAPSurvivesUnprotectedIKEAuth` | `internal/component/ike/engine/rfc7296_unprotected_handshake_test.go` | AC-1, RFC7296-2.4-3 negative | |
| `TestResponderAuthSurvivesUnprotectedIKEAuth` | same | AC-2, RFC7296-2.4-3 negative | |
| `TestResponderEAPEndsOnAuthenticatedMalformedRound` | same | AC-3, RFC7296-2.4-3 positive | |
| `TestUnprotectedInputDoesNotExtendTheHandshake` | same | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the change takes no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-md5-challenge` | `test/ipsec/ipsec-eap-md5-challenge.ci` | AC-6 non-regression: two daemons, Ze responder as EAP authenticator, the changed handlers still carry a genuine exchange to an established SA. The `.ci` runner cannot inject a forged IKE datagram mid-handshake, so the forgery proof through the daemon socket is the interop scenario | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `responder-eap-survives-unprotected-auth` | `test/interop-ipsec/scenarios/` | strongSwan | AC-5: forged IKE_AUTH mid-EAP is counted, never acted on, and the real client completes | |

## Files to Modify
- `internal/component/ike/engine/responder.go` - `handleAuthRequest` calls the drop helper
- `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP` calls the drop helper
- `internal/le/interoplab/ipsec/checkers.go` - scenario table row
- `docs/architecture/ike/ipsec-14-responder.md` - trap and proof paragraphs
- `ai/doors/access-security.md` - trap row
- `plan/journal/unprotected-message-changes-sa-state.md` - 2026-08-14 row fixed

## Files to Create
- `internal/component/ike/engine/responder_unauthenticated.go` - `discardUnauthenticatedHandshakeRequest`
- `internal/component/ike/engine/rfc7296_unprotected_handshake_test.go` - the four unit tests
- `internal/le/interoplab/ipsec/unprotected_eap.go` - the interop checker
- `test/interop-ipsec/scenarios/responder-eap-survives-unprotected-auth/` - `swanctl.conf`, `ze.conf`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | N-A | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no config |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new dependency |
| Prometheus counters/metrics | Yes | new reason `handshake-unauthenticated` on `ze_ipsec_error_notify_suppressed_total` |
| BGP family surface | N-A | IKE |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/ipsec-eap-authentication.md` Interop row gains the scenario once it passes |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | `docs/guide/ipsec.md` names the series and "the name of that guard"; it lists no reasons to extend |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/discrimination/rfc7296/RFC7296-2.4-3.json` gains three records |
| 10 | Test infrastructure changed? | No | one scenario and checker, discovered by the existing catalog |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-14-responder.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | a new label value of an existing series, described in `ipsec-14-responder.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `ipsec-14-responder.md` (Design header of `responder.go`, updated) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring** -- the helper is called from both handlers, reached through `handleInbound`
   - Tests: `TestResponderEAPSurvivesUnprotectedIKEAuth`, `TestResponderAuthSurvivesUnprotectedIKEAuth`
   - Files: `responder_unauthenticated.go`, `responder.go`, `responder_eap.go`
   - Verify: red with the helper's body disabled, green with it
2. **Phase: Distinction and bound** -- authenticated malformed still ends; no refresh of the handshake
   - Tests: `TestResponderEAPEndsOnAuthenticatedMalformedRound`, `TestUnprotectedInputDoesNotExtendTheHandshake`, `ipsec-eap-md5-challenge.ci`
   - Files: same
3. **Phase: Interop** -- strongSwan scenario
   - Tests: `responder-eap-survives-unprotected-auth`
   - Files: `unprotected_eap.go`, `checkers.go`, scenario directory
   - Owed after the interop run: the `features/ipsec-eap-authentication.md`
     Interop row gains `responder-eap-survives-unprotected-auth` only once the
     scenario has passed green and gone red with the fix reverted (the body of
     `discardUnauthenticatedHandshakeRequest` set to `return false`, Ze image
     rebuilt). The fix commit lands without that row; the row rides with the
     recorded interop evidence.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | `errInnerParse` and `eapMessageDiscarded` keep their branches and order |
| Data flow | no SA field written on the drop path |
| Rule: refuse-by-default | the dropped error is counted, so it is recorded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| helper called from both handlers | `grep -n discardUnauthenticatedHandshakeRequest internal/component/ike/engine/*.go` |
| four unit tests green | scoped `go test -run` of the engine package |
| genuine daemon EAP exchange green | `./le test functional ipsec` selecting `ipsec-eap-md5-challenge` |
| interop scenario green, red with the fix reverted | `./le test integration interop-ipsec` with `IPSEC_INTEROP_SCENARIO=responder-eap-survives-unprotected-auth` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | an unauthenticated datagram changes no SA field and draws no datagram |
| Resource exhaustion | `CreatedAt` untouched, so the reaper still frees the slot; the token bucket bounds the drop rate |
| Log injection | the drop logs at debug only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| Interop red | read charon's and Ze's logs; check A-2 and A-3 |
| 3 fix attempts failed | STOP. Report all 3 approaches. |

## Design Insights

- The responder already had one handler (`handleResponderAuthRound`) that dropped and kept; the two older handlers were the outliers of a pair, the class in `plan/journal/guard-added-to-one-half-of-a-pair.md`.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/unprotected-message-changes-sa-state.md` | 2026-08-14 | ike responder EAP round |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Drop non-`errInnerParse` errors, keep StateDead for `errInnerParse` | drop every error, as `handleResponderAuthRound` does | an authenticated malformed request is the peer's own and the spec requires the distinction |
| Count under `ze_ipsec_error_notify_suppressed_total` | a new metric | the drop withholds the error notification RFC 7296 Section 2.21.2 allows only in a protected response; one series answers "what did the guards stop" |
| Debug log plus counter | WARN per datagram | a forged datagram must not write the operator's log |
| Fix `handleAuthRequest` too | only `handleResponderEAP` | the same shape one state earlier; a fix on one half of a pair is the recorded failure class |

## Known Limitations
- The initiator half (`fsm.go`) is owned by `plan/release/validation/spec-rfc-verdict-fix-ike-eap.md` (journal row 2026-09-29).

## RFC Documentation (Scope: protocol)

`responder_unauthenticated.go` quotes RFC 7296 Section 2.4 above the helper; both call sites name RFC 7296 Section 2.4.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
