# Spec: fixit-fwdpool-backpressure-timing -- a backpressure test that waits on a duration instead of a condition

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`TestFwdPool_BackpressureBehavior`
(`internal/component/bgp/reactor/forward_pool_test.go`) fails under
`make ze-unit-reactor-test-race` at `-count=20`. It passes 5 runs out of 5 in isolation.
`-race` reports zero data races.

**This is a BROKEN TEST, and it must be FIXED. It must NOT go into
`plan/known-failures/`.** `ai/rules/completion.md` is explicit: a test that
passes on a quiet host and fails on a busy one waits on a duration instead of on
a condition, and "passes in isolation" is a banned excuse. It reproduces, so
there is no recording path for it. The load is not the explanation, it is the
symptom.

### The mechanism, measured on 2026-08-19

This section replaces the 2026-08-02 reading, which named the wrong wait and the
wrong root cause. Both corrections came from measurement, not from re-reading.

The test builds a pool with `chanSize: 2` and a handler that blocks on a channel.
It then does this:

| Step | What the test does | What it assumes | Verdict |
|------|--------------------|-----------------|---------|
| 1 | Three `Dispatch` calls, with a receive on `handlerStarted` after the first | that the steady state is one item in the handler plus two in the channel | SOUND. `runWorker` calls `drainBatch` before `safeBatchHandle` (`internal/component/bgp/reactor/forward_pool.go`), so the channel is empty when `handlerStarted` fires and the two later dispatches fill it deterministically |
| 2 | A fourth `Dispatch` in a goroutine | that this one must block | sound |
| 3 | `require.Never(..., 100*time.Millisecond, 10*time.Millisecond)` | that 100 milliseconds of not-completing PROVES blocking | VACUOUS, not the flake. It also passes when the goroutine has not started, and it passes with backpressure removed from `Dispatch` |
| 4 | Close the blocker, then `require.Eventually(..., 2*time.Second, ...)` | that the fourth dispatch completes once the handler drains | THIS is where the failure lands |

**The failure is on step 4.** Measured race-instrumented at `GOMAXPROCS=8` with
48 CPU burners on 32 cores: 1 failure in 400 runs, 1 in 600. Durations are
bimodal, 599 runs at 0.10-0.12s against a single run at 2.10s, which is the
2-second `require.Eventually` budget expiring. The blocked dispatch simply is
not rescheduled inside 2 seconds on an oversubscribed host, so a wait that
carries a deadline fails while the code behaves correctly.

So the reason to fix this is not the timeout figure. It is step 3: an assertion
that a thing has not happened yet is not an assertion that it cannot happen, and
that one passes with the behavior it names deleted.

### The fix shape

Wait on the blocked condition rather than on a clock, at both waits.
`fwdWorker.pending` already carries it: `fwdPool.Dispatch` increments it before
the blocking channel send and decrements it after, so a full `w.ch` plus
`pending == 1` means one dispatcher is parked inside that send.
`TestFwdPool_StopUnblocksDispatch`, in the same file, already reads it that way.
No observable is added and `forward_pool.go` is not modified.

The completion wait becomes a receive on the dispatch goroutine's own result
channel, which is the event itself and carries no deadline.

The same rewrite applies to the twin test `TestForwardPoolBackpressurePropagation`
in the same file, which carries the identical `require.Never` plus 2-second
`require.Eventually` pair (`ai/rules/completion.md`: the unit fixed is the
problem, not the file that was opened).

The assertion the test exists to make is unchanged and must survive: a dispatch
into a full channel BLOCKS and no item is dropped. Weakening that to reach green
is banned (`ai/rules/testing.md`).

Source: `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, row 7.

## Required Reading

- [ ] `ai/rules/completion.md` - load is never an explanation, it is the bug
  → Constraint: no `plan/known-failures/` shard. The failure reproduces, so it gets fixed.
- [ ] `ai/rules/testing.md` - reproduce a flake under a stress reproducer, never a loop over the full suite
  → Constraint: `scripts/dev/stress-repro.py` drives a FUNCTIONAL suite by name and cannot take a Go package, so the reproducer for a Go unit test is a race-built package binary looped under CPU burners. Do not hunt with repeated verify runs.
- [ ] `ai/rules/testing.md` - the assertion is the requirement
  → Constraint: the backpressure claim survives the rewrite unchanged.
- [ ] `ai/rules/testing.md` - "Reactor Concurrency Code"
  → Constraint: `make ze-unit-reactor-test-race` is the required verification for anything in this package that shares state across goroutines.

**Key insights:**
- Two duration-shaped steps, not one. Fixing only `require.Never` may leave the fill sequence racy.
- `-race` clean means there is no data race. It says nothing about a timing assumption, which is what this is.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_pool_test.go` - `TestFwdPool_BackpressureBehavior` at the `require.Never` / `require.Eventually` pair; VALIDATES AC-10 (backpressure on full channel), PREVENTS silent message drops under load
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `newFwdPool`, `Dispatch`, and the per-peer worker loop; read the worker's take path before assuming channel occupancy

**Behavior to preserve:** the test's subject. A `Dispatch` into a full per-peer channel must block, and must not drop the item. The `VALIDATES:` and `PREVENTS:` annotations stay accurate. No production backpressure behavior changes.

**Behavior to change:** how the test waits. Durations become conditions, read from `fwdWorker.pending`, which the pool already keeps. No observable is added.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`pool.Dispatch(key, item)` called from the reactor's forward path, against a per-peer channel of fixed size.

### Transformation Path
1. `Dispatch` sends the item to the per-peer channel.
2. A per-peer worker goroutine takes from that channel and calls the handler.
3. When the channel is full, the send in `Dispatch` blocks until the worker takes.
4. The test drives this by blocking the handler, so the worker cannot take, so the channel fills.
5. The test then asserts that a further `Dispatch` does not complete.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Caller ↔ per-peer channel | a buffered channel send that blocks when full | Yes, test read on 2026-08-02 |
| Channel ↔ worker goroutine | a take, possibly batched | No, confirm at design time; this is the crux |
| Test ↔ pool state | `len(w.ch)` under `pool.mu`, plus `fwdWorker.pending`, which `Dispatch` raises before the send and lowers after it | Yes, read on 2026-08-19. There is no product gap |

### Integration Points
- `internal/component/bgp/reactor/forward_pool.go` - the pool. Read, not modified.
- `make _ze-unit-pkg-test-impl PKG=./internal/component/bgp/reactor` - the scoped target. `make ze-unit-reactor-test-race` runs the same package under `-race` for the whole suite.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | fill during design: any new observable is read through the pool's own API, not by reaching into its fields from the test |
| No unintended coupling | No | fill during design |
| No duplicated functionality | No | fill during design: check for an existing depth or length accessor before adding one |
| Zero-copy preserved where applicable | N-A | no wire path touched |
| Registration over hardcoding (`ai/rules/plugins.md`) | N-A | no command, view, family, or handler added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The failure is a timing assumption in the test, not a defect in `Dispatch`. | `-race` is clean and the test passes in isolation, which points at the assertion rather than at the code. | A real backpressure defect exists and is in scope here (`ai/rules/completion.md`). | reproduce under CPU oversubscription, then read the worker's take path | confirmed |
| A-2 | The worker takes a batch, so channel occupancy after three dispatches is not deterministically two. | The handler signature receives a slice of items, which suggests batching. NOT yet confirmed against the take path. | Step 1 is sound and only `require.Never` needs replacing. | read `forward_pool.go`'s worker loop | BROKEN |
| A-3 | No observable for full or drained exists on the pool today. | The test uses durations, which is what an author does when no signal is available. | Use the existing one. Do not add a second. | grep the pool's exported and package-level accessors | BROKEN |

**A-2 is broken.** `fwdPool.runWorker` calls `drainBatch` and only then
`safeBatchHandle` (`internal/component/bgp/reactor/forward_pool.go`). The drain
therefore completes before the handler signals `handlerStarted`, and it drains a
channel the test has not yet written to. The fill is deterministic, step 1 is
sound, and the failure is not there. Measured: the failing run spends 2.10s at
the step-4 `require.Eventually`, not at the fill.

**A-3 is broken.** `fwdWorker.pending` already exists
(`internal/component/bgp/reactor/forward_pool.go`, the `fwdWorker` struct),
`fwdPool.Dispatch` increments it before the blocking send and decrements it
after, and `TestFwdPool_StopUnblocksDispatch` in the same test file already uses
it to prove a dispatch is parked. There is no product gap and no new observable:
`forward_pool.go` is not modified by this spec.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fix replaces one duration with a longer one and the test reds again later on a busier host. | A rewrite that still names a millisecond figure as the proof. | A duration may bound a poll, never carry the assertion. The assertion reads a condition. |
| R-2 | The test is quietly relaxed to assert something weaker that always passes. | The `PREVENTS:` line stops matching what the test checks. | The backpressure claim is the requirement (`ai/rules/testing.md`). |
| R-3 | A new observable exists only for the test and is dead in production. | An accessor with one caller, and that caller is a `_test.go` file. | A test-only observable is acceptable when it reads state the pool already keeps. Adding new STATE for a test is not. Record which it is. |
| R-4 | The failure is chased by looping `make ze-precommit-verify`. | Repeated full-suite runs in the session log. | `ai/rules/testing.md`: loop a race-built binary of the suspected package under CPU burners instead. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | If A-1 is wrong and the defect is in `Dispatch`, forwarded UPDATEs are dropped silently under load. That is the exact failure the test's `PREVENTS:` line names. |
| How is it reverted? | Single commit revert. |
| Who else touches this path? | Any session working the reactor forward pool or its per-peer workers. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| The reactor dispatches into a per-peer channel that is already full | → | `Dispatch` blocks until the worker takes, dropping nothing | `TestFwdPool_BackpressureBehavior`, rewritten to wait on the drained condition (existing test, no new feature) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make _ze-unit-pkg-test-impl PKG=./internal/component/bgp/reactor RUN='TestFwdPool\|TestForwardPool'` | `TestFwdPool_BackpressureBehavior` and `TestForwardPoolBackpressurePropagation` pass |
| AC-2 | A race-built package test binary run under CPU oversubscription: `go test -race -c` the reactor package, start CPU burners at 1.5x the core count, then at least 200 iterations of the two tests at `GOMAXPROCS=8`. `scripts/dev/stress-repro.py` does NOT apply: it takes a functional suite name, not a Go package | Every iteration passes. The pre-fix binary reproduced the failure at roughly 1 run in 400 under the same load |
| AC-3 | The rewritten tests | No assertion rests on an elapsed duration. A duration may bound a poll interval only, and carries a comment saying which condition the loop waits on |
| AC-4 | Backpressure removed from `Dispatch` (a deliberate mutation: a `default:` arm that drops instead of blocking) | Both tests FAIL. The pre-fix versions PASS under the same mutation, which is what makes the rewrite worth doing |
| AC-5 | `plan/known-failures/` | Contains no shard for this test, before or after |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFwdPool_BackpressureBehavior` | `internal/component/bgp/reactor/forward_pool_test.go` | AC-1, AC-3, AC-4: a full channel blocks the dispatcher and drops nothing | rewritten, green |
| `TestForwardPoolBackpressurePropagation` | `internal/component/bgp/reactor/forward_pool_test.go` | AC-1, AC-3, AC-4: the same claim at `chanSize: 4` | rewritten, green |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| not applicable, no user-facing behavior changes: this repairs a unit test's timing assumption. The driving surface is `make ze-unit-reactor-test-race` | `internal/component/bgp/reactor/` | a developer runs the reactor race target and it is green every iteration | |

## Files to Modify
- `internal/component/bgp/reactor/forward_pool_test.go` - replace both duration-shaped waits with condition waits, in `TestFwdPool_BackpressureBehavior` and in the twin `TestForwardPoolBackpressurePropagation`
- `internal/component/bgp/reactor/forward_pool.go` - NOT modified. A-3 is broken: `fwdWorker.pending` already carries the condition

## Implementation Steps

1. Read the worker take path and settle A-2 (broken: `drainBatch` runs before the handler signals, so the fill is deterministic).
2. Settle A-3 (broken: `fwdWorker.pending` is the observable, and `TestFwdPool_StopUnblocksDispatch` already reads it).
3. Rewrite both waits in both tests. Every remaining duration bounds a poll and carries a comment naming the condition.
4. Mutate `fwdPool.Dispatch`'s blocking send into a `select` with a dropping `default:`, confirm both tests red, revert the mutation (AC-4).
5. `make _ze-unit-pkg-test-impl PKG=./internal/component/bgp/reactor RUN='TestFwdPool|TestForwardPool'`, then the AC-2 load run over a race-built package binary.

### Twin defect fixed in the same pass

`TestFwdPool_BackpressureBehavior` released the handler with a bare
`close(blocker)` on the success path only, while `defer pool.Stop()` waits for
that worker. Any failed assertion therefore HUNG the whole package for the
20-minute `go test` timeout instead of reporting a red. The mutation run at step
4 hit it: the assertion failed in under a second and the binary then sat until it
was killed. The test now closes the blocker from a `defer` registered after the
`pool.Stop()` defer, guarded by `atomic.Bool.CompareAndSwap`, which is the idiom
the twin test already used.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Both duration-shaped steps replaced, not only `require.Never` |
| Correctness | The `VALIDATES:` and `PREVENTS:` annotations still describe what the test checks |
| Rule: `ai/rules/completion.md` | No shard was written, and no sentence blames host load |
| Rule: `ai/rules/testing.md` | The backpressure assertion is unchanged in strength |
| Registration over hardcoding | N-A |

## Known Limitations
- Two tests carried this shape and both are fixed here. The rest of
  `forward_pool_test.go` uses `require.Eventually` over a counter or a pool
  field, which is a condition wait already.
- The AC-2 load run proves the fix under oversubscription. It does not re-measure
  the pre-fix failure rate: that measurement (1 in 400 and 1 in 600,
  race-instrumented, `GOMAXPROCS=8`, 48 burners on 32 cores) comes from the audit
  that produced this spec's Task section.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] `make ze-precommit-verify` passes
- [ ] `make ze-unit-reactor-test-race` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste the mutated-behavior run from AC-4)
- [ ] Tests PASS (paste the loaded run)

### Evidence, 2026-08-19

The mutation is a `default:` arm in `fwdPool.Dispatch`'s send select that drops
the item and lowers `pending`. It was applied through a `go test -overlay` copy,
so `forward_pool.go` in the tree was never edited.

| Run | Test code | `Dispatch` | Result |
|-----|-----------|------------|--------|
| AC-4 before | pre-fix | mutated | `require.Never` reports "Condition satisfied", then the test HANGS in `defer pool.Stop()` until the 20-minute `go test` timeout |
| AC-4 before, slow-start | pre-fix plus a 150ms sleep in the fourth-dispatch goroutine | mutated | PASS in 1.33s. This is the vacuity: with the goroutine slow to start, the pre-fix test is green with backpressure deleted |
| AC-4 after | rewritten | mutated | FAIL in 1.00s, both tests, "Condition never satisfied", no hang |
| AC-4 after, slow-start | rewritten plus the same 150ms sleep | mutated | FAIL in 1.11s. The sleep no longer hides the drop |
| AC-1 | rewritten | unmodified | `make _ze-unit-pkg-test-impl PKG=./internal/component/bgp/reactor RUN='TestFwdPool\|TestForwardPool'` ok in 2.33s; the whole package ok in 155s |
| AC-2 | rewritten | unmodified | race-built package binary, 250 iterations at `GOMAXPROCS=8` under 48 CPU burners on 32 cores: 0 failures, slowest iteration 5.09s wall. The pre-fix `require.Eventually` budget was 2s, so that slowest iteration is inside the window the old test failed in |
| AC-5 | - | - | `plan/known-failures/` carries no shard for either test |

---

<!-- Closure sections. The three checklists below were absent from this spec and
     were authored at closure, from the evidence each step produced. -->

## Deliverables Checklist

| Deliverable | Verification method | Evidence |
|-------------|---------------------|----------|
| Both waits in `TestFwdPool_BackpressureBehavior` read a condition | read the test at each wait site | `internal/component/bgp/reactor/forward_pool_test.go`, `TestFwdPool_BackpressureBehavior`. The park wait polls `w.pending.Load() == 1` against a full `w.ch`. The completion wait is `ok2 := <-dispatched` and carries no deadline |
| Both waits in `TestForwardPoolBackpressurePropagation` read a condition | read the test at each wait site | same file, `TestForwardPoolBackpressurePropagation`. Same predicate, and `<-dispatched` for completion |
| A failed assertion reports instead of hanging the package | force the assertion red and time the run | the rewritten tests red in 1.00s against a mutated `Dispatch`. The pre-fix tests under the same mutation printed `Condition satisfied` and then sat until the test timeout fired |
| `forward_pool.go` is not modified | `git show --stat c08237575` | the commit carries no `forward_pool.go` path |

## Security Review Checklist

| Concern | Checked | Finding |
|---------|---------|---------|
| Untrusted input | the diff is test-only and every value is a literal the test builds | none |
| Allocation from a caller-controlled size | the diff adds no `make` call | none |
| Race or TOCTOU in the new predicate | the predicate reads `pool.workers` under `pool.mu` and reads `w.pending` through `atomic.Int32` | none. 860 race-instrumented executions reported no data race |
| Shipped surface reached | `forward_pool.go` is unmodified, so no production path changes | none |

## Documentation Update Checklist

| Category | Update needed | Evidence |
|----------|---------------|----------|
| feature list, user guide, config syntax, CLI reference, API/RPC, plugin SDK, wire format, comparison table | No | the diff changes no user-visible behavior, and `forward_pool.go` is unmodified |
| test infrastructure | No | no target, runner, or harness changed. `make ze-unit-reactor-test-race` is the same command it was |
| architecture design | No | `grep -rn forward_pool docs/ ai/` returns 61 anchors. Every one names `forward_pool.go`, `forward_pool_barrier.go`, `forward_pool_congestion.go`, `forward_pool_weight*.go`, or `forward_pool_property_test.go`. None names `forward_pool_test.go`, and none of the anchored files is in the diff |
| doctor checks | No | the diff adds no runtime dependency: no file path, socket, port, module, binary, or certificate |
| RFC status | No | neither test carries an `RFC requirement:` tag, and the diff adds none. `scripts/dev/audit-test-relaxation.py c08237575~1` reports 13 RFC-tagged findings and names no reactor forward-pool file |
| `docs/architecture/core-design.md`, the design document `forward_pool.go` declares in its `// Design:` header | No | `forward_pool.go` is not in the diff, so nothing that document describes changed. Its one anchor for this file, at `docs/architecture/core-design.md`, names `fwdBatchHandler` and `fwdWriteDeadlineDefault`, and neither symbol is touched |
| `.claude/rules/design-principles.md`, the second document that header declares | No | same reason. The diff changes no zero-copy or copy-on-modify behavior, because it changes no production file |

## Implementation Summary

### What Was Implemented
- `TestFwdPool_BackpressureBehavior` and `TestForwardPoolBackpressurePropagation` (`internal/component/bgp/reactor/forward_pool_test.go`) now prove the parked state instead of waiting out a clock. The park wait polls `len(w.ch) == cap(w.ch)` together with `w.pending.Load() == 1`, which `fwdPool.Dispatch` raises before its blocking send and lowers in both arms of the select after it. A non-blocking receive on `dispatched` follows, so the test also states that the parked dispatch has not returned.
- The completion wait is a bare receive on the dispatch goroutine's own channel. It carries no deadline, and that is the step both pre-fix failures landed on.
- `TestFwdPool_BackpressureBehavior` releases its handler from a `defer` registered after the `pool.Stop()` defer, guarded by `atomic.Bool.CompareAndSwap`. A failed assertion now reports in one second where it used to hang the package until the `go test` timeout.
- `forward_pool.go` is unmodified. `fwdWorker.pending` already carried the condition, and `TestFwdPool_StopUnblocksDispatch` already read it that way.

### Bugs Found/Fixed
- The pre-fix `require.Never` passed with backpressure deleted. Measured at closure: a pre-fix binary with a 150ms delay before the fourth dispatch, run against a `Dispatch` whose blocking send was mutated into a dropping `default:` arm, PASSES in 0.15s. The rewritten test under the same mutation and the same delay FAILS in 1.00s. Covered by both rewritten tests.
- The pre-fix test hung the whole package on any red. Reproduced at closure: the pre-fix binary under the mutation printed `Condition satisfied` at the `require.Never`, then sat in `defer pool.Stop()` until the 60s test timeout panicked with a goroutine dump. Covered by the deferred `release()`.

### Documentation Updates
- None. The grep in the Documentation Update Checklist above is the proof: no anchored doc claim names `forward_pool_test.go`, and no anchored file is in the diff.
- `make ze-doc-verify` was not run, because no documentation file was edited.

### Deviations from Plan
- None against the implementation steps. A-2 and A-3 were both broken during design, and the spec records both with the producing function.
- The commit that carries this work, `c08237575`, also carries two unrelated repairs: `test/plugin/as112-probe-anycast-not-loopback.ci` and two marker comments in `internal/component/cli/client/main.go`. That bundling costs this commit its single focus (`ai/rules/git-safety.md`, "Commit Granularity"). It is recorded, not rewritten: history is never rewritten to reclaim a boundary.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed the worker takes a batch, so channel occupancy after three dispatches is not deterministically two | `fwdPool.runWorker` calls `drainBatch` and only then `safeBatchHandle`, so the drain finishes before the handler signals `handlerStarted`, and it drains a channel the test has not written to yet. The fill is deterministic and step 1 of the test is sound | read `fwdPool.runWorker` (`internal/component/bgp/reactor/forward_pool.go`) | no test change followed. The spec records A-2 broken |
| assumption | A-3 assumed the pool exposes no observable for the blocked state, so the fix would have to add one | `fwdWorker.pending` already carries it, `fwdPool.Dispatch` maintains it, and `TestFwdPool_StopUnblocksDispatch` already reads it | read the `fwdWorker` struct and `fwdPool.Dispatch` | `forward_pool.go` was left unmodified. R-3 does not apply: no new state and no new accessor |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The flaky test is fixed rather than quarantined | Done | `internal/component/bgp/reactor/forward_pool_test.go`, `TestFwdPool_BackpressureBehavior` | 4 failures in 43 pre-fix runs of `-count=20`, 0 in 43 post-fix runs under the same load |
| The twin test carrying the same shape is fixed in the same pass | Done | same file, `TestForwardPoolBackpressurePropagation` | identical predicate and identical completion wait |
| The backpressure claim survives unchanged | Done | same file, both tests | a mutation that deletes backpressure reds both tests. The pre-fix pair passes under it |
| No `plan/known-failures/` shard | Done | `plan/known-failures/` | a grep for either test name over that directory returns nothing |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | both tests pass | a race-built package binary of the tree at HEAD ran both tests and exited 0 in 0.00s each |
| AC-2 | Done | 43 invocations of `-test.count=20` under 32 CPU burners on 16 cores at `GOMAXPROCS=2` | 860 executions of each test, 0 failures. The AC asked for 200 iterations at `GOMAXPROCS=8`. The load run is harsher on both axes: 2x oversubscription against 1.5x, and a quarter of the parallelism |
| AC-3 | Done | read both tests | the only durations left are `require.Eventually`'s 1s budget and 1ms tick, both carrying the comment that names the condition the loop waits on. The completion wait is a bare channel receive |
| AC-4 | Done | a `default:` arm in `fwdPool.Dispatch`'s send select that drops the item | both rewritten tests FAIL in 1.00s. Both pre-fix tests PASS in 0.15s when the fourth dispatch is delayed 150ms. The mutation was applied through `go test -overlay`, so `forward_pool.go` in the tree was never edited |
| AC-5 | Done | `plan/known-failures/` | no shard exists for either test |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestFwdPool_BackpressureBehavior` | Done | `internal/component/bgp/reactor/forward_pool_test.go` | rewritten, green under load, red under the mutation |
| `TestForwardPoolBackpressurePropagation` | Done | same file | rewritten, green under load, red under the mutation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/forward_pool_test.go` | Done | 107 lines changed in `c08237575` |
| `internal/component/bgp/reactor/forward_pool.go` | Unchanged, as planned | `git show --stat c08237575` names no such path |

### Audit Summary
- **Total items:** 11
- **Done:** 11
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The test stops failing under load | load run of a race-built binary | 43 invocations of `-test.count=20` under 32 CPU burners on 16 cores at `GOMAXPROCS=2`: pre-fix 4 failures, post-fix 0, over 860 executions each. Every pre-fix failure landed on the 2s `require.Eventually`, each at 2.10s to 2.17s, which is that budget expiring |
| The test still discriminates | mutation run | `fwdPool.Dispatch`'s blocking send mutated into a dropping `default:` arm through `go test -overlay`. Rewritten tests FAIL in 1.00s. Mutation application verified by requiring exactly one occurrence of the inserted marker in the overlaid source and zero in the tree, and by the behavioral difference: the same test code passes in 0.00s against the unmodified `Dispatch` |
| The rewrite is worth doing, not cosmetic | mutation run against the pre-fix code | the pre-fix pair PASSES in 0.15s with backpressure deleted when the fourth dispatch is delayed 150ms. That is the vacuity the rewrite removes |
| A red reports instead of hanging | timed red | pre-fix under the mutation: `Condition satisfied`, then a hang to the 60s timeout. Rewritten under the mutation: FAIL in 1.00s |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `TestFwdPool_BackpressureBehavior` fails under `make ze-unit-reactor-test-race` (row 7 of `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`) | done | fixed in `c08237575` and closed by this spec |

The shard is NOT removed. Two rows in it are still live and homed elsewhere:
the child-2 `.ci` substitution row and the child-3 AC-9 dead-code row, each
pointing at its own spec.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-fwdpool-backpressure-timing-ebf5d9b3-b158-40df-bba0-32a51591883e.md`, recorded over `internal/component/bgp/reactor/forward_pool_test.go` |
| `review_gate.py check` | clean. `review_gate: OK (0 code files, clean, hashes match)`. It reports zero code files because the code landed in `c08237575` and commit A carries only spec, deferral and journal text |
| Rounds | 1 |
| Reviewer lenses used | removed-behavior audit, logic and concurrency, simplicity and duplication, style pass over the changed Go, documentation drift, security and allocation |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | no BLOCKER and no ISSUE found | - | - |

Three NOTEs were recorded and none blocks. The predicate takes `pool.mu.Lock()`
where a read lock would do, and it matches the sibling test that already reads
`pending` that way. The 13-line predicate is copied between the two tests rather
than shared. The completion wait states why it carries no deadline but does not
name what catches a genuine hang, which is the `go test` timeout.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/forward_pool_test.go` | Yes | `gopls symbols` on it lists `TestFwdPool_BackpressureBehavior` and `TestForwardPoolBackpressurePropagation` |
| `internal/component/bgp/reactor/forward_pool.go` | Yes, unmodified | `gopls symbols` lists `(*fwdPool).Dispatch` and the `fwdWorker` struct with its `pending` field |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | both tests pass | race-built binary of HEAD, both tests, exit 0 |
| AC-2 | no failure under CPU oversubscription | 43 invocations of `-test.count=20`, 32 burners on 16 cores, `GOMAXPROCS=2`: 0 failures in 860 executions |
| AC-3 | no assertion rests on an elapsed duration | read both wait sites: `require.Eventually` over `pending` with a comment naming the condition, then a bare `<-dispatched` |
| AC-4 | the mutation reds both tests | both FAIL in 1.00s. Marker count in the overlaid source is 1, in the tree 0 |
| AC-5 | no known-failure shard | a grep over `plan/known-failures/` returns nothing for either test name |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| The reactor dispatches into a per-peer channel that is already full | none. This spec repairs a unit test and adds no user-facing behavior | Yes. `fwdPool.Dispatch` blocks on a full `w.ch` and both rewritten tests prove it. `make ze-unit-reactor-test-race` is the driving surface |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | the failure is in the test. Every pre-fix failure landed on the 2s completion budget, and the post-fix binary is clean over the same load with `forward_pool.go` unchanged |
| A-2 | broken | `fwdPool.runWorker` drains before it hands the batch to the handler |
| A-3 | broken | `fwdWorker.pending` already existed and `TestFwdPool_StopUnblocksDispatch` already read it |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No doc names `forward_pool_test.go` | `grep -rn forward_pool docs/ ai/`: 61 hits, none naming that file | Yes |
| No anchored file is in the diff | the diff touches one file, and no `<!-- source: -->` anchor names it | Yes |

## Core Insight

`require.Never` over "a goroutine has not finished yet" asserts nothing when the
goroutine may not have started. It reports the same green for a parked
dispatcher and for a dispatcher the scheduler has not run. The condition the
test wanted was already in the product, on `fwdWorker.pending`, and reading it
made the assertion both stronger and deadline-free.
