# Spec: fixit-linger-rejection-reaches-no-verdict

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-16 |

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
  `reject=` directive. `make ze-functional-plugin-test`, or any suite running that fixture,
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
| A-1 | No caller of `completed` acts on a failing `Result` when `Linger` is set | the runner decides on the token | the fix is smaller: wire the existing return | read every call site in `internal/test/peer/peer.go` | unvalidated |
| A-2 | Fixtures relying on linger for a negative are currently green and vacuous | one measured case | fewer fixtures are affected | grep `option=linger` against `reject=` and run each | unvalidated |

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
| a `.ci` whose check peer lingers and then receives a frame its `reject=` forbids | -> | the verdict path from `completed` to `failedCheckPeers` | a new `.ci` under `test/plugin/`, plus a Go case in `internal/test/runner/peer_contract_test.go` |

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

- `internal/test/peer/reject.go` -- `(*Peer).completed`, and its doc comment,
  which currently states a behavior the code does not have
- `internal/test/runner/peer_contract.go` -- `failedCheckPeers`
- `docs/architecture/testing/ci-format.md` -- the `option=linger` contract
- `test/plugin/filter-family-export-flowspec.ci` -- its header claim, once true

## Files to Create

- a `test/plugin/` fixture proving AC-1 and AC-2

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
   - Verify: `make ze-doc-verify`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations

- A fixture whose linger red is a real product defect is out of this spec's
  scope: it gets its own row and its own spec. This spec makes the mechanism
  able to fail, it does not fix what starts failing.
