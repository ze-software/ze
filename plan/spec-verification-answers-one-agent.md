# Spec: verification answers one agent about its own change

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | spec-shared-machine-job-admission (closed 2026-09-05; the admission machinery this extends is `internal/le/job`, `Admit` and `Ticket.Release`), spec-verify-scope-1-shared-checkout-freshness (closed 2026-09-05; the freshness certificate this reads is `CheckCertificate`, `internal/le/verify/engine/status.go`, documented at `docs/architecture/testing/verify-freshness-scope.md`) |
| Phase | 1/3 |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Nine agent sessions worked this checkout on 2026-09-05. The dominant cost was not
thinking, it was waiting on gates, and the measurement says the waiting is
structural rather than slow.

**The root cause is one missing call.** `currentHere` and `runCurrent`
(`internal/le/verify/current.go`) call `verifyengine.RunMode` directly and never
call `Admit`; the file imports `internal/le/job` only for `job.Head` and
`job.Unknown`, and `internal/le/verify/lifecycle.go` imports no job package at
all. Only `runHere` (`internal/le/verify/lint/actions.go`) and the `verify lock`
action admit, and nothing in the tree calls `verifylock.Run`. So sixteen
`le verify` processes ran concurrently at load 73-108, three of them past 5000
seconds. `(*Admission).attach` exists precisely to collapse duplicate work and
could not: those processes never asked for a slot.

**The second cause is that sharing is offered on the wrong question.**
`(*Admission).shares` (`internal/le/job/registry.go`) compares a whole-checkout
`TreeHash` (`treehash.go`), so a journal row written by another session voids the
match and a second identical lint runs from scratch. In a checkout nine sessions
write, the whole-tree hash almost never holds still.

**The third is that an agent cannot ask the cheap question at all.** An agent
needs one answer: *does any red name MY files?* Today it can only get that by
running or waiting for a whole-tree verification, 50 to 63 minutes of which 83 to
86% is two whole-tree Go analyses (lint over 20 flavors, and a 6-part staticcheck
matrix). Yet every stage log carries its `VERIFY FAILURE GROUP:` JSON the moment
that stage ends, so the answer exists on disk long before the run finishes.

The goal is that an agent gets a verdict about its own change without starting or
waiting for a whole-tree run, and that duplicate whole-tree runs collapse into one.

**Measured, 2026-09-05.** A full verify: 3756 s and 2991 s in two runs.
`verify lint/run` 1397 s and 1220 s; the staticcheck matrix 1855 s and 1246 s;
every other stage together about 4%. Lint on identical work ranged 124 s to
2341 s across the day, a 19x spread that is contention rather than work.

**Waiting time is invisible by construction, and this spec does not fix that.**
`(*Ticket).Release` (`internal/le/job/job.go`) passes only
`time.Since(t.started)` to `appendDuration` (`internal/le/job/registry.go`), and
`t.started` is set at admission in `take`. `Ticket.Waited` exists and reaches
`job.Report.WaitedSeconds` but is never persisted, so every duration on disk is
working time. Recording it is named in Work Not Done.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the freshness certificate, its scope rules, and how heavy verification is meant to enter
  → Constraint: the page states heavy verification enters through `./le job run` or the `verify-lock` action. That is false today and this spec makes it true, so the page is evidence of intent rather than of behavior.
  → Decision: the same page claims every scoped stage reads the scope files. No stage reads `scope-packages.txt`. Repair both statements in the work that changes them.
- [ ] `docs/architecture/core-design.md` - the admission seam and why one run serves both askers
  → Constraint: sharing a running job is a design commitment already made; this spec extends its reach rather than introducing it.

**Key insights:**
- Attach is not broken. It is never reached from the verify entry point.
- The expensive stages are whole-tree by nature; the win is not running them N times, not making them faster.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/verify/current.go` - `currentHere`, `runCurrent`: resolve HEAD, call `verifyengine.RunMode`, return the report. No admission.
- [ ] `internal/le/verify/lifecycle.go` - the worktree entry point. Imports no job package.
- [ ] `internal/le/verify/lint/actions.go` - `runHere`: the one heavy action that DOES admit, and the shape to mirror.
- [ ] `internal/le/job/job.go` - `Admit`, `queue`, `insideParent`, `(*Ticket).Release`, `KindAttached`.
- [ ] `internal/le/job/attach.go` - `(*Admission).attach`, `follow`: returns the shared verdict and whether anything was observed.
- [ ] `internal/le/job/registry.go` - `(*Admission).shares`, `take`, `appendDuration`.
- [ ] `internal/le/job/treehash.go` - `TreeHash`, `SnapshotTree`: the whole-checkout fingerprint.
- [ ] `internal/le/verify/engine/artifacts.go` - `declaredGroups`, `writeRunArtifacts`: the failure-group parse this spec reuses.
- [ ] `internal/le/verify/engine/scope.go` - `nameChangeScope` (the set-and-restore idiom P1 needs), `publishChangeScope`.

**Behavior to preserve:**
- A verify run's exit codes and its report shape. Callers and the commit gate read them.
- `SnapshotTree` and the freshness certificate keep their whole-tree fingerprint: a certificate legitimately asserts something about the whole tree.
- An attach that observes no result still returns the job to the queue rather than accepting an unobserved verdict.

**Behavior to change:**
- `le verify` enters through admission, so a second concurrent verify attaches or queues instead of running.
- Sharing for a per-label job is decided on the inputs that label READS, not on the whole checkout.
- A new read-only action answers "does any red name these files" against a run that has not finished.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An agent runs `./le verify current mode full`, `./le verify worktree`, or the new query action.

### Transformation Path
1. The verify action calls `Admit(label, argv)` (`internal/le/job/job.go`).
2. `claim` either takes a slot, or `shares` matches a running holder and `attach` follows its log to the shared verdict.
3. A claimed run sets `job.ParentKey` to its own entry before the stage loop, so the in-process lint stage answers `KindInside` in `insideParent` rather than queueing behind its own parent.
4. Each stage writes `tmp/verify/<mode>-<n>/NN-<stage>.log` as it ends, carrying its `VERIFY FAILURE GROUP:` JSON.
5. The query action reads whatever logs exist in a run directory and answers for the paths it was given, without waiting for `writeRunArtifacts`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| verify action ↔ job admission | `Admit` / `Ticket` / `Release` | No |
| running holder ↔ follower | the registry entry and its log file | No |
| in-flight run ↔ query action | stage log files under `tmp/verify/<mode>-*/` | No |

### Integration Points
- `runHere` (`internal/le/verify/lint/actions.go`) - the existing admitted heavy action; P1 mirrors it.
- `declaredGroups` (`internal/le/verify/engine/artifacts.go`) - the failure-group parse the query action reuses rather than reimplements.
- `structuralGateReds` (`internal/le/commit/verification.go`) - an existing consumer of failure groups, and the natural test subject for P3.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The in-process `verify lint/run` stage is the only nested admission a verify run performs | `stages.go` stage list; `runHere` is the one admitted action inside the engine | A second nested admission deadlocks against its own parent slot | A verify run under P1 that completes, plus a test asserting `KindInside` for the nested stage | confirmed. `Admit` has three call sites outside the job package: `verifylint.runHere`, and the two entry points P1 added. `verifylock.Run` reaches `job.Run` and is not a stage. `TestNestedLintStageDoesNotQueueBehindItsParent` asserts `KindInside` for the nested lint, and goes red when `nameJobParent` is broken |
| A-2 | A per-label input fingerprint for lint can be stated exhaustively: tracked `.go` diff, untracked `.go` files, `.golangci.yml`, `feature-gates.txt` | `internal/le/verify/lint` reads no other input | A lint verdict is shared when an unlisted input differs, so a red is missed | A test that changes each listed input and asserts the share is voided, plus one that changes an unlisted file and asserts it is not | broken (P2): the named list is wrong in both directions. `feature-gates.txt` is NOT read by lint, whose feature tags come from `.golangci.yml` `build-tags:` through `parseConfigTags` and `plan` (`internal/le/verify/lint/verifylint.go`). Lint DOES read `go.mod` (`gotoolchain.New`), the Go loader's `go.sum` and `vendor/modules.txt`, and any file a package embeds, which `//go:embed data/*.md` shows is not always a `.go` file. So the fingerprint EXCLUDES instead of listing (`lintIgnores`, `internal/le/job/treehash.go`): an input nobody named still voids the share, and a missing entry costs a duplicate run rather than a stale verdict |
| A-3 | Every stage log is complete and parseable the moment the stage ends | `01-verify-lint-run.log` carried its failure-group JSON in both of today's runs | The query action answers from a truncated log and reports a false green | A test over a run directory holding 3 of 44 logs | confirmed-with-a-guard: `runMode` (engine/run.go) writes each stage log in ONE call after the stage returns, and the body ends `### Stage result: <name> exit=N`. `stageResult` (verify/reds.go) requires that line, so a log met mid-write counts as not reported. `TestRedsAnswersFromAnUnfinishedRun` carries a truncated fourth log and asserts it |
| A-4 | Attaching to a verify is safe for the follower's own commit attribution | `attach` returns the holder's verdict, and the holder judged a tree the follower does not control | A follower reads a verdict about somebody else's tree as its own | The query action answers per-path, so a shared red that names no file of mine is not mine | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | P1 serialises verification so hard that agents wait longer than they do today | A queue depth that grows while one holder runs | Slots are configurable (`SlotsKey`); attach means a second asker shares rather than queues |
| R-2 | P2's fingerprint omits an input and shares a stale verdict | A lint red that disappears without a code change | The input list is stated in one place beside the label and tested per input |
| R-3 | The query action becomes a way to commit over a red by asking a question narrow enough to answer green | Commits whose attribution cites the query rather than a run | The action answers about PATHS and says which stages have not reported yet; it never claims the run passed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing an operator sees: this is development tooling. A wrong P2 shares a stale verdict, which can let a red land; a wrong P1 stalls development. |
| How is it reverted? | Single commit revert per phase. No artifact format changes and no config migration. |
| Who else touches this path? | spec-shared-machine-job-admission and the `verify-scope` family, all closed on 2026-09-05. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify current mode full` | → | `Admit` reached from `currentHere` | `TestVerifyCurrentEntersAdmission` |
| a second concurrent `./le verify` | → | `(*Admission).attach` returning the holder's verdict | `TestTwoVerifiesShareOneRun` |
| the nested lint stage inside an admitted verify | → | `insideParent` answering `KindInside` | `TestNestedLintStageDoesNotQueueBehindItsParent` |
| `./le verify reds file <path>` against a run in flight | → | the failure-group reader over `tmp/verify/<mode>-*/NN-*.log` | `TestRedsAnswersFromAnUnfinishedRun` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two `le verify` invocations of the same mode on the same tree | Exactly one claims a slot; the second attaches or queues, and never runs the stages a second time |
| AC-2 | A verify that claimed a slot reaches its in-process lint stage | The stage runs without queueing behind its own parent, and `insideParent` reports it as nested |
| AC-3 | An attached verify whose holder exits non-zero | The follower's process exit status is the holder's, with no pipe and no wrapper involved |
| AC-4 | A second lint asked while a first runs, after another session wrote a `plan/journal/*.md` row | The second still shares the first's run |
| AC-5 | A second lint asked while a first runs, after a tracked `.go` file changed | The second does NOT share; it queues and runs |
| AC-6 | `./le verify reds file <path>` while a run holds 3 of its 44 stage logs | Answers whether any completed stage names that path, and states plainly how many stages have not reported |
| AC-7 | The same query when no run exists at all | Says so, and does not answer green |
| AC-8 | A verify run that completes under admission | Writes the same artifacts and returns the same exit codes as it does today |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyCurrentEntersAdmission` | `internal/le/verify/current_test.go` | AC-1: the entry point calls `Admit` | |
| `TestTwoVerifiesShareOneRun` | `internal/le/job/contention_test.go` | AC-1: one claims, one attaches | |
| `TestAttachedVerifyExitsWithHolderCode` | `internal/le/job/contention_test.go` | AC-3: the follower's status is the holder's | |
| `TestNestedLintStageDoesNotQueueBehindItsParent` | `internal/le/verify/engine/stages_test.go` | AC-2 | |
| `TestLintShareSurvivesAnUnrelatedFile` | `internal/le/job/contention_test.go` | AC-4 | pass; red when `InputHash` answers the whole-tree hash |
| `TestLintShareVoidedByAGoChange` | `internal/le/job/contention_test.go` | AC-5 | pass; red when `ignoredTree` answers true for every path |
| `TestEveryInputTheLintLabelReadsVoidsItsShare` | `internal/le/job/contention_test.go` | AC-5: one row per input a lint verdict can depend on | pass; all nine rows red under the same break |
| `TestTheTreesTheLintLabelIgnoresDoNotVoidItsShare` | `internal/le/job/contention_test.go` | AC-4: one row per declared tree | pass; red under both breaks |
| `TestTheTreesTheLintLabelIgnoresHoldNoGoFile` | `internal/le/job/contention_test.go` | The evidence behind the declaration, walked over the real checkout | pass |
| `TestALabelThatDeclaresNoInputsIsFingerprintedOverTheWholeCheckout` | `internal/le/job/contention_test.go` | The fail-closed default for every other label | pass |
| `TestRedsAnswersFromAnUnfinishedRun` | `internal/le/verify/reds_test.go` | AC-6 | PASS |
| `TestRedsRefusesToAnswerWithNoRun` | `internal/le/verify/reds_test.go` | AC-7 | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| stage logs present in a run directory | 0 to 44 | 44 | N/A | N/A |
| `ze.run.slots` | 1 to host CPUs | host CPUs | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-shares-one-run` | `test/runner/verify-shares-one-run.ci` | Two verifies started together produce one run and two identical verdicts | ABSENT. Homed in `plan/spec-two-verifies-share-one-run-end-to-end.md` (Work Not Done); `TestTwoVerifiesShareOneRun` proves the same scenario in two real processes |
| `verify-reds-in-flight` | `test/runner/verify-reds-in-flight.ci` | An agent asks whether a running verify has reddened its own file and gets an answer before the run ends | PASS |

## Files to Modify
- `internal/le/verify/current.go` - `currentHere`, `runCurrent`: admit before running.
- `internal/le/verify/lifecycle.go` - the worktree entry point: same.
- `internal/le/verify/engine/run.go` - set and restore `job.ParentKey` around the stage loop.
- `internal/le/job/registry.go` - `shares`: consult a per-label input fingerprint.
- `internal/le/job/treehash.go` - the per-label fingerprint beside the whole-tree one.
- `internal/le/verify/engine/artifacts.go` - expose the failure-group parse to the query action.
- `docs/architecture/testing/verify-freshness-scope.md` - two statements the code contradicts today.
- `internal/le/verify/engine/stages.go` - the `changedStages` comment claims lint and unit testing use changed-tree identities; it keeps `structural("verify lint", "run")` byte-identical in both modes.

## Files to Create
- `internal/le/verify/reds.go` - the in-flight query action.
- `test/runner/verify-shares-one-run.ci`
- `test/runner/verify-reds-in-flight.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Development tooling holds no daemon config |
| YANG validation constraints | No | As above |
| YANG custom validators | No | As above |
| CLI commands/flags | Yes | The `verify reds` action registers in `internal/le/verify` |
| CLI grammar (keyword before value) | Yes | `verify reds file <path>`, keyword before value (`ai/rules/cli.md`) |
| Editor autocomplete | No | `le` actions are not the operator CLI |
| Functional test for new RPC/API | Yes | `test/runner/verify-reds-in-flight.ci` |
| Pipe completeness | Yes | The query answers structured data, so `\| json`, `\| yaml` and `\| table` render it |
| Env var registration | No | `ze.run.slots` already exists |
| Doctor check for runtime dependencies | No | No new runtime dependency; this reads files the run already writes |
| Prometheus counters/metrics | No | Development tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a protocol change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Development tooling, not a product feature |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md` for `verify reds` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/architecture/testing/verify-freshness-scope.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the two new `.ci` files |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, the admission seam section |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The `verify reds` action joins the `le` command surface |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | `./le spec citation anchors spec plan/spec-verification-answers-one-agent.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/running-commands.md` shows verify invocations |

### Critical Review Checklist

Feature-specific only; the six generic checks in `ai/rules/quality.md` always
apply and are not repeated. Every row here exists because this spec can fail in
that particular way, and two of them were written after a phase found the
failure rather than before.

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has an implementation named at file plus symbol, and each of the four Wiring Test rows names a test that exists and runs |
| A shared verdict is about the asker's tree | The input fingerprint EXCLUDES trees rather than listing inputs, because a missed input shares a stale verdict while a missed exclusion only duplicates a run. Every excluded tree is proven to hold no Go file by a test that walks the real checkout, and `//go:embed` cannot leave its package directory |
| No green from ignorance | The query action returns a passing verdict only when the population is non-empty, nothing is pending, and no red went unattributed. A stage counts as reported only once its log carries its closing result line, so a log read mid-write is pending and never clean |
| The nested stage is not its own deadlock | A claimed verify reaching the in-process lint stage answers `KindInside` through `insideParent`, and `job.ParentKey` is RESTORED after the stage loop rather than left set. The set-and-restore follows `nameChangeScope`; a second idiom for the same job is a finding |
| Attach carries the verdict, not the attach | A follower's process exit status is the holder's, measured with no pipe and no wrapper. A pipeline reports its last command's status, which is what made an earlier journal row accuse a correct component (`1a81cfe92`) |
| Data flow | Admission is asked at the ENTRY point, not inside the engine. `SnapshotTree`, `DirtyManifest` and the freshness certificate keep their whole-tree fingerprint: a certificate legitimately asserts something about the whole tree, and narrowing it here would be a different spec |
| Naming | The action's payload keys are kebab-case, and the label is stated once beside its input declaration rather than repeated as a string literal at each use |
| Rule: `ai/rules/cli.md` | `verify reds file <path>` is keyword before value, and its payload renders under `\| json`, `\| yaml` and `\| table` |
| Rule: `ai/rules/principles.md` | No branch answers zero, empty or default in place of an answer. The three verdict values that are not a pass are distinguishable: no run, undetermined, and not-named |
| Rule: `ai/rules/no-layering.md` | The failure-group predicate exists ONCE. Phase 3 was one edit away from a second copy of "is this declared path mine" |

## Implementation Steps

**Phase 1 (wiring): admission at the verify entry point.** Mirror `runHere` in
`currentHere` and in `lifecycle.go`'s entry. Set and restore `job.ParentKey`
around the stage loop, following the set-and-restore idiom `nameChangeScope`
already uses, so the nested lint stage answers `KindInside`. Verify: AC-1, AC-2,
AC-3, AC-8, and a real two-verify race on this machine.

**Phase 2: share on what the label reads.** Add a per-label input fingerprint
consulted by `shares`, leaving `SnapshotTree` and the certificate untouched. State
lint's input list in one place beside the label. Verify: AC-4, AC-5, with one test
per listed input.

**Phase 3: answer from a run in flight.** Add the `verify reds` action over the
stage logs, reusing `declaredGroups`. It reports which stages have not yet
reported, and never renders an unfinished run as passed. Verify: AC-6, AC-7, and
`structuralGateReds` pointed at a directory holding 3 of 44 logs.

## Work Not Done

Each item below is real and is NOT in this spec's scope. None is a deferral of
this spec's own criteria.

| Item | Where it belongs |
|------|------------------|
| Waiting time is never recorded: `Release` persists only time since admission, and `Ticket.Waited` reaches `job.Report.WaitedSeconds` and is dropped | Its own spec. Without it, no future measurement can separate queueing from work |
| `runUnitRaceChanged` (`internal/le/verify/deps/verifydeps.go`) notes an empty selection on stderr and exits 0, so a certificate can carry a green stage that ran no test | `plan/journal/green-that-could-not-have-been-red.md` has the class; the fix is a spec |
| `PhaseResult` and `AllTestsReport` (`internal/le/qemu/alltests_report.go`) record a phase's exit code and no test count, so a phase that runs zero tests and exits 0 renders `ALL PHASES PASSED` | Same class, same route |
| `publishChangeScope` (`internal/le/verify/engine/scope.go`) writes `scope-packages.txt` and names `ZE_VERIFY_SCOPE_PACKAGES`, and no non-test code reads it | Either wire it or delete it; an artifact nobody consults is a claim nobody checks |
| `checkScoped` (`internal/le/verify/engine/status.go`) stales every scoped path on any HEAD move, so an identical foreign edit is tolerated uncommitted and voids everything once committed | Becomes the binding constraint once phases 1 and 2 let a run finish |
| `spec-verify-scope-5` phases 2 to 5 select suites from execution coverage; its own phase-1 measurement records A-1, A-2 and A-3 broken, with every recording suite covering 423-513 of 646 packages and instrumentation costing +52% wall clock | That spec. If it continues, it should select on the import graph `Scope.resolveSelector` already walks |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named in Work Not Done, in its own bucket

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
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-verification-answers-one-agent.md` only

---

## Implementation Summary

### What Was Implemented
- **Phase 1 (`a5bac9f20`).** `runCurrent` (`internal/le/verify/current.go`) and the
  `verify worktree` lifecycle (`internal/le/verify/lifecycle.go`) claim the
  `verify` label through `job.Admit` before any work. `slotFor` opens the
  ticket's log and answers the `verifyengine.Slot` the engine writes progress to.
  `nameJobParent` (`internal/le/verify/engine/run.go`) sets and restores
  `job.ParentKey` around the stage loop, so the in-process `verify lint/run`
  stage admits `KindInside`. `tell` copies each finished stage to the slot log
  and `beat` writes one line a minute inside a stage, which is the growth the
  registry's stall breaker reads as liveness.
- **Phase 2 (`0af9c6602`).** `InputHash(root, label)` (`internal/le/job/treehash.go`)
  fingerprints the inputs one label READS, and `(*Admission).shares`, `take` and
  `Admit` compare it instead of `TreeHash`. `lintIgnores` declares the seven
  trees a lint does not read, beside `job.LintLabel`. A label with no declaration
  falls through to `TreeHash`, so nothing changed for any label but `lint`.
  `SnapshotTree`, `DirtyManifest` and the freshness certificate are untouched.
- **Phase 3 (`df930475b`).** `./le verify reds file <path>`
  (`internal/le/verify/reds.go`) reads whatever stage logs a run has written and
  answers `named`, `undetermined`, `not-named` or `no-run`, carrying `stages`,
  `reported` and `pending` in every answer. `declaredGroups` and `artifactGroup`
  became `DeclaredGroups` and `Group`; the attribution predicate moved to
  `failuregroup` as `CarriesPaths`, `CleanPath` and `Covers`, and
  `structuralGateReds` reads it there.

### Bugs Found/Fixed
- **The `internal/le/verify/engine` test build was broken at HEAD.** Phase 3
  renamed `declaredGroups` to `DeclaredGroups` and left four call sites in
  `internal/le/verify/engine/artifacts_test.go` on the old name, so the package
  failed typecheck. `./le verify worktree`, run by this closure, reported
  `undefined: declaredGroups (typecheck)` against that file at its lint stage.
  The four call sites are renamed in this commit; `go vet` and the package's own
  tests are green over the repair.
- **A stale claim on `(*Admission).Admit`.** Its doc said a second `Admit` in the
  same process never sees the first as its parent, because only a child receives
  the environment. Phase 1's `nameJobParent` sets `job.ParentKey` in-process for
  exactly that purpose, which is AC-2. The paragraph now names both writers
  (`ai/rules/stale-comments.md`).

### Documentation Updates
- `docs/architecture/testing/verify-freshness-scope.md` - two statements the code
  contradicted are repaired (heavy verification now admits ITSELF rather than
  entering through `job run`; `scope-packages.txt` is written and read by nobody),
  and a new "Asking before the run ends" section covers the query. Anchors added
  for `Slot`, `nameJobParent`, `runCurrent`, `jobLabel`, `slotFor`, `readReds`,
  `verdictOf`.
- `docs/contributing/running-commands.md` - a section for `verify reds` with the
  four verdicts, their exit codes, and the sentence that it is a query and never
  a certificate.
- `docs/contributing/testing.md` - what a lint's inputs are, that the declaration
  EXCLUDES, and why that direction is the safety property. Anchor added for
  `InputHash`, `lintIgnores`.
- `docs/functional-tests.md` - the runner suite row names the in-flight
  failure-query fixture.
- `./le doc check verify` is RED for reasons outside this spec: the published
  `gh-pages` command-equivalent surface and `ai/RFC-REQUIREMENTS.md`, both owned
  by other sessions. No failing line names a file of this spec.

### Deviations from Plan
- **A-2 was broken in both directions and the design was inverted.** The spec
  asked for an INCLUSION list of lint's inputs. Phase 2 declares an EXCLUSION
  list, because a missed inclusion shares a stale verdict while a missed
  exclusion only duplicates a run. `feature-gates.txt` is not read by lint
  (verified: no reference to it anywhere under `internal/le/verify/lint/`), and
  `go.mod`, `go.sum` and `vendor/modules.txt` are.
- **`test/runner/verify-shares-one-run.ci` was not written.** It is homed in
  `plan/spec-two-verifies-share-one-run-end-to-end.md`; see Work Not Done.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 stated lint's inputs as a list of four, `feature-gates.txt` among them | `feature-gates.txt` is not read by lint, and `go.mod`, `go.sum`, `vendor/modules.txt` and embedded assets are. An inclusion list fails toward a stale shared verdict | Phase 2 read `parseConfigTags` and `gotoolchain.New` before writing the list | The fingerprint EXCLUDES, and `TestTheTreesTheLintLabelIgnoresHoldNoGoFile` walks the real checkout to keep the declaration true |
| approach | Phase 3 renamed an unexported symbol and did not follow it into its own package's test file | Four call sites in `artifacts_test.go` kept the old name, so the package failed typecheck at HEAD | This closure's `./le verify worktree`, at the lint stage | Renamed in commit A. The lesson is the journal row: the set a rename owes is what the rename can REACH, not the files the diff already touched |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `le verify` enters admission so duplicate whole-tree runs collapse | Done | `runCurrent`, `admitWorktree` (`internal/le/verify/current.go`, `lifecycle.go`) | Observed live during this closure: one `verify` entry held in `tmp/.ze-jobs/` while the run worked |
| Sharing is decided on the inputs a label reads | Done | `InputHash`, `lintIgnores` (`internal/le/job/treehash.go`); `shares`, `take`, `Admit` | |
| An agent gets a verdict about its own change without starting or waiting for a whole-tree run | Done | `Reds`, `readReds`, `verdictOf` (`internal/le/verify/reds.go`) | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestTwoVerifiesShareOneRun` (`internal/le/verify/current_test.go`) | Two real processes; the follower runs no stage and the marker file stays absent |
| AC-2 | Done | `TestNestedLintStageDoesNotQueueBehindItsParent` | Asserts `job.KindInside` for the nested lint |
| AC-3 | Done | `TestTwoVerifiesShareOneRun` | The follower's own `exec.ExitError` status, no pipe and no wrapper |
| AC-4 | Done | `TestLintShareSurvivesAnUnrelatedFile` | Two helper PROCESSES, identical argv; a journal row leaves one run serving both |
| AC-5 | Done | `TestLintShareVoidedByAGoChange`, `TestEveryInputTheLintLabelReadsVoidsItsShare` | Ten rows, one per input a lint verdict can depend on |
| AC-6 | Done | `TestRedsAnswersFromAnUnfinishedRun`, `test/runner/verify-reds-in-flight.ci` | 3 whole logs and 1 caught mid-write |
| AC-7 | Done | `TestRedsRefusesToAnswerWithNoRun` | Answers `no-run` at a non-zero status |
| AC-8 | Done | `TestRunCurrentFullAndChangedModes` | Same populations, same certificates, same artifacts under admission |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestVerifyCurrentEntersAdmission` | Done | `internal/le/verify/current_test.go` | |
| `TestTwoVerifiesShareOneRun` | Done | `internal/le/verify/current_test.go` | |
| `TestAttachedVerifyExitsWithHolderCode` | Changed | folded into `TestTwoVerifiesShareOneRun` | The same case reads both assertions off one race; a second race to re-assert the status buys nothing |
| `TestNestedLintStageDoesNotQueueBehindItsParent` | Changed | `internal/le/verify/current_test.go`, not `engine/stages_test.go` | The nested admission is asked from a stage of an admitted `runCurrent`, which lives in the `verify` package |
| `TestLintShareSurvivesAnUnrelatedFile` | Done | `internal/le/job/contention_test.go` | |
| `TestLintShareVoidedByAGoChange` | Done | `internal/le/job/contention_test.go` | |
| `TestEveryInputTheLintLabelReadsVoidsItsShare` | Done | `internal/le/job/contention_test.go` | |
| `TestTheTreesTheLintLabelIgnoresDoNotVoidItsShare` | Done | `internal/le/job/contention_test.go` | |
| `TestTheTreesTheLintLabelIgnoresHoldNoGoFile` | Done | `internal/le/job/contention_test.go` | |
| `TestALabelThatDeclaresNoInputsIsFingerprintedOverTheWholeCheckout` | Done | `internal/le/job/contention_test.go` | |
| `TestRedsAnswersFromAnUnfinishedRun` | Done | `internal/le/verify/reds_test.go` | |
| `TestRedsRefusesToAnswerWithNoRun` | Done | `internal/le/verify/reds_test.go` | |
| `verify-reds-in-flight` | Done | `test/runner/verify-reds-in-flight.ci` | |
| `verify-shares-one-run` | Skipped | absent | Homed in `plan/spec-two-verifies-share-one-run-end-to-end.md`. Needs the owner's word |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/verify/current.go` | Done | |
| `internal/le/verify/lifecycle.go` | Done | |
| `internal/le/verify/engine/run.go` | Done | |
| `internal/le/job/registry.go` | Done | |
| `internal/le/job/treehash.go` | Done | |
| `internal/le/verify/engine/artifacts.go` | Done | |
| `docs/architecture/testing/verify-freshness-scope.md` | Done | |
| `internal/le/verify/engine/stages.go` | Changed | The `changedStages` comment was accurate and needed no edit; the statement the code contradicted was on the doc page, and that page is repaired |
| `internal/le/verify/reds.go` | Done | |
| `test/runner/verify-reds-in-flight.ci` | Done | |
| `test/runner/verify-shares-one-run.ci` | Skipped | See Work Not Done |

### Audit Summary
- **Total items:** 33
- **Done:** 29
- **Partial:** 0
- **Skipped:** 2 (`verify-shares-one-run` and its `.ci`, one item counted in two tables; needs the owner's word)
- **Changed:** 3 (recorded in Deviations and in the tables above)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An agent gets a verdict about its own change without starting or waiting for a whole-tree run | functional | `test/runner/verify-reds-in-flight.ci` drives the real `le` binary against a checkout holding 3 of a full run's stage logs and reads four answers: `undetermined` with the unreported remainder for an untouched path, `named` with the stage for a reddened one, the same payload under `\| json`, and `no-run` for a checkout with no run. Both guards were broken and observed RED before the phase landed |
| Duplicate whole-tree runs collapse into one | functional | `TestTwoVerifiesShareOneRun`: a second operating-system process attaches, writes no marker (so it ran no stage), and exits with the holder's own status read from `exec.ExitError` with no pipe between. Confirmed on the real binary during this closure: the `verify` label held one entry while the run worked, and the nested `verify lint/run` created none |
| A lint's share is not voided by work it does not read | functional | `TestLintShareSurvivesAnUnrelatedFile` and `TestLintShareVoidedByAGoChange` run two helper PROCESSES with identical argv, so only the input change explains the outcome: the journal row leaves the marker at one line, the Go change leaves it at two. `TestEveryInputTheLintLabelReadsVoidsItsShare` carries one row per input, and `TestTheTreesTheLintLabelIgnoresHoldNoGoFile` walks the real checkout to keep the exclusion true |
| A verify run stays alive across a stage longer than the stall window | measurement | The registry breaks a holder whose log has not grown for 1800 s. During this closure's own `./le verify worktree`, the slot log carried `### Stage running: verify lint/run, 660s`, one line a minute, while the stage rendered nothing |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| `test/runner/verify-shares-one-run.ci`: the two-verify share proven through the `le` BINARY | `currentHere` hands `runCurrent` the native in-process dispatcher (`actionRunner`, `internal/le/verify/actions.go`), so a real `le verify current mode changed` runs all 45 stages, beginning with `verify lint/run` and the six-part staticcheck matrix at 1220 to 2341 s each. A scenario driving two of those runs a whole verification inside the suite a verification runs. The package-level proof exists and drives two real processes; what is missing is a seam that reaches the binary over a bounded population without giving the product a second code path | `plan/spec-two-verifies-share-one-run-end-to-end.md` |
| Waiting time is never recorded: `Release` persists only time since admission | Named in the spec's own Work Not Done at design time as out of scope | Its own spec; not written, and the owner decides whether it runs |
| `runUnitRaceChanged` exits 0 on an empty selection | Out of scope, same as at design time | `plan/journal/green-that-could-not-have-been-red.md` holds the class |
| `PhaseResult` and `AllTestsReport` record no test count | Out of scope, same class | Same route |
| `publishChangeScope` writes `scope-packages.txt` and nothing reads it | Out of scope. This spec repaired the doc page that claimed otherwise rather than the artifact | Either wire it or delete it; not written |
| `checkScoped` stales every scoped path on any HEAD move | Out of scope. It becomes the binding constraint now that a run can finish | Not written |
| `signalContext` and the stage context are two halves of one pair: a TERM to a running `le verify` is not observed until the stage in flight ends | Cancellation is not this spec's subject and no AC reaches it | `plan/journal/guard-added-to-one-half-of-a-pair.md`, written by phase 1 |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/verification-answers-one-agent-zeclose-vspec.md` (15 files) |
| `./le spec session review check` | `OK (clean, hashes match)`, exit 0 |
| Rounds | 2 |
| Reviewer lenses used | completeness against AC and Wiring rows; the ten feature-specific Critical Review rows; `ai/rules/principles.md` zero-as-answer; `ai/rules/stale-comments.md`; `ai/rules/no-layering.md`; `docs/contributing/ze-go-style.md`; security (path handling, signal targets, registry TOCTOU); `ai/rules/documentation.md` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `declaredGroups` was renamed to `DeclaredGroups` and four call sites in the package's own test file kept the old name, so `internal/le/verify/engine` failed typecheck at HEAD. `./le verify worktree` reported it at the lint stage | `internal/le/verify/engine/artifacts_test.go`, in `TestDeclaredGroupsAreReadBackWhenTheCountAgrees`, `TestAMalformedGroupLineBecomesAGroupThatSaysSo`, `TestEveryDoubtAboutTheDeclaredSetRefusesIt` and `TestAStageLogThatCannotBeReadDeclaresNothing` | Renamed to `DeclaredGroups`. `go vet -tags ze_core,ze_le ./internal/le/verify/...` exits 0 and the package's tests pass |
| 2 | ISSUE | `(*Admission).Admit`'s doc claims a second `Admit` in the same process never sees the first as its parent, "because a parent is found through the entry named in the environment, which only a child receives". `nameJobParent` sets that variable in-process, which is the whole of AC-2 | `internal/le/job/job.go`, above `Admit` | The paragraph names both writers of `ParentKey`, `childEnviron` and `nameJobParent`, and says what a holder that names nothing leaves behind |
| 3 | ISSUE | `test/runner/verify-shares-one-run.ci` is in Files to Create and in the Functional Tests table, and does not exist | `test/runner/` | Homed in `plan/spec-two-verifies-share-one-run-end-to-end.md`, written in this commit, with the measurement that says why it is not one edit. Dropping it is the owner's decision and this closure does not take it |

Notes recorded and not blocking: `stopBeat()` is called rather than deferred, so
a stage that panics leaks one goroutine into a process that is ending anyway;
`verdictOf` states three guards in one `||` where `docs/contributing/ze-go-style.md`
prefers three guard clauses; `df930475b` carried doc hunks belonging to two
sibling agents, which its own message states.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/verify/reds.go` | Yes | `ls -la` answers `-rw-rw-r-- 1 thomas thomas 13131 Sep  5 18:04 internal/le/verify/reds.go` |
| `test/runner/verify-reds-in-flight.ci` | Yes | `ls -la` answers `-rw-rw-r-- 1 thomas thomas 1531 Sep  5 17:55 test/runner/verify-reds-in-flight.ci` |
| `test/runner/verify-shares-one-run.ci` | No | `ls` answers `cannot access 'test/runner/verify-shares-one-run.ci': No such file or directory`. Homed in Work Not Done |
| `plan/spec-two-verifies-share-one-run-end-to-end.md` | Yes | Written in this commit; passes the native spec validation hook |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3, AC-8 | The entry point admits, the nested stage is inside, the follower takes the holder's status | `./le job run label unit-vclose command go test -tags ze_core,ze_le -count=1 ./internal/le/verify/...` answers `ok github.com/ze-software/ze/internal/le/verify 25.077s` |
| AC-4, AC-5 | A lint shares across an unread file and not across a Go change | the same run answers `ok github.com/ze-software/ze/internal/le/job 33.088s` |
| AC-6, AC-7 | The in-flight query answers and never renders an unfinished run as passed | the same run answers `ok github.com/ze-software/ze/internal/le/verify 25.077s` for `reds_test.go`, and `ok github.com/ze-software/ze/internal/le/commit 5.677s` for `TestStructuralRedsSeeNothingUntilARunPublishesItsIndex` |
| AC-1 (live) | One verify holds one slot and the nested lint holds none | `tmp/.ze-jobs/` during this closure's own run held one `verify` entry and no `lint` entry from that process; the one `lint` entry present belongs to session `zeimpl-eaprevoke`, traced through its parent process |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le verify reds file <path>` against a run in flight | `test/runner/verify-reds-in-flight.ci` | Yes. Read it and its driver `internal/test/fixture/misc_fixture_runner_reds.go`: it builds a checkout with 3 stage logs, runs the real `le` binary four times, and asserts `undetermined`, `named`, the json keys, and `no-run` |
| `./le verify current mode full` -> `Admit` | package test, no `.ci` | `TestVerifyCurrentEntersAdmission` reads the registry from inside a stage and asserts one entry, `ParentKey` naming it, no entry left, and a duration row |
| a second concurrent `./le verify` -> `attach` | absent `.ci` | `TestTwoVerifiesShareOneRun` drives two operating-system processes. The `.ci` is homed in Work Not Done |
| the nested lint stage -> `insideParent` | package test | `TestNestedLintStageDoesNotQueueBehindItsParent` asserts `job.KindInside` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `Admit` has three call sites outside the job package: `verifylint.runHere` and the two entry points phase 1 added. `TestNestedLintStageDoesNotQueueBehindItsParent` asserts `KindInside` |
| A-2 | broken | `grep -rn "feature-gates" internal/le/verify/lint/` is empty, so lint does not read it; `parseConfigTags` takes the tags from `.golangci.yml`. The fingerprint EXCLUDES instead of listing (`lintIgnores`) |
| A-3 | confirmed-with-a-guard | `runMode` writes each stage log in one `os.WriteFile` after the stage returns, and the body ends `### Stage result: <name> exit=N`. `stageResult` requires that line, so a log met mid-write counts as not reported. `TestRedsAnswersFromAnUnfinishedRun` carries a truncated fourth log |
| A-4 | confirmed | Stronger than the spec's own method. `InputHash` falls through to `TreeHash` for a label with no declaration, and `labelIgnores` holds only `LintLabel` (`internal/le/job/treehash.go`). So a `verify` attach happens only when the WHOLE-tree fingerprint matched, and the holder judged exactly the tree the follower has. `TestALabelThatDeclaresNoInputsIsFingerprintedOverTheWholeCheckout` pins the fall-through |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 3, 17: `docs/contributing/running-commands.md` for `verify reds` | The four verdicts and their exit codes match `verdictOf` and `redsHere` (`internal/le/verify/reds.go`) read line by line | Yes |
| 6: `docs/architecture/testing/verify-freshness-scope.md` | The two false statements are repaired against `runCurrent` and `publishChangeScope`; anchors name `Slot`, `nameJobParent`, `readReds`, `verdictOf`, each declared where named | Yes |
| 10: `docs/functional-tests.md` | The runner row names the in-flight failure-query fixture, which exists at `test/runner/verify-reds-in-flight.ci` | Yes |
| 12: `docs/architecture/core-design.md`, the admission seam | No. `ai/CODE-TO-DOCS.md` routes `internal/le/job/registry.go` and `treehash.go` to `docs/contributing/testing.md`, and `internal/le/verify/current.go`, `lifecycle.go` and `reds.go` to `docs/architecture/testing/verify-freshness-scope.md`. Both are updated. `grep -n "admission" docs/architecture/core-design.md` returns one row of a command-group table, which no change here touched | Yes, as No |
| 15: the `le` command surface | `TestActionsDeclareWorktreeCurrentRedsAndList` asserts the four verbs in declaration order, and `TestRedsGrammarPutsTheKeywordBeforeTheValue` asserts `reds` is listed and does not write | Yes |
| 16: source anchors over changed files | `./le doc check verify` names six undeclared anchors, none in a file this spec changed; its failures are the `gh-pages` command-equivalent surface and `ai/RFC-REQUIREMENTS.md`, owned by other sessions | Yes |
| 1, 2, 4, 5, 7, 8, 9, 11, 13, 14 | No. This is development tooling: no daemon config, no RPC, no plugin, no wire format, no RFC behavior, no daemon comparison, no route metadata, no counter | Yes, as No |

## Core Insight

An exclusion list and an inclusion list describe the same set and fail in
opposite directions. Naming what a job READS shares a stale verdict the day
somebody adds an input nobody listed. Naming what it DOES NOT READ costs a
duplicate run instead, and that cost lands on the session that pays it rather
than on the next reader of a green that could not have been red.
