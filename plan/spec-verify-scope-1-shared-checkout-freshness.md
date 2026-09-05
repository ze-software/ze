# Spec: verify-scope-1-shared-checkout-freshness

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-verify-scope-0-umbrella.md` |
| Phase | 1-2 (scoped freshness), 3 (moved-path record), 4 (red attribution), 5 (debt clearing) and 6 (rules) done; functional tests, docs, A-1 and the Architectural Verification table done |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make a verify verdict survive another session's concurrent edit, and make
verification debt clearable by running the gate rather than by editing a table.

Six sessions share this checkout and it carries 248 uncommitted files. Three
mechanisms turn that into a permanent block:

1. **Freshness is whole-tree.** `tree_hash` (`internal/le/verify/status/answer.go`)
   folds `HEAD`, the whole `git diff HEAD`, and every untracked file into one
   number. Any byte written by any session flips it.
2. **A concurrent edit voids the whole run.** `writeVerifyStatus`
   (`internal/le/verify/engine/run.go`) compares the tree hash before and after the
   run and writes `treeMovedSentinel` when they differ. No tree hashes to
   `tree-moved-during-run`, so the record reports STALE for ever. The live
   `tmp/ze-verify.status` holds exactly that after a 79 minute run.
3. **Every escape is permanent.** `record_debt` (`internal/le/commit/prepare.go`)
   writes a row per override into `plan/verification-debt/<session>.md`, and
   `open_debt_rows` refuses `--push` while one row is open. The ledger holds 232
   open and 22 cleared rows, and no script writes `cleared`.

The scoped answer already exists and has no production caller. `dirty_manifest`
and `manifest_scoped` (`internal/le/verify/status/answer.go`) record a per-path
fingerprint of everything differing from HEAD, and `verify-status.sh check
<PATH>...` compares only the named paths. The function's own comment states the
reason: "The commit is scoped to a file list; the evidence must be scopeable to
the same list, or a session can never hold evidence about its own code".
`verify_status` (`internal/le/commit/prepare.go`) calls `[str(script), "check"]`
with no paths.

`plan/journal/concurrent-rfc-gate-stale.md` records nine occurrences of this
class since 2026-07-30.

## Required Reading

### Architecture Docs
- [ ] `docs/features/ai-first.md` - register once, expose everywhere: one command and discovery surface
- [ ] `ai/rules/precommit-verify.md` - what a commit owes in verification, and how to judge a red in a shared checkout
  → Constraint: a red another session produced is not this session's to clear
- [ ] `ai/rules/git-safety.md` - the commit-script path and the verify gates on it
  → Constraint: the helper's refusals are the enforcement point; changing them changes the contract every session reads

**Key insights:**
- `STRUCTURAL_GATES` (`internal/le/commit/prepare.go`) is the set `--unverified` cannot wave through, because a structural red means the tree is broken rather than flaky. Scoping must not weaken that: a structural red inside the session's own paths still blocks.
- `TRACKED_BUILD_GATE` is exempt by design and the owner ruled on it on 2026-08-04. Leave it alone.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/verify/status/answer.go` - `tree_hash` (whole-tree), `dirty_manifest` (per-path), `manifest_scoped` (compares named paths), the `check` command with and without arguments
- [ ] `internal/le/verify/engine/run.go` - `writeVerifyStatus`, `computeTreeHash`, `treeMovedSentinel`, `runVerify`'s `startHash`
- [ ] `internal/le/commit/prepare.go` - `verify_status`, `structural_gate_reds`, `STRUCTURAL_GATES`, `record_debt`, `debt_owed`, `open_debt_rows`, `DEBT_FLAGS`
- [ ] `plan/verification-debt/*.md` - the row format and the reasons recorded

**Behavior to preserve:**
- `tmp/ze-verify.status` keeps its path and its field names (`exit`, `timestamp`, `mode`, `skipped`, `git_sha`, `tree_hash`). Both readers parse them.
- `verify-status.sh check` with no arguments keeps its whole-tree meaning. `hook-parity-check.py` pins its exit code.
- A structural gate red inside the session's own paths still refuses the commit.
- `TRACKED_BUILD_GATE`'s post-commit exemption is unchanged.

**Behavior to change:**
- `verify_status` passes the commit's file list, so the verdict answers about the session's own paths.
- A concurrent edit stops voiding the whole run. The record names which paths moved instead of voiding all of them.
- Debt rows gain a clearing path that re-runs the owed gate.
- A red attributable only to paths outside the session's change set stops being charged to that session.

## Data Flow (MANDATORY)

### Entry Point
- `internal/le/commit/prepare.go create --file <path> ...`, which already knows the exact file list the commit will carry.

### Transformation Path
1. `runVerify` records `startHash` and the start manifest, then runs the stages.
2. `writeVerifyStatus` writes the record plus the manifest of paths that moved during the run.
3. `verify_status` calls `verify-status.sh check <the commit's paths>`.
4. `manifest_scoped` compares only those rows, and reports FRESH when they are unchanged.
5. `structural_gate_reds` attributes each red to the paths that produced it, and the helper charges debt only for reds inside the session's own set.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Verify runner ↔ status file | `tmp/ze-verify.status`, `tmp/ze-verify-manifest.txt` | Yes: `TestWriteVerifyStatusRecordsMovedPathsNotSentinel` (`internal/le/verify/engine/verifyengine_test.go`) writes the pair and reads it back through `verify-status.sh` |
| Status file ↔ commit helper | `verify-status.sh check <paths>` exit code | Yes: `TestVerifyStatusScope` (`internal/le/`) and `test/runner/verify-scope-freshness-scoped.ci` |
| Failure index ↔ commit helper | `tmp/ze-verify-failures.json` | Yes: `TestStructuralRedAttribution` and `TestDebtNotChargedForForeignRed` (`internal/le/`) |

### Integration Points
- `manifest_scoped` - already implemented, needs a caller.
- `failureGroup.Related` (`internal/le/verify/engine/run.go`) - already carries related paths per failure group, and is the natural attribution source.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `verify_status` (`internal/le/commit/prepare.go`) asks `verify-status.sh check <paths>` and reads its exit code. It never opens `tmp/ze-verify.status` or `tmp/ze-verify-manifest.txt` itself, so the status script stays the only reader of both |
| No unintended coupling (components stay isolated) | Yes, with one literal carried on both sides | The Go runner and the shell script cannot import each other, so the FILE FORMAT is the contract: `movedDuringRun` (`internal/le/verify/engine/run.go`) and `MOVED_MARKER` (`internal/le/verify/status/answer.go`) spell the same word, and each carries a comment naming the other. No symbol crosses |
| No duplicated functionality (extends existing, does not recreate) | No, and the duplication is deliberate | `manifest_scoped` and `dirty_manifest` (`internal/le/verify/status/answer.go`) already existed and only gained a caller. But `computeDirtyManifest` (`internal/le/verify/engine/run.go`) is a second implementation of `dirty_manifest`, in Go, and says so in its own comment. The runner needs the fingerprint the stages READ, which is taken at run start and no later shell call can recover. `computeTreeHash` was already a mirror of `tree_hash` before this spec |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding and no hot path. Every file this spec touches is shell, Python, or a once-per-run Go writer |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `DEBT_GATE_RUNNERS` (`internal/le/commit/prepare.go`) is a table keyed through `dict(DEBT_FLAGS)`, so a new override adds one row and a reworded gate cell cannot orphan an entry. `clear_debt` looks the runner up; it holds no branch per gate |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A stage's verdict about path P is unaffected by an edit to path Q outside P's package and importers | The stages are per-package linters, per-package tests, and whole-tree structural checks | A scoped FRESH hides a real red | The structural gates keep a whole-tree scope; only the per-package stages scope | **BROKEN IN PART, 2026-08-19, and the basis was wrong in two ways.** (1) NO stage is per-package: `stagesForMode` (`internal/le/verify/engine/run.go`) returns whole-tree `make` targets for both modes, and this spec narrowed no stage. What narrowed is the FRESHNESS question alone -- `manifest_scoped` (`internal/le/verify/status/answer.go`) compares the named paths' manifest rows, so the claim a scoped FRESH supports is "the paths this commit carries are byte-identical to the ones the run read", never "a stage re-ran over P alone". (2) The exclusion names the wrong direction. P's importers cannot change P's own compile or test verdict; P's DEPENDENCIES can, and the row does not exclude them. So an uncommitted Q that P imports, edited after the pass, leaves a verdict about P that was produced against a Q the checkout no longer holds. That residual is not new and is not closed by scoping: the local tree is never the tree CI builds, and `./le repository tracked-build check` closes it after the commit by compiling what git holds. The structural half of the mitigation DOES hold, by `structural_gate_reds` (`internal/le/commit/prepare.go`): a red charges unless every one of its groups named files and every one is foreign, and a red with no scope charges unconditionally |
| A-2 | `failureGroup.Related` already names enough paths to attribute a red | `classifyStage` populates it per failure kind | Attribution needs a new producer per stage | Read `classifyStage` and its group builders before designing AC-4 | **BROKEN IN PART, 2026-08-19.** `Related` carries a FILE PATH only in `classifyLint`. `classifyVet` carries a package pattern (`./pkg/...`), which attributes to a directory. `classifyWiringDocs` carries a check name (`"wiring"`, or the sub-target), `classifyFunctional` a suite name, `classifyExabgp` test names, and `genericGroup` the stage's own name. AC-4 is answerable for lint and vet, and is NOT answerable for the rest from today's data. See the Decision below |
| A-3 | Clearing a debt row by re-running the owed gate is cheap enough to be used | The owed gates are named per row | Clearing costs another full hour and nobody runs it | Measure one clear against `tmp/.ze-verify-duration.txt` | **CONFIRMED, 2026-08-19, and the reason is amortization rather than a cheap gate.** `clear_debt` runs each DISTINCT gate once per pass, not once per row, so the 222 open rows naming `./le verify current mode full` or the structural set are judged by one run of it. That run is the recorded 924-3889s (`tmp/.ze-verify-duration.txt`, 4 runs, 2026-08-18/19). The `discovery-index freshness` gate is 8.8s measured over HEAD and covers 16 rows. The 32 `independent critical review` rows have no runnable gate at any price and stay open by design |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scoped freshness lets a session commit over another session's genuine structural red | CI red on a commit the local gate passed | Structural gates stay whole-tree scoped; only the per-package verdicts scope |
| R-2 | Per-path move detection is wrong for a stage that reads the whole tree | A stage reports green for a tree it never saw | Each stage declares its scope, and a whole-tree stage keeps the whole-tree rule |
| R-3 | Automatic debt clearing marks a row cleared without the gate passing | A cleared row for a gate that is still red | Clearing records the gate's exit code and refuses to write `cleared` on non-zero |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A commit lands over an unverified change. Nothing at runtime: every file is commit and verify tooling |
| How is it reverted? | Single commit revert. The three edited files are independent |
| Who else touches this path? | Every session in this checkout reads `commit_helper.py` and `verify-status.sh` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `commit_helper.py create --file X` | → | `verify_status(repo, paths)` | `test_verify_status_passes_commit_paths` |
| `verify-status.sh check A B` | → | `manifest_scoped` | `test_check_scoped_ignores_unnamed_paths` |
| a concurrent edit during a run | → | `writeVerifyStatus` moved-path record | `TestWriteVerifyStatusRecordsMovedPathsNotSentinel` |
| `./le commit debt-clear` | → | the clearing entry point | `test_debt_clear_reruns_the_owed_gate` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A green verify ran, then another session edits a file the commit does not carry | `commit_helper.py create --file <own paths>` reports FRESH and does not require an override |
| AC-2 | A green verify ran, then the committing session edits a file the commit DOES carry | The helper reports STALE and refuses, as today |
| AC-3 | A verify run is in flight and another session writes an unrelated file | The record names the moved paths; it does not write a value that can never match |
| AC-4 | A structural gate is red, its failure group carries FILE PATHS, and every one lies outside the commit's file list | The helper does not charge a debt row for that gate |
| AC-5 | A structural gate is red and any path in its failure group lies inside the commit's file list | The helper refuses, as today |
| AC-4b | A structural gate is red and its failure group carries no path at all (a check name, a suite name, or the stage's own name) | The helper charges the debt row, as today, and names WHICH group it could not attribute |
| AC-6 | An open debt row names a gate that now passes over the committed code | `./le commit debt-clear` re-runs that gate, records its exit, and sets the row to `cleared` only on exit 0 |
| AC-7 | An open debt row names a gate that is still red | The clearing target leaves the row open and prints the gate's output |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWriteVerifyStatusRecordsMovedPathsNotSentinel` | `internal/le/verify/engine/verifyengine_test.go` | AC-3: a concurrent edit records paths, never an unmatchable value | |
| `TestStagesForModeMatchesGolden` | `internal/le/verify/engine/verifyengine_test.go` | The stage list stays single-sourced | |
| `test_verify_status_passes_commit_paths` | `internal/le/` | AC-1, AC-2: the helper scopes to its own file list | |
| `test_check_scoped_ignores_unnamed_paths` | `internal/le/` | The scoped compare reads only named paths | |
| `test_debt_not_charged_for_foreign_red` | `internal/le/` | AC-4, AC-5: attribution decides the charge | |
| `test_debt_clear_reruns_the_owed_gate` | `internal/le/` | AC-6, AC-7: clearing runs the gate and honors its exit | green, with `TestDebtClear`'s nine other cases |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| scoped path count | 0-N | N | N/A | N/A |
| debt row count per shard | 0-N | N | N/A | N/A |

<!-- The only numbers here are counts with no upper bound and no failure mode at
     zero: zero scoped paths falls back to the whole-tree compare, which is
     today's behavior. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-scope-freshness-scoped` | `test/runner/verify-scope-freshness-scoped.ci` | A developer's record stays FRESH for their own paths while a second writer edits an unrelated file | green in `./le functional runner` (3/3). Drives the real `verify-status.sh` over a throwaway git repo it builds itself, so the shared `tmp/ze-verify.status` is never touched. Discriminates: with the scoped arm of `check` removed, seq=2 fails with "AC-1: scoped check refused my untouched path: STALE: tree changed since last PASS" |
| `verify-scope-debt-clear` | `test/runner/verify-scope-debt-clear.ci` | A developer clears a debt row, and the clearing re-runs the owed gate rather than trusting the edit | green in the retired `ze-functional-runner-test` (current: `./le functional runner`) (3/3). Drives `commit_helper.py debt-clear` against two throwaway repos whose only `make` target is the one the row owes, one exiting 0 and one exiting 3, and binds the retired `ze-verify-debt-clear` (current: `./le commit debt-clear`) to that entry point by reading its recipe with `make -n`. Discriminates: with the gate's exit trusted rather than read, AC-6 fails on "the owed gate did not run" and AC-7 on "cleared 1 row(s), 0 still open" |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling. No wire-visible behavior changes | |

## Files to Modify
- `internal/le/verify/status/answer.go` - the scoped `check` path, already implemented, gains its contract test
- `internal/le/verify/engine/run.go` - `writeVerifyStatus` records moved paths instead of the sentinel
- `internal/le/commit/prepare.go` - `verify_status` takes paths; red attribution; the debt clearing entry point
- `internal/le/` native action tables - the `ze-verify-debt-clear` target
- `ai/rules/precommit-verify.md` - the judging rule changes, so its text does too
- `ai/rules/git-safety.md` - the commit path's verify gate description

## Files to Create
- `internal/le/` - the scoped-compare contract
- `test/runner/verify-scope-freshness-scoped.ci` - end-to-end scoped freshness
- `test/runner/verify-scope-debt-clear.ci` - end-to-end debt clearing
- `docs/architecture/testing/verify-freshness-scope.md` - the scoped-freshness contract

Both `.ci` files build their own git repo under the runner's tmpfs directory and
run every command inside it, because `verify-status.sh` resolves
`tmp/ze-verify.status` and `tmp/ze-verify-manifest.txt` against the working
directory and several sessions read the real pair.

One step of the debt-clearing scenario is out of a `.ci`'s reach and is proven a
different way. Running `./le commit debt-clear` for real would enter this
repository's own gates from inside a running functional suite: the target's
runner for the commonest owed gate is `gate_command("make", "./le verify current mode full")`
(`internal/le/commit/prepare.go`), which is the hour-long run. The test therefore
reads the target's recipe with `make -n`, which prints it without executing it
(the recipe holds no `$(MAKE)`), asserts that the recipe runs
`commit_helper.py debt-clear`, and then drives that entry point for real against
the fixture repos. What is proven is the binding plus the behavior; what is not
proven is that `make` in this repository's own root reaches it, which no test
can assert without paying for the gates.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Commit tooling, no runtime config |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | No `ze` subcommand; a make target only |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No `ze` CLI output added |
| Env var registration | N-A | No `ze.*` leaf added |
| Doctor check for runtime dependencies | N-A | No new runtime path, socket, port, module, or binary |
| Prometheus counters/metrics | N-A | No daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC surface |
| 10 | Test infrastructure changed? | Yes | Done: `docs/functional-tests.md`, the Runner row of the Functional Suite Inventory now names `verify-scope-freshness-scoped.ci` and `verify-scope-debt-clear.ci` and says what they spawn, and the verify-artifact table gained `tmp/ze-verify-manifest.txt` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | Done: `docs/architecture/testing/verify-freshness-scope.md` (new) carries the two artifacts, the scoped question, the four conditions the scope never widens, the moved-path marker and its one blind spot, red attribution, and debt clearing. `docs/functional-tests.md` links it |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` and `ai/` for anchors naming `commit_helper.py`, `verify-status.sh`, `verify_run.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/git-safety.md` "Commit Rules" and `ai/rules/precommit-verify.md` both describe the refusals this changes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the scoped path reachable and prove it is not yet honored
   - Tests: `test_verify_status_passes_commit_paths`, `test_check_scoped_ignores_unnamed_paths`
   - Files: `internal/le/commit/prepare.go`, `internal/le/`
   - Verify: the tests fail because `verify_status` still calls `check` with no paths
2. **Phase: Scoped freshness** -- `verify_status` takes the commit's file list and passes it through
   - Tests: as above, plus `verify-scope-freshness-scoped.ci`
   - Files: `internal/le/commit/prepare.go`
   - Verify: AC-1 and AC-2 both hold
3. **Phase: Moved-path record** -- replace the whole-run sentinel with a per-path record
   - Tests: `TestWriteVerifyStatusRecordsMovedPathsNotSentinel`
   - Files: `internal/le/verify/engine/run.go`, `internal/le/verify/status/answer.go`
   - Verify: AC-3 holds, and a run whose tree moved still answers about the paths that did not
4. **Phase: Red attribution** -- charge debt only for reds inside the session's own paths
   - Tests: `test_debt_not_charged_for_foreign_red`
   - Files: `internal/le/commit/prepare.go`
   - Verify: AC-4 and AC-5 both hold
5. **Phase: Debt clearing** -- a make target that re-runs the owed gate
   - Tests: `test_debt_clear_reruns_the_owed_gate`, `verify-scope-debt-clear.ci`
   - Files: `internal/le/` native action tables, `internal/le/commit/prepare.go`
   - Verify: AC-6 and AC-7 both hold, and the 232 open rows are re-judged rather than deleted
6. **Phase: Rules** -- update `ai/rules/precommit-verify.md` and `ai/rules/git-safety.md` to state the new judging rule

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | The scoped path is reachable from `commit_helper.py create`, not only from the shell |
| Correctness | A structural red inside the session's own paths still refuses. Scoping must not become a bypass |
| Naming | One name for the commit's file list, shared by the helper and the status script |
| Data flow | Attribution reads `failureGroup.Related`, and does not re-derive paths from the log text |
| Rule: `ai/rules/git-safety.md` | The helper's refusals stay fail-closed when the checker is unavailable |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Scoped freshness reaches the helper | `grep -n 'check' internal/le/commit/prepare.go` shows the path list passed |
| The sentinel no longer voids a run | `grep -c treeMovedSentinel internal/le/verify/engine/run.go` |
| Debt is clearable | `./le commit debt-clear` re-judges the open rows |
| The ledger shrinks | `grep -c '| open |' plan/verification-debt/*.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Path arguments reach `awk` inside `manifest_scoped`. A path holding a shell metacharacter or a newline must not change what is compared |
| Authorization that could fail open | `verify_status` returns `unknown` when the checker is missing, and `unknown` does not block. Scoping must not widen that hole |

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

- The scoped freshness mechanism was written for this exact problem, carries a comment naming it, and was never given a caller. The cost of the missing call is measurable: 232 open debt rows and a `--push` that no session can use.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Scope only the per-package verdicts, keep structural gates whole-tree | Scope every stage | A structural red means the tree is broken. `STRUCTURAL_GATES` exists because those reds are never flaky, and scoping them would turn the strongest gate into the weakest |
| Record which paths moved, rather than voiding the run | Keep the sentinel and shorten the run | Shortening the run does not help: any concurrent edit still voids it, and six sessions guarantee one |
| Clearing re-runs the gate | Let a human edit the row to `cleared` | That is today's behavior and it produced 232 open rows against 22 cleared. A record is not a fix |
| Attribute a red only where the failure group carries a path, and charge the debt otherwise | Attribute every red; or attribute none | A-2 turned out to be true for `classifyLint` alone. Guessing attribution for a group that names a check or a suite would let a real red go uncharged, and rung 3 of `ai/rules/rule-precedence.md` forbids silently reducing scope. Charging what cannot be attributed is the fail-closed half, and AC-4b makes the helper SAY which group it could not attribute, so the gap is visible rather than silent |

## Known Limitations
- A stage that genuinely reads the whole tree still needs a still tree. This spec does not make those stages scoped; sub-spec 2 decides which of them can be.
- `./le doc wiring` IS attributable, and it was the gate 65 of the 95 open structural debt rows name. Reading the stage's prose could never attribute it: `classifyWiringDocs` matches a check NAME, and `main()` in `internal/le/doc/wiring/wiring.go` runs four sub-checks that print `<path>:<line>: ...` beside two that name no file of ours, so capturing the four alone would let a log carrying a foreign wiring issue AND a ratchet failure drop the whole gate. Sub-spec 6 (`plan/spec-verify-scope-6-wiring-docs-attribution.md`) made the PRODUCER declare its groups instead: `declare_failure_group` prints one group per failure, with `PATH_BEARING_KIND = "files"` when the failure names paths and `subcheck` when it judges a population, so the ci-sleep ratchet and each delegated target still charge the committing session while a wiring red names its files.
- A structural red is attributed only where its group's `kind` is one `PATH_BEARING_GROUP_KINDS` (`internal/le/commit/prepare.go`) holds. A producer that adds a kind gets no attribution until that set is taught to read it, so its reds are charged rather than dropped. That is the deliberate direction, and it costs a producer one line in the allowlist.
- The moved-path record compares the run's START and END snapshots, so an edit that begins and ends between them is invisible in both its shapes, and no acceptance criterion covers either. `docs/architecture/testing/verify-freshness-scope.md` names them, and `TestWriteVerifyStatusRecordsMovedPathsNotSentinel` (`internal/le/verify/engine/verifyengine_test.go`) drives the second. The whole-tree hash has the identical hole, and closing either needs a third observation of the tree.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
