# Spec: test-trace-mode

| Field | Value |
|-------|-------|
| Status | done |
| Depends | test-web-parallel (closed), verify-debugging-protocol (closed) |
| Phase | complete |
| Updated | 2026-06-17 |

> **Spec 3 of 3** in the test-runner sequence: `test-web-parallel` -> **`test-trace-mode`**.
> (`test-runner-unify` was absorbed into `test-web-parallel`; web migrated to `ParallelRunner`.)
> Implementation landed in commit `093bf7bda` ("feat(test): add per-step trace output to test runners").
> This spec is being closed retroactively: the feature shipped but the spec was never updated from skeleton.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/testing/ci-format.md` - directive runner model and debug surfaces
4. `plan/spec-verify-debugging-protocol.md` - the in-progress dependency; owns failure-group + verify-mode rendering on the same files
5. `internal/component/web/testing/runner.go`, `internal/component/cli/testing/runner.go`, `internal/test/runner/runner_exec.go` - the three runners trace-mode instruments

## Task

Add **per-step trace output** to the three directive-based test runners with two tiers:

1. **Default (no flag):** when a test **fails**, print a per-step replay showing which steps passed before the failure and what broke. Passing tests stay as a one-line PASS.
2. **Verbose (`-v`):** print per-step trace for **every** test, pass or fail.

Today every runner reports only a file-level PASS/FAIL. None print "this assertion held / this one broke" step by step. The web (`.wb`) `-v` flag is parsed but entirely unused; the editor (`.et`) `-v` only prints the first failure; the `.ci` runner fail-fasts on the first missed `expect=` and reports nothing about the assertions that passed.

**Scope (user-confirmed): all three directive families.**

| Family | Files | Runner entry | Current per-step output |
|--------|-------|--------------|--------------------------|
| Web UI | `.wb` | `internal/component/web/testing/runner.go` `runWBTestCase` (ordered `Steps`) | none; `-v` parsed but UNUSED (highest-value target) |
| Editor TUI | `.et` | `internal/component/cli/testing/runner.go` `runTestCase` (ordered `Steps`) | `-v` prints first failure only |
| CI / config | `.ci` | `internal/test/runner/` shared runner; checks in `runner_exec.go` / `runner_validate.go` | fail-fast: sets `rec.Error` on first miss; needs collect-all refactor |

Out of scope: Go `*_test.go` (`go test -v` already provides per-test output).

**Output format (user-confirmed): DUAL.**
- Human: colored, aligned `✓` / `✗` trace, one line per step, in execution order. Example: `  5 ✗ expect element text="missing" → NOT FOUND`.
- Machine/AI: one stable greppable key=value marker line per step, coherent with the existing `VERIFY FAILURE GROUP: {json}` idiom in `internal/test/runner/failure_group.go`. Example: `VERIFY STEP: {"file":"scenario.wb","step":5,"kind":"expect","assert":"element","status":"fail","detail":"not found"}`.

**Two-tier design consequences:**
- Default tier (failure-only): no parallelism interleaving problem, since failure output is already printed post-execution. This is the high-value path.
- Verbose tier (all tests): needs serialization under parallelism (buffer-per-test then flush atomically, or force `-p 1`). Editor and `.ci` runners are concurrent.
- `.ci` collect-all: record step results as execution proceeds; dump the trace only on failure (default) or always (`-v`). The fail-fast verdict is unchanged, only the reporting is richer.

**Relationship to `verify-debugging-protocol` (now closed): separate spec, layered on it.**
That spec owns the macro failure-routing layer (compact failure index, conservative grouping, exact reproducers) and the `ZE_VERIFY_MODE` rendering layer. Trace-mode is the orthogonal micro layer (per-step visibility). It MUST reuse the verify-mode + failure-group foundation rather than invent a parallel machine-readable convention.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `docs/architecture/testing/ci-format.md` - directive runner model, `.ci`/`.wb`/`.et` formats, debug surfaces
  → Constraint: all three runner files carry `// Design: docs/architecture/testing/ci-format.md`; this doc is the canonical place to document trace mode (BLOCKING doc update).
  → Constraint: doc line 41 already states "In `ZE_VERIFY_MODE=1`, failed suites emit native failure-group metadata" — trace mode is a sibling rendering, document it next to that.
- [ ] `internal/test/runner/failure_group.go` - the machine-readable output convention to match
  → Constraint: existing machine token is `VERIFY FAILURE GROUP: {single-line-json}` (uppercase prefix + JSON), with human `group:/kind:/related:/rerun:` lines under a `VERIFY FAILURE INDEX:` header, gated on failures.
  → Decision: trace token MUST follow the same prefix+JSON shape (e.g. `VERIFY STEP: {json}`) for one uniform convention, NOT a bespoke `STEP file#N key=value` form. (Reconcile with user's preview at DESIGN.)
- [ ] `internal/test/runner/color.go` - glyph/color helpers
  → Constraint: `Colors` (Red/Green/Yellow/Cyan/Gray) is TTY-aware via `slogutil.UseColor(os.Stdout)`. NO `✓`/`✗` glyph helper exists yet — trace mode adds them.
  → Constraint: `Colors` lives in `internal/test/runner` (the `.ci` package); web/cli testing are separate packages and cannot import it without a cycle. Shared trace format needs a LEAF package.
- [ ] `internal/test/runner/parallel.go` - verify-mode gate + parallelism
  → Constraint: detail gate idiom is `(r.verbose || verifyModeEnabled())`; `verifyModeEnabled()` = `os.Getenv("ZE_VERIFY_MODE") == "1"`. Reuse this gate, do not invent a new env var.
  → Constraint: editor (`ParallelRunner`) and `.ci` (`opts.Parallel`) run tests CONCURRENTLY. Default tier (failure-only trace) is safe: failure output is post-execution. Verbose tier (`-v`, all tests) needs serialization (force `-p 1` or buffer-per-test then flush atomically).
- [ ] `plan/spec-verify-debugging-protocol.md` - dependency (now closed)
  → Decision: reuse its `ZE_VERIFY_MODE` gate + `VERIFY *` token family; add nothing that competes.
- [ ] `ai/rules/derive-not-hardcode.md` / `ai/rules/error-messages.md` - emitter shape
  → Constraint: emit structured per-step data (kind, assert, params, status, detail), format at the edge; one stable greppable phrase per step.
- [ ] `docs/functional-tests.md` - `ze-test` suite list, documented log workflow (doc update target)

### RFC Summaries
- N/A - this spec changes test-runner output, not a wire protocol.

**Key insights:**
- Three runners, three packages: web `internal/component/web/testing` (sequential, `-v` UNUSED, steps carry source `Line`), editor `internal/component/cli/testing` (parallel via `ParallelRunner`, `-v`=first-failure-only, steps located by ordinal -- NO `Line`), `.ci` `internal/test/runner` (parallel, fail-fast on first miss, gated by `ZE_VERIFY_MODE`).
- Web is the lowest-risk, highest-value first target: sequential loop, ordered `Steps`, flag already parsed.
- Two-tier model simplifies `.ci`: record step results during execution, dump trace on failure (default) or always (`-v`). Fail-fast verdict unchanged; only reporting is richer. No full collect-all refactor needed for the default tier.
- Reuse, don't reinvent: `ZE_VERIFY_MODE` gate, `Colors`, and the `VERIFY ...: {json}` token family already exist. Trace mode adds a leaf trace package + `✓`/`✗` glyphs + a `VERIFY STEP` token.
- Parallelism interleaving only matters for `-v` (all tests); default tier (failure-only) prints post-execution. Shared format in a leaf package (no import cycle with `internal/test/runner`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/testing/runner.go` - `RunWBFile` (L383) → `runWBTestCase` (L401-423)
  → Constraint: `runWBTestCase` loops `tc.Steps` (`WBStepAction`/`WBStepExpect`), executes via `executeAction`/`checkExpectation`, returns `*WBTestResult{Error: "line N: <kind>: <err>"}` on FIRST failure. No writer/tracer param. Adding trace = thread a tracer through this function.
- [ ] `internal/component/web/testing/parser.go` - `WBStep`/`WBAction`/`WBExpectation`
  → Constraint: actions and expects carry `Line`, `Kind`, `Values map[string]string` — enough to render a trace line with source line + params. `Steps` preserves action/expect interleaving.
- [ ] `cmd/ze-test/web.go` - sequential per-file loop (L200-252)
  → Constraint: `RunWBFile` called in a SEQUENTIAL loop; prints `%-7s %d/%d <tag> <nick> <name>` per file. `-v` parsed (L40-41) but UNUSED — wiring it is the smallest first win. Sequential = no interleave problem for `.wb`.
- [ ] `internal/component/cli/testing/runner.go` - `runTestCase` step loop (L242-329)
  → Constraint: loops `tc.Steps` (`StepSession`/`StepRestart`/`StepInput`/`StepExpect`/`StepWait`); returns `*TestResult{Error: "step N (...): ..."}` on FIRST failure. `StepExpect` retries up to 5× with `SettleWait` then `CheckExpectation`. Locator is `stepIdx+1` (ordinal).
- [ ] `internal/component/cli/testing/parser.go` - `InputAction`/`Expectation`/`Step`
  → Constraint: `InputAction`{Action,Values} and `Expectation`{Type,Values} carry NO `Line` field — `.et` trace must locate by step ordinal, OR a `Line` field is added to these structs. Asymmetry vs `.wb`.
- [ ] `cmd/ze-test/editor.go` - `ParallelRunner` + `SetOnFail`/`SetVerbose`
  → Constraint: editor runs via `ParallelRunner` (concurrent). Per-step trace needs serialization or per-test buffering.
- [ ] `internal/test/runner/runner_exec.go` - `runTest`/`runOrchestrated`
  → Constraint: expectation checks (exit code L918-927, stderr L939, stdout-match L946, stdout-not-match L953, plus json/syslog/file) are SCATTERED and FAIL-FAST: each sets `rec.Error` and returns on first miss. Reporting passing assertions requires refactor to collect-all (keep evaluating for the trace while still marking the test failed).
- [ ] `internal/test/runner/runner_validate.go` - `validateJSON`/`validateLogging`/`validateFileChecks`
  → Constraint: same fail-fast pattern; each returns first `error`. These produce the per-assertion detail strings trace would surface.
- [ ] `internal/test/runner/record.go` - `Record`, `State` enum; `cmd/ze-test/ci_runner.go` - `RunOptions{Verbose}`
  → Constraint: `RunOptions.Verbose` is plumbed but largely unused in the orchestrated `.ci` path; trace gives it meaning.
- [ ] `internal/test/runner/runner.go` (L302-310) - `printFailureReports`
  → Constraint: `.ci` failures print only when `!allSuccess && !Quiet`; failure groups only under `verifyModeEnabled()`. Trace output gates similarly.

**Behavior to preserve:**
- Default (non-verbose) one-line-per-file PASS/FAIL summaries across all three runners — unchanged.
- `VERIFY FAILURE INDEX` / `VERIFY FAILURE GROUP` output owned by `verify-debugging-protocol` — unchanged; trace adds a sibling token, not a replacement.
- Exit codes and pass/fail semantics — unchanged. Trace is ADDITIVE output only.
- `.ci` test still FAILS on a real assertion failure; collect-all changes reporting, not the verdict.

**Behavior to change:**
- All three runners: failed tests automatically emit per-step trace (default, no flag).
- Web `-v` becomes functional (currently a no-op): shows per-step trace for ALL tests including passes.
- Editor `-v` upgrades from first-failure-only to full per-step trace for ALL tests.
- `.ci` gains per-assertion trace; step results recorded during execution, dumped on failure (default) or always (`-v`).
- New `✓`/`✗` glyph helpers + a `VERIFY STEP` machine token + a shared leaf trace package.

## Data Flow (MANDATORY)

### Entry Point
- Two-tier gate: (1) trace on failure is always active (default), (2) trace on all tests via `-v`/`--verbose` or `ZE_VERIFY_MODE=1`.

### Transformation Path
1. CLI subcommand parses `-v` (web.go / editor.go / ci_runner.go). Resolves verbose bool (`-v || verifyModeEnabled()`).
2. Subcommand constructs a `Tracer` (writer = stdout, colors, verbose) and threads it into the per-file runner: `runWBTestCase` / `runTestCase` / `.ci` `runOrchestrated`.
3. Each step/assertion site calls `tracer.Record(kind, assert, params, status, detail)` as it executes. Steps are accumulated in the tracer.
4. On test completion: if test failed, `tracer.Flush()` emits the step trace (always). If test passed, `tracer.Flush()` emits only when verbose is set.
5. Each flushed step emits TWO lines: a human `✓/✗` aligned line and a machine `VERIFY STEP: {json}` token. Under parallelism with `-v`, lines are buffered per test and flushed atomically.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI flag → runner | tracer threaded into `runWBTestCase`/`runTestCase`/`runOrchestrated` | [ ] |
| Runner package → shared format | leaf `trace` package imported by all three runner packages (no cycle with `internal/test/runner`) | [ ] |
| `.ci` fail-fast → collect-all | accumulate per-assertion results, then emit; verdict unchanged | [ ] |

### Integration Points
- Reuse `verifyModeEnabled()` (`ZE_VERIFY_MODE=1`) as the env gate; reuse `slogutil.UseColor` for TTY detection.
- Align the `VERIFY STEP` token with `failure_group.go`'s `VERIFY FAILURE GROUP: {json}` convention.
- Web command loop (cmd/ze-test/web.go) is sequential — trace plugs in directly; editor/`.ci` need serialization.

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality (reuse verify-mode/failure-group, do not recreate)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-test web -v <file>.wb` | → | `runWBTestCase` emits a per-step trace line through the reporter | `TestRunWBFileVerboseEmitsPerStepTrace` |
| `ze-test editor -v <file>.et` | → | `runTestCase` emits a per-step trace line through the reporter | `TestRunETFileVerboseEmitsPerStepTrace` |
| `ze-test ui -v <file>.ci` | → | `.ci` runner collects and emits per-assertion results | `TestCIRunnerVerboseEmitsPerAssertionTrace` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| (fill during design) | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| | | | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| | | | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Not applicable | - | - | trace-mode changes test-runner output, not a wire protocol | N/A |

### Future (if deferring any tests)
- (none planned)

## Files to Modify
- (fill during design)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] | `cmd/ze-test/web.go`, `cmd/ze-test/editor.go`, `cmd/ze-test/ci_runner.go` |
| Test infrastructure changed | [ ] | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/testing/ci-format.md` |

## Files to Create
- (fill during design)

## Implementation Steps

(fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two-tier: failure trace by default, all-test trace via `-v` | Always-on verbose; flag-gated only | Failure trace is the high-value path and has no parallelism cost. `-v` for full trace serves debugging sessions where you want the complete picture. |
| Separate spec layered on verify-debugging-protocol | Fold into that spec; pause until it closes | That spec is scoped to land its failure-routing cut as one unit; trace-mode is an orthogonal micro layer |
| Dual output (human ✓/✗ + greppable `VERIFY STEP` token) | Human-only; JSON-lines only | Serves terminal reading and AI/tooling parsing in one pass; matches existing `VERIFY FAILURE GROUP` idiom |
| All three families in one spec | Web-only first; web+editor first | Shared tracer concept; `.ci` is heavier but belongs with the same format contract (user-confirmed) |

## Implementation Summary (retroactive)

Implemented in commit `093bf7bda`. All three runners now record per-step `trace.StepResult` slices and emit dual-format trace output via `trace.PrintTrace`.

| What | How |
|------|-----|
| Leaf trace package | `internal/test/trace/trace.go` -- `StepResult`, `PrintTrace`, `ErrString`; no import cycles |
| Human output | Colored `checkmark`/`cross` glyphs via `textbuf.Buffer` color API (not `Colors` methods) |
| Machine output | `VERIFY STEP: {json}` token per step, matching `VERIFY FAILURE GROUP` convention |
| Web runner (.wb) | `runWBTestCase` records `trace.StepResult` per action/expect; `cmd_web.go` prints on failure (always) and on pass (`-v`) |
| Editor runner (.et) | `runTestCase` records `trace.StepResult` per step type; `cmd_editor.go` prints on failure (always) and on pass (`-v`) |
| CI runner (.ci) | `recStep` closure in `runner_exec.go` records per-assertion steps into `rec.StepTrace`; `report.go` prints under failed-test reports |
| Verbose gate | Web/editor: `-v` flag gates passing-test traces at the command level. CI: trace printed for all failed tests when `StepTrace` is non-empty |
| Parallelism | Default tier (failure-only) is safe: printed post-execution. Verbose tier (`-v`): web/editor iterate after `pr.Run()` completes |
| Tests | 5 unit tests in `internal/test/trace/trace_test.go` (human pass, human fail, machine format, line number, step fallback) |

**Deviations from spec design:**
- `.ci` did NOT need a full collect-all refactor. A `recStep` closure records steps incrementally; fail-fast verdict unchanged.
- Glyphs added directly in `trace.go` via `textbuf.Buffer`, not as `Colors` methods.
- Editor steps use ordinal fallback (no `Line` field added to `InputAction`/`Expectation`).
- File paths changed: CLI entry points are `internal/test/cli/cmd_web.go` and `internal/test/cli/cmd_editor.go`, not `cmd/ze-test/web.go` and `cmd/ze-test/editor.go` (those never existed; the spec's paths were wrong).

## Known Limitations
- Editor `.et` trace shows step ordinal, not source line number (parser structs lack `Line` field).
- CI trace is only emitted in failure reports, not gated on `-v` for passing tests.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (or N/A — non-protocol feature)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
