# Spec: fixit-child-rekey-answer-vs-installed-selectors

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-08-22 |

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

**The same defect sits on the INITIATOR role, and it is worse there.**
`applyChildRekeyResponse` (`internal/component/ike/engine/rekey.go`) walked the
response payloads with a type switch over `*wire.PayloadNonce` and `*wire.PayloadSA`
alone. TSi and TSr were never read, and `newRekeyedChild` then inherited
`old.Selectors`. So a Ze-initiated rekey installed the pre-rekey scope whatever the
peer answered, and it did so with no branch that could ever agree by accident: the
responder path at least answers the floor when the floor branch is taken, while this
path read nothing at all. It is measured, not theorised. Against strongSwan 5.9.14,
charon logged `inbound CHILD_SA ze-child{2} established with SPIs ... and TS
10.1.0.0/24 === 10.2.0.0/25` and programmed that pair, while Ze's kernel kept
`10.2.0.0/24 <-> 10.1.0.0/24`.

The correct behavior already shipped one path over. The IKE_AUTH initiator calls
`recordInitiatorSelectors` (`internal/component/ike/engine/ts_narrow.go`) through
`adoptAuthResponseNegotiation` (`internal/component/ike/engine/transport_mode.go`)
and installs what came back, having first checked that the answer widens nothing. The
asymmetry between two initiator paths in one package was the bug.

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
- The INITIATOR installs the answer rather than the retired pair's selectors.
  `applyChildRekeyResponse` (`internal/component/ike/engine/rekey.go`) reads TSi
  and TSr, adopts them through `recordInitiatorSelectors`, and refuses an answer
  that widens past the proposal or narrows below the scope in use. A response
  carrying no TS payload is refused rather than answered from the retired pair.

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
| A-4 | A real peer can answer a Ze-initiated rekey with a set that differs from the scope in use, so the initiator half of this defect is reachable rather than theoretical | `applyChildRekeyResponse` (`internal/component/ike/engine/rekey.go`) read no TS payload at all, so it installed the retired pair's scope for EVERY answer | the initiator half is unreachable and needs no fix | an interop scenario where strongSwan answers a Ze-initiated rekey | confirmed 2026-08-22 against strongSwan 5.9.14. `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows` reproduces it with the producer reverted: charon logged `inbound CHILD_SA ze-child{3} established with SPIs ... and TS 10.1.0.0/24 === 10.2.0.0/25` while ze kept `10.2.0.0/24 <-> 10.1.0.0/24` |
| A-5 | An answer WIDER than the scope in use but inside the proposal is legal and must be installed, so the initiator cannot refuse every answer that differs from the floor | RFC 7296 Section 2.9 lets the responder return any subset of the proposal, and Section 2.9.2 sets a floor and no ceiling below the proposal | the fix would refuse instead of adopt, breaking a peer whose own policy widened between IKE_AUTH and the rekey | a unit test over both halves of the pair | confirmed 2026-08-22: `TestChildRekeyInitiatorInstallsTheAnsweredSelectors` installs the answered `10.1.0.0/24 <-> 10.2.0.0/24` over a floor of `10.1.0.0/25` and over a floor of `10.2.0.0/25`. NOT reachable against charon, which re-derives the same narrowing from the stored `child_cfg` at every rekey |
| A-3 | Refusing with TS_UNACCEPTABLE is conformant when the proposal covers no floor pair | `[RFC7296-2.9.2-2]` forbids narrowing below the scope in use, and `[RFC7296-2.9-1]` names TS_UNACCEPTABLE for an empty result | option A below is not on the table | the RFC text of Section 2.9.2 read whole | confirmed 2026-08-19: `rfc/full/rfc7296.txt:2536-2552` read whole. The section states the MUST NOT and then states that such a request means the SA "should have been already deleted after the policy change took effect" |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Option B changes the installed policy selector at a rekey, which is what `spec-fixit-child-sa-rekey-policy` proved must not happen by accident | `samePolicySelector` answers false and the retiring pair strips the live policy | the replacement claims the new selector under the same `Owner` before the retired pair is removed, and the XFRM `.ci` measures it |
| R-2 | Option A refuses a rekey the peer will retry, so the tunnel hard-expires | the peer logs TS_UNACCEPTABLE and retries until expiry | the refusal is the conformant answer; a log line names both selector sets so an operator can act |
| R-4 | The INITIATOR refusal has the same shape one role over: the soft lifetime is a level trigger, so ze retries the same rekey every second until the hard expiry tears the Child SA down | Ze's log repeats "the rekey answer narrows the scope in use" once a second, and the peer accumulates one answered Child SA per retry | accepted, and identical to R-2's shape. Both are only reachable through the `narrowedRekeyPairs` test override, because a production proposal always covers the scope in use. A hold-off after a permanent refusal would be a separate change on a path this spec does not own |
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
| The same rekey on the real XFRM backend | → | `installChildTolerant`, `childPolicyParams` | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` |
| A peer's CREATE_CHILD_SA rekey RESPONSE, answering a rekey Ze started | → | `applyChildRekeyResponse` | `TestChildRekeyInitiatorInstallsTheAnsweredSelectors` |
| The same response from a real strongSwan | → | `recordInitiatorSelectors`, `newRekeyedChild` | `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer-initiated Child SA rekey whose TSi/TSr cover at least one floor pair | The answer and the installed policy selector name the same set, as they do today |
| AC-2 | A peer-initiated Child SA rekey whose TSi/TSr cover no floor pair | Ze either refuses the exchange or answers and installs one set. No path leaves the two disagreeing |
| AC-3 | A peer-initiated Child SA rekey carrying no TS payloads | Ze refuses it with INVALID_SYNTAX, rather than answering with the previous exchange's selectors |
| AC-4 | Every AC above, on the real XFRM dataplane | The kernel policy selector equals the set the response announced |
| AC-5 | A Ze-initiated Child SA rekey whose response answers a set inside the proposal that covers the scope in use | Ze installs the ANSWERED set, in both the selector list and the kernel policy prefixes, whichever half of the pair moved |
| AC-6 | A Ze-initiated Child SA rekey whose response widens past the proposal, narrows below the scope in use, or carries no TS payload | Ze refuses it, installs nothing, keeps the SA in use, and sends no error notification (RFC 7296 Section 2.21.3) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Narrows the traffic selectors on the peer daemon, then waits for its rekey timer | peer CREATE_CHILD_SA -> `respondChildRekey` -> `narrowSelectors` -> answer plus install | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` |
| 2 | Runs a strongSwan peer whose child config narrows between establishment and rekey | strongSwan -> Ze responder -> XFRM policy | `test/interop-ipsec/scenarios/13-child-rekey-narrowing` |
| 3 | Runs a strongSwan peer that answers Ze's own rekey with a narrower scope | Ze `initiateChildRekey` -> strongSwan answer -> `applyChildRekeyResponse` -> refusal, SA in use kept | `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRekeyAnswerMatchesTheInstalledSelectors` | `internal/component/ike/engine/child_rekey_orientation_test.go` | the answered pairs equal the installed pairs, in both orientations | green 2026-08-19. It decrypts the response and reads the TS payloads, so it judges the wire answer rather than the value that feeds it |
| `TestRekeyProposalBelowTheFloorIsRefused` | `internal/component/ike/engine/child_rekey_orientation_test.go` | a proposal covering no floor pair draws TS_UNACCEPTABLE, not a silent divergence | green 2026-08-19, and red with the refusal disabled. It lives beside the test above rather than in `rekey_test.go`: the two share one fixture, and one concern belongs in one file |
| `TestRekeyWithoutTrafficSelectorsIsRefused` | `internal/component/ike/engine/rekey_test.go` | a rekey request with no TS payloads never reuses the previous exchange's answer | green 2026-08-19, and red against the pre-fix producer, which announced `10.1.0.0/24 <-> 10.2.0.0/24`: the previous exchange's set, in the orientation of THAT exchange. Four fixtures sent a TS-less rekey and asserted success (`TestRespondChildRekey`, `TestResSelectedDHGroupNoneOmitsKEFromTheResponse`, and the two callers of `rkyChildRekeyRequest`). Each now proposes selectors, which is what RFC 7296 Section 1.3.3 puts in the request |
| `TestChildRekeyInitiatorInstallsTheAnsweredSelectors` | `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | AC-5: the replacement carries the ANSWERED set, in `Selectors` and in `TSLocal`/`TSRemote`, whichever half the responder had narrowed at IKE_AUTH. Its second case is the AC-6 widening arm | green 2026-08-22. Red against the pre-fix producer on all four assertions: it installed `10.1.0.0/25 <-> 10.2.0.0/24` where the peer answered `10.1.0.0/24 <-> 10.2.0.0/24`, and it adopted an answer of `10.1.0.0/16` ze never proposed. Tagged `RFC7296-2.9-1` in both polarities |
| `TestChildRekeyAnswerBelowTheScopeInUseIsRefused` | `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | AC-6: an answer covering no pair of the scope in use draws `errTSUnacceptable`, installs nothing, and names both scopes. Driven over BOTH halves of the pair | green 2026-08-22, red against the pre-fix producer on both halves. Tagged `RFC7296-2.9.2-1` and `RFC7296-2.9.2-2` in both polarities |
| `TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused` | `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | AC-6: a response omitting TSi or TSr is refused rather than answered from the retired pair | green 2026-08-22, red against the pre-fix producer. Seven fixtures built a TS-less rekey RESPONSE and asserted success; each now carries the SA's own scope through the shared `childRekeyAnswerTS` helper, which is what RFC 7296 Section 1.3.3 puts in the response |
| `TestContendingFunctionalTestsDeclareExclusiveGroup` | `internal/test/runner/exclusive_group_test.go` | every real-XFRM ipsec `.ci` declares `option=exclusive:group=ipsec-xfrm` | green 2026-08-22. The cluster row is new; the three members are the two rekey tests and `ipsec-teardown-leaves-nothing` |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Floor pairs covered by the proposal | 0..len(floor) | len(floor) | N/A | N/A |
| Selector port | 0-65535 | 65535 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-child-rekey-xfrm-narrowing` | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` | the peer narrows its selectors and rekeys; the kernel policy matches the answer | green 2026-08-22, `PASS 1/1 100.0% 21.5s` in QEMU, and RED with the `coversFloor` refusal disabled: it then reports `REKEY-ACCEPTED` with the replacement SPIs rather than timing out, so it names the divergence itself. AC-4 is proven on the real dataplane |
| `ipsec-child-rekey-xfrm` | `test/ipsec/ipsec-child-rekey-xfrm.ci` | AC-1 on the real dataplane, unchanged by the initiator half | green 2026-08-22 in QEMU, `PASS 2` beside the narrowing test once both declare `option=exclusive:group=ipsec-xfrm`. Without the group they read each other's node-wide SPIs and both go red |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `14-initiator-rekey-answer-narrows` | `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows` | strongSwan 5.9.14 | AC-6 end to end: charon answers a Ze-initiated rekey with a scope narrower than the SA in use, and Ze refuses it, keeps its ESP SPIs, and keeps the scope in use in the kernel and in `show vpn ipsec sa` | green 2026-08-22, and RED with the initiator adoption reverted: `Ze log missing: 'narrows the scope in use ...'` while charon logged `inbound CHILD_SA ze-child{3} established with SPIs ... and TS 10.1.0.0/24 === 10.2.0.0/25`. That red IS the measured divergence |
| `13-child-rekey-narrowing` | `test/interop-ipsec/scenarios/13-child-rekey-narrowing` | strongSwan | User Story 2: a peer whose child policy narrows mid-tunnel ends with the same SPD as Ze | green 2026-08-22. It does NOT exercise the Section 2.9.2 refusal; see the paragraph below |

**Why the RESPONDER-side strongSwan stimulus is unreachable, so nobody re-derives
it.** AC-2 needs a peer that SENDS a rekey whose selectors no longer cover the
scope in use. charon cannot send one. It builds a CHILD_SA rekey from the
CHILD_SA's own stored `child_cfg` on both roles (`child_rekey.c`, `build_i` and
`process_r`), so `swanctl --load-conns` never changes what an ESTABLISHED
CHILD_SA proposes or answers, and its proposal therefore always covers the scope
that CHILD_SA installed. With `start_action = start` charon instead deletes the
SA and dials a new one, which is what RFC 7296 Section 2.9.2 says a policy change
should do: "the SA should have been already deleted after the policy change took
effect." Scenario 13 covers that route. The refusal itself is proven between two
Ze daemons in `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` and in the engine
unit tests. The INITIATOR-side stimulus IS reachable, because Ze proposes and
charon answers, and scenario 14 is it.

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
- `internal/component/ike/engine/rekey.go` - the INITIATOR half. DONE:
  `applyChildRekeyResponse` reads TSi and TSr from the response, refuses a
  response that omits either, builds the rekey floor from `old.Selectors` in this
  exchange's orientation, and adopts the answer through
  `recordInitiatorSelectors` before it derives any key. `newRekeyedChild` takes
  the negotiated set as a parameter and derives `Selectors`,
  `SelectorsLocalIsTSi`, `TSLocal` and `TSRemote` from it, so both roles install
  what THIS exchange agreed. Both production callers pass `sa.NegotiatedPairs`.
- `internal/component/ike/engine/ts_narrow.go` - `recordInitiatorSelectors` takes
  the same `floor` parameter `narrowChildSelectors` takes, and refuses with
  `errTSUnacceptable` when `coversFloor` fails. DONE. The IKE_AUTH caller
  (`adoptAuthResponseNegotiation`, `transport_mode.go`) passes nil: no scope is in
  use before the first Child SA.
- `internal/component/ike/engine/transport_mode.go` - the nil floor above. DONE.
- `test/ipsec/ipsec-child-rekey-xfrm.ci`,
  `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci`,
  `test/ipsec/ipsec-teardown-leaves-nothing.ci` - DONE:
  `option=exclusive:group=ipsec-xfrm`. The three read `ip xfrm state` and
  `ip xfrm policy`, which are NODE-WIDE, so run together they read each other's
  SPIs. MEASURED 2026-08-22: the two rekey tests each reported the other's
  replacement SPIs as its own and both went red; serialized, both pass.
- `internal/test/runner/exclusive_group_test.go` - DONE: the cluster invariant
  gains a `dir` field and the `ipsec-xfrm` row, so a fourth real-XFRM ipsec `.ci`
  fails the ratchet unless it declares the group.
- `docs/architecture/ike/ipsec-13-rekey-wire.md` - the rekey answer is a
  statement about the SA that was installed. DONE: a new trap entry, "The wire
  answer against the installed policy", corrected to say that `newRekeyedChild`
  installs what THIS exchange negotiated, plus a second entry, "The answer the
  INITIATOR reads".
- `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - the design doc
  `ts_narrow.go` declares in its header. DONE: the "initiator checks the answer"
  decision gains the half that says it INSTALLS the answer and applies the rekey
  floor, and names RFC 7296 Section 2.21.3 for sending no error notification.

## Files to Create
- `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` - the functional test above.
  DONE, green in QEMU.
- `internal/component/ike/engine/child_rekey_initiator_answer_test.go` - the three
  initiator unit tests. DONE.
- `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows` - the interop
  scenario above, with `ze.conf`, `swanctl.conf`, `ze-env` and `check.py`. DONE,
  green against strongSwan 5.9.14 and red with the producer reverted.

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
| 10 | Test infrastructure changed? | Yes | DONE: `docs/functional-tests.md` gains the `ipsec-xfrm` cluster beside the ddos and BFD ones. The three real-XFRM ipsec `.ci` now declare `option=exclusive:group=ipsec-xfrm`, and `TestContendingFunctionalTestsDeclareExclusiveGroup` (`internal/test/runner/exclusive_group_test.go`) ratchets it, so a fourth one cannot land without the group. The runner itself is unchanged: the option already existed |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | DONE: `docs/architecture/ike/ipsec-13-rekey-wire.md`, the trap entries "The wire answer against the installed policy" and "The answer the INITIATOR reads", and `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`, the design doc `ts_narrow.go` declares in its header |
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
- AC-3 has no dataplane producer, and that is why it carries a unit proof alone.
  `proposeChildTSPayloads` (`internal/component/ike/engine/rekey.go`) always emits
  TSi and TSr, falling back to the wildcard when the pairs cannot be encoded, so
  no Ze peer can send a TS-less rekey and no `.ci` can stage one.
  `TestRekeyWithoutTrafficSelectorsIsRefused` is the proof. The same holds for the
  TS-less RESPONSE half of AC-6, which
  `TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused` proves.
- The test-only override `narrowedRekeyPairs`
  (`internal/component/ike/engine/testport.go`) exists only because
  `peerConfigChanged` (`internal/component/ike/engine/reconcile.go`) compares no
  traffic selector, so no Ze peer can narrow its own selectors on a live tunnel.
  That defect has its own spec, `plan/spec-fixit-ipsec-peer-reload-ignored.md`,
  whose R-3 commits to deleting the override once a real producer exists. It is
  not fixed here. Both the `.ci` and interop scenario 14 depend on the override
  for their stimulus.

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
- [ ] Lesson routed to a `plan/journal/<class>.md` row (`plan/learned/NNN-*.md` was retired in `2cff2050a`)
- [ ] **Commit A:** code + tests + docs + spec + journal rows
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

The answer and the install became ONE value on both exchange roles.

- **Responder** (landed in `83c1a5175` and `c5aac686c`). `coversFloor`
  (`internal/component/ike/engine/ts_narrow.go`) refuses a rekey proposal that covers no
  pair of the scope in use, with `errTSUnacceptable`, which `notifyForRefusal`
  (`internal/component/ike/engine/notify_error.go`) maps to TS_UNACCEPTABLE.
  `respondChildRekey` (`internal/component/ike/engine/rekey.go`) refuses a request missing
  TSi or TSr with `errMalformedRequest`, which the same function maps to INVALID_SYNTAX.
- **Initiator** (this closure). `applyChildRekeyResponse`
  (`internal/component/ike/engine/rekey.go`) captures `*wire.PayloadTS` in its payload
  walk, refuses a response missing either payload, builds the rekey floor from
  `old.Selectors` swapped into this exchange's orientation, and adopts the answer through
  `recordInitiatorSelectors` BEFORE it derives any key.
- **The single builder.** `newRekeyedChild` (`internal/component/ike/engine/rekey.go`)
  takes a `negotiated []tsPair` and derives `Selectors`, `SelectorsLocalIsTSi`, `TSLocal`
  and `TSRemote` from it. Both production callers pass `sa.NegotiatedPairs`, the value
  `pairsToWire` builds the wire answer from. An empty set inherits from the retired pair,
  so the next rekey still has a floor.
- **The floor on the initiator.** `recordInitiatorSelectors`
  (`internal/component/ike/engine/ts_narrow.go`) gained the same `floor` parameter
  `narrowChildSelectors` takes. `adoptAuthResponseNegotiation`
  (`internal/component/ike/engine/transport_mode.go`) passes nil: no scope is in use before
  the first Child SA.
- **Test serialization.** The three real-XFRM ipsec `.ci` declare
  `option=exclusive:group=ipsec-xfrm`, and `TestContendingFunctionalTestsDeclareExclusiveGroup`
  (`internal/test/runner/exclusive_group_test.go`) ratchets it with a fail-closed
  `minChecks` of 3.

### Bugs Found/Fixed

| Bug | Test that now covers it |
|-----|-------------------------|
| The rekey INITIATOR read no TS payload at all and installed the retired pair's scope whatever the peer answered. Measured against strongSwan 5.9.14 | `TestChildRekeyInitiatorInstallsTheAnsweredSelectors`, and `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows` |
| The three real-XFRM ipsec `.ci` read node-wide `ip xfrm state` and each reported a sibling's SPIs as its own | `TestContendingFunctionalTestsDeclareExclusiveGroup` |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` claimed the selector orientation "is written once, at creation, from the IKE_AUTH role, and `newRekeyedChild` inherits it unchanged". False after this change: a rekey that negotiates a set stores that set's own orientation. Found by the closure review, fixed here | `make ze-doc-verify` (the anchor sits on the corrected paragraph) |

Two finds were journaled rather than fixed, because neither blocks a goal of this spec:
`plan/journal/zero-value-as-valid-answer.md` (an IKE_AUTH response with no TS payload
leaves `ChildSA.Selectors` nil, so `coversFloor` is skipped for that SA's life and
`selectorPort` returns `AnyPortMatch`) and `plan/journal/guard-enumerates-instead-of-subtracting.md`
(`peerConfigChanged` compares no traffic selector).

### Documentation Updates
- `docs/architecture/ike/ipsec-13-rekey-wire.md`: "The wire answer against the installed
  policy" corrected, plus a new entry "The answer the INITIATOR reads".
  Anchors: `rekey.go -- applyChildRekeyResponse`,
  `ts_narrow.go -- recordInitiatorSelectors, checkAnswerWithin`.
- `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`: the design doc `ts_narrow.go`
  declares. The initiator decision gained the install-and-floor half and RFC 7296
  Section 2.21.3. Anchors: `rekey.go -- applyChildRekeyResponse, respondChildRekey,
  newRekeyedChild`, `ts_narrow.go -- coversFloor, floorWithinProposal`.
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`: the stale orientation claim above.
- `docs/guide/ipsec.md`, "Traffic selectors and narrowing": the operator-visible half of
  the INITIATOR refusal. Anchor: `rekey.go -- applyChildRekeyResponse, newRekeyedChild`.
- `docs/functional-tests.md`: the `ipsec-xfrm` exclusive cluster.
- `rfc/requirements/rfc7296.md`: regenerated by `make ze-rfc-index-update`.
- `make ze-doc-verify`: PASSED, 3025 anchors across 23 digests resolve.

### Deviations from Plan
- The spec was written for the RESPONDER alone. AC-5 and AC-6 were added when the
  INITIATOR half was measured against strongSwan, and the fix widened to both roles. That
  is a scope INCREASE toward full conformance, which `ai/rules/rfc-compliance.md` requires
  rather than gates.
- `narrowSelectors` reports no branch. The caller asks about the RESULT instead
  (`coversFloor`), which needs no new return value and also covers the transport-mode
  filter that runs after it.
- AC-3 and the TS-less half of AC-6 have no dataplane producer, so both keep a unit proof.
  `proposeChildTSPayloads` (`internal/component/ike/engine/rekey.go`) always emits TSi and
  TSr, falling back to the wildcard, so no Ze peer can send a TS-less rekey or answer one.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed installing the narrowed set keeps the make-before-break window intact | It is option B's assumption, and Section 2.9.2 rejects option B. Nothing installs a set narrower than the scope in use, so `samePolicySelector` is never asked about one | Reading Section 2.9.2 whole rather than its summary | A-2 recorded `moot`, not `confirmed`. Option B is a rejected alternative in Key Design Decisions |
| approach | The spec's first draft held a question for Thomas, on the reading that two conformant answers existed | Section 2.9.2 closes the space. `ai/rules/rfc-compliance.md` bans putting full compliance beside a narrower option | Re-reading the RFC text after `ai/rules/rfc-compliance.md` | The question was withdrawn and the section rewritten to say so |
| approach | The initiator half was journaled as "not fixed, the fix is a design decision" | The RFC answers it: Section 2.9.2's first sentence binds the NEW SA, so it binds the end that installs one | The interop probe measured the divergence against charon | The spec gained AC-5 and AC-6, the half was implemented, and the journal row was corrected in this closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The responder's answer and its install come from one value | Done | `respondChildRekey`, `newRekeyedChild` (`internal/component/ike/engine/rekey.go`) | Both read `sa.NegotiatedPairs` |
| A rekey proposal covering no floor pair is refused | Done | `narrowChildSelectors`, `coversFloor` (`internal/component/ike/engine/ts_narrow.go`) | `errTSUnacceptable` maps to TS_UNACCEPTABLE |
| A rekey request with no TS payload is refused | Done | `respondChildRekey` (`internal/component/ike/engine/rekey.go`) | `errMalformedRequest` maps to INVALID_SYNTAX |
| The INITIATOR installs the answer, not the retired pair's selectors | Done | `applyChildRekeyResponse`, `newRekeyedChild` (`internal/component/ike/engine/rekey.go`) | Adopted through `recordInitiatorSelectors` before key derivation |
| The INITIATOR refuses an answer below the scope in use, and sends no error notify | Done | `recordInitiatorSelectors` (`internal/component/ike/engine/ts_narrow.go`), `handleCreateChildSAOwned` (`internal/component/ike/engine/inbound.go`) | The default error branch drops `pendingRekey` and logs; no notify is built |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRekeyAnswerMatchesTheInstalledSelectors`, `test/ipsec/ipsec-child-rekey-xfrm.ci` | PASS in QEMU 2026-08-22 |
| AC-2 | Done | `TestRekeyProposalBelowTheFloorIsRefused`, `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` | Both re-measured red against a disabled `coversFloor` |
| AC-3 | Done | `TestRekeyWithoutTrafficSelectorsIsRefused` | No dataplane producer exists; see Known Limitations |
| AC-4 | Done | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci`, `test/ipsec/ipsec-child-rekey-xfrm.ci` | `PASS 2/2 100.0% 27.0s` in QEMU |
| AC-5 | Done | `TestChildRekeyInitiatorInstallsTheAnsweredSelectors` | Both halves of the pair, `Selectors` AND `TSLocal`/`TSRemote` |
| AC-6 | Done | `TestChildRekeyAnswerBelowTheScopeInUseIsRefused`, `TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`, `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows` | Interop PASS against strongSwan 5.9.14 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRekeyAnswerMatchesTheInstalledSelectors` | Done | `internal/component/ike/engine/child_rekey_orientation_test.go` | Reads the TS payloads of the decrypted response |
| `TestRekeyProposalBelowTheFloorIsRefused` | Done | `internal/component/ike/engine/child_rekey_orientation_test.go` | Red re-measured 2026-08-22 |
| `TestRekeyWithoutTrafficSelectorsIsRefused` | Done | `internal/component/ike/engine/rekey_test.go` | Red re-measured 2026-08-22 |
| `TestChildRekeyInitiatorInstallsTheAnsweredSelectors` | Done | `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | Red re-measured 2026-08-22 |
| `TestChildRekeyAnswerBelowTheScopeInUseIsRefused` | Done | `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | Red re-measured 2026-08-22 |
| `TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused` | Done | `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | Red re-measured 2026-08-22 |
| `TestContendingFunctionalTestsDeclareExclusiveGroup` | Done | `internal/test/runner/exclusive_group_test.go` | `ok 13.126s` with the cache defeated |
| `ipsec-child-rekey-xfrm-narrowing` | Done | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` | PASS 21.5s in QEMU; RED with `coversFloor` disabled |
| `ipsec-child-rekey-xfrm` | Done | `test/ipsec/ipsec-child-rekey-xfrm.ci` | PASS 5.6s in QEMU |
| `14-initiator-rekey-answer-narrows` | Done | `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows` | PASS; RED with the floor disabled |
| `13-child-rekey-narrowing` | Done | `test/interop-ipsec/scenarios/13-child-rekey-narrowing` | User Story 2 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/rekey.go` | Done | Both roles, plus `newRekeyedChild`'s new parameter |
| `internal/component/ike/engine/ts_narrow.go` | Done | `coversFloor`, and the floor parameter on `recordInitiatorSelectors` |
| `internal/component/ike/engine/transport_mode.go` | Done | The nil floor at IKE_AUTH |
| `internal/component/ike/engine/testport.go` | Done | `narrowedRekeyPairs`, the test-only stimulus |
| `test/ipsec/ipsec-child-rekey-xfrm.ci`, `-narrowing.ci`, `ipsec-teardown-leaves-nothing.ci` | Done | `option=exclusive:group=ipsec-xfrm` |
| `internal/test/runner/exclusive_group_test.go` | Done | The `ipsec` cluster row and the `dir` field |
| `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | Done | Created |
| `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows/` | Done | Created: `ze.conf`, `swanctl.conf`, `ze-env`, `check.py` |
| `docs/architecture/ike/ipsec-13-rekey-wire.md`, `rfcgate-1b-rfc7296-pilot.md` | Done | Plus `ipsec-8-ikev2-child-xfrm.md` and `docs/guide/ipsec.md`, found by the closure review |

### Audit Summary
- **Total items:** 31
- **Done:** 31
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A Child SA rekey Ze RESPONDS to never announces one selector set and installs another | functional, on the real XFRM dataplane | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci`, `PASS 21.5s` in QEMU 2026-08-22. RED with `coversFloor` disabled: `REKEY-ACCEPTED: peer-a installed states for a proposal that covers no pair of the scope in use` |
| A Child SA rekey Ze INITIATES installs the scope the peer answered | interop, strongSwan 5.9.14 | `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows`, PASS 2026-08-22. RED with the floor disabled: `Ze holds ESP policy prefixes ['10.1.0.0/24', '10.2.0.0/25'], expected exactly ['10.1.0.0/24', '10.2.0.0/24']` |
| The peer and Ze end a mid-tunnel narrowing with the same SPD | interop, strongSwan | `test/interop-ipsec/scenarios/13-child-rekey-narrowing`, PASS |
| RFC 7296 Sections 2.9 and 2.9.2 are proven, in both polarities | tagged unit tests plus the gate | `make ze-rfc-check` OK: 2966 gated MUST-level requirements, 3589 tags resolved. `RFC7296-2.9-1`, `RFC7296-2.9.2-1` and `RFC7296-2.9.2-2` each GAINED tags in both polarities (`rfc/requirements/rfc7296.md`); none lost one |
| Two contending real-XFRM `.ci` no longer read each other's kernel state | functional plus a ratchet | `PASS 2/2 100.0% 27.0s` in QEMU, run serialized. `TestContendingFunctionalTestsDeclareExclusiveGroup` fails closed below three members |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The spec metadata records `Deferral shard: -`, and no shard exists for this stem, so there is none to remove and no foreign shard was emptied |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-child-rekey-answer-vs-installed-selectors-<session>.md`, written by `scripts/dev/review_gate.py record` over 33 files |
| `review_gate.py check` | `review_gate: OK (17 code files, clean, hashes match ...)` over the commit's Go population |
| Rounds | 2. Round 1 found three ISSUEs and one NOTE; round 2 over the fixes reported 0 BLOCKER, 0 ISSUE |
| Reviewer lenses used | RFC conformance (Sections 2.9, 2.9.2 and 2.21.3 read whole in `rfc/full/rfc7296.txt`), guard correctness (fail-closed, zero-value), discrimination (five producer mutations re-measured), doc-anchor freshness, security (peer-controlled TS payloads, resource bounds), test-relaxation audit |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The doc claimed "The orientation is written once, at creation, from the IKE_AUTH role, and `newRekeyedChild` inherits it unchanged." `newRekeyedChild` now sets `SelectorsLocalIsTSi` from the exchange role whenever the rekey negotiated a set, which is every production rekey. The doc carries a source anchor on the changed file, so `ai/rules/stale-comments.md` applies | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, "One field cannot carry both the KEYMAT role and the selector orientation" | The paragraph rewritten to state the real invariant: the orientation names the exchange that NEGOTIATED the set, and an inherited set keeps both |
| 2 | ISSUE | Documentation checklist row 6 was answered for the RESPONDER refusal alone. The INITIATOR refusal is equally operator-visible (Ze retries at each lifetime tick and the tunnel hard-expires), and no user-facing page said so | `docs/guide/ipsec.md`, "Traffic selectors and narrowing" | One paragraph plus a source anchor on `applyChildRekeyResponse, newRekeyedChild` |
| 3 | ISSUE | `plan/journal/guard-added-to-one-half-of-a-pair.md` carried "not fixed" for the initiator half, and named a design question the RFC had since answered. A row that outlives its own truth misleads the next reader | `plan/journal/guard-added-to-one-half-of-a-pair.md`, the row whose Spec cell is this stem | The Fix cell rewritten with what was done, the RFC sentences that decided it, and the committed reproduction |
| 4 | NOTE | An IKE_AUTH response carrying SAr2 and no TS payload leaves `ChildSA.Selectors` nil, so `coversFloor` is skipped for that SA's life and `selectorPort` returns `AnyPortMatch`. Pre-existing, reachable only from a non-conforming peer, and outside every AC | `adoptAuthResponseNegotiation` (`internal/component/ike/engine/transport_mode.go`), `createFirstChildSA` and `selectorPort` (`internal/component/ike/engine/child.go`) | Journaled at `plan/journal/zero-value-as-valid-answer.md`. Not fixed: the fix belongs to IKE_AUTH and owes its own interop proof |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` | Yes | `-rw-r--r-- 14K Aug 22 12:49` |
| `test/ipsec/ipsec-child-rekey-xfrm.ci` | Yes | `-rw-r--r-- 9.5K Aug 22 12:49` |
| `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | Yes | `-rw-r--r-- 15K Aug 22 13:25` |
| `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows/check.py` | Yes | `-rw-r--r-- 8.5K Aug 22 13:04` |
| `test/interop-ipsec/scenarios/13-child-rekey-narrowing/check.py` | Yes | `-rw-r--r-- 8.1K Aug 22 12:11` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A rekey covering the floor answers and installs one set | `ipsec-child-rekey-xfrm` `PASS 5.6s` in QEMU 2026-08-22 |
| AC-2 | A rekey covering no floor pair is refused | `TestRekeyProposalBelowTheFloorIsRefused` red with `coversFloor` disabled: `respondChildRekey answered a proposal that covers no pair of the scope in use, announcing [10.2.0.128/25 <-> 10.1.0.0/24]` |
| AC-3 | A TS-less rekey draws INVALID_SYNTAX | `TestRekeyWithoutTrafficSelectorsIsRefused` red against a restored TS-less fallback: `respondChildRekey answered a request that proposed no traffic selectors, announcing [10.1.0.0/24 <-> 10.2.0.0/24]` |
| AC-4 | The kernel policy equals the announced set | `ipsec-child-rekey-xfrm-narrowing` `PASS 21.5s`, prints `POLICY-STABLE`. RED with `coversFloor` disabled: `REKEY-ACCEPTED` |
| AC-5 | The initiator installs the ANSWERED set | `TestChildRekeyInitiatorInstallsTheAnsweredSelectors` red with `newRekeyedChild` reverted to inheritance, on all four assertions: `installed with [10.1.0.0/25 <-> 10.2.0.0/24] and the peer answered [10.1.0.0/24 <-> 10.2.0.0/24]` |
| AC-6 | The initiator refuses a widened, narrowed or TS-less answer, installs nothing, keeps the SA | `TestChildRekeyAnswerBelowTheScopeInUseIsRefused` red with the floor disabled on both halves; `TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused` red with the TS-less fallback restored; interop 14 PASS, and red with the floor disabled |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A peer's rekey whose TS proposal covers no floor pair | `test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci` | Read: peer-b carries `ze_test_ike_rekey_ts_local=192.0.2.96/27` on its own `cmd=background` line, peer-a runs the real XFRM backend, `refused.py` reads `ip xfrm state` and `ip xfrm policy`, and `expect=stderr:contains=the rekey proposal narrows the scope in use` pins the refusal |
| The same rekey on the real XFRM backend | `test/ipsec/ipsec-child-rekey-xfrm.ci` | Read: the same crossed-roles fixture, plus `option=exclusive:group=ipsec-xfrm` |
| A rekey RESPONSE answering a rekey Ze started | `internal/component/ike/engine/child_rekey_initiator_answer_test.go` | Read: `zeRekeyFixture` drives `initiateChildRekey` then `applyChildRekeyResponse` with real `wire.PayloadTS` payloads, and asserts `Selectors` AND `TSLocal`/`TSRemote` |
| The same response from a real strongSwan | `test/interop-ipsec/scenarios/14-initiator-rekey-answer-narrows/check.py` | Read: `ze_esp_spis` and `ze_esp_policy_prefixes` both RAISE on an empty read, so no assertion can pass on a failed command |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestRekeyProposalBelowTheFloorIsRefused` reaches the case from wire payloads |
| A-2 | moot | Option B is rejected by Section 2.9.2; nothing installs a narrowed set |
| A-3 | confirmed | `rfc/full/rfc7296.txt:2534-2552` read whole in this closure |
| A-4 | confirmed | Interop 14 PASS, and RED with the floor disabled while charon logs `TS 10.1.0.0/24 === 10.2.0.0/25` |
| A-5 | confirmed | `TestChildRekeyInitiatorInstallsTheAnsweredSelectors` installs the answered `/24` over a `/25` floor, on both halves |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 6, user guide | `docs/guide/ipsec.md` holds "Traffic selectors and narrowing"; the initiator paragraph added here matches `applyChildRekeyResponse` and the level-trigger retry in `startChildRekey` (`internal/component/ike/engine/established.go`) | Yes |
| Row 9, RFC status | `make ze-rfc-check` OK. No `{gap}` added or removed, no level lowered, and the three requirement ids each GAINED tags in both polarities | Yes |
| Row 10, test infrastructure | `docs/functional-tests.md` gains the `ipsec-xfrm` cluster; the three `.ci` carry the option and the ratchet's `minChecks` is 3 | Yes |
| Row 12, internal architecture | `docs/architecture/ike/ipsec-13-rekey-wire.md` and `rfcgate-1b-rfc7296-pilot.md` edited; `ipsec-8-ikev2-child-xfrm.md` corrected by this review | Yes |
| Row 16, existing source anchors | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` anchors `rekey.go -- newRekeyedChild`, and its claim was STALE. Corrected | Yes |
| Rows 1-5, 7-8, 11, 13-15, 17 | No user-facing feature, config leaf, CLI command, RPC, plugin, wire format, SDK, comparison entry, route metadata key, counter, or registry entry changed. The diff covers `internal/component/ike/engine/`, `internal/test/runner/`, `test/`, `docs/` and `rfc/requirements/` only | Yes |
| `make ze-doc-verify` | PASSED, 3025 anchors across 23 digests resolve | Yes |

## Core Insight

A wire answer and a dataplane install are two statements about ONE object. Produced from
two variables they agree only by accident, and the accident here was the floor branch of
`narrowSelectors`. The fix is not a comparison between the two variables. It is deleting
one of them: `newRekeyedChild` takes the negotiated set as an argument, so the value the
peer was told and the value the kernel holds cannot differ.

The same reading resolves the role question. RFC 7296 Section 2.9.2's first sentence binds
the NEW SA, not the responder, so it binds every end that CREATES one. A guard written for
the answering role alone leaves the installing role free to build the SA that sentence
forbids.
