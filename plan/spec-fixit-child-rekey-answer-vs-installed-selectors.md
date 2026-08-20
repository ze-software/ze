# Spec: fixit-child-rekey-answer-vs-installed-selectors

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 2/3 |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

It records an RFC 7296 Section 2.9 violation found on 2026-08-19 while closing
`spec-fixit-child-sa-rekey-policy`. An earlier draft held the spec back for a
question to Thomas, on the reading that two conformant answers existed. That
reading was wrong on the RFC text, and the question was never owed:
Section 2.9.2 determines the answer, and `ai/rules/rfc-compliance.md` bans
putting full compliance beside a narrower option for the owner to choose
between. **Key Design Decisions** below carries the governing sentences.

## Task

**A Child SA rekey Ze responds to can announce one selector set on the wire and
install a different one in the kernel.**

RFC 7296 Section 2.9: "TS payloads specify the selection criteria for packets
that will be forwarded over the newly set up SA." The answer Ze sends is
therefore a statement about the SA it just installed. It is not one today.

Two producers disagree, and both were read on 2026-08-19:

- `respondChildRekey` (`internal/component/ike/engine/rekey.go`) builds the
  response TS payloads from `sa.NegotiatedPairs`, which `narrowChildSelectors`
  (`internal/component/ike/engine/ts_narrow.go`) has just overwritten with the
  narrowing THIS exchange produced.
- `newRekeyedChild` (`internal/component/ike/engine/rekey.go`) installs the
  replacement Child SA with `old.Selectors`, inherited from the retired pair and
  never compared against the narrowing.

The two agree only while `narrowSelectors`
(`internal/component/ike/engine/ts_narrow.go`) answers the floor. That is its
first branch, taken when `floorWithinProposal` finds at least one floor pair
covered by the peer's proposal. When no floor pair is covered, the function falls
through to the empty-policy branch or to the intersection branch, and the
answered set is then derived from the peer's proposal instead of from the
installed selectors.

**The intersection branch is reachable from the wire.** `respondChildRekey`
refuses a request only for a missing Ni or ESP SPI (`errMalformedRequest`) and
for a proposal that matches no offered ESP proposal (`matchOfferedESPProposal`).
Nothing refuses a TS proposal that covers no floor pair. A peer whose configured
selectors narrowed since the tunnel came up sends exactly that at its next rekey,
which is the case Section 2.9 names: "the configurations of the two endpoints are
being updated but only one end has received the new information."

A second path reaches the same divergence. `respondChildRekey` calls
`narrowChildSelectors` only when both TSi and TSr are present in the request. A
rekey request carrying neither leaves `sa.NegotiatedPairs` holding the previous
exchange's answer, and the response announces that.

**What an operator sees.** The peer programs its SPD from the answer. Ze programs
its own from the retired pair's selectors. Traffic inside the difference is
protected at one end and dropped at the other, with no notification and no log
line, until an operator compares the two SPDs by hand.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA programming
      and policy ownership
  → Constraint: the engine speaks `dataplane.SPParams`, never netlink. The
    selector set and its orientation travel together on `ChildSA`.
- [ ] `docs/architecture/ike/ipsec-13-rekey-wire.md` - the rekey exchange on the
      wire
  → Constraint: make-before-break. The replacement carries traffic before the
    retired pair is deleted, so its selectors cannot be renegotiated late.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - Sections 2.9 and 2.9.2
  → Constraint: `[RFC7296-2.9-1]` permits narrowing and forbids widening.
    `[RFC7296-2.9.2-1]` and `[RFC7296-2.9.2-2]` forbid a rekey narrowing below
    the scope currently in use. Section 2.9's prose binds the payload to the SA:
    the selectors sent are the criteria for packets forwarded over that SA.

**Key insights:**
- The answer and the installation are two statements about one SA. They are
  produced from two variables today.
- The floor branch is what makes them agree, and it is not the only branch.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/rekey.go` - `respondChildRekey` refuses a
      missing Ni or ESP SPI, refuses an unoffered proposal, narrows against the
      floor, installs `newRekeyedChild`, then answers from `sa.NegotiatedPairs`
- [ ] `internal/component/ike/engine/ts_narrow.go` - `narrowSelectors` answers
      the floor first, then the empty-policy branch, then the intersection
      branch; `narrowChildSelectors` writes `sa.NegotiatedPairs`,
      `sa.NegotiatedTSi` and `sa.NegotiatedTSr`
- [ ] `internal/component/ike/engine/child.go` - `selectorPort` reads the
      installed selectors and `ChildSA.SelectorsLocalIsTSi`; `samePolicySelector`
      decides whether retiring a pair strips the live policy

**Behavior to preserve:**
- The Section 2.9.2 floor. A rekey never installs or announces a set narrower
  than the scope in use while the peer's proposal still covers it.
- `TestPeerInitiatedRekeyIsNarrowedInTheExchangeOrientation` and
  `TestRekeyKeepsThePolicyOrientationOfTheRetiredPair`: the orientation of the
  stored selectors is inherited, never derived from the rekey's exchange role.
- `test/ipsec/ipsec-child-rekey-xfrm.ci` on the real XFRM backend.

**Behavior to change:**
- The answer and the installed policy come from one value, or the exchange is
  refused. `narrowChildSelectors` (`internal/component/ike/engine/ts_narrow.go`)
  refuses a narrowing that does not cover the scope in use, so the only answer
  that reaches `pairsToWire` is the set `newRekeyedChild` installed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer's CREATE_CHILD_SA rekey request, decrypted on an established IKE SA.
- Format at entry: `wire.PayloadTS` for TSi and TSr, plus SA and Nonce payloads.

### Transformation Path
1. `classifyInbound` routes the request to `respondChildRekey`.
2. `respondChildRekey` builds the floor from `old.Selectors`, swapping it into
   this exchange's orientation when `old.SelectorsLocalIsTSi` is true.
3. `narrowChildSelectors` narrows the proposal against the configured policy and
   that floor, and writes `sa.NegotiatedPairs`.
4. `newRekeyedChild` builds the replacement from `old`, inheriting `Selectors`.
5. `installChildTolerant` programs states and policies from those selectors.
6. `pairsToWire` builds the response TS payloads from `sa.NegotiatedPairs`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ peer daemon | TSi and TSr payloads in the CREATE_CHILD_SA response | No |
| Engine ↔ dataplane | `dataplane.SPParams` built by `childPolicyParams` | No |

### Integration Points
- `narrowSelectors` - the single narrowing decision for every responder path.
- `newRekeyedChild` - the single builder of a replacement Child SA.
- `installChildTolerant` - the single install path for a rekeyed pair.

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
| A-1 | A peer can send a rekey proposal that covers no floor pair | RFC 7296 Section 2.9 names the split-configuration case; nothing in `respondChildRekey` refuses it | the divergence is unreachable and this spec closes as invalid | an interop scenario where the peer's selectors narrow between establishment and rekey | confirmed 2026-08-19: reached from wire payloads in `TestRekeyProposalBelowTheFloorIsRefused`. With the refusal disabled, `respondChildRekey` answered `10.2.0.128/25 <-> 10.1.0.0/24` while the replacement carried `10.2.0.0/24 <-> 10.1.0.0/24` |
| A-2 | Installing the narrowed set instead of the inherited one keeps the make-before-break window intact | `samePolicySelector` (`internal/component/ike/engine/child.go`) decides whether retiring the old pair strips the live policy | option B below breaks the tunnel it is meant to fix | a unit test on `samePolicySelector` over a narrowed replacement, plus `test/ipsec/ipsec-child-rekey-xfrm.ci` | moot 2026-08-19: it is option B's assumption, and Section 2.9.2 rejects option B. Nothing installs a narrowed set, so `samePolicySelector` is never asked about one |
| A-3 | Refusing with TS_UNACCEPTABLE is conformant when the proposal covers no floor pair | `[RFC7296-2.9.2-2]` forbids narrowing below the scope in use, and `[RFC7296-2.9-1]` names TS_UNACCEPTABLE for an empty result | option A below is not on the table | the RFC text of Section 2.9.2 read whole | confirmed 2026-08-19: `rfc/full/rfc7296.txt:2536-2552` read whole. The section states the MUST NOT and then states that such a request means the SA "should have been already deleted after the policy change took effect" |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Option B changes the installed policy selector at a rekey, which is what `spec-fixit-child-sa-rekey-policy` proved must not happen by accident | `samePolicySelector` answers false and the retiring pair strips the live policy | the replacement claims the new selector under the same `Owner` before the retired pair is removed, and the XFRM `.ci` measures it |
| R-2 | Option A refuses a rekey the peer will retry, so the tunnel hard-expires | the peer logs TS_UNACCEPTABLE and retries until expiry | the refusal is the conformant answer; a log line names both selector sets so an operator can act |
| R-3 | A fix that reads the installed set for the answer re-orients it wrongly | the answered TSi carries this node's side on a peer-initiated rekey | the answer is built from `ChildSA.Selectors` swapped by `ChildSA.SelectorsLocalIsTSi`, which is the pairing the closed spec established |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Traffic inside the disputed selector range is dropped at one end of a live tunnel, silently |
| How is it reverted? | Single commit revert. No config migration, no persisted state |
| Who else touches this path? | `spec-fixit-child-sa-rekey-policy` owns the orientation fields this fix reads |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer's CREATE_CHILD_SA rekey whose TS proposal covers no floor pair | → | `respondChildRekey` | `TestRekeyAnswerMatchesTheInstalledSelectors` |
| The same rekey on the real XFRM backend | → | `installChildTolerant`, `childPolicyParams` | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` | <!-- doc-links: ignore (test AC-4 of this spec will create; the spec is not authorised to run) -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer-initiated Child SA rekey whose TSi/TSr cover at least one floor pair | The answer and the installed policy selector name the same set, as they do today |
| AC-2 | A peer-initiated Child SA rekey whose TSi/TSr cover no floor pair | Ze either refuses the exchange or answers and installs one set. No path leaves the two disagreeing |
| AC-3 | A peer-initiated Child SA rekey carrying no TS payloads | Ze refuses it with INVALID_SYNTAX, rather than answering with the previous exchange's selectors |
| AC-4 | Every AC above, on the real XFRM dataplane | The kernel policy selector equals the set the response announced |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Narrows the traffic selectors on the peer daemon, then waits for its rekey timer | peer CREATE_CHILD_SA -> `respondChildRekey` -> `narrowSelectors` -> answer plus install | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` | <!-- doc-links: ignore (test AC-4 of this spec will create; the spec is not authorised to run) -->
| 2 | Runs a strongSwan peer whose child config narrows between establishment and rekey | strongSwan -> Ze responder -> XFRM policy | `test/interop-ipsec/scenarios/NN-child-rekey-narrowing` | <!-- doc-links: ignore (test AC-4 of this spec will create; the spec is not authorised to run) -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRekeyAnswerMatchesTheInstalledSelectors` | `internal/component/ike/engine/child_rekey_orientation_test.go` | the answered pairs equal the installed pairs, in both orientations | green 2026-08-19. It decrypts the response and reads the TS payloads, so it judges the wire answer rather than the value that feeds it |
| `TestRekeyProposalBelowTheFloorIsRefused` | `internal/component/ike/engine/child_rekey_orientation_test.go` | a proposal covering no floor pair draws TS_UNACCEPTABLE, not a silent divergence | green 2026-08-19, and red with the refusal disabled. It lives beside the test above rather than in `rekey_test.go`: the two share one fixture, and one concern belongs in one file |
| `TestRekeyWithoutTrafficSelectorsIsRefused` | `internal/component/ike/engine/rekey_test.go` | a rekey request with no TS payloads never reuses the previous exchange's answer | green 2026-08-19, and red against the pre-fix producer, which announced `10.1.0.0/24 <-> 10.2.0.0/24`: the previous exchange's set, in the orientation of THAT exchange. Four fixtures sent a TS-less rekey and asserted success (`TestRespondChildRekey`, `TestResSelectedDHGroupNoneOmitsKEFromTheResponse`, and the two callers of `rkyChildRekeyRequest`). Each now proposes selectors, which is what RFC 7296 Section 1.3.3 puts in the request |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Floor pairs covered by the proposal | 0..len(floor) | len(floor) | N/A | N/A |
| Selector port | 0-65535 | 65535 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-child-rekey-xfrm-narrowing` | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` | the peer narrows its selectors and rekeys; the kernel policy matches the answer | NOT WRITTEN. AC-4 is unproven on the real dataplane | <!-- doc-links: ignore (test AC-4 of this spec will create; the spec is not authorised to run) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-child-rekey-narrowing` | `test/interop-ipsec/scenarios/` | strongSwan | a peer that narrows between establishment and rekey ends with the same SPD as Ze | NOT WRITTEN |

## Files to Modify
- `internal/component/ike/engine/rekey.go` - `respondChildRekey` produces the
  answer and the installation from one value. DONE: the comment above
  `pairsToWire` states that invariant rather than the branch that used to hold
  it. DONE (AC-3): a request without TSi or TSr joins the malformed-request
  guard and draws INVALID_SYNTAX, so `narrowChildSelectors` runs for every
  request that reaches the answer. The `anyChildTSPayloads` fallback is gone,
  and a scope no TS payload can carry is refused with `errTSUnacceptable`, as
  the IKE_AUTH responder already refuses it (`initiator.go`).
- `internal/component/ike/engine/ts_narrow.go` - `narrowChildSelectors` refuses
  a narrowing that does not cover the scope in use. DONE: `coversFloor` is the
  new predicate, and the refusal reuses `errTSUnacceptable`, which
  `notifyForRefusal` already maps to TS_UNACCEPTABLE. `narrowSelectors` reports
  no branch: the caller asks about the RESULT instead, which needs no new
  return value and also covers the transport-mode filter that runs after it.
- `docs/architecture/ike/ipsec-13-rekey-wire.md` - the rekey answer is a
  statement about the SA that was installed. DONE: a new trap entry, "The wire
  answer against the installed policy".

## Files to Create
- `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` - the functional test above. <!-- doc-links: ignore (test AC-4 of this spec will create; the spec is not authorised to run) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no operator-visible setting changes |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | no new env var |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | already counted. `respondError` (`internal/component/ike/engine/notify_error.go`) calls `countErrorNotifySent`, so the refusal lands on `ze_ipsec_error_notify_sent_total` under the TS_UNACCEPTABLE type. A second counter would be a duplicate. `inbound.go` also logs the refusal at WARN, and the error names both selector sets |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a defect is removed |
| 2 | Config syntax changed? | No | no leaf changes |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | DONE: `docs/guide/ipsec.md`, "Traffic selectors and narrowing". The refusal is operator-visible, so the page says what a peer that narrows mid-tunnel now meets, and what to do about it |
| 7 | Wire format changed? | No | the payloads are unchanged; their CONTENT is corrected |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | DONE, and neither hand-written file needed an edit. `[RFC7296-2.9.2-1]` and `[RFC7296-2.9.2-2]` keep their text and their MUST NOT level, and no `{gap}` was added or removed, so the `docs/features/rfc-status.md` row still holds. The two new tags reached `rfc/requirements/rfc7296.md` and `ai/RFC-REQUIREMENTS.md` through `make ze-rfc-index-update`, and `make ze-rfc-check` is green |
| 10 | Test infrastructure changed? | No | existing runners |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | DONE: `docs/architecture/ike/ipsec-13-rekey-wire.md`, the trap entry "The wire answer against the installed policy" |
| 13 | Route metadata keys added/changed? | N-A | not routing |
| 14 | Prometheus counters added/changed? | No | the refusal reuses `ze_ipsec_error_notify_sent_total`; see the Integration Checklist row above |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` anchors `engine/rekey.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none show a rekey answer |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add the two unit tests and the `.ci`
   from the Wiring Test table, red against today's code
   - Tests: `TestRekeyAnswerMatchesTheInstalledSelectors`,
     `ipsec-child-rekey-xfrm-narrowing`
   - Files: `internal/component/ike/engine/child_rekey_orientation_test.go`,
     `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` <!-- doc-links: ignore (test AC-4 of this spec will create; the spec is not authorised to run) -->
   - Verify: both fail on the divergence, naming both selector sets
2. **Phase: One value** -- apply Thomas's answer from Key Design Decisions
   - Tests: the three unit tests above
   - Files: `internal/component/ike/engine/rekey.go`,
     `internal/component/ike/engine/ts_narrow.go`
   - Verify: red before, green after, and reverting the change reddens both
3. **Phase: Interop** -- the strongSwan scenario with a narrowing peer
   - Tests: `test/interop-ipsec/scenarios/NN-child-rekey-narrowing` <!-- doc-links: ignore (test AC-4 of this spec will create; the spec is not authorised to run) -->
   - Verify: the two SPDs agree after the rekey

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The answered orientation is derived from `SelectorsLocalIsTSi`, never from the rekey's exchange role |
| Data flow | One value produces both the answer and the install call |
| Rule: `ai/rules/rfc-compliance.md` | The chosen answer is quoted against Sections 2.9 and 2.9.2, and the tagged tests name the requirement ids |
| Rule: `ai/rules/interop-and-goal-validation.md` | Reverting the production change reddens the `.ci` and the interop scenario, not only the unit tests |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The answer equals the installed set on every branch | `make ze-unit-pkg-test PKG=./internal/component/ike/engine` |
| The XFRM policy equals the answer | `make ze-qemu-integration-test` |
| A strongSwan peer that narrows ends with the same SPD | `make ze-interop-ipsec-test` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The TS payloads are peer-controlled. A proposal outside the floor must not widen what Ze programs, and an absent TS payload must not select a stale answer |
| Resource exhaustion | The narrowing loop is bounded by the proposal length, which `wireToSelectors` already caps |

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

- A wire answer and a dataplane install are two statements about one object. When
  they are produced from two variables, they agree only by accident, and the
  accident here is the floor branch of `narrowSelectors`.

## Key Design Decisions

**THERE IS NO QUESTION FOR THOMAS. The RFC determines the answer.** An earlier
draft of this section put two answers side by side and called both conformant.
That was wrong on the text, and `ai/rules/rfc-compliance.md` bans the shape it
took: full compliance MUST NOT be offered beside a narrower option for Thomas to
pick between.

RFC 7296 Section 2.9.2 states the obligation in the document's own words:

> Thus, the new SA MUST NOT have narrower selectors than the original.
> [...] The responder MUST NOT narrow down the Traffic Selectors narrower
> than the scope currently in use.

That closes the space rather than opening a choice. Answering the intersection
IS narrowing below the scope in use, so it is not a conformant alternative.
Widening beyond the proposal is refused by `[RFC7296-2.9-1]`. When a peer's
proposal covers no pair of the scope in use, no legal narrowed set exists, and
the only answer left is the one Section 2.9 already names for that case.

Section 2.9.2 also says why the situation means what it does: a rekey needing a
narrower scope than the SA in use implies the policy changed, and "in that case,
the SA should have been already deleted after the policy change took effect".
So refusing is not Ze declining a workable option. It is Ze declining to paper
over a state that should not exist.

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ze refuses with TS_UNACCEPTABLE when a rekey proposal covers no pair of the scope in use | **B. Answer the intersection and install it.** REJECTED, not conformant: this is the responder narrowing below the scope in use, which Section 2.9.2 states as a MUST NOT. That the tunnel survives is not a reason, and a surviving tunnel carrying a scope the RFC forbids is the failure rather than the mitigation. **C. Answer the floor.** REJECTED: `[RFC7296-2.9-1]` forbids widening beyond what the peer proposed | Determined by RFC 7296 Section 2.9.2, quoted above. The machinery already exists: `errTSUnacceptable` (`internal/component/ike/engine/ts_narrow.go`) is what `narrowChildSelectors` already returns when `narrowSelectors` fails or transport mode leaves nothing |

## Known Limitations

- The VPP IPsec backend cannot be driven by IKE, so any fix is proven on XFRM
  only (`spec-fixit-vpp-ipsec-inoperable`, closed 2026-08-10).

## RFC Documentation (Scope: protocol)

Add `// RFC 7296 Section 2.9: "<quoted requirement>"` above the code that produces
the answer, and `// RFC 7296 Section 2.9.2: "<quoted requirement>"` above the code
that applies the floor. Tag the unit tests with `RFC requirement: RFC7296-2.9-1`,
`RFC7296-2.9.2-1` and `RFC7296-2.9.2-2`, in both polarities.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
