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
