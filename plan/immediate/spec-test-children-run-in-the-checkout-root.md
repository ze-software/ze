# Spec: test children run in the checkout root

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Bucket: `plan/immediate/`. No operator meets this, so the bucket test in
`plan/README.md` reads against it on its face. It is filed here because of what
the class already produced: on 2026-09-05 a run left two ed25519 PRIVATE KEYS and
five config files at the repository root, none of them ignored, so any
`git add -A` from any session would have committed a private key
(`plan/journal/test-artifacts-land-in-the-repository-root.md`). A published key
is a defect the first release cannot carry, and the producer of that class is
still open at fifteen spawn sites.

## Task

Fifteen spawn sites in `internal/test/runner` and `internal/test/cli` start a
child process without setting the command's working directory. A child with no
directory inherits the harness's own, which is the checkout root that `./le` and
`ze-test` are started from. Eight of the fifteen start a DAEMON.

A `ze` daemon started in the checkout writes `database.zefs`, its rendered
config, its host keys, and its `rollback/` and `crash/` trees into the repository
root. `internal/test/cli/cmd_web.go` roots the daemon's config through
`ze.config.dir` and leaves the working directory alone, so every write the daemon
makes against a relative name still lands in the root.

The repair is already declared. `Record.WorkDir`
(`internal/test/runner/record.go`) is "the directory every child of this test
runs in", `childWorkingDirectory` (`internal/test/runner/runner.go`) applies it
per binary, and `runOrchestrated` (`internal/test/runner/runner_exec.go`) gives
it to the peer child and the client child. Three runners and the `ze-test` CLI
never took that route. `refuseRepoRoot` (`internal/test/fixture/fixture.go`) is
the only guard, it lives on the child side of the fixture entry point, and it
covers none of the fifteen.

The goal is one mechanism every harness spawn goes through, a directory decision
recorded per site, and a check that fails closed on a sixteenth site.

The find is recorded at
`plan/journal/test-artifacts-land-in-the-repository-root.md`, row dated
2026-09-07. This spec adds no journal row.

### The fifteen sites

The journal row names thirteen. Reading the two packages for this spec found two
more, and one of them is the site the automated ExaBGP suite uses for its `ze`
daemon.

| # | File | Enclosing function | Child | Daemon |
|---|------|--------------------|-------|--------|
| 1 | `internal/test/runner/decoding.go` | `(*decodingRunner).runTest` | `ze bgp decode` | No |
| 2 | `internal/test/runner/parsing.go` | `(*parsingRunner).runLegacyTest` | `ze config validate` (negative arm) | No |
| 3 | `internal/test/runner/parsing.go` | `(*parsingRunner).runLegacyTest` | `ze config validate -q` (positive arm) | No |
| 4 | `internal/test/runner/runner_validate.go` | `(*Runner).decodeToEnvelope` | `ze bgp decode --json` | No |
| 5 | `internal/test/cli/cmd_bgp.go` | `zeTestRunClientOnly` | `ze server <config>` | Yes |
| 6 | `internal/test/cli/cmd_web.go` | `zeTestStartLGServer` | `ze-test peer --mode sink` | Yes |
| 7 | `internal/test/cli/cmd_web.go` | `zeTestStartLGServer` | `ze -`, the looking glass | Yes |
| 8 | `internal/test/cli/cmd_web.go` | `zeTestStartLGNoEngineServer` | `ze-test lg --listen` | Yes |
| 9 | `internal/test/cli/cmd_web.go` | `zeTestStartChaosServer` | `ze-chaos --in-process --web` | Yes |
| 10 | `internal/test/cli/cmd_web.go` | `zeTestStartWebServer` | `ze start --web --web-only` | Yes |
| 11 | `internal/test/cli/cmd_web.go` | `zeTestCloseAllBrowserSessions` | `agent-browser close --all` | No |
| 12 | `internal/test/cli/cmd_exabgp.go` | `runExaBGPServerForeground` | `ze-test interop-bgp exabgp-server` | Yes |
| 13 | `internal/test/cli/cmd_exabgp.go` | `runExaBGPClientForeground` | `ze start <config>` | Yes |
| 14 | `internal/test/cli/cmd_exabgp.go` | `migrateExaBGPConfig` | `ze exabgp migrate` | No |
| 15 | `internal/test/cli/cmd_exabgp_process.go` | `startExaProcess` | `ze start <config>`, and the ExaBGP server | Yes |

Site 11 and site 15 are the two this spec adds. Site 15 matters most: the
foreground pair at 12 and 13 is the interactive rerun path, while
`startExaProcess` is what `startExaBGPServer` and `startExaBGPClient` both call,
so the automated ExaBGP suite starts every one of its `ze` daemons there.

Thirteen other spawns in the same two packages already name a directory, and each
one is correct. `(*Runner).Build`, `buildZe` and `zeTestBuildChaos` run `go
build` and name the checkout root. `runOneCommand` names the `.ci` work
directory. `runOrchestrated` names `rec.WorkDir` for both its children.
`startWithETXTBSYRetry` inherits the directory of the command it restarts.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/runner-architecture.md` - the "Scratch roots" section declares where per-run and per-test working directories go
  → Decision: per-run and per-test working directories live under `sessionpath.DefaultScratchRoot()` when a session is active, and under the system temp directory off-session. A directory this spec creates takes the same root and adds no second rule.
  → Constraint: the section carries source anchors naming `internal/test/sessionpath` and `internal/test/runner/runner.go`. A change to which directory a child runs in makes it wrong, so its edit lands in the same work.
- [ ] `docs/functional-tests.md` - the operator-facing account of what `ze-test` runs and where it writes
  → Constraint: `ai/CODE-TO-DOCS.md` anchors this page from `cmd_web.go`, `cmd_exabgp.go`, `cmd_bgp.go`, `decoding.go` and `parsing.go`, so it is named in the Documentation checklist rather than assumed unaffected.
- [ ] `ai/rules/principles.md` - one declaration, derived everywhere else
  → Decision: the fact "a harness child runs in a directory of its own, and only a repository-anchored tool keeps the checkout root" is already declared once, in `childWorkingDirectory` and `repositoryAnchoredBinary`. The repair promotes that declaration rather than writing a second one per site.
  → Constraint: a guard must not read a missing value as permission. A source walk that finds no spawn site must go red, not green.
- [ ] `ai/rules/simplicity.md` - the simplest fully correct answer, machinery cut and correctness kept
  → Decision: fifteen sites in two packages, plus a journal class with four rows across four dates, is what makes one shared constructor cheaper than fifteen independent edits. The rejected shape is a new package: `internal/test/cli` already imports `internal/test/runner`, and `internal/test/runner` does not import `internal/test/fixture`, so the constructor lands in `runner` with no import cycle and no new tier.
- [ ] `ai/rules/commands.md` - artifacts go under the session scratch root, never at the `tmp/` root
  → Constraint: the rule governs an AGENT's own files, so it is not authority over a test child. It is cited for the ROOT it names, which `sessionpath.EnsureScratchRoot` already implements for the harness. The harness reaches that root through `sessionpath`, which is the declaration both obey, so no site calls `./le session scratch ensure`.
- [ ] `ai/rules/evidence.md` - a guard fails closed, and a zero is never a valid-looking answer
  → Constraint: the lint resolves `os/exec` by IMPORT PATH, never by the identifier `exec`, so an alias cannot evade it, and it carries a floor on the number of call sites it found.
- [ ] `ai/rules/no-layering.md` - delete X, then implement Y
  → Constraint: the inline temp-directory creation in `runOrchestrated` is deleted when the shared directory maker lands. Two ways to create a work directory is what this rule bans.

**Key insights:**
- `Record.WorkDir` and `childWorkingDirectory` already answer this question for the `.ci` route. The work is reach, not invention.
- `repositoryAnchoredBinary` names the only children that keep the checkout root: `go`, `le`, `./le`. None of the fifteen is one of them.
- Five of the fifteen pass a path that resolves against the working directory today. Each needs its path made absolute BEFORE the directory changes, which is the repair `runOrchestrated` already made on 2026-08-28.
- `internal/test/fixture` is out of scope and stays out. Its roughly 180 spawns inherit the fixture's own directory, which `refuseRepoRoot` already proves is not the checkout root.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner.go` - holds `workDirPrefix` (the value `ze-work-`), `repositoryAnchoredBinary` and `(*Runner).childWorkingDirectory`. `(*Runner).Build` runs three `go build` children and names `r.baseDir` for each.
- [ ] `internal/test/runner/runner_exec.go` - `runOrchestrated` creates `rec.WorkDir` under `sessionpath.DefaultScratchRoot()` with `workDirPrefix` for EVERY record, removes it when the test ends, and names it for the peer child and the client child. It anchors a relative config path against the base directory before handing it over.
- [ ] `internal/test/runner/record.go` - `WorkDir` is documented as "the directory every child of this test runs in". `TmpfsTempDir` keeps a narrower meaning and is set only when the record declares files.
- [ ] `internal/test/runner/decoding.go` - `decodingRunner` carries `tests`, `baseDir`, `zePath` and `colors`, and holds no working directory. `runTest` builds every `ze bgp decode` argument from literals and the hex payload, so no argument is a path.
- [ ] `internal/test/runner/parsing.go` - the `.ci` path creates a work directory in `setupWorkDir` under `sessionpath.DefaultScratchRoot()`, and `runOneCommand` names it. The legacy `.conf` path in `runLegacyTest` names no directory, and writes its inline-config temp file into the system temp directory rather than the session scratch root.
- [ ] `internal/test/runner/runner_validate.go` - `decodeToEnvelope` has one caller, `validateJSON`, and `validateJSON` takes the record. The record and its `WorkDir` are one frame up.
- [ ] `internal/test/cli/cmd_bgp.go` - `zeTestRunClientOnly` reads the config path from the record's `config` option and starts `ze server`. `buildZe` names `baseDir` for its `go build`.
- [ ] `internal/test/runner/record_parse.go` - `(*EncodingTests).parseOption` builds the record's `config` value by joining the `.ci` file's directory with the config name, so it carries whatever spelling discovery walked with and can be relative.
- [ ] `internal/test/cli/cmd_web.go` - `zeTestStartLGServer` and `zeTestStartWebServer` each create a temp directory under `sessionpath.DefaultScratchRoot()`, pass it as `ze.config.dir`, store it on `zeTestWebServer.tempDir`, and leave the working directory alone. `zeTestStartLGNoEngineServer` and `zeTestStartChaosServer` create no directory at all. `zeTestBuildChaos` names `baseDir`. `(*zeTestWebServer).stop` kills the child, then the auxiliary child, then removes `tempDir` when it is set.
- [ ] `internal/test/cli/cmd_exabgp.go` - `exaBGPClientConfig` creates a directory per test in the system temp directory, seeds it from the run's config directory, and writes `migrated.conf` into it, so the config path it answers is absolute. `exaBGPServerArgs` passes `test.ciFile` and the operator's `--save` value through unchanged.
- [ ] `internal/test/cli/cmd_exabgp_process.go` - `startExaProcess` takes a program, arguments and an environment, sets `Setpgid`, joins the stdout and stderr readers before `Wait`, captures both into a locked buffer, and names no directory.
- [ ] `internal/test/fixture/fixture.go` - `refuseRepoRoot` reads the working directory, calls `sessionpath.IsRepoRoot`, and answers an error in the checkout root. It runs at the fixture dispatch entry point, after the driver is resolved and before the driver runs. A working directory it cannot read is refused too.
- [ ] `internal/test/sessionpath/sessionpath.go` - `IsRepoRoot` reads `go.mod` in the named directory and matches the module directive. `DefaultScratchRoot` and `EnsureScratchRoot` answer the session scratch root, or "" off-session, and never answer an error.
- [ ] `internal/test/runner/work_dir_test.go` - `TestChildWorkingDirectoryAnchorsOnlyRepositoryTools` pins the per-binary decision. `TestRecordWithoutTmpfsStillRunsOutsideTheCheckout` runs `/bin/pwd` as a real child and asserts the child's own answer, which is the discrimination pattern this spec reuses.
- [ ] `internal/test/runner/accept_only_lint_test.go` - the precedent for a source-scanning lint test in this package, with `repoRootForTest` deriving the checkout root from the test file's own location.
- [ ] `internal/le/repository/repository.go` - `Run` composes five checks in one slice of steps. A sixth is an edit to that central enumeration.

**Behavior to preserve:**
- Every path an operator or a fixture passes keeps the meaning it has today. The `--save` directory of `ze-test exabgp`, the `.ci` file argument, and the config path of `ze server` each resolve against the same place after the change as before.
- `TmpfsTempDir` keeps its narrower meaning. Three consumers read it as "did the record declare files", and a directory that is always set answers a different question.
- The five `go build` children, and the `./le` and `go test` steps, keep the checkout root, which `repositoryAnchoredBinary` already decides.
- `(*zeTestWebServer).stop` keeps killing the children before it removes the directory.

**Behavior to change:**
- Each of the fifteen children runs in a named directory instead of inheriting one.
- `runLegacyTest` writes its inline-config temp file under the session scratch root instead of the system temp directory, which is the root `setupWorkDir` in the same file already uses.

## Data Flow (MANDATORY)

### Entry Point
- An operator or an agent runs `./le verify`, `./le functional`, `ze-test bgp`, `ze-test web`, `ze-test parse` or `ze-test exabgp`. The shell's working directory is the checkout root, because `./le` is a path in it.

### Transformation Path
1. The harness process starts with the checkout root as its working directory.
2. A suite runner selects tests and decides what to spawn.
3. A spawn site builds a command and starts it.
4. The child inherits the parent's working directory whenever the site names none.
5. The child resolves every relative name it writes against that directory: `database.zefs`, `daemon.log`, host keys, `rollback/`, `crash/`, `meta/`.

The change replaces stage 3. Every site builds its command through one
constructor that takes the directory as an argument and refuses to answer
without one.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness → child process | a command carrying a working directory, an environment and arguments | No |
| Harness → filesystem | a directory created under `sessionpath.DefaultScratchRoot()` and removed when the run ends | No |
| `internal/test/cli` → `internal/test/runner` | an existing import; the constructor is exported from `runner` | No |

### Integration Points
- `(*Runner).childWorkingDirectory` and `repositoryAnchoredBinary` (`internal/test/runner/runner.go`) - the decision the constructor formalizes. `childWorkingDirectory` keeps its per-record job and calls the constructor's rule rather than restating it.
- `Record.WorkDir` (`internal/test/runner/record.go`) - the directory site 4 receives, passed down from `validateJSON`.
- `sessionpath.DefaultScratchRoot`, `EnsureScratchRoot`, `IsRepoRoot` - the root every created directory takes, and the test the constructor refuses on.
- `zeTestWebServer.tempDir` (`internal/test/cli/cmd_web.go`) - the field that already owns cleanup for sites 6, 7 and 10, and that sites 8 and 9 start using.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Every spawn in the two packages routes through one constructor, and the lint proves no site bypasses it |
| No unintended coupling (components stay isolated) | No | `internal/test/cli` already imports `internal/test/runner` (`go list -deps`), so no import edge is added, and `internal/test/fixture` is untouched |
| No duplicated functionality (extends existing, does not recreate) | No | The constructor promotes `childWorkingDirectory` and the directory creation in `runOrchestrated` rather than writing a second rule, and the old inline creation is deleted |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A: no wire encoding path is touched |
| Registration over hardcoding | No | The lint discovers spawn sites by walking the syntax tree of two packages, and holds no list of file names, so a new file is covered without an edit. Its one enumeration is the two package paths, which is the scope statement itself |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ze bgp decode` writes no per-test file, so sites 1 and 4 can share one directory per run instead of one per test | `(*decodingRunner).runTest` builds every argument from literals and the hex payload, and no argument is a path | Two parallel decode children clobber one file. The repair is a per-test directory, at one directory created and removed per test in a suite of thousands | Run the decode suite with the shared directory, then list it: a file whose name is not unique per test breaks the assumption | unvalidated |
| A-2 | `ze config validate` resolves an `include` against the config file, not against the working directory | `runOrchestrated` already runs `ze` with a config path from another directory, and the `.ci` half of the parse suite passes | The legacy `.conf` tests that use `include` go red the moment the directory changes | The `parse` suite, whose legacy `.conf` half is the population sites 2 and 3 serve | unvalidated |
| A-3 | `test.ciFile` (site 12) and the migration source (site 14) can carry a relative spelling, as the record's `config` option does | `(*EncodingTests).parseOption` builds the config path by joining the `.ci` directory, and discovery walks a relative tree; `parseExaBGPCI` takes its `.ci` path from the same kind of walk | An unanchored path breaks when the directory changes, and the ExaBGP suite cannot find its fixture | Anchor unconditionally, which is correct whichever spelling arrives, then run the ExaBGP suite | unvalidated |
| A-4 | `agent-browser` (site 11) writes nothing it must find again in a later run | The call is `close --all`, a best-effort cleanup whose error is discarded | A browser session store keyed to the working directory is not found, and `close --all` closes nothing | Run the web suite twice in a row and confirm no browser session leaks between runs | unvalidated |
| A-5 | Off-session, `sessionpath.DefaultScratchRoot()` answers "" and a temp directory created under an empty root falls back to the system temp directory, so a human shell and CI both get a real directory | `EnsureScratchRoot` documents "" as the fallback answer, and `runOrchestrated` already relies on that behavior for every record | An off-session run creates no directory, the constructor refuses every spawn, and the whole suite fails outside a session | A unit test that calls the directory maker with an empty root and asserts a usable directory | unvalidated |
| A-6 | `startExaProcess` can take one more parameter without a caller outside `cmd_exabgp.go` | A grep over `internal/test/` finds two callers, `startExaBGPServer` and `startExaBGPClient`, both in that file | A third caller is missed and fails to compile, which is a visible red rather than a silent defect | The build | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A path that resolved against the checkout root today resolves elsewhere, and a test fails with "file not found" rather than with the real reason | The parse, decode, ExaBGP or web suite goes red at a path error on the first run after the change | Make every path argument absolute in the same edit that names the directory. The five sites that carry one are 2, 3, 5, 12 and 14, and the implementation steps name them |
| R-2 | The operator's `--save` value of `ze-test exabgp` is relative, and the change redirects the operator's own output into a scratch directory | An operator reports that `--save out` writes nothing to `./out` | Resolve `--save` against the harness's own working directory BEFORE the spawn, so the flag keeps its meaning, and pin it with a unit test over the argument builder |
| R-3 | The lint passes vacuously because it found no spawn site, after an import alias, a build tag, or a package move | Nothing. That is what makes it a risk | Resolve `os/exec` by import path, and carry a floor on the number of call sites the walk found. A stale floor is a visible red; an evaded walk is not |
| R-4 | A test that today finds a file an earlier run left in the checkout root stops finding it, and goes red | A web or ExaBGP test fails only after the change, at a missing-file assertion | That test was passing on a cross-run artifact, which is the defect this spec exists to remove. Fix the test to declare the file it needs. Do not restore the shared directory |
| R-5 | Sites 8 and 9 gain a directory and its removal, and a child that outlives the stop path writes into a directory that is gone | A `ze-chaos` or `ze-test lg` child logs a write error after the test ends | `(*zeTestWebServer).stop` kills the children before it removes the directory, which is the order sites 6, 7 and 10 already use |
| R-6 | A directory created off-session lands in the system temp directory and is never removed, because `./le session reap` only knows session roots | An unowned temp directory per run | Each creator answers its own remover, and every caller defers it, which is what `runOrchestrated` already does |
| R-7 | The suite-scoped directory for sites 1, 12 and 14 outlives a killed run | A directory under the session scratch root carrying `workDirPrefix` and no owner | The prefix names what made it, which is why `workDirPrefix` exists, and `./le session reap` removes an abandoned session root |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The functional suites. No shipped code path is touched: every changed source file is under `internal/test/`, which no `ze` build tag compiles into the daemon. The failure mode is a red suite, never a wrong route or a dropped session |
| How is it reverted? | A single commit revert. Nothing is persisted, no config migrates, and no peer sees anything |
| Who else touches this path? | Every session that runs `./le verify` or `ze-test`. `runner_exec.go` was changed for this class on 2026-08-28, and this spec changes it again only to delete the inline directory creation |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-test exabgp <nick>` starts its client daemon | → | `startExaProcess` (`internal/test/cli/cmd_exabgp_process.go`) | `TestExaProcessRunsInTheDirectoryItIsGiven` |
| `ze-test parse <nick>` runs a legacy `.conf` test | → | `(*parsingRunner).runLegacyTest` (`internal/test/runner/parsing.go`) | `TestLegacyParseChildRunsOutsideTheCheckout` |
| A developer adds a sixteenth spawn site to either package | → | the syntax-tree walk over `internal/test/runner` and `internal/test/cli` | `TestEveryHarnessSpawnNamesItsDirectory` |
| A caller asks the constructor for a command with no directory | → | the constructor's refusal | `TestChildCommandRefusesAnEmptyDirectoryAndTheCheckoutRoot` |
| A `.ci` step starts a `ze` daemon during a suite run | → | the repository root's untracked set | `harness-root-clean` (`test/plugin/harness-root-clean.ci`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `startExaProcess` is asked to run a program that prints its own working directory | The program answers the directory the caller named. It never answers the harness's own directory |
| AC-2 | `(*parsingRunner).runLegacyTest` runs with a `zePath` that prints its own working directory | The recorded output names the per-test directory the runner created, and it is not the runner's base directory |
| AC-3 | The constructor is asked for a command with an empty directory | It answers an error naming the program it refused to start. It never answers a command whose directory is unset |
| AC-4 | The constructor is asked for a command whose directory `sessionpath.IsRepoRoot` accepts | It answers an error. The repository-root constructor is the only route that produces a command rooted there |
| AC-5 | A source walk over the non-test files of `internal/test/runner` and `internal/test/cli` finds an `os/exec` command call outside the constructor's own file | The lint names that file and the enclosing function, and fails |
| AC-6 | The same source walk finds fewer call sites than its recorded floor | The lint fails, and its message states that the walk found less than it should and is no longer trustworthy |
| AC-7 | The source walk reads a file that imports `os/exec` under an alias | The alias is resolved from the import declaration, and the call is still found |
| AC-8 | `ze-test exabgp` runs with a relative `--save` value | The saved logs appear under that path, relative to the directory the operator ran the command in, exactly as before the change |
| AC-9 | The functional suite runs from a clean checkout | `git status --porcelain` over the repository root reports the same set of paths before and after the run |
| AC-10 | Each of the fifteen sites in the site table is read after the change | Each names a directory, and the five that pass a path argument (sites 2, 3, 5, 12, 14) make that path absolute before the spawn |
| AC-11 | `internal/test/runner` is read after the change | The record work directory is created through the shared maker. No second creation of a directory carrying `workDirPrefix` remains in the package |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An agent runs the functional suite in a shared checkout and reads `git status` afterwards | `./le functional` → suite runner → spawn site → child in its own directory → repository root untouched | `harness-root-clean` |
| 2 | A developer adds a spawn site to `internal/test/cli` and forgets the directory | `go test ./internal/test/runner/` → syntax-tree walk → the new file and function are named | `TestEveryHarnessSpawnNamesItsDirectory` |
| 3 | An operator reruns one ExaBGP test with `--save out` to read the BGP logs | `ze-test exabgp <nick> --save out` → argument builder makes the path absolute → the server writes under the operator's `./out` | `TestExaBGPServerArgsKeepARelativeSaveDirectory` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExaProcessRunsInTheDirectoryItIsGiven` | `internal/test/cli/child_command_site_test.go` | AC-1. Runs `/bin/pwd` through `startExaProcess`, waits for the process to end, and reads the child's own answer off its stdout buffer. This is the daemon site: `startExaBGPClient` starts `ze start <config>` through the same function | |
| `TestLegacyParseChildRunsOutsideTheCheckout` | `internal/test/runner/child_command_site_test.go` | AC-2. Builds a `parsingRunner` whose `zePath` prints its working directory, runs one legacy `.conf` test, and asserts the recorded output names the per-test directory and not the base directory | |
| `TestChildCommandRefusesAnEmptyDirectoryAndTheCheckoutRoot` | `internal/test/runner/child_command_test.go` | AC-3, AC-4. Both refusals, and the error naming the program | |
| `TestRepositoryRootCommandIsTheOnlyRouteToTheCheckout` | `internal/test/runner/child_command_test.go` | AC-4. The named constructor answers a command rooted at the checkout, and the ordinary one cannot | |
| `TestNewChildWorkDirFallsBackOffSession` | `internal/test/runner/child_command_test.go` | A-5. An empty scratch root still yields a usable directory, and the answered remover deletes it | |
| `TestEveryHarnessSpawnNamesItsDirectory` | `internal/test/runner/child_command_lint_test.go` | AC-5, AC-6, AC-7, AC-11. The syntax-tree walk, the floor, and the alias case, driven over fixture sources in a temp tree and over the two real packages | |
| `TestExaBGPServerArgsKeepARelativeSaveDirectory` | `internal/test/cli/cmd_exabgp_test.go` | AC-8, R-2. The argument builder answers an absolute path for a relative `--save` value | |
| `TestChildWorkingDirectoryAnchorsOnlyRepositoryTools` | `internal/test/runner/work_dir_test.go` | Existing. It must still pass unchanged, which proves `childWorkingDirectory` kept its meaning while its rule moved | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| the lint's spawn-site floor | 1 to the count the walk finds | the count recorded when the lint lands | 0, an evaded walk, which must fail | N/A: a walk that finds more sites than the floor is the normal case and passes |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `harness-root-clean` | `test/plugin/harness-root-clean.ci` | An agent runs the suite in a shared checkout and finds no file it did not write. The test records the repository root's untracked set, runs a step that starts a `ze` daemon, and asserts the set is unchanged | |

The `.ci` route through `internal/test/fixture` was proved clean on 2026-09-07,
and its guard landed as `refuseRepoRoot`. This test states the same promise for
the routes this spec repairs, so a regression in either is one red test.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is tooling. No wire-visible behavior changes: every edited source file is under `internal/test/`, which no shipped build compiles | |

## Goal Validation

| Goal | Evidence that proves it |
|------|------------------------|
| G-1: no harness child inherits the checkout root | `TestExaProcessRunsInTheDirectoryItIsGiven` and `TestLegacyParseChildRunsOutsideTheCheckout` each read the CHILD's own answer, and each is recorded RED before the change and GREEN after. The first covers a daemon spawn site |
| G-2: a sixteenth site cannot be added wrong | `TestEveryHarnessSpawnNamesItsDirectory`, proved by adding a bare command call to a scratch copy of one package and recording the red, then removing it. The floor is proved separately, by pointing the walk at a package with no spawn and recording that red |
| G-3: no test loses a path it resolved against the working directory | The `parse`, `decode`, `web` and `exabgp` suites pass, and `TestExaBGPServerArgsKeepARelativeSaveDirectory` pins the one operator-visible path |
| G-4: the repository root stays clean across a full run | `git status --porcelain` captured before and after `./le verify current mode full`, with the two outputs identical, pasted into the closure section. This is the measurement the 2026-08-28 row used and the one the 2026-09-05 row failed |

## Files to Modify
- `internal/test/runner/runner.go` - `childWorkingDirectory` calls the shared rule instead of restating it; `(*Runner).Build` moves its three `go build` children to the repository-root constructor
- `internal/test/runner/runner_exec.go` - `runOrchestrated` moves its two children to the shared constructor, and its inline directory creation is deleted in favor of the shared maker
- `internal/test/runner/decoding.go` - site 1: `decodingRunner` gains a run-scoped directory, created in `Run` and removed when the run ends
- `internal/test/runner/parsing.go` - sites 2 and 3: a per-test directory, the config path made absolute first, and the inline-config temp file moved to the session scratch root
- `internal/test/runner/runner_validate.go` - site 4: `decodeToEnvelope` takes the directory from `validateJSON`, which already holds the record
- `internal/test/cli/cmd_bgp.go` - site 5: a directory for the client-only daemon, with the config path made absolute first; `buildZe` moves to the repository-root constructor
- `internal/test/cli/cmd_web.go` - sites 6, 7 and 10 take the temp directory the function already creates; sites 8 and 9 create one and store it on `zeTestWebServer.tempDir`; site 11 takes a directory; `zeTestBuildChaos` moves to the repository-root constructor
- `internal/test/cli/cmd_exabgp.go` - sites 12, 13 and 14: the client takes its own config directory, the server and the migration take the suite's run directory, and the `.ci` path, the migration source and `--save` are made absolute first
- `internal/test/cli/cmd_exabgp_process.go` - site 15: `startExaProcess` takes the directory as a parameter and passes it to the constructor
- `internal/test/cli/cmd_exabgp_test.go` - the `--save` assertion
- `docs/architecture/testing/runner-architecture.md` - the "Scratch roots" section states that every harness child runs in a named directory, that only a repository-anchored tool keeps the checkout root, and that the lint enforces it
- `docs/functional-tests.md` - the note that a suite writes nothing into the checkout

## Files to Create
- `internal/test/runner/child_command.go` - the child constructor, the repository-root constructor, and the work directory maker
- `internal/test/runner/child_command_test.go` - the refusals and the off-session fallback
- `internal/test/runner/child_command_lint_test.go` - the syntax-tree walk, the floor, and the alias case
- `internal/test/runner/child_command_site_test.go` - the legacy parse site, reading the child's own answer
- `internal/test/cli/child_command_site_test.go` - the ExaBGP daemon site, reading the child's own answer
- `test/plugin/harness-root-clean.ci` - the functional guard over the repository root

### The mechanism

| Symbol | Takes | Answers | Refuses |
|--------|-------|---------|---------|
| child command constructor | a context, a directory, a program name, arguments | a command whose working directory is the named one | an empty directory, and a directory `sessionpath.IsRepoRoot` accepts. The error names the program it refused to start |
| repository-root command constructor | a context, the checkout root, a program name, arguments | a command rooted at the checkout, which is what `go build`, `go test` and `./le` need | nothing. Naming the root is this symbol's whole job, and its name is what makes the choice visible to a reader and greppable |
| child work directory maker | a name prefix | a directory under `sessionpath.DefaultScratchRoot()`, and the function that removes it | a root it cannot create under, by answering the error |

The two constructors are the one declaration. The lint is what makes them the
only route. `childWorkingDirectory` keeps its per-record job and calls the same
rule, so the `.ci` route and these fifteen sites cannot drift apart.

### Per-site directory decision

| # | Enclosing function | Directory it gets | Where that directory comes from |
|---|--------------------|-------------------|--------------------------------|
| 1 | `(*decodingRunner).runTest` | one directory for the whole decode run | new, created in `(*decodingRunner).Run` and removed when the run ends. A-1 is what allows one directory rather than one per test |
| 2, 3 | `(*parsingRunner).runLegacyTest` | a per-test directory | new, created in that function. The config path is made absolute against the runner's base directory first |
| 4 | `(*Runner).decodeToEnvelope` | `rec.WorkDir` | it already exists. `validateJSON` holds the record and passes the directory down |
| 5 | `zeTestRunClientOnly` | one directory for the client-only run | new, created in that function. The config path is made absolute against the base directory first |
| 6, 7 | `zeTestStartLGServer` | the temp directory the function already creates | it exists, and the stop path already removes it |
| 8 | `zeTestStartLGNoEngineServer` | a new temp directory | new, created in that function and stored on `zeTestWebServer.tempDir`, so the existing stop path removes it |
| 9 | `zeTestStartChaosServer` | a new temp directory | new, created in that function and stored on `zeTestWebServer.tempDir` |
| 10 | `zeTestStartWebServer` | the temp directory the function already creates | it exists |
| 11 | `zeTestCloseAllBrowserSessions` | a temp directory created and removed around the call | new. The call is a best-effort cleanup with no other state, so the directory lives only as long as it |
| 12 | `runExaBGPServerForeground` | one directory for the ExaBGP run | new. The `.ci` path and the operator's `--save` value are made absolute first |
| 13 | `runExaBGPClientForeground` | the directory holding the migrated config | it already exists: `exaBGPClientConfig` created it and wrote `migrated.conf` into it. The daemon's config directory and its working directory then agree |
| 14 | `migrateExaBGPConfig` | the ExaBGP run directory | shared with site 12. The migration source is made absolute first |
| 15 | `startExaProcess` | a new parameter | the caller decides: `startExaBGPClient` passes the directory holding the migrated config, `startExaBGPServer` passes the ExaBGP run directory |

No site keeps the checkout root. Two groups look like they need it and do not.
The five `go build` children already name the base directory explicitly, which
is the visible choice this spec asks for, and they keep it through the
repository-root constructor. The five sites that pass a path resolved against
the working directory need that path made ABSOLUTE, not the directory left
inherited.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface changes. The feature is internal to the test harness |
| YANG validation constraints | N-A | No leaf is added |
| YANG custom validators | N-A | No leaf is added |
| CLI commands/flags | N-A | No flag is added or changed. `ze-test exabgp --save` keeps its exact meaning, which is what AC-8 pins |
| CLI grammar (keyword before value) | N-A | No command is added |
| Editor autocomplete | N-A | No leaf is added |
| Functional test for new RPC/API | Yes | `test/plugin/harness-root-clean.ci` |
| Pipe completeness | N-A | No command output is produced |
| Env var registration | N-A | No `ze.*` setting is added. `sessionpath` already reads `ze.session.id`, with `CLAUDE_CODE_SESSION_ID` as its direct-launch fallback |
| Doctor check for runtime dependencies | N-A | No file path, socket, port, kernel module, binary or certificate is added at RUNTIME. The directories are created and removed by the harness inside one run, and no shipped build compiles `internal/test/` |
| Prometheus counters/metrics | N-A | No observable daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No SAFI, capability or attribute is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No operator-visible behavior changes. `docs/features.md` describes the product, and this is harness internals |
| 2 | Config syntax changed? | No | No parser and no YANG change |
| 3 | CLI command added/changed? | No | No `ze` or `ze-test` flag is added or given a new meaning |
| 4 | API/RPC added/changed? | No | No command handler is touched |
| 5 | Plugin added/changed? | No | No plugin registers or changes |
| 6 | Has a user guide page? | No | The harness is documented for contributors, which rows 10 and 12 cover |
| 7 | Wire format changed? | No | Nothing reaches the wire |
| 8 | Plugin SDK/protocol changed? | No | `internal/test/fixture` is untouched, so the plugin process protocol is unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Scope is tooling. No `rfc/short/` row moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | No daemon capability changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/runner-architecture.md`, the "Scratch roots" section |
| 13 | Route metadata keys added/changed? | No | No metadata key is touched |
| 14 | Prometheus counters added/changed? | No | No counter is defined |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry entry changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time: run `./le spec citation anchors spec plan/immediate/spec-test-children-run-in-the-checkout-root.md` and name every page it lists. Known from `ai/CODE-TO-DOCS.md`: `docs/architecture/testing/runner-architecture.md` and `docs/functional-tests.md` are anchored from the files in Files to Modify, and `docs/architecture/testing/ci-format.md` is anchored from `parsing.go`, `decoding.go`, `runner_validate.go` and `cmd_exabgp.go`. `ci-format.md` describes the `.ci` FORMAT, which this spec does not change, so it is named here as unaffected with that reason |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/functional-tests.md` shows `ze-test` invocations. Check each against the changed argument handling, in particular any example passing a relative `--save` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the constructor exists and is reachable, and the lint sees the sites
   - Tests: `TestChildCommandRefusesAnEmptyDirectoryAndTheCheckoutRoot`, `TestRepositoryRootCommandIsTheOnlyRouteToTheCheckout`, `TestNewChildWorkDirFallsBackOffSession`, `TestEveryHarnessSpawnNamesItsDirectory`
   - Files: `internal/test/runner/child_command.go`, `internal/test/runner/child_command_test.go`, `internal/test/runner/child_command_lint_test.go`
   - Verify: the refusal tests pass, and the lint FAILS naming all fifteen sites. That failing list is the work order for phases 3 and 4
2. **Phase: the two site tests go red** -- the discrimination is observed before any site is repaired
   - Tests: `TestExaProcessRunsInTheDirectoryItIsGiven`, `TestLegacyParseChildRunsOutsideTheCheckout`
   - Files: `internal/test/cli/child_command_site_test.go`, `internal/test/runner/child_command_site_test.go`
   - Verify: both fail, and each failure names the directory the child actually ran in. Record both outputs. They are the RED half of G-1
3. **Phase: `internal/test/runner`, sites 1 to 4** -- each site names a directory, and each path argument is made absolute first
   - Tests: `TestLegacyParseChildRunsOutsideTheCheckout` turns green; `TestChildWorkingDirectoryAnchorsOnlyRepositoryTools` still passes; the `parse` and `decode` suites pass
   - Files: `decoding.go`, `parsing.go`, `runner_validate.go`, `runner.go`, `runner_exec.go`
   - Verify: A-2 is settled by the legacy `.conf` half of the parse suite, A-1 by listing the decode run directory afterwards, and AC-11 by grepping the package for a second work-directory creation
4. **Phase: `internal/test/cli`, sites 5 to 15** -- the web, BGP and ExaBGP suites
   - Tests: `TestExaProcessRunsInTheDirectoryItIsGiven` turns green; `TestExaBGPServerArgsKeepARelativeSaveDirectory`; the `web` and `exabgp` suites pass
   - Files: `cmd_bgp.go`, `cmd_web.go`, `cmd_exabgp.go`, `cmd_exabgp_process.go`, `cmd_exabgp_test.go`
   - Verify: the lint passes over both packages, and A-3, A-4, A-6 and R-2 are settled by the suites, the build and the `--save` test
5. **Phase: the guard is proved and the pages are corrected** -- the lint's own discrimination, and the two pages
   - Tests: `harness-root-clean`; a scratch spawn site added to force the lint red, then removed; the walk pointed at a package with no spawn to force the floor red
   - Files: `test/plugin/harness-root-clean.ci`, `docs/architecture/testing/runner-architecture.md`, `docs/functional-tests.md`
   - Verify: both recorded reds, and the two pages stating the rule and carrying source anchors that name `child_command.go`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All fifteen rows of the per-site table name a directory. Read each site after the change rather than trusting the lint, because the lint proves the ROUTE and the table names the DECISION |
| Feature completeness | The eight daemon sites (5, 6, 7, 8, 9, 10, 12, 13, 15) each run in a directory the harness removes, and no daemon writes into the checkout |
| Correctness | Every path argument is absolute before the spawn: the legacy test file, the record's `config` option, the ExaBGP `.ci` path, the migration source, and the operator's `--save` |
| Naming | The child constructor's name says it gives a directory. The repository-root constructor's name says the checkout root is a CHOICE, so a reader can grep every deliberate use |
| Data flow | The directory is passed as an argument at every site. No site reads a package-level variable or an environment variable to find it |
| Rule: `ai/rules/principles.md` | The rule is declared once. `childWorkingDirectory` calls it and does not restate it |
| Rule: `ai/rules/evidence.md` | The lint resolves `os/exec` by import path and carries a floor, so an empty walk fails |
| Rule: `ai/rules/no-layering.md` | The inline directory creation in `runOrchestrated` is DELETED when the shared maker lands |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Fifteen sites name a directory | `grep -n "exec.Command" internal/test/runner/*.go internal/test/cli/*.go` filtered to non-test files names only `child_command.go` |
| The lint runs in the ordinary Go stage | `go test ./internal/test/runner/ -run TestEveryHarnessSpawnNamesItsDirectory -v` |
| The daemon site is proved | `go test ./internal/test/cli/ -run TestExaProcessRunsInTheDirectoryItIsGiven -v`, with the pre-change red pasted beside it |
| One work directory maker | `grep -n "MkdirTemp" internal/test/runner/*.go` shows the work-directory prefix in `child_command.go` only |
| The root stays clean | `git status --porcelain` before and after `./le verify current mode full`, both outputs pasted |
| The pages are corrected | `grep -n "child_command" docs/architecture/testing/runner-architecture.md docs/functional-tests.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret material at rest | The class already wrote two ed25519 private keys to the repository root. After the change, run the web and ExaBGP suites and confirm every generated key sits inside a directory the harness removes |
| Path handling | Every path from a fixture or an operator flag is made absolute. No path is joined against a directory the caller did not name |
| Fail-closed guard | The constructor answers an error rather than starting a command with no directory. Confirm no call site discards that error |
| Resource exhaustion | Every created directory has a matching remover and every caller defers it. A directory a killed run leaves behind carries the prefix that names what made it |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| A suite fails at a missing path | R-1: the path resolved against the working directory. Anchor it. Do not restore the inherited directory |
| A suite fails at a missing file an earlier run wrote | R-4: the test was passing on a cross-run artifact. Fix the test to declare the file |
| The lint passes with fifteen sites still bare | R-3: the walk is not seeing them. Check the import-path resolution and the floor before touching a site |
| Lint failure | Fix inline. If architectural → DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The fact was declared in August and never reached three of the four runners. A declaration with no check reaches only the code its author edited that day.
- `refuseRepoRoot` guards the CHILD side, and it works because a fixture driver is harness code. The same guard cannot go into `ze`: a developer runs `ze` from the checkout on purpose, so the product must not refuse it. That asymmetry is what puts this guard at the spawn site instead.
- Two of the fifteen sites already created the right directory and passed it as `ze.config.dir` while leaving the working directory alone. Half a repair reads as a whole one, which is why the table records a decision per site rather than a count.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One shared constructor in `internal/test/runner`, used by both packages | Fifteen independent directory assignments | The journal class holds four rows across four dates, and the last repair fixed one route and left three. An edit per site leaves nothing behind that a sixteenth site meets |
| The constructor lives in `internal/test/runner` | A new leaf package for it | `internal/test/cli` already imports `runner` (`go list -deps`), and `runner` already owns `childWorkingDirectory`, `repositoryAnchoredBinary`, `workDirPrefix` and `Record.WorkDir`. A new package moves the declaration away from the code that holds it |
| A source-scanning lint test in the package | A sixth check in `./le repository`; extending `refuseRepoRoot` to the product | `./le repository` composes its checks in one central slice, edited for a rule about one package, and it runs less often than `go test`. `refuseRepoRoot` guards the child, and these children are `ze`, `ze-test` and `ze-chaos`, which a developer legitimately runs from the checkout. The lint runs in the ordinary Go stage and is owned by the package it governs |
| The lint's scope is `internal/test/runner` and `internal/test/cli` | All of `internal/test/` | `internal/test/fixture` holds roughly 180 spawns whose parent already runs in the per-test directory, which `refuseRepoRoot` proves. Widening the lint forces 180 rewrites that fix nothing. The boundary is which process's working directory is the checkout root, and only these two packages' is |
| Sites 1, 12 and 14 share one directory per run; sites 2 and 3 get one per test | A per-test directory everywhere | The decode children take no path argument and write no per-test file (A-1), and the decode suite runs thousands of tests. The legacy parse children take a config path and can write beside it, so they get their own |
| A path argument is made absolute at the site | Letting the new directory resolve it | The path is the caller's, and its meaning must not change. This is the repair `runOrchestrated` already made on 2026-08-28 |
| Site 13 gets the directory holding its own config | The shared run directory | `exaBGPClientConfig` already builds a directory per test, seeds it, and writes the migrated config into it, precisely so concurrent daemons stop sharing one `database.zefs`. The daemon's config directory and its working directory then name the same place |

## Known Limitations
- `internal/test/fixture` keeps `refuseRepoRoot` as its only guard, and its own spawns do not route through the constructor. That is deliberate: a fixture driver's working directory is already the per-test directory, and `refuseRepoRoot` fails closed if it ever is not.
- `internal/test/perfrunner`, `internal/test/localdatacoverage` and `internal/test/golden` each hold one spawn and sit outside the lint's scope. No suite runner starts them from the checkout root. Widening the lint to cover them is a separate decision that needs its own reading of each caller.
- The `go test` half of this class stays open. A Go test whose working directory is its own package directory still writes daemon state into the source tree, which the 2026-09-03 journal row records under `cmd/ze/hub/`. This spec repairs the `ze-test` half. The Go half is named in that row and belongs to a spec of its own.

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`), not library-only
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
