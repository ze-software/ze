# Spec: verify-scope-0-umbrella

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/verify-scope.md` |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`make ze-precommit-verify` takes 74 minutes and certifies nothing. Two separate
faults produce that result, and each sub-spec fixes one of them.

**Fault 1: the gate judges a tree no session owns.** `computeTreeHash`
(`scripts/status/verify_run.go`) hashes `HEAD`, the whole `git diff HEAD`, and
every untracked file. Six sessions share this checkout and it carries 248
uncommitted files. `writeVerifyStatus` writes `treeMovedSentinel` when the tree
moved during the run, and no tree can hash to that value, so the record reports
STALE for ever. The live `tmp/ze-verify.status` holds
`tree_hash=tree-moved-during-run` after a 79 minute run.

The consequence is a ratchet. `commit_helper.py create` refuses a commit
without a FRESH green record, each override writes a row into
`plan/verification-debt/`, and `open_debt_rows` refuses `--push` while one row
is open. The ledger holds 232 open rows and 22 cleared rows, and no script in
the repository writes `cleared`. Every open row states the same reason: the red
belongs to another session.

`plan/journal/concurrent-rfc-gate-stale.md` records nine occurrences of this
class since 2026-07-30.

**Fault 2: the gate repeats work the change cannot affect.** SSH compiles out
of the binary, so an SSH change cannot alter BGP behavior, yet every stage runs
over the whole tree. The taxonomy that describes the blast radius already
exists and no stage reads it: `feature-gates.txt` holds 142 package rows over
36 build tags, one of which is `ze_ssh internal/component/ssh`.

Measured stage costs, derived from stage-log timestamps under `tmp/verify/`:

| Stage | Full run | Changed run |
|-------|----------|-------------|
| `ze-functional-test` | 1472s | 1998s |
| `ze-staticcheck-feature-matrix-check` | 874s | 1138s |
| `ze-unit-test-race-changed` | 638s | - |
| `ze-unit-test-cached` | 439s | - |
| `ze-unit-test-changed` | - | 1163s |
| 24 other stages | 995s | 461s |
| **Total** | **4418s** | **4760s** |

`ze-precommit-verify-changed` is slower than the full run, because
`scripts/dev/changed-pkgs.sh` expands the changed set by the transitive `.Deps`
closure and builds that closure with no build tags. 38 of 646 packages carry
200 or more transitive importers, so one edit under `internal/core/` selects a
third of the tree. In the same graph `internal/component/ssh` carries zero
importers, because its importers sit behind `//go:build ze_ssh`. The selector
over-selects and under-selects at once.

**Goal.** A session that edits one feature gets a verdict about that feature,
in minutes, and that verdict survives another session's concurrent edit.

## Sub-Specs

| # | Spec | Fixes | Depends |
|---|------|-------|---------|
| 1 | `plan/spec-verify-scope-1-shared-checkout-freshness.md` | Fault 1: scoped freshness, per-path move detection, debt that clears | - |
| 2 | `plan/spec-verify-scope-2-change-set-selector.md` | Fault 2 foundation: one tag-aware change-set selector | - |
| 3 | `plan/spec-verify-scope-3-selector-consumers.md` | Fault 2 consumers: staticcheck rows, functional suites | 2 |
| 4 | `plan/spec-verify-scope-4-suite-budget-and-ci.md` | The `plugin` suite wall-clock cap, and CI sharding | - |
| 5 | `plan/spec-verify-scope-5-suite-coverage-map.md` | Functional suite selection, from a RECORDED package-to-suite map. **STOPPED at phase 1 by its own gate** | 2 |

**Sub-spec 5 stopped, and its reason generalises beyond it.** Phase 1 was a gate:
measure whether a suite's EXECUTED package set is small enough to select on, and
stop if it is not. Over a full instrumented functional run the intersection
across the 20 suites that record anything is **423 packages of 646**, union 534.
A change in any of those 423 selects every suite, so the map can never exclude
more than 112.

The cause is Ze's own architecture rather than the method. `init()` in each
`register.go` runs on every process start -- 332 of 342 `register.go` files
carry one, and the generated composition root holds 302 blank imports whose only
purpose is to run them -- so a package counts as executed whatever the command
does. `ze show version` alone records 425 packages, 242 of them covered only
inside `register.go`. Three unrelated commands recorded 426, 426 and 424, union
428.

**The registration pattern that makes Ze pluggable is what makes coverage-based
suite attribution impossible.** Any later attempt to answer "which code does this
test exercise" must discount registration, or work finer than the package.

Two further findings, either of which would have blocked it alone. Four suites
record NOTHING: `editor` runs inside the `ze-test` harness, `web` writes a meta
file and no counters, `runner` tests the harness, and `policy` skips
unprivileged. And the instrumented binary is not behaviourally equivalent under
load: back to back on one tree, `plugin` gave 628/628 clean against 626/628
instrumented, and `ui` 184/184 against 181/184 and 177/184, every failure
`daemon did not become ready`. Instrumentation costs +45% suite time, +52% wall.

So `ze-functional-test` stays unscoped, and AC-U1's under-15-minute target is
not reachable by any route this umbrella found.
| 6 | `plan/spec-verify-scope-6-wiring-docs-attribution.md` | Per-failure groups for `ze-doc-wiring-check`, so attribution reaches the ledger's largest class | 1 |

**Sub-spec 5 exists because every static route to a suite map was measured and
failed** (owner approval, 2026-08-19). `go list -deps ./cmd/ze` links 562 of 646
packages, so by imports every suite exercises 87% of the module; the
`exec=go test` idiom reaches 4.1% of the `.ci` corpus and none of the four
expensive suites; filename prefixes answer only run-`plugin`-or-not, since all
665 of them sit in one suite; and one of the nine suite-name matches is false.
Sub-spec 3's functional half is therefore closed as not statically derivable,
and its staticcheck half stands on its own.

## Owner Decisions

| Date | Decision |
|------|----------|
| 2026-08-19 | A worktree per session is REFUSED. Sessions edit the same files too often and the merges cost more than the isolation saves. Every fix here must work in one shared checkout |
| 2026-08-19 | A central checking daemon is REFUSED. The owner proposed it previously and withdrew it |
| 2026-08-19 | Running the 38 staticcheck matrix rows as parallel processes is REFUSED. The box is partitioned deliberately (`GO_TEST_PROCS` = cores/4, `ZE_RUN_SLOTS` = cores/`GO_TEST_PROCS`) so concurrent sessions coexist, and a 38-way fan-out starves the other five. Sub-spec 3 scopes the ROW COUNT instead |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/tracked-build-gate.md` - the one gate that reads the committed population rather than the working tree
  → Constraint: a stage that reads the working tree cannot answer for what git holds
- [ ] `docs/functional-tests.md` - the functional runner and its suites
  → Constraint: the stage list lives in `stagesForMode` and nowhere else

**Key insights:**
- The stage list is single-sourced in `stagesForMode` (`scripts/status/verify_run.go`), pinned by `TestStagesForModeMatchesGolden`. A gate absent from that function runs nowhere, in CI or locally.
- `ai/rules/testing.md` derives a `.ci` file's `functional/verify` tier from the literal `all_suites` line in `mk/test-functional.mk`. Any per-change suite skipping must not lower a tagged RFC requirement's tier. Sub-spec 3 owns that obligation.
- The scoped freshness answer is already built. `dirty_manifest` and `manifest_scoped` (`scripts/dev/verify-status.sh`) exist, and their own comment states the reason: "The commit is scoped to a file list; the evidence must be scopeable to the same list, or a session can never hold evidence about its own code". No production caller passes a path.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/status/verify_run.go` - runs 30 stages sequentially; `stageResult` records no duration; `writeVerifyStatus` stamps `treeMovedSentinel` on any concurrent edit
- [ ] `scripts/dev/verify-status.sh` - `tree_hash` covers the whole repository; `manifest_scoped` implements a scoped answer that no production caller uses
- [ ] `scripts/dev/changed-pkgs.sh` - transitive, untagged reverse-dependency expansion
- [ ] `scripts/checks/staticcheck_feature_matrix.go` - `judgeStaticcheckFeatureMatrix` spawns one `staticcheck -matrix ./...`; `deriveFeatureMatrix` emits 38 rows
- [ ] `mk/test-functional.mk` - 24 suites in a sequential shell loop; `ZE_SKIP_SUITES` is the only scoping knob
- [ ] `scripts/dev/commit_helper.py` - `verify_status` runs `verify-status.sh check` with no path arguments; `STRUCTURAL_GATES` names the reds that `--unverified` cannot wave through; `record_debt` writes rows and nothing clears them

**Behavior to preserve:**
- The stage list stays single-sourced in `stagesForMode`.
- A gate that a change CAN affect still runs. Scoping removes repetition, never coverage.
- The verify record keeps its file path and field names: `verify-status.sh` and `commit_helper.py` both read them.

**Behavior to change:** named per sub-spec.

## Data Flow (MANDATORY)

### Entry Point
- A session edits files, then runs `make ze-precommit-verify` or `make ze-precommit-verify-changed`.

### Transformation Path
1. `verify-lock.sh` (an alias for `ze-run.sh`) admits the job into a slot.
2. `verify_run.go` reads `stagesForMode`, runs each stage in order, and writes `tmp/ze-verify.status`.
3. `verify-status.sh check` compares the recorded tree hash against the live one.
4. `commit_helper.py create` reads that verdict and the failure index, then allows or refuses the commit.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Verify runner ↔ commit helper | `tmp/ze-verify.status`, `tmp/ze-verify-failures.json` | No |
| Verify runner ↔ make stages | sub-make invocation per stage | No |
| Selector ↔ stages | none today; sub-spec 2 creates it | No |

### Integration Points
- `stagesForMode` - the only live stage list.
- `feature-gates.txt` - the package-to-tag manifest, parsed today by `plugin_imports.go` and `dep_audit.py`.

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
| A-1 | Scoping which gates run cannot hide a defect, because a gate a change cannot affect returns the same verdict either way | `feature-gates.txt` plus the import graph decide reachability | A scoped-out gate hides a real red | Sub-spec 2 ships a selector self-test that proves the excluded set is unreachable | unvalidated |
| A-2 | The owner accepts a per-change verdict as the merge gate, with the whole-tree sweep moved to CI and cadence | Owner instruction 2026-08-19 | The local gate must stay whole-tree and only Fault 1 is fixable | Owner confirmation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A scoped selector drops a suite that the change could reach, and a defect lands | A red in CI that the local gate reported green | The selector fails OPEN: an unclassified path selects everything, and the selector logs what it dropped |
| R-2 | Per-change suite skipping lowers a tagged RFC requirement's derived tier | `make ze-rfc-check` reports a tier change | Sub-spec 3 makes the tier derivation read the selector, not the static suite list |
| R-3 | The debt ledger is cleared by a script that hides a real unrun gate | A cleared row for a gate that never ran | Clearing re-runs the owed gate over the committed code and records its exit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The merge gate reports green on an unverified change. No runtime behavior changes: every file here is build and test tooling |
| How is it reverted? | Single commit revert per sub-spec. The stage list is one function |
| Who else touches this path? | Every session in this checkout. `commit_helper.py` and `verify-status.sh` are read by all of them |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-precommit-verify-list` | → | `stagesForMode` | `TestStagesForModeMatchesGolden` |
| `scripts/dev/commit_helper.py create` | → | `verify_status` with the commit's file list | `test_verify_status_scopes_to_commit_paths` |
| `make ze-verify-scope-selector` | → | the sub-spec 2 selector | `TestSelectorFailsOpenOnUnknownPath` |
| `make ze-verify-debt-clear` | → | the sub-spec 1 clearing target | `test_debt_clear_reruns_the_owed_gate` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-U1 | Sub-specs 1 to 4 are closed | `make ze-precommit-verify-changed` over a single-feature change completes in under 15 minutes on the dev box |
| AC-U2 | Another session edits an unrelated file while a verify run is in flight | The run still produces a usable verdict for the paths it judged |
| AC-U3 | A gate is red because of a file outside the committing session's change set | `commit_helper.py create` does not charge that session a debt row for it |
| AC-U4 | The debt ledger holds open rows for gates that now pass | A make target clears them, and it re-runs each owed gate to do so |
| AC-U5 | A change touches only `internal/component/ssh` | No functional suite runs that the selector cannot reach from that package, and the staticcheck matrix judges at most 4 rows |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStagesForModeMatchesGolden` | `scripts/status/verify_run_test.go` | The stage list stays single-sourced across every sub-spec edit | |
| per sub-spec | per sub-spec | per sub-spec | |

### Functional Tests
<!-- test/runner/ is where the test tooling proves itself end to end
     (stop-background.ci is its existing member). A `.ci` there runs the real
     scripts through `exec=` and asserts their output, which a Go unit test on
     the same functions cannot do: the selector's product is what the shell
     pipeline prints to a make recipe, not what a function returns. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-scope-selector` | `test/runner/verify-scope-selector.ci` | A developer edits one SSH file and asks which suites and matrix rows apply; the selector names them and names nothing else | |
| `verify-scope-freshness-scoped` | `test/runner/verify-scope-freshness-scoped.ci` | A developer's verify record stays FRESH for their own paths while a second writer edits an unrelated file | |
| `verify-scope-debt-clear` | `test/runner/verify-scope-debt-clear.ci` | A developer clears a debt row, and the clearing re-runs the owed gate rather than trusting the edit | |

## Files to Modify
- `scripts/status/verify_run.go` - freshness record, stage durations, stage selection
- `scripts/dev/verify-status.sh` - scoped freshness callers
- `scripts/dev/changed-pkgs.sh` - reduced to a dispatcher: it reads the answer the run published, or runs the selector. The transitive untagged expansion is deleted rather than wrapped, and the tag-aware logic lives in `runSelector` (`scripts/checks/verify_scope_selector.go`)
- `scripts/dev/commit_helper.py` - refusal attribution, debt clearing
- `scripts/checks/staticcheck_feature_matrix.go` - row scoping
- `mk/test-functional.mk` - suite selection and the suite wall-clock cap
- `.github/workflows/verify.yml` - sharding

## Files to Create
- per sub-spec

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Build and test tooling only, no runtime config |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | No `ze` subcommand changes; make targets only |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No CLI output added |
| Env var registration | N-A | No `ze.*` leaf added. Make-level variables are not `env.MustRegister()` names |
| Doctor check for runtime dependencies | N-A | No new runtime path, socket, port, module, or binary |
| Prometheus counters/metrics | N-A | No daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling, not a product feature |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `ai/rules/testing.md` tier derivation, if sub-spec 3 changes how a suite earns `functional/verify` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/` gains the selector's contract |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming the seven files above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/git-safety.md` and `ai/rules/precommit-verify.md` describe the commit path this changes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- each sub-spec starts with its own wiring phase, and the rows in the Wiring Test table above are its targets
   - Tests: the four rows in the Wiring Test table
   - Files: `scripts/dev/commit_helper.py`, `scripts/status/verify_run.go`, and the two new make targets
   - Verify: each wiring test fails because the target it names does not exist yet
2. **Phase: Sub-spec 1** -- shared-checkout freshness and debt
3. **Phase: Sub-spec 2** -- the change-set selector
4. **Phase: Sub-spec 4** -- suite wall-clock budget and CI sharding, independent of 2
5. **Phase: Sub-spec 3** -- the selector's consumers, after 2 closes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-U row maps to a sub-spec AC |
| Feature completeness | No sub-spec leaves a stage both unscoped and unmeasured |
| Correctness | The selector fails open, and its self-test proves the excluded set unreachable |
| Naming | One name for the change set, used by every consumer |
| Data flow | The selector is computed once per run and read by stages, never recomputed per stage |
| Rule: `ai/rules/testing.md` | A derived `functional/verify` tier still means the suite runs when its subject changes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four closed sub-specs | `ls plan/spec-verify-scope-*` returns only this umbrella |
| A measured run under 15 minutes | `tail tmp/.ze-verify-duration.txt` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The selector reads git output and `feature-gates.txt`. A path it cannot classify must select everything, never nothing |

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

- The blast-radius map is already generated and maintained. `feature-gates.txt` carries 142 package rows over 36 tags, `plugin_imports.go` turns it into 37 `all_ze_*.go` files, and `dep_audit.py` builds a first-party reverse import graph on every run and discards it. Three components parse the same facts in three languages and none of them answers "what can this change affect".
- The scoping knobs also exist at both layers: `ZE_SKIP_SUITES` in make, and `runner.Selection` with `--pattern` in the test runner. The missing piece is the map, not the mechanism.
- The scoped freshness manifest was written for exactly this problem, with a comment naming it, and then never wired to a caller. Sub-spec 1 is mostly the missing call.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix the shared checkout's gate rather than isolate sessions | A git worktree per session | Owner decision 2026-08-19: sessions edit the same files too often, and the merges cost more than the isolation saves |
| Scope the staticcheck matrix by row count | Run the 38 rows as parallel processes | Owner decision 2026-08-19: the box is partitioned so concurrent sessions coexist, and a 38-way fan-out starves them |
| One selector, computed once, read by every stage | Per-stage change detection | A second copy drifts. `stagesForMode` already proved that: two duplicate Makefile stage lists drifted for an unknown period before `spec-fixit-verify-stage-ssot.md` deleted them |

## Known Limitations
- The whole-tree sweep still has to happen somewhere. This umbrella moves it to CI and to the cadence targets, and does not delete it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
