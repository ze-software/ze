# Spec: fixit-test-harness-fail-open-guards

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Two guards in the functional test harness fail OPEN. One reports success for a
run that was cancelled. The other can never fire, because a shorter timeout always
wins first.**

Found on 2026-08-02 by the independent review of
`plan/spec-rfcgate-2-deferred-rs-replay-evidence.md`, while closing
`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`.

A guard that fails open in the test harness is worse than no guard. It converts a
run that proved nothing into a green bar, and a green bar is what everyone reads.
`ai/rules/fail-closed-guards.md` states the rule these two break: fail closed or say
something, and a zero value must never be a valid looking answer.

### Guard 1: a cancelled accept reports success

`acceptConnMapBatch` (`internal/test/peer/peer_connmap.go`) accepts a batch of
connections. When `ln.Accept()` returns an error it checks whether the context is
done, and if it is, returns `Result{Success: true}`.

Nothing consults `checker.Completed()`. So a run cancelled before the checker
finished its assertions is indistinguishable from a run that passed them. The
cancellation path is the one taken on timeout and on operator interrupt, which are
exactly the cases where a false green is most costly.

### Guard 2: a diagnostic that can never fire

`run_rs_observer` (`test/scripts/ze_api.py`) documents itself as failing closed:
when the replay does not complete before `eor_timeout` it reports the named
diagnostic `ZE-OBSERVER-FAIL`. Its default `eor_timeout` is 30.0 seconds.

`test/plugin/bgp-rs-relay-aspath-transparency.ci` runs with a foreground timeout of
20 seconds. The foreground timeout therefore always expires first, the process is
torn down, and the diagnostic the harness advertises never reaches anyone. The test
still denies a broken run, so the coverage is real, but the failure it reports is
"timed out" rather than the specific diagnosis the harness was written to give.

The same arithmetic applies to any `.ci` whose timeout is under 30 seconds. This
spec must survey them rather than fix the one instance found.

## Required Reading

| Document | Why |
|----------|-----|
| `ai/rules/fail-closed-guards.md` | The rule both guards break, and the shape of the repair |
| `ai/rules/testing.md` | Observer-exit antipattern, and how a harness hides a broken production path |
| `internal/test/peer/peer_connmap.go` | Guard 1's producing function |
| `test/scripts/ze_api.py` | Guard 2's producing function and its timeout contract |

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/peer/peer_connmap.go` - `acceptConnMapBatch` returns `Result{Success: true}` on a context-cancelled accept
- [ ] `test/scripts/ze_api.py:1766` - `eor_timeout: float = 30.0`, with the fail-closed contract documented at `:1786`
- [ ] `test/plugin/bgp-rs-relay-aspath-transparency.ci` - foreground timeout of 20 seconds

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every currently passing functional test must still pass. This spec tightens a
  guard; it must not change what a correct run does.
- The `.ci` file format and the observer's public arguments.

**Behavior to change:** (only what the user asked for)
- A cancelled accept must not report success unless the checker completed.
- A harness timeout must be reachable, or the harness must not advertise it.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A `.ci` file's `timeout=` directive, and the test runner's context deadline.
- A TCP listener accepting peer connections inside `internal/test/peer`.

### Transformation Path
1. The runner starts a `.ci` with a foreground timeout and a context.
2. `acceptConnMapBatch` accepts connections until the batch fills or the context ends.
3. On context end it returns a `Result`, which the runner renders as pass or fail.
4. In parallel, `run_rs_observer` waits up to `eor_timeout` for the replay and, on
   expiry, emits `ZE-OBSERVER-FAIL`.

The defect in step 3 is that the `Result` does not consult the checker. The defect in
step 4 is that the step 1 deadline is shorter than the step 4 deadline, so step 4's
branch is unreachable.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test runner ↔ peer harness | `Result` struct with a `Success` bool | No |
| `.ci` timeout ↔ observer timeout | Two independent numbers, never compared | No |

### Integration Points
- `checker.Completed()` - exists and reports whether assertions finished. Guard 1 must
  consult it.
- The `.ci` parser's `timeout=` handling - guard 2's repair needs to know the value.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

| ID | Assumption | Validation | Status |
|----|-----------|------------|--------|
| A-1 | No currently green test depends on the cancelled-accept success path | Tighten the guard, run the full functional suite | unvalidated |
| A-2 | `checker.Completed()` is reachable from `acceptConnMapBatch` | Read the call site and the type | unvalidated |
| A-3 | The 20s versus 30s mismatch is not unique to one `.ci` | Survey every `.ci` timeout against the observer default | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | Tightening guard 1 turns load-sensitive tests red | Run the suite under parallelism before and after, and read `ai/rules/flaky-under-load.md` |
| R-2 | Lowering `eor_timeout` makes a slow but correct replay fail | Derive the observer timeout from the `.ci` timeout rather than picking a second constant |

## Blast Radius

`internal/test/peer/peer_connmap.go`, `test/scripts/ze_api.py`, and any `.ci` whose
timeout is shorter than the observer default. No product code. A false green removed
here may expose tests that were passing for the wrong reason, which is the point.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A batch accept cancelled before the checker completes | → | `acceptConnMapBatch` | `TestCancelledAcceptDoesNotReportSuccess` |
| A batch accept cancelled after the checker completes | → | `acceptConnMapBatch` | `TestCancelledAcceptAfterCompletionStillPasses` |
| An observer whose replay exceeds its budget | → | `run_rs_observer` | a `.ci` whose timeout exceeds the observer timeout |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ln.Accept()` fails with the context done, and the checker has NOT completed | `Result.Success` is false and the error names cancellation before completion |
| AC-2 | `ln.Accept()` fails with the context done, and the checker HAS completed | `Result.Success` is true, unchanged from today |
| AC-3 | `run_rs_observer` runs inside a `.ci` whose foreground timeout is shorter than `eor_timeout` | Either the observer timeout is derived from the `.ci` budget, or the mismatch is reported as a harness error rather than silently unreachable |
| AC-4 | Every `.ci` invoking `run_rs_observer` | Surveyed, and none leaves the diagnostic unreachable |
| AC-5 | The full functional suite, before and after | Same set of passing tests, or a named test whose green was false, with the evidence |
| AC-6 | Guard 1 mutated to return success unconditionally | `TestCancelledAcceptDoesNotReportSuccess` turns red |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Interrupts a functional run partway | runner → context cancel → `acceptConnMapBatch` | `TestCancelledAcceptDoesNotReportSuccess` |
| 2 | Runs an RS test whose replay stalls | `.ci` → `run_rs_observer` → diagnostic | The `.ci` added under AC-3 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCancelledAcceptDoesNotReportSuccess` | `internal/test/peer/peer_connmap_test.go` | AC-1, AC-6 | |
| `TestCancelledAcceptAfterCompletionStillPasses` | `internal/test/peer/peer_connmap_test.go` | AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `eor_timeout` versus the `.ci` foreground timeout | observer budget must be under the `.ci` budget | `.ci` timeout minus the teardown margin | 0, no time to observe | any value at or above the `.ci` timeout, which is the defect |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| an RS observer test with a reachable diagnostic | `test/draft/plugin/` then promoted | A stalled replay reports `ZE-OBSERVER-FAIL` rather than a bare timeout | |

### Interop Tests (Scope: tooling)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | none | none | Harness only, no wire behavior and no peer daemon involved | |

## Files to Modify
- `internal/test/peer/peer_connmap.go` - guard 1
- `test/scripts/ze_api.py` - guard 2
- `docs/functional-tests.md` - the observer's timeout contract, once AC-3 settles how the two budgets relate
- any `.ci` the AC-4 survey names

Note for a future reader: every file above lives in the test harness, and the
`validate-spec.sh` feature-integration check reads that as a tests-only spec. The
harness IS the product here. A guard that fails open turns a run that proved nothing
into a green bar, so the harness is exactly where this work belongs.

## Files to Create
- `internal/test/peer/peer_connmap_test.go` if it does not already exist
- one functional `.ci` under `test/draft/plugin/`, promoted once green

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | Test harness only |
| CLI commands/flags | No | None |
| Functional test for new RPC/API | Yes | The AC-3 `.ci` |
| Doctor check for runtime dependencies | No | No runtime dependency added |
| Prometheus counters/metrics | No | No observable daemon state |
| BGP family surface | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Harness internals |
| 2 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the observer's timeout contract changes |
| 3 | Wire format changed? | No | None |
| 4 | Internal architecture changed? | No | None |

## Implementation Steps

1. Read `checker.Completed()` and confirm A-2.
2. Write the two guard 1 tests, watch them fail, then tighten the guard.
3. Run the full functional suite and compare the passing set against A-1.
4. Survey every `.ci` calling `run_rs_observer` against its own timeout (AC-4).
5. Derive the observer budget from the `.ci` budget, or make the mismatch an error.
6. Add the functional `.ci` for AC-3 and promote it once green.

## Design Insights

Two timeouts that must be ordered, chosen independently in two files, will drift. The
repair is to derive one from the other, not to pick better constants
(`ai/rules/derive-not-hardcode.md`).

## Key Design Decisions

Not yet taken.

## Known Limitations

Removing a false green can turn tests red that were passing for the wrong reason.
That is the intended outcome and must not be treated as a regression caused by this
spec.

## RFC Documentation (Scope: tooling)

No RFC behavior. This spec changes the test harness only. No requirement id is added,
retired or re-levelled, so `make ze-rfc-check` is unaffected.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features, or N-A with a reason

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Provenance

The independent review of `plan/spec-rfcgate-2-deferred-rs-replay-evidence.md`,
2026-08-02, items 3 and 4 of its "left for someone else" list.
