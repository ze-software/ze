# Spec: fixit-runner-kill-background -- stop a background process mid-test

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

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
| `internal/test/runner/runner_exec_util.go:368` | the forced `Process.Kill()` inside `terminateGracefully` -- a SIGTERM-then-SIGKILL graceful-stop helper invoked at teardown (`runner_exec.go:963`), NOT a command-timeout path |

Background processes are started at `runner_exec.go:759` (`cmd.Mode == modeBackground`,
`modeBackground` defined at `runner_exec_util.go:113`). The append at `:759-760` is
`bgProcs = append(bgProcs, proc)` -- a bare `*exec.Cmd` with NO name field. Nothing
between start and teardown can terminate one, and nothing today can name one.

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
It is test infrastructure, not product code. Two distinct pieces of work: (a) a NAMING
grammar -- background processes have no name field today (the `:759-760` append stores a
bare `proc`), so a first-class way to name a background process at start and reference it
later must be ADDED to the grammar; this is the bulk of the grammar work, not a lookup.
And (b) the stop primitive itself. Design must settle: how a `.ci` names the
target process, which signal is sent (SIGKILL vs SIGTERM -- DPD needs a peer that stops
answering, and SIGTERM may let strongSwan/ze send a DELETE, which defeats the test), and
whether the runner waits for the process to actually exit before advancing.

→ AUTONOMOUS DEFAULT (2026-07-17): the three "must settle" items are resolved as follows so
a fresh implementer has zero questions. Thomas: override any of these if wrong.
- **Signal = SIGKILL by default.** Rationale: SIGTERM lets ze's IKE engine send a peer DELETE
  (the clean-close path `sa.State == StateDead`, `internal/component/ike/engine/established.go:139`),
  which tears the SA down via the DELETE branch and NOT via the DPD-timeout branch — defeating
  AC-4. SIGKILL leaves the responder silent, which is the exact condition DPD detects (A-1,
  validated below). This is the fail-closed choice toward the test's intent. The directive
  still exposes the signal explicitly (`signal=kill|term`, default `kill`) per R-1.
- **Runner waits for exit before advancing.** Rationale: AC-1 requires the process be
  "terminated at step N, before step N+1 runs"; advancing before the OS has reaped the process
  is a race that makes AC-1 non-deterministic. The runner reaps (`(*exec.Cmd).Wait`) the killed
  process, bounded by the step timeout, before the next step. This mirrors the existing
  `terminateGracefully` reap (`runner_exec_util.go:361-372`).
- **Naming grammar (provisional syntax, override the spelling before it ships).** Add an
  optional `:name=NAME` field to the existing `cmd=background:` line (additive, every current
  `.ci` still parses) and a new `cmd=stop:seq=N:name=NAME[:signal=kill|term]` directive.
  Rationale: this reuses the established `cmd=` family and its `seq=` ordering
  (`parseCmd`, `record_parse.go:690-714`; `parseCmdExec`, `:718-761`), so the stop step is
  ordered relative to other commands exactly as `cmd=foreground`/`cmd=background` already are,
  and registers as a new `case "stop"` in `parseCmd` rather than a special case in the executor.
  The `NAME → *exec.Cmd` binding is stored alongside the tracked `*exec.Cmd` at the
  `modeBackground` append (`runner_exec.go:759-760`). This is the smallest self-contained
  grammar change; the exact field/directive spelling is provisional and may be renamed.

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
- [ ] `internal/test/runner/runner_exec_util.go` - `:113` defines `modeBackground`; `:368` is the forced `Process.Kill()` inside `terminateGracefully`, the SIGTERM-then-SIGKILL graceful-stop helper invoked at teardown (`runner_exec.go:963`) -- not a command-timeout path
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
| A-1 | A killed responder is indistinguishable from a silent peer, so DPD fires | RFC 7296 liveness: DPD triggers on unanswered retransmits | SIGKILL may cause a TCP/UDP-level error ze treats differently from silence, and DPD never fires | Prototype: kill the responder, observe ze's logs for DPD probe retransmits | ~~unvalidated (BLOCKS AC-4)~~ VALIDATED 2026-07-17 from source (see note below table) |
| A-2 | Naming a background process requires NEW grammar -- no name field exists today | Code-verified 2026-07-16: `modeBackground` (`runner_exec_util.go:113`) at `runner_exec.go:759`; the append `:759-760` stores a bare `proc` with no name | Confirms the naming grammar is a first-class phase (the bulk of the grammar work), not a lookup under an existing scheme | Read of `runner_exec.go:759-760` and `runner_exec_util.go:113` | resolved: new naming grammar required |
| A-3 | This primitive is the only thing blocking the DPD test | `plan/learned/1107:59-63` says so | Restoring the `.ci` needs more (e.g. the bgp-namespace event-subscription gap, `spec-fixit-plugin-event-subscription`) | Prototype the restored `.ci` end to end | unvalidated (BLOCKS AC-4) |

→ A-1 VALIDATED (2026-07-17, from source — no prototype needed): the IKE transport is an
*unconnected* UDP socket (`net.ListenUDP`, `internal/component/ike/transport/udp.go:47`), so a
SIGKILL'd responder delivers no connection-level error to the initiator. The transport read
loop only logs and `continue`s on any read error and never tears down the SA or clears DPD state
(`udp.go:88-102`); `sendDPD` swallows send errors yet still sets `awaitReply`
(`internal/component/ike/engine/dpd.go:101-111`). DPD teardown is driven purely by the ticker +
timeout: `dpd.timedOut(now)` calls `cleanupChild` and returns `errTimeout`
(`internal/component/ike/engine/established.go:145-148`); the ONLY thing that clears `awaitReply`
is a genuine authenticated inbound / message-ID-matched INFORMATIONAL reply (`established.go:118-119`
→ `handleDPDResponse`, `dpd.go:116-122`). A *clean* close would instead be a peer-sent DELETE
(`sa.State == StateDead`, `established.go:139-143`) — a DIFFERENT branch that SIGKILL specifically
prevents. Therefore a killed responder reads exactly as a silent peer and DPD fires after the
timeout; the "If wrong" scenario (ze treating a UDP-level error differently from silence) is ruled
out by source. This resolves the A-1 half of AC-4's blocker; AC-4's remaining blocker is A-3, the
end-to-end event-subscription gap, which must still be prototyped in Phase 4.

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
| AC-4 | An IPsec DPD `.ci` kills the responder | The initiator tears the SA down after the DPD interval, observable via `show vpn ipsec sa`. BLOCKED-BY (both unvalidated, must be confirmed first): A-1 (does SIGKILL read as DPD-silence, or a transport error ze handles differently?) and A-3 (`spec-fixit-plugin-event-subscription` may also gate this). AC-4 cannot be claimed until both are validated |
| AC-5 | Every pre-existing `.ci` | Parses and runs exactly as before |

→ AC-4 UPDATE (2026-07-17): A-1 is now VALIDATED from source (see the Assumptions note), so the
"does SIGKILL read as DPD-silence" half of AC-4's blocker is cleared. AC-4's only remaining
blocker is A-3 (`spec-fixit-plugin-event-subscription` may also gate the end-to-end observation).
Confirm A-3 by prototyping the restored `.ci` in Phase 4; if A-3 is broken, land AC-1..AC-3 and
route AC-4 to the blocking spec with a new deferral row per the Failure Routing table. AC-4 is
still not claimable until A-3 is confirmed.

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
| 2. Audit | Files to Modify; naming grammar is a first-class phase (A-2 verified: new grammar needed) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify` |
| 14. Present summary + close | two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add the directive to the parser with a stub executor
   - Tests: `TestParseStopBackgroundDirective`
   - Verify: the directive parses; the executor is a stub so `stop-background.ci` FAILS
2. **Phase: Naming grammar (first-class, the bulk of the grammar work)** — ADD a way for a `.ci` to name a background process at start and reference it later. Verified (A-2): no name field exists today (the `:759-760` append stores a bare `proc`), so this adds new grammar in `record_parse.go` plus a name alongside the tracked `*exec.Cmd`; it is not a lookup under an existing scheme
   - Tests: `TestParseStopBackgroundDirective` (naming half)
   - Verify: a `.ci` can name a background process at start and the name is stored alongside its `*exec.Cmd`
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
- The stop primitive targets only processes the runner itself started and tracked in `bgProcs`
  (`runner_exec.go:430`); it cannot signal an arbitrary PID. A `.ci`-supplied name that does not
  match a tracked background process FAILS the test (AC-2), it never signals anything (this is
  the fail-closed guard called out in the Security Review Checklist).
- Default signal is SIGKILL, which is deliberately ungraceful: the killed process gets no chance
  to flush, close sockets, or send a protocol teardown. That is required for the DPD proof (a
  clean DELETE would defeat AC-4) but means the primitive is not a substitute for a graceful
  stop; tests wanting graceful shutdown must pass `signal=term`.
- AC-4 (the DPD end-to-end proof) additionally depends on A-3 (`spec-fixit-plugin-event-subscription`).
  If that gap is still open when this spec is implemented, the primitive (AC-1..AC-3) still lands;
  only the AC-4 observation is routed onward per the Failure Routing table.
- The primitive observes "process is gone", not "process finished cleanly" — SIGKILL'd processes
  have no exit code worth asserting, so the directive does not expose an expected-exit check for
  the stopped process.

## RFC Documentation

The unblocked test asserts IKEv2 Dead Peer Detection (liveness), RFC 7296 Section 2.4. This
spec adds only test infrastructure; it changes no protocol code. The RFC grounding matters
because it fixes what the killed peer must look like on the wire for the proof to be valid.

| RFC / Section | Requirement | Where enforced in ze | Why it constrains this spec |
|---------------|-------------|----------------------|-----------------------------|
| RFC 7296 §2.4 (liveness) | A peer is declared dead only after a liveness probe (empty INFORMATIONAL) goes unanswered past the timeout | `sendDPD` / `dpd.timedOut` / teardown at `internal/component/ike/engine/dpd.go:79-113`, `established.go:145-148` | The responder must go **silent**, so the stop primitive must remove it without letting it answer — SIGKILL, not SIGTERM |
| RFC 7296 §1.4 / §2.4 (clean close) | A peer that shuts down cleanly SHOULD send a DELETE payload in an INFORMATIONAL exchange | `sa.State == StateDead` branch, `established.go:139-143` | A clean DELETE tears the SA down via a DIFFERENT path than DPD; SIGKILL prevents it so the test proves DPD, not DELETE-handling (R-1) |
| RFC 7296 §2.3 (message-ID correlation) | Responses are correlated to their request by message ID | `dpdState.matchesProbe`, `dpd.go:30-32`; `established.go:118` | A killed responder sends no message-ID-matched reply, so `awaitReply` never clears and the timeout fires — the mechanism A-1 relies on |

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
