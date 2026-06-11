# Spec: functional-test flakiness under host load

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-06-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/testing/ci-format.md` - runner and timeout semantics
4. `scripts/dev/verify-lock.sh` - what the lock does and does not cover

## Task

During the 2026-06-10 structural-review session, four consecutive
`make ze-verify` cycles each failed on something different; the last failed
ONLY in the functional stage, on three `test/plugin` cases (`1 add-remove`
reported as group `plugin:unknown:add`, `4 announce` and `11 api-doctor-check`
as timeouts) that then passed in isolation in 7 seconds total
(`bin/ze-test bgp plugin 1 4 11` -> 3/3 PASS). The box was running
overlapping verify cycles, subagent `go test` invocations against large
package trees, and an interactive session at the time.

Investigate WHY functional tests fail under host load when the code is not
broken, and implement prevention so a loaded machine produces either a green
gate or a failure that is self-evidently environmental, never an ambiguous
red that costs a debugging round-trip.

Investigation questions (answer all with evidence before designing fixes):

1. Which resource actually starves: CPU (wall-clock timeouts on slow
   spawn/handshake), the shared Go build cache, `bin/ze` rebuild races,
   TCP port collisions, or `tmp/` workspace collisions between concurrent
   runners?
2. Why was test 1 classified `plugin:unknown:add` rather than a timeout --
   what did the runner actually observe (reproduce; the stage logs from the
   original run were overwritten by later cycles)?
3. The verify lock (`scripts/dev/verify-lock.sh`) serializes `make
   ze-verify*` invocations but deliberately does not cover raw `go test` or
   `bin/ze-test` (documented in `ai/rules/git-safety.md`). Is that gap the
   trigger, or is load from non-test processes sufficient on its own?
4. The runner keeps an EMA timing baseline (`tmp/test-timings.json`) with
   auto-timeouts at a multiple of baseline. Does a loaded run UPDATE the
   baseline (amplifying or masking future timeouts) or only read it? Should
   baseline updates be suppressed on contended runs?
5. Do per-test timeouts measure wall clock where a readiness signal or CPU
   time would be the honest signal?

Prevention candidates to EVALUATE (choose with evidence; do not implement
all blindly):

- Record host load context (load average, concurrent ze/ze-test/go-test
  process count) in every failure group so environmental failures are
  machine-classifiable; the verify summary says "contended run" explicitly.
- Scale or gate timeouts by observed load, or replace the load-sensitive
  wall-clock waits with readiness signals where one exists.
- Extend the lock (or a lighter advisory warning) to `bin/ze-test` suite
  invocations so a verify run cannot silently overlap a manual suite run.
- A documented "contended verdict" path: when the gate fails AND load was
  abnormal, the failure index names the contention next to the isolation
  rerun commands it already provides.
- Policy boundary (Thomas, standing): the project rejects retry-on-failure
  flake masking. Keep that stance; prevention comes from isolation and
  classification, never retries.

## Investigation Answers

### Q1: Which resource starves?

**CPU (wall-clock timeouts).** Evidence:

- All test timeouts use `context.WithTimeout(ctx, timeout)` which is pure wall-clock
  (`runner_exec.go:233`, `runner_exec.go:481`).
- `SuggestedTimeout` derives per-test timeouts as `max(5s, 5x EMA_avg)` capped by
  fallback 20s (`timing.go:135-151`). On a quiet machine, plugin tests complete in
  ~1-2s, giving a timeout of ~5-10s. Under CPU starvation, ze-daemon startup alone
  can exceed this.
- Port collisions: **ruled out**. `ports.go` uses advisory flock + TCP bind-probe
  (`reservePortLocks` + `isPortRangeFree`), properly coordinating concurrent runners.
- `bin/ze` rebuild races: **secondary, already mitigated**. `runner_exec.go:745-762`
  retries ETXTBSY 3 times with 10ms sleep. The race exists but the retry handles it.
- `tmp/` workspace collisions: **diagnostic casualty only**. Stage logs in `tmp/verify/`
  get overwritten by subsequent cycles but this doesn't cause failures.
- Go build cache: **amplifier, not root cause**. Concurrent `go test` sessions contend
  on the build cache, consuming CPU and I/O that starves the functional runner.

### Q2: Why `plugin:unknown:add`?

**Near-miss timeout that the runner cannot classify.** Traced the path:

1. `runner_exec.go:393-396`: `testCtx.Err()` was nil (context deadline had NOT expired).
2. `runner_exec.go:409`: peer output did NOT contain `"successful"` (exchange incomplete).
3. `runner_exec.go:446-453` `default` branch: peer output had no `"mismatch"` (requires a
   wrong message, not no message) and no `"connection refused"`. Sets `FailureType = "unknown"`.
4. `failure_group.go:117-125` `recordFailureKind`: `FailureType` is `"unknown"`.
5. `failure_group.go:129-133` `subsystemPrefix("add-remove")`: extracts `"add"` (first
   segment before `-`).
6. Group key assembled as `plugin:unknown:add`.

The ze-daemon was too CPU-starved to complete the BGP exchange. ze-peer's internal
message-wait gave up, peer exited without printing "mismatch" or "successful", but the
test context's wall-clock deadline hadn't quite expired. There is no `"near-timeout"`
or `"incomplete-exchange"` classification.

### Q3: Verify lock gap

**The gap is a contributor but not the sole trigger.**

The verify lock (`scripts/dev/verify-lock.sh`) serializes `make ze-verify*` via flock.
It deliberately does NOT cover `bin/ze-test` or raw `go test` (documented scope).

On 2026-06-10: six subagent `go test` sessions ran alongside the verify cycle. The lock
serialized verify-vs-verify but allowed massive concurrent CPU load from the subagent
sessions. However, sufficiently heavy non-test processes (IDE indexing, compilation)
would produce the same symptom. The lock gap amplified the problem; CPU starvation is
the mechanism.

### Q4: Loaded run and baseline updates

**A loaded run DOES update the baseline, corrupting it.**

`parallel.go:296-300`: timed-out tests are excluded (`State != StateTimeout`), but
passing-but-slow tests are recorded. Under load:

- A test completing in 2x normal time gets EMA'd with alpha=0.3, inflating the average.
- `MaxMs = math.Max(entry.MaxMs, ms)` only grows; one loaded run permanently raises it.
- `SuggestedTimeout = max(5s, 5x EMA)`: inflated EMA loosens future timeouts.
- `IsSlow = duration > 2x EMA`: inflated EMA suppresses slow detection.
- Stress mode (`RunWithCount`) sets `SkipTimings = true`, but normal verify does not.

**Baseline updates should be suppressed when the run is contended.**

### Q5: Wall clock vs readiness signals

**Wall clock only.** All test timeouts measure total elapsed wall-clock via
`context.WithTimeout`. Readiness signals exist but only for startup synchronization:

- `syncWriter.WaitFor("listening on")`: 5s wall-clock cap on ze-peer readiness.
- `waitReady()`: polls for readiness file before writing daemon.pid.
- `http=wait:`: polls for HTTP endpoint readiness.

The overall test timeout does not track progress. Under CPU starvation, the test IS
making progress (just slowly), but the wall-clock deadline fires regardless. The honest
signal would be "no progress for X seconds" (per-message progress tracking), but that
requires ze-peer protocol changes and is a larger scope than this spec.

### Prevention Evaluation

| Candidate | Verdict | Rationale |
|-----------|---------|-----------|
| Record host load context in failure groups | **Implement** | Low risk, high diagnostic value. Adds load_avg + process count to FailureGroup; verify summary labels "contended" runs. Zero quiet-machine behavior change. |
| Suppress baseline updates on contended runs | **Implement** | Low risk, prevents slow corruption. When load exceeds threshold at run start, skip timings.Record/Save. Same load gate as above. |
| Classify "near-timeout" distinctly | **Implement** | Low risk, fixes Q2. When peer exits with error and elapsed > 80% of timeout, classify as "near-timeout" instead of "unknown". |
| Scale timeouts by load | **Defer** | Medium risk, complex. Load can change during a test. Better to classify and suppress baseline than to make timeouts adaptive. |
| Replace wall-clock with readiness signals | **Defer** | High scope. Requires ze-peer protocol changes (per-message progress). Out of scope for this spec. |
| Extend lock to bin/ze-test | **Implement (advisory)** | Low risk. Emit a visible warning when verify detects concurrent ze-test/go-test processes. Not a blocking lock (would break manual debugging). |
| Contended verdict path | **Implement** | Low risk, high value. When gate fails AND load was abnormal, the failure index names the contention next to rerun commands. |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations. -->
- [ ] `docs/architecture/testing/ci-format.md` - timeout directives, runner semantics
  -> Constraint: option=timeout overrides auto-timeout; per-test timeout is wall-clock via context.WithTimeout
- [ ] `docs/architecture/testing/runner-architecture.md` - parallel runner, timing baseline
  -> Constraint: Runner.Run delegates scheduling to ParallelRunner; timings recorded for non-timeout tests; timed-out tests excluded from baseline
- [ ] `ai/rules/testing.md` - no-retry policy, iteration workflow, sleep ratchet
  -> Constraint: failures are never auto-retried; do not weaken this

### RFC Summaries (MUST for protocol work)
- Not applicable (test infrastructure, no protocol work).

**Key insights:**
1. All test timeouts are wall-clock (context.WithTimeout). CPU starvation makes tests
   fail not because they are wrong but because the OS scheduler can't give them CPU.
2. The "unknown" failure classification is a near-miss timeout: the peer gave up waiting
   for messages but the context deadline hadn't expired yet, so StateTimeout was never set.
3. Loaded runs pollute the timing baseline: passing-but-slow tests inflate the EMA (alpha=0.3),
   which loosens future SuggestedTimeout and suppresses IsSlow detection.
4. Port collisions are not the trigger; advisory flock + bind-probe in ports.go handles them.
5. The verify lock serializes verify-vs-verify but allows unbounded go test concurrency from
   subagent sessions, which was the dominant load source on 2026-06-10.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE designing)
- [ ] `internal/test/runner/parallel.go` - concurrency cap, status ticker
  -> Decision: timing baseline updated for passing tests only (StateTimeout excluded); EMA alpha=0.3; stress mode sets SkipTimings=true but normal verify does not
- [ ] `internal/test/runner/runner_exec.go` - process spawn, timeout kill
  -> Decision: context.WithTimeout is pure wall-clock; ETXTBSY already retried (3 attempts, 10ms sleep); "unknown" FailureType set at line 453 when peer output has no "mismatch"/"connection refused"
- [ ] `internal/test/runner/record.go` + `failure_group.go` - how "unknown" classification arises
  -> Decision: recordFailureKind returns FailureType or "unknown" if empty; subsystemPrefix extracts first segment before separator; no "near-timeout" classification exists
- [ ] `scripts/status/verify_run.go` - `classifyFunctional`, failure-group construction
  -> Decision: classifyFunctional parses native VERIFY FAILURE GROUP JSON tokens from suite output; verify runner sets ZE_VERIFY_MODE=1 in stage env
- [ ] `scripts/dev/verify-lock.sh` - flock scope, stuck-holder break
  -> Decision: flock serializes make ze-verify* only; MAX_LOCK_AGE=1800s; stuck-holder SIGTERM then SIGKILL; does not cover bin/ze-test or go test
- [ ] `internal/test/runner/timing.go` - tmp/test-timings.json read/update
  -> Decision: EMA alpha=0.3, SuggestedTimeout=max(5s, 5x EMA) capped by fallback; IsSlow=duration>2x EMA (needs 3+ samples); MaxMs only grows; Save is atomic (temp+rename) but concurrent runners can overwrite each other's updates

**Behavior to preserve:** (unless user explicitly said to change)
- No retry-on-failure anywhere (project stance, `ai/rules/testing.md`).
- Failure groups stay machine-readable (`tmp/ze-verify-failures.json`,
  kebab-case keys) with exact isolation rerun commands.
- The verify lock's existing `make ze-verify*` semantics.
- Quiet-machine behavior, timings, and FRESH semantics unchanged.

**Behavior to change:** (only if user explicitly requested)
- Ambiguous environmental failures: a loaded run must be distinguishable
  from a regression without a manual rerun.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `make ze-verify` -> `scripts/status/verify_run.go` -> per-stage `make`
  targets -> `bin/ze-test <suite>` -> per-test daemon/peer process spawn.

### Transformation Path
1. Suite runner selects tests, spawns processes per test
2. Timeout supervision (EMA baseline, kill-after) marks PASS/FAIL/TIMEOUT
3. Stage log parsed into failure groups (`classifyStage` / `classifyFunctional`)
4. Status file + failure index written; FRESH/STALE derived from tree hash

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner <-> OS scheduler | wall-clock timers vs CPU starvation | [ ] |
| Concurrent runners <-> shared resources | ports, build cache, bin/, tmp/ | [ ] |

### Integration Points
- `failureGroup` JSON (consumed by agents for rerun decisions)
- `scripts/dev/verify-status.sh` FRESH/STALE semantics

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (n/a, test infrastructure)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-verify` under induced load | -> | load context recorded in failure groups | [name after design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Reproduce the failure with induced CPU/process load | Failure mode matches the 2026-06-10 evidence and the starved resource is identified with measurements |
| AC-2 | Any functional-stage failure | Failure group records host load context (load average, concurrent runner count) |
| AC-3 | Failure with abnormal recorded load | Verify summary explicitly labels the run as contended next to the isolation rerun commands |
| AC-4 | Chosen mitigation(s) active | The AC-1 reproduction no longer produces an ambiguous red: green, or failure labeled contended |
| AC-5 | Quiet machine | Gate behavior, timings, baseline updates, and FRESH semantics unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [load-context capture] | `scripts/status/verify_run_test.go` | failure groups carry load fields | |
| [unknown-classification root cause] | `internal/test/runner/` | the observed "unknown" outcome gets a named classification | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| contended-load threshold | [define in design] | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [contended-run labeling] | [pick suite during design] | a failed gate under load names the contention and the rerun | |

### Interop Tests (MANDATORY for protocol features)
- Skipped: test infrastructure, no wire-protocol behavior.

## Files to Modify
<!-- determined by the investigation; expected candidates -->
- `scripts/status/verify_run.go` - load context in failure groups/summary
- `internal/test/runner/` - timeout/baseline behavior under load
- `scripts/dev/verify-lock.sh` - if lock-scope extension is chosen
- `ai/rules/testing.md` - document the contended-verdict reading

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | not expected |
| CLI commands/flags | [ ] | not expected |
| Functional test for new RPC/API | [ ] | n/a unless a new surface is added |
| Pipe completeness | [ ] | not expected |
| Env var registration | [ ] | if a load-threshold knob is added: `ze.*` via `env.MustRegister()` |
| Doctor check for runtime dependencies | [ ] | not expected |
| Prometheus counters/metrics | [ ] | not expected |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | no |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md`, `docs/architecture/testing/` |
| 15 | Agent discovery? | [ ] | `ai/rules/testing.md` (how to read a contended verdict), `ai/INDEX.md` |

## Implementation Phases

### Phase 1: Load detection + near-timeout classification
- Add `HostLoad` struct and `SnapshotHostLoad()` to `internal/test/runner/`
- Add `FailTypeNearTimeout` constant and near-timeout detection in `runner_exec.go`
- Add `Contended` field to `FailureGroup`
- Unit tests for load snapshot, near-timeout classification, contended detection

### Phase 2: Baseline suppression + contended labeling
- Suppress `timings.Record`/`timings.Save` when run is contended (`parallel.go`)
- Label contended failures in `PrintFailureGroups` output
- Pass load context through to verify runner's failure index (`verify_run.go`)
- Unit tests for baseline suppression, contended label formatting

### Phase 3: Advisory warning + documentation
- Emit advisory warning in verify runner when concurrent ze-test/go-test detected
- Update `docs/architecture/testing/runner-architecture.md` with contended-run docs
- Update `ai/rules/testing.md` with how to read contended verdicts

## Risks & Assumptions

### Assumptions
| ID | Assumption | Validation | Status |
|----|-----------|------------|--------|
| A-1 | `runtime.NumCPU()` and load average are available on darwin and linux | grep runtime.NumCPU; check syscall.Sysinfo | unvalidated |
| A-2 | Counting concurrent ze/go processes via `pgrep` or `/proc` is reliable enough for advisory classification | test on darwin | unvalidated |
| A-3 | 80% of timeout is a reasonable near-timeout threshold | review against timing baseline data | unvalidated |

### Risks
| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | Load detection overhead slows quiet-machine runs | Keep it to one syscall at run start, not per-test |
| R-2 | Near-timeout threshold too aggressive on slow machines | Make threshold configurable or only classify when load is also high |

## Critical Review Checklist

| # | What to verify | Expected |
|---|---------------|----------|
| 1 | Quiet-machine behavior unchanged | Same timings, same FRESH, same failure groups |
| 2 | Near-timeout only triggers when elapsed > 80% AND peer failed | Never on passing tests, never on early failures |
| 3 | Baseline not updated when contended | Timings file unchanged after contended run |
| 4 | Contended label appears in failure index | Machine-readable JSON has contended field |
| 5 | Advisory warning is visible but not blocking | Warning printed to stderr, exit code unaffected |

## Deliverables Checklist

| # | Deliverable | Verification |
|---|-------------|-------------|
| 1 | `SnapshotHostLoad()` function | Unit test: returns non-zero CPU count and load on darwin/linux |
| 2 | Near-timeout classification | Unit test: elapsed 85% of timeout + peer failure = near-timeout; elapsed 50% = unknown |
| 3 | Contended field in FailureGroup | Unit test: GroupFunctionalFailures with contended load sets field |
| 4 | Baseline suppression | Unit test: ParallelRunner with contended=true does not save timings |
| 5 | Contended label in verify output | Unit test: PrintFailureGroups includes contended marker |
| 6 | Advisory concurrent-process warning | Integration: verify_run prints warning when concurrent processes detected |

## Security Review Checklist

| # | Concern | Check |
|---|---------|-------|
| 1 | Process enumeration (pgrep) does not expose sensitive data | Only counts processes, does not log arguments |
| 2 | Load data in JSON output does not leak system info | Only load_avg and process count, no PIDs or paths |

## Evidence from 2026-06-10 (for the investigating session)

- Failing run: functional stage only; groups `plugin:unknown:add` (test 1),
  `plugin:timeout:announce` (test 4), `plugin:timeout:api` (test 11).
- Isolation rerun minutes later: `bin/ze-test bgp plugin 1 4 11` -> 3/3 PASS
  in 7.1s (timing: 1:6.9s 4:6.6s 11:7.1s).
- Concurrent activity during the failing run: a second verify cycle had just
  drained, six subagent sessions had been running `go test` against large
  package trees, and commits were being created mid-run.
- The stage logs under `tmp/verify/` from that run were overwritten by later
  cycles; reproduce rather than rely on them.
- Related prior art: the reload/managed suites are already forced to `-p 1`
  (suite isolation fragility), and `plan/learned/868-test-web-parallel.md`
  covers per-test daemon isolation for the web suite.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded
