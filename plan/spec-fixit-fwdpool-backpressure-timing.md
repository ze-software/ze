# Spec: fixit-fwdpool-backpressure-timing -- a backpressure test that waits on a duration instead of a condition

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`TestFwdPool_BackpressureBehavior`
(`internal/component/bgp/reactor/forward_pool_test.go`) fails under
`make ze-race-reactor` at `-count=20`. It passes 5 runs out of 5 in isolation.
`-race` reports zero data races.

**This is a BROKEN TEST, and it must be FIXED. It must NOT go into
`plan/known-failures/`.** `ai/rules/fix-dont-record.md` is explicit: a test that
passes on a quiet host and fails on a busy one waits on a duration instead of on
a condition, and "passes in isolation" is a banned excuse. It reproduces, so
there is no recording path for it. The load is not the explanation, it is the
symptom.

### The mechanism, read on 2026-08-02

The test builds a pool with `chanSize: 2` and a handler that blocks on a channel.
It then does this:

| Step | What the test does | What it assumes |
|------|--------------------|-----------------|
| 1 | Three `Dispatch` calls, with a receive on `handlerStarted` after the first | that the steady state is one item in the handler plus two in the channel |
| 2 | A fourth `Dispatch` in a goroutine | that this one must block |
| 3 | `require.Never(..., 100*time.Millisecond, 10*time.Millisecond)` | that 100 milliseconds of not-completing PROVES blocking |
| 4 | Close the blocker, then `require.Eventually(..., 2*time.Second, ...)` | that the fourth dispatch completes once the handler drains |

Steps 1 and 3 are both duration-shaped.

Step 3 is the obvious one: 100 milliseconds of silence is not proof that a
dispatch is blocked, it is proof that it has not finished yet. On a loaded host
the fourth dispatch can be scheduled and complete inside that window if the
channel was not actually full, and the assertion inverts.

Step 1 is the one that makes step 3 unreliable, and it is the likelier root
cause. `handlerStarted` proves the handler goroutine ENTERED. It does not prove
how many items the worker had already taken off the channel when it did.
Confirm at design time whether the worker drains a BATCH: if it does, the
channel occupancy after three dispatches is a scheduling outcome, not a
guaranteed two, and the fourth dispatch may legitimately not block.

### The fix shape

Wait on the drained condition rather than on a clock. The test needs a way to
observe that the per-peer channel is full, and a way to observe that it has been
drained. Neither exists today, which is the product gap
(`ai/rules/fix-dont-record.md`: "If none exists, ADD one; a missing signal is a
product gap, not a test problem"). Expect this to need a small observable on the
pool, not only a test rewrite. Confirm at design time whether an existing
accessor already answers it before adding one.

The assertion the test exists to make is unchanged and must survive: a dispatch
into a full channel BLOCKS and no item is dropped. Weakening that to reach green
is banned (`ai/rules/no-test-deletion.md`).

Source: `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, row 7.

## Required Reading

- [ ] `ai/rules/fix-dont-record.md` - load is never an explanation, it is the bug
  → Constraint: no `plan/known-failures/` shard. The failure reproduces, so it gets fixed.
- [ ] `ai/rules/flaky-under-load.md` - use `scripts/dev/stress-repro.py`, never a loop over the full suite
  → Constraint: reproduce under the stress reproducer, then read the test; do not hunt with repeated verify runs.
- [ ] `ai/rules/no-test-deletion.md` - the assertion is the requirement
  → Constraint: the backpressure claim survives the rewrite unchanged.
- [ ] `ai/rules/testing.md` - "Reactor Concurrency Code"
  → Constraint: `make ze-race-reactor` is the required verification for anything in this package that shares state across goroutines.

**Key insights:**
- Two duration-shaped steps, not one. Fixing only `require.Never` may leave the fill sequence racy.
- `-race` clean means there is no data race. It says nothing about a timing assumption, which is what this is.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_pool_test.go` - `TestFwdPool_BackpressureBehavior` at the `require.Never` / `require.Eventually` pair; VALIDATES AC-10 (backpressure on full channel), PREVENTS silent message drops under load
- [ ] `internal/component/bgp/reactor/forward_pool.go` - `newFwdPool`, `Dispatch`, and the per-peer worker loop; read the worker's take path before assuming channel occupancy

**Behavior to preserve:** the test's subject. A `Dispatch` into a full per-peer channel must block, and must not drop the item. The `VALIDATES:` and `PREVENTS:` annotations stay accurate. No production backpressure behavior changes.

**Behavior to change:** how the test waits. Durations become conditions. If an observable must be added to `forward_pool.go` to make the condition readable, that is a deliberate addition and not a workaround.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

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
| Test ↔ pool state | nothing. There is no readable signal for full or drained | Yes, and this absence is the product gap |

### Integration Points
- `internal/component/bgp/reactor/forward_pool.go` - the pool; may gain an observable so the condition is readable.
- `make ze-race-reactor` - the target that reproduces the failure.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | fill during design: any new observable is read through the pool's own API, not by reaching into its fields from the test |
| No unintended coupling | No | fill during design |
| No duplicated functionality | No | fill during design: check for an existing depth or length accessor before adding one |
| Zero-copy preserved where applicable | N-A | no wire path touched |
| Registration over hardcoding (`ai/rules/plugin-self-containment.md`) | N-A | no command, view, family, or handler added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The failure is a timing assumption in the test, not a defect in `Dispatch`. | `-race` is clean and the test passes in isolation, which points at the assertion rather than at the code. | A real backpressure defect exists and is in scope here (`ai/rules/no-parking.md`). | reproduce under `scripts/dev/stress-repro.py`, then read the worker's take path | unvalidated |
| A-2 | The worker takes a batch, so channel occupancy after three dispatches is not deterministically two. | The handler signature receives a slice of items, which suggests batching. NOT yet confirmed against the take path. | Step 1 is sound and only `require.Never` needs replacing. | read `forward_pool.go`'s worker loop | unvalidated |
| A-3 | No observable for full or drained exists on the pool today. | The test uses durations, which is what an author does when no signal is available. | Use the existing one. Do not add a second. | grep the pool's exported and package-level accessors | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fix replaces one duration with a longer one and the test reds again later on a busier host. | A rewrite that still names a millisecond figure as the proof. | A duration may bound a poll, never carry the assertion. The assertion reads a condition. |
| R-2 | The test is quietly relaxed to assert something weaker that always passes. | The `PREVENTS:` line stops matching what the test checks. | The backpressure claim is the requirement (`ai/rules/no-test-deletion.md`). |
| R-3 | A new observable exists only for the test and is dead in production. | An accessor with one caller, and that caller is a `_test.go` file. | A test-only observable is acceptable when it reads state the pool already keeps. Adding new STATE for a test is not. Record which it is. |
| R-4 | The failure is chased by looping `make ze-verify`. | Repeated full-suite runs in the session log. | `ai/rules/flaky-under-load.md`: use `scripts/dev/stress-repro.py` against the suspected package. |

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
| AC-1 | `make ze-race-reactor` at `-count=20` | `TestFwdPool_BackpressureBehavior` passes on every iteration |
| AC-2 | `scripts/dev/stress-repro.py` against the reactor package under CPU oversubscription | The test passes; the pre-fix run must have reproduced the failure first |
| AC-3 | The rewritten test | No assertion rests on an elapsed duration. A duration may bound a poll interval only, and carries a comment saying which condition the loop waits on |
| AC-4 | Backpressure removed from `Dispatch` (a deliberate mutation) | The test FAILS. A test that passes with the behavior removed proves nothing |
| AC-5 | `plan/known-failures/` | Contains no shard for this test, before or after |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFwdPool_BackpressureBehavior` | `internal/component/bgp/reactor/forward_pool_test.go` | AC-1, AC-3, AC-4: a full channel blocks the dispatcher and drops nothing | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| not applicable, no user-facing behavior changes: this repairs a unit test's timing assumption. The driving surface is `make ze-race-reactor` | `internal/component/bgp/reactor/` | a developer runs the reactor race target and it is green every iteration | |

## Files to Modify
- `internal/component/bgp/reactor/forward_pool_test.go` - replace both duration-shaped waits with condition waits
- `internal/component/bgp/reactor/forward_pool.go` - only if the drained or full condition is not readable today; confirm A-3 first

## Implementation Steps

1. Reproduce with `scripts/dev/stress-repro.py` against the reactor package. Capture the first failure's full output. Do not loop `make ze-verify` (`ai/rules/flaky-under-load.md`).
2. Read the worker take path and settle A-2. This decides whether step 1 of the test is also broken.
3. Settle A-3: find or add the observable for full and drained.
4. Rewrite the waits. Every remaining sleep or timeout bounds a poll and carries a comment naming the condition.
5. Mutate: remove backpressure from `Dispatch`, confirm the test reds, revert the mutation (AC-4).
6. `make ze-race-reactor`, then `make ze-verify`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Both duration-shaped steps replaced, not only `require.Never` |
| Correctness | The `VALIDATES:` and `PREVENTS:` annotations still describe what the test checks |
| Rule: `ai/rules/fix-dont-record.md` | No shard was written, and no sentence blames host load |
| Rule: `ai/rules/no-test-deletion.md` | The backpressure assertion is unchanged in strength |
| Registration over hardcoding | N-A |

## Known Limitations
- This fixes one test. Sibling tests in the same file may share the timing assumption; if the read in step 2 shows they do, fix them in the same pass rather than filing them (`ai/rules/no-parking.md`).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] `make ze-verify` passes
- [ ] `make ze-race-reactor` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste the mutated-behavior run from AC-4)
- [ ] Tests PASS (paste the `-count=20` run)
