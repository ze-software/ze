# Spec: verify-gate-warm-timed-first-red

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The full verification gate (`./le verify current`, `./le verify worktree`, code under
`internal/le/verify/`) takes hours. The `verify` rows of `tmp/.ze-verify-duration.txt`
(written by `internal/le/job/registry.go`, `job.DurationFile`) read, for the ten runs
from 2026-10-04 to 2026-10-18: 7942, 12149, 7965, 7034, 7646, 8262, 6046, 7332, 10265
and 6355 seconds, so 1.7h to 3.4h a run. A run that will go red in a stage taking
seconds can spend an hour in lint and Staticcheck before that stage starts.

The owner approved (2026-10-21) ONE spec covering three changes, and a separate later
design spec for resumable per-stage green (named under Known Limitations, not written
here):

| # | Change | Symptom it removes |
|---|--------|--------------------|
| 1 | Warm caches in the throwaway worktree | Every package recompiles every run, no test result is ever cached, lint starts from an empty cache |
| 2 | Per-stage timing recorded | No stage has a duration anywhere, so nothing about a run can be measured or ordered |
| 3 | A local first-red mode, cheap stages first | A red found in seconds waits behind the most expensive stages, and the run goes on to the end |

Goal: a second consecutive verification of one commit reuses what the first compiled,
every stage's time is on disk after every run, and a run asked to stop at the first red
reports a red in a cheap stage within minutes, while CI keeps its run-to-end shard
behaviour and no shortened run can ever read as a pass.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - certificate, native stage execution, failure attribution, cut runs
  → Constraint: `CheckCertificate` reads the recorded exit code before any scope, so any non-zero run is STALE for every path list. A stopped run is non-zero by construction (it stopped on a red), so it cannot certify; this spec MUST keep that and test it.
  → Constraint: a cut run records its piece (`full-part-2-of-6`) and `worktreeArgv` carries the piece so pieces never attach to each other. A first-red run is different work from a run-to-end run of the same commit, so the keyword MUST travel in `worktreeArgv` too, or a first-red run attaches to a whole run (or the reverse) and takes its verdict.
  → Constraint: "A red stage does not hide later reds" is the documented engine contract. First-red is an explicit opt-in that changes it for one run; the default keeps it.
  → Decision: the lifecycle already publishes the certificate and manifest byte for byte (`CopyCertificate`) and links shared caches before the first stage (`sharedCacheLink`, `sharedModcacheLink`). The lint-cache link and the timing and failure-index publication join those same two places rather than adding a new mechanism.
  → Constraint: `verify reds` treats `Pending != 0` as undetermined (`verdictOf`, `internal/le/verify/reds.go`). A stopped run leaves stages pending forever, so that answer stays undetermined: correct, and it must also say the run stopped.
- [ ] `ai/rules/precommit-verify.md` - the gate runs against a commit in a throwaway worktree
  → Constraint: nothing here may move the full population onto the working tree. The worktree stays throwaway; only caches are shared.
- [ ] `ai/doors/le-tooling.md` - le's outside parties
  → Constraint: every concurrent session and CI depend on le. CI does NOT call the engine: `.github/workflows/verify.yml` runs `./le verify list full`, deals lines round robin over 6 shards (`NR % n == i % n`) and runs each action itself, continuing past reds. So `verify list full` order MUST stay the declared order or shard balance moves.
  → Constraint: the shared machine has a full-disk failure class (`plan/journal/full-disk-false-red.md`). A shared cache must stay on the device and under the budget its cleaner already knows.
- [ ] `docs/contributing/running-commands.md` - job admission, scratch, full disk
  → Constraint: verify admits itself under the `verify` label (`jobLabel`, `internal/le/verify/current.go`); a second run of the same work attaches. Concurrent verify runs are possible only for different work (another commit, another piece), which is why a fixed reused worktree path is rejected (Key Design Decisions).

**Key insights:**
- Go puts the package directory into every compile action ID unless `-trimpath` is set (`(*Builder).buildActionID` in `cmd/go/internal/work/exec.go`, Go 1.27: it hashes `dir <p.Dir>` when trimpath is off). `worktreePath` (`internal/le/verify/lifecycle.go`) is unique per run, so every run compiles cold even though `cache/` is linked to the shared GOCACHE.
- Three le paths already pass `-trimpath` for exactly this reason and measured it: Staticcheck `judgeEnvironment` (`internal/le/go/staticcheck/judge.go`, 495.7s cold vs 11.3s warm), `rfc` compile (`internal/le/rfc/check_compile.go`), tracked build (`internal/le/repo/compiles/repositorytrackedbuild.go`, `Build`, 36s then 5.7s).
- golangci-lint 2.14.0 keys its package cache on the MODULE-RELATIVE file path plus content hash (`(*Cache).computePkgHash` in its `internal/cache/cache.go`, which keys each file through `toRelativePath(pkg.Module, f)`), so its cache is path-independent; it is lost only because `LintCache(root)` resolves to `<worktree>/tmp/golangci-lint-cache`, which the worktree removal deletes.
- The worktree's failure index (`tmp/ze-verify-failures.json`) is never published to the checkout, which this spec depends on and repairs (Current Behavior).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/go/toolchain/gotoolchain.go` - `GoTest`, `GoList`, `GoVet` build argv with `-tags` and `-timeout` only; `Overrides` sets GOCACHE, GOLANGCI_LINT_CACHE (`LintCache(t.Root)`), CGO_ENABLED and the optional GOTMPDIR/GOMAXPROCS/GOMEMLIMIT/GOOS/GOARCH; it sets no GOFLAGS. `Environment` appends `Overrides` after `os.Environ()`. `LintCache` comment: deliberately NOT under `cache/`, measured 9.5G on 2026-09-13.
  → Decision (implementation, 1b8693f797): the GOFLAGS entry is composed in `Toolchain.Environment`, not in `Overrides`. `Overrides` never sees the inherited environment, so it cannot start from the effective value the next constraint requires. `Environment` appends exactly one GOFLAGS entry built from `EffectiveGoFlags` (the one resolver, moved here from `job.goTestDefaults`, which is deleted) plus `TrimpathGoFlags`. Proof: `TestEnvironmentComposesTrimpathWithTheEffectiveGOFLAGS`.
  → Constraint: `Overrides` is the one place every le-driven Go command (unit, race, vet, functional binary builds via `internal/le/test/functional/binaries.go` `tc.Environment`, lint via `internal/le/go/lint/verifylint.go`) gets its environment, so it is the single declaration point for `-trimpath`.
  → Constraint: a GOFLAGS environment variable REPLACES the `go env -w` file value rather than adding to it. `internal/le/job/process.go` `goTestDefaults` already resolves the effective value (last GOFLAGS entry, else `go env GOFLAGS`). Composing `-trimpath` MUST start from that effective value, from one resolver, or the owner's own GOFLAGS silently disappear.
- [ ] `internal/le/go/staticcheck/judge.go` - `judgeEnvironment` appends `-trimpath` to the inherited GOFLAGS (inherited env only, not the `go env` file).
  → Decision: once `Overrides` carries the flag, this private addition is a second declaration and is deleted (no-layering), provided its `environ` comes from `Toolchain.Environment`; if it does not, it is switched to it. Its comment's measurement moves to `Overrides`.
- [ ] `internal/le/rfc/check_compile.go` - passes `-trimpath` in the `GoVet` argv.
  → Decision: the argv copy is removed once the environment carries it. `internal/le/repo/compiles/repositorytrackedbuild.go` builds its own environment without the toolchain and keeps its argv flag; it is named in the `Overrides` comment as the one exception.
- [ ] `internal/le/test/functional/binaries.go` - `buildCommands`/`daemonBuildCommands` argv `go build [-cover] -tags ... -o ... ./cmd/ze` under `tc.Environment(EnvOptions{})`.
  → Constraint: gets `-trimpath` through the environment; argv unchanged.
- [ ] `internal/le/verify/lifecycle.go` - `run`: admit, sweep, `worktreePath(root, sha, stamp)` unique per run, `git worktree add --detach`, create `<wt>/tmp`, owner marker, then `sharedCacheLink(path)` and `sharedModcacheLink(root, path)` (reported, never refusing), `RunPart`, red-stage diagnostics, `CopyCertificate(path, root)`, `deps.logs` (saveLogs) on red only, deferred removal.
  → Constraint: a link that cannot be made is REPORTED and the run continues cold, as both existing links do.
- [ ] `internal/le/verify/cleanup.go` - `saveLogs` copies `<wt>/tmp/verify` to `tmp/verify-worktree-logs/<name>` with `os.CopyFS` (mtimes lost) on red runs only; a green run's logs are deleted with the worktree.
- [ ] `internal/le/verify/current.go` - keywords `full`/`fast` (`population`), `routeCurrent` sends full to the worktree lifecycle and fast in place (`runFast`); `slotFor` opens the job-registry progress log that `Release` deletes.
- [ ] `internal/le/verify/actions.go` - `verify worktree` keywords `commit`, `keep`. (Corrected during implementation: `part`/`of` were NOT keywords of `verify worktree`; `Options.Part`/`Parts` were reachable only from `commit debt-clear`.)
  → Decision (implementation, 88fb40e846): `part` and `of` were added as `verify worktree` keywords, because the approved entry point (`./le verify worktree [commit <rev>] [keep] [first-red] [part <i> of <n>]`, Data Flow) and AC-14's refusal name them. The `first-red` with `part` refusal lives in `Options.part` (`firstRedWithPart`, `internal/le/verify/lifecycle.go`).
- [ ] `internal/le/verify/engine/stages.go` - `fullStages()` declared order: lint, about 30 cheap structural and doc checks, 6 Staticcheck pieces, more checks, `unit-cached`, 3 race pieces, functional `exabgp-test`, `gating`, `alloc`; order tuned for the 6-shard round robin (`staticcheckParts`, `raceParts` comments). `fastStages` drops three stages and keeps identities. `Structural` is a field on the stage.
  → Constraint: the declared order is CI's balance and MUST NOT change. Cost order is applied at run time to an uncut local run only.
- [ ] `internal/le/verify/engine/run.go` - `runMode` runs `part.deal(population)` serially; per stage writes `### Stage NN/MM: <name>`, `Command:`, output, `### Stage result: <name> exit=N` (no time), `tell`s the block to the slot progress log, continues past red and crashed stages, breaks only on interrupt, a non-red refusal or a log-write failure; `Report.Completed` is true only when every stage ran; then writes the combined log, `writeRunArtifacts`, and `WriteCertificate`. `StageReport` has no time fields.
- [ ] `internal/le/verify/reds.go` - `stageResult` parses the LAST `### Stage result: ` line by cutting at ` exit=` and `Atoi` of the rest.
  → Constraint: anything appended after `exit=N` on that line breaks this parser and every fixture that writes the line (`internal/test/fixture/misc_fixture_runner_reds.go`, `internal/le/verify/reds_test.go`, `internal/le/commit/commit_test.go`). Timing goes on its own line BEFORE the result line, so the result line stays last and unchanged.
- [ ] `internal/le/verify/engine/artifacts.go` - `artifactStage{stage, exit-code, detail-log, groups}`, index `{mode, exit-code, run-dir, combined-log, generated-at, stages}` written to `FailuresJSONPath`, `FailuresLogPath`, `CombinedLogPath`, and `FullJSONPath` for full mode, all under the run's root.
- [ ] `internal/le/verify/engine/status.go` - `CopyCertificate` publishes `StatusPath` and `ManifestPath` only.
  → Constraint (defect, inside this work): after a worktree run the checkout's `tmp/ze-verify-failures.json` is never written. In this checkout on 2026-10-21 it is absent while `tmp/ze-verify.status` is from the 2026-10-18 worktree run, whose saved `01-go-lint-run.log` shows a lint red (`ineffassign` in `internal/le/interoplab/bgp/check_bfd_link_local.go`). Lint is structural, yet `structuralGateReds` read no file and charged nothing. A related row exists: `plan/journal/stale-artifact-reused.md` (2026-09-13, index read with no freshness check).
- [ ] `internal/le/commit/verification.go` - `structuralGateReds` reads `<root>/tmp/ze-verify-failures.json`; a missing or unreadable file answers no reds; it charges only stages present in the index with a non-zero exit.
  → Constraint: a stage that never ran is absent from today's index, so a stopped run would let a commit past every structural stage it did not reach. The index MUST list unreached stages and the gate MUST charge an unreached structural stage (fail closed).
- [ ] `internal/le/commit/prepare.go` - `checkVerificationGates` calls `structuralGateReds` when the certificate is not FRESH and refuses on any charged structural stage unless `structural-red-ok`.
- [ ] `internal/le/commit/actions.go` - `commit debt-clear` runs `verify.Options{Commit: "HEAD", Part, Parts}`, cut pieces.
- [ ] `.github/workflows/verify.yml` - CI shards `verify list full`, runs actions directly, continues past reds.
- [ ] Go 1.27 `src/cmd/go/internal/test/test.go` - `computeTestInputsID` hashes `chdir`, `stat` and `open` entries by ABSOLUTE name, and `hashOpen`/`hashStat` hash size, mode and MODTIME; a file under 2s old refuses caching (`modTimeCutoff`). Only `os.Getenv`/`LookupEnv` reads are logged, not `os.Environ`.
  → Constraint: with `-trimpath` a test binary's ID becomes path-independent, but a test that opens or stats a file inside the module (testdata, fixtures, the repository's own files) still misses on every fresh worktree, because both its absolute path and its git-checkout mtime differ. This spec states that limit and measures the share; it does not promise caching for those tests.
- [ ] `internal/test/runner/accept_only_lint_test.go`, `internal/le/workflowcheck/workflowcheck_test.go`, `internal/component/command/registry/local_data_functional_coverage_test.go`, `internal/perf/allocgate_test.go` - each finds the repository root from `runtime.Caller(0)` plus `..` steps.
  → Constraint: under `-trimpath` `runtime.Caller` returns a module-path-prefixed file name, not an absolute path, so these four go red. They are repaired to find the root from the test's working directory (go test runs in the package directory), using an existing repository-root helper if one serves. `internal/le/go/versionpin/goversion.go` uses `FuncForPC(...).Name()` and `internal/core/callsite/callsite.go` uses `frame.Function`: both unaffected.

**Behavior to preserve:**
- `./le verify list full` and `./le verify list fast` print the declared order, unchanged (CI shard balance).
- A run with no new keyword runs every stage and continues past reds.
- `### Stage result: <name> exit=N` stays the last line of every stage log, same spelling.
- Cut runs (`part i of n`), and therefore `commit debt-clear`, keep the declared dealt order and run to the end.
- The worktree is still unique per run and still removed; `cache/` and `gokrazy/modcache` links unchanged.
- The lint cache stays at `<checkout>/tmp/golangci-lint-cache`, not under `cache/`, so `./le scratch cache-clean` and its `lintCacheBudget` still own it.
- Any non-zero run is STALE for every path list.

**Behavior to change:**
- Every le-driven Go command builds with `-trimpath`.
- The worktree's lint cache is the checkout's.
- Each stage records its start and duration; the run's per-stage times reach the checkout whether the run is green or red.
- The worktree's failure index reaches the checkout.
- An uncut local run orders stages by recorded cost; a new keyword stops at the first red.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le verify current [full|fast] [first-red]` and `./le verify worktree [commit <rev>] [keep] [first-red] [part <i> of <n>]`, parsed in `internal/le/verify/current.go` and `actions.go`.

### Transformation Path
1. Keyword parse: `first-red` becomes an option on `Options` (worktree) or the in-place run; refused together with `part`.
2. Order: the engine reads the checkout's stage-time history and reorders an uncut population by cost (stable, unrecorded first). A cut run ignores costs.
   → Decision (review fix, parent's decision 2026-10-21, replaces 297965054d's `withUnrunStages`): not run is not red. A failure index carries forward, for every full-population stage the run did not hold, the verdict the checkout's earlier index recorded (`carryForward`, `writeFailureIndex` in `internal/le/verify/engine/artifacts.go`, read-merge-write under a lock on `tmp/ze-verify-failures.lock`; a missing or malformed earlier index is empty and `carry-note` says so). A carried red is charged and named `red at <sha> in <run>`; a stage no run judged is `outside-run`, never charged (the unverified-debt row owes it). Only a first-red stop's own unreached stages stay charged as not reached (AC-11). Each refusal line names one stage, its cause and its fixing command (superseded below). Proof: `TestAPieceCarriesAnEarlierRedForAStageItDidNotRun`, `TestAStageNeverJudgedStaysUnjudged`, `TestAMalformedEarlierIndexIsTreatedAsEmptyAndSaidSo`, `TestAFastStopCountsOnlyItsOwnPopulationAsNotReached` (engine `carry_test.go`); `TestAPieceKeepsAnEarlierStructuralRedCharged`, `TestAGreenFastRunChargesNoStructuralStage`, `TestAGreenSweepOfEveryPieceChargesNoStructuralStage` (commit).
   → Decision (third review, parent's decision 2026-10-21, NOTE 4 accepted): a first-red stop's unreached stages stay charged until a later run judges them or supersedes them; a stage a later run does not hold is then `outside-run` and owed by the unverified-debt row as not run, never charged as red. Each refusal line names the diagnosing rerun (`diagnose with:`, the group's own rerun, which publishes no index) apart from the runs that clear the charge (`clears when a verify run reaches it:`, `clearingRuns` in `internal/le/commit/verification.go`). Proof: `TestACarriedRedNamesTheRunThatClearsIt`, `TestACarriedRedWithNoRecordedCommitSaysSo` (commit); `TestAFullGreenRunAfterACarriedRedCarriesNothing`, `TestARecarriedRedKeepsItsOriginalOrigin`, `TestAFailedIndexWriteSaysWhatItLeftAndTheFix`, `TestAFailedCopyAfterTheIndexSaysTheGateReadsThisRun` (JSON index written first, so a failed copy leaves the gate on this run), `TestAnEarlierIndexWithNoCommitIsNamedAsSuch` (engine `carry_test.go`).
   → Decision (implementation, 88fb40e846): the engine owns the history through `Plan.History` (`internal/le/verify/engine/stagetimes.go`): `RunMode`/`RunPart` read costs from `<Plan.History>/tmp/ze-verify-stage-times.tsv` and append the reached stages' times there when the run ends. `lifecycle.run` passes the checkout root and `runFast` passes the in-place root, so a worktree run appends to the checkout's file and there is no separate publication step for the times (step 7 below publishes the failure index only).
3. Lifecycle (worktree only): after `tmp/` exists, link `<wt>/tmp/golangci-lint-cache` to the checkout's, beside the two existing links.
4. Environment: every stage process builds Go through `Toolchain.Environment`, whose `Overrides` now carries `GOFLAGS=<effective> -trimpath`.
5. Engine loop: stamp start, run, stamp duration, write `### Stage timing:` then `### Stage result:`; with first-red, stop after the first non-zero stage and record every unreached stage.
6. Artifacts: the failure index carries start, seconds and reached per stage; the certificate is written as today (non-zero exit on a stopped run).
7. Publication: the worktree lifecycle copies the failure index (paths rewritten to the saved-logs location on a red run) and appends the run's stage times to the checkout's history; an in-place run appends directly.
8. Commit gate: `structuralGateReds` charges an unreached structural stage.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| verify parent ↔ stage process | environment variables (GOFLAGS) and the stage's stdout/stderr pipe | No |
| worktree ↔ checkout | symlink (lint cache), file copy (index), append (times) | No |
| verify ↔ commit preparation | `tmp/ze-verify-failures.json` at the checkout | No |
| le ↔ CI | `verify list full` order | No |

### Integration Points
- `Toolchain.Environment` - gains the GOFLAGS entry (see the Decision under Current Behavior); the GOFLAGS resolver `EffectiveGoFlags` moves into the toolchain from `job/process.go` `goTestDefaults` so both read one function.
- `lifecycle.run` - third link beside `sharedCacheLink`/`sharedModcacheLink`; publication beside `CopyCertificate`.
- `runMode` / `RunPart` - receive the order and the stop option.
- `writeRunArtifacts` - new per-stage fields.
- `structuralGateReds` - reads `reached`.
- `verify reds` - names a stopped run.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | All Go commands keep going through `Toolchain.Environment`; publication goes through the lifecycle that already publishes the certificate |
| No unintended coupling (components stay isolated) | Yes | Only `internal/le` packages change; no product package learns anything (`TestNormalZeLinksNoInternalLe` unaffected) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Two private `-trimpath` copies are deleted in favour of one; the GOFLAGS resolver is moved, not copied |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling, no wire path |
| Registration over hardcoding, outbound | Yes | No new stage, command root or handler; one keyword on two existing actions |
| Registration over hardcoding, inbound | Yes | Cost comes from the recorded history keyed by `Identity.Name`; no list of stage names is written. Searched: `stages.go` (population, `fastStages` switch untouched), `.github/workflows/verify.yml` (reads `verify list`), `structuralGateReds` (derives from `Structural`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Stages are order-independent: no stage reads an artifact an earlier stage writes | Each stage is its own le process over the tree (`dispatch.RunAction`); CI already runs them on 6 machines in a different grouping | Cost order changes a verdict | Grep each stage action for writes under the root; one cost-ordered full run and one declared-order run at the same commit name the same reds | unvalidated |
| A-2 | `-trimpath` changes no diagnostic of lint, vet, Staticcheck or go test beyond the path spelling of `runtime.Caller` | `judge.go` comment (compile errors name the same tree-relative file), rfc and tracked-build precedent | A stage's output or attribution parse changes | Full run with the flag: every red's failure groups still name checkout-relative files (`failuregroup.CleanPath` accepts them) | unvalidated |
| A-3 | Only the four `runtime.Caller` tests depend on absolute source paths | grep of `runtime.Caller(`, `CallersFrames`, `frame.File` over `internal cmd pkg` | More reds appear under the flag | `./le test unit` with the flag set: the red list is exactly the four, then zero after the repair | unvalidated |
| A-4 | golangci-lint's cache is safe for two processes at once | It is a fork of cmd/go's cache (atomic rename writes); in-place lint at the checkout and a worktree lint can already run together on the same machine | Corrupted lint cache, false red | Two concurrent `./le go lint run` (checkout and kept worktree) both green; failure would show as cache read errors in the log | unvalidated |
| A-5 | golangci-lint 2.14.0's cache key is module-relative | `computePkgHash` read in the module cache copy of v2.14.0 | Shared lint cache still misses | Second run's lint stage seconds well below the first's in the history file | unvalidated |
| A-6 | The commissioning agent's lint cold figure (18 passes, 29.5 min) holds | Reported, not reproduced here; the 2026-10-18 lint log shows the passes but carries no time | Expected gain smaller | The first timed run records lint's seconds | unvalidated |
| A-7 | A test whose behaviour depends on an env var it reads through `os.Environ` (not logged by the test cache) gives the same answer across two verify runs of one commit | Today's worktree runs never hit the test cache, so this was never exercised there; in-place `./le test unit` has had the same property all along | A cached pass hides a red that only a different environment shows | Compare the second run's `(cached)` set against a `-count=1` run of the same packages: identical verdicts | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Setting GOFLAGS drops the owner's own `go env -w GOFLAGS` | `go env GOFLAGS` inside a stage differs from outside plus `-trimpath` | Compose from the effective value through the one resolver; unit test with a GOENV file |
| R-2 | A later GOFLAGS writer (`internal/le/rfc/discriminate_observe.go` sets `GOFLAGS=-overlay=...`) replaces the composed value | Discrimination builds cold | Those writers compose with the inherited value; only cost, never correctness |
| R-3 | A stopped run lets a commit past an unreached structural stage | Commit preparation after a first-red run reports no structural red | Unreached structural stages are charged (AC-11); the index is published (AC-8) |
| R-4 | A first-red run attaches to a run-to-end run of the same commit, or the reverse | Report says it shared another run | Keyword in `worktreeArgv` (AC-13) |
| R-5 | Cost order sends a never-measured slow stage last forever because runs keep stopping earlier | A stage's history line never appears | Unrecorded stages sort FIRST, so a stage is measured the first time it exists |
| R-6 | Concurrent sessions append to the history at once | Torn lines | One short line per stage written with a single `O_APPEND` write; a malformed line is skipped by the reader and counted in a diagnostic |
| R-7 | History grows without bound | File size | About 55 lines a run; the reader uses the latest line per stage; trimming is not built |
| R-8 | The published index points at worktree paths that no longer exist | `detail-log` opens nothing | Rewrite `run-dir`/`detail-log` to `tmp/verify-worktree-logs/<name>/` on a red run; a green run's index carries no red stage log to point at |
| R-9 | One-time cold rebuild of every in-place cache after the flag lands | First `./le test unit` after the change is slow | Expected once; named in the commit body |
| R-10 | `os.CopyFS` loses mtimes in saved logs | none | Irrelevant once each log carries its own timing line |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | No shipped binary changes. A wrong ordering or stop could hide a red from one local run; a wrong publication could let a commit past a structural red (guarded by AC-11 and AC-8). CI is unaffected by construction |
| How is it reverted? | Single commit revert; the history file and lint link are untracked and harmless |
| Who else touches this path? | `plan/spec-verify-scope-*` (scope narrowing in the same engine), `internal/le/commit` (debt), any session editing `internal/le/verify` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify worktree first-red` / `./le verify current first-red` | → | keyword parse into `Options`/in-place run, `worktreeArgv`, `fastArgv` | `TestFirstRedReachesTheEngineFromBothEntryPoints` (`internal/le/verify/firstred_test.go`) |
| `./le verify worktree part 1 of 2 first-red` | → | refusal in `Options.part` | `test/runner/verify-first-red-refuses-a-cut.ci`, `TestFirstRedWithAPartIsRefused` (`internal/le/verify/firstred_test.go`) |
| `./le verify current fast first-red` | → | `RunMode` stop, combined log, failure index, history, certificate | `test/runner/verify-first-red-stops.ci` (fixture `internal/test/fixture/misc_fixture_runner_first_red.go`, `verifyFirstRedStopsDriver`) |
| uncut run with a history file | → | `runMode` cost order (`planOrder`, `costOrder`) | `TestAnUncutRunRunsCheapestRecordedStageFirst` (`internal/le/verify/engine/run_test.go`) |
| any le-driven `go test`/`go build`/lint | → | `Toolchain.Environment` GOFLAGS | `TestEnvironmentComposesTrimpathWithTheEffectiveGOFLAGS` (`internal/le/go/toolchain/gotoolchain_test.go`) |
| `./le verify worktree` | → | lint-cache link | `TestAWorktreeSharesTheCheckoutLintCache` (`internal/le/verify/lifecycle_test.go`) |
| `./le commit create` after a stopped run | → | `structuralGateReds` | `TestAnUnreachedStructuralStageIsCharged` (`internal/le/commit/commit_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any `Toolchain.Environment` with no GOFLAGS inherited and none in the go env file | The environment holds exactly one GOFLAGS entry, `-trimpath` |
| AC-2 | Inherited GOFLAGS `-mod=mod`, or the same value only in a GOENV file | The entry reads `-mod=mod -trimpath`; an inherited value already holding `-trimpath` is not doubled |
| AC-3 | `judge.go` and `check_compile.go` after the change | Neither adds `-trimpath` itself; Staticcheck's environment still carries it (through `gotoolchain.TrimpathGoFlags`, the toolchain's one declaration, which `judgeEnvironment` composes) |
| AC-4 | The four `runtime.Caller` tests run with GOFLAGS `-trimpath` | They pass, and they find the same root as before without `runtime.Caller` |
| AC-5 | `./le verify worktree` | Before the first stage, `<wt>/tmp/golangci-lint-cache` is a link to `<checkout>/tmp/golangci-lint-cache` and the lifecycle prints one `verify-worktree: shared lint cache:` line; a link that cannot be made is printed and the run continues; worktree removal leaves the checkout's cache intact |
| AC-6 | Every stage of every run | Its log carries `### Stage timing: <name> start=<RFC3339 UTC> seconds=<decimal>` immediately before an unchanged `### Stage result: <name> exit=N`; `verify reds` still parses every log |
| AC-7 | Every run, green or red, worktree or in place | The engine's report and failure index carry `start` and `seconds` per stage, and one line per reached stage (stage, seconds, exit, mode, commit, start) is appended to `<checkout>/tmp/ze-verify-stage-times.tsv` |
| AC-8 | A worktree run, green or red | `<checkout>/tmp/ze-verify-failures.json` and `.log` are the worktree run's; on a red run their `run-dir` and `detail-log` name `tmp/verify-worktree-logs/<name>/` paths that exist |
| AC-9 | An uncut run with a history holding times for some stages | Stages run with every unrecorded stage first (declared order), then recorded stages by ascending latest seconds, ties in declared order; `verify list full`/`fast` output is unchanged; a cut run's order is the dealt declared order |
| AC-10 | `first-red`, a stage exits non-zero | No later stage starts; the report's exit is that stage's code (1, or 3 for unjudged), `Completed` is false, the report and index name the stopping stage and list each unreached stage with `reached: false`; the lifecycle prints `stopped at <stage>, <n> stages not run` and the command that reruns them: a worktree stop names `le verify worktree commit <sha>`, which runs them all; a fast stop names `le verify current fast` for the fast population and `le verify worktree` for the stages only the full population holds |
| AC-11 | Commit preparation after a stopped run whose unreached stages include a structural one | The commit is refused naming the unreached structural stages as not reached, unless `structural-red-ok` is given; without `first-red` behaviour is unchanged |
| AC-12 | Certificate after a stopped run | `./le verify status check` with no paths and with any path list answers STALE |
| AC-13 | A `first-red` worktree run and a plain one of the same commit | They are separate work in the job registry; neither attaches to the other |
| AC-14 | `first-red` with `part` | Refused before any work with exit 2, naming why (a piece is dealt for debt clearing, which needs every stage of the piece) and the two spellings that work |
| AC-15 | `verify reds file <path>` over a stopped run | Verdict undetermined, and the answer says the run stopped at `<stage>` and the pending stages will never report |
| AC-16 | Two consecutive `./le verify worktree` runs, each in its own fresh worktree path (the second may be at a later commit; only packages unchanged between the two commits are expected to hit) | The second run's `unit-cached` and race logs hold `(cached)` packages (the run saved under `tmp/verify-worktree-logs/20261018T015239.996486000Z-p66715-r1-0b037fd2fdcd/` held 0 of 657 `ok` lines); the `(cached)` and `ok` counts of both runs are recorded in Goal Validation. Wall-clock seconds are recorded but are not the proof: other sessions build and test on the same host while the runs go, so a time comparison measures their load as much as the cache (owner, 2026-10-21) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnvironmentComposesTrimpathWithTheEffectiveGOFLAGS` | `internal/le/go/toolchain/gotoolchain_test.go` | AC-1, AC-2 (env value, GOENV file value, already present) | |
| `TestJudgeRunsWithPathIndependentCompiles` (existing, extended; a rename was refused as a test weakening) | `internal/le/go/staticcheck/judge_test.go` | AC-3 | |
| the existing GOENV cases of `internal/le/job/process_test.go`, now through `gotoolchain.EffectiveGoFlags` (no new test) | `internal/le/job/process_test.go` | one GOFLAGS resolver | |
| existing four tests, repaired | the four files above | AC-4 | |
| `TestAWorktreeSharesTheCheckoutLintCache` | `internal/le/verify/lifecycle_test.go` | AC-5, removal leaves target | |
| `TestEveryStageLogCarriesItsTimingBeforeItsResult` | `internal/le/verify/engine/run_test.go` | AC-6 with an injected clock | |
| `TestStageResultStillParsesALogWithATimingLine` | `internal/le/verify/reds_test.go` | AC-6 parser | |
| `TestStageTimesReachTheCheckoutOnGreenAndRed` | `internal/le/verify/firstred_test.go` | AC-7 | |
| `TestEveryReachedStageIsAppendedToTheHistory` | `internal/le/verify/engine/stagetimes_test.go` | AC-7 history line columns | |
| `TestAWorktreeRunPublishesItsFailureIndex` | `internal/le/verify/lifecycle_test.go` | AC-8, rewritten paths exist | |
| `TestAnUncutRunRunsCheapestRecordedStageFirst` | `internal/le/verify/engine/run_test.go` | AC-9 ordering, unrecorded first, ties | |
| `TestACutRunKeepsTheDealtOrder` | `internal/le/verify/engine/part_test.go` | AC-9 cut | |
| `TestVerifyListKeepsTheDeclaredOrder` | `internal/le/verify/firstred_test.go` | AC-9 CI order | |
| `TestFirstRedStopsAndRecordsEveryUnreachedStage` | `internal/le/verify/engine/run_test.go` | AC-10 | |
| `TestARunWithoutFirstRedStillRunsPastARed` | `internal/le/verify/engine/run_test.go` | AC-10 negative, behaviour to preserve | |
| `TestAStoppedRunIsStaleForEveryPathList` | `internal/le/verify/engine/status_test.go` | AC-12 | |
| `TestAnUnreachedStructuralStageIsCharged` | `internal/le/commit/commit_test.go` | AC-11 | |
| `TestFirstRedIsSeparateWorkFromAWholeRun` | `internal/le/verify/firstred_test.go` | AC-13 | |
| `TestFirstRedReachesTheEngineFromBothEntryPoints` | `internal/le/verify/firstred_test.go` | wiring, both actions | |
| `TestFirstRedWithAPartIsRefused` | `internal/le/verify/firstred_test.go` | AC-14 | |
| `TestRedsNamesAStoppedRun` | `internal/le/verify/firstred_test.go` | AC-15 | |
| `TestAMalformedHistoryLineIsSkippedAndCounted` | `internal/le/verify/engine/stagetimes_test.go` | R-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| history `seconds` | 0 to any finite float | 0 | negative: line skipped | NaN, Inf or unparseable: line skipped |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-first-red-refuses-a-cut` | `test/runner/verify-first-red-refuses-a-cut.ci` | Operator types `le verify worktree part 1 of 2 first-red`, gets exit 2 and the reason, no worktree created | |
| `verify-first-red-stops` | `test/runner/verify-first-red-stops.ci`, fixture `verifyFirstRedStopsDriver` in `internal/test/fixture/misc_fixture_runner_first_red.go` (seeds the history so `arch tier/check`, red in a checkout with no `internal/component`, runs first) | A throwaway checkout whose first cheap stage is red: `le verify current fast first-red` stops after it, the log names the unreached stages, `le verify status check` answers STALE | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/go/toolchain/gotoolchain.go` - GOFLAGS in `Overrides`, the moved resolver
- `internal/le/job/process.go` - `goTestDefaults` uses the toolchain resolver
- `internal/le/go/staticcheck/judge.go` - delete the private `-trimpath` addition
- `internal/le/rfc/check_compile.go` - drop the argv flag
- the four `runtime.Caller` test files listed in Current Behavior
- `internal/le/verify/lifecycle.go` - lint-cache link, publication of index and times, keyword in `worktreeArgv`, stop line
- `internal/le/verify/current.go`, `internal/le/verify/actions.go` - `first-red` keyword, refusal with `part`, history read for order, in-place append
- `internal/le/verify/cleanup.go` - `saveLogs` returns the destination used to rewrite index paths
- `internal/le/verify/engine/run.go` - timing, order, stop, unreached stages
- `internal/le/verify/engine/artifacts.go` - `start`, `seconds`, `reached`, stopping stage
- `internal/le/verify/engine/status.go` - publication of the index beside `CopyCertificate`
- `internal/le/verify/reds.go` - name a stopped run
- `internal/le/commit/verification.go` - charge unreached structural stages
- `docs/architecture/testing/verify-freshness-scope.md` - native stage execution, failure attribution, the lint link, publication
- `docs/contributing/running-commands.md` - the `first-red` keyword and the history file

## Files to Create
- `internal/le/verify/engine/stagetimes.go` - history append and latest-per-stage read
- `test/runner/verify-first-red-refuses-a-cut.ci`, `test/runner/verify-first-red-stops.ci` and the fixture for the latter

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling has no YANG |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `internal/le/verify/actions.go`, `current.go`: keyword `first-red` |
| CLI grammar (keyword before value) | Yes | bare keyword like `keep`; `./le cli grammar` must stay green |
| Editor autocomplete | N-A | le keywords complete from the action declaration |
| Functional test for new RPC/API | Yes | the two `test/runner/*.ci` above |
| Pipe completeness | N-A | no ze CLI output |
| Env var registration | N-A | GOFLAGS is Go's, set for children, not a `ze.*` variable |
| Doctor check for runtime dependencies | N-A | the new paths are le artifacts under `tmp/`, not runtime dependencies of `ze` |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | developer tooling, no `features/` entry |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md` (le verify keywords); `docs/guide/command-reference.md` is ze's, N-A |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | N-A | no guide page for le verify |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | none |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`; `docs/functional-tests.md` only if its runner fixture section lists fixtures |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` (the `// Design:` doc of `stages.go`) |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | a keyword, not a command |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation: `./le spec citation anchors spec plan/spec-verify-gate-warm-timed-first-red.md`; known now: verify-freshness-scope.md anchors `run.go`, `lifecycle.go`, `status.go`, `artifacts.go`, `reds.go`, `verification.go`, `stages.go`. `docs/architecture/testing/tracked-build-gate.md` (declared by `repositorytrackedbuild.go`): unaffected, that file keeps its own argv `-trimpath` and changes only by being named in the `Overrides` comment; the implementer re-reads the page's `-trimpath` paragraph and edits it if it claims the tracked build is the only path-independent build. `docs/architecture/core-design.md` (declared by a file in Files to Modify): unaffected unless it describes how le's toolchain sets the Go environment, checked by reading its "Command engine and composition roots" section at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `running-commands.md` and `verify-freshness-scope.md` verify examples checked against `actions.go` |

## Implementation Steps

1. **Phase: Wiring** - `first-red` keyword on both actions, refusal with `part`, carried in `Options` and `worktreeArgv`, reaching `RunPart` as an option that does nothing yet
   - Tests: `TestFirstRedReachesTheEngineFromBothEntryPoints`, `verify-first-red-refuses-a-cut.ci`, `TestFirstRedIsSeparateWorkFromAWholeRun`
   - Files: `actions.go`, `current.go`, `lifecycle.go`, `engine/run.go`
   - Verify: entry point reachable; the stop behaviour test fails
2. **Phase: -trimpath** - resolver moved, `Environment` composes, private copies deleted, four tests repaired
   - Tests: `TestEnvironmentComposesTrimpathWithTheEffectiveGOFLAGS`, `TestJudgeRunsWithPathIndependentCompiles` (extended), the GOENV cases of `internal/le/job/process_test.go`, the four repaired tests
   - Files: `gotoolchain.go`, `process.go`, `judge.go`, `check_compile.go`, four test files
   - Verify: the four go red with the flag before their repair, green after (paste both)
3. **Phase: lint cache link**
   - Tests: `TestAWorktreeSharesTheCheckoutLintCache`
   - Files: `lifecycle.go`
4. **Phase: timing** - timing line, report and index fields, history file, publication of times and of the failure index
   - Tests: `TestEveryStageLogCarriesItsTimingBeforeItsResult`, `TestStageResultStillParsesALogWithATimingLine`, `TestStageTimesReachTheCheckoutOnGreenAndRed`, `TestAWorktreeRunPublishesItsFailureIndex`, `TestAMalformedHistoryLineIsSkippedAndCounted`
   - Files: `engine/run.go`, `engine/artifacts.go`, `engine/stagetimes.go`, `engine/status.go`, `lifecycle.go`, `cleanup.go`, `current.go`
5. **Phase: cost order and first-red stop**
   - Tests: `TestAnUncutRunRunsCheapestRecordedStageFirst`, `TestACutRunKeepsTheDealtOrder`, `TestVerifyListKeepsTheDeclaredOrder`, `TestFirstRedStopsAndRecordsEveryUnreachedStage`, `TestAStoppedRunIsStaleForEveryPathList`, `TestAnUnreachedStructuralStageIsCharged`, `TestRedsNamesAStoppedRun`, `verify-first-red-stops.ci`
   - Files: `engine/run.go`, `engine/artifacts.go`, `reds.go`, `commit/verification.go`
6. **Phase: docs** - each page is edited in the phase that changes it (`ai/rules/documentation.md`); this step only checks that none was skipped
7. **Phase: goal measurement** - two consecutive `./le verify worktree` runs in fresh worktree paths, judged by cache-hit counts (AC-16), one cost-ordered against one declared-order comparison (A-1, judged by whether every stage's verdict matches, not by time), one `first-red` run with a seeded cheap red in place (`./le verify current fast first-red`), judged by which stages it reached

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No path from a stopped run to FRESH; an unreached structural stage always charges; `verify list` order byte-identical to before |
| Correctness | GOFLAGS composed from the effective value, never replacing it |
| Naming | JSON keys kebab-case (`start`, `seconds`, `reached`, `stopped-at`); keyword `first-red` |
| Data flow | Cost order lives only in the engine and is computed from the history; no stage-name list appears anywhere |
| Rule: no-layering | Both private `-trimpath` copies deleted, not left beside the new one |
| Rule: user-facing-errors | The `part` refusal and the stop line say what, why, and the command that runs every stage |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `-trimpath` in every le Go command | `TestEnvironmentComposesTrimpathWithTheEffectiveGOFLAGS` under `./le job run label unit-pkg command go test -tags ... ./internal/le/go/toolchain/` |
| Shared lint cache | `ls -l` of the kept worktree's `tmp/golangci-lint-cache` during a `keep` run |
| Timing | the count of `### Stage timing:` lines over a saved run equals its stage count; `tmp/ze-verify-stage-times.tsv` grows by the reached-stage count |
| Published index | `tmp/ze-verify-failures.json` mode and generated-at after a worktree run |
| Cold-to-warm drop | AC-16 `(cached)` counts of both runs pasted in Goal Validation |
| First-red stop through the real entry point | `./le test functional runner`: `verify-first-red-stops` and `verify-first-red-refuses-a-cut` PASS; with the stop in `RunMode` disabled, `verify-first-red-stops` FAILs (the run went on to stage 02 `go lint/run`) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | History lines are untrusted text from a shared file: parse strictly, skip malformed, never let a stage name from the file add a stage that is not in the population |
| Symlink safety | The lint link is created only inside the run's own fresh worktree `tmp/`. A path that already exists there, link or not, is never replaced or followed for writing: it is left as it is, the lifecycle prints that it was left, and the run continues (`sharedLintCacheLink`, `internal/le/verify/lifecycle.go`) |
| Fail open | An unreadable history means declared order; an unreadable index at commit time keeps today's behaviour plus the STALE certificate |

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

- The worktree has never published its failure index, so structural attribution after a worktree run read nothing. First-red is only safe once that is fixed, so the fix is part of this spec rather than a separate journal-only record.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `-trimpath` once, in `Toolchain.Overrides` as composed GOFLAGS | Add the flag to each argv builder (`GoTest`, `GoVet`, `GoList`, functional builds) | golangci-lint has no `-trimpath` flag; its `go list -export` load only sees GOFLAGS, so argv alone leaves lint cold. One declaration also retires two private copies |
| Keep the unique worktree path | One stable reused worktree path | The test cache also hashes the absolute path and MODTIME of each opened file; a fresh checkout has new mtimes, so a stable path would not make file-reading tests cache, and it would forbid two different verifications (another commit, another piece) at once |
| Do not rewrite checkout mtimes | Set every file's mtime to its last-change commit time | Restores caching for file-reading tests only at the cost of a per-file history walk, and a wrong mtime makes the test cache UNSOUND (it hashes mtime and size, not content). Rejected: correctness over speed |
| Link the worktree's lint cache to the checkout's `tmp/golangci-lint-cache` | Move the lint cache under `cache/`; point GOLANGCI_LINT_CACHE at a per-user path | The 9.5G cache was kept off `cache/` on purpose (`LintCache` comment, `plan/journal/full-disk-false-red.md`); the link keeps it on the checkout's device and under `lintCacheBudget`, matching `sharedModcacheLink` |
| Timing on its own `### Stage timing:` line before the result line | Append fields to the result line | The result line is parsed by `stageResult` and written by three fixtures; appending breaks all of them |
| Append-only history `tmp/ze-verify-stage-times.tsv` at the checkout | Per-run JSON only; reuse `tmp/.ze-verify-duration.txt` | One observation per line is the single fact both measurement and ordering derive from; per-run JSON alone cannot order a run after a stopped one; the job duration file records job labels and is read by the registry, so stage rows in it change its readers |
| Cost from recorded times, unrecorded first | A declared cost per stage on `Stage` | A declared cost is a hand estimate that drifts with caches and machines; recorded times are the fact. Unrecorded-first guarantees every stage is measured on its first run |
| Cost order for every uncut local run; declared order for `verify list` and cut runs | Reorder `fullStages()` | CI deals `verify list full` round robin, and `commit debt-clear` deals pieces from the declared order; both keep their balance and their deal |
| Stopping is the opt-in keyword `first-red`; the default still runs to the end | Stop by default | HEAD in a shared checkout often carries a known red another session committed (pre-release rule). A default stop would end on it and leave every later structural stage unreached, which AC-11 then refuses: every commit would need a second, whole run. Cost order alone already puts cheap reds on the progress log within minutes |
| `first-red` refused with `part` | Allow it | A piece exists to clear debt, which records a piece only when it exits 0; stopping a red piece saves time but loses the reds the sweep reports. Refusing keeps one meaning per spelling |
| Unreached structural stages charge at commit | Treat unreached as unknown and let the debt row carry it | Fail closed: a structural red means the tree is broken, and "not looked at" cannot be read as "not broken" |

## Known Limitations

- **Resumable per-stage green** (owner-approved separate later design spec, not written here): reuse a stage's green across runs keyed on a content hash of that stage's inputs, assembling a certificate from parts, generalising `internal/le/commit/debtpart.go`. This spec's timing history is the input that work will want for its cost model.
- **Local stage parallelism**: running independent stages at once, and using more than `NumCPU/procDivisor` (`procDivisor`, `gotoolchain.go`) when verify is the only heavy job on the machine. Needs its own spec; not started.
- Tests that open or stat files inside the module still miss the Go test cache in every fresh worktree (Key Design Decisions, mtime row). AC-16 measures how many packages that leaves uncached.

## RFC Documentation (Scope: protocol)

N-A: tooling.

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: Scope tooling)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
