# Spec: fixit-linger-rejection-reaches-no-verdict

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | AC-1, AC-2 and AC-3 landed at `715a54fad`. AC-4 and AC-5 open: they are a suite run, not a code change |
| Deferral shard | - |
| Updated | 2026-08-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`option=linger` cannot fail a test. A rejection the linger loop finds is detected,
returned, and then discarded, so every negative assertion held open by linger is
vacuous.

`(*Peer).completed` (`internal/test/peer/reject.go`) prints the peer's success
token BEFORE it enters the linger loop:

- with `Linger` unset it returns `Result{Success: true}` and the caller prints
  the token,
- with `Linger` set it prints `"\nsuccessful\n"` itself, then loops, and on a
  rejection returns the failing `Result` from `p.rejected`.

`failedCheckPeers` (`internal/test/runner/peer_contract.go`) decides each
check-mode peer on the token alone: `strings.Contains(peers[i].combined(),
peerSuccessToken)`. Once the token is on the output, the peer counts as passed.
The failing `Result` reaches no verdict.

**The comment on `completed` states the opposite**, and it is the reason the
mechanism is trusted: "The linger loop reads to keep the session alive, and it
re-checks every frame it reads against the rejections. That is what makes
`option=linger` the way to hold a negative assertion open for the rest of the
test." The first sentence is true. The second is false as wired.

**Reproduction, already measured.** With an export filter mutated to accept, a
check peer printed `successful`, then `lingering`, then the forbidden
`180A0000` frame, and the runner reported PASS.

**Blast radius is every fixture that relies on linger for a negative.**
`test/plugin/filter-family-export-flowspec.ci` states in its own header that
linger is what catches a route arriving after the End-of-RIB. That test is
currently green and proves nothing about that route.

**How it was found.** Repairing AC-3 of `spec-fixit-ci-peer-block-silent-directives`
needed a negative assertion held past the End-of-RIB. Linger was the documented
way and did not work. The export fixtures there were built on a fence instead, so
that spec's goal did not depend on this and it was written up rather than fixed
in place (`ai/rules/completion.md`). Journal row:
`plan/journal/balance-assertion-vacuous-without-a-loan.md`.

**Why the token is the wrong contract, not just the wrong order.** Printing the
token early was deliberate: the comment says teardown is a kill and a post-`Run`
print can be lost. So moving the print to the end trades a vacuous pass for a
lost pass. The verdict has to stop being carried by a string on the peer's
stdout, or the string has to be retractable.

**This is the same shape `failedCheckPeers` already exists to close.** Its own
comment records that the verdict used to be one `strings.Contains` over every
peer's output concatenated, so the first peer's success masked the rest, and two
suite tests read as "flaky, passes 1 in 10" while failing deterministically. The
fix moved the check per-peer. It did not move the check off the token, so the
same vacuous-pass shape survives one layer in.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` -- the `option=linger` contract as
      published, which this spec must correct or make true
- [ ] `ai/rules/testing.md` -- the vacuous-test rule this defect is an instance of
- [ ] `ai/rules/evidence.md` -- a guard that neither denies nor speaks does not
      exist

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/peer/reject.go` -- `(*Peer).completed`, `(*Peer).rejected`
  → Constraint: the early print exists because teardown is a kill. Any fix must
    keep a real pass visible when the process is killed.
- [ ] `internal/test/runner/peer_contract.go` -- `failedCheckPeers`,
      `peerSuccessToken`, `countCheckPeers`, `isSelfValidated`
  → Constraint: `failedCheckPeers` already fails closed when fewer peers
    produced a capture than the `.ci` declared. The token check is the only
    branch that fails open.
- [ ] `internal/test/peer/peer.go` -- every call site of `completed`, and who
      consumes the `Result` it returns
  → Decision: if no caller consumes a failing `Result` from `completed`, the
    return value is dead and that is the defect's second half.

**Behavior to preserve:**
- A genuine pass stays visible after a kill-based teardown.
- `option=linger` keeps holding the session open for the rest of the test.
- Non-linger check peers keep their current verdict path.
- The per-peer verdict `failedCheckPeers` already establishes.

**Behavior to change:**
- A rejection found during linger fails the test.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.ci` fixture declares a check-mode `ze-peer` with `option=linger` and a
  `reject=` directive. `./le functional plugin`, or any suite running that fixture,
  is the only entry point.

### Transformation Path
1. The fixture's peer block is parsed and a check-mode `ze-peer` is launched.
2. The exchange completes and `(*Peer).completed` is reached.
3. With linger, the success token is printed and the loop begins.
4. A frame arriving later is re-checked by `p.rejected`.
5. A rejection returns a failing `Result`.
6. `failedCheckPeers` reads the peer's combined output, sees the token, and
   passes the peer. Step 5's value is never consulted.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze-peer -> runner | the peer's stdout text, scanned for `peerSuccessToken` | Yes, read in `failedCheckPeers` |
| linger loop -> caller | the `Result` returned by `completed` | Not verified: whether any caller acts on it is the first thing to read |

### Integration Points
- `docs/architecture/testing/ci-format.md` publishes the `option=linger` contract.
- Every `.ci` carrying `option=linger` with a negative assertion.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | the verdict bypasses the `Result` and rides on stdout text |
| No unintended coupling (components stay isolated) | Yes | the fix stays inside the test rig |
| No duplicated functionality (extends existing, does not recreate) | Yes | `failedCheckPeers` is the existing verdict site and is where the fix belongs |
| Zero-copy preserved where applicable (refs, not copies) | N-A | test rig, not a hot path |
| Registration over hardcoding | N-A | no registration surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | No caller of `completed` acts on a failing `Result` when `Linger` is set | the runner decides on the token | the fix is smaller: wire the existing return | read every call site in `internal/test/peer/peer.go` | **broken** |
| A-2 | Fixtures relying on linger for a negative are currently green and vacuous | one measured case | fewer fixtures are affected | grep `option=linger` against `reject=` and run each | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Fixing the verdict turns currently-green fixtures red, because they were never proving what they claim | the first suite run after the fix | each red is a real finding, not a regression: triage rather than revert |
| R-2 | Moving the print loses a genuine pass under kill-based teardown | a pass becomes a spurious fail | do not move the print; retract it or carry the verdict off stdout |

## Blast Radius

Every `.ci` fixture that pairs `option=linger` with a negative assertion, and the
published `option=linger` contract. No daemon code.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| a `.ci` whose check peer lingers and then receives a frame its `reject=` forbids | -> | the verdict path from `completed` to `failedCheckPeers` | `TestRejectedPrintsTheRejectionMarker` (`internal/test/peer/reject_test.go:177`) and `TestPeerVerdictFailsOnRetractionAfterSuccess` (`internal/test/runner/peer_contract_test.go:436`), plus the AC-4 mutation run over `test/plugin/filter-family-export-flowspec.ci` |

**Corrected at closure.** This row asked for "a new `.ci` under `test/plugin/`",
and no such file was created, because the suite cannot express a run that MUST
fail: a `.ci` whose verdict is RED is indistinguishable to the runner from a
`.ci` that is broken. The verdict PRODUCER is therefore the proof boundary, and
both halves of it carry a Go test. The end-to-end half is supplied instead by
MUTATING the daemon under an existing fixture and observing that fixture turn
red, which is AC-4 and which ran on 2026-08-29.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A check peer with `option=linger` receives a frame its `reject=` forbids, after the success token was printed | The test FAILS, naming the rejection |
| AC-2 | A check peer with `option=linger` receives nothing forbidden | The test PASSES, as today |
| AC-3 | A check peer with `option=linger` is killed at teardown after a genuine pass | The test PASSES: the fix does not lose a real pass |
| AC-4 | `test/plugin/filter-family-export-flowspec.ci` | Its header claim is true: reverting the mechanism it names turns it red |
| AC-5 | Every existing fixture carrying `option=linger` | Triaged: each is green for a reason, or its red is a real finding with a row |

## End-to-End User Stories

- A test author writes a negative assertion that must hold for the rest of the
  test, uses `option=linger` as the documentation instructs, and gets a red when
  the forbidden frame arrives.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a check peer whose linger loop rejects is reported failed | `internal/test/runner/peer_contract_test.go` | AC-1 | |
| a check peer that lingers cleanly is reported passed | `internal/test/runner/peer_contract_test.go` | AC-2 | |
| a killed peer that genuinely passed is reported passed | `internal/test/runner/peer_contract_test.go` | AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| linger rejection fails the test | `test/plugin/` | the fixture is red when the forbidden frame arrives and green when it does not | |

## Files to Modify

- `internal/test/peer/reject.go` -- `(*Peer).rejected` prints `RejectionMarker`;
  `(*Peer).completed`'s doc comment now says the returned `Result` is not what
  decides the run. Modified at `715a54fad` and again at closure
- `internal/test/runner/peer_contract.go` -- `failedCheckPeers` reads the
  retraction after the token and lets it win. Modified at `715a54fad`
- `docs/architecture/testing/ci-format.md` -- the `option=linger` contract.
  Modified at closure: the `linger` option row and the `reject=bgp` properties
  list now state that a linger rejection retracts the success token
- `test/plugin/filter-family-export-flowspec.ci` -- its header claim. Modified at
  closure: the header now names the retraction and records the AC-4 mutation

## Files to Create

- None. The `test/plugin/` fixture this section asked for was NOT created, and
  is not owed: see the note under Wiring Test. The suite has no way to declare a
  run that must fail, so a fixture proving AC-1 would be a fixture the runner
  reports as broken.

## Implementation Steps

1. **Phase: Reproduce** -- write the failing case at the runner level
   - Verify: AC-1 is RED with today's code
2. **Phase: Decide the contract** -- the verdict stops riding on a stdout token,
   or the token becomes retractable. Read A-1 first: if the `Result` is already
   returned and merely dropped, wiring it is the smaller fix
   - Verify: AC-3 still green, so a real pass survives a kill
3. **Phase: Fix** -- at the layer the decision names
   - Verify: AC-1, AC-2, AC-3
4. **Phase: Triage the corpus** -- every `option=linger` fixture
   - Verify: AC-4, AC-5, with a row per red
5. **Phase: Correct the published contract and the false comment**
   - Verify: `./le doc check verify`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Deliverables Checklist

<!-- Added at closure: the spec was written without this table. -->

| Deliverable | Verification method | Result |
|-------------|--------------------|--------|
| A linger rejection reaches the verdict | `go test ./internal/test/runner/ -run TestPeerVerdictFailsOnRetractionAfterSuccess` | pass |
| The peer emits the retraction | `go test ./internal/test/peer/ -run TestRejectedPrintsTheRejectionMarker` | pass |
| One definition of the marker, shared by both halves | `grep -n "RejectionMarker" internal/test/peer/reject.go internal/test/runner/peer_contract.go` | `peerRejectionMarker = peer.RejectionMarker`; no second literal |
| The repaired assertion discriminates | mutate `handleFilterUpdate` to stop stripping the matched MP attribute, run `./le functional plugin` | `249 filter-family-export-flowspec` FAILED on `stdin=filtered`; green after revert |
| The published contract states the retraction | `docs/architecture/testing/ci-format.md`, the `linger` option row and the `reject=bgp` properties list | both updated, with source anchors |

## Security Review Checklist

<!-- Added at closure: the spec was written without this table. -->

| Concern | Applies? | Finding |
|---------|----------|---------|
| Untrusted input reaching the new code | No | `RejectionMarker` is a compile-time constant printed by the test peer; the needle it names came from the `.ci`, which the runner already parses with `peer.ParseRejectRule` |
| A verdict that fails OPEN | Addressed | this change is the repair: the token check used to pass a peer that had already been rejected. `failedCheckPeers` still fails closed on a peer that produced no capture |
| Injection through the printed line | No | the line is written to the peer's own stdout and consumed by `strings.Contains` in the runner. A `.ci` author who put the literal `ZE-PEER-REJECTED` in a peer's output could redden their own test; they cannot make a red read green, which is the direction that matters |
| Denial of service / unbounded work | No | one extra `strings.Contains` per check peer, once per run |
| Secrets or privilege | No | test rig only; no daemon code changed by `715a54fad` |

## Documentation Update Checklist

<!-- Added at closure, from ai/rules/planning.md's reference list. -->

| Category | Update needed? | File and section, or the evidence for No |
|----------|---------------|------------------------------------------|
| Feature list | No | `grep -rn "option=linger" docs/features/` returns nothing; linger is test-rig grammar, not a product feature |
| User guide | No | same grep over `docs/guide/` returns nothing |
| Config syntax | No | no YANG leaf, no env var, no config option changed |
| CLI reference | No | no command, flag, exit code or JSON envelope changed |
| API / RPC docs | No | no RPC changed |
| Plugin SDK | No | `pkg/plugin/` untouched |
| Wire format | No | no protocol bytes changed; the marker is peer stdout text |
| RFC compliance | No | no RFC-level behavior implemented, changed or newly proven; `docs/features/rfc-status.md` needs no row |
| Comparison table | No | `docs/comparison.md` makes no claim about the test rig |
| Test infrastructure | **Yes** | `docs/architecture/testing/ci-format.md`: the `linger` option row and the `reject=bgp` "four properties" list. Both updated, each with a `<!-- source: -->` anchor naming the producing symbol |
| Architecture design | No | `docs/architecture/core-design.md` describes the daemon, not the runner's verdict |
| Doctor checks | No | no new runtime dependency: no path, socket, port, module, binary or certificate |

## Known Limitations

- A fixture whose linger red is a real product defect is out of this spec's
  scope: it gets its own row and its own spec. This spec makes the mechanism
  able to fail, it does not fix what starts failing.


## Implementation Summary

### What Was Implemented

- `(*Peer).rejected` (`internal/test/peer/reject.go:146`) prints `RejectionMarker`
  (`"ZE-PEER-REJECTED"`, `reject.go:136`) on the peer's own output before it
  returns the failing `Result`.
- `failedCheckPeers` (`internal/test/runner/peer_contract.go:124`) reads that
  retraction AFTER the success token and lets it override:
  `strings.Contains(out, peerSuccessToken) && !strings.Contains(out, peerRejectionMarker)`.
- The marker has ONE definition. The runner's `peerRejectionMarker`
  (`peer_contract.go:86`) is `= peer.RejectionMarker`, not a second literal, so
  the two halves cannot drift on a string.
- All three landed at `715a54fad`, with a test on each half.
- At closure: the published contract
  (`docs/architecture/testing/ci-format.md`), `(*Peer).completed`'s doc comment,
  and `test/plugin/filter-family-export-flowspec.ci`'s header.

### Bugs Found/Fixed

- **The defect itself.** A rejection found during linger was detected, returned,
  and discarded. Covered by `TestPeerVerdictFailsOnRetractionAfterSuccess`
  (`internal/test/runner/peer_contract_test.go:436`) and
  `TestRejectedPrintsTheRejectionMarker` (`internal/test/peer/reject_test.go:177`).
- **Three plugin fixtures raced ze's initial-sync End-of-RIB.** Found while
  triaging AC-5, and fixed because a suite with known flakes is not evidence for
  AC-5. `system-sockets-show` and `system-profile-show` were registered
  unwrapped in `internal/test/fixture/plugin_fixture_16.go`, while 15 siblings
  in the same table go through `plugin16AfterEOR` (`plugin_fixture_16.go:32`),
  and `fixture10MPLSForwardingShow` (`internal/test/fixture/plugin_fixture_10.go`)
  never called the `fixture10WaitEOR` its own file already defines. Each
  observer returned before ze had sent the End-of-RIB the `.ci` expects, ze then
  shut down, and the peer read a Cease NOTIFICATION where the marker should
  have been. All three now wait. **These two files could not be committed by
  this closure**: they carry another live session's uncommitted repository-wide
  refactor, and naming them would cross-commit it (`ai/rules/git-safety.md`).
  The fix sits in the working tree for the session that owns those hunks to
  land.

### Documentation Updates

- `docs/architecture/testing/ci-format.md`: the `linger` option row (`:347`) and
  a fifth bullet in the `reject=bgp` properties list now state that a rejection
  found during linger retracts the success token. Both carry
  `<!-- source: -->` anchors naming `RejectionMarker`, `(*Peer).rejected`,
  `failedCheckPeers` and `peerRejectionMarker`.
- `test/plugin/filter-family-export-flowspec.ci`: its LINGER paragraph named
  `internal/test/peer/peer.go` for `Peer.completed`, which lives in
  `internal/test/peer/reject.go`. Corrected, and the paragraph now names the
  retraction and records the AC-4 mutation.
- No other category applies; see the Documentation Update Checklist above for
  the grep behind each No.

### Deviations from Plan

- **No new `.ci` was created**, and the Files to Create section now says so.
  The runner has no way to declare a run that MUST fail, so a fixture proving
  AC-1 would be reported as a broken fixture. The verdict producer is the proof
  boundary instead, and the end-to-end half is supplied by AC-4's mutation over
  an EXISTING fixture.
- **A-1 was broken**, and the fix was therefore not "wire the existing return".
  See the Mistake Log.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed no caller of `completed` acts on a failing `Result` under linger | `zeTestPeerCommand` (`internal/test/cli/cmd_peer.go:79-81`) DOES act on it: `if !result.Success { fmt.Fprintln(os.Stderr, "failed"); return 1 }`. The peer prints the failure and exits 1. What discards the verdict is one layer further out: `drainPeers` (`internal/test/runner/runner_exec_util.go:498`) writes `_ = peers[idx].proc.Wait()`, so the runner never reads the peer's exit code | read at closure, from the producing function rather than from the runner's verdict site | recorded as `broken`. It did not change the fix: the verdict still had to reach the runner through the one channel the runner reads, which is the peer's output. It DOES change the diagnosis: the peer was never silent, the runner was deaf |
| approach | The spec's Wiring Test asked for a new `.ci` proving AC-1 | the suite cannot express a run that must fail | at implementation time (`715a54fad`'s message) | replaced with the two Go tests plus AC-4's mutation run; the spec now says so |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A rejection found during linger fails the test | Done | `internal/test/runner/peer_contract.go:133` | the retraction overrides the token |
| A genuine pass stays visible after a kill-based teardown | Done | `internal/test/peer/reject.go:171` | the early print is unchanged; only a retraction can withdraw it |
| `option=linger` still holds the session open | Done | `internal/test/peer/reject.go:167` | `completed`'s loop is untouched |
| Non-linger check peers keep their verdict path | Done | `internal/test/runner/peer_contract.go:124` | a non-linger peer never prints the marker, so the added conjunct is inert for it |
| The published contract stops being false | Done | `docs/architecture/testing/ci-format.md` | the `linger` row and the `reject=bgp` list |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPeerVerdictFailsOnRetractionAfterSuccess`, first assertion | token + marker in one capture is exactly the state the old verdict called a pass |
| AC-2 | Done | `TestPeerVerdictFailsOnRetractionAfterSuccess`, second assertion | token + `lingering`, no marker, passes |
| AC-3 | Done | same test, second assertion | the early print IS the kill-survival path; a peer killed after a genuine pass carries the token and no marker |
| AC-4 | Done | mutation run, 2026-08-29 | see Goal Validation |
| AC-5 | Done | `./le functional plugin`, two runs | see Goal Validation |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| a check peer whose linger loop rejects is reported failed | Done | `internal/test/runner/peer_contract_test.go:436` | |
| a check peer that lingers cleanly is reported passed | Done | `internal/test/runner/peer_contract_test.go:436` | second assertion of the same test |
| a killed peer that genuinely passed is reported passed | Done | `internal/test/runner/peer_contract_test.go:436` | same assertion: the capture of a killed peer that passed is the token with no marker |
| linger rejection fails the test (functional) | Changed | AC-4 mutation over `test/plugin/filter-family-export-flowspec.ci` | see Deviations: no new `.ci` |
| the peer emits the retraction | Done | `internal/test/peer/reject_test.go:177` | added beyond the plan |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/peer/reject.go` | Done | `715a54fad` + closure comment |
| `internal/test/runner/peer_contract.go` | Done | `715a54fad` |
| `docs/architecture/testing/ci-format.md` | Done | closure |
| `test/plugin/filter-family-export-flowspec.ci` | Done | closure |
| a new `test/plugin/` fixture | Changed | not created, and not owed: see Deviations |

### Audit Summary
- **Total items:** 20
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (both in Deviations: the unwritten `.ci`, and the functional test it would have carried)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `option=linger` can fail a test | functional, by mutation | `handleFilterUpdate` (`internal/component/bgp/plugins/filter_family/handler.go:30`) was mutated in a detached worktree at `106d9e063` to accept the matched ipv4/flow UPDATE instead of stripping its MP attribute. `./le functional plugin` then reported `249 filter-family-export-flowspec` FAILED on `stdin=filtered`, and the peer's capture reads `successful` / `lingering` / the forbidden frame / `ZE-PEER-REJECTED`, ending `failed: received bytes that reject=bgp:pattern=01180A0100 forbids`. Reverting the mutation returned the same fixture to green in the next run. That is AC-4: the assertion discriminates |
| The negative assertions held open by linger stop being vacuous | corpus triage | The vacuous population is `option=linger` paired with `reject=bgp`, which is NINE fixtures, not the 62 that pair linger with any `reject=`. `reject=bgp` is the only reject type ze-peer reads (`docs/architecture/testing/ci-format.md`); the other 53 use `reject=stderr` / `reject=stdout`, which the RUNNER checks over the client's output and which the linger loop never touched. The nine are `redistribute-export-reject`, `bgp-rs-control-community-withdraw-egress`, `filter-family-export-flowspec`, `wellknown-no-export-withdraw-egress`, `rfc7606-54-discard-unrecognized-nlri`, `wellknown-no-advertise-egress`, `wellknown-no-export-egress`, `control-community-withdraw-egress`, `originated-nexthop-peer-own`. All nine are green under the repaired verdict, in both the idle run of 2026-08-29 (641/644, 59 skip, 102s) and the control run at `106d9e063` (641/644, 59 skip, 109s) |
| The published contract is true | documentation | `docs/architecture/testing/ci-format.md` states the retraction in both places a reader meets `linger`, each with a source anchor |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The spec declares `Deferral shard: -` and `plan/deferrals/fixit-linger-rejection-reaches-no-verdict.md` does not exist. `grep -rn "linger-rejection-reaches-no-verdict" plan/deferrals/` returns nothing, so no foreign shard names this spec as a Destination |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-linger-rejection-reaches-no-verdict-bae6e1b4-738f-4436-9754-92603923b680.md` (2 code files) |
| `./le spec session review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | logic + wiring, guard correctness (fail-open direction), evidence/vacuity, prose and STE, style pass over the changed Go |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The spec recorded A-1 as `unvalidated` while the code it judged had already shipped, and the assumption is FALSE as written | this spec, Risks & Assumptions | read `zeTestPeerCommand` and `drainPeers`; A-1 marked broken with the producing function named, and a Mistake Log row added |
| 2 | ISSUE | `(*Peer).completed`'s doc comment still let a reader believe the returned `Result` decides the run | `internal/test/peer/reject.go:167` | a paragraph naming `failedCheckPeers` and the retraction |
| 3 | ISSUE | The published `option=linger` contract said nothing about the retraction, so an author reading it could not tell a working negative assertion from the vacuous one | `docs/architecture/testing/ci-format.md` | the `linger` option row and a fifth `reject=bgp` property, with source anchors |
| 4 | ISSUE | Three `test/plugin` fixtures raced ze's initial-sync End-of-RIB, so AC-5's own evidence was a suite with known flakes in it | `internal/test/fixture/plugin_fixture_16.go`, `internal/test/fixture/plugin_fixture_10.go` | the two unwrapped observers now go through `plugin16AfterEOR`, and `fixture10MPLSForwardingShow` calls `fixture10WaitEOR`. All three green in the control run |
| 5 | ISSUE | The `.ci` header cited `internal/test/peer/peer.go` for `Peer.completed`, which lives in `reject.go` | `test/plugin/filter-family-export-flowspec.ci` | citation corrected; the paragraph now also names the retraction |
| 6 | ISSUE | Round 2, on this closure's OWN diff: the `reject=bgp` section opens "Four properties make it an assertion rather than a hope" and finding 3 added a fifth bullet under it, so the count contradicted the list it introduces | `docs/architecture/testing/ci-format.md:849` | corrected to "Five properties". `grep -rn "four properties" ai/ docs/` finds no other citer of that count |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/peer/reject.go` | Yes | `gopls symbols` lists `RejectionMarker Constant 136:7`, `(*Peer).rejected Method 146:16`, `(*Peer).completed Method 167:16` |
| `internal/test/runner/peer_contract.go` | Yes | `grep -n peerRejectionMarker` -> `:86`, `:133` |
| `internal/test/peer/reject_test.go` | Yes | `gopls symbols` lists `TestRejectedPrintsTheRejectionMarker Function 177:6` |
| `internal/test/runner/peer_contract_test.go` | Yes | `grep -n "^func Test"` -> `TestPeerVerdictFailsOnRetractionAfterSuccess` at `:436` |
| `test/plugin/filter-family-export-flowspec.ci` | Yes | the AC-4 run names it as `249/703` |
| a new `test/plugin/` fixture | No, and not owed | see Deviations |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a rejection after the token fails the peer | `./le job run label unit-verdict command go test ./internal/test/runner/ ./internal/test/peer/ -run '...' -count=1` -> `ok internal/test/runner 0.712s`, `ok internal/test/peer 0.348s` |
| AC-2 | a clean lingering peer passes | same run, second assertion of the same test |
| AC-3 | a killed peer that genuinely passed still passes | same assertion; and every non-rejecting linger fixture in the suite stayed green across both runs |
| AC-4 | the assertion discriminates | mutation run: `249 filter-family-export-flowspec` FAIL with `ZE-PEER-REJECTED` in the capture; green on revert |
| AC-5 | the linger corpus is triaged | nine `linger` + `reject=bgp` fixtures enumerated and all green; the other 53 `linger` + `reject=` fixtures use runner-side rejects and were never in the class |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a check peer that lingers and then receives a forbidden frame | `test/plugin/filter-family-export-flowspec.ci` | Yes. Read the file: `filtered-peer` carries `option=linger:value=true`, an `expect=` on the ipv4/flow End-of-RIB, and `reject=bgp:conn=1:pattern=01180A0100`; `unfiltered-peer` asserts the same needle IS on the wire, so a run that never announced the route fails there instead. Under mutation the fixture went red on `filtered-peer` and the capture named the pattern |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `zeTestPeerCommand` (`internal/test/cli/cmd_peer.go:79-81`) acts on the failing `Result`: prints `failed` to stderr and returns 1. The discard is `drainPeers` (`internal/test/runner/runner_exec_util.go:498`), whose `_ = peers[idx].proc.Wait()` throws the peer's exit status away, with the comment "the verdict reads output, not this status" |
| A-2 | confirmed | `filter-family-export-flowspec` was green before `715a54fad` and its CLAIM 1 could not fire, which the mutation run now demonstrates from the other side. The corrected population is nine fixtures, not 62 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "a rejection found during linger RETRACTS the success token" | `internal/test/peer/reject.go:152` prints the marker; `internal/test/runner/peer_contract.go:133` reads it after the token | Yes |
| Feature list / user guide / comparison | `grep -rn "option=linger" docs/features/ docs/guide/ docs/comparison.md` returns nothing | Yes |
| RFC status | no protocol behavior implemented, changed or newly proven; the diff is the test rig and its documentation | Yes |
| Doctor checks | no new runtime dependency introduced | Yes |

## Core Insight

**A verdict carried on an append-only channel cannot be withdrawn, so the
producer has to be able to retract.** The success token was printed early for a
correct reason (teardown is a kill, and a post-run print is lost), and that made
the announcement unwithdrawable while the verdict was a bare
`strings.Contains` for it. Moving the print would have traded a vacuous pass for
a lost pass. The fix was neither: keep the announcement early, and give the same
channel a retraction the reader checks second.

The general shape is worth carrying: whenever a producer must speak BEFORE it
knows the answer, the consumer needs a second token whose absence is not
silence. `failedCheckPeers` already knew this in one direction (it fails closed
when fewer peers reported than the `.ci` declared) and not in the other.
