# Spec: harness-cannot-pass-on-its-own-failure

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Test tooling. Each owned row is a narrow shape in the functional runner, a fixture, ze-peer or the discrimination recorder; the common path still goes red on a product failure. The interop checker half of the same root cause blocks release proof and lives in `plan/release/validation/spec-interop-checkers-cannot-pass-on-an-empty-read.md` |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Root cause: the test machinery decides its verdict from a signal that its own
failure leaves untouched. A fixture that cannot reach the runner, a runner that
rebuilds the binary it was told to test, a reader that trusts a printed summary
over an exit code, an assertion span that a later command fills, and a
discrimination recorder that accepts a build failure as a red all end the same
way: the run reads green, or reads as proof, when the machinery under it did
not run the thing it reports on.

Why it matters: a green that could not have been red is false evidence. It
lets a broken feature pass, and it publishes RFC discrimination records that
prove nothing. Each occurrence was found by hand, one at a time; nothing stops
the next.

Goal: one rule, enforced where the verdict is made. The component that runs
the test owns the verdict, and it counts a pass only when it observed the
thing it ran succeed. Every failure of the machinery itself is a red with its
cause. A self-test corpus of deliberately broken runs stops the family coming
back.

Interop checker rows with the same root cause are owned by
`plan/release/validation/spec-interop-checkers-cannot-pass-on-an-empty-read.md`,
because those checkers carry the release evidence for shipped protocols.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - `.ci` runner contract, `ZE_BIN`, `LE_TEST_NO_BUILD`
  → Constraint: the page reads `ZE_BIN` as a pin; the runner does not respect it (Current Behavior)
- [ ] `docs/contributing/rfc-conformance-gates.md` - discrimination record contract
  → Constraint: a record is a recorded break under which the tagged unit goes red
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Decision: the self-test corpus below is the mechanical form of the revert-and-see-red walk

## Current Behavior (MANDATORY)

Source files read on 2026-10-23:
- [ ] `internal/test/fixture/fixture.go` - `ReportFailure` (line 311) writes the observer sentinel to process stderr and drops every call after the first through the package-global `reportedOnce atomic.Bool` (line 303). A fixture the daemon runs as a plugin writes it after `plugin.Run` returns, which is after the scenario asked the daemon to shut down, so nothing relays it
- [ ] `internal/test/fixture/fixture_test.go` - `readOneReport` resets `reportedOnce` before each case, so the test-order red is gone; the process-global latch itself remains
- [ ] `internal/test/runner/runner.go` - `(*Runner).Build` runs `go build -o r.zePath` whenever `LE_TEST_NO_BUILD` is unset, so an explicit `ZE_BIN` is overwritten with the shared tree's ze
- [ ] `internal/test/runner/runner_exec.go` - `runOrchestrated` keeps a mid-file foreground helper's assertion span open to the end of the run (2026-10-14 row); `routeStdinBlock` now routes a `-` correctly (row fixed)
- [ ] `internal/test/runner/record_parse.go` - `checkKeys` refuses an unknown key on `expect=stdout`, so `not-contains=` is no longer silently dropped (rows fixed)
- [ ] `internal/test/peer/peer.go` - `(*Peer).Run` turns a check peer stopped short into `ErrExpectationsUnmet` (row fixed)
- [ ] `internal/test/peer/open.go` - `buildOpen` now takes every sender value from `openIdentity` and `Config`; the OPEN mirror rows are fixed
- [ ] `internal/test/peer/reject.go` - `completed` with `option=linger` prints the success token, and a later rejection retracts it with `RejectionMarker`, which `failedCheckPeers` (`internal/test/runner/peer_contract.go`) reads (row fixed)
- [ ] `internal/le/rfc/discriminate.go` - `interopCitationState` checks that the cited `fail(N)` exists, never that it is the assertion the broken run failed
- [ ] `internal/le/rfc/discriminate_observe.go` - `observationRunner.attribution` returns no refusal for interop once a scenario is selected, so a mutant that failed to compile (`0 passed, 0 failed`) was recorded as proof of RFC7911-2-2

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test functional <suite>` and `./le test bgp plugin` run `.ci` files through `internal/test/runner`
- `./le rfc discriminate-record` runs a broken build through `observationRunner`

### Transformation Path
1. The runner builds or resolves ze, starts the daemon, fixtures and ze-peer, and collects each process's output
2. The runner reads exit codes, sentinels and peer tokens, and decides pass or fail per record
3. The discrimination recorder applies a break, runs the carrier, reads the run's outcome, and writes a record when the outcome is red

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner to fixture process | stderr sentinel `ZE-OBSERVER-FAIL` | [ ] |
| Runner to ze-peer | stdout tokens `successful` and `RejectionMarker` | [ ] |
| Recorder to carrier run | exit code and parsed assertion output | [ ] |

### Integration Points
- `failedCheckPeers` and the observer sentinel reader in `internal/test/runner`
- `observationRunner` and `interopCitationState` in `internal/le/rfc`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | [ ] | the verdict stays in the runner and the recorder |
| No unintended coupling | [ ] | fixtures report through the sink the runner hands them |
| No duplicated functionality | [ ] | one failure sink replaces the global latch and the late stderr write |
| Registration over hardcoding | [ ] | corpus cases register in one table the self-test walks |

## Fix design

One rule, applied at each verdict point: a pass requires a positive observation
by the component that ran the test, and any machinery failure is a red that
names it.

| Verdict point | Change |
|---------------|--------|
| Fixture failure | The runner hands each fixture a per-run failure sink (a file path in the run directory, named in the fixture's environment). `ReportFailure` appends to it at the moment of failure. The runner reads it after every process exits, whether or not the daemon relayed anything. The package-global latch is deleted |
| Binary under test | An explicit `ZE_BIN` is a pin: the runner never builds into it, and verifies it exists. A build happens only into the runner's own path |
| Tool verdicts | Where a gate reads a tool, the exit code decides and a printed summary is never read as the verdict. An exit code that disagrees with the summary is red and names both |
| Assertion span | A mid-file foreground helper's span closes when the helper exits, so a later command's output cannot satisfy its assertions |
| Capability-bound tests | A `.ci` whose feature needs a kernel capability declares it; the runner skips with the missing capability named, and never passes one whose install was refused |
| ze-peer session routing | `action=send:conn=N` under `conn_map` writes to session N, or refuses |
| Discrimination | A red counts only when the run reached the tagged unit and failed the assertion the record cites. A build failure, setup failure or zero-assertion run is refused with its cause |

The check that stops it coming back: a self-test corpus, one broken run per
shape above (fixture fails after daemon shutdown, `ZE_BIN` set to a broken
binary, tool exits non-zero with a clean summary, later output under a closed
span, refused kernel install, send to conn 2, mutant that does not compile).
Each case MUST go red, and the corpus runs with the runner's own package tests.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` with a plugin fixture that fails after shutdown | → | runner failure sink reader | `TestFixtureFailureAfterShutdownIsRed` |
| `ZE_BIN` naming a broken ze | → | `(*Runner).Build` pin arm | `TestExplicitZEBinIsNeverRebuilt` |
| `./le rfc discriminate-record` with a non-compiling mutant | → | `observationRunner.attribution` | `TestBuildFailureIsNeverDiscrimination` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A fixture run as a plugin fails after the scenario asked the daemon to stop | The record is red and its report carries the fixture's failure text |
| AC-2 | Two fixtures in one process each report a failure | Both failures appear in the report |
| AC-3 | `ZE_BIN` names a ze built from a broken tree, `LE_TEST_NO_BUILD` unset | The test runs that binary and goes red; the file at `ZE_BIN` is unchanged |
| AC-4 | A gate's tool exits non-zero and prints a clean summary (`0 issues.`) | The gate is red and names the exit code and the summary |
| AC-5 | A `.ci` asserts under a mid-file foreground helper and only a later command writes the expected text | The assertion fails |
| AC-6 | A kernel-install `.ci` runs where the install is refused | The test is reported skipped with the missing capability named, never passed |
| AC-7 | `action=send:conn=2` under `conn_map` with two sessions | The UPDATE arrives on session 2 |
| AC-8 | `discriminate-record` with a mutant that does not compile | The record is refused and the message names the build failure |
| AC-9 | `discriminate-record` citing `fail(6)` when the broken run failed at `fail(2)` | The record is refused and names both assertions |
| AC-10 | The self-test corpus | Every case is red; removing any one fix turns its case green, and that is recorded once as the red proof |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | Validation |
|----|------------|-------|------------|
| A-1 | A file sink in the run directory reaches the runner when stderr does not | fixtures already write into the run's work directory | AC-1 test |
| A-2 | Making `ZE_BIN` a pin breaks no caller that relied on the rebuild | QEMU runs already set `LE_TEST_NO_BUILD=1` with `ZE_BIN` | grep callers before the change |

### Risks
| Risk | Mitigation |
|------|------------|
| Closing helper spans turns existing `.ci` red | Re-read each affected `reject=` and `expect=` under a mid-file helper; a red is a test that was vacuous |
| Capability markers turn passing kernel tests into skips on unprivileged hosts | Intended: they never proved the install there |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFixtureFailureAfterShutdownIsRed` | `internal/test/runner/` | AC-1, AC-2 | [ ] |
| `TestExplicitZEBinIsNeverRebuilt` | `internal/test/runner/runner_test.go` | AC-3 | [ ] |
| `TestToolExitDisagreesWithSummaryIsRed` | the gate's package | AC-4 | [ ] |
| `TestHelperSpanClosesAtHelperExit` | `internal/test/runner/` | AC-5 | [ ] |
| `TestConnMapSendReachesNamedSession` | `internal/test/peer/` | AC-7 | [ ] |
| `TestBuildFailureIsNeverDiscrimination` | `internal/le/rfc/` | AC-8 | [ ] |
| `TestCitationMustMatchFailingAssertion` | `internal/le/rfc/` | AC-9 | [ ] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| harness self-test corpus | `test/harness/` | AC-6, AC-10: every broken run goes red | [ ] |

## Files to Modify
- `internal/test/fixture/fixture.go` - failure sink, delete `reportedOnce`
- `internal/test/runner/runner.go`, `runner_exec.go` - sink reader, `ZE_BIN` pin, helper span, capability skip
- `internal/test/peer/peer_connmap.go` - send routing
- `internal/le/rfc/discriminate.go`, `discriminate_observe.go` - attribution and citation checks
- `docs/functional-tests.md` - `ZE_BIN` pin, failure sink, capability declaration

## Files to Create
- `test/harness/` corpus cases and the table that lists them

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env var, doctor, metrics, BGP family | N-A | test tooling only |
| Functional test for new RPC/API | N-A | no RPC |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 9 | RFC behavior newly proven? | Yes | `docs/contributing/rfc-conformance-gates.md` (attribution rule) |
| 16 | Changed files referenced by doc anchors? | Yes | `docs/architecture/testing/ci-format.md` (helper span, capability declaration, failure sink); `docs/architecture/core-design.md` (declared by the runner files; update only if the harness section names the latch or the rebuild). Re-derive with `./le spec citation anchors spec plan/post-release/engineering/spec-harness-cannot-pass-on-its-own-failure.md` |
| 1-8, 11-15, 17 | Product surfaces | No | no product change |

## Implementation Steps
1. Write the corpus cases first; record each one green against today's tree (the defect).
2. Failure sink and latch deletion; AC-1, AC-2.
3. `ZE_BIN` pin; AC-3. Tool-exit rule; AC-4.
4. Helper span and capability skip; AC-5, AC-6. Re-read each `.ci` that turns red.
5. conn_map send routing; AC-7.
6. Discrimination attribution and citation; AC-8, AC-9.
7. Corpus all red; AC-10.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `green-that-could-not-have-been-red.md` | 2026-10-09 | runner builds into `ZE_BIN`, overwriting the broken ze it was given |
| `green-that-could-not-have-been-red.md` | 2026-09-02 | six kernel-FIB `.ci` with no capability marker pass where install is refused |
| `green-that-could-not-have-been-red.md` | 2026-08-22 | `./le changed scope` printed `0 issues.` and exited 7 |
| `green-that-could-not-have-been-red.md` | 2026-08-29 | `ReportFailure` from a plugin fixture never reaches the runner |
| `green-that-could-not-have-been-red.md` | 2026-09-05 | shared lint verdict read from the summary, not the exit |
| `green-that-could-not-have-been-red.md` | 2026-10-14 | `runOrchestrated` mid-file helper span stays open |
| `green-that-could-not-have-been-red.md` | 2026-10-21 | `interopCitationState` checks the cited assertion exists, not that it failed |
| `green-that-could-not-have-been-red.md` | 2026-10-23 | `observationRunner.attribution` accepts a build failure as proof |
| `test-against-broken-path.md` | 2026-08-15 | ze-peer `action=send:conn=2` under `conn_map` writes to the first session (not re-read today) |

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence today |
|------------|------|-----------------|----------------|
| `green-that-could-not-have-been-red.md` | 2026-09-06 | `.ci` stdin `-` rewrite | `routeStdinBlock` decides once; only the daemon config arg gets the file (`runner_exec.go` 755-762) |
| `green-that-could-not-have-been-red.md` | 2026-08-31 | `expect=stdout:not-contains=` two parsers | `checkKeys("expect=stdout", kv, "pattern", "contains")` refuses the key (`record_parse.go`) |
| `green-that-could-not-have-been-red.md` | 2026-09-05 | `parseExpect` drops `not-contains=` | same `checkKeys` refusal |
| `green-that-could-not-have-been-red.md` | 2026-09-11 | check peer accept arm answers success | `(*Peer).Run` returns `ErrExpectationsUnmet` unless the checker completed (`peer.go` 476-487) |
| `green-that-could-not-have-been-red.md` | 2026-09-08 | graceful-restart `.ci` over mirrored code-64 | `buildOpen` resolves ze-peer's own families (`open.go` 519-528); product half is in `unwired-feature.md` |
| `green-that-could-not-have-been-red.md` | 2026-10-05 | stress repro counted attached results | stress run accepts only `KindInside` or `KindClaimed`, children run in the claimed slot (`stressrepro/run.go` 189-198) |
| `green-that-could-not-have-been-red.md` | 2026-09-03 | `opBIRDRouteAbsent` could never be green | reads the whole table, a failed query is never absence (`check_engine.go` 160) |
| `green-that-could-not-have-been-red.md` | 2026-08-22 | `_observer_excepthook` in `ze_api.py` | `test/scripts/ze_api.py` deleted |
| `green-that-could-not-have-been-red.md` | 2026-08-23 | `ospf-sr-frr` `check.py` fell through to pass | `check.py` retired; native scenario uses `opWaitContains` |
| `test-against-broken-path.md` | 2026-09-07 | `generateOpen` mirrors ze's OPEN | `generateOpen` gone; `buildOpen` takes sender values from `openIdentity` |
| `test-against-broken-path.md` | 2026-09-07 | `ebgp-encode.ci` Bad Peer AS | same `buildOpen` change |
| `test-against-broken-path.md` | 2026-09-06 | BMP sender `.ci` Bad Peer AS | same; the row's own cell says it no longer reproduces |
| `test-against-broken-path.md` | 2026-09-02 | `TestRunConvertsPanicToObserverFailure` order | `readOneReport` resets the latch (`fixture_test.go` 22-24) |
| `test-against-broken-path.md` | 2026-08-22 | `attach-process-reload.ci` observer decode | `ze_api.py` deleted; the fixture is Go now |
| `test-against-broken-path.md` | 2026-08-23 | `config-edit-ssh.ci` observer decode | same |
| `balance-assertion-vacuous-without-a-loan.md` | 2026-08-16 | `option=linger` beside `reject=bgp` | a rejection while lingering prints `RejectionMarker`, which `failedCheckPeers` reads (`reject.go` 181-183, `peer_contract.go` 151) |

## Related specs
- `plan/post-release/engineering/spec-harness-fail-open-guard-backlog.md` (skeleton) holds nine historical observations; its row C is the `ospf-sr-frr` row above. This spec does not reopen them.
- `plan/post-release/engineering/spec-testing-latched-counter-vacuity-trap.md` owns the latched-counter trap, a test-authoring shape, not a machinery failure.
- `plan/post-release/engineering/spec-ci-peer-harness-contract-backlog.md` owns the AS4_PATH matcher question; no overlap with AC-7.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] Scoped tests pass: `./le job run label harness command go test ./internal/test/... ./internal/le/rfc/...`, then `./le test functional plugin` and `./le verify current fast`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests N-A: tooling with no protocol peer

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
