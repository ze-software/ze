# Spec: fixit-runner-kill-background -- stop a background process mid-test

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/1107-test-coverage-gaps.md` `:59-63` - the deferral that created this spec
4. `internal/test/runner/runner_exec.go` - `bgProcs` lifecycle

## Task

The `.ci` runner can start a background process but cannot **stop one mid-test**. Its
only `Process.Kill()` calls are teardown or timeout paths, verified 2026-07-16:

| Site | Role |
|------|------|
| `internal/test/runner/runner_exec.go:430` | `var bgProcs []*exec.Cmd` -- the tracking slice |
| `internal/test/runner/runner_exec.go:431-436` | `defer func(){ ... p.Process.Kill() }` -- teardown only, runs when the test ends |
| `internal/test/runner/runner_exec.go:146`, `:161`, `:220` | ze-peer teardown / timeout |
| `internal/test/runner/runner_exec_util.go:368` | command timeout |

Background processes are started at `runner_exec.go:759` (`cmd.Mode == modeBackground`).
Nothing between start and teardown can terminate one.

**What this blocks.** Any behavior that fires only on peer death cannot be observed
end to end. The known case is IPsec DPD liveness teardown: prove the initiator tears
down the SA (child-down / sa-down) when the responder goes silent. `test/ipsec/`'s
`ipsec-dpd-timeout.ci` was DELETED for exactly this reason. DPD timer/probe logic is
unit-tested (`internal/component/ike/engine/dpd_test.go`, `established_test.go`); only
the kill-and-observe leg is blocked.

**Provenance.** Deferred from `spec-test-coverage-gaps` AC-3 on 2026-07-10. The row in
`plan/deferrals.md` named this file with "(new when picked up)" and the skeleton was
never created, so the row dangled with a destination that did not exist and blocked the
commit gate. `plan/learned/1107-test-coverage-gaps.md:59-63` records the gap and already
names `spec-fixit-runner-kill-background.md` as its owner -- this file makes that true.

**Scope.** Add a runner primitive to stop a named background process at a chosen step.
It is test infrastructure, not product code. Design must settle: how a `.ci` names the
target process, which signal is sent (SIGKILL vs SIGTERM -- DPD needs a peer that stops
answering, and SIGTERM may let strongSwan/ze send a DELETE, which defeats the test), and
whether the runner waits for the process to actually exit before advancing.

**Not in scope.** Rewriting `ipsec-dpd-timeout.ci` itself. Once the primitive exists,
restoring that `.ci` is follow-on work that this spec's AC-4 proves is possible.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - the `.ci` record format and runner contract
  → Constraint: a new directive is a change to the shared test grammar; every suite parses it
- [ ] `ai/rules/testing.md` - what a functional test must prove
  → Constraint: the primitive must enable a real observation, not a sleep-and-hope
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - why the `.ci` was deleted rather than weakened
  → Constraint: do not re-add the test until the primitive makes it real

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - IKEv2 Section 2.4 liveness / DPD, the behavior the unblocked test asserts
  → Constraint: DPD declares a peer dead only after retransmits go unanswered; the killed process must stop answering, not close cleanly

**Key insights:**
- The runner already tracks every background process in one slice (`bgProcs`), so the primitive has an obvious anchor: it needs a name-to-process map and a step-time lookup, not new lifecycle machinery.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_exec.go` - `:430` tracks `bgProcs`; `:431-436` kills them only in a deferred teardown; `:759` starts `modeBackground` processes; `:922` picks the foreground daemon as "the last non-peer background process"
- [ ] `internal/test/runner/runner_exec_util.go` - `:368` kills on command timeout only
- [ ] `internal/test/runner/record_parse.go` - parses `.ci` directives; a new directive registers here

**Behavior to preserve:**
- Existing teardown semantics: every background process is still killed when the test ends, including on failure and timeout. The new primitive must not leak processes.
- Every existing `.ci` file must parse and run unchanged. The directive is additive.
- The foreground-daemon selection at `:922` must keep working when a background process has already been killed mid-test.

**Behavior to change:**
- A `.ci` gains the ability to terminate a named background process at a chosen step and to observe what happens next.

## Data Flow (MANDATORY)

### Entry Point
- A `.ci` record line declaring a stop action against a named background process, parsed by `record_parse.go` and executed by the step executor in `runner_exec.go`.

### Transformation Path
1. `.ci` declares a background process with a name at start time (`modeBackground`, `runner_exec.go:759`).
2. Runner starts it and appends the `*exec.Cmd` to `bgProcs` (`:430`).
3. A later `.ci` step names that process in a stop directive.
4. Runner looks the process up, signals it, and (per design) waits for exit.
5. Subsequent steps observe the product's reaction (for DPD: poll `show vpn ipsec sa` until the SA is gone, or assert the child-down/sa-down event).
6. Teardown still runs and must tolerate an already-dead process.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` grammar ↔ runner | new directive parsed in `record_parse.go`, dispatched in the step executor | [ ] |
| Runner ↔ OS process | signal delivery to a tracked `*exec.Cmd`; exit reaping | [ ] |
| Runner ↔ product under test | the killed peer stops answering; ze's DPD observes silence | [ ] |

### Integration Points
- `bgProcs` (`runner_exec.go:430`) - the existing tracking slice the lookup extends.
- `modeBackground` (`runner_exec.go:759`) - where a name would be recorded.
- `record_parse.go` - directive parsing; note `parseOption` rejects unknown option types, so an unregistered directive fails closed (this is why `test/pppoe/` is dead).

### Architectural Verification
- [ ] No bypassed layers (the directive goes through the normal parse + step-execute path)
- [ ] No unintended coupling (the primitive is generic; nothing about it is IPsec-specific)
- [ ] No duplicated functionality (extends `bgProcs`, does not add a second process registry)
- [ ] Registration over hardcoding — the directive registers in the parser's table rather than adding a special case to the executor (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A killed responder is indistinguishable from a silent peer, so DPD fires | RFC 7296 liveness: DPD triggers on unanswered retransmits | SIGKILL may cause a TCP/UDP-level error ze treats differently from silence, and DPD never fires | Prototype: kill the responder, observe ze's logs for DPD probe retransmits | unvalidated |
| A-2 | Background processes can be uniquely named in `.ci` today | `runner_exec.go:759` starts them; naming is unverified | The directive needs a naming scheme added to the grammar first, widening scope | Read `record_parse.go` `modeBackground` parsing | unvalidated |
| A-3 | This primitive is the only thing blocking the DPD test | `plan/learned/1107:59-63` says so | Restoring the `.ci` needs more (e.g. the bgp-namespace event-subscription gap, `spec-fixit-plugin-event-subscription`) | Prototype the restored `.ci` end to end | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | SIGTERM lets the peer send a clean DELETE, so the test proves teardown-on-DELETE, not DPD | The SA drops instantly instead of after the DPD interval | Default to SIGKILL; make the signal explicit in the directive |
| R-2 | A new directive changes the shared grammar and destabilizes unrelated suites | Unrelated `.ci` files fail to parse | Additive directive, default-off; run the full functional suite |
| R-3 | The DPD test becomes slow (DPD intervals are seconds) or flaky under load | Test time balloons; intermittent reds | Configure a short DPD interval in the test config; assert via polling, not sleeps (`ai/rules/testing.md`) |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` stop directive naming a background process | → | parser directive + step executor lookup in `bgProcs` | `TestParseStopBackgroundDirective` |
| Runner executes the stop step | → | signal delivery + exit reaping | `TestStopBackgroundKillsNamedProcess` |
| A `.ci` kills a background sleeper and asserts it is gone | → | full parse → start → stop → assert path | `test/runner/stop-background.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `.ci` names a background process and stops it at step N | The process is terminated at step N, before step N+1 runs |
| AC-2 | The stop directive names an unknown process | The test FAILS with a clear error; it does not silently pass (`ai/rules/fail-closed-guards.md`) |
| AC-3 | A test that stops a background process then ends | Teardown does not error on the already-dead process; no leaked processes |
| AC-4 | An IPsec DPD `.ci` kills the responder | The initiator tears the SA down after the DPD interval, observable via `show vpn ipsec sa` |
| AC-5 | Every pre-existing `.ci` | Parses and runs exactly as before |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Test author stops a background daemon mid-test | `.ci` directive → parser → step executor → signal → assert | `test/runner/stop-background.ci` |
| 2 | Test author proves DPD tears down a dead peer's SA | `.ci` → kill responder → DPD interval → `show vpn ipsec sa` empty | `test/ipsec/ipsec-dpd-timeout.ci` (restored) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseStopBackgroundDirective` | `internal/test/runner/record_parse_test.go` | the directive parses; unknown names rejected (AC-2) | |
| `TestStopBackgroundKillsNamedProcess` | `internal/test/runner/runner_exec_test.go` | AC-1: the named process is dead after the step | |
| `TestTeardownToleratesStoppedProcess` | `internal/test/runner/runner_exec_test.go` | AC-3: no error, no leak | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| stop step index | 1..N steps | N | 0 (before start) | N+1 (after teardown) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `stop-background` | `test/runner/stop-background.ci` | a background process is stopped mid-test and the test observes it | |
| `ipsec-dpd-timeout` | `test/ipsec/ipsec-dpd-timeout.ci` | the initiator tears down the SA when the responder goes silent (AC-4) | |

### Future (if deferring any tests)
- None. AC-4 is the reason the primitive exists; it must be proven, not deferred.

## Files to Modify
- `internal/test/runner/record_parse.go` - parse and register the stop directive
- `internal/test/runner/runner_exec.go` - name background processes at `:759`; look up and signal at step time; keep teardown at `:431-436` idempotent
- `docs/functional-tests.md` - document the new directive

## Files to Create
- `test/runner/stop-background.ci` - wiring proof
- `test/ipsec/ipsec-dpd-timeout.ci` - restored DPD test (AC-4)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; resolve A-2 |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify` |
| 14. Present summary + close | two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add the directive to the parser with a stub executor
   - Tests: `TestParseStopBackgroundDirective`
   - Verify: the directive parses; the executor is a stub so `stop-background.ci` FAILS
2. **Phase: Naming** — resolve A-2; give background processes addressable names
   - Verify: a `.ci` can name a background process at start
3. **Phase: Stop** — implement lookup + signal + reap
   - Tests: `TestStopBackgroundKillsNamedProcess`, `TestTeardownToleratesStoppedProcess`
   - Verify: `test/runner/stop-background.ci` goes green
4. **Phase: DPD proof** — restore `ipsec-dpd-timeout.ci` (AC-4); resolve A-1 and A-3 here
   - Verify: the SA tears down after the DPD interval, with no sleeps
5. **Full verification** → `make ze-verify` plus the full functional suite (AC-5)
6. **Complete spec** → learned summary; TWO commits (A: code+tests+spec+learned; B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each have a test |
| Fail-closed | An unknown process name FAILS the test; it never no-ops (`ai/rules/fail-closed-guards.md`) |
| Correctness | The DPD test fails if the primitive is removed (mutation-check the proof) |
| Data flow | No process escapes `bgProcs` tracking |
| Rule: no-workarounds | The restored `.ci` asserts DPD, not a proxy for it |
| Registration over hardcoding | Directive registers in the parser table; no executor special case (`ai/rules/plugin-self-containment.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Stop directive documented | grep `docs/functional-tests.md` for the directive |
| DPD test restored and green | run `test/ipsec/ipsec-dpd-timeout.ci` |
| No leaked processes | run the suite twice; check for orphans |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | A `.ci`-supplied process name must not be able to signal an arbitrary PID outside `bgProcs` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken (DPD never fires on kill) | Back to design: the test may need a network-level black-hole instead of a kill |
| A-3 broken (more gaps block the DPD test) | Land the primitive + AC-1..AC-3; route AC-4 to the blocking spec with a new deferral row |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- The deferral row that created this spec named a file that was never created, so the commit gate flagged it for months. The skeleton rule exists precisely to stop that; "(new when picked up)" is not a destination.

## Known Limitations
- (fill during design)

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/test/runner/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
