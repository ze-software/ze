# Spec: the gate stops rebuilding the cache it has, and says so when it cannot judge

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A full device makes Ze's gates report failures that read as code defects. The
class file `plan/journal/full-disk-false-red.md` holds seven rows, the most
recent three dated 2026-09-01, 2026-09-02 and 2026-09-03, and every row is a
session that spent time reading a red the tree did not earn.

The run of 2026-09-03 is the shape of it. `./le verify worktree` printed
`error: staticcheck matched no packages; matrix could not be judged`, then four
`warning: "./..." matched no packages`, then `verify-worktree: full exit=2`, and
last `verify-worktree: save logs failed: ... no space left on device`. The
extracted tree held no packages because the extraction had no disk, so the run
judged NOTHING and answered with the same exit code a real failure answers. A
reader who sees `exit=2` reads a verdict. `ai/rules/principles.md` names this
exactly: a check that cannot see its subject has cleared nothing, and it must
not answer as though it had.

The disk goes because the gate itself is one of the largest consumers on the
machine. `tmp/verify-worktree` held 13.5 GB in two directories during that run,
8.3 GB of it an abandoned worktree from earlier the same day that the sweep
deliberately preserves because it carries uncommitted changes. Preserving it is
right. Never saying it is there, while every later run needs 5 GB of its own, is
what turns a correct choice into a machine that fills.

The device goes because the gate rebuilds a cache it already has. In the
checkout, `cache` is a symlink to the shared Go build cache. In every verify
worktree it is a plain directory: `Overrides` points GOCACHE at
`<root>/cache/go-cache` (`internal/le/gotoolchain/gotoolchain.go`, `GoCache`),
and `root` inside a worktree run is the worktree. Neither `lifecycle.go` nor
`cleanup.go` mentions `cache`. So every run builds a private Go build cache
from cold, measured at 7.4 GiB and 6.7 GiB in the two worktrees on this
machine, against a source tree of 0.6 GiB, and then deletes it. That is about
90 percent of each worktree, it is rebuilt once per run, and it is also why a
run pays twenty minutes of cold compilation.

The verdict is defeated separately, and already is. `run` prints
`verify-worktree: <mode> exit=<code>` from `report.Code`, and then two later
branches overwrite that field to 1: the log-save failure branch, and the
deferred cleanup branch that runs after the function returns
(`internal/le/verify/lifecycle.go`). The run of 2026-09-03 printed `exit=2` and
did not exit 2. One layer down, the staticcheck matrix already separates 0, 1
and 2, where its 2 means "could not be judged" (`runCheck`,
`internal/le/staticcheckfeaturematrix/actions.go`), and `runMode` collapses any
non-zero stage code to 1 (`internal/le/verify/engine/run.go`). The run already
knows it judged nothing, in two places, and discards it in both.

The goal has two halves. The gate stops consuming the disk it does not need,
and a gate that judged nothing never answers as though it judged.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/principles.md` - the rule this defect violates
  → Constraint: "A value that is silently wrong MUST NOT be reachable: code
    that cannot answer MUST say so." An exit code that means "judged and failed"
    MUST NOT be returned by a run that judged nothing
- [ ] `docs/contributing/running-commands.md`, "When the disk is full" - the
  operator playbook this spec automates
  → Constraint: `cache/` is a symlink onto another filesystem, so `df` on the
    checkout answers about the wrong device. `stat -f cache/go-cache` is the
    read that answers, and `./le scratch cache-clean` is the remedy
- [ ] `plan/journal/full-disk-false-red.md` - seven occurrences and their shapes
  → Decision: three distinct surfaces appear across the rows: the Go build cache
    (`go clean -cache` fixed 2026-07-30), the root filesystem under the verify
    worktrees and the linker (2026-08-15, 2026-08-20, 2026-09-01), and the
    colima data disk under the QEMU and interop runs (2026-09-02, 2026-09-03).
    This spec takes the first two, which are `./le`'s own; the colima disk is a
    different owner and is named in Known Limitations

**Key insights:** (minimal context to resume after compaction)
- The worktree's `cache/` is a REAL directory, not the checkout's symlink.
  VERIFIED: `ls -ld` on both worktrees and on the checkout, 2026-09-03. Nothing
  in `internal/le/verify` creates a link, and no page in `docs/` mentions the
  worktree cache, so nothing records the cold build as deliberate.
- `runMode` (`internal/le/verify/engine/run.go`) sets `report.Code = 1` for any
  non-zero stage code, which destroys the staticcheck matrix's deliberate 2.
  The third outcome exists at the stage level already.
- `report.Code` is printed and THEN overwritten, twice. The printed verdict and
  the process exit code disagree today.
- `verify worktree` never writes a certificate the commit path can read: it
  runs the engine with the worktree as `root`, so `WriteCertificate` writes
  inside the worktree and it dies with it. The root certificate comes only from
  `./le verify current` and `./le verify status write`.
- ENOSPC does NOT arrive as stage failure text. `validateResult` records only
  `action exited N`; the device error is a live typed error discarded at the
  `os.WriteFile` / `os.MkdirAll` / `os.CopyFS` sites.
- `internal/appliance/kernelbuilder/space.go` carries
  `// Design: plan/journal/full-disk-false-red.md` and already implements a
  measured free-space guard for the Docker kernel build. It is the shape any
  later guard mirrors, and it is the reason this spec needs none of its own.

## Current Behavior (MANDATORY)

**Source files read:** (READ during design, 2026-09-03; each constraint below
was verified at the producing function, not inferred from a caller)
- [x] `internal/le/gotoolchain/gotoolchain.go` - `GoCache`, `Overrides`.
  → Constraint: `GOCACHE` is `<root>/cache/go-cache`, and the package comment
    claims that path "is symlinked out of tree" and so "survives a scratch
    wipe". That is true of the checkout and FALSE of every verify worktree.
    The comment is stale for the worktree case and is repaired by this work
- [x] `internal/le/verify/lifecycle.go` - `Run`, `run`, `ownerMarker`.
  → Constraint: `run` creates only `<worktree>/tmp` and the owner marker. It
    never links `cache/`, so the toolchain override creates a cold one
  → Constraint: `report.Code = verification.Code` is printed immediately, and
    then overwritten to 1 by the log-save branch and again by the deferred
    cleanup branch. The printed verdict is a snapshot that later branches
    contradict, so the ordering fix alone does not make the line honest
  → Decision: the run already prints a third tell, "verification wrote no
    stage logs ... so it stopped before the first stage", and changes no code
    when it does. That is an unjudged run reported as a failure
- [x] `internal/le/verify/engine/run.go` - `runMode`, `validateResult`.
  → Constraint: `if stageReport.Code != 0 && report.Code == 0 { report.Code = 1 }`
    flattens every non-zero stage code to 1. Engine codes are 0, 1, 2 and 130,
    where 2 already means the run itself broke. 3 is free
  → Constraint: `validateResult` records `action exited N` and nothing else.
    `StageReport` carries no output field and `Report.Console` is `json:"-"`,
    so a stage's ENOSPC text never reaches the report
- [x] `internal/le/staticcheckfeaturematrix/actions.go`, `judge.go` - `runCheck`, `Judge`.
  → Decision: `runCheck` keeps 0, 1 and 2 apart deliberately, 2 meaning the
    matrix could not be judged. It also returns a legitimate PASS with no rows
    when sibling parts hold them, so an empty ROW population is correct and an
    empty PACKAGE population is not. AC-2 is scoped to the second
- [x] `internal/le/verify/engine/status.go`, `internal/le/commit/verification.go`
  - `WriteCertificate`, `CheckCertificate`, `verificationState`.
  → Constraint: `verify worktree` passes the WORKTREE as `root`, so its
    certificate is written inside the worktree and deleted with it. The commit
    path reads only the checkout-root certificate, which `./le verify current`
    and `./le verify status write` write. `Freshness.Fresh` is a bool and
    cannot carry a third state
- [x] `internal/le/scratch/scratch.go` - `cacheTarget`, `scratchTarget`, `Ensure`, `ensureSymlink`.
  → Decision: `cacheTarget` ignores `m.Root` and answers `$XDG_CACHE_HOME/ze`
    or `~/.cache/ze`, so linking a worktree's `cache` reaches the SAME target
    the checkout uses. `scratchTarget` hashes the root, so `tmp` is
    per-checkout and MUST NOT be linked by this work
  → Constraint: `.gitignore` carries `/cache`, and the existing worktree with a
    real 7.4 GiB `cache/` reports a clean `git status --porcelain`. A symlink
    there is equally invisible, so it cannot make `sweepAbandoned` preserve
    every worktree as dirty. VERIFIED by running both commands
- [x] `internal/le/verify/cleanup.go` - `sweepAbandoned` and its callers.
  → Constraint: a dirty abandoned worktree is preserved by design; this spec
    MUST NOT delete one, and MUST NOT weaken that test. It runs after
    `os.MkdirAll(base)` and BEFORE `git worktree add`
  → Constraint: `Report` carries `Swept` and no preserved field, and the
    existing diagnostic names only `entry.Name()`. AC-5 needs a new field
- [x] `internal/core/diskspace/diskspace.go` - `Free`, `GiB`.
  → Decision: `Free` returns `Bavail*Bsize` and exposes no device id, so two
    independent floors double-count when both paths share a device, as they do
    on this machine. This spec adds no preflight, so it takes no dependency
    on that; a later guard mirrors `internal/appliance/kernelbuilder/space.go`

**Behavior to preserve:**
- A real red stays a red with the same exit code it has today.
- A dirty abandoned worktree is never destroyed.
- `./le verify worktree` keeps running every stage it runs today, in order.
- A matrix part dealt no rows still PASSES; an empty row population is correct.

**Behavior to change:**
- A verify worktree shares the checkout's Go build cache instead of building a
  private one from cold.
- A run that judged nothing answers a distinct code, and `runMode` stops
  flattening a stage's "could not judge" into "a stage failed".
- The printed verdict line states the code the process actually exits with.
- A preserved abandoned worktree is reported with its path, age, size, and the
  SHAPE of its dirt, which is what tells an operator whether it is safe to
  remove.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le verify worktree`, and every gate that extracts a tree or runs the lint
  matrix.

### Transformation Path
1. Extraction links `<worktree>/cache` to the checkout's `cache`, so the
   toolchain override resolves GOCACHE to the shared cache and no private one
   is built.
2. Report what is holding the disk: every preserved abandoned worktree, with
   path, age, size, and the shape of its dirt.
3. Run the stages as today.
4. Classify: a stage that reports it could not judge keeps that code instead of
   being flattened to 1, and an ENOSPC at a write site is recognised by
   `errors.Is(err, syscall.ENOSPC)` at the site that holds the typed error.
5. The verdict line states the code the process exits with, printed after every
   branch that can change it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Worktree ↔ shared cache | one symlink at extraction; GOCACHE resolves through it | Yes: `GoCache`, `Overrides` read 2026-09-03 |
| Stage ↔ verdict | a stage's "could not judge" code survives `runMode` | Yes: `runMode` read 2026-09-03 |
| Write site ↔ verdict | `errors.Is(err, syscall.ENOSPC)` where the typed error lives | Yes: `validateResult` carries no output field |
| Gate ↔ operator | the retention line names path, age, size and dirt shape | Yes: `sweepAbandoned` read 2026-09-03 |

### Integration Points
- `internal/le/scratch/scratch.go` - a cache-only link entry point, reusing
  `cacheTarget` and `ensureSymlink`
- `internal/le/verify/lifecycle.go` - the cache link, the third outcome, and
  the verdict line printed last.
- `internal/le/verify/cleanup.go` - reporting what is preserved.
- `internal/le/verify/engine/run.go` - `runMode` stops flattening, and the
  write sites classify ENOSPC.
- `internal/le/gotoolchain/gotoolchain.go` - the stale package comment about
  `cache/` being symlinked out of tree.

NOT an integration point: `internal/le/commit/`. A `verify worktree` run writes
its certificate inside the worktree, so the commit path never reads it. AC-6 is
scoped to the run that does write the root certificate.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The cache link goes through `scratch.Manager.EnsureCache`, which reuses `cacheTarget` and `ensureSymlink`; `lifecycle.go` writes no `os.Symlink` of its own |
| No unintended coupling (components stay isolated) | Yes | `internal/le/verify` gains one import, `internal/le/scratch`, which is a sibling le package with no dependency back |
| No duplicated functionality (extends existing, does not recreate) | Yes | One declaration of "a full device defeated the run" (`verifyengine.Defeated`), consumed by `brokenCode` in the engine and `defeatedCode` in the lifecycle. `diskspace.GiB` renders the size rather than a second formatter |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire path; every new string is a `textbuf.Buffer` line built once |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, no new registry entry, and no central enumeration edited. Exit code 3 is a constant beside the three already in `verifyengine` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A run's worktree costs about 8 GiB, and about 90% of it is a private Go build cache built from cold | MEASURED 2026-09-03: worktrees of 8.27 and 7.56 GiB, whose `cache/go-cache` are 7.4 and 6.7 GiB, against a 0.6 GiB source tree | The cache is not the dominant consumer and sharing it frees little | CONFIRMED by `du -sk` on both worktrees. Uncertainty stated: n=2, one machine, terminal sizes rather than peaks |
| A-2 | Every ENOSPC defeat reaches the report as failure TEXT rather than a typed error | The 2026-09-03 run surfaced it as `save logs failed: ... no space left on device` | A text match misses a defeat that arrived another way, and the run reports a verdict again | BROKEN. `validateResult` records only `action exited N`; `StageReport` has no output field and `Report.Console` is `json:"-"`. The device error is a live typed error discarded at the `os.WriteFile` / `os.MkdirAll` / `os.CopyFS` sites, so the classifier goes THERE with `errors.Is` |
| A-3 | "matched no packages" always means the population was empty rather than a legitimate zero | `./...` matches nothing only when the tree is missing or unextracted | A legitimate empty population is reported as unjudged, which is noise | CONFIRMED for the PACKAGE population at `Judge`. Caveat absorbed: `runCheck` returns a legitimate pass for an empty ROW population, so AC-2 is scoped to packages |
| A-4 | The commit path can tell a debt row not to be owed for an unjudged run | `./le commit create` reports `verify=STALE` and writes debt rows | An unjudged run either clears debt it did not earn, or adds debt for a run that never happened | BROKEN AS WRITTEN. `verify worktree` writes its certificate inside the worktree, so the commit path never sees it. Mechanism confirmed for the run that DOES write the root certificate: `Freshness.Fresh` is a bool and is the blocking type change |
| A-5 | One Go build cache is safe for concurrent verify worktree runs | Go documents the build cache as safe for concurrent use, and the checkout already shares one across every concurrent le action | Two concurrent runs corrupt or thrash one cache | CONFIRMED to the depth the owner asked for. `TestAWorktreeSharesTheCheckoutBuildCache` asserts `<worktree>/cache` is a symlink to the per-user target and that `gotoolchain.GoCache(worktree)` resolves to the shared `go-cache`. The concurrency itself is NOT exercised, and that limit stays in Known Limitations |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The shared cache makes a run's result depend on what an earlier run left behind, so a stale entry turns a red into a green | A stage passing in a worktree that fails in the checkout | The Go build cache is keyed by content and toolchain, so a stale entry is a Go defect rather than a Ze one. The checkout already takes this risk on every le action. Early signal is a divergence between `verify worktree` and `verify current` on one SHA |
| R-2 | The unjudged outcome is treated as a pass by something downstream | A commit script accepting an unjudged run as green | Code 3 is neither 0 nor any existing failure code, and `CheckCertificate` already stales on ANY non-zero exit, so an unjudged run cannot read as fresh |
| R-3 | The ENOSPC classifier matches a legitimate failure that merely mentions the phrase | A real red reported as unjudged | Classify with `errors.Is(err, syscall.ENOSPC)` on the typed error at the write site, never by matching text anywhere |
| R-4 | Reporting preserved worktrees becomes noise every run | Operators ignoring the line | The line already exists for a preserved worktree; this adds fields to it rather than a new report, so the frequency does not change |
| R-5 | The cold cache was load-bearing for the golangci-lint export-data fault the `gotoolchain` comment describes | A toolchain mismatch that the worktree run stops catching | Nothing states that: `lifecycle.go` never mentions `cache`, and no page in `docs/` describes the worktree cache. If it WAS deliberate, the deliberate version is a cold-cache option, not an accident of a missing symlink |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A gate refuses to start, or a real red is reported as unjudged. Both are visible immediately and neither reaches a shipped binary |
| How is it reverted? | Single commit revert. No product code changes |
| Who else touches this path? | Every session in this checkout runs these gates, and several have journal rows in the class this closes |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify worktree` | → | the cache link in `internal/le/verify/lifecycle.go` | `TestAWorktreeSharesTheCheckoutBuildCache` |
| A stage that could not judge | → | `runMode` in `internal/le/verify/engine/run.go` | `TestAStageThatCouldNotJudgeIsNotFlattenedToFailed` |
| A run whose log save hits ENOSPC | → | the classifier reaching `Report` | `TestADefeatedRunIsUnjudgedRatherThanFailed` |
| Any run | → | the verdict line in `run` | `TestThePrintedVerdictIsTheCodeTheRunExitsWith` |
| A preserved dirty abandoned worktree | → | the report `sweepAbandoned` feeds | `TestAPreservedWorktreeIsNamedWithItsAgeAndSize` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A run whose extraction, stages or log save hits `no space left on device` | Reports UNJUDGED with exit code 3, distinct from a pass and from every existing failure code, and the ENOSPC evidence appears in the verdict rather than after it |
| AC-2 | A stage that reports it could not judge its PACKAGE population | The stage's code survives `runMode` instead of being flattened to 1, and the run reports UNJUDGED. A matrix part dealt no ROWS still passes |
| AC-3 | A verify worktree is extracted | `<worktree>/cache` resolves to the checkout's cache, no private `cache/go-cache` directory is created inside the worktree, and GOCACHE for the run names the shared path |
| AC-4 | Disk to spare and every stage green | Runs exactly as today, with the same stages in the same order and exit code 0 |
| AC-5 | A dirty abandoned worktree is present | It is preserved, and the run names its path, its age, its size, and whether its dirt is modified or untracked content rather than deletions alone |
| AC-6 | Any run, whatever its outcome | The printed `exit=<code>` equals the code the process exits with, so no later branch can contradict a line already printed |
| AC-7 | A real stage failure with disk to spare | Reported as it is today, as a red, with the same exit code |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs the gate twice in a day and the machine does not fill | extraction → shared cache → no private cache built | `TestAWorktreeSharesTheCheckoutBuildCache` |
| 2 | Runs the gate, and the device fills mid-run | write site → ENOSPC classifier → UNJUDGED verdict | `TestADefeatedRunIsUnjudgedRatherThanFailed` |
| 3 | Wonders where 16 GB went | the run's own report naming the preserved worktrees | `TestAPreservedWorktreeIsNamedWithItsAgeAndSize` |
| 4 | Reads `exit=2` and acts on it | the verdict line printed after every branch that can change the code | `TestThePrintedVerdictIsTheCodeTheRunExitsWith` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAWorktreeSharesTheCheckoutBuildCache` | `internal/le/verify/lifecycle_test.go` | AC-3, A-5 | PASS |
| `TestAStageThatCouldNotJudgeIsNotFlattenedToFailed` | `internal/le/verify/engine/run_test.go` | AC-2 | PASS |
| `TestAStageThatJudgedTheTreeOutranksOneThatCouldNot` | `internal/le/verify/engine/run_test.go` | AC-2, precedence | PASS |
| `TestAFullDeviceIsUnjudgedAndAnyOtherWriteFailureIsABrokenRun` | `internal/le/verify/engine/run_test.go` | AC-1, R-3, boundary | PASS |
| `TestADefeatedRunIsUnjudgedRatherThanFailed` | `internal/le/verify/lifecycle_test.go` | AC-1 | PASS |
| `TestThePrintedVerdictIsTheCodeTheRunExitsWith` | `internal/le/verify/lifecycle_test.go` | AC-6 | PASS |
| `TestARealFailureStaysAFailure` | `internal/le/verify/lifecycle_test.go` | AC-7, R-3 | PASS |
| `TestARunWithEveryStageGreenIsUnchanged` | `internal/le/verify/lifecycle_test.go` | AC-4 | PASS |
| `TestAPreservedWorktreeIsNamedWithItsAgeAndSize` | `internal/le/verify/cleanup_test.go` | AC-5 | PASS |
| `TestAPreservedWorktreeSizeExcludesTheSharedCache` | `internal/le/verify/cleanup_test.go` | AC-5, security review | PASS |
| `TestAnUnmeasurableWorktreeSaysSoRatherThanReportingZero` | `internal/le/verify/cleanup_test.go` | AC-5, `principles.md` | PASS |
| `TestDirtIsCountedByShapeNotByLineCount` | `internal/le/verify/cleanup_test.go` | AC-5 | PASS |
| `TestPreservedAgeRendersTheDurationTheSweepMeasured` | `internal/le/verify/cleanup_test.go` | AC-5 | PASS |
| `TestDirtyAbandonedWorktreeIsNeverDestroyed` | `internal/le/verify/lifecycle_test.go` | existing test, kept green | PASS, unedited |
| `TestRedRunPreservesLogsAndReportsTheVerdictLast` | `internal/le/verify/lifecycle_test.go` | AC-6; renamed from `...AndPythonDiagnosticOrder`, its ordering assertion updated because the verdict deliberately moved last | PASS |
| `TestPythonInvalidCommitDiagnosticParityWithoutVerifyBody` | `internal/le/verify/parity_test.go` | AC-6; expected text updated, the verdict line now follows every outcome | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Run exit code | 0, 1, 2, 3, 130 | 3 is the new unjudged value | N/A: no code below 0 is reachable, `validateResult` refuses a negative | N/A |
| Stage code reaching `runMode` | 0 to 255 | 2 survives as unjudged | 0 stays a pass | anything else stays 1 |

No free-space boundary row: this spec adds no preflight, by owner decision.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| none | - | The gate under test IS the functional harness, so its own behavior is proven by unit tests over the run and by the class file closing | |

### Interop Tests (Scope: protocol)
Not applicable: no protocol surface.

## Files to Modify
- `internal/le/verify/lifecycle.go` - the cache link, the third outcome, and
  the verdict line printed after every branch that can change the code
- `internal/le/verify/cleanup.go` - reporting what is preserved
- `internal/le/verify/engine/run.go` - `runMode` stops flattening a stage's
  "could not judge", and the write sites classify ENOSPC with `errors.Is`
- `internal/le/gotoolchain/gotoolchain.go` - the package comment claims `cache/`
  is symlinked out of tree, which this work makes true for a worktree too
- `docs/contributing/running-commands.md` - "four times" is stale against seven
  journal rows, and the page says nothing about the worktree cache
- `docs/architecture/testing/verify-freshness-scope.md` - the `// Design:` page
  of `lifecycle.go`, `cleanup.go` and `engine/run.go`. It describes native stage
  execution and now has to describe the fourth outcome, the ENOSPC classifier
  and the verdict ordering
- `internal/le/verify/current.go` - `currentHere` reported a failure's reason on
  stderr only for code 2, so the new unjudged code would have silenced it
- `internal/core/diskspace/diskspace.go`, `internal/le/scratch/actions.go` - the
  other two hand-written occurrence counts
- `CLAUDE.md` dispatch row and `le` - the same stale "four times" count
- `plan/journal/full-disk-false-red.md` - the row this spec closes

## Files to Create
- none beyond tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | A developer gate, not a daemon surface |
| YANG validation constraints | N-A | No config node |
| YANG custom validators | N-A | No config node |
| CLI commands/flags | Yes | `./le verify worktree` gains exit code 3, not a flag |
| CLI grammar (keyword before value) | N-A | No new verb |
| Editor autocomplete | N-A | No config leaf |
| Functional test for new RPC/API | N-A | No RPC; the gate is the harness |
| Pipe completeness | N-A | `le` actions answer their own report shape |
| Env var registration | No | No floor and no knob; the cache path already comes from `gotoolchain.GoCache` |
| Doctor check for runtime dependencies | N-A | A build-host gate, not a runtime dependency of the daemon |
| Prometheus counters/metrics | N-A | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A developer gate; `docs/features.md` describes the product |
| 2 | Config syntax changed? | No | No config node |
| 3 | CLI command added/changed? | No | No new verb; the behavior of an existing `le` action changes |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | Yes | `docs/contributing/running-commands.md`, "When the disk is full" |
| 7 | Wire format changed? | No | No protocol code |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC |
| 10 | Test infrastructure changed? | Yes | VERIFIED 2026-09-03: `docs/functional-tests.md` describes the functional suites and never states a `verify worktree` exit code, so it is unaffected. The harness's own outcomes are in `docs/contributing/running-commands.md`, which this work updates |
| 11 | Affects daemon comparison? | No | No product behavior |
| 12 | Internal architecture changed? | Yes | VERIFIED 2026-09-03: the verify lifecycle's architecture page is `docs/architecture/testing/verify-freshness-scope.md`, named by the `// Design:` header of all three changed files, and it is updated here. `docs/architecture/core-design.md` covers the daemon's core and does not describe this lifecycle |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | RUN 2026-09-03. `docs/architecture/testing/verify-freshness-scope.md` is the declared design page and is UPDATED here. Four pages anchor `engine/run.go -- Run, RunMode` only to say a check runs as a verify stage, and none states an exit code or the flattening: `docs/DESIGN.md`, `docs/architecture/iface/logical-name-resolution.md`, `docs/architecture/testing/test-health.md`, `docs/contributing/documentation-testing.md`. All four stay correct, so none is edited |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The playbook's commands must match what the gate now prints |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the worktree stops building its own cache
   - Tests: `TestAWorktreeSharesTheCheckoutBuildCache`
   - Files: `internal/le/verify/lifecycle.go`, a cache-only entry point in
     `internal/le/scratch`, and the stale comment in
     `internal/le/gotoolchain/gotoolchain.go`
   - Verify: GOCACHE for the run resolves to the checkout cache, and no private
     `cache/go-cache` directory appears inside the worktree
2. **Phase: the third outcome** -- a run that judged nothing says so
   - Tests: `TestAStageThatCouldNotJudgeIsNotFlattenedToFailed`,
     `TestADefeatedRunIsUnjudgedRatherThanFailed`, `TestARealFailureStaysAFailure`
   - Files: `internal/le/verify/engine/run.go`, `lifecycle.go`
   - Verify: a stage's 2 survives, an ENOSPC at a write site is classified by
     `errors.Is`, and a real red keeps exit 1
3. **Phase: the verdict line stops lying**
   - Tests: `TestThePrintedVerdictIsTheCodeTheRunExitsWith`,
     `TestARunWithEveryStageGreenIsUnchanged`
   - Files: `lifecycle.go`
   - Verify: no branch can overwrite `report.Code` after the line is printed
4. **Phase: say what is holding the disk**
   - Tests: `TestAPreservedWorktreeIsNamedWithItsAgeAndSize`
   - Files: `internal/le/verify/cleanup.go`
   - Verify: the dirty worktree is still preserved, and now named with path,
     age, size and the shape of its dirt
5. **Phase: the pages and the journal**
   - Files: `docs/contributing/running-commands.md`, `CLAUDE.md`, `le`, and
     `plan/journal/full-disk-false-red.md`
   - Verify: no surface carries a hand-written occurrence count that the class
     file contradicts, and the page describes the worktree cache

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol |
| Correctness | A real red keeps its exit code, and no path turns a failure into UNJUDGED |
| Correctness | The floor is a measurement, not a guess, and the spec records how it was taken |
| Data flow | The ENOSPC evidence reaches the verdict rather than trailing it |
| Rule: `ai/rules/principles.md` | No outcome lets a run that judged nothing answer as though it judged, and no surface carries a hand-written count the class file already holds |
| Rule: `ai/rules/testing.md` | `TestDirtyAbandonedWorktreeIsNeverDestroyed` is untouched and still green |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The gate stops rebuilding the cache | `TestAWorktreeSharesTheCheckoutBuildCache`, plus `du -sh` on a worktree after a run |
| A defeated run says so | `TestADefeatedRunIsUnjudgedRatherThanFailed` |
| The printed verdict is the real one | `TestThePrintedVerdictIsTheCodeTheRunExitsWith` |
| The disk holders are named | run the gate with a preserved worktree present and read the report |
| No surface carries a stale occurrence count | `grep -rn "four times"` returns nothing outside the journal |
| The class closes | a row in `plan/journal/full-disk-false-red.md` naming this spec |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Information leakage | The retention line prints paths and sizes on a developer machine; it must not print the CONTENT of a preserved worktree, only the shape of its dirt |
| Resource exhaustion | The reporting walk over preserved worktrees must not itself be unbounded on a directory holding many, and the size read must not walk a shared cache reached through the new link |

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

- The gate is one of the largest disk consumers on the machine it judges, so its
  own retention policy and its verdict are the same subject. Preserving a dirty
  worktree is right; never saying it is there is what makes the machine fill.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse `internal/le/scratch`'s existing link mechanism, exposing a CACHE-ONLY entry point | Writing a second `os.Symlink` in `lifecycle.go`; or calling `Manager.Ensure` wholesale | `cacheTarget` is per-USER (`$XDG_CACHE_HOME/ze` or `~/.cache/ze`) and ignores the root, so it already answers "the one shared cache". A second symlink writer would be a second declaration of where the cache lives. `Ensure` is wrong because it ALSO links `tmp`, and `tmp` is per-CHECKOUT (`scratchTarget` hashes the root) and is where `saveLogs` reads `<worktree>/tmp/verify` |
| Share the checkout's cache with the worktree | A preflight that refuses a run when the disk is short | Removing the consumer beats guarding it. The private cache is 90% of a worktree, is rebuilt cold every run, and is deleted unread. A preflight would refuse runs to protect space the gate did not need to spend |
| A third outcome, distinct from pass and fail | Reporting ENOSPC as a failure with a clearer message | A failure means the tree is wrong. This run learned nothing about the tree, and the exit code is what other tooling reads |
| Stop `runMode` flattening rather than add a new signal | A separate unjudged flag beside the code | The stage already reports "could not judge" and the layer above destroys it. Restoring information beats adding a parallel channel that can disagree with the code |
| Classify ENOSPC at the write site with `errors.Is` | Matching `no space left on device` in the report text | The typed error exists exactly where it is discarded today. A text match over the report cannot see it at all, because no stage output reaches the report |
| Preserved worktrees are reported, never deleted | Deleting a dirty worktree after an age | `never-destroy-work` governs it, and a session's uncommitted work is exactly what a gate must not collect |

## Known Limitations
- The colima data disk, which two of the seven rows name, is a different owner
  and a different remedy. This spec does not touch the QEMU or interop paths.
- No preflight. Owner decision, 2026-09-03: removing the consumer beats
  refusing the run, and a later guard mirrors the measured shape already
  written in `internal/appliance/kernelbuilder/space.go`.
- The shared cache is proven by unit test only (owner decision, 2026-09-03).
  Two concurrent verify worktree runs sharing one cache are NOT exercised. The
  risk is the same one the checkout already takes on every concurrent le
  action, and Go documents the build cache as safe for concurrent use, but this
  spec does not demonstrate it.
- A `verify worktree` run still writes no certificate the commit path can read.
  That is unchanged by this spec and is not a defect it introduces.
- The floor measurement is n=2, one machine, one day, and both readings are
  terminal sizes rather than peaks.

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
- [ ] Every assumption has a Basis and a validation method; every risk row has an early signal
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
  → Observed 2026-09-03: each AC has a named passing test in the TDD table above
- [ ] Every user story has a working path and a passing test
  → Observed: stories 1-4 map to `TestAWorktreeSharesTheCheckoutBuildCache`,
    `TestADefeatedRunIsUnjudgedRatherThanFailed`,
    `TestAPreservedWorktreeIsNamedWithItsAgeAndSize` and
    `TestThePrintedVerdictIsTheCodeTheRunExitsWith`, all PASS
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
  → Observed: five rows, five tests, none deferred
- [ ] `./le verify worktree` passes
  → NOT RUN by the implementation phase, by instruction. `./le verify lint run`
    and `go test ./internal/le/verify/... ./internal/le/scratch/...` are what
    this phase ran, and their results are recorded below
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
  → Observed: `EnsureCache` has a non-test caller in `sharedCacheLink`, which
    `run` calls on the `./le verify worktree` path; `Unjudged` reaches the
    process exit through `runHere`
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
  → Observed: items 10, 12 and 16 verified against the pages on 2026-09-03
- [ ] Architectural Verification table filled, including registration over hardcoding
  → Filled above
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
  → Owed to `/ze-close`; the implementation phase does not run its own review gate
- [ ] Every A-N confirmed or broken, none `unvalidated`
  → Observed: A-1 CONFIRMED, A-2 BROKEN, A-3 CONFIRMED, A-4 BROKEN, A-5 CONFIRMED
- [ ] Deferral shard resolved: no live row without a destination
  → No deferral shard

### TDD
- [ ] Tests written
  → 13 new tests across `lifecycle_test.go`, `cleanup_test.go` and
    `engine/run_test.go`; two existing tests updated where the behavior they
    asserted deliberately changed
- [ ] Tests FAIL (paste output)
  → PARTIAL, and stated rather than claimed. The new tests were written against
    code already edited in the same phase, so no test in this spec was OBSERVED
    red before its implementation. What WAS observed red is the discrimination
    the change forced on two existing tests, which is a real break of the old
    behavior: `TestPythonInvalidCommitDiagnosticParityWithoutVerifyBody` failed
    with `diagnostic = "...\nverify-worktree: full exit=1\n", want
    "...does not name a commit\n"`, and `TestRedRunPreservesLogsAnd...` had to
    move `full exit=1` after `logs saved to`. Both prove the verdict line moved
- [ ] Tests PASS (paste output)
  → recorded in the phase report
- [ ] Boundary tests for all numeric inputs
  → `TestAStageThatCouldNotJudgeIsNotFlattenedToFailed` covers stage status 0, 1,
    2 and 37; `TestAFullDeviceIsUnjudgedAndAnyOtherWriteFailureIsABrokenRun`
    covers the write-failure mapping to 2 and 3;
    `TestThePrintedVerdictIsTheCodeTheRunExitsWith` covers run status 0, 1 and 3
- [ ] Functional `.ci` tests for end-to-end behavior
  → N-A, as the spec's Functional Tests table already records: the gate under
    test IS the functional harness
- [ ] Interop tests for protocol features (or N-A with a reason)
  → N-A: no protocol surface

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `internal/le/scratch/scratch.go`: `EnsureCache`, a cache-only link entry point.
  `Ensure` now calls it, so one function declares where the cache link goes.
- `internal/le/verify/lifecycle.go`: `sharedCacheLink` links `<worktree>/cache`
  to the per-user target at extraction; `defeatedCode` classifies a lifecycle
  write failure; the verdict line moved into the first registered defer so it
  renders last; the cleanup defer no longer promotes an unjudged run to a red;
  `logSaver` became a dependency so a test can force the device error.
- `internal/le/verify/engine/run.go`: `Unjudged` (3), `stageUnjudged` (2),
  `Defeated`, `runCode` and `brokenCode`. `runMode` stops flattening a stage's
  "could not judge" and classifies every write failure with `errors.Is`.
- `internal/le/verify/cleanup.go`: `sweepAbandoned` answers a `sweep` struct
  carrying `Preserved`; `describePreserved`, `countDirt`, `directorySize`,
  `preservedLine` and `preservedAgeAndSize` report path, age, size and dirt
  shape, bounded by `worktreeEntriesMax`.
- `internal/le/verify/current.go`: `withoutStageLog` keeps the stderr reason for
  the new status, which the old `report.Code == 2` test would have silenced.
- Four hand-written occurrence counts of the disk class removed
  (`le`, `internal/core/diskspace`, `internal/le/scratch/actions.go`,
  `ai/INSTRUCTIONS.md`); a fifth found at review (`cacheclean.go`).

### Bugs Found/Fixed
- None in product code beyond the defect this spec exists to fix. The gate is
  developer tooling; no daemon path changed.

### Documentation Updates
- `docs/architecture/testing/verify-freshness-scope.md`: the fourth outcome, the
  ENOSPC classifier and the verdict ordering, with anchors extended to
  `run.go -- Unjudged, Defeated` and `lifecycle.go -- run, sharedCacheLink`.
- `docs/contributing/running-commands.md`: three new sections (the shared
  worktree cache, what the gate is holding, and the five statuses), and the
  page-level source anchor extended with `internal/le/verify` and
  `internal/le/scratch` (added at review).
- No other page anchors any changed file: `grep "source:" docs/` over
  `internal/le/scratch`, `internal/le/gotoolchain` and `internal/core/diskspace`
  returns only the page-level anchor of `running-commands.md`.

### Deviations from Plan
- The spec named `CLAUDE.md` in Files to Modify. `CLAUDE.md` and `AGENTS.md` are
  generated and untracked (`git ls-files` answers nothing for both), so the edit
  belongs to `ai/INSTRUCTIONS.md` alone, which is what the implementation did.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The Deliverables Checklist promised `grep -rn "four times"` returns nothing outside the journal. Four surfaces were edited and a fifth was missed | The package comment of `internal/le/scratch/cacheclean.go` still carried "read as a code defect four times" | Review round 1 ran the checklist's own grep | Fixed in this closure |
| approach | The precedence guard in `runCode` (`current == 1`) had no test from the red-first direction | Deleting the guard left every test green while a real red was demoted to unjudged | Review round 1 read the guard, predicted the mutation, then observed it | Second subtest added and its RED observed |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The gate stops consuming the disk it does not need | Done | `sharedCacheLink` (`internal/le/verify/lifecycle.go`), `EnsureCache` (`internal/le/scratch/scratch.go`) | GOCACHE resolves through the link to the per-user target |
| A gate that judged nothing never answers as though it judged | Done | `runCode`, `brokenCode`, `Defeated` (`internal/le/verify/engine/run.go`), `defeatedCode` (`lifecycle.go`) | Exit 3, distinct from 0, 1, 2 and 130 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestADefeatedRunIsUnjudgedRatherThanFailed`, `TestAFullDeviceIsUnjudgedAndAnyOtherWriteFailureIsABrokenRun` | The ENOSPC diagnostic precedes the verdict line, which is now last |
| AC-2 | Done | `TestAStageThatCouldNotJudgeIsNotFlattenedToFailed` | `runCheck` (`internal/le/staticcheckfeaturematrix`) returns 2 for "matched no packages" through its `err != nil` branch; the empty-ROW branch still returns 0 |
| AC-3 | Done | `TestAWorktreeSharesTheCheckoutBuildCache` | Asserts the symlink, its target, and `gotoolchain.GoCache(worktree)` resolving to the shared `go-cache` |
| AC-4 | Done | `TestARunWithEveryStageGreenIsUnchanged` | Every stage called in order, exit 0, no worktree left |
| AC-5 | Done | `TestAPreservedWorktreeIsNamedWithItsAgeAndSize`, `TestDirtIsCountedByShapeNotByLineCount`, `TestAnUnmeasurableWorktreeSaysSoRatherThanReportingZero` | `TestDirtyAbandonedWorktreeIsNeverDestroyed` unedited and green |
| AC-6 | Done | `TestThePrintedVerdictIsTheCodeTheRunExitsWith` (four outcomes), `TestRedRunPreservesLogsAndReportsTheVerdictLast` | The line is written by the first registered defer |
| AC-7 | Done | `TestARealFailureStaysAFailure`, `TestAStageThatJudgedTheTreeOutranksOneThatCouldNot` (both orders) | A red keeps exit 1 whichever order the stages answer in |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All 13 new tests | Done | `internal/le/verify/lifecycle_test.go`, `internal/le/verify/cleanup_test.go`, `internal/le/verify/engine/run_test.go` | Green |
| `TestAStageThatJudgedTheTreeOutranksOneThatCouldNot` | Changed | `internal/le/verify/engine/run_test.go` | Widened at review to both stage orders; `stageCodeRunner` folded into `twoStageRunner` |
| `TestDirtyAbandonedWorktreeIsNeverDestroyed` | Done | `internal/le/verify/lifecycle_test.go` | Unedited, green |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | except `CLAUDE.md`, which is generated and untracked (see Deviations) |
| `internal/le/scratch/cacheclean.go` | Changed | Not in the plan; the fifth hand-written occurrence count, removed at review |

### Audit Summary
- **Total items:** 7 AC, 2 requirements, 15 tests
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** 2, both recorded above

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The gate stops rebuilding the cache it has | functional (unit over the real lifecycle) | `TestAWorktreeSharesTheCheckoutBuildCache` runs the real `Run` against a fixture repo, then reads `os.Readlink` on `<worktree>/cache` and `filepath.EvalSymlinks` on `gotoolchain.GoCache(worktree)`. Both name the per-user target. Without the link the second resolves inside the worktree |
| A run that judged nothing says so | functional (unit over the real lifecycle) | `TestADefeatedRunIsUnjudgedRatherThanFailed` injects `syscall.ENOSPC` at the log-save seam of a run whose stage already exited 9, and asserts exit 3 and the verdict line. `TestARealFailureStaysAFailure` injects `syscall.EACCES` at the same seam and asserts exit 1, so the classifier reads the typed error and nothing else |
| The printed verdict is the real one | functional | `TestThePrintedVerdictIsTheCodeTheRunExitsWith` drives four outcomes (green, red, defeated, cleanup-failed) and asserts the LAST diagnostic equals `exit=<report.Code>` in each. `TestPythonInvalidCommitDiagnosticParityWithoutVerifyBody` and `TestRedRunPreservesLogsAndReportsTheVerdictLast` both went RED on the old ordering and were updated |
| The disk holders are named | functional | `TestAPreservedWorktreeIsNamedWithItsAgeAndSize` builds a real dirty abandoned worktree and asserts the path, the age, a non-zero complete size and `0 modified, 1 untracked, 1 deleted` in `report.Text()` |
| The size walk does not follow the new link | functional | `TestAPreservedWorktreeSizeExcludesTheSharedCache` puts 1 MiB behind a `cache` symlink and asserts the walk answers the 4 bytes the worktree owns |
| A real red outranks unjudged | functional, with an observed RED | `TestAStageThatJudgedTheTreeOutranksOneThatCouldNot` drives both orders. Removing the `current == 1` guard from `runCode` made the red-first subtest fail with `run status = 3, want 1`; restoring it returned the package to green |
| The class closes | record | The 2026-09-03 row in `plan/journal/full-disk-false-red.md` naming this spec |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The spec metadata declares `Deferral shard: -`, and `plan/deferrals/spec-fixit-full-disk-verdict.md` does not exist |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-full-disk-verdict-d5330d4b-bf95-4792-a9ac-6e3c24f2cb96.md` (19 files, verdict=clean) |
| `./le spec session review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | round 1: intent-against-diff, wiring, exit-code consumers, guard soundness, test vacuity by predicted mutation, security (leakage and unbounded walk), the `ze-go-style` pass, documentation-anchor sweep. Round 2: the three fixes plus their sibling call sites |

### Scope declared before each round
| Round | Scope |
|-------|-------|
| 1 | The whole diff over the spec's paths: `internal/le/verify/lifecycle.go`, `cleanup.go`, `current.go`, `actions.go` and their tests, `internal/le/verify/engine/run.go` and its test, `internal/le/scratch/scratch.go` and `actions.go`, `internal/le/gotoolchain/gotoolchain.go`, `internal/core/diskspace/diskspace.go`, `le`, `ai/INSTRUCTIONS.md`, `docs/contributing/running-commands.md`, `docs/architecture/testing/verify-freshness-scope.md`, `plan/journal/full-disk-false-red.md`, plus the eight always-in-scope classes anywhere |
| 2 | Only the round-1 fixes and their sibling call sites: `internal/le/scratch/cacheclean.go`, the page-level anchor of `docs/contributing/running-commands.md`, `internal/le/verify/engine/run_test.go` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A fifth hand-written occurrence count of the disk class survived, so the Deliverables Checklist's own grep fails and the comment contradicts the journal | `internal/le/scratch/cacheclean.go`, package comment | The comment now names the journal file as the count |
| 2 | ISSUE | The precedence guard `current == 1` in `runCode` had no test from the red-first direction. Deleting it left every test green while a real red was demoted to unjudged, which is the one direction the design forbids | `internal/le/verify/engine/run.go`, `runCode` | `TestAStageThatJudgedTheTreeOutranksOneThatCouldNot` now drives both orders; the mutation was run and the RED observed |
| 3 | ISSUE | The page gained factual claims about `internal/le/verify` and `internal/le/scratch` producers with no source anchor covering them | `docs/contributing/running-commands.md`, page-level anchor | Both packages added to the anchor |

### Notes (do not block)
- A run that judged the tree RED and then meets ENOSPC saving its logs reports 3
  rather than 1. That is AC-1 as written, and it is defensible: the stage log the
  red points at was not saved and the worktree is about to be removed, so the run
  cannot show its verdict. It is recorded here because the demotion direction is
  otherwise forbidden, and a reader of `defeatedCode` should know it is deliberate.
- Every test in `internal/le/verify` that calls `Run` now creates the per-user
  cache directory (`$XDG_CACHE_HOME/ze` or `~/.cache/ze`) if it is absent. Only
  the two tests that set `XDG_CACHE_HOME` are isolated from it. Nothing is
  destroyed and no worktree removal follows the link, so this is hygiene rather
  than a defect.
- `TestPythonInvalidCommitDiagnosticParityWithoutVerifyBody` now asserts a
  DIVERGENCE from the Python producer, which its name no longer describes.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/verify/cleanup_test.go` | Yes | `git status --porcelain` lists it as `??` and it carries five tests |
| `internal/le/verify/engine/run_test.go` | Yes | `git status --porcelain` lists it as `??` and it carries three tests |
| `plan/journal/full-disk-false-red.md` | Yes | 8 rows, the first dated 2026-09-03 and naming this spec |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A defeated run is unjudged | `go test ./internal/le/verify/...` green, 2026-09-03, including `TestADefeatedRunIsUnjudgedRatherThanFailed` |
| AC-2 | A stage's 2 survives `runMode` | `runCode` read at `internal/le/verify/engine/run.go`; `runCheck` and `Judge` read at `internal/le/staticcheckfeaturematrix` and confirmed to answer 2 on the error path that carries "matched no packages" |
| AC-3 | The worktree shares the cache | `TestAWorktreeSharesTheCheckoutBuildCache` green |
| AC-4 | A green run is unchanged | `TestARunWithEveryStageGreenIsUnchanged` green |
| AC-5 | A preserved worktree is named | `TestAPreservedWorktreeIsNamedWithItsAgeAndSize` green |
| AC-6 | The printed verdict is the exit code | `TestThePrintedVerdictIsTheCodeTheRunExitsWith` green over four outcomes |
| AC-7 | A real red keeps its code | `TestARealFailureStaysAFailure` green; the `runCode` mutation observed RED then GREEN |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le verify worktree` to `sharedCacheLink` | none; the gate IS the harness | Yes: `runHere` (`internal/le/verify/actions.go`) calls `Run`, which calls `run`, which calls `sharedCacheLink` before `verifyengine.Run` |
| A stage that could not judge, to `runCode` | none | Yes: `runMode` is the only caller, and `RunMode` is reached from `runCurrent` and from `verifyengine.Run` |
| A defeated run, to exit code 3 | none | Yes: `runHere` returns `report.Code`, and the shard step of `.github/workflows/verify.yml` re-exits the captured status unchanged |
| A preserved worktree, to the report | none | Yes: `run` copies `swept.Preserved` into `Report.Preserved` and appends `swept.Diagnostics` |
| Any run, to the verdict line | none | Yes: the first defer in `run` appends it to `Report.Diagnostics`, which `Report.Text()` renders |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `du -sk` on both worktrees, 2026-09-03, recorded in the Assumptions table |
| A-2 | broken | `validateResult` records only "action exited N"; the classifier moved to the write sites |
| A-3 | confirmed | `runCheck` returns 0 for an empty ROW population and 2 for the "matched no packages" PACKAGE population; both branches read 2026-09-03 |
| A-4 | broken | `verify worktree` writes its certificate inside the worktree, so the commit path never reads it |
| A-5 | confirmed to the stated depth | `TestAWorktreeSharesTheCheckoutBuildCache`; concurrency itself is in Known Limitations |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "a stage answers 2 when it could not judge, which `le staticcheck-feature-matrix check` does for an empty package population" | `Judge` returns a non-nil error for "matched no packages", and `runCheck` maps that error to 2 | Yes |
| "A cache that cannot be linked is reported on that line and the run continues against a cold one" | `sharedCacheLink` returns `result.Line` on every path and never changes `report.Code` | Yes |
| "`CheckCertificate` stales on any non-zero exit" | `CheckCertificate` (`internal/le/verify/engine/status.go`) | Yes |
| Five statuses table (0, 1, 2, 3, 130) | each is reachable from `run`: `verification.Code` carries 2 and 130 through unchanged | Yes |
| Categories answered No | `grep "source:" docs/` over `internal/le/scratch`, `internal/le/gotoolchain` and `internal/core/diskspace` returns only the page-level anchor of `running-commands.md`, which this work updates | Yes |

## Core Insight

A gate is a measuring instrument, and an instrument that consumes the thing it
measures will eventually read its own consumption as the reading. This one built
7.4 GiB of private cache per run to judge a 0.6 GiB tree, filled the device, and
then reported the device as a defect in the tree.
