# Spec: gates-report-cause-and-progress

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling: how `./le` gates report failure and progress, and what a commit checks. No shipped binary changes |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Root cause: `./le` has no single failure and progress path. Each gate, wrapper
and child-process step builds its own report, so the cause is dropped at
whichever layer assumed another layer printed it, a long run reports only to a
registry log the operator does not watch, and the commit path runs no static
check at all. Three symptoms of one gap:

| Symptom | Mechanism |
|---------|-----------|
| A gate fails with no cause | A wrapper renders its own summary and drops the inner answer, exit code or stderr; a refusal writes to process stderr instead of the writer it was given |
| A long run is silent | Stage start, end, count and heartbeat are written to the job registry log only |
| Lint findings land at HEAD | `./le commit create` runs no lint and no static tree check over the files it commits, so each session finds the last session's findings |

Why it matters: the owner ran `./le verify current fast` on 2026-10-23; the
scope widened to every package after an exit 130, one line said so, then
nothing for 45 minutes, and he cancelled it with no idea whether it was nearly
done or stuck. A failure with no cause costs a session an investigation that
the gate had already done. Lint debt at HEAD makes every scoped lint red for
reasons the current change did not cause.

Goal: one failure path that always carries exit code, cause and stderr tail;
progress on the operator's terminal with stage, done/total and elapsed time;
and a commit that lints what it commits.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/running-commands.md` - how `./le` runs and reports
  → Constraint: every user-visible message says what, why and the fix command (`ai/rules/user-facing-errors.md`)
- [ ] `docs/contributing/committing.md` - the commit script and its refusals
  → Decision: the lint stage is one more refusal of the same kind, scoped to committed files
- [ ] `ai/rules/writing.md` - project text is US English
  → Constraint: `misspell` `locale: US` in `.golangci.yml` is correct. The analysis that called it a contradiction with a UK prose rule is wrong: UK spelling applies only to text in Thomas's voice

## Current Behavior (MANDATORY)

Source files read on 2026-10-23:
- [ ] `internal/le/le/action/leaction.go` - `Sweep.Text` (line 803) now renders a failing row whose answer implements `Text()`, then `Failed: <verbs>`. A failing row whose answer is not prose, or is nil, is still skipped, so its cause and exit code are dropped. `ReportError` (line 594), `refuseVerb` (602) and `refuseValue` (613) write through `textbuf.StdErr()` to the process, never to the `errOut` writer `leroot.Run` receives
- [ ] `internal/le/verify/engine/run.go` - `beat` (line 231) writes `### Stage running: <stage>, Ns` and the stage loop writes `### Stage NN/NN` blocks through `tell` to `slot.Progress`
- [ ] `internal/le/verify/current.go` - `slotFor` (line 170) sets `slot.Progress` to the job registry log file only. The terminal gets neither the stage lines nor the heartbeat; a stage that runs every package reports no inner count anywhere
- [ ] `internal/le/go/lint/failuregroup.go` - `declareLintFailureGroup` (line 78) always declares "golangci-lint reported findings in the files named here", whatever made the stage red
- [ ] `internal/le/interoplab/wait.go` - `waitNeverMeasuredError` (line 91) reports only the last probe error, which can be the wait's own deadline
- [ ] `internal/le/test/qemu/` - `applianceStepError` keeps the exit code in `mplsboot.go`, `hugepages.go`, `crashcapture.go`; the install proofs (`install_build.go`, `install_iso.go`, `install_ventoy.go`) do not use it
- [ ] `internal/le/commit/verification.go` - reads `tmp/ze-verify-failures.json` (line 96); nothing in `internal/le/commit` runs golangci-lint or a tree check
- [ ] `.golangci.yml` - `misspell` `locale: US` (line 256), matching `ai/rules/writing.md`
- [ ] `internal/le/go/lint/verifylint.go` - scope entries are now prefixed with `./` (lines 497-502); that row is fixed
- [ ] `internal/le/verify/deps/verifydeps.go` - `planVulnerability` returns "resolve installed govulncheck" through `ReportError` (lines 534-537); that row is fixed

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Any `./le` line dispatched by `leroot.Run(name, answer, args, out, errOut)`
- `./le verify current fast` through `runFast` and `verifyengine.RunMode`
- `./le commit create` through `internal/le/commit`

### Transformation Path
1. An action runs, possibly as a child process, and returns an answer and a code
2. A sweep or wrapper collects answers and renders a summary
3. The verify engine runs stages in order and writes stage blocks
4. The commit command checks inputs, writes the message and the script

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Action to dispatcher | answer value and exit code | [ ] |
| Child process to wrapper | exit code, stdout, stderr | [ ] |
| Engine to operator | registry log today; terminal after this spec | [ ] |

### Integration Points
- `leaction.Sweep`, `ReportError`, `refuseVerb`, `refuseValue`
- `verifyengine.Slot.Progress`, `beat`, `tell`
- `internal/le/commit` refusal list

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | [ ] | refusals go through the dispatcher's writer |
| No duplicated functionality | [ ] | one failure type replaces per-wrapper error building |
| Registration over hardcoding | [ ] | a stage reports its inner count through the slot, no per-stage switch |

## Fix design

1. **One failure value.** A failing action answers a failure carrying exit code,
   a one-line cause and the last lines of the child's stderr. `Sweep.Text`
   renders it for every failing row, prose or not. Child-process wrappers
   (qemu install proofs, `repo compiles`, the changed-package selector, the
   exabgp api runner, netfilter fixtures) build it through one helper that keeps
   the exec error, the code and the stderr tail. A wait with only failed probes
   reports the distinct failures with counts, excluding its own deadline.
2. **The given writer.** `ReportError`, `refuseVerb` and `refuseValue` take the
   writer `leroot.Run` received. Nothing in an action writes to the process
   stderr directly.
3. **Progress on the terminal.** The verify engine writes to the terminal, as
   well as to the registry log: each stage as it starts (`stage N/M: <name>`) and
   ends (exit, seconds), a heartbeat every interval with stage elapsed and run
   elapsed, and, for a stage that runs packages or suites, done/total. The same
   slot is open to any long `./le` action.
4. **Lint on commit.** `./le commit create` runs scoped golangci-lint, and the
   static tree checks keyed to file paths (number-parse allowlist, UI placement,
   ci-dispatch), over the Go files it commits. A finding on a line the commit
   changes refuses the commit with the finding and the fix; findings elsewhere
   in those files are printed, not refused.
5. **Lint summary from evidence.** The lint failure group says "findings" only
   when the collector saw findings; a red with none says the lint did not run
   and prints the child's cause.

The check that stops it coming back: a source test over `internal/le` that
fails on an action writing to `os.Stderr` or calling `StdErr()` outside the
dispatcher, and a sweep test that fails when a failing row of any answer type
renders no cause.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify current fast` | → | engine terminal progress | `TestFastVerifyPrintsStageProgressToTerminal` |
| `./le <area> a b` with `b` failing silently | → | `Sweep.Text` failure rendering | `TestSweepRendersCauseForEveryFailingRow` |
| `./le commit create` with a lint finding on a committed line | → | commit lint stage | `TestCommitRefusesLintFindingOnChangedLine` |
| `leroot.Run` with a captured `errOut` and an unknown verb | → | `refuseVerb` | `TestRefusalWritesToGivenWriter` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le verify current fast` with any scope | Each stage prints on the terminal when it starts, with its number and the total (`stage 3/12`), and when it ends, with exit code and seconds |
| AC-2 | A stage running longer than the heartbeat interval | The terminal gets a line per interval naming the stage, stage elapsed and run elapsed |
| AC-3 | A stage that runs packages or suites | The heartbeat line carries done/total for that stage |
| AC-4 | A swept action fails with a non-prose or nil answer | The output names the action, its exit code and its cause above `Failed:` |
| AC-5 | A child step exits non-zero with empty stdout | The report carries the exit code and the stderr tail |
| AC-6 | `leroot.Run` with a buffer as `errOut` and an unknown verb | The refusal is in the buffer; process stderr is empty |
| AC-7 | A wait whose probes all failed, the last on the wait deadline | The report names the distinct non-deadline failures with counts |
| AC-8 | Lint stage red because golangci-lint never ran | The message says lint did not run and prints the cause, not "findings" |
| AC-9 | `./le commit create` over a Go file whose changed line has a lint finding | The commit is refused, naming the file, line, linter and fix |
| AC-10 | `./le commit create` over a Go file with findings only on unchanged lines | The commit proceeds and prints those findings |
| AC-11 | `./le commit create` that leaves a number-parse allowlist entry stale | The commit is refused naming the entry |
| AC-12 | An action writing directly to process stderr is added | The source test fails naming the file |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | Validation |
|----|------------|-------|------------|
| A-1 | Scoped lint over a commit's files fits in commit time | scoped lint of one package runs in seconds | measure on a 10-file commit |
| A-2 | Terminal progress does not break consumers that parse verify output | the report and certificate are separate files | grep consumers of verify stdout |

### Risks
| Risk | Mitigation |
|------|------------|
| Lint on commit blocks a session on debt it did not write | Only findings on changed lines refuse |
| Progress lines interleave with stage output | One writer goroutine, the same stop-before-write rule `beat` already holds |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFastVerifyPrintsStageProgressToTerminal` | `internal/le/verify/` | AC-1, AC-2 | [ ] |
| `TestStageHeartbeatCarriesDoneTotal` | `internal/le/verify/engine/` | AC-3 | [ ] |
| `TestSweepRendersCauseForEveryFailingRow` | `internal/le/le/action/` | AC-4 | [ ] |
| `TestChildStepErrorKeepsCodeAndStderr` | the helper's package | AC-5 | [ ] |
| `TestRefusalWritesToGivenWriter` | `internal/le/le/action/` | AC-6 | [ ] |
| `TestWaitReportsDistinctFailures` | `internal/le/interoplab/` | AC-7 | [ ] |
| `TestLintGroupSaysNotRunWithoutFindings` | `internal/le/go/lint/` | AC-8 | [ ] |
| `TestCommitRefusesLintFindingOnChangedLine` | `internal/le/commit/` | AC-9, AC-10 | [ ] |
| `TestCommitRefusesStaleAllowlistEntry` | `internal/le/commit/` | AC-11 | [ ] |
| `TestNoActionWritesProcessStderr` | `internal/le/` | AC-12 | [ ] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-verify-progress.ci` | `test/ui/` | operator runs fast verify and sees stage, count and elapsed | [ ] |

## Files to Modify
- `internal/le/le/action/leaction.go` - failure value, `Sweep.Text`, writer-taking refusals
- `internal/le/verify/engine/run.go`, `internal/le/verify/current.go` - terminal progress, inner counts
- `internal/le/go/lint/failuregroup.go` - summary from evidence
- `internal/le/interoplab/wait.go` - distinct failures
- `internal/le/test/qemu/install_build.go`, `install_iso.go`, `install_ventoy.go` - child step helper
- `internal/le/repo/compiles/`, `internal/le/repo/changed/selector.go` - keep exec and package errors
- `internal/test/runner/runner.go` - `RunWithCount` keeps each failure's diagnostic
- `internal/le/commit/` - lint and tree-check stage
- `docs/contributing/running-commands.md`, `docs/contributing/committing.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, env var, doctor, metrics, BGP family | N-A | dev tooling only |
| CLI commands/flags | Yes | output of `./le verify` and `./le commit create` changes; no new grammar |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command changed? | Yes | `docs/contributing/running-commands.md` (progress), `docs/contributing/committing.md` (lint refusal) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` (engine progress on the terminal) |
| 16 | Changed files referenced by doc anchors? | Yes | `docs/architecture/testing/qemu-integration.md` (install proof errors), `docs/architecture/testing/interop.md` (wait failure report), `docs/architecture/testing/ci-format.md` (`RunWithCount` keeps failures); `docs/architecture/core-design.md` is declared by the changed files and is unaffected unless its tooling section names the failure path. Re-derive with `./le spec citation anchors spec plan/post-release/engineering/spec-gates-report-cause-and-progress.md` |
| 1-2, 4-15, 17 | Product surfaces | No | no product change |

## Implementation Steps
1. Terminal progress in the verify engine (AC-1 to AC-3), first, because it is the owner's complaint.
2. Failure value, `Sweep.Text`, child step helper (AC-4, AC-5, AC-7, AC-8).
3. Writer-taking refusals and the source test (AC-6, AC-12).
4. Commit lint and tree-check stage (AC-9 to AC-11).
5. Fix the lint findings named in the owned rows that are still present at HEAD, so the commit stage starts from a clean tree for the files it will see.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `long-run-reports-no-progress.md` | 2026-10-23 | `./le verify current fast` silent for 45 minutes |
| `diagnostic-bypasses-the-writer-it-was-given.md` | 2026-08-26 | `ReportError`, `refuseVerb`, `refuseValue` write process stderr |
| `failing-gate-prints-no-cause.md` | 2026-10-10 | `appArmorProfileUnapplied` misses crun's refusal |
| `failing-gate-prints-no-cause.md` | 2026-10-10 | qemu install proofs drop `result.Code` |
| `failing-gate-prints-no-cause.md` | 2026-10-06 | `Runner.RunWithCount` clears a failure's diagnostic |
| `failing-gate-prints-no-cause.md` | 2026-10-05 | `repo/compiles` `Build` discards the exec error |
| `failing-gate-prints-no-cause.md` | 2026-10-05 | `loadPackageGraph` template omits `.Error` |
| `failing-gate-prints-no-cause.md` | 2026-09-13 | `ze-test exabgp api` exits 2 with no output |
| `failing-gate-prints-no-cause.md` | 2026-09-02 | `declareLintFailureGroup` says findings for any red |
| `failing-gate-prints-no-cause.md` | 2026-10-15 | `firewallReload` observer prints no nft table |
| `failing-gate-prints-no-cause.md` | 2026-10-17 | `waitNeverMeasuredError` reports only the last error |
| `lint-contract-not-applied.md` | 2026-10-23 | `config/parser_diagnostics.go` errorlint, ifElseChain |
| `lint-contract-not-applied.md` | 2026-10-23 | reactor `fwdPool`, `policy_reload.go`, `session_ownership.go` |
| `lint-contract-not-applied.md` | 2026-10-22 | number-parse allowlist out of date |
| `lint-contract-not-applied.md` | 2026-10-21 | `susOutOfSA` paramTypeCombine |
| `lint-contract-not-applied.md` | 2026-10-10 | `interoplab/bgp/` findings at HEAD |
| `lint-contract-not-applied.md` | 2026-10-10 | `internal/test/fixture/` 37 findings |
| `lint-contract-not-applied.md` | 2026-10-09 | rsvpte misspell `neighbour`, unparam |
| `lint-contract-not-applied.md` | 2026-10-05 | full native lint, artifact 1902 |
| `lint-contract-not-applied.md` | 2026-10-05 | UI placement gate, `TrimSpawn` |
| `lint-contract-not-applied.md` | 2026-10-04 | `checkNumberParseSites` MUP entry |
| `lint-contract-not-applied.md` | 2026-09-28 | number-parse allowlist kept a removed line |
| `lint-contract-not-applied.md` | 2026-09-23 | final baseline lint |
| `lint-contract-not-applied.md` | 2026-09-23 | required post-edit baseline lint |
| `lint-contract-not-applied.md` | 2026-09-21 | `internal/le/rfc`, `site` misspell `analysed` |
| `lint-contract-not-applied.md` | 2026-09-20 | `specstatus_test.go` rangeValCopy |
| `lint-contract-not-applied.md` | 2026-09-17 | required full lint run |
| `lint-contract-not-applied.md` | 2026-09-12 | `internal/test/fixture/` 19 findings |
| `lint-contract-not-applied.md` | 2026-09-10 | runner tests `honoured`, `exec.Command` |
| `lint-contract-not-applied.md` | 2026-09-10 | command, IKE, IPsec interop lint |
| `lint-contract-not-applied.md` | 2026-08-31 | authradius `setupCapturingAuth` unparam |
| `lint-contract-not-applied.md` | 2026-08-31 | `internal/le/rfc` `carriers.go` goimports |
| `lint-contract-not-applied.md` | 2026-08-30 | rpki `writeResetQuery` unparam, capability flavor |
| `lint-contract-not-applied.md` | 2026-08-29 | host flavor, 16 findings |
| `lint-contract-not-applied.md` | 2026-08-19 | ci-dispatch call-site gate |
| `lint-contract-not-applied.md` | 2026-08-19 | l2tp diag tools: 8 findings at HEAD |

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence today |
|------------|------|-----------------|----------------|
| `failing-gate-prints-no-cause.md` | 2026-10-18 | `./le go lint run scope <pkg>` without `./` | scope entries are prefixed with `./` (`verifylint.go` 497-502) |
| `failing-gate-prints-no-cause.md` | 2026-09-01 | `le integration` sweep drops a native report | `Sweep.Text` renders a failing prose row above the summary (`leaction.go` 803-829); the non-prose case is AC-4 |
| `failing-gate-prints-no-cause.md` | 2026-09-02 | `verify deps vulnerability` exit 127, no cause | `planVulnerability` returns "resolve installed govulncheck" through `ReportError` (`verifydeps.go` 534-537, `actions.go` 72) |
| `lint-contract-not-applied.md` | 2026-08-19 | retired `scripts/evidence` l2tp diag findings | `scripts/evidence` no longer exists |

## Rows in these classes not owned here
- `failing-gate-prints-no-cause.md` 2026-10-10 `ze appliance build` leaves `tmp/appliance-build-*`: cleanup skipped by `os.Exit`, a lifecycle defect.
- `failing-gate-prints-no-cause.md` 2026-09-27 link-local tests fail as a byte diff without `fd00::2`: host state with no preflight, the test-isolation family.
- `failing-gate-prints-no-cause.md` 2026-09-19 guard self-check names the wrong causes: a guard message, the gate-population family.

## Related specs
- `plan/post-release/engineering/spec-le-one-verb-one-job.md` (skeleton) lists `refuseVerb` and `refuseValue` answering 2 and saying why. It names the file as `internal/le/leaction/leaction.go`; the file is `internal/le/le/action/leaction.go`. This spec owns where they write; that one owns the exit-code contract.
- `plan/post-release/engineering/spec-lint-ratchet-clock-and-discarded-errors.md` (ready) adds new lint captures. Its captures only refuse new occurrences if lint runs on commit, which AC-9 provides.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Scoped tests pass: `./le job run label gates command go test ./internal/le/le/action/... ./internal/le/verify/... ./internal/le/commit/...`, `./le test functional ui`, then `./le verify current fast` showing the progress lines

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests N-A: tooling with no protocol peer

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
