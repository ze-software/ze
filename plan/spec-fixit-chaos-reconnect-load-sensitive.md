# Spec: fixit-chaos-reconnect-load-sensitive

| Field | Value |
|-------|-------|
| Status | done |
| Scope | testing |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `plan/deferrals/fixit-chaos-reconnect-load-sensitive.md` |
| Updated | 2026-08-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 by a `make ze-verify-changed` run for
`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. Owner decision the same day: land the pilot work
under an override, and give this defect its own spec rather than a load excuse.

Status is `skeleton` on purpose. The failure is reproduced and attributed, and the
mechanism is NOT fully diagnosed. The section "What is still open" states the remainder.
The next session then inherits a record rather than a guess dressed as a finding.

## Task

**`TestInProcessChaosReconnect` fails on a loaded host and passes on a quiet one.**
`ai/rules/completion.md` names that shape and forbids recording it. A test that
survives only on a quiet host is a broken test. Load is the bug rather than the excuse.

The failure under `ze-verify-changed`:

```
--- FAIL: TestInProcessChaosReconnect (6.30s)
    runner_test.go:696: "1" is not greater than "1"
    Messages: peer should re-establish after chaos disconnect
```

The first assertion passed, so a disconnect DID fire. The second failed, so the peer never
recorded a second `peer.EventEstablished` before the run ended.

**The observed mechanism.** The test asks for `ChaosRate: 1.0` with `ChaosInterval: 1s`
over a fixed `Duration` of 60 seconds (`internal/chaos/inprocess/runner_test.go`).
Chaos therefore offers a disconnect every second for the whole run. Real TCP on loopback
and real goroutine scheduling sit under the virtual clock. On a loaded host a
re-establishment takes longer, while the disconnects keep arriving at the same rate. The
count of `EventEstablished` is read only after `Run` returns
(`runner_test.go`), so nothing waits for the second one.

## Attribution: this is not the pilot's change

`go list -deps ./internal/chaos/inprocess/` returns ZERO packages under
`internal/component/ike`. Every production file the pilot run changed is under
`internal/component/ike/`. The test cannot see any of them.

Reproduction, measured the same day:

| Run | Result |
|-----|--------|
| Under `make ze-verify-changed` | FAIL at 6.30s |
| Isolated, correct feature tags, `-count=5` | 5 of 5 pass, 20.6s total |

The first isolated attempt failed for an unrelated reason and is recorded so nobody repeats
it: a bare `go test` without the feature tags reports `no such module: ze-bgp-conf`.
`ai/rules/commands.md` names that trap.

## The mechanism (resolved 2026-07-31)

**The run ends on VIRTUAL time, and the reconnect it provoked needs REAL time.**

The advance loop exits at `for simulated < cfg.Duration`
(`internal/chaos/inprocess/runner.go`). It spends the 60 second window in about 0.6
seconds of real time, which the file already states at `runner.go`. `simCancel()` then
runs (`runner.go` before this work). The reconnect loop reads that cancel at
`runner.go` and returns instead of restarting the simulator.

An instrumented run under load captured the whole failure in two milliseconds:

```
23:14:44.414  disconnected + reconnecting    (simulator returned, reconnect loop restarted)
23:14:44.415  advance loop exit, simulated=1m0s -> simCancel()
23:14:44.415  error: reading OPEN: use of closed network connection
23:14:44.415  simulator returned, simCtxErr=context canceled
```

The context watcher (`internal/chaos/peer/simulator.go`) closed the connection in
the middle of the OPEN exchange. On a quiet host the same reconnect completes in 10
milliseconds. Nothing in the test waits for it, because the count is read after `Run`
returns (`runner_test.go`).

### Both earlier candidates are refuted

| Candidate | Refutation |
|-----------|------------|
| `guard.AllowChaos` permits a new disconnect before the re-establishment finished | `AllowChaos` (`internal/chaos/guard/guard.go`) holds no establishment gate. Every action except `ActionHoldTimerExpiry` falls into the case at `guard.go`, whose body is a comment and nothing else. It also cannot fire at a peer that is down. `Scheduler.Tick` builds its candidates from established peers alone, and returns early when there are none (`internal/chaos/engine/scheduler.go`) |
| The established snapshot is stale by a tick, so chaos targets a peer that just went down | The staleness is real. It cannot suppress the count. A stale action lands on a channel of capacity 1 (`runner.go`) and is read at `simulator.go`, which the session loop reaches only after `emit(Event{Type: EventEstablished})` at `simulator.go`. The recovery is therefore already counted before any chaos action can end the session |

Both fall to one line. **`internal/chaos/peer/simulator.go` emits `EventEstablished`
before the simulator can read a chaos action, so no chaos-side decision can prevent the
second establishment.** Only a teardown that stops the handshake can, and that is
`simCancel()`. In the captured failure the peer never reached `simulator.go` at all. It
died in `readMsg` during the OPEN exchange (`simulator.go`), which no guard reaches.

## What the diagnosis cost, and what it found

Three further states hide a perturbation that no peer state shows. Each one was found by
measurement after a narrower gate still failed under load.

| Hidden state | Why peer state cannot show it |
|--------------|-------------------------------|
| The disconnect event trails the decision | `EventDisconnected` is emitted only after `<-readerDone`, and `readLoop` waits for its own drain goroutine first (`internal/chaos/peer/simulator_reader.go`). The drain goroutine that writes `establishedState` adds another wakeup |
| An action is queued or running | `ActionConnectionCollision` holds the simulator for 500 milliseconds of real time (`internal/chaos/peer/simulator_actions.go`) and reports `Disconnected: false`. The peer reads as healthy throughout |
| An action's consequence is deferred | `ActionHoldTimerExpiry` only stops the keepalives (`simulator_actions.go`). The session survives until the reactor's hold timer expires 20 seconds of virtual time later |

## A sibling instance, same class, different suite

`test/web/commit-flow.wb` failed the same way on 2026-07-30, in a later
`ze-verify-changed` run. It took 36.8 seconds and failed under load. It passes in 14.1
seconds when `make ze-web-test` runs the suite alone. That is a slowdown of 2.6 times, and
the test does not survive it.

It is recorded here rather than in its own spec, because the class and the fix shape are
identical. Both are functional tests that assert on elapsed time instead of on state.
Whoever fixes the chaos test reads this one next, because the same reasoning applies.

**Homed 2026-08-03 (bookkeeping audit).** Being recorded here was not the same as being
owned: this spec's Files to Modify covers `internal/chaos/` alone and no AC of it reaches
the web suite, so the sibling would have died at closure. It now carries a live row in
`plan/deferrals/fixit-chaos-reconnect-load-sensitive.md`, homed at
`plan/spec-fixit-migrate-sleeps-infra.md`, whose subject is exactly this: replace a blind
wait with a wait on a real condition. Its predecessor already converted `rbac-web`, so the
web suite is inside that family's remit.

Re-checked at source on 2026-08-03, not carried from the paragraph above.
`test/web/commit-flow.wb` still carries `option=timeout:value=45s` and two blind
`action=wait:ms=1000` steps. This is also a DIFFERENT failure from the commit-flow entry
`plan/known-failures/RESOLVED.md` closes on 2026-07-29: that one was positive expectations
sampled once against an asynchronous page, in `checkElement` and `checkHTML`. This one was
measured the day after and its mechanism is the elapsed time the test allows itself.

Attribution, established the same day. The web suite has no dependency on anything the IKE
work changed. Adding `ipsec` to `all_suites` did not cause it either. `run_suite` is
invoked sequentially in `mk/test-functional.mk`, with no backgrounded call. The change
therefore adds total runtime and no concurrency.

## Required Reading

| Document | Why |
|----------|-----|
| `ai/rules/completion.md` | Load is never an explanation, and a shard that blames it is refused |
| `ai/rules/testing.md` | `scripts/dev/stress-repro.py` reproduces this cheaply |
| `internal/chaos/inprocess/runner.go` | The advance loop and the `DisconnectAt` precedent at `:55-61` |
| `internal/chaos/inprocess/chaos.go` | The scheduler loop and the guard call |

## The precedent to follow

`DisconnectAt` in the same package was made condition-gated for this exact class of
problem, and its doc comment states the principle (`internal/chaos/inprocess/runner.go`):

<!-- ste: ignore -->
> It is an earliest bound, not an instant: disconnecting a session that has not come up yet exercises a scenario nobody wrote, and on a slow host a fixed instant lands there.

Whatever the mechanism turns out to be, the fix follows that shape: wait for the condition,
and never lengthen the run. "Generous is a synonym for unknown".

## Current Behavior (MANDATORY)

Source files read on 2026-07-30:

- [ ] `internal/chaos/inprocess/runner.go`
- [ ] `internal/chaos/inprocess/runner_test.go`
- [ ] `internal/chaos/inprocess/chaos.go`

`chaosEnabled` comes from `cfg.ChaosRate > 0` (`runner.go`). When it is set, both
`DisconnectAt` branches disable themselves through their own `!chaosEnabled` guard
(`runner.go`, `:578`), so a chaos run uses none of the condition-gated machinery. The
chaos scheduler runs on ticks alone (`chaos.go`).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`Run` (`internal/chaos/inprocess/runner.go`), called from `runner_test.go`. Format at
entry is a `RunConfig`.

### Transformation Path

The advance loop drives virtual time and feeds `chaosTick`. `chaosSchedulerLoop` reads a
tick and calls `sched.Tick(now, es.Snapshot())` (`chaos.go`). Each surviving action
passes `guard.AllowChaos` (`chaos.go`) and is sent non-blocking to the peer's channel
(`chaos.go`). Lifecycle events land in `result.Events`, which the test counts after
`Run` returns.

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| advance loop -> chaos scheduler | out | A virtual tick |
| chaos scheduler -> peer channel | out | A chaos action, dropped when the peer is busy |
| reactor -> result | in | The lifecycle events the assertion counts |

### Integration Points

None outside `internal/chaos/inprocess`. No other suite reads these events.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| `Run` with `ChaosRate` 1.0 (`runner_test.go`) | -> | whichever gate the diagnosis names | `TestInProcessChaosReconnect` under stress |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| `TestInProcessChaosReconnect` under `stress-repro.py` | AC-2 |
| A case driving a chaos event at a re-connecting session | AC-4 |

### Functional Tests

None. This is a test-harness defect with no user-facing surface, so no `.ci` test applies.
The chaos suite is its own regression net.

## Files to Modify

| File | Change |
|------|--------|
| `internal/chaos/inprocess/chaos.go` | The gate, once the diagnosis names it |
| `internal/chaos/inprocess/runner_test.go` | Only if the assertion itself must wait on state |

## Implementation Steps

1. Reproduce under load with `scripts/dev/stress-repro.py`, and record the failure.
2. Read the two candidates above and refute one in writing.
3. Apply the condition gate the diagnosis names.
4. Re-run under the same pressure, and confirm it holds.

## Goal Gates

`make ze-verify`.

## Quality Gates

`make ze-lint-changed`, and `scripts/dev/stress-repro.py` for the load proof.

## RFC Documentation (Scope: testing)

None. No RFC obligation is involved.

## The fix

`cfg.Duration` becomes an earliest bound for teardown, in the same sense as
`RunConfig.DisconnectAt`. The advance loop is unchanged. After it, the background clock
advancer starts FIRST, because a handshake cannot finish while virtual time stands still.
`waitSettled` (`internal/chaos/inprocess/chaos.go`) then holds the teardown on observed
state, and ctx bounds a peer that never returns.

The run owes ONE observable recovery to each peer chaos knocked down. It does not owe a
healthy peer at the final instant. At chaos rate 1.0 the peer is usually down when the
window closes. An earlier gate demanded a healthy peer, and it held for the full 90 second
ctx in 4 of 80 runs.

| Change | File |
|--------|------|
| `waitSettled` plus `chaosProgress`, which counts dispatched actions against applied ones and latches the fall and the recovery | `internal/chaos/inprocess/chaos.go` |
| Advancer hoisted above the settle wait, gate composed at the call site, `WentDown` and `Recovered` latched from the drain | `internal/chaos/inprocess/runner.go` |
| `SimulatorConfig.OnSessionEnd`, called at the decision and before the reader drain | `internal/chaos/peer/simulator.go` |
| Send and close of the callback channels serialized under `sendMu` | `pkg/plugin/rpc/bridge.go` |

## A product data race, found by the fix

`ai/rules/completion.md` predicts that a real wait exposes a genuine race. It did.

`DirectBridge.SendCallback` sends on `callbackCh` while `CloseCallbacks` closes it
(`pkg/plugin/rpc/bridge.go`). The author knew the send CAN panic and added a `recover`, but
`recover` does not make the pair safe. A send concurrent with a close is a data race, and
`-race` failed the test that reached it. Reactor shutdown reached it through
`Process.Stop` while a peer was in `broadcastValidateOpen`.

`CloseCallbacks` now takes `sendMu` for writing and both senders take it for reading, so a
send can never overlap the close. The close still signals the SDK event loop to exit, so
the contract at `pkg/plugin/sdk/sdk_dispatch.go` is unchanged.

## Evidence

Go unit tests are outside `ze-test`, so `scripts/dev/stress-repro.py` cannot drive this one.
The same technique drove it: 64 CPU and GC burners oversubscribing 32 cores, with 10
concurrent copies of the race-instrumented test binary.

The `-race` binary matters. Its isolated time is 6.46 seconds, which is the 6.30 seconds
the original report recorded, so the failing run was the race pass.

| Run | Result |
|-----|--------|
| Isolated, before | PASS, 4.13s plain and 6.46s under `-race` |
| Under load, before | **6 of 30 FAILED**, verbatim `"1" is not greater than "1"` |
| Under load, after | **0 of 100 failed**, then **0 of 60 failed** on the final binary |
| Package, after | `internal/chaos/...` and `pkg/plugin/...` green under `-race` |
| Isolated, after | All 13 `TestInProcess*` pass, target at 6.38s against the 6.46s baseline |

## Acceptance Criteria

| Id | Criterion | Evidence |
|----|-----------|----------|
| AC-1 | The mechanism is named with a `file:line`, and the losing candidate above is refuted in writing | "The mechanism" above names `runner.go` and `runner.go`. Both candidates are refuted against `guard.go`, `scheduler.go` and `simulator.go` |
| AC-2 | `TestInProcessChaosReconnect` passes under stress pressure | 0 of 100, then 0 of 60, against 6 of 30 before |
| AC-3 | `Duration` is unchanged and no sleep is lengthened | `git diff internal/chaos/inprocess/runner_test.go` is empty. No `time.Sleep` value changed |
| AC-4 | Chaos stays adversarial mid-run, so a half-open session is still reachable | The scheduler, the guard, the stale snapshot and the drop path are untouched. The gate runs after the advance loop, which is the only feeder of `chaosTick`, so no chaos is scheduled during it |

## Checklist

- [ ] Tests written first, reproduced under load, and the Tests FAIL output recorded
- [ ] Mechanism named, and both other candidates refuted
- [ ] Tests PASS recorded under the same pressure
- [ ] `make ze-verify` green

---

## Implementation Summary

### What Was Implemented

`cfg.Duration` became an earliest bound for teardown, in the same sense as
`RunConfig.DisconnectAt`. The advance loop is unchanged and no sleep was lengthened.

- `waitSettled`, `chaosProgress` and `endsSession` (`internal/chaos/inprocess/chaos.go`).
  `chaosProgress` counts dispatched actions against applied ones, and latches the fall
  and the recovery for each peer. `endsSession` names the five actions that are certain
  to end a session. `ActionConnectionCollision` is absent from that set on purpose,
  because RFC 4271 Section 6.8 permits the reactor to keep the session.
- The background clock advancer was hoisted above the settle wait, and the gate is
  composed at the call site in `Run` (`internal/chaos/inprocess/runner.go`). A handshake
  cannot finish while virtual time stands still, so the advancer must start first.
- `SimulatorConfig.OnSessionEnd` (`internal/chaos/peer/simulator.go`), called at the
  moment the simulator decides the session is over and before the reader drain.
- `DirectBridge.sendMu`, `beginSend` and `endSend` (`pkg/plugin/rpc/bridge.go`) serialize
  the callback sends against `CloseCallbacks`.

One commit carries all four files: `60eaeb60e`.

### Bugs Found/Fixed

- **A product data race.** `DirectBridge.SendCallback` sent on `callbackCh` while
  `CloseCallbacks` closed it (`pkg/plugin/rpc/bridge.go`). The author knew the send can
  panic and added a `recover`, but `recover` does not make the pair safe. Reactor shutdown
  reached it through `Process.Stop` while a peer was in `broadcastValidateOpen`. Covered
  by `TestInProcessChaosReconnect` under `-race`, which failed on it before the fix.

### Documentation Updates

None. The change is internal to the chaos harness and the plugin bridge, and it adds no
config surface, no CLI surface and no wire behavior. No `docs/` file carries a source
anchor to any of the four files:

```
$ grep -rn "chaos/inprocess\|chaos/peer/simulator\|plugin/rpc/bridge" docs/
(no match)
```

### Deviations from Plan

- **Files to Modify named `internal/chaos/inprocess/chaos.go` alone for the gate.** The
  fix also needed `runner.go` for the call site and the advancer order, `simulator.go` for
  the down edge, and `bridge.go` for the race the real wait exposed. The spec's own "The
  fix" section records all four, so the deviation is against the earlier table only.
- **`runner_test.go` was NOT modified.** The plan allowed it "only if the assertion itself
  must wait on state". It did not, so AC-3 holds with an empty diff.
- **The implementation was committed inside `60eaeb60e`, whose subject is IKE work.** The
  chaos fix had no commit of its own. See the Mistake Log.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A later session read the spec's Task section, concluded the 1 second `ChaosInterval` was the defect, and lengthened it to 15 seconds | The interval was never the cause. AC-3 forbids that edit in writing, and the fix had already landed. At loadavg 29.5 the shipped 1 second interval passed 3 of 3 | The edit was measured against its own hypothesis: the old value passed under the same load that was supposed to break it | Reverted. `git diff internal/chaos/inprocess/runner_test.go` is empty |
| escalation | The implementation entered git inside `60eaeb60e`, a commit whose subject is ten RFC 7296 violations | Single-focus commits are required (`ai/rules/git-safety.md`, "Commit Granularity"). Four chaos and bridge files rode an IKE commit | Found at closure, looking for the commit that introduced `waitSettled` | Recorded here. History rewriting is forbidden and the code is correct, so nothing is undone |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The test must not fail on a loaded host | Done | `waitSettled` at the call site in `Run` (`internal/chaos/inprocess/runner.go`) | 0 of 100 then 0 of 60 under load at implementation, 7 of 7 re-measured 2026-08-05 |
| Load must be treated as the bug, not the excuse | Done | The gate waits on observed state, never on a duration | `Duration` unchanged, no sleep lengthened |
| Follow the `DisconnectAt` precedent: wait for the condition, never lengthen the run | Done | `waitSettled` (`internal/chaos/inprocess/chaos.go`) | The doc comment states the same earliest-bound principle |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | "The mechanism (resolved 2026-07-31)" names the advance loop and `simCancel()` in `internal/chaos/inprocess/runner.go` | Both losing candidates refuted in writing against `guard.AllowChaos`, `Scheduler.Tick` and `internal/chaos/peer/simulator.go` |
| AC-2 | Done | `TestInProcessChaosReconnect` under load | 0 of 100 and 0 of 60 at implementation. Re-measured 2026-08-05: 7 of 7, loadavg 9.7 to 29.5, including 3 of 3 under `-race` |
| AC-3 | Done | `git diff internal/chaos/inprocess/runner_test.go` is empty | Verified 2026-08-05 after reverting the interval edit in the Mistake Log |
| AC-4 | Done | `chaosSchedulerLoop` (`internal/chaos/inprocess/chaos.go`) | The scheduler, the guard, the stale snapshot and the drop branch are byte-identical. The success branch gained one `Dispatched` call and nothing else. The gate runs after the advance loop, which is the only feeder of `chaosTick`, so no chaos is scheduled during the wait |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestInProcessChaosReconnect` under `stress-repro.py` | Changed | `internal/chaos/inprocess/runner_test.go` | Go unit tests sit outside `ze-test`, so `stress-repro.py` cannot drive this one. The spec's Evidence section records the substitute: CPU and GC burners oversubscribing the cores, with concurrent copies of the race binary |
| A case driving a chaos event at a re-connecting session | Changed | `TestInProcessChaosReconnect` | No dedicated case was added. At `ChaosRate` 1.0 with a 1 second interval over 60 virtual seconds, the existing test already offers chaos at sessions that are coming back. AC-4 rests on the untouched scheduler path rather than on a new case, and that is a structural proof a diff can check |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/chaos/inprocess/chaos.go` | Done | `waitSettled`, `chaosProgress`, `endsSession` |
| `internal/chaos/inprocess/runner_test.go` | Done, unmodified | The plan gated this on "only if the assertion itself must wait on state". It did not |
| `internal/chaos/inprocess/runner.go` | Changed | Not in the plan's table. Holds the call site and the advancer order |
| `internal/chaos/peer/simulator.go` | Changed | Not in the plan's table. Holds `OnSessionEnd` |
| `pkg/plugin/rpc/bridge.go` | Changed | Not in the plan's table. Holds the race fix the real wait exposed |

### Audit Summary
- **Total items:** 14
- **Done:** 10
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (all recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `TestInProcessChaosReconnect` passes on a loaded host | chaos test under load | At implementation: 6 of 30 failed before, 0 of 100 then 0 of 60 after. Re-measured 2026-08-05 by a later session on the shipped config: `-count=1` at loadavg 11.95 PASS 4.411s; `-count=3` at loadavg 29.5 PASS 3 of 3; `-race -count=3` PASS 3 of 3 in 20.881s |
| The run stops on state, not on elapsed time | code | `waitSettled` takes a `settled func() bool` and polls it (`internal/chaos/inprocess/chaos.go`). It carries no deadline of its own; `ctx` bounds a peer that never returns |
| Chaos stays adversarial, so the fix buys no green by removing pressure | diff | `ChaosRate` 1.0 and `ChaosInterval` 1s are unchanged, and the drop branch of `chaosSchedulerLoop` is untouched. The discriminator was run: with the shipped 1 second interval the test passes under the load that broke it before, so the pass is not bought by spacing the disconnects out |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `test/web/commit-flow.wb` fails under load for the same reason: it asserts on elapsed time instead of on state, carrying `option=timeout:value=45s` and two blind `action=wait:ms=1000` steps | deferred | Homed at `plan/spec-fixit-migrate-sleeps-infra.md`, which exists and whose subject is replacing a blind wait with a wait on a real condition. The row stays live, so `plan/deferrals/fixit-chaos-reconnect-load-sensitive.md` is NOT removed by this closure |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-chaos-reconnect-load-sensitive-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md` |
| `review_gate.py check` | clean |
| Reviewer lenses used | concurrency and lock ordering, latch ordering against the event stream, gate vacuity, comment accuracy |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER and no ISSUE survived the pass | - | - |

Three NOTE findings, recorded and not blocking:

1. `chaosProgress.WentDown` is gated on `perturbed[idx]`, and `perturbed` is set only by
   `Executed`, which the drain drives from `EventChaosExecuted`. `RunSimulator` calls
   `endSession()` BEFORE it emits that event (`internal/chaos/peer/simulator.go`), so the
   `OnSessionEnd` callback's `WentDown` call is a no-op on a peer's FIRST chaos-caused
   fall. The latch is set moments later by `EventDisconnected` in the drain
   (`internal/chaos/inprocess/runner.go`). The settle verdict is unaffected, because
   `es.Set(idx, false)` in the same callback is immediate and `AwaitingFirstFall` reads
   `es.Up(i)`. A reader who expects both writes in that callback to take effect at once
   will be surprised.
2. `beginSend` holds `sendMu` for reading while it blocks on a full channel, and a pending
   `CloseCallbacks` writer then blocks every later `RLock` (`pkg/plugin/rpc/bridge.go`).
   The wait is bounded by the caller's `ctx` in both senders, so this is a delay and not a
   deadlock. The doc comment on `beginSend` already states the reasoning.
3. `chaosProgress.Owed` returns true from `expectFall` alone, so a dispatched
   session-ending action whose fall never arrives holds the gate until `ctx` ends. That is
   the product regression case, and the call-site comment says so.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/chaos/inprocess/chaos.go` | yes | `grep -n "func waitSettled"` gives `69:func waitSettled(ctx context.Context, settle bool, settled func() bool, poll time.Duration) {` |
| `internal/chaos/peer/simulator.go` | yes | `grep -n OnSessionEnd` gives `109: OnSessionEnd func()`, plus call sites at `139` and `143` |
| `pkg/plugin/rpc/bridge.go` | yes | `grep -n sendMu` gives `78`, `79`, `182`, `186`, `199`, `201`, `207` |
| `plan/spec-fixit-migrate-sleeps-infra.md` | yes | `ls -la` gives 42965 bytes, the destination of the live deferral row |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The mechanism is named and the losing candidates are refuted | The spec section "The mechanism (resolved 2026-07-31)" names `runner.go` and its `simCancel()`, and the refutation table cites `guard.AllowChaos`, `Scheduler.Tick` and `internal/chaos/peer/simulator.go` |
| AC-2 | Passes under stress pressure | Re-run 2026-08-05 on the shipped config: `-race -count=3` gives `ok github.com/ze-software/ze/internal/chaos/inprocess 20.881s`, 3 of 3 |
| AC-3 | `Duration` unchanged and no sleep lengthened | `git diff --stat internal/chaos/inprocess/runner_test.go` prints nothing |
| AC-4 | Chaos stays adversarial | `git show 60eaeb60e -- internal/chaos/inprocess/chaos.go` adds exactly one line inside `chaosSchedulerLoop`, the `inFlight.Dispatched` call on the successful-send branch. The drop branch and `g.AllowChaos` are unchanged |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `Run` with `ChaosRate` 1.0 (`internal/chaos/inprocess/runner_test.go`) | none; the spec's TDD plan states no `.ci` applies to a test-harness defect | yes. The entry point is `TestInProcessChaosReconnect`, which calls `Run`, which reaches `waitSettled` under `chaosEnabled && cfg.StopKeepalivesAt == 0`. Removing the gate returns the original failure, which is how 6 of 30 was measured |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 (implicit): the failure is load, not the IKE pilot that surfaced it | confirmed | `go list -deps ./internal/chaos/inprocess/` returns zero packages under `internal/component/ike` |
| A-2 (implicit): a real wait exposes a genuine race | confirmed | It did. `DirectBridge.SendCallback` against `CloseCallbacks` (`pkg/plugin/rpc/bridge.go`), failed under `-race` before `sendMu` |
| A-3 (implicit): the 1 second `ChaosInterval` is the defect | broken | Refuted 2026-08-05. At loadavg 29.5 the shipped 1 second interval passes 3 of 3. Recorded in the Mistake Log and in Deviations |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No `docs/` page describes the chaos harness internals or the plugin bridge send path | `grep -rn "chaos/inprocess\|chaos/peer/simulator\|plugin/rpc/bridge" docs/` returns no match | yes |
| No RFC status row changes | The change adds no protocol behavior. `ActionConnectionCollision` is EXCLUDED from `endsSession` because RFC 4271 Section 6.8 permits the reactor to keep the session, which preserves existing behavior rather than adding any | yes |

## Core Insight

**A test that reads a counter after a run has already chosen its clock.** The chaos runner
ends on VIRTUAL time. The reconnect it provokes needs REAL time. The assertion therefore
counted an event the run had made unreachable. The fix is not a longer window and not a
slower chaos rate. It is to make the end of the run a condition rather than an instant,
exactly as `RunConfig.DisconnectAt` already was for the start of a disconnect.

The general shape: three states hide a perturbation that no peer state shows.

- One action sits on a channel and has not reached the simulator.
- One runs inside the simulator. `ActionConnectionCollision` holds it for 500 real
  milliseconds and reports the peer healthy throughout.
- One waits on a hold timer that expires 20 virtual seconds later.

A gate built on peer state alone reads a healthy peer through all three. Counting
dispatched actions against applied ones makes that work visible. Latching the fall and the
recovery stops the gate demanding a healthy peer the run never promised.
