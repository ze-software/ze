# Spec: fixit-plugin-concurrency-is-pinned-to-a-ci-constant

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 3/4 |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Why this does not block the first release (2026-08-23)

Moved from `plan/` on the owner's standing instruction that a genuine improvement
belongs here rather than in the release backlog.

Nothing this spec changes is a defect in the shipped product. It changes how long
a test suite takes on a host larger than the one the constant was measured on.
No wire byte moves, no configuration is accepted and then ignored, no
authentication is skipped, no route is lost, and nothing leaks
(`plan/future/README.md` lists what a defect is).

**One finding inside it IS a defect, and it does not travel with the spec.** The
measured failure cluster at high concurrency is an IN-TEST deadline: `runMCP`
(`internal/test/cli/cmd_mcp.go`) defaults `-timeout` to 10s and `waitReady`
(`internal/test/cli/cmd_mcp_client.go`) produces the message. `ParallelTimeoutHeadroom`
(`internal/test/runner/parallel.go`) widens the RUNNER's budget and never reaches
an in-test deadline, so a test fails for the harness rather than for the product.
That is a test wrong about what it asserts. It belongs in `plan/`, and the reason
it is named here is so a later reader does not assume this whole file was parked.

## Task

`ZE_PLUGIN_PARALLEL ?= 8` and `ZE_ENCODE_PARALLEL ?= 8` (`internal/le/functional/suites.go`)
are constants chosen for a 4-vCPU CI runner and applied unchanged to every host.
On a 32-core box that costs the `plugin` suite about 400 seconds per run.

`SuiteConcurrencyFloor`'s comment names the provenance: 8 "is the value
`ZE_PLUGIN_PARALLEL` has been running the 530-test plugin suite at on GitHub's
4-vCPU hosted runner". It is a measured survivable figure for the smallest host
this project builds on, pinned as the value for the largest.

**Measured**, the retired `ze-functional-plugin-test ZE_PLUGIN_PARALLEL=N` (current: `./le functional plugin`), seven runs
on a 32-core box, against the suite's 4545s sum of per-test medians:

| N | suite | speedup | parallel efficiency | pass |
|---|-------|---------|---------------------|------|
| 8 | 589.5s | 7.7x | 96% | 664/665 |
| 16 | 322.5s | 14.1x | 88% | 665/665 |
| 32 | 216.5s / 166.0s | 23.8x | 74% | 659, 660 |
| 64 | 196.5s | 23.1x | 36% | 654/665 |

The curve flattens AT the core count. `plugin` is core-bound: 64 sits inside the
two-run spread at 32, buys nothing measurable, and costs pass rate. The floor is
close behind -- the slowest single test is 96.3s plus about 20-25s of
isolated-binary build -- so 166s is already within 1.4x of what the suite can
reach.

**One failure cluster is attributable to concurrency, and it gates the change.**
Every red at N=32 in one run was `MCP server not ready: no listener on
127.0.0.1:<port> after 10s`. `runMCP` (`internal/test/cli/cmd_mcp.go`) defaults
`-timeout` to 10s and `waitReady` (`internal/test/cli/cmd_mcp_client.go`)
produces that message. It is an IN-TEST deadline, so `ParallelTimeoutHeadroom`
(`internal/test/runner/parallel.go`) never reaches it -- that widens the
RUNNER's per-test budget, and exists precisely because ".ci timeouts sit at
70-100% of the uncontended runtime". Count per run, at N = 8/16/32/64/8/8/32:
**1 / 0 / 6 / 5 / 2 / 0 / 0**.

**→ Correction (2026-08-19, main thread), and it narrows this spec.** The
recommendation first put to the owner was to change `DefaultSuiteConcurrency`
from `2*runtime.NumCPU()` to `runtime.NumCPU()`. That was reasoning from the
wrong suite and it is NOT in this spec. `plugin` never calls
`DefaultSuiteConcurrency`: it runs through the bgp runner, whose default is
`DefaultParallelConcurrent` (`internal/test/cli/cmd_bgp.go`), overridden by the
makefile. `DefaultSuiteConcurrency` governs the 22 `registerCIRoot` suites,
which the sweep never measured -- and its 2x is deliberate, with an incident
behind it: its comment records that on 2026-07-26 a GitHub job died mid-`ospf`
with the runner agent killed (exit 143) because "unset" meant "all at once" and
launched 97 daemons, and that scaling at 2x lets a large host keep approximating
all-at-once. That is the right shape for a WAIT-bound suite, which `ospf` looks
like. Changing it on `plugin`'s evidence would risk the suites the evidence does
not cover. Left alone; a separate measurement would be needed to touch it.

## → Finding (2026-09-05, implementation session): the code shipped before this spec was worked

Both code phases are IN THE TREE and were verified against source on 2026-09-05.
This spec was written on 2026-08-20 against a makefile that no longer exists, and
the port to `internal/le/functional` carried the change with it. Nothing here was
re-designed: the finding is that the work is done, and what is left is the
MEASUREMENT the spec owes.

| Phase | Landed in | Producer today |
|-------|-----------|----------------|
| 1, the MCP deadline | `22a1fd952`, 2026-08-20, the commit that also added this spec | `cmdMcp` (`internal/test/cli/cmd_mcp.go`) multiplies `-timeout` by `runner.ChildParallelFactor`; `(*Runner).parallelFactorEnv` (`internal/test/runner/runner_exec_util.go`) publishes it |
| 2, the derived concurrency | `1b87bdbc7` and `da417ecc4`, 2026-08-25, as part of the make-to-`le` port | `Parallel` and `cores` (`internal/le/functional/budget.go`), reached by `Suite.Command` (`internal/le/functional/suites.go`) for a `Scaled` suite |
| 4, the docs | already on `docs/functional-tests.md`, "How a suite's concurrency is chosen" | that section, its table and its source anchors |

**Phase 3, the measurement, is what stays open.** AC-2 and AC-4 each need repeated
runs of the `plugin` and `encode` suites, and the owner refused a suite run in this
session because another session holds the box for the ExaBGP compatibility work.
A-1 and A-2 stay `unvalidated` for that reason, and neither is a code gap.

Two page defects were repaired here, both from the make-to-`le`-to-Go port naming
a symbol that no longer produces the value: `docs/functional-tests.md` cited
`internal/le/functional.Answer` as the source of the derived `-p`, where `Parallel`
(`internal/le/functional/budget.go`) produces it, and cited a `BUDGET_DEFAULTS`
entry in the same dispatcher, where `budgetDefaults` in `budget.go` holds it.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - the suites and their budgets
  → Constraint: a per-suite budget is a wall-clock cap, not a concurrency knob
- [ ] `docs/architecture/testing/ci-format.md` - the Design page `internal/le/functional/budget.go` declares
  → Constraint: the budget and the derived `-p` are two answers from one suite table, so a second list of suites drifts from it
- [ ] `ai/rules/testing.md` - what a suite owes
  → Constraint: raising a timeout is not a fix on its own

**Key insights:**
- `ParallelTimeoutHeadroom = 3` already absorbs contention for the runner's per-test budget, when concurrency > 1. The MCP deadline is the shape it cannot reach, because it lives inside the test binary.
- Nothing in `internal/test/` reads `GO_TEST_PROCS` or `ZE_RUN_SLOTS`: grep returns zero. A suite sizes for the whole box while `internal/le/job/job.go` admitted the job on a quarter-box budget. This spec does not close that; it is recorded as a limitation.
- 90 tests report SKIP on this host (`ospf` alone 29 of 82), which is why `ospf` finishes 74 tests in 45s against a 2653s sum of medians recorded elsewhere. A suite measured only here is not measured everywhere.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/functional/budget.go` - `ParallelFloor`, `cores`, `Parallel`: the derivation `ZE_PLUGIN_PARALLEL` and `ZE_ENCODE_PARALLEL` override
- [ ] `internal/le/functional/suites.go` - `Suite.Scaled` and `Suite.Command`, which put the derived `-p` on the command line
- [ ] `internal/test/cli/cmd_bgp.go` - the `-p` flag default for the five bgp-runner suites
- [ ] `internal/test/runner/parallel.go` - `SuiteConcurrencyFloor`, `DefaultSuiteConcurrency`, `ParallelTimeoutHeadroom`, `parallelFactor`
- [ ] `internal/test/cli/cmd_mcp.go`, `internal/test/cli/cmd_mcp_client.go` - the 10s readiness deadline and `waitReady`

**Behavior to preserve:**
- `DefaultSuiteConcurrency` is UNCHANGED. See the Correction above.
- `reload` stays at `-p 1` and `managed` at 1: `register.go` records that they share the kernel routing table. `vpp` stays at 1.
- The 4-vCPU CI runner keeps 8, through the floor.
- `ZE_SUITE_TIMEOUT_PLUGIN` stays at 1500s. The worst run measured was 751.5s on a busy box, half the budget.
- Every `-p` stays overridable from the command line, and `-p 0` still means all.

**Behavior to change:**
- `ZE_PLUGIN_PARALLEL` and `ZE_ENCODE_PARALLEL` derive from the host instead of being pinned at 8.
- The MCP readiness deadline scales with contention the way the runner's budget does.

## Data Flow (MANDATORY)

### Entry Point
- `./le functional plugin`, `./le functional encode`, and the aggregate `./le functional`.

### Transformation Path
1. The makefile computes the suite's `-p` and passes it.
2. `cmd_bgp.go` uses it in place of `DefaultParallelConcurrent`.
3. `parallelRunner` runs that many tests at once and widens each per-test budget by `ParallelTimeoutHeadroom`.
4. A test carrying its own in-binary deadline does not see that widening.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `le functional` ↔ bgp runner | the `-p` flag | Yes -- `TestParallelIsFlooredAndOverridable` and `TestScaledSuitesCarryTheDerivedConcurrency` drive `Suite.Command`, which is the argv the run executes. The makefile the row named was retired in `eae282592` |
| runner ↔ per-test budget | `parallelFactor` | Yes -- `withParallelHeadroom`, unchanged |
| runner ↔ an in-test deadline | `ze.test.parallel-factor` in the child environment | Yes -- `mcp-parallel-factor-published.ci` (producer), `TestMCPReadinessScalesWithConcurrency` (consumer) |

### Integration Points
- `parallelFactor`, which the MCP readiness wait should read.
- `SuiteConcurrencyFloor`, the existing floor and the reason CI is unaffected.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The child reads the factor through `internal/core/env`, and `(*Runner).parallelFactorEnv` sits in the same `childEnv` list as `testBudgetEnv` and `pluginStageStallEnv` |
| No unintended coupling (components stay isolated) | Yes | `internal/test/cli` already imports `internal/test/runner`; nothing new points the other way |
| No duplicated functionality (extends existing, does not recreate) | Yes | `ChildParallelFactor` reads what `(*Runner).parallelFactor` publishes: one number, two readers. The makefile floor is held equal to `runner.SuiteConcurrencyFloor` by a test rather than restated in prose |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding. The one string built uses `textbuf.Buffer` |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The env key registers through `env.MustRegister`. No switch, no per-suite branch |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Scaling the MCP deadline removes that failure cluster | the cluster is exactly that message and its count tracks N | 32 stays flaky for another reason | AC-2: the message's count stays 0 across repeated runs at 32 | unvalidated. The mechanism is proven: `cmdMcp` (`internal/test/cli/cmd_mcp.go`) multiplies its `-timeout` by `runner.ChildParallelFactor`, and `mcp-parallel-factor-published.ci` proves the factor crosses the process boundary. The COUNT is not proven. No suite run since 2026-08-25 is recorded, and no `plan/known-failures/` shard or `plan/journal/` row names that message |
| A-2 | `encode` behaves like `plugin` | both run through the bgp runner and both were pinned at 8; `encode` has 57 tests and a 6.6s median | `encode` regresses where `plugin` gains | AC-4 measures `encode` separately, not by analogy | unvalidated. AC-4 is a measurement, and no after-measurement exists for either suite |
| A-3 | The derived value does not starve concurrent sessions | the box admits four such jobs; 32 concurrent daemons each is 128 | the box thrashes and every session slows | AC-5, and the limitation is recorded rather than closed | confirmed as arithmetic, unmeasured as behavior. `defaultSlots` (`internal/le/job/job.go`) answers `runtime.NumCPU() / gotoolchain.CoresPerJob()`, which is 32/8 = 4 on this box, and `Parallel` (`internal/le/functional/budget.go`) answers 32. Four admitted suites therefore start 128 tests on 32 cores, and `docs/functional-tests.md` now carries that number. Whether the box thrashes at that load is unmeasured |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The speedup lands as flakes | the failing set grows at the derived value | The MCP deadline is fixed FIRST and proven before the concurrency changes |
| R-2 | A small host regresses | CI wall clock rises | The floor keeps 8 where cores are few; AC-3 pins it |
| R-3 | The gain is measured on a quiet box and vanishes on a busy one | the sweep's own baselines spread 1.49x | Repeat the after-measurement, and report the spread rather than one number |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two functional suites get slower or flakier. No product behavior changes |
| How is it reverted? | Single commit revert; the constants return |
| Who else touches this path? | Every session running the plugin or encode suite, and CI |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an MCP test under concurrency | → | the scaled readiness deadline | `TestMCPReadinessScalesWithConcurrency`, `mcp-ready-under-load.ci` |
| the runner exec'ing any `cmd=` child | → | `ze.test.parallel-factor` in its environment | `TestParallelFactorEnvPublishesTheRunnerFactor`, `mcp-parallel-factor-published.ci` |
| `./le functional plugin` | → | the derived `ZE_PLUGIN_PARALLEL` | `TestParallelIsFlooredAndOverridable`, `TestScaledSuitesCarryTheDerivedConcurrency` |
| a 4-vCPU host | → | the floor | `TestParallelIsFlooredAndOverridable` (cores 1, 7 and 8 all answer 8), `TestConcurrencyFloorIsTheRunnersOwn` |
| `reload`, `managed`, `vpp` | → | their recorded serial setting | `TestSerialSuitesStaySerial` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The MCP readiness wait runs under concurrency > 1 | Its deadline is widened by the same factor the runner applies to a per-test budget, not replaced by a larger constant |
| AC-2 | `./le functional plugin` at the derived value, repeated at least three times | `MCP server not ready` appears zero times across every repeat |
| AC-3 | A host with 4 cores | `ZE_PLUGIN_PARALLEL` and `ZE_ENCODE_PARALLEL` are still 8. CI is unchanged |
| AC-4 | `plugin` and `encode`, before and after, same host | Each is measured SEPARATELY and neither regresses. `encode` is not assumed to follow `plugin` |
| AC-5 | The derived value on this 32-core host | It is at most the core count, and the spec says what 32 concurrent daemons cost when four such jobs are admitted at once |
| AC-6 | `reload`, `managed`, `vpp` | Still 1, for their recorded reasons |
| AC-7 | `-p` given explicitly on the command line | The explicit value still wins |
| AC-8 | `DefaultSuiteConcurrency` | Unchanged, and a test pins it so this spec cannot drift into the 22 suites it did not measure |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMCPReadinessScalesWithConcurrency` | `internal/test/cli/cmd_mcp_test.go` | AC-1 (consumer) | PASS. Reverting the scaling reports `after 150ms` for the 3x case |
| `TestParallelFactorEnvPublishesTheRunnerFactor` | `internal/test/runner/runner_exec_util_test.go` | AC-1 (producer) | PASS. Publishing a constant makes one of its two cases disagree |
| `TestDefaultSuiteConcurrencyIsBounded` | `internal/test/runner/parallel_test.go` | AC-8 | PASS. It already pinned `max(SuiteConcurrencyFloor, 2*NumCPU)` exactly, so a second test asserting the same expression would be a duplicate. Its doc comment now carries this spec's reason for leaving the value alone |
| `TestParallelIsFlooredAndOverridable` | `internal/le/functional/functional_test.go` | AC-3, AC-5, AC-7 | PASS 2026-09-05 |
| `TestConcurrencyFloorIsTheRunnersOwn` | `internal/le/functional/functional_test.go` | AC-3 | PASS 2026-09-05. Holds `ParallelFloor` equal to `runner.SuiteConcurrencyFloor`, which is what the retired regex over `parallel.go` did |
| `TestSerialSuitesStaySerial` | `internal/le/functional/functional_test.go` | AC-6 | PASS 2026-09-05, WRITTEN in this session: the port to Go dropped the retired `test_serial_suites_stay_serial` and left AC-6 with no test. Recorded red: making `reload` `Scaled` reports `suite reload ends with [-p 32], want [-p 1]` |
| `TestScaledSuitesCarryTheDerivedConcurrency` | `internal/le/functional/functional_test.go` | AC-6 | PASS 2026-09-05. Pins which suites the derivation reaches at all |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| derived `ZE_PLUGIN_PARALLEL` | 8-NumCPU | NumCPU | 8 is the floor, never below | the core count, where efficiency measured 36% |
| MCP readiness deadline | 10s and up | scaled by `parallelFactor` | 10s under concurrency, which is the defect | N/A |

<!-- The floor is a hard lower bound: a small host must still get 8, the
     measured survivable figure on the smallest host this project builds on. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-ready-under-load` | `test/plugin/mcp-ready-under-load.ci` | An MCP test does not fail for want of readiness when the box is busy | PASS. It sets the factor itself, because the inherited value is 1 under a single-test run and 3 under the suite, and a test whose expected value changes with the way it is run cannot assert one |
| `mcp-parallel-factor-published` | `test/plugin/mcp-parallel-factor-published.ci` | The runner hands the factor to every child it execs | PASS. Deleting `r.parallelFactorEnv()` from `runner_exec.go` and rebuilding turned it red (2026-08-19); nothing else crosses that process boundary |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Test infrastructure. No wire-visible behavior changes | |

## Files to Modify
- `internal/test/cli/cmd_mcp.go`, `internal/test/cli/cmd_mcp_client.go` - the readiness deadline
- `internal/le/functional/suites.go` - `ZE_PLUGIN_PARALLEL` and `ZE_ENCODE_PARALLEL` derived
- `docs/functional-tests.md` - how a suite's concurrency is chosen, and why the floor exists

## Files to Create
- `test/plugin/mcp-ready-under-load.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test infrastructure only |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | The `-p` and `-timeout` flags exist and keep their meaning |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC added |
| Pipe completeness | N-A | No `ze` CLI output added |
| Env var registration | N-A | Make variables, not `ze.*` leaves |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Test infrastructure |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No suite leaves `all_suites`, so no `.ci` tier moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep for anchors naming `test-functional.mk`, `cmd_mcp.go`, `parallel.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/functional-tests.md` documents the per-suite targets |

## Implementation Steps

1. **Phase: The MCP deadline FIRST** -- R-1 says the speedup lands as flakes otherwise
   - Scale it by the same `parallelFactor` the runner applies to a per-test budget. A bigger constant is NOT the fix: the point is that it tracks contention (`ai/rules/completion.md`, a generous timeout is a synonym for an unknown one)
   - Tests: `TestMCPReadinessScalesWithConcurrency`, `mcp-ready-under-load.ci`
   - Verify: run `plugin` at 32 at least three times and count the message. Zero, or phase 2 does not land
2. **Phase: Derive the two constants** -- floored at `SuiteConcurrencyFloor`, capped at the core count
   - Tests: `TestPluginConcurrencyIsDerivedNotPinned`, `TestSmallHostKeepsTheFloor`, `TestSerialSuitesStaySerial`, `TestExplicitParallelWins`, `TestSuiteConcurrencyDefaultIsUnchanged`
3. **Phase: Measure both suites** -- AC-4, `plugin` and `encode` separately, before and after, repeated
   - **If either regresses, say so and stop.** Report the spread, not one number: the sweep's own baselines spread 1.49x on a shared box
4. **Phase: Docs** -- how concurrency is chosen, why the floor exists, and that the runner does not read the job budget

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Only the two measured suites change. `DefaultSuiteConcurrency` is untouched and pinned by a test |
| Correctness | The MCP fix tracks contention rather than raising a number |
| Naming | One expression for the derived value, used by both suites |
| Data flow | The floor still wins on a small host, so CI is untouched |
| Rule: `ai/rules/testing.md` | No suite leaves `all_suites` and no `.ci` tier moves |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The flake vector is closed | `MCP server not ready` count across three runs at the derived value |
| Concurrency derives | `TestPluginConcurrencyIsDerivedNotPinned` |
| Neither suite regresses | per-suite wall clock, before and after, with the spread |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | 32 concurrent daemons on a box admitting four such jobs is 128 daemons. State the cost even though closing the gap between the runner and the job budget is separate work |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The sweep refuted the hypothesis that prompted it. `ospf` runs 74 tests with a 52s median in 45s of wall clock, which looks wait-bound and suggested `plugin` might keep paying past the core count. It does not: efficiency falls from 96% to 36% between 8 and 64. The `ospf` figure is explained by 29 of its 82 tests skipping on this host.
- A measurement of one suite does not license a change to a shared default. `plugin` never reaches `DefaultSuiteConcurrency`, and the first version of this recommendation would have changed it anyway.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derive the two makefile constants only | change `DefaultSuiteConcurrency` to 1x cores | `plugin` never calls it, and its 2x has a recorded CI incident behind it. See the Correction |
| Cap at the core count | 2x cores, as the other default does | 64 measured 36% efficient and cost pass rate on this suite |
| Fix the MCP deadline before raising concurrency | raise first, chase flakes after | The cluster is measured and attributable; a speedup delivered as flakes discredits the change |
| Scale the deadline, do not enlarge it | `-timeout 30s` | The runner already has the factor, and a larger constant is a synonym for an unknown one |

## Known Limitations
- The runner still does not read `GO_TEST_PROCS` or `ZE_RUN_SLOTS`, so a suite sizes for the whole box while its job was admitted on a quarter of it.
- Measured on one 32-core host where 90 tests SKIP. A capability-carrying host may behave differently, and this spec cannot answer for it.
- The 22 `registerCIRoot` suites keep `2*NumCPU`. Whether that is right for them is unmeasured.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes, or `./le verify current mode full` over the shared checkout. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
