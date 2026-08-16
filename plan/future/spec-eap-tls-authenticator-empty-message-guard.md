# Spec: eap-tls-authenticator-empty-message-guard

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze's EAP-TLS authenticator answers an empty EAP-TLS message with another empty EAP-TLS
message, in a round where RFC 5216 sanctions neither.**

The peer answers the Start with a bare flags octet. `tlsMethod.Process`
(`internal/component/ike/eap/eap_tls.go`) reaches its last branch with no engine output and
replies with `TypeData: []byte{0}`, which is the fragment-acknowledgement form. The peer
can repeat this, and Ze answers each round the same way until the stale-handshake reaper
ends the exchange.

**This is hardening, not a deviation.** The evidence is in "RFC Documentation" below. RFC
5216 commands the bare acknowledgement in one case. It states no obligation to refuse one
in any other case, and the document carries no MUST NOT that reaches this path. The spec is
filed here for that reason. `plan/future/README.md` refuses defects, and this is not one.

The find came out of `spec-fixit-eap-tls-clienthello-race`, whose fix removed the
same wrong answer from the PEER half. That half now reports `errTLSClientStalled` instead
of an acknowledgement. The authenticator half kept its acknowledgement deliberately, and
this spec is the question that decision left open.

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5216.md` - the EAP-TLS message flow and its compliance checklist
  → Constraint: `[RFC5216-2.1.5-1]` is the only checklist row about the empty message. It
    commands the acknowledgement rather than restricting it
  → Constraint: `[RFC5216-2.1.3-4]` and `[RFC5216-2.1.3-8]` own the two other rounds where
    a no-data EAP-Response is legitimate. A new refusal MUST NOT reach either one

**Key insights:**
- The obligation in Section 2.1.5 is conditional on the M flag. It is a command for one
  case, not a permission that excludes the rest.
- Every legitimate no-data round is handled by a branch ABOVE the one this spec changes.

## Current Behavior (MANDATORY)

**Source files read on 2026-08-10:**
- [ ] `internal/component/ike/eap/eap_tls.go` - `tlsMethod.Start` sends the Start with the S
  flag and no data. `tlsMethod.Process` dispatches the peer's answer through six branches in
  this order. The fragment acknowledgement, the alert reply, reassembly, the handshake
  error, the completed handshake, and last the empty branch that returns `[]byte{0}`
- [ ] `internal/component/ike/eap/eap.go` - `Session.handleMethod` calls `Process` and turns
  its `MethodResult` into the outbound packet or into an EAP-Failure

**Behavior to preserve:**
- The fragment acknowledgement Ze sends when the peer sets the M flag.
- The EAP-Failure Ze sends after its own TLS alert, and the EAP-Success it sends after the
  peer's no-data answer to the closing flight.
- `eapTLSMaxPeerBuffered` stays reachable. `spec-fixit-eap-tls-clienthello-race`
  records the measurement. An unconditional guard in `Process` ends the exchange on the
  first message. No second message then crosses the backlog ceiling, and
  `TestEAPTLSProcessRefusesUnboundedPeerBuffer` goes red.

**Behavior to change:**
- The last branch of `tlsMethod.Process` stops answering an unsanctioned empty message with
  another one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An inbound `EAP-Response` with `Type=EAP-TLS` reaches `Session.handleMethod` (`eap.go`) and
is passed to `tlsMethod.Process` (`eap_tls.go`).

### Transformation Path

1. `Process` returns early for a fragment acknowledgement it is waiting for, and for the
   reply to an alert it already sent.
2. It reassembles the peer's fragments, answers an M-flagged message with a bare
   acknowledgement, and refuses a short message.
3. It feeds whatever it reassembled to the TLS engine and waits for the engine to settle.
4. With no engine error, no completed handshake and no engine output, it returns a bare
   flags octet. That is the branch this spec changes.

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| IKE -> EAP session | in | The peer's EAP-Response |
| EAP session -> TLS method | in | The packet, through `Process` |
| TLS method -> IKE | out | Today a bare acknowledgement, after this spec an error |

### Integration Points

`Session.handleMethod` already turns a `MethodResult.Err` into an EAP-Failure, so the new
refusal needs no new plumbing. `internal/component/ike/engine` reports the failed method to
the operator through the existing IKE path.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | The refusal is returned as `MethodResult.Err`, the one channel `handleMethod` reads |
| No unintended coupling | Yes | The change stays inside `internal/component/ike/eap` |
| No duplicated functionality | Yes | It replaces a branch rather than adding a parallel one |
| Zero-copy preserved where applicable | N-A | No buffer ownership changes |
| Registration over hardcoding | N-A | No new command, view, family or handler |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Every legitimate no-data round is answered by a branch above the last one | The branch comment in `eap_tls.go` calls the last branch defensive, and names the branches that absorb each real case | A conforming peer is refused mid-exchange | Run the eap package tests and `test/ipsec-interop/scenarios/04-eap-tls` with the refusal in place | unvalidated |
| A-2 | `eapTLSMaxPeerBuffered` needs at least one more round after the first empty message | The measurement in `spec-fixit-eap-tls-clienthello-race` | The backlog ceiling becomes unreachable again | `TestEAPTLSProcessRefusesUnboundedPeerBuffer` | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A conforming peer implementation sends one empty message Ze has not accounted for, and the refusal breaks a working exchange | Scenario 04 fails, or an operator reports a method failure with a peer that used to authenticate | Count consecutive empty rounds and refuse on the second, rather than on the first |
| R-2 | The refusal makes `eapTLSMaxPeerBuffered` unreachable, exactly as the unconditional guard did | `TestEAPTLSProcessRefusesUnboundedPeerBuffer` goes red | The counting design in R-1 leaves the second round available to the ceiling |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An EAP-TLS peer that used to authenticate is refused, so its IKE_AUTH fails and no SA comes up |
| How is it reverted? | Single commit revert. No config, no wire state survives it |
| Who else touches this path? | `spec-fixit-eap-tls-clienthello-race` changed the peer half of the same package |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an EAP-Response with `Type=EAP-TLS` and no data, in answer to the Start | → | the last branch of `tlsMethod.Process` | `TestEAPTLSProcessRefusesUnsanctionedEmptyMessage` |
| the same message through two daemons | → | `Session.handleMethod` turning the error into an EAP-Failure | `test/ipsec/ipsec-eap-tls-empty-answer.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The peer answers the Start with an empty EAP-TLS message | The authenticator ends the method with a named error, and the session sends an EAP-Failure |
| AC-2 | The peer sets the M flag | The authenticator still answers with a bare fragment acknowledgement |
| AC-3 | The peer answers the closing flight with a no-data EAP-Response | The authenticator still answers with EAP-Success |
| AC-4 | The peer replies to the authenticator's TLS alert | The authenticator still answers with EAP-Failure carrying the recorded cause |
| AC-5 | A peer sends more data than `eapTLSMaxPeerBuffered` allows | The backlog ceiling still refuses it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs an IKEv2 responder with EAP-TLS and meets a peer that never sends a ClientHello | IKE -> `Session.handleMethod` -> `tlsMethod.Process` -> EAP-Failure | `test/ipsec/ipsec-eap-tls-empty-answer.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 2 | Runs the same responder with a conforming peer | the unchanged branches of `Process` | `test/ipsec/ipsec-eap-tls-clienthello.ci` and scenario 04 |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEAPTLSProcessRefusesUnsanctionedEmptyMessage` | `internal/component/ike/eap/eap_tls_empty_message_test.go` | AC-1 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestEAPTLSProcessStillAcknowledgesAnMFlaggedMessage` | `internal/component/ike/eap/eap_tls_empty_message_test.go` | AC-2 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess` | existing | AC-3, unchanged | |
| `TestEAPTLSProcessRefusesUnboundedPeerBuffer` | existing | AC-5, unchanged | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| consecutive empty rounds, if the counting design is chosen | 0 to the chosen limit | the limit | N/A | the limit plus one |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-eap-tls-empty-answer` | `test/ipsec/ipsec-eap-tls-empty-answer.ci` | A peer that answers the Start with an empty message is refused, and the operator sees a failed method rather than a hung exchange | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

The peer half no longer produces this message, so the `.ci` needs a peer that does. The
cheapest seam is a private test env var on the peer half, in the shape
`internal/component/ike/engine/testport.go` already uses for `ze.test.ike.port` and
`ze.test.ike.dataplane`. Settle the seam in Phase 1, before you write the test.

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `04-eap-tls` | `test/ipsec-interop/scenarios/` | strongSwan | A conforming peer still authenticates after the refusal lands | |

strongSwan sends a ClientHello, so it cannot exercise AC-1. It is the regression guard for
AC-2 through AC-5, and `plan/journal/shared-leniency-hides-the-defect.md` records why a
ze-to-ze test cannot replace it for a wire-form claim.

## Files to Modify

- `internal/component/ike/eap/eap_tls.go` - refuse the unsanctioned empty message in the
  last branch of `tlsMethod.Process`, and update the branch comment that documents the
  acknowledgement
- `internal/component/ike/eap/peer.go` - only if the test seam for the misbehaving peer
  lands here

## Files to Create

- `internal/component/ike/eap/eap_tls_empty_message_test.go` - the unit tests above <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- `test/ipsec/ipsec-eap-tls-empty-answer.ci` - the functional test above <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No operator-visible setting is added |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | No | The failure is reported through the existing IKE surface |
| CLI grammar | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-eap-tls-empty-answer.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| Pipe completeness | N-A | No new output |
| Env var registration | Yes, if the test seam is an env var | `env.MustRegister` beside the existing IKE test keys, marked private |
| Doctor check for runtime dependencies | No | No new file, socket, port or binary |
| Prometheus counters/metrics | Yes | A counter for refused EAP-TLS methods is worth a row. Decide in Phase 2 |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The change refuses a message that never came from a working peer |
| 7 | Wire format changed? | Yes | `docs/architecture/ike/` EAP-TLS page, if it describes this round |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5216.md` gains no new requirement. The reading of Section 2.1.5 belongs beside `[RFC5216-2.1.5-1]` |
| 12 | Internal architecture changed? | No | One branch inside one method |
| 16 | Changed source file referenced by doc source anchors? | Check | Grep `docs/` for anchors naming `eap_tls.go` |

Rows not listed do not apply: the change adds no config, no CLI, no plugin, no route
metadata and no daemon comparison claim.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - settle the test seam that lets a peer send the
   unsanctioned empty message, and write the two wiring tests so they fail.
   - Tests: `TestEAPTLSProcessRefusesUnsanctionedEmptyMessage`, `ipsec-eap-tls-empty-answer`
   - Files: the seam, `eap_tls_empty_message_test.go`, the `.ci`
   - Verify: both fail because `Process` still answers with a bare acknowledgement
2. **Phase: The refusal** - replace the branch, keeping `eapTLSMaxPeerBuffered` reachable.
   - Tests: the two above, plus the four existing tests named in the TDD plan
   - Files: `internal/component/ike/eap/eap_tls.go`
   - Verify: the new tests pass, no existing eap test goes red, scenario 04 still completes

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 to AC-5 each have a test that fails when the branch is reverted |
| Correctness | The refusal names the message it refused and the round it arrived in |
| Data flow | The refusal travels as `MethodResult.Err`, never as a `Response` beside an error, which `handleMethod` discards |
| Rule: `ai/rules/rfc-compliance.md` | The three legitimate no-data rounds keep their answers, each proven by its own test |
| Rule: `ai/rules/evidence.md` | The guard fails closed and its zero value is not a valid-looking answer |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The refusal in `tlsMethod.Process` | `make ze-unit-pkg-test PKG=./internal/component/ike/eap` |
| The functional test | `make ze-functional-ipsec-test` |
| The interop regression | `make ze-interop-ipsec-test IPSEC_INTEROP_SCENARIO=04-eap-tls` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | An unauthenticated peer chooses how many rounds it drives. Count the rounds and refuse, rather than leaving the reaper as the only bound |
| Error leakage | The failure text reaches an unauthenticated peer only as EAP-Failure, which carries no text. Keep the detail in the log |

### Failure Routing

| Failure | Route To |
|---------|----------|
| An existing eap test goes red | A-1 is broken. Re-read the branch it exercises, then decide about the test |
| Scenario 04 fails | R-1 is real. Move to the counting design |
| The backlog ceiling test goes red | A-2 is broken. The refusal is too early |

## Design Insights

The peer half and the authenticator half of the same package made the same wrong answer for
different reasons. The peer half was a race, fixed in
`spec-fixit-eap-tls-clienthello-race`. The authenticator half is a leniency, and it
is what makes a ze-to-ze functional test blind to the peer-half defect.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| File in `plan/future/` | File in `plan/` as a defect | RFC 5216 states no obligation to refuse the message, so the wire form Ze produces is permitted. `plan/future/README.md` names the defect classes, and this matches none |
| Refuse rather than ignore | Drop the message silently | A silent drop leaves the peer waiting and the exchange to the reaper, which is the failure mode this spec removes |
| Decide first-round versus counting refusal in Phase 2 | Pick now | `eapTLSMaxPeerBuffered` needs a second round, and only the tests can say whether one design serves both |

## Known Limitations

- No test peer sends this message today, so AC-1 needs a seam built for it.
- strongSwan cannot exercise AC-1, so the interop scenario stays a regression guard.

## RFC Documentation (Scope: protocol)

RFC 5216 Section 2.1.5 states the obligation this spec is measured against:

```
   Similarly, when the EAP server receives an EAP-Response with the M
   bit set, it MUST respond with an EAP-Request with EAP-Type=EAP-TLS
   and no data.  This serves as a fragment ACK.
```

The MUST is conditional on the M bit. It commands the bare acknowledgement for that case and
says nothing about any other case. RFC 5216 carries no MUST NOT that reaches an empty
message outside it. Section 2.1.3 governs the one other empty EAP-Response. It puts the
obligation on the answer, which is an EAP-Failure, not on a refusal. So Ze's acceptance of
an unsanctioned empty message is receive-side leniency the RFC does not forbid.

Add `// RFC 5216 Section 2.1.5: "<quoted requirement>"` above the branch that keeps the
acknowledgement for the M-flagged case, so the next reader sees which case the RFC commands.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated, not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm` the spec
