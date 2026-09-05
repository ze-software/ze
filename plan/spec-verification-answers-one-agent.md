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
| `verify-shares-one-run` | `test/runner/verify-shares-one-run.ci` | Two verifies started together produce one run and two identical verdicts | |
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
