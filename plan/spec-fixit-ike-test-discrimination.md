# Spec: fixit-ike-test-discrimination

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Four places in the IKE and IPsec test surface report green without testing the
thing they name.**

Found on 2026-08-02 while closing `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. Two were
measured by reverting the production guard and re-running. Two were read.

`ai/rules/interop-and-goal-validation.md` states the standard this spec applies: a
passing test is evidence only if it would FAIL when the behavior under test is
broken. Each item below fails that standard, so the coverage it appears to give is
not there.

### Item 1: the RFC 2759 one-octet guard survives its own revert

`mschapv2Method.Process` (`internal/component/ike/eap/eap_mschapv2.go`) refuses
an empty response at `:89`, guarding the `response.TypeData[0]` read at `:93`. Its
comment explains the floor carefully, including why a blanket four-octet floor was
wrong.

Reverting that guard leaves the WHOLE `eap` package green. No test drives an
MSCHAPv2 response with an empty `TypeData`, so nothing reaches the panic the guard
prevents. MEASURED on 2026-08-02.

### Item 2: the pendingRekey clear defer survives its own revert

`runEstablished` (`internal/component/ike/engine/established.go`) clears the
session rekey slot in a deferred call at `:71`, with a comment stating it is safe
there and only there.

Reverting that defer leaves the whole `engine` package green. MEASURED on
2026-08-02.

This one is UNPROVEN HARDENING rather than a false claim, and the distinction
matters. `RFC7296-2.12-1` is tagged over `forgetKeys`, and those tags describe what
they actually prove. No public compliance claim outruns its evidence here. What is
missing is a test for the clear itself.

### Item 3: two EAP-TLS interop scenarios never assert ESP is accepted

`04-eap-tls/check.py` and `06-eap-tls13/check.py` both stop at `wait_xfrm_sa` for
each container and then call `log_pass`. Presence of an XFRM SA is necessary and
not sufficient: it proves the SAs were installed, never that ESP traffic is
accepted across them.

Scenario `07-responder-psk/check.py` is the in-tree control and shows the fix
already exists: it pings across the tunnel and then reads the ESP counters, with a
comment recording which counter advances and which does not. Scenario `02` follows
the same pattern by carrying BGP routes through the tunnel.

So this is a back-fill of a pattern the tree already has, not a new technique. READ,
not measured.

### Item 4: a loopback probe is a blind instrument

A loopback egress moves NO xfrm counter. Any future QEMU probe built on `lo` will
therefore report zero whether the dataplane works or not, which is the
absence-assertion vacuity trap named in `ai/rules/interop-and-goal-validation.md`.

This item is a CONSTRAINT rather than a defect: nothing in the tree is wrong today.
It is recorded here so the next author of an xfrm probe reads it before choosing an
interface, and so that author pairs any counter assertion with a positive control
that is known to move the counter.

`ai/rules/platform-linux.md` is arguably the better long-term home for this
constraint. Moving it there is a rule change and belongs to the owner, so this spec
records it and does not edit the rule.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/interop-and-goal-validation.md` - the discrimination standard and the vacuity traps
  → Constraint: revert the behavior under test and confirm the test FAILS, before claiming it validates anything
- [ ] `ai/rules/testing.md` - test sensitivity ratchets and the assert-nothing detector
  → Constraint: a test whose oracle is implicit needs the `// test-asserts-nothing:` annotation, so silence is never the answer
- [ ] `ai/rules/platform-linux.md` - what the Alpine VM provides and the virtual substitutes
  → Constraint: a dataplane assertion needs an interface that actually carries the traffic, which is why item 4 exists

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2759.md` - MSCHAPv2 packet shapes, which set the one-octet floor
  → Constraint: the Success Response carries only an OpCode, so the floor is one octet and not four
- [ ] `rfc/short/rfc7296.md` - IKE rekey and the SA lifecycle the pending slot tracks
  → Constraint: `RFC7296-2.12-1` is already tagged over `forgetKeys` and its tags are honest, so item 2 adds coverage without changing a claim

**Key insights:** (minimal context to resume after compaction)
- Items 1 and 2 are measured. Items 3 and 4 are read.
- Item 2 is unproven hardening, NOT a false compliance claim. Say so, and do not narrow any RFC row for it.
- Scenario `07-responder-psk` is the in-tree control for item 3. Copy it.
- Item 4 changes no code. It is a constraint for the next probe author.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/eap/eap_mschapv2.go` - `Process` at `:82`, the empty-response guard at `:89`, the `TypeData[0]` read at `:93`
- [ ] `internal/component/ike/engine/established.go` - `runEstablished` at `:56`, the `pendingRekey.clear()` defer at `:71`
- [ ] `test/ipsec-interop/scenarios/04-eap-tls/check.py` - stops at `wait_xfrm_sa` then `log_pass`
- [ ] `test/ipsec-interop/scenarios/06-eap-tls13/check.py` - same shape
- [ ] `test/ipsec-interop/scenarios/07-responder-psk/check.py` - the control: pings, then reads ESP counters

**Behavior to preserve:** (unless the user explicitly said to change it)
- Both production guards stay exactly as they are. This spec adds tests, it does not change the guards.
- The `RFC7296-2.12-1` tags stay as they are. They describe `forgetKeys` honestly and no claim is being corrected.
- Scenarios `02` and `07` already discriminate and must not be disturbed.
- Every existing `RFC requirement:` tag keeps its id and polarity. `ai/rules/testing.md` makes a tagged test the requirement itself.

**Behavior to change:** (only what the user asked for)
- Add a test that reddens when the RFC 2759 one-octet guard is reverted.
- Add a test that reddens when the `pendingRekey` clear defer is reverted.
- Extend scenarios `04` and `06` to assert ESP is accepted, following scenario `07`.
- Record the loopback constraint where a probe author will read it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An EAP-MSCHAPv2 response arrives from a peer, carrying an attacker-controlled `TypeData` length.
- An established IKE session enters and leaves `runEstablished`, carrying a session-owned rekey slot.
- An interop lab brings up a tunnel between ze and strongSwan.

### Transformation Path
1. EAP response parsed by `Process`, which reads `TypeData[0]` after the length guard.
2. Session rekey slot set by the timer paths, cleared by the deferred call on loop exit.
3. Interop scenario waits for the XFRM SA, then asserts on the dataplane.
4. ESP counters read from both containers to prove traffic crossed the tunnel.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer ↔ EAP method | EAP packet with peer-controlled length | No |
| Session ↔ rekey slot | in-process struct field, cleared on loop exit | No |
| Ze ↔ strongSwan | ESP over the interop lab network | No |
| Container ↔ kernel | `ip xfrm state` counters read per container | No |

### Integration Points
- `internal/component/ike/eap` - the method registry the MSCHAPv2 method is reached through.
- `internal/component/ike/engine` - the owner loop that holds the rekey slot.
- `test/ipsec-interop/` - the shared scenario helpers, including `wait_xfrm_sa`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An empty-`TypeData` MSCHAPv2 response is reachable from the wire, not only from a constructed unit input | read, the EAP receive path | Item 1 is a defensive guard with no reachable trigger, which lowers severity but still leaves it unproven | trace the EAP packet path from the transport to `Process` | unvalidated |
| A-2 | Reverting each guard really does leave the package green, and the run was not scoped too narrowly | measured 2026-08-02, both packages | The coverage exists and one of the two items closes immediately | re-run the revert with the full feature tags per `ai/rules/commands.md` | unvalidated |
| A-3 | ESP counters in scenarios `04` and `06` behave as they do in `07`, so the control transfers | read, `07-responder-psk/check.py` | The assertion needs a different shape for the EAP-TLS labs | run the extended scenarios once | unvalidated |
| A-4 | No other test already covers either guard from a different package | measured for the two packages, not tree-wide | An existing test covers it and the new one duplicates | grep the tree for the guard's behavior before writing the test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The new EAP test asserts the error string rather than the behavior, so a reword breaks it | the test fails on a message change that keeps behavior | assert that no panic occurs and a method failure is returned, never the exact text |
| R-2 | Extending scenarios `04` and `06` makes them flaky, since interop is nightly and advisory | intermittent reds in the nightly tier | wait on the counter condition, never on elapsed time (`ai/rules/completion.md`) |
| R-3 | Item 4's constraint stays in this spec and is deleted at closure, so the next probe author never sees it | a future probe built on `lo` | route the constraint to a durable home at closure: the learned summary at minimum, `ai/rules/platform-linux.md` if the owner agrees |
| R-4 | Adding assertions to a scenario carrying an `RFC requirement:` tag trips the rfc-tagged-test hook | the edit is blocked at write time | check each scenario for tags first; a tagged change needs the owner's approval, never a self-issued one |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. This spec adds tests and changes no production behavior. The risk is a flaky nightly interop scenario, which costs signal rather than function. |
| How is it reverted? | Single commit revert. Test-only. |
| Who else touches this path? | `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md` (evidence carriers and tiers), `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md` (which interop trees have an automated caller) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| EAP response with empty TypeData | → | `mschapv2Method.Process` length guard | `TestMSCHAPv2ProcessRefusesEmptyTypeData` |
| Established loop exit | → | `runEstablished` pendingRekey clear | `TestRunEstablishedClearsPendingRekeyOnExit` |
| EAP-TLS tunnel up | → | ESP counters on both peers | `test/ipsec-interop/scenarios/04-eap-tls/check.py` |
| EAP-TLS 1.3 tunnel up | → | ESP counters on both peers | `test/ipsec-interop/scenarios/06-eap-tls13/check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An MSCHAPv2 response whose `TypeData` is empty | `Process` returns a method failure and does not panic |
| AC-2 | The one-octet guard is reverted | AC-1's test FAILS, proving it discriminates |
| AC-3 | `runEstablished` returns with a rekey pending | The session rekey slot is clear afterwards |
| AC-4 | The clear defer is reverted | AC-3's test FAILS, proving it discriminates |
| AC-5 | Scenario `04-eap-tls` runs against strongSwan | ESP traffic is proven accepted, not only that an XFRM SA exists |
| AC-6 | Scenario `06-eap-tls13` runs against strongSwan | Same as AC-5 |
| AC-7 | A reader plans a QEMU xfrm probe | The loopback constraint and the positive-control requirement are recorded where that reader looks |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Authenticates with EAP-TLS and sends traffic | IKE_AUTH → Child SA → ESP across the tunnel | `04-eap-tls/check.py` |
| 2 | Authenticates with EAP-TLS 1.3 and sends traffic | same, over TLS 1.3 | `06-eap-tls13/check.py` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMSCHAPv2ProcessRefusesEmptyTypeData` | `internal/component/ike/eap/eap_mschapv2_test.go` | AC-1, AC-2 | |
| `TestRunEstablishedClearsPendingRekeyOnExit` | `internal/component/ike/engine/established_test.go` | AC-3, AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MSCHAPv2 `TypeData` length | 1..N octets | 1 (OpCode only, the Success Response) | 0 (empty, the guard's case) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing IPsec suite | `test/ipsec/*.ci` | No new `.ci` is expected. Confirm at design time whether the EAP guard is reachable from a `.ci`, and add one if it is | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `04-eap-tls` | `test/ipsec-interop/scenarios/` | strongSwan | ESP accepted across an EAP-TLS tunnel, not only SA presence | |
| `06-eap-tls13` | `test/ipsec-interop/scenarios/` | strongSwan | Same, over TLS 1.3 | |

## Files to Modify
- `scripts/evidence/` - the positive-control helper or its header comment, so item 4's constraint reaches a probe author
- `test/ipsec-interop/scenarios/04-eap-tls/check.py` - assert ESP is accepted, following scenario `07`
- `test/ipsec-interop/scenarios/06-eap-tls13/check.py` - same
- `internal/component/ike/eap/eap_mschapv2_test.go` - the discriminating test for the one-octet guard
- `internal/component/ike/engine/established_test.go` - the discriminating test for the clear defer

## Files to Create
- none expected. Confirm at design time whether a shared ESP-counter assertion helper belongs in the interop library rather than in each scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test-only spec |
| YANG validation constraints | N-A | Test-only spec |
| YANG custom validators | N-A | Test-only spec |
| CLI commands/flags | N-A | Test-only spec |
| CLI grammar (keyword before value) | N-A | Test-only spec |
| Editor autocomplete | N-A | Test-only spec |
| Functional test for new RPC/API | N-A | No new RPC or API |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No new counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Test discrimination only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | | Item 2 adds coverage without changing a claim. Confirm at design time that no `docs/features/rfc-status.md` row moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if a shared interop assertion helper lands |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for the two scenario paths at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - prove each item discriminates before fixing it
   - Tests: the two unit tests, written to FAIL only when the guard is present, then reverted to confirm
   - Files: the two `_test.go` files
   - Verify: each new test reddens when its guard is reverted, which is AC-2 and AC-4
2. **Phase: Interop assertions** - extend scenarios `04` and `06`
   - Tests: `04-eap-tls/check.py`, `06-eap-tls13/check.py`
   - Files: both `check.py` files, and any shared helper
   - Verify: check each scenario for an `RFC requirement:` tag FIRST (R-4). Wait on the counter, never on elapsed time
3. **Phase: Record the constraint** - item 4
   - Tests: none. This phase produces text, not behavior
   - Files: `scripts/evidence/`
   - Verify: a probe author reading the evidence helpers meets the loopback constraint and the positive-control requirement

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | All four items addressed, and item 4 has a durable home rather than only this spec |
| Correctness | Each new test was actually run against the reverted guard, and the revert was undone afterwards |
| Naming | Test names state the behavior asserted, not the mechanism |
| Data flow | The interop assertion reads a counter that the traffic really moves, per item 4 |
| Rule: `ai/rules/interop-and-goal-validation.md` | No new assertion is an absence assertion |
| Rule: `ai/rules/testing.md` | No tagged test changed behavior without the owner's approval |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| EAP guard discriminates | Revert the guard, run `go test ./internal/component/ike/eap/`, confirm RED, restore |
| Rekey clear discriminates | Revert the defer, run `go test ./internal/component/ike/engine/`, confirm RED, restore |
| Scenarios assert ESP | `make ze-ipsec-interop-test` with scenarios 04 and 06 |
| Constraint recorded | `grep -rn "loopback" scripts/evidence/` returns the note |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Item 1 IS an input-validation guard on peer-controlled length. The test must drive it from the peer-facing entry point, not the helper alone (`ai/rules/evidence.md`) |
| Resource exhaustion | None introduced |
| Error leakage | The new EAP test must not pin an error string that reveals internal state |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Item 4 changes no code. If it stays only in this spec it dies at closure, which R-3 tracks.
- Interop scenarios run in the nightly tier and are advisory, so items 3's proof does not gate a merge.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
