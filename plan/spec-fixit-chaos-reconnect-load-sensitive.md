# Spec: fixit-chaos-reconnect-load-sensitive

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | testing |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `plan/deferrals/fixit-chaos-reconnect-load-sensitive.md` |
| Updated | 2026-07-31 |

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
