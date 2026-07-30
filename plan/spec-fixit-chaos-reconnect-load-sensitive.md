# Spec: fixit-chaos-reconnect-load-sensitive

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | testing |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-chaos-reconnect-load-sensitive.md` |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 by a `make ze-verify-changed` run for
`plan/spec-rfcgate-1b-rfc7296-pilot.md`. Owner decision the same day: land the pilot work
under an override, and give this defect its own spec rather than a load excuse.

Status is `skeleton` on purpose. The failure is reproduced and attributed, and the
mechanism is NOT fully diagnosed. The section "What is still open" states the remainder.
The next session then inherits a record rather than a guess dressed as a finding.

## Task

**`TestInProcessChaosReconnect` fails on a loaded host and passes on a quiet one.**
`ai/rules/fix-dont-record.md` names that shape and forbids recording it. A test that
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
over a fixed `Duration` of 60 seconds (`internal/chaos/inprocess/runner_test.go:666-680`).
Chaos therefore offers a disconnect every second for the whole run. Real TCP on loopback
and real goroutine scheduling sit under the virtual clock. On a loaded host a
re-establishment takes longer, while the disconnects keep arriving at the same rate. The
count of `EventEstablished` is read only after `Run` returns
(`runner_test.go:683-696`), so nothing waits for the second one.

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
`ai/rules/bash-output.md` names that trap.

## What is still open

**The scheduler already sees session state, so the obvious fix is already in place.**
`chaosSchedulerLoop` calls `sched.Tick(now, es.Snapshot())`
(`internal/chaos/inprocess/chaos.go:98`), and `establishedState` is built whenever chaos is
enabled (`internal/chaos/inprocess/runner.go:260`). A first guess that chaos fires blindly
at a not-yet-established session is therefore WRONG, and it was discarded before this spec
was written.

Two candidate mechanisms remain, and the next session must decide between them by reading
rather than by guessing:

| Candidate | What to read |
|-----------|--------------|
| `guard.AllowChaos` permits a new disconnect before the previous re-establishment finished | `internal/chaos/inprocess/chaos.go:100` and the `guard` package |
| The scheduler's established snapshot is stale by a tick, so it targets a peer that has just gone down | `engine.Scheduler.Tick`, and how `establishedState` is updated |

The non-blocking send at `internal/chaos/inprocess/chaos.go:103-109` drops an action when a
peer is busy. Understand that before any change. It already sheds load, and it CAN be the
reason the test usually passes.

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
| `ai/rules/fix-dont-record.md` | Load is never an explanation, and a shard that blames it is refused |
| `ai/rules/flaky-under-load.md` | `scripts/dev/stress-repro.py` reproduces this cheaply |
| `internal/chaos/inprocess/runner.go` | The advance loop and the `DisconnectAt` precedent at `:55-61` |
| `internal/chaos/inprocess/chaos.go` | The scheduler loop and the guard call |

## The precedent to follow

`DisconnectAt` in the same package was made condition-gated for this exact class of
problem, and its doc comment states the principle (`internal/chaos/inprocess/runner.go:55-61`):

<!-- ste: ignore -->
> It is an earliest bound, not an instant: disconnecting a session that has not come up yet exercises a scenario nobody wrote, and on a slow host a fixed instant lands there.

Whatever the mechanism turns out to be, the fix follows that shape: wait for the condition,
and never lengthen the run. "Generous is a synonym for unknown".

## Current Behavior (MANDATORY)

Source files read on 2026-07-30:

- [ ] `internal/chaos/inprocess/runner.go`
- [ ] `internal/chaos/inprocess/runner_test.go`
- [ ] `internal/chaos/inprocess/chaos.go`

`chaosEnabled` comes from `cfg.ChaosRate > 0` (`runner.go:251`). When it is set, both
`DisconnectAt` branches disable themselves through their own `!chaosEnabled` guard
(`runner.go:507`, `:578`), so a chaos run uses none of the condition-gated machinery. The
chaos scheduler runs on ticks alone (`chaos.go:92-113`).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point

`Run` (`internal/chaos/inprocess/runner.go`), called from `runner_test.go:670`. Format at
entry is a `RunConfig`.

### Transformation Path

The advance loop drives virtual time and feeds `chaosTick`. `chaosSchedulerLoop` reads a
tick and calls `sched.Tick(now, es.Snapshot())` (`chaos.go:98`). Each surviving action
passes `guard.AllowChaos` (`chaos.go:100`) and is sent non-blocking to the peer's channel
(`chaos.go:103-109`). Lifecycle events land in `result.Events`, which the test counts after
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
| `Run` with `ChaosRate` 1.0 (`runner_test.go:670`) | -> | whichever gate the diagnosis names | `TestInProcessChaosReconnect` under stress |

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

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | The mechanism is named with a `file:line`, and the losing candidate above is refuted in writing |
| AC-2 | `TestInProcessChaosReconnect` passes under `scripts/dev/stress-repro.py` pressure |
| AC-3 | `Duration` is unchanged and no sleep is lengthened |
| AC-4 | Chaos stays adversarial mid-run, so a half-open session is still reachable |

## Checklist

- [ ] Tests written first, reproduced under load, and the Tests FAIL output recorded
- [ ] Mechanism named, and the other candidate refuted
- [ ] Tests PASS recorded under the same pressure
- [ ] `make ze-verify` green
