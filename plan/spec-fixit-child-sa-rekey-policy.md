# Spec: fixit-child-sa-rekey-policy

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `-` |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task — REWRITTEN 2026-08-17 to the shipped design

**The headline defect this spec was written for is FIXED. Three residuals
remain, and one of them is a live product defect. Everything below the "As
originally written" heading is kept for history and MUST NOT be implemented.**

Re-verified at the producers 2026-08-17:

- `xfrmBackend.InstallPolicy` (`internal/component/ike/dataplane/xfrm_linux.go`)
  upserts with `netlink.XfrmPolicyUpdate`, never `XfrmPolicyAdd`. No
  `XfrmPolicyAdd` call remains anywhere in `internal/`, `cmd/` or `pkg/`. The
  EEXIST this spec was written for cannot occur.
- `removeChildSAExcept` (`internal/component/ike/engine/child.go`) gates policy
  removal on `samePolicySelector`, so retiring a superseded pair no longer
  removes the live pair's policy.
- The shipped design also grew per-peer policy ownership (`dataplane.SPParams.Owner`,
  claimed BEFORE the netlink call), which is what now refuses a foreign peer that
  EEXIST used to refuse.

→ Decision: **the shipped design is KEPT.** This spec originally proposed moving
the policy to session ownership. That is not what shipped, it is not needed to
fix the defect, and replacing a shipped, interop-proven design with the
alternative would be a rewrite with no defect behind it (`ai/rules/no-layering.md`,
`ai/rules/simplicity.md`).

### R-1 (live product defect) — one field carries two meanings, and a rekey flips it

`ChildSA.LocalIsInitiator` is written per EXCHANGE: `newRekeyedChild`
(`internal/component/ike/engine/rekey.go`) receives `true` from
`applyChildRekeyResponse` (ze initiated this rekey) and `false` from
`respondChildRekey` (the peer did). The same call inherits `Selectors` from the
retired pair, and those stay in the ORIGINAL exchange's TSi/TSr orientation.

`selectorPort` (`internal/component/ike/engine/child.go`) reads
`side := pair.R; if local == child.LocalIsInitiator { side = pair.I }`, so it
uses the per-exchange field to orient a value that did not move. A peer-initiated
rekey of a ze-initiated Child SA therefore reads the peer's side as ze's.

With asymmetric ports the replacement installs a PORT-SWAPPED policy, and
`samePolicySelector` then answers false where it should answer true, so the
retiring pair also removes a selector the live pair still needs. Read, not
measured: no test today produces a peer-proposed port narrowing.

**Root cause: one field means "my KEYMAT role in this exchange" and is also read
as "the orientation the stored Selectors are in". Those diverge at the first
rekey the other side initiates.** The fix separates them: the orientation
travels with `Selectors` and is inherited unchanged, and `LocalIsInitiator`
keeps its KEYMAT meaning.

### R-2 (coverage hole) — the rekey `.ci` proves nothing about XFRM

`test/ipsec/ipsec-child-rekey.ci` pins `ze_test_ike_dataplane=noop`, so it
exercises no policy or state programming at all. There is no
`ipsec-child-rekey-xfrm.ci`. A test that cannot observe the subject it names is
the vacuous-test class (`ai/rules/testing.md`).

### R-3 (A-1 is still unvalidated, and it is load-bearing) — no QEMU test proves reqid resolution

The shipped design depends on the kernel resolving ONE installed policy to a
REPLACED state carrying the same `ReqID` (RFC 4301 Section 4.4.1.2). Neither
`xfrm_rekey_policy_integration_linux_test.go` nor
`xfrm_policy_owner_integration_linux_test.go` calls `XfrmStateAdd`, so no QEMU
test exercises that resolution. The assumption the whole make-before-break rekey
rests on has never been measured.

### Assumptions, revalidated 2026-08-17

| ID | Outcome | Evidence |
|----|---------|----------|
| A-1 | **VALIDATED 2026-08-18**, in QEMU against a real kernel | `TestXFRMPolicyResolvesToAReplacedState` (`internal/component/ike/dataplane/xfrm_reqid_resolution_integration_linux_test.go`): one policy installed once resolved to a state and then to its replacement at the same request id, read from `XfrmOutNoStates` and from each state's own packet counter, bracketed by two controls that DO move the counter |
| A-2 | **confirmed** | `installChildSA` has exactly two callers (`createFirstChildSA`, `installChildTolerant`); `installChildTolerant` has exactly two (`applyChildRekeyResponse`, `respondChildRekey`). Three install paths, no fourth |
| A-3 | **confirmed, by a different mechanism than assumed** | `narrowChildSelectors` writes only to `sa.NegotiatedPairs`; `newRekeyedChild` inherits `Selectors` and never reads the narrowing result, so a rekey cannot move the installed selector. R-1's original compare-and-replace mitigation is NOT needed |

### Acceptance criteria that replace AC-1..AC-6

The original AC-1..AC-6 describe the session-owned design and are VOID. The
criteria for this spec are now:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-R1 | A peer-initiated rekey of a ze-initiated Child SA whose selectors carry asymmetric ports | The replacement's policy selector is oriented identically to the retired pair's, and `samePolicySelector` answers true |
| AC-R2 | The same, with the roles reversed (ze-initiated rekey of a peer-initiated Child SA) | Same |
| AC-R3 | A Child SA rekey on the real XFRM dataplane | A functional test observes the state and policy programming, not the noop backend |
| AC-R4 | A policy installed, a state installed at `ReqID` N, then replaced by a second state at the same `ReqID` | The policy still resolves to the new state, measured in QEMU against a real kernel |

## Implementation record 2026-08-18

R-1, R-2 and R-3 are implemented. Two further defects on the same surface were found
while implementing them, both measured and both fixed, and each carries a row in
`plan/journal/field-carries-two-meanings.md`.

| Residual | Outcome | Evidence |
|----------|---------|----------|
| R-1 | fixed at the source. The orientation of the stored selectors is recorded beside them and inherited unchanged at rekey, so `LocalIsInitiator` keeps its KEYMAT meaning alone | `TestRekeyKeepsThePolicyOrientationOfTheRetiredPair` (both role directions), RED before and GREEN after |
| R-2 | `test/ipsec/ipsec-child-rekey-xfrm.ci`, on the REAL xfrm backend, with the two roles crossed so the daemon under test answers a rekey the peer starts | PASS in QEMU. It discriminates: with the orientation read reverted it reports POLICY-MOVED and names the port that changed sides |
| R-3 | the QEMU test A-1 always needed | see the A-1 row above |
| found: the responder's narrowing orientation | fixed | `TestPeerInitiatedRekeyIsNarrowedInTheExchangeOrientation`, RED before and GREEN after |
| found: the responder's Message ID counter | fixed | `TestResponderFirstRequestMatchesWhatTheInitiatorExpects`, RED before and GREEN after |

The original AC-1..AC-6 stay VOID. AC-R1 to AC-R4 are met by the rows above.

RFC 7296 Section 2.2's two-counter rule is EXTRACTED and PROVEN. It is stated in
indicative prose that carries no RFC 2119 keyword, so the `rfc2119` register derived
no site for it and it had no checklist row. Commit `86b6aa291` added the row
`[RFC7296-2.2-3] [MUST]` to `rfc/short/rfc7296.md`, recorded it under section 2.2
`unsourced-ids` in `rfc/extraction/rfc7296.json` with a resign reason, and tagged it in
both polarities: positive in
`internal/component/ike/engine/rfc7296_msgid_responder_request_test.go` and negative in
`internal/component/ike/engine/rfc7296_msgid_test.go`. Nothing about it is open, and
`ai/rules/rfc-compliance.md` asks for no question: the ask is owed only when Ze is about
to do less than the RFC states.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-child-sa-rekey-policy-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | OK, clean, 20 files hash-pinned |
| Rounds | 4. Round 4 earned its pass on a PRODUCT defect: `esp_spis()` in the scenario round 2 added read through a fail-open helper, so a broken `docker exec` would have skipped every dataplane assertion in silence, and `make ze-functional-docker-exec-check` was RED for the whole checkout |
| Reviewer lenses used | RFC conformance and extraction state, test discrimination, removed-behavior audit, orientation and data-flow correctness, documentation against the producer, interop coverage |

**Round 1 recorded `findings` on purpose and its artifact is gone.** It read commit
`86b6aa291` and reported 3 BLOCKER and 4 ISSUE. The fixes were made by the
implementation session, so that session could not record a clean verdict over its
own edits (`ai/rules/planning.md`), and `tmp/` was cleaned before an independent
pass ran. Rounds 2 to 4 are that pass, by a closure agent that wrote none of the
code. Round 2 re-read the whole diff from source rather than citing the lost
artifact, CONFIRMED each round-1 fix at its producer instead of trusting the record,
and settled the three residuals the 2026-08-02 second-pass audit left OPEN. Round 3
read only round 2's own fixes. Round 4 read the new interop scenario under the gate
that reads it.

**One thing on this surface stays OPEN and this closure claims nothing about it.**
I3b routed the answer-versus-install divergence to
`spec-fixit-child-rekey-answer-vs-installed-selectors`, and `coversFloor` settles the
RESPONDER path only. `applyChildRekeyResponse` (`rekey.go`), the INITIATOR path, still
reads no TSi or TSr from a rekey response, so a Ze-initiated rekey installs the
pre-rekey selectors while the peer installs the narrowed ones. That spec is live and
owns it.

**The 2026-08-02 second-pass residuals are all closed.** (1) `samePolicySelector`
flipping orientation IS R-1, fixed here; `LocalIsInitiator` has exactly one production
reader left, the RFC 7296 Section 2.17 KEYMAT-half choice in `installChildSA`.
(2) Removal and install no longer disagree about policy identity: every engine Child SA
policy removal goes through `RemovePolicyParams(childPolicyParams(child, dir))`, and no
production caller of the three-argument `RemovePolicy` remains in `internal/`, `pkg/` or
`cmd/`. (3) `resolvePendingAfterOwnerLoop` stranding a `pendingChild` DISSOLVES, which
the audit named as the cheaper answer: `pendingChild` is set only by
`finishResponderEstablish` under `ps.ownedSA.Load() != nil`, the same condition that made
`register.go` set `pendingSA` first, and every clearer of `pendingSA` clears
`pendingChild` in the same block (`reapStalePending`, `cleanupPendingSA`, and the
promotion path in `fsm.go`). The combination cannot arise.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| B1 | BLOCKER | The spec published an OPEN question about RFC 7296 Section 2.2 that `86b6aa291` had already answered. The row, both test tags and the extraction resign reason all landed in that commit | `plan/spec-fixit-child-sa-rekey-policy.md`, Implementation record | The paragraph now records what was done and states that no question is owed |
| I1 | ISSUE | `asymmetricPortChild` left `SelectorsLocalIsTSi` false in both table cases while its doc comment claimed the flag followed `localIsInitiator`, so the test could not catch a wrong write of that flag | `internal/component/ike/engine/child_rekey_orientation_test.go`, `asymmetricPortChild` | The helper writes both fields and the comment says so. Mutation-checked: deriving the flag from the rekey's exchange role reddens BOTH table cases |
| I2 | ISSUE | `SA.NegotiatedPairs` carried no record of WHICH exchange's orientation it holds, while `narrowChildSelectors` overwrites it per exchange and `createFirstChildSA` reads it | `internal/component/ike/engine/sa.go`, `SA.NegotiatedPairs` | The documented invariant. Both production callers of `createFirstChildSA` answer IKE_AUTH and run before `maintainSA` services any exchange, so no rekey can move the orientation under them. The comment names that and points a third caller at `ChildSA.SelectorsLocalIsTSi` |
| I3a | ISSUE | The comment above `pairsToWire(sa.NegotiatedPairs)` claimed the response carries the same subset the replacement was installed with. `newRekeyedChild` installs `old.Selectors` | `internal/component/ike/engine/rekey.go`, `respondChildRekey` | The comment now states which branch of `narrowSelectors` makes the two agree, and names the spec that records the divergence |
| I3b | ISSUE | The answered set and the installed set diverge on the intersection branch, which is reachable from the wire | `internal/component/ike/engine/rekey.go`, `respondChildRekey` and `newRekeyedChild` | NOT fixed here. RFC 7296 Section 2.9 governs it, so `ai/rules/rfc-compliance.md` routes it to `spec-fixit-child-rekey-answer-vs-installed-selectors` and a question for Thomas, rather than an inline fix inside a closing commit (`ai/rules/rule-precedence.md`) |
| I4 | ISSUE | Two documentation rows were marked "Yes" and the work had not happened | `plan/spec-fixit-child-sa-rekey-policy.md`, Documentation Update Checklist | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` documents the orientation field against the KEYMAT role, the `docs/features/rfc-status.md` RFC 7296 row records the three behaviors this spec proved, and both rows say what landed |
| I5 | ISSUE | Round 2. Interop non-conformance, always-in-scope (`ai/rules/planning.md`, "Bounding the loop"). The commit changed a value on the wire, and no strongSwan scenario witnessed it. Scenario 05 has Ze as the connection INITIATOR, whose request counter was always right; 07, 09, 11 and 25 have Ze ANSWERING an exchange charon started. Nothing had a responder-role Ze speak first, so reverting the counter fix leaves every existing scenario green. The spec's own Interop Tests row was blank | `internal/component/ike/engine/responder.go`, `finishResponderEstablish` | `test/interop-ipsec/scenarios/26-responder-raises-child-rekey`. charon dials, Ze answers IKE_AUTH, and Ze's 30s ESP lifetime makes ZE raise the CREATE_CHILD_SA. PASS 2026-08-22 with ESP counters advancing on both containers after the rekey. MUTATION-VERIFIED in a throwaway worktree at HEAD with `sa.NextMsgID = msgID + 1` restored: FAIL, "strongSwan never parsed a CREATE_CHILD_SA REKEY_SA request from a responder-role Ze" |
| I6 | ISSUE | Round 2. A documentation claim false about the product (`ai/rules/evidence.md`). The `## Proof` section cited scenario 05 as this page's interop evidence, and that scenario cannot fail for the behavior the page describes. Its second sentence, "The Docker VM on macOS has no XFRM or ESP", is contradicted by the I5 run on this host, where the dataplane assertions ran and passed | `docs/architecture/ike/ipsec-13-rekey-wire.md`, `## Proof` | The Proof section names both exchange directions and what each proves, a Decisions paragraph states the two-counter rule with source anchors, and the XFRM sentence says the assertions are GATED rather than absent. `ai/digests/ipsec-ike.md` gains the same both-roles statement |
| N3 | NOTE | Round 3, over round 2's own fixes. A comment asserted a property the mutation run disproved: `OUT_OF_WINDOW` was called "the whole discrimination of this scenario", and the mutated run showed charon logs no refusal at all | `test/interop-ipsec/scenarios/26-responder-raises-child-rekey/check.py` | The comment records the measurement and calls the pattern a diagnostic. The assertion order is unchanged. A NOTE does not re-open a round |
| I7 | ISSUE | Round 2. A doc comment asserting a PRODUCT property that is false, on the function the commit changed. `finishResponderEstablish`'s header read "advances the message-ID counters (RFC 7296 Section 2.2: post-IKE_AUTH each side's request counter resumes at 2)", which is the behavior `86b6aa291` REMOVED, while the body directly below says `sa.NextMsgID` is deliberately not written and stays 0. A reader settling the two-counter question meets the header first (`ai/rules/stale-comments.md`) | `internal/component/ike/engine/responder.go`, `finishResponderEstablish` | The header names the counter that moves, says it is one of the two Section 2.2 gives an endpoint, and points at the body for why this side's own id stays 0. Written to the same line count, so the file's pre-existing 1018 lines do not grow |
| I8 | ISSUE | Round 4. A fail-open read in the scenario round 2 added, which is the vacuity `ai/rules/interop-and-goal-validation.md` exists to stop. `esp_spis()` read through `lab.ze_xfrm_state`, whose `docker_exec_quiet` returns `""` for a failed read AND for a kernel holding no state. Step 3 reads an empty set as "this host has no XFRM" and skips every dataplane assertion, so a broken `docker exec` would have skipped them in silence. `make ze-functional-docker-exec-check` went RED at 122 sites against a floor of 121, and that gate is a stage of `ze-precommit-verify`, so it blocked every session in this shared checkout | `test/interop-ipsec/scenarios/26-responder-raises-child-rekey/check.py`, `esp_spis` | It reads through `docker_exec`, which raises and names the container, the command and the stderr. An empty set is a reading now. The floor was NOT raised: `docker-exec-check: OK (121 unchecked <= floor 121)`. The scenario was re-run afterwards, because the fix changed the code path that produced the earlier PASS: PASS 2026-08-22, ESP counters advancing on both containers |

## As originally written (HISTORY — do not implement)

**Every Child SA rekey fails on Linux, and the tunnel stops carrying traffic when
the retired SA hard-expires.**

Ze installs an XFRM policy per Child SA. A rekey is make-before-break, so it
installs the replacement pair while the retired one is still installed. The
replacement's policies are identical to the retired pair's in every field the
kernel reads, because `newRekeyedChild` inherits `TSLocal`, `TSRemote`, `IfID`,
`ReqID`, `Mode` and `Selectors`. The kernel refuses the second install.

The goal is that a Child SA rekey installs new STATES and leaves the policy
alone, and that retiring the old pair removes its states without removing the
policy the live pair depends on.

### The measurement

`TestXFRMSecondInstallOfOneSelectorIsRefused`
(`internal/component/ike/dataplane/xfrm_rekey_policy_integration_linux_test.go`)
runs in the QEMU Alpine VM against a real kernel:

    second install refused, as expected: xfrm: policy add: file exists
    --- PASS

Its second half also proves only ONE policy exists: a second `RemovePolicyParams`
for the same selector fails.

### The chain, measured or read

| Link | How it is known |
|------|-----------------|
| A second policy with one selector is refused `EEXIST`, and only one exists | measured, QEMU, real kernel |
| `netlink.XfrmPolicyAdd` sends `XFRM_MSG_NEWPOLICY`, the exclusive form | read, `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` |
| `newRekeyedChild` inherits every field the policy selector is built from | read, `engine/rekey.go` |
| `installChildSA` installs both policies unconditionally, with no guard | read, `engine/child.go` |
| `isXFRMUnsupported` matches `ENOPROTOOPT`, `EPROTONOSUPPORT`, `EAFNOSUPPORT`, `ENOSYS` only | read, `engine/child.go` |
| `installChildTolerant` returns any error it cannot classify as unsupported | read, `engine/child.go` |

So the replacement's inbound policy install returns `EEXIST`, and
`installChildSA`'s own rollback then calls `RemoveSA` on both freshly installed
states. The rekey exchange fails after the peer has already keyed its side.

### Why the model is wrong, not the check

The policy is treated as owned by a Child SA. The kernel keys a policy on the
selector pair and resolves it to a state through `ReqID` (RFC 4301 Section
4.4.1.2). A replacement pair with the same `ReqID` is picked up by the policy
that is already installed, so the policy never needs reinstalling. It belongs to
the SESSION, and it must go when the session goes, not when one pair of states
is retired.

That also makes today's teardown wrong in the other direction:
`removeChildSAOutgoing` calls `RemovePolicy` with the selector the LIVE pair
still needs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ipsec/` (whichever pages cover Child SA programming)
  → Constraint: the dataplane vocabulary is `dataplane.SPParams` and
    `dataplane.SAParams`; the engine never speaks netlink directly.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - Section 2.8 rekey, Section 1.3.2 CREATE_CHILD_SA
  → Constraint: make-before-break. The replacement must carry traffic before the
    retired pair is deleted, so the policy must never be absent between the two.
- [ ] `rfc/short/rfc4301.md` if present, else `rfc/full/rfc4301.txt` Section 4.4.1.2
  → Constraint: the policy template resolves to a state through the endpoints and
    the request id. That is why one policy serves successive states.

**Key insights:**
- One policy, many successive states. `ReqID` is the join.
- The engine already carries `ReqID` on `ChildSA` and inherits it at rekey, so the
  join is present and correct today. Only the install and remove calls are wrong.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/child.go` - `installChildSA` installs two
  states then two policies, unconditionally. `removeChildSA` removes both
  policies and both states. `installChildTolerant` swallows only "unsupported".
- [ ] `internal/component/ike/engine/rekey.go` - `newRekeyedChild` inherits the
  selector fields. `respondChildRekey` and `applyChildRekeyResponse` both call
  `installChildTolerant` while the retired pair is still installed.
- [ ] `internal/component/ike/engine/delete.go` - `closeDesignatedChildSAs` calls
  `removeChildSA` on the superseded pair when the peer confirms a rekey.
- [ ] `internal/component/ike/engine/established.go` - `cleanupChild` removes both
  the superseded and the live pair at session teardown.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `InstallPolicy` calls
  `XfrmPolicyAdd`; `RemovePolicy` and `RemovePolicyParams` call `XfrmPolicyDel`.

**Behavior to preserve:**
- The first Child SA of a session still installs both policies and both states.
- A session teardown still leaves no XFRM state or policy behind.
- Transport mode still installs a policy with no tunnel endpoints.
- `installChildTolerant` still degrades gracefully where XFRM is absent.

**Behavior to change:**
- A rekey installs states only.
- Retiring a superseded pair removes states only.
- The policy pair is installed once per session and removed once, at teardown.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A CREATE_CHILD_SA rekey arrives (peer-initiated) or fires on ze's own timer.
- Format at entry: an SK-encrypted CREATE_CHILD_SA carrying SA, Ni/Nr, TSi, TSr.

### Transformation Path
1. `respondChildRekey` or `applyChildRekeyResponse` builds the replacement pair
   through `newRekeyedChild`, inheriting the selector fields.
2. `installChildTolerant` calls `installChildSA`, which installs two states and
   then, today, two policies.
3. The peer's Delete for the retired pair reaches `closeDesignatedChildSAs`,
   which calls `removeChildSA` and, today, removes the policies.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ dataplane | `dataplane.Dataplane` interface: `InstallSA`, `RemoveSA`, `InstallPolicy`, `RemovePolicy`, `RemovePolicyParams` | No |
| Dataplane ↔ kernel | netlink XFRM: `XfrmStateAdd`, `XfrmPolicyAdd`, `XfrmPolicyDel` | Yes, measured in QEMU |

### Integration Points
- `ChildSA` (`engine/child.go`) carries the selector fields the policy is built
  from. It is the natural place to record whether this pair owns the policy.
- `PeerSession` (`engine/register.go`) outlives every Child SA of the session,
  so it is the natural owner of the policy lifecycle.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire buffers on this path |
| Registration over hardcoding | N-A | no new command, view, family or handler |

## Key Design Decisions

- **The policy belongs to the session, not to a Child SA.** The alternative,
  reinstalling with `XfrmPolicyUpdate` on every rekey, keeps the wrong ownership
  and leaves the teardown race in place: the retiring pair would still delete a
  policy the live pair needs.
- **`ReqID` is already the join and does not change.** No new field is needed on
  the wire or in config.
- **Fail closed on a policy install that genuinely fails.** Today an `EEXIST` is
  indistinguishable from a real failure. After this change a duplicate install is
  not attempted at all, so any error from `InstallPolicy` is a real one.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every IPsec tunnel. A policy installed too late leaves traffic unprotected; a policy removed too early drops it. |
| How is it reverted? | Single commit revert. No config migration, nothing a peer observes. |
| Who else touches this path? | `spec-fixit-vpp-ipsec-inoperable` (the VPP backend refuses every policy IKE produces, so it is unaffected until `plan/future/spec-ipsec-vpp-policy-interface.md` lands), `plan/spec-ipsec-opaque-selector-port-mask.md` (the same policy selector). |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The kernel resolves an installed policy to a NEW state with the same `ReqID`, so a rekey needs no policy touch | RFC 4301 Section 4.4.1.2; `xfrmPolicyFromParams` writes the template `Reqid` | The replacement carries no traffic and the fix is worse than the defect | A QEMU test that installs a policy, installs state A, replaces it with state B at the same `ReqID`, and asserts the policy still resolves | unvalidated |
| A-2 | Only `createFirstChildSA` and the rekey paths install a Child SA | grep of `installChildSA` and `installChildTolerant` call sites | A third path installs no policy and the tunnel never comes up | grep at implementation time | unvalidated |
| A-3 | Selectors never change across a rekey | `newRekeyedChild` inherits `TSLocal`/`TSRemote`; RFC 7296 Section 2.9.2 forbids narrowing below the scope in use | A rekey that legitimately renegotiates selectors needs the policy replaced, so the session-owned policy is stale | Read `respondChildRekey`'s `narrowChildSelectors` and decide whether it can widen or shift the pair | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A-3 is broken: a rekey CAN change the selectors, so a session-owned policy would be wrong after one | `narrowChildSelectors` returns a pair different from `old.Selectors` | Compare the replacement's selectors against the installed policy's, and replace the policy only when they differ |
| R-2 | Session teardown paths are more numerous than `cleanupChild`, so a policy leaks | An XFRM policy survives a `clear` in the QEMU test | Enumerate every teardown path in the same pass, as the `forgetKeys` work did for keys |
| R-3 | The transport-mode policy has a different shape, so one lifecycle does not fit both | `xfrm_transport_integration_linux_test.go` reddens | Keep the mode on the session-owned record, exactly as `ChildSA` carries it now |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A CREATE_CHILD_SA rekey the peer initiates | → | `respondChildRekey` → `installChildTolerant` | `TestChildRekeyInstallsStatesWithoutTouchingThePolicy` |
| A CREATE_CHILD_SA rekey ze initiates | → | `applyChildRekeyResponse` → `installChildTolerant` | `TestInitiatedChildRekeyInstallsStatesWithoutTouchingThePolicy` |
| The peer's Delete confirming a rekey | → | `closeDesignatedChildSAs` → `removeChildSA` | `TestRetiringASupersededPairKeepsTheLivePolicy` |
| Session teardown | → | `cleanupChild` | `TestSessionTeardownRemovesThePolicyExactlyOnce` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A Child SA rekey completes while the retired pair is installed | The replacement's states are installed and no `InstallPolicy` call is made |
| AC-2 | A Child SA rekey completes | The rekey does not fail, and no state is rolled back |
| AC-3 | The peer's Delete retires the superseded pair | The retired pair's states are removed and the policy remains installed |
| AC-4 | The session ends with a live pair and a superseded pair | Every state and both policies are removed, and no second delete is attempted |
| AC-5 | A real Linux kernel, a policy installed, a state replaced at the same `ReqID` | The policy still resolves to the new state (A-1) |
| AC-6 | A policy install fails for a reason other than a duplicate | The Child SA install fails and says why, exactly as today |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a site-to-site tunnel past its Child SA lifetime | timer → `initiateChildRekey` → `applyChildRekeyResponse` → states installed → peer Delete → old states removed | `TestChildRekeyInstallsStatesWithoutTouchingThePolicy` plus the QEMU rekey test |
| 2 | Runs a tunnel against strongSwan past the rekey interval | the IPsec interop lab with a short Child SA lifetime | the interop scenario named below |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestChildRekeyInstallsStatesWithoutTouchingThePolicy` | `internal/component/ike/engine/rekey_policy_test.go` | AC-1, AC-2 through a fake dataplane that records every call | |
| `TestRetiringASupersededPairKeepsTheLivePolicy` | `internal/component/ike/engine/rekey_policy_test.go` | AC-3 | |
| `TestSessionTeardownRemovesThePolicyExactlyOnce` | `internal/component/ike/engine/rekey_policy_test.go` | AC-4 | |
| `TestPolicyInstallFailureStillFailsTheChildSA` | `internal/component/ike/engine/rekey_policy_test.go` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no new numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-child-rekey-xfrm` | `test/ipsec/ipsec-child-rekey-xfrm.ci` | AC-1 to AC-4 through the daemon: two ze instances rekey a Child SA with the REAL xfrm backend, and the tunnel still has a policy afterwards. `option=needs-linux:caps=net-admin`, so it runs under `make ze-qemu-needs-linux-test` | |
| `TestXFRMPolicyResolvesToAReplacedState` | `internal/component/ike/dataplane/xfrm_rekey_policy_integration_linux_test.go` | AC-5, against a real kernel in QEMU | |
| `TestXFRMSecondInstallOfOneSelectorIsRefused` | same file | the measurement this spec rests on | done |

**Why the existing `.ci` is blind to this.** `test/ipsec/ipsec-child-rekey.ci`
already drives a full Child SA rekey between two ze instances, and it passes. It
sets `option=env:var=ze_test_ike_dataplane:value=noop`, so `installChildTolerant`
returns before any XFRM call. The test proves the protocol exchange and can never
see the dataplane collision. The new `.ci` above is that test with the real
backend and the capability marker the backend needs.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| a short-lifetime rekey scenario | `test/interop-ipsec/` | strongSwan | a tunnel survives a Child SA rekey and keeps carrying traffic | |

## Files to Modify
- `internal/component/ike/engine/child.go` - split the policy install and remove
  out of `installChildSA` and `removeChildSA`.
- `internal/component/ike/engine/rekey.go` - the two rekey paths install states only.
- `internal/component/ike/engine/delete.go` - retiring a superseded pair removes
  states only.
- `internal/component/ike/engine/established.go` - `cleanupChild` owns the policy
  removal, once.
- `internal/component/ike/engine/register.go` - `PeerSession` records the installed
  policy pair.

## Files to Create
- `internal/component/ike/engine/rekey_policy_test.go` - the four unit tests above.

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
| Doctor check for runtime dependencies | No | no new runtime dependency; XFRM is already checked |
| Prometheus counters/metrics | No | the failure this removes was silent, and the fix makes it not happen; a counter would count zero |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| Doc | Update? | File / reason |
|-----|---------|---------------|
| Feature list | No | no new feature; a defect is removed |
| User guide | No | no operator-visible change |
| RFC compliance | Yes, done 2026-08-19 | `docs/features/rfc-status.md` RFC 7296 row now records Section 2.2's two independent Message ID counters, Section 2.9's narrowing orientation, and the Section 2.8 rekey measured on the XFRM backend |
| Architecture | Yes, done 2026-08-19 | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`: `SelectorsLocalIsTSi` is the selector orientation and `LocalIsInitiator` is the KEYMAT role, so one field no longer carries both. This row read "one policy, successive states" until 2026-08-19, which described the VOID session-owned design |
| Architecture, rekey wire | Yes, done 2026-08-22 | `docs/architecture/ike/ipsec-13-rekey-wire.md`: a Decisions paragraph states RFC 7296 Section 2.2's two independent counters against `finishResponderEstablish` and `classifyInbound`, and the `## Proof` section names both exchange directions. It had claimed interop proof from scenario 05 alone, which stays green when the responder counter fix is reverted |
| Architecture, engine | No | `docs/architecture/ike/ipsec-7-ikev2-engine.md` is declared by `internal/component/ike/engine/register.go`, which "Files to Modify" above lists for the VOID session-owned design. No `register.go` edit shipped and `PeerSession` gained no policy record, so the engine page's shape is unchanged |
| Agent digest | Yes, done 2026-08-22 | `ai/digests/ipsec-ike.md`: the rekey bullet claimed interop verification from scenario 05 alone. It now names both exchange roles and what the second one proves |
| Test infrastructure | Yes, done 2026-08-22 | `test/interop-ipsec/scenarios/26-responder-raises-child-rekey` is new. Scenarios are discovered from the directory listing (`test/interop-ipsec/run.py`), so it needs no registration, and `make ze-interop-ipsec-test` runs it |

## Implementation Steps

1. Validate A-1 first, in QEMU. If the kernel does NOT resolve a replaced state
   through the existing policy, this design is wrong and the spec stops here.
2. Validate A-3 by reading `narrowChildSelectors`. If a rekey can change the
   selectors, add the compare-and-replace path R-1 names before going further.
3. Add the policy record to `PeerSession` and the four unit tests, red first.
4. Split the policy calls out of `installChildSA` and `removeChildSA`.
5. Point the two rekey paths and the retire path at the states-only variants.
6. Enumerate every session teardown path, as the `forgetKeys` work did.
7. Run the QEMU integration suite and the strongSwan interop lab.

## Goal Gates

- `make ze-precommit-verify` passes.
- `make ze-qemu-integration-test` passes, including the two tests above.
- The strongSwan interop scenario survives a Child SA rekey.

## Quality Gates

- Every AC has a named test whose assertion states the AC's observable behavior.
- Every test is mutation-verified: disabling the production change reddens it.

## RFC Documentation (Scope: protocol)

RFC 7296 Section 2.8 is already enrolled. This spec adds no requirement row: it
makes an implemented obligation actually work. If `make ze-rfc-check` shows a
Section 2.8 row whose evidence is unit-only, this is the work that earns it a
functional or interop tier.

## Known Limitations

The VPP IPsec backend cannot be driven by IKE: it installs SAs and refuses every policy
the engine produces (`spec-fixit-vpp-ipsec-inoperable`, closed 2026-08-10), so this fix is
proven on XFRM only.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] A-1 validated in QEMU before any production edit
- [ ] A-3 validated by reading `narrowChildSelectors`
- [ ] Every teardown path enumerated
- [ ] `make ze-precommit-verify` green
- [ ] QEMU integration green
- [ ] Interop scenario green

## Audit 2026-08-02: implemented by a DIFFERENT design. NOT ready to close

Read against the code on 2026-08-02, during the closure of
the rfcgate-1b RFC 7296 pilot spec. This section is a bookkeeping record. It changes
no code and closes nothing.

**The tunnel-level defect is fixed and interop-proven. The spec text is counterfactual.**
Commit `1963345b4` added this spec file and the fix in one commit, and the fix took the
alternative this spec's Key Design Decisions section explicitly REJECTS. There is no
`PeerSession`-owned policy record. What shipped is:

- `xfrmBackend.InstallPolicy` upserts, at `internal/component/ike/dataplane/xfrm_linux.go`
  (`netlink.XfrmPolicyUpdate`). No `XfrmPolicyAdd` remains in production code (verified
  2026-08-02 by a tree-wide grep over `internal/`, `cmd/` and `pkg/`).
- Removal is guarded by a shared-selector comparison: `samePolicySelector`
  (`internal/component/ike/engine/child.go`) feeding `dropPolicy` in
  `removeChildSAExcept` (`child.go`).

**AC verdicts.**

| AC | Verdict | Evidence |
|----|---------|----------|
| AC-1 | NOT landed as written | `installChildSA` still calls `dp.InstallPolicy` unconditionally at `child.go` and `child.go`, on every install including both rekey paths. The AC says "no `InstallPolicy` call is made", which is false today. The HARM it proxied for is gone, cured one layer down by the upsert |
| AC-2 | Landed, by a different mechanism | The duplicate error cannot occur, because UPDPOLICY replaces. `TestXFRMPolicyInstallIsIdempotent` |
| AC-3 | Landed | `removeChildSAExcept`, retired from `closeDesignatedChildSAs`. `TestRetiredChildKeepsThePolicyTheReplacementUses` |
| AC-4 | Partially landed | First clause holds. The second ("no second delete is attempted") is false BY DESIGN: the peer-Delete-then-teardown path repeats `removeChildSA` on the live pair, and the code comment concedes it as harmless |
| AC-5 | NOT landed | `TestXFRMPolicyResolvesToAReplacedState` does not exist. Neither QEMU test installs an XFRM state at all, so neither exercises reqid resolution. **A-1 is still `unvalidated` and still load-bearing**, because the upsert design has one policy serve successive states |
| AC-6 | Landed, behavior unchanged | The install-failure paths still fail closed. No test named for it exists |

**Residual work.** Each line is concrete enough to implement from.

- Rewrite Key Design Decisions and the ACs to state the design that SHIPPED: one upserted
  policy per selector plus a shared-selector guard on removal. Not a `PeerSession`-owned
  policy record.
- Restate AC-1: `InstallPolicy` IS called on rekey and MUST be idempotent.
- Restate AC-4's second clause, or stop the live pair's teardown being repeated.
- Write the AC-5 QEMU test: install a policy, install a state at one request id, replace it
  with a second state at the same request id, assert the policy still resolves. This is what
  validates A-1.
- Write the AC-6 test proving a non-unsupported policy-install error still fails the Child SA
  install and rolls back both states.
- Add a teardown test for the two-child "exactly once" case. The existing test covers only
  the single-child case.
- Audit `reapStalePending` and `cleanupPendingSA` (`engine/established.go`): both tear down a
  pending child with no `keep`, so a pending child sharing the live child's selector and
  interface id can strip the LIVE pair's policy. This is the bug class AC-3 fixes, on a path
  the fix did not reach. **Read, not measured. Unverified.**
- Audit the three responder rollback sites for the same hazard.
- Add a `.ci` exercising the real dataplane. `test/ipsec/ipsec-child-rekey.ci` still pins the
  no-op dataplane and is blind to this whole class, exactly as this spec's own "Why the
  existing `.ci` is blind" paragraph says.
- Reconcile Files to Modify with reality: `dataplane/xfrm_linux.go` was changed and is not
  listed.

Tracked by `plan/deferrals/rfcgate-1b-rfc7296-pilot.md`, which names this spec as the
destination. The spec stays OPEN.

## Audit 2026-08-02, second pass: three more policy-identity residuals

Found while closing the rfcgate-1b RFC 7296 pilot spec, after the audit above was
written. All three sit on the same surface this spec owns: what identifies a policy, and
what may remove one. They are appended rather than merged into the list above, so the
first pass stays as it was written.

The three responder rollback sites are NOT repeated here. The list above already carries
them as "Audit the three responder rollback sites for the same hazard", and they are
`responder.go`, `:821` and `:846`. Each calls `removeChildSA`, which is
`removeChildSAExcept(child, nil, ...)`, so a rollback can remove a policy a surviving
pair still answers to.

**1. `samePolicySelector` flips orientation on a peer-initiated rekey.** `selectorPort`
(`internal/component/ike/engine/child.go`) chooses which side of the selector pair to
read from `child.LocalIsInitiator` at `:732`. `newRekeyedChild` resets that field. So the
same selector compares unequal across a peer-initiated rekey, and `samePolicySelector`
(`child.go`) answers false where it should answer true.

Refuted as a policy DROP: an incorrect false makes `removeChildSAExcept` drop a policy it
should keep, which the upsert then reinstalls. The real consequence is an ORPHANED policy,
left behind because the guard did not recognise the pair that still needs it. Exercising it
needs a peer-proposed port narrowing, which no current test produces. READ, not measured.

**2. Removal and install disagree on what identifies a policy.** Install passes `SPParams`
carrying `Proto`, `IfID`, `ReqID`, `Action` and `Priority`
(`internal/component/ike/dataplane/dataplane.go`). Removal is the three-argument
`RemovePolicy(src, dst *net.IPNet, dir SADir)` (`dataplane.go`), which carries no
ports, no protocol and no interface id.

This predates the `keep` work, so it is not a regression. It matters now because the `keep`
design assumes removal matches install: a guard that decides "this selector is still in
use" is only as good as the identity the removal call can express. A removal keyed on
fewer fields than the install can remove a policy the guard meant to protect.

**3. `resolvePendingAfterOwnerLoop` returns early on a nil `pendingSA`.** The function
(`internal/component/ike/engine/fsm.go`, called at `:330`) returns before it can
consider a `pendingChild`. A non-nil `pendingChild` holding the only claim on a shared
policy is therefore stranded when `pendingSA` is nil.

READ, not measured, and stated that way deliberately. What would settle it is a case that
produces a nil `pendingSA` beside a non-nil `pendingChild`. If that combination cannot
arise, the finding dissolves, and proving it cannot arise is the cheaper answer.

**All three stay OPEN and none is closed by the first pass.** Rows in
`plan/deferrals/rfcgate-1b-rfc7296-pilot.md` name this spec as their destination.

**Superseded 2026-08-22.** All three are closed, and so are the first pass's own
residuals. The Review Gate section above carries the evidence for each, and the
Deferrals Resolved table below carries the verdicts. The shard named in the line above
no longer exists: it was removed when the rfcgate-1b spec closed.

## Implementation Summary

### What Was Implemented

- `ChildSA.SelectorsLocalIsTSi` (`internal/component/ike/engine/child.go`): the orientation
  of `ChildSA.Selectors`, written once at creation from the IKE_AUTH role and inherited
  unchanged by `newRekeyedChild`. `selectorPort` reads it. `LocalIsInitiator` keeps its
  KEYMAT meaning and has exactly one production reader left, the RFC 7296 Section 2.17
  key-half choice in `installChildSA`.
- `swapPairs` (`internal/component/ike/engine/ts_narrow.go`): the whole conversion between
  the two TSi/TSr orientations, its own inverse. `respondChildRekey` swaps the stored floor
  into the request's orientation before comparing.
- `narrowChildSelectors` orients the configured policy by the RESPONDER role, constant,
  because every caller of it is answering a proposal.
- `finishResponderEstablish` (`internal/component/ike/engine/responder.go`) no longer writes
  `sa.NextMsgID` from the peer's IKE_AUTH id. `resumeRequestsAfter` is deleted.
- `test/ipsec/ipsec-child-rekey-xfrm.ci`: a Child SA rekey on the real XFRM backend, roles
  crossed, reading the kernel rather than a log line.
- `internal/component/ike/dataplane/xfrm_reqid_resolution_integration_linux_test.go`:
  assumption A-1, measured in QEMU.
- `test/interop-ipsec/scenarios/26-responder-raises-child-rekey`: a responder-role Ze
  raising its own CREATE_CHILD_SA against strongSwan.

### Bugs Found/Fixed

- One field carrying two meanings, so a role-flipping rekey installed a port-swapped policy
  (R-1). `TestRekeyKeepsThePolicyOrientationOfTheRetiredPair`.
- The responder narrowed against the IKE SA role, so with `traffic-selector` configured Ze
  answered TS_UNACCEPTABLE to every peer-initiated Child SA rekey on a tunnel it had
  initiated. `TestPeerInitiatedRekeyIsNarrowedInTheExchangeOrientation`.
- The responder's Message ID counter took the peer's request id, so a responder-role Ze
  could raise no exchange at all. `TestResponderFirstRequestMatchesWhatTheInitiatorExpects`,
  and now `test/interop-ipsec/scenarios/26-responder-raises-child-rekey` against strongSwan.
- Each of the three carries a row in `plan/journal/field-carries-two-meanings.md`.

### Documentation Updates

- `docs/features/rfc-status.md`, RFC 7296 row: Section 2.2 two counters, Section 2.9
  narrowing orientation, Section 2.8 rekey on the XFRM dataplane.
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`: "One field cannot carry both the
  KEYMAT role and the selector orientation", anchored to
  `child.go -- ChildSA.SelectorsLocalIsTSi, selectorPort` and `rekey.go -- newRekeyedChild`.
- `docs/architecture/ike/ipsec-13-rekey-wire.md`: a Decisions paragraph on the two
  independent request counters, anchored to `responder.go -- finishResponderEstablish` and
  `msgid.go`, and a `## Proof` section naming both exchange directions.
- `ai/digests/ipsec-ike.md`: the rekey bullet names both interop scenarios.
- `make ze-doc-verify` PASSED (3025 digest anchors resolve).

### Deviations from Plan

- The whole session-owned policy design is VOID. What shipped is one upserted policy per
  selector plus a shared-selector guard on removal, and the 2026-08-17 rewrite made that the
  spec's subject. AC-1..AC-6 are void; AC-R1..AC-R4 are the criteria.
- `internal/component/ike/engine/register.go` and `established.go` are in "Files to Modify"
  for the void design and were not touched.
- `internal/component/ike/engine/rekey_policy_test.go` was never created. The four unit tests
  it was to hold describe the void design.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 was written as "selectors never change across a rekey", to be validated by reading `narrowChildSelectors` for widening | The mechanism is different: `narrowChildSelectors` writes only `sa.NegotiatedPairs`, and `newRekeyedChild` inherits `old.Selectors` without ever reading the narrowing result. A rekey cannot move the installed selector at all | reading the producer at implementation time | A-3 is confirmed by a different mechanism, and R-1's compare-and-replace mitigation is not needed. `coversFloor` (`ts_narrow.go`) then made the RESPONDER's answer agree with what it installs, under the sibling spec. The INITIATOR path is not settled: `applyChildRekeyResponse` reads no TSi or TSr from a rekey response, and `spec-fixit-child-rekey-answer-vs-installed-selectors` owns it |
| approach | The spec proposed moving the policy to `PeerSession` ownership | The defect was already cured one layer down by `xfrmBackend.InstallPolicy` upserting, and per-peer `SPParams.Owner` refuses the foreign peer EEXIST used to refuse | the 2026-08-02 audit read the shipped code against the spec | The shipped design is KEPT and the spec was rewritten to it (`ai/rules/no-layering.md`, `ai/rules/simplicity.md`) |
| escalation | A wire-visible protocol change shipped with no foreign-implementation witness, and the spec's own Interop Tests row stayed blank through implementation | Every existing strongSwan scenario has Ze initiating, or responding-and-answering. None had a responder-role Ze speak first, so reverting the counter fix left all of them green | closure review, round 2 | `test/interop-ipsec/scenarios/26-responder-raises-child-rekey`, mutation-verified. The general lesson is that "an interop scenario exists for this feature" is not the same claim as "an interop scenario fails when this change is reverted" |
| approach | The new scenario read Ze's XFRM state through `lab.ze_xfrm_state`, copied from scenario 05 | `docker_exec_quiet` returns `""` for a failed read AND for a kernel holding no state, and the scenario reads an empty set as "no XFRM here" and skips every dataplane assertion. A broken `docker exec` would have skipped them in silence | `make ze-functional-docker-exec-check` went RED at 122 sites against a floor of 121, blocking `ze-precommit-verify` for the whole checkout | The scenario reads through `docker_exec`, which raises and names the container and the command. The floor was not raised, and the scenario was re-run because the fix changed the path that produced its PASS |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| R-1: separate the selector orientation from the KEYMAT role | Done | `internal/component/ike/engine/child.go`, `ChildSA.SelectorsLocalIsTSi`, `selectorPort`; `rekey.go`, `newRekeyedChild` | `LocalIsInitiator` has one production reader left, and it is the Section 2.17 key-half choice |
| R-2: a rekey `.ci` that can observe XFRM | Done | `test/ipsec/ipsec-child-rekey-xfrm.ci` | Real backend on peer-a, noop on peer-b, roles crossed, asymmetric port |
| R-3: a QEMU test for reqid resolution | Done | `internal/component/ike/dataplane/xfrm_reqid_resolution_integration_linux_test.go` | Bracketed by controls that must move the counter |
| Found and fixed: the responder's narrowing orientation | Done | `internal/component/ike/engine/ts_narrow.go`, `narrowChildSelectors` | `policyPairs(sa.PeerCfg, false)`; every caller responds |
| Found and fixed: the responder's Message ID counter | Done | `internal/component/ike/engine/responder.go`, `finishResponderEstablish` | `resumeRequestsAfter` deleted from `msgid.go` |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-R1 | Done | `TestRekeyKeepsThePolicyOrientationOfTheRetiredPair`, case "peer rekeys a ze-initiated child" | Asserts `selectorPort` on both sides, `samePolicySelector`, and `childPolicyParams` ports in both directions |
| AC-R2 | Done | The same test, case "ze rekeys a peer-initiated child" | Same assertions, roles reversed |
| AC-R3 | Done | `test/ipsec/ipsec-child-rekey-xfrm.ci` | Reads `ip xfrm state` and `ip xfrm policy` through three phases and fails on POLICY-MOVED |
| AC-R4 | Done | `TestXFRMPolicyResolvesToAReplacedState` | One policy, state A then state B at one reqid, read from `XfrmOutNoStates` and each state's own packet counter |
| AC-1..AC-6 | VOID | - | They describe the session-owned design the 2026-08-17 rewrite discarded |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestChildRekeyInstallsStatesWithoutTouchingThePolicy` | Changed | - | VOID with AC-1. The shipped design DOES call `InstallPolicy` on rekey, and it upserts. `TestXFRMPolicyInstallIsIdempotent` (`dataplane/xfrm_rekey_policy_integration_linux_test.go`) is the surviving claim |
| `TestRetiringASupersededPairKeepsTheLivePolicy` | Done, renamed | `internal/component/ike/engine/child_rekey_policy_test.go`, `TestRetiredChildKeepsThePolicyTheReplacementUses` | |
| `TestSessionTeardownRemovesThePolicyExactlyOnce` | Changed | - | VOID: the live pair's teardown is repeated by design and the code concedes it as harmless |
| `TestPolicyInstallFailureStillFailsTheChildSA` | Changed | - | VOID with AC-6. The install-failure paths still fail closed and are unchanged by this spec |
| `ipsec-child-rekey-xfrm` | Done | `test/ipsec/ipsec-child-rekey-xfrm.ci` | |
| `TestXFRMPolicyResolvesToAReplacedState` | Done | `internal/component/ike/dataplane/xfrm_reqid_resolution_integration_linux_test.go` | |
| `TestXFRMSecondInstallOfOneSelectorIsRefused` | Changed | - | Gone with `XfrmPolicyAdd`. `TestXFRMPolicyInstallIsIdempotent` replaces it |
| a short-lifetime rekey scenario, strongSwan | Done | `test/interop-ipsec/scenarios/05-child-rekey` and `26-responder-raises-child-rekey` | 26 is new here and covers the direction 05 cannot fail on |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/child.go` | Done | `SelectorsLocalIsTSi` added and read; not the split the void design asked for |
| `internal/component/ike/engine/rekey.go` | Done | Orientation inherited, floor swapped into the request's orientation |
| `internal/component/ike/engine/delete.go` | Changed | Untouched. `removeChildSAExcept` already guards the retire path |
| `internal/component/ike/engine/established.go` | Changed | Untouched by this spec |
| `internal/component/ike/engine/register.go` | Changed | Untouched. It was listed to hold the void `PeerSession` policy record |
| `internal/component/ike/engine/rekey_policy_test.go` | Changed | Never created. `child_rekey_orientation_test.go` and `child_rekey_policy_test.go` hold the surviving claims |
| `internal/component/ike/engine/msgid.go` | Done, not in the plan | `resumeRequestsAfter` deleted |
| `internal/component/ike/engine/responder.go` | Done, not in the plan | `finishResponderEstablish` |
| `internal/component/ike/engine/ts_narrow.go` | Done, not in the plan | `swapPairs`, and the responder-constant narrowing orientation |

### Audit Summary

- **Total items:** 27
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 7, every one of them a consequence of the 2026-08-17 rewrite to the shipped
  design, recorded in Deviations above. No item was reduced in scope.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A rekey never moves the installed policy selector, whichever end starts it | unit, both role directions | `TestRekeyKeepsThePolicyOrientationOfTheRetiredPair` PASS 2026-08-22 (`make ze-unit-pkg-test PKG=./internal/component/ike/engine`). It compares `selectorPort`, `samePolicySelector` and `childPolicyParams` across the rekey, on a selector with a different port on each side, which is what makes a swap observable |
| The rekey works on the REAL dataplane, not the noop backend | functional, QEMU, real kernel | `test/ipsec/ipsec-child-rekey-xfrm.ci`. It waits for two states, then two more beside them, then the retirement, and compares the policy selector set across all three. It discriminates: with the orientation read reverted it prints POLICY-MOVED and names the port that changed sides |
| The kernel resolves one policy to successive states at one request id (A-1) | integration, QEMU, real kernel | `TestXFRMPolicyResolvesToAReplacedState`. Bracketed by an opening and a closing control that MUST move `XfrmOutNoStates`, plus each state's own packet counter, so the "counter did not move" readings are readings and not a dead instrument |
| A responder-role Ze can raise its own exchange, and a foreign implementation accepts it | interop, strongSwan 5.9.14 | `test/interop-ipsec/scenarios/26-responder-raises-child-rekey` PASS 2026-08-22, ESP counters advancing on both containers after the rekey. MUTATION-VERIFIED: in a throwaway worktree at HEAD with `sa.NextMsgID = msgID + 1` restored, it FAILS with "strongSwan never parsed a CREATE_CHILD_SA REKEY_SA request from a responder-role Ze" |
| A tunnel survives a Child SA rekey against strongSwan and keeps carrying traffic | interop, strongSwan 5.9.14 | `test/interop-ipsec/scenarios/05-child-rekey` (Ze as connection initiator) and scenario 26 (Ze as original responder). Both assert ESP counters advance per SPI after the rekey, on Ze and on the peer |
| RFC 7296 Section 2.2's two counters are gated, not merely implemented | RFC gate | `rfc/short/rfc7296.md` row `[RFC7296-2.2-3] [MUST]`, both polarities tagged, `rfc/extraction/rfc7296.json` records it under section 2.2 `unsourced-ids` with a resign reason. `make ze-rfc-check` OK: 2966 gated MUST-level requirements, 3581 tags resolved |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The spec metadata declares no deferral shard (`Deferral shard: -`) and none exists | done | `plan/deferrals/rfcgate-1b-rfc7296-pilot.md`, which the two 2026-08-02 audits name as the source of their rows, was removed when that spec closed. `grep -rl "child-sa-rekey-policy" plan/deferrals/` returns nothing, so no live row anywhere names this spec |
| First audit pass: `reapStalePending` and `cleanupPendingSA` tear down a pending child with no `keep` | done | `reapStalePending` (`established.go`) calls `removeChildSAExcept(pc, firstSharingSelector(pc, ps.getChildSA()), ...)`. `cleanupPendingSA` keeps its unconditional removal on purpose: it runs only after `Stop()` has joined the owner goroutine, when every policy of the session is going |
| First audit pass: audit the three responder rollback sites for the same hazard | done | They are one site now, `rollbackFirstChildSA` (`responder.go`), and it passes `firstSharingSelector(child, ps.getChildSA(), ps.getPendingChild())` |
| Second audit pass 1: `samePolicySelector` flips orientation on a peer-initiated rekey | done | It is R-1 and it is fixed here |
| Second audit pass 2: removal and install disagree on what identifies a policy | done | Every engine Child SA policy removal calls `RemovePolicyParams(childPolicyParams(child, dir))`. No production caller of the three-argument `RemovePolicy` remains |
| Second audit pass 3: `resolvePendingAfterOwnerLoop` strands a `pendingChild` on a nil `pendingSA` | cancelled | The combination cannot arise, which the audit named as the cheaper answer. Proof in the Review Gate section above |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `test/ipsec/ipsec-child-rekey-xfrm.ci` | Yes | `ls -l` 2026-08-22: 9.0K |
| `internal/component/ike/dataplane/xfrm_reqid_resolution_integration_linux_test.go` | Yes | `ls -l` 2026-08-22: 12K |
| `internal/component/ike/engine/child_rekey_orientation_test.go` | Yes | `ls -l` 2026-08-22: 18K |
| `internal/component/ike/engine/rfc7296_msgid_responder_request_test.go` | Yes | `ls -l` 2026-08-22: 4.7K |
| `test/interop-ipsec/scenarios/26-responder-raises-child-rekey/check.py` | Yes | `ls -l` 2026-08-22, beside `ze.conf` and `swanctl.conf` |
| `internal/component/ike/engine/rekey_policy_test.go` | No | `ls` reports no such file. It belongs to the VOID design; see Files from Plan |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-R1 | a peer-initiated rekey of a Ze-initiated Child SA keeps the policy orientation | `make ze-unit-pkg-test PKG=./internal/component/ike/engine RUN='^TestRekeyKeepsThePolicyOrientationOfTheRetiredPair$'` -> ok, 2026-08-22. The producer is `newRekeyedChild` (`rekey.go`) inheriting `old.SelectorsLocalIsTSi`, read by `selectorPort` (`child.go`) |
| AC-R2 | the same with the roles reversed | Same run, table case "ze rekeys a peer-initiated child" |
| AC-R3 | a rekey is observed on the real XFRM dataplane | `test/ipsec/ipsec-child-rekey-xfrm.ci` carries `option=needs-linux:caps=net-admin`, runs `peer-a` without `ze_test_ike_dataplane=noop`, and asserts on `ip xfrm state` and `ip xfrm policy`. PASS in QEMU 2026-08-18 against commit `86b6aa291`; `child.go`, `responder.go` and `msgid.go` have not moved since (`git log 86b6aa291..HEAD` is empty for all three) |
| AC-R4 | one policy resolves to a replaced state at one reqid | `TestXFRMPolicyResolvesToAReplacedState`, QEMU 2026-08-18. Neither the test nor `dataplane/xfrm_linux.go` has changed since |
| Message ID counters | a responder-role Ze raises its first request at id 0 | `make ze-unit-pkg-test RUN='^TestResponderFirstRequestMatchesWhatTheInitiatorExpects$'` -> ok, 2026-08-22, and `test/interop-ipsec/scenarios/26-responder-raises-child-rekey` PASS against strongSwan 2026-08-22 |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A CREATE_CHILD_SA rekey the peer initiates, answered on the real XFRM backend | `test/ipsec/ipsec-child-rekey-xfrm.ci` | Yes. Read, not inferred: peer-b holds `lifetime 10` and the noop backend, peer-a holds `lifetime 3600` and the real one, so peer-b's soft expiry drives the rekey peer-a answers. `rekey.py` reads the kernel through three phases |
| A CREATE_CHILD_SA rekey Ze raises as the original IKE responder | `test/interop-ipsec/scenarios/26-responder-raises-child-rekey/check.py` | Yes. charon carries `start_action = start` and every rekey timer at 0, Ze carries `connection-type respond` and `lifetime 30`, so the only rekey on the wire is Ze's |
| `ChildSA.SelectorsLocalIsTSi` reaches the kernel policy | `test/ipsec/ipsec-child-rekey-xfrm.ci` | Yes. `selectorPort` -> `childPolicyParams` -> `InstallPolicy`, and the `.ci`'s selector carries a port on one side only, which is what makes an orientation error visible in `ip xfrm policy` |
| `TestChildRekeyInstallsStatesWithoutTouchingThePolicy` and the other three Wiring Test rows | none | VOID with the session-owned design. The two rows above replace them |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestXFRMPolicyResolvesToAReplacedState`, QEMU 2026-08-18, against a real kernel, bracketed by two controls |
| A-2 | confirmed | `grep -rn "createFirstChildSA(" internal/` gives two production callers, `initiatorFirstChildSA` (`child.go`) and `buildAuthResponse` (`responder.go`); `installChildTolerant` has two, `applyChildRekeyResponse` and `respondChildRekey`. Three install paths, no fourth |
| A-3 | confirmed, by a different mechanism | `narrowChildSelectors` writes only `sa.NegotiatedPairs` (`ts_narrow.go`), and `newRekeyedChild` inherits `old.Selectors` without reading it, so a rekey cannot move the installed selector. Recorded in the Mistake Log |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/rfc-status.md`, "Section 2.2 two counters ... It took the peer's id before this fix" | `finishResponderEstablish` writes no `NextMsgID`; `advanceMsgID` and `advanceExpectedMsgID` (`msgid.go`) own the two counters and the ceiling | Yes |
| `docs/features/rfc-status.md`, "Section 2.9 narrowing orientation" | `narrowChildSelectors` calls `policyPairs(sa.PeerCfg, false)` (`ts_narrow.go`) | Yes |
| `docs/features/rfc-status.md`, "Section 2.8 rekey on the XFRM dataplane" | `newRekeyedChild` inherits `Selectors` and `SelectorsLocalIsTSi`; `test/ipsec/ipsec-child-rekey-xfrm.ci` measures it | Yes |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, "One field cannot carry both..." | Source anchors `child.go -- ChildSA.SelectorsLocalIsTSi, selectorPort` and `rekey.go -- newRekeyedChild` both resolve | Yes |
| `docs/architecture/ike/ipsec-13-rekey-wire.md`, the two-counter paragraph and `## Proof` | Anchored to `responder.go -- finishResponderEstablish` and `msgid.go`; the Proof section names scenario 05, scenario 26 and the `.ci`, and each exists | Yes |
| `ai/digests/ipsec-ike.md`, the rekey bullet | Both scenario paths exist; `make ze-doc-verify` resolves 3025 digest anchors | Yes |
| Feature list, user guide: no update | `grep -rn "SelectorsLocalIsTSi" docs/guide docs/features` finds nothing outside the rfc-status row, and no operator-visible surface changed: no YANG leaf, no CLI verb, no env var | Yes |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md`: no update | It is declared by `internal/component/ike/engine/register.go`, which this spec lists for the VOID session-owned design and never edited. `git log 86b6aa291..HEAD -- register.go` and the diff of `86b6aa291` both show no change to it | Yes |

## Core Insight

A field's name says what it holds; only its WRITER says when it changes. `LocalIsInitiator`
was written per exchange and read as though it were written per SA, and the two readings
agreed on every tunnel until the far end started a rekey. The fix was not a better read: it
was giving the second meaning its own field, written beside the data it describes and
inherited with it.

The same shape produced the Message ID defect one function away. `sa.NextMsgID` names this
end's next request, and `finishResponderEstablish` wrote it from a number that belonged to
the peer's counter. RFC 7296 Section 2.2 states the two counters are independent in
indicative prose with no capitalised keyword, so the extraction walk derived no site, no
checklist row existed, and the gate that exists to catch exactly this was silent. An
obligation a keyword scan cannot see is still an obligation, and `unsourced-ids` is where it
goes.

What made the third finding possible is narrower and worth keeping: a suite can hold a
scenario for a feature and still hold none that FAILS when the change is reverted. Ze had a
strongSwan Child SA rekey scenario throughout. It had Ze on the initiating side, so it could
not have caught a defect that silenced the responding side completely.

The scenario written to close that gap opened the same shape one more time, in the test
harness rather than the daemon. `docker_exec_quiet` returns the empty string for a failed
read and for a kernel holding no state, and the scenario reads an empty state as "this host
has no XFRM" and skips its dataplane assertions. One value, two meanings, and the caller
cannot tell them apart: the daemon defect and the test defect are the same sentence.
