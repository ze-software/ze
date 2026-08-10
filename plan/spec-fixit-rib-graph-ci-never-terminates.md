# Spec: fixit-rib-graph-ci-never-terminates

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Corrected 2026-08-10. The original premise was wrong: `rib-graph.ci` is not
special.** Every scaffolding-peer test in `test/plugin/` pays the same flat 10s.
The population is 29 files: 28 declare `--mode sink` and `event-predicate-wait.ci`
declares `--mode echo`, which the same teardown gap reaches. The three graph tests
only sit closest to the edge because their budget is 20s. Startup plus 10s lands
on that budget, so the outcome turns on machine load: `rib-graph-best` and
`rib-graph-filtered` were seen timing out at 20.0s and later passing. Nothing
about an observer, a barrier, or the route server distinguishes them. Measured:
sink-mode tests 12.6-13.2s against check-mode siblings at 4.1-4.3s.

**The gain is per-test latency, not suite wall clock.** Measured on 2026-08-10 at
`-p 12`: 585/585 in 213.9s with the fix against 211.6s without, because the 10s
drains overlapped other tests. What the fix buys is the margin on the three
20s-budget graph tests and on the 15s-budget echo test, each of which sat within
seconds of its deadline. No record of this work may read as a suite speed-up.

**One of those four is not slow, it is RED.** Measured 2026-08-10 by the review,
unfixed and serial: `test/plugin/event-predicate-wait.ci` FAILS at 15.0s with
`TYPE: timeout` while the daemon itself completed correctly, its observer having
logged the matched update and a clean shutdown. The 10s drain alone exceeds the
margin the test had. Fixed, it passes in 2.4s. This defect is therefore a real
failure that presented as slowness, not a latency cost.

The cost is harness teardown, not the daemon and not the test. A sink, echo or
inject `ze-peer` never ends itself, nothing signals it, and the drain barrier
then waits out its whole bound.

**Raising the timeout is NOT the fix, and neither is raising `peerDrainGrace`.**
Both hide a wait that should be zero.

**Found** 2026-08-08 while re-running the plugin suite to verify unrelated
fixes during the repair of GitHub Actions run 31225029268. `rib-graph` was not
one of the 7 stages that run failed, so this is separable and was recorded
rather than fixed, on owner instruction.

**The three open questions are answered** (2026-08-10, each against its
producing function):

| Question | Answer |
|----------|--------|
| Which side fails to signal | The runner. `(*Runner).runOrchestrated` in `internal/test/runner/runner_exec.go` takes the `ExpectExitCode` arm and waits the daemon alone, its teardown loop skips peer processes by construction, and `drainPeers` in `runner_exec_util.go` then waits out `peerDrainGrace` |
| Whether a sink `ze-peer` ever exits on its own | It does not. The accept loop of `(*Peer).Run` in `internal/test/peer/peer.go` continues for every non-check mode, and its only exit is `ctx.Done()`, which returns success. `internal/test/cli/cmd_peer.go` maps SIGTERM to that cancel, so a signal IS the peer's normal exit |
| Whether changes landed 2026-08-08 caused it | No. Nothing in the reactor, the route-server plugin or the plugin supervisor is on this path. The cost is a property of every sink-mode test, not of a date |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` peer block and its consumers
  -> Decision: the runner owns peer teardown; a `.ci` file declares no teardown directive
  -> Constraint: only a check-mode peer reports a verdict, so only a check-mode peer may be left to exit on its own

**Key insights:** (minimal context to resume after compaction)
- The daemon shuts down correctly. The whole cost is harness teardown.
- `peerOutput` already records `checkMode` at launch, so the two peer kinds are
  distinguishable where the fix goes. No new state is needed.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/peer/peer.go` - the accept loop of `(*Peer).Run` continues for every non-check mode, so a sink peer has no exit but `ctx.Done()`, which returns success
- [ ] `internal/test/cli/cmd_peer.go` - SIGTERM and SIGINT cancel that context, so a signal is the peer's normal exit and it still leaves status 0
- [ ] `internal/test/runner/runner_exec.go` - `(*Runner).runOrchestrated`: the `ExpectExitCode` arm waits the daemon alone, and the teardown loop skips every peer process
- [ ] `internal/test/runner/runner_exec_util.go` - `drainPeers` waits the un-waited peers under `peerDrainGrace` (10s) and returns when that bound expires
- [ ] `internal/test/runner/peer_contract.go` - `failedCheckPeers` reads each check peer's own capture, which is what a signal would destroy

**Behavior to preserve:**
- Every assertion the tests make today. They pass; only teardown is slow.
- The check-mode verdict path: a check peer is never signaled by the runner.

**Behavior to change:**
- A scaffolding peer is signaled at teardown instead of being waited out.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`.ci` line `cmd=background:exec=ze-peer --mode sink ...`, parsed into a
`RunCommand` and launched by `(*Runner).runOrchestrated`.

### Transformation Path
1. The launch records `checkMode` on the peer's `peerOutput`, from `isCheckPeerExec`.
2. The `ExpectExitCode` arm waits the foreground daemon, and leaves `waited` false on every peer.
3. The teardown loop signals the daemons and skips the peers.
4. `terminateScaffoldPeers` sends SIGTERM to each non-check peer. It waits for nothing.
5. `drainPeers` reaps the signaled peers under its existing grace and returns in milliseconds.

### Boundaries Crossed

| From | To | What crosses |
|------|----|--------------|
| runner teardown | ze-peer process | SIGTERM |
| ze-peer signal handler | `(*Peer).Run` accept loop | context cancel |
| ze-peer exit | runner capture | the peer's complete stdout and stderr, and status 0 |

### Integration Points
One call in `(*Runner).runOrchestrated`, between the daemon teardown loop and
the drain barrier. The `await=stderr` arm reaches the same tail.

The call sits AFTER the arm switch, not before it. Moving it earlier would also
cover the `default:` arm, whose own `proc.Wait()` loop waits a scaffolding peer
out to the test budget, but it is not free: while an arm runs, the daemon is
still exchanging with these peers. `event-predicate-wait.ci` needs its echo peer
to reflect an UPDATE back before `ze` exits, and an earlier signal cuts the
exchange that test measures. Proven by experiment (2026-08-10): a build with the
call moved ahead of the switch fails that test with `ZE-OBSERVER-FAIL`, because
the echo peer is killed before it can reflect the UPDATE.

The echo test is the ONLY test that constrains the call site. The three graph
tests do not: their peer is a sink that asserts nothing, routes reach the graph
through the API, and all three PASS in 3.3s with the call moved ahead of the
switch. The code comment in `runner_exec.go` names `event-predicate-wait.ci`
alone for that reason.

No `.ci` reaches the `default:` arm with a scaffolding peer today: all 29 declare
`expect=exit` or `await=`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `cmd=background:exec=ze-peer --mode sink` in `test/plugin/rib-graph.ci` | -> | `terminateScaffoldPeers` called by `(*Runner).runOrchestrated` | the test's own wall clock, serial: 13.0s before, 2.8-3.1s after |

## 🧪 TDD Test Plan

### Unit Tests

Each test is mutation-proven: dropping the term it covers from the predicate in
`terminateScaffoldPeers` turns that test red and leaves the other green.

| Test | File | Validates | Fails when |
|------|------|-----------|-----------|
| `TestTerminateScaffoldPeersReapsSinkPeer` | `internal/test/runner/peer_teardown_test.go` | the sink peer is signaled, so the `drainPeers` that follows reaps it and returns at once | the loop body stops signaling: an unsignaled `sleep` outlives the grace and the 5s deadline fires |
| `TestTerminateScaffoldPeersLeavesCheckPeer` | `internal/test/runner/peer_teardown_test.go` | a check peer is not signaled and is still running afterwards | `checkMode` is dropped from the predicate: the peer is signaled and exits in milliseconds |

There is no third test. `terminateScaffoldPeers` waits for nothing, so it needs
no `waited` guard and there is no second `Wait` for a test to cover.

### Functional Tests
- [ ] `test/plugin/rib-graph.ci`, `rib-graph-best.ci` and `rib-graph-filtered.ci` pass, each about 10s faster
- [ ] `test/plugin/event-predicate-wait.ci` passes, 10s faster: unfixed and serial it TIMES OUT at its 15s budget
- [ ] `make ze-plugin-test` shows no regression, which covers both peer kinds. This
  is NOT a claim that the suite is all-green: this checkout is shared and several
  sessions edit it at once. Attribute every red instead of asserting a total.
  Measured 2026-08-10 at `-p 8`: 583/587, 22 skipped, failing `geodns-dot-pki`,
  `mgmt-guard-api-env-started-settings-survive`,
  `mgmt-guard-web-env-started-address-survives` and
  `prefix-teardown-holds-peer-down`. All four are foreign to this change, and the
  reason is structural rather than statistical: the two `mgmt-guard-*` files
  declare no `cmd=background` peer at all, so `terminateScaffoldPeers` iterates an
  empty slice; the other two launch `ze-peer --port $PORT` with no `--mode`, so
  `zePeerExecMode` returns `peer.ModeCheck` and the predicate's first term skips
  them. Re-run on the pre-fix binaries, three of the four pass and
  `mgmt-guard-web-env-started-address-survives` still fails, which is the
  in-flight `spec-fixit-mgmt-listener-auth-guard` work at its last phase.

## Files to Modify

| File | Change |
|------|--------|
| `internal/test/runner/runner_exec_util.go` | add `terminateScaffoldPeers` beside `drainPeers` |
| `internal/test/runner/runner_exec.go` | call it in `(*Runner).runOrchestrated`, before the drain barrier |
| `internal/test/runner/peer_teardown_test.go` | new: the two unit tests above |
| `internal/test/runner/runner_stop_test.go` | `withinDeadline` takes the deadline from a constant, which keeps the package lint-clean. Three call sites already there, and the new test is the fourth. No deadline value changed |

### Documentation Update Checklist (BLOCKING)

Added at closure: the spec was written without it. Every No is backed by a grep,
not by a guess.

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Test-harness teardown. `docs/features.md` names product features, and no row covers the `.ci` runner |
| 2 | Config syntax changed? | No | No YANG leaf and no config reader is touched |
| 3 | CLI command added/changed? | No | No flag, verb or exit code changed. `ze-peer` reads the same `--mode` |
| 4 | API/RPC added/changed? | No | The change is a signal to a child process |
| 5 | Plugin added/changed? | No | Nothing under `internal/plugins/` or `internal/component/bgp/plugins/` |
| 6 | Has a user guide page? | No | The runner is developer infrastructure |
| 7 | Wire format changed? | No | No byte on any BGP connection changes |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin/` or `pkg/ze/` symbol is touched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Teardown of a test process is outside every RFC Ze implements |
| 10 | Test infrastructure changed? | **Yes** | `docs/architecture/testing/ci-format.md`, new section "A scaffolding ze-peer is signaled at teardown", with four anchors |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares daemons, not test harnesses |
| 12 | Internal architecture changed? | No | One helper beside `drainPeers`; no boundary moves |
| 13 | Route metadata keys added/changed? | No | No `meta` key is read or written |
| 14 | Prometheus counters added/changed? | No | No metric is defined or registered |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No `register.go` is touched, so no inventory changes |
| 16 | Any changed source file referenced by existing doc source anchors? | **Yes, checked** | 9 anchors name `runner_exec.go` or `runner_exec_util.go`. Each covers a different subject (netns entry, bare-name resolution, timeout resolution, probe-timeout injection, `modeStop`, `quickZe`, stdout reject, `stopNamedBackground`). None describes peer teardown, so none went stale |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The `--mode sink` example at `ci-format.md` is still exact: the peer block is unchanged and needs no teardown directive |

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
2. Fix at the owning layer, never at the symptom.
3. Prove the fix discriminates: red before, green after.

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| The verdict path is untouched | No check-mode peer is signaled anywhere in the new path, so `failedCheckPeers` reads the same capture it read before |
| The peer's contract is honored | Teardown by signal is the sink peer's documented normal exit, not a kill that loses output |
| No timeout was raised | `peerDrainGrace` and every `.ci` budget are unchanged |
| Every wait stays bounded | `terminateScaffoldPeers` only signals. The `select` on `time.NewTimer(grace)` in `drainPeers` stays the ONLY wait on a peer, so a peer a signal does not stop is still bounded by the grace. Calling `terminateGracefully` here would end in a bare `cmd.Wait()`, replacing that bound with an unbounded wait on the runner's goroutine |
| Nothing new to unwind | The function marks no peer waited and holds no state, so a peer the default arm already waited needs no exclusion: `Signal` on a reaped `os.Process` returns `ErrProcessDone` |
| The call site is late on purpose | It sits after the arm switch: an earlier signal would cut the exchange an arm is still measuring (see Integration Points) |
| Discrimination | Reverting the call restores the 10s, measured on the same binaries |

### Deliverables Checklist

Added at closure: the spec was written without it.

| Deliverable | Verification method |
|-------------|---------------------|
| `terminateScaffoldPeers` exists beside `drainPeers` | `gopls symbols internal/test/runner/runner_exec_util.go` names it |
| It is CALLED, not merely defined | `grep -n 'terminateScaffoldPeers(peerOutputs)' internal/test/runner/runner_exec.go` |
| The call sits after the arm switch and before the drain barrier | read the call site: the `terminateGracefully` loop is above it, the barrier comment below |
| Both unit tests exist and pass | `make ze-test-pkg PKG=./internal/test/runner` |
| The four target `.ci` tests pass | `ze-test bgp plugin -p 1 rib-graph rib-graph-best rib-graph-filtered event-predicate-wait` |
| No timeout and no grace was raised | `git diff` shows no edit to `peerDrainGrace` and no `option=timeout` change in any `.ci` |
| The `.ci` contract is documented | `grep -n 'scaffolding ze-peer is signaled' docs/architecture/testing/ci-format.md` |

### Security Review Checklist

Added at closure: the spec was written without it.

| Check | What to look for |
|-------|-----------------|
| Signal targeting | The PID must come from a process the runner itself started, never from a file or from `.ci` text. `terminateScaffoldPeers` reads `peers[i].proc.Process` only, and `proc` is assigned in `runOrchestrated` after `Start` returned no error |
| Nil dereference | `proc` is nil for a peer never started and for one a `cmd=stop` step cleared. The predicate skips nil before it reaches `.Process` |
| Signal to a reaped PID | `os.Process.Signal` on a process `Wait` already reaped returns `ErrProcessDone` and makes no syscall, so no recycled PID can be reached |
| Resource exhaustion | The function allocates nothing and holds no wait. It cannot extend the runner's runtime; it only shortens it |
| Error leakage | The `Signal` error is deliberately discarded. It carries no test verdict, and reporting it would turn an already-exited peer into a spurious failure |
| Privilege | The signal runs as the test runner's own user against its own child. No setuid path and no external PID is involved |

## Checklist

### Goal Gates (MUST pass)
- [ ] `test/plugin/rib-graph.ci` terminates without a raised timeout
- [ ] The root cause is named at its producing function, not inferred

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `make ze-verify` green before commit

---

## Implementation Summary

### What Was Implemented
- `terminateScaffoldPeers` in `internal/test/runner/runner_exec_util.go`: sends
  SIGTERM to every peer whose `checkMode` is false and whose `proc` is non-nil.
  It waits for nothing, so it adds no wait to the runner's goroutine.
- One call in `(*Runner).runOrchestrated` (`internal/test/runner/runner_exec.go`),
  after the daemon teardown loop and before the drain barrier.
- Two unit tests in `internal/test/runner/peer_teardown_test.go`, each proven to
  fail when the term it covers is dropped from the predicate.
- `withinDeadline` in `internal/test/runner/runner_stop_test.go` now takes its
  deadline from `teardownTestDeadline` instead of a repeated literal. No deadline
  value changed; the three existing call sites and the new one all read 5s.
- `docs/architecture/testing/ci-format.md`: new section "A scaffolding ze-peer is
  signaled at teardown", with four source and test anchors.

### Bugs Found/Fixed
- `test/plugin/event-predicate-wait.ci` was RED, not slow. Unfixed and serial it
  fails at 15.0s with `TYPE: timeout` while the daemon completes correctly. Fixed,
  it passes. The two unit tests cover the mechanism; the `.ci` covers the daemon.

### Documentation Updates
- `docs/architecture/testing/ci-format.md`, section "A scaffolding ze-peer is
  signaled at teardown". Anchors: `runner_exec_util.go -- terminateScaffoldPeers,
  drainPeers`; `runner_exec.go -- runOrchestrated teardown, terminateScaffoldPeers
  call`; `cmd_peer.go -- cmdPeer, SIGTERM mapped to the peer's context cancel`;
  test anchor on `peer_teardown_test.go`.
- No other doc claim went stale: 9 existing anchors name the two changed files and
  each covers a different subject (netns entry, bare-name resolution, timeout
  resolution, probe-timeout injection, `modeStop`, `quickZe`, stdout reject,
  `stopNamedBackground`). None describes peer teardown.

### Deviations from Plan
- The spec was written without a Deliverables Checklist, a Security Review
  Checklist and a Documentation Update Checklist. All three were added at closure
  and filled from the finished work, rather than closing without them.
- `TestTerminateScaffoldPeersSkipsWaitedPeer` was written and then deleted, because
  its subject disappeared when the function stopped waiting. The deletion carries a
  `test-relax:` note at the head of `peer_teardown_test.go`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The original spec framed `rib-graph.ci` as special: an observer, a barrier or a shutdown unique to the graph tests | Nothing distinguishes them. All 29 scaffolding-peer tests in `test/plugin/` pay the same flat 10s; the graph tests only sit closest to their 20s budget | Measuring sink-mode tests at 12.6-13.2s against check-mode siblings at 4.1-4.3s | The Task section was rewritten before implementation. The fix is at the runner, not at any graph test |
| assumption | The cost was framed as latency on four tests that still passed | One of the four was failing. `event-predicate-wait.ci` times out at 15.0s unfixed and serial | Serial re-run on the pre-fix binaries during closure | Recorded in the Task section and in the doc note; the defect's severity is a real red, not a slow pass |
| approach | The record justified the late call site partly by claiming the graph tests need their sink peer to hold the session up | The graph peer is a sink that asserts nothing and routes arrive by API. All three pass in 3.3s with the call moved earlier. Only `event-predicate-wait.ci` constrains the call site | Independent review measured the moved-call build | The graph clause was dropped from Integration Points, so the spec now matches the code comment |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Name the side that fails to signal, at its producing function | Done | `(*Runner).runOrchestrated`, `internal/test/runner/runner_exec.go` | The `ExpectExitCode` arm waits the daemon alone and the teardown loop skips peers |
| Signal a scaffolding peer instead of waiting it out | Done | `terminateScaffoldPeers`, `internal/test/runner/runner_exec_util.go` | SIGTERM only; reaping stays with `drainPeers` |
| Do not raise any timeout or grace | Done | `peerDrainGrace`, `internal/test/runner/runner_exec_util.go` | Still `10 * time.Second`. `git diff` shows no edit to it and no `.ci` budget change |
| Leave the check-mode verdict path untouched | Done | the `checkMode` term of the `terminateScaffoldPeers` predicate | `failedCheckPeers` reads the same capture it read before |
| Keep every wait bounded | Done | `drainPeers` `select` on `time.NewTimer(grace)` | `terminateScaffoldPeers` holds no `Wait`, so it adds no unbounded wait |

### Acceptance Criteria
This spec states its acceptance criteria as Goal Gates rather than AC-N rows.

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| Goal Gate 1: `rib-graph.ci` terminates without a raised timeout | Done | `ze-test bgp plugin -p 1 rib-graph ...`: PASS in 2.7s, against 13.1s on the pre-fix binaries | No `option=timeout` in any `.ci` changed |
| Goal Gate 2: the root cause is named at its producing function | Done | `(*Runner).runOrchestrated` and `drainPeers`, both named in Current Behavior and in the code comment | Not inferred from a caller |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestTerminateScaffoldPeersReapsSinkPeer` | Done | `internal/test/runner/peer_teardown_test.go` | Mutation-proven: an unreachable loop body makes it hang and fail at the 5s deadline, leaving the other test green |
| `TestTerminateScaffoldPeersLeavesCheckPeer` | Done | `internal/test/runner/peer_teardown_test.go` | Mutation-proven: dropping `checkMode` from the predicate signals the peer and fails this test, leaving the other green |
| `test/plugin/rib-graph.ci`, `-best`, `-filtered` | Done | serial run, all PASS at 2.6-2.7s | 12.6-13.4s on the pre-fix binaries |
| `test/plugin/event-predicate-wait.ci` | Done | serial run, PASS | `TIME` at 15.0s on the pre-fix binaries |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/runner/runner_exec_util.go` | Done | `terminateScaffoldPeers` added beside `drainPeers` |
| `internal/test/runner/runner_exec.go` | Done | one call, inside `runOrchestrated` |
| `internal/test/runner/peer_teardown_test.go` | Done | new file, two tests |
| `internal/test/runner/runner_stop_test.go` | Done | `teardownTestDeadline` constant, four call sites |
| `docs/architecture/testing/ci-format.md` | Changed | not in the plan's Files to Modify; added at closure by the Documentation Update Checklist, row 10 |

### Audit Summary
- **Total items:** 16
- **Done:** 15
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`docs/architecture/testing/ci-format.md`, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A scaffolding-peer test no longer pays a flat 10s at teardown | functional, measured both ways | `ze-test bgp plugin -p 1 rib-graph rib-graph-best rib-graph-filtered event-predicate-wait`: 2.7s / 2.6s / 2.7s / 7.0s, all PASS. Same command on the pre-fix binaries: 13.1s / 12.6s / 13.4s and `TIME` at 15.0s |
| The 15s-budget echo test stops failing | functional, discriminating | Pre-fix, serial: `event-predicate-wait` reports `TYPE: timeout` at 15.0s. Fixed: PASS. Reverting the fix restores the failure, so the test is not vacuous |
| The check-mode verdict path is untouched | unit, mutation-proven | `TestTerminateScaffoldPeersLeavesCheckPeer` asserts the check peer is STILL RUNNING after teardown. Dropping `checkMode` from the predicate makes it fail |
| Every wait stays bounded | unit, mutation-proven | `TestTerminateScaffoldPeersReapsSinkPeer` bounds the whole teardown at `teardownTestDeadline` (5s) and asserts it returns under 1s. Making the loop body unreachable makes it hang and fail |
| No regression across the peer population | functional, attributed | `make ze-plugin-test ZE_PLUGIN_PARALLEL=8`: 583/587, 22 skipped. The four reds are foreign, two of them structurally (`mgmt-guard-api-env-started-settings-survive` and `mgmt-guard-web-env-started-address-survives` declare no `cmd=background` peer, so the new loop iterates an empty slice) and two by predicate (`geodns-dot-pki` and `prefix-teardown-holds-peer-down` launch `ze-peer` with no `--mode`, so `zePeerExecMode` returns `peer.ModeCheck` and the first term skips them). Re-run on the pre-fix binaries, three of the four pass and `mgmt-guard-web-env-started-address-survives` still fails |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `test/plugin/rib-graph.ci` times out at 20s run alone, producing function not yet identified | done | This spec. The producing function is `(*Runner).runOrchestrated`, whose `ExpectExitCode` arm never signals a scaffolding peer; `drainPeers` then burns `peerDrainGrace`. Fixed by `terminateScaffoldPeers` |
| `test/plugin/forward-overflow-two-tier.ci` failed once, unreproduced | deferred | Untouched. Still homed at `spec-fixit-forward-overflow-two-tier-flake` |
| BGP-LS withdrawal proven by unit tests only | deferred | Untouched. Still homed at `spec-fixit-bgpls-withdrawal-functional-proof` |
| `path_problem` measures the checkout, not the corpus | assigned | Untouched. Assigned to another session, homed at `spec-fixit-learned-staleness-measurement` |

The shard `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` still holds three
live rows, so this closure does NOT remove it. No foreign shard was emptied.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-rib-graph-ci-never-terminates-640fa955-f03a-45e8-a58f-4b367f5859e6.md` |
| `review_gate.py check` | clean (`review_gate: OK`, hashes match, re-verified at closure) |
| Rounds | 2. Round 2 was earned by a product question: whether the late call site is necessary. It was answered by experiment, not by argument |
| Reviewer lenses used | bounded-wait analysis, check-mode verdict path, mutation discrimination on both predicate terms, and the deleted test's subject |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | Round 2 reported 0 BLOCKER and 0 ISSUE. The 5 NOTEs are recorded below; none is a product defect | - | - |

The five NOTEs, and what each got:

| # | NOTE | Disposition |
|---|------|-------------|
| N1 | Integration Points justified the late call site partly by the graph tests, which measurement contradicts | Record defect. The graph clause was dropped, so the spec now matches the code comment |
| N2 | The goal gate asserted the whole suite passes; it does not in a shared checkout | Record defect. The Functional Tests item and Goal Validation now attribute every red |
| N3 | `TestTerminateScaffoldPeersReapsSinkPeer` asserts elapsed <= 1s on top of the 5s deadline | Kept. 1s against a 10s grace leaves an order of magnitude of headroom, and the assertion is what makes the test discriminate a shortened grace |
| N4 | `TestTerminateScaffoldPeersLeavesCheckPeer` leaves a `proc.Wait()` goroutine past the test body | Kept. `startSleeper` uses `exec.CommandContext(t.Context(), ...)`, so the child dies and the Wait returns when the test context is cancelled |
| N5 | A `--dial` ze-peer never gets `proc` assigned, so nothing signals it | Pre-existing and unreachable: no `.ci` uses `--dial`. Not this change's, and creating a branch for a case with no caller would be machinery the problem does not need |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/runner/peer_teardown_test.go` | Yes | `ls -la`: 3621 bytes, 2026-08-10 |
| `internal/test/runner/runner_exec_util.go` | Yes | `ls -la`: 37714 bytes, 2026-08-10 |
| `internal/test/runner/runner_exec.go` | Yes | `ls -la`: 55321 bytes, 2026-08-10 |
| `internal/test/runner/runner_stop_test.go` | Yes | `ls -la`: 6333 bytes, 2026-08-10 |
| `test/plugin/rib-graph.ci` | Yes | `ls -la`: 4706 bytes |
| `test/plugin/rib-graph-best.ci` | Yes | `ls -la`: 3902 bytes |
| `test/plugin/rib-graph-filtered.ci` | Yes | `ls -la`: 4664 bytes |
| `test/plugin/event-predicate-wait.ci` | Yes | `ls -la`: 3770 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| Goal Gate 1 | `rib-graph.ci` terminates without a raised timeout | `ze-test bgp plugin -p 1 ...`: `2.7s 4/4 PASS 455 rib-graph`. `grep -n 'peerDrainGrace =' internal/test/runner/*.go` still reads `10 * time.Second` |
| Goal Gate 2 | The root cause is named at its producing function | `grep -n terminateScaffoldPeers internal/test/runner/*.go` shows the definition in `runner_exec_util.go` and the single call in `runner_exec.go`, inside `runOrchestrated` |
| TDD: tests fail without the fix | Both unit tests discriminate | Re-run by the independent review under `-overlay`, so no source file was edited: dropping `checkMode` fails `LeavesCheckPeer` alone; an unreachable loop body fails `ReapsSinkPeer` alone |
| TDD: tests pass with the fix | Green | `make ze-test-pkg PKG=./internal/test/runner`: `ok github.com/ze-software/ze/internal/test/runner 13.384s` under `-race` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `cmd=background:exec=ze-peer --mode sink` | `test/plugin/rib-graph.ci` | Yes. The file declares a sink peer, and the wall clock is the assertion: 13.1s pre-fix, 2.7s fixed, both measured serially on the same `.ci` |
| `cmd=background:exec=ze-peer --mode echo` | `test/plugin/event-predicate-wait.ci` | Yes. Pre-fix the file reports `TYPE: timeout` at 15.0s; fixed it passes. That is a red-to-green transition, not a latency change |

### Assumptions Resolved
The spec carries its assumptions as the three open questions in the Task section.

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1: the runner, not the peer, fails to signal | confirmed | `(*Runner).runOrchestrated`: the `ExpectExitCode` arm waits the daemon alone and the teardown loop calls `terminateGracefully` on daemons only |
| A-2: a sink `ze-peer` never exits on its own | confirmed | The accept loop of `(*Peer).Run` (`internal/test/peer/peer.go`) continues for every non-check mode; its only exit is `ctx.Done()`. `cmdPeer` (`internal/test/cli/cmd_peer.go`) maps SIGINT and SIGTERM to that cancel |
| A-3: changes landed 2026-08-08 did not cause it | confirmed | The cost is a property of every scaffolding-peer test, reproduced here on binaries built before this session's change |
| A-4: `proc` is never non-nil with a nil `Process` | confirmed | `runOrchestrated` assigns `po.proc` only after `startWithETXTBSYRetry` returned no error, and the `modeStop` path is the one place that sets it back to nil |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 10, test infrastructure changed | New section in `docs/architecture/testing/ci-format.md`. Its anchors name `terminateScaffoldPeers` and `drainPeers` in `runner_exec_util.go`, `runOrchestrated` in `runner_exec.go`, and `cmdPeer` in `cmd_peer.go`. Each symbol was read at its file before the anchor was written | Yes |
| Row 16, existing anchors on the changed files | `grep -rn 'source: internal/test/runner' docs/ ai/` returns 9 anchors on the two changed files. Their subjects are netns entry, bare-name resolution, timeout resolution, probe-timeout injection, `modeStop`, `quickZe`, stdout reject and `stopNamedBackground`. None describes peer teardown, so none went stale | Yes, no update needed |
| Rows 1-9 and 11-15, and row 17 | No product surface changed: no YANG leaf, no CLI flag, no RPC, no `register.go`, no wire byte, no metric. The `--mode sink` example at `ci-format.md` is unchanged and still exact | Yes, no update needed |

## Core Insight

A bound that is always reached is not a safety net, it is a missing signal wearing
one. `drainPeers` looked correct in isolation: it waits, it times out, it never
hangs. What it could not say is that nothing upstream ever asked the peer to stop,
so the timeout WAS the normal path for 29 tests. The tell is cheap and general:
when a grace is consumed in full on every run rather than on a rare one, the wait
is not slow, it is unanswered.

The second half is what made this hard to see. The cost presented as latency, so it
was recorded as latency, and one of the four affected tests was in fact failing.
A budget converts an unanswered wait into a red at whatever moment machine load
pushes it past the deadline, which is why "it is only slow" is never a safe reading
of a wait nobody terminates.
