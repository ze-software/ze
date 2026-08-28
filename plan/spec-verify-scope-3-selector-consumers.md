# Spec: verify-scope-3-selector-consumers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-verify-scope-2-change-set-selector.md` |
| Phase | 3/3 -- the staticcheck half is built, green, documented and review-fixed. The approved phases 3 and 4 (the suite map and the tier derivation) moved to `plan/spec-verify-scope-5-suite-coverage-map.md`, because no static signal attributes a `.ci` file to a Go package |
| Deferral shard | `plan/deferrals/verify-scope.md` | <!-- doc-links: ignore (artifact a later phase of this spec will create) -->
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make the staticcheck matrix read the sub-spec 2 selector, so it judges only the
build combinations the change can move.

**The functional half of this spec MOVED, and this spec no longer builds it.**
It was scoped as the second consumer, and the measurement that ended it is
recorded in `plan/spec-verify-scope-5-suite-coverage-map.md`: no static signal
attributes a `.ci` file to a Go package. `go list -deps ./cmd/ze` links 562 of
the module's 646 packages, so an import-graph map makes every suite "exercise"
almost everything, and the four other candidates reach 4.1% of the corpus or
less. Deriving the map at RUN TIME is a different design carrying its own risks,
so sub-spec 5 holds the suite selection, its fail-open branches, and the tier
derivation. Every AC, assumption, test and file below that named a suite says
where it went.

**The staticcheck matrix: 38 rows, one process, 874s.** `deriveFeatureMatrix`
(`internal/le/staticcheckmatrix/staticcheckmatrix.go`) emits `all_features`,
`core_only`, and one `without_ze_<tag>` row per tag, 38 in total, verified by
`./le staticcheck-feature-matrix check --print-matrix`.
`judgeStaticcheckFeatureMatrix` spawns ONE `staticcheck -checks=-all -matrix
./...` and feeds the rows on stdin. Upstream runs build configs serially, so the
874s is 38 sequential full-module analyses with a warm cache.

The owner REFUSED parallelising those rows (umbrella, Owner Decisions,
2026-08-19): the box is partitioned so six sessions coexist, and a 38-way
fan-out starves five of them. The answer is to judge fewer rows. A change
confined to packages gated by tag T can only alter the verdict of the rows whose
tag set differs with respect to T, plus any row removing a tag that gates a
package in the change's import closure. For an SSH-only change that is
`all_features`, `core_only` and `without_ze_ssh`.

**The obligation the moved half carries, and why it is not this spec's.**
`ai/rules/testing.md` derives a `.ci` file's `functional/verify` tier from the
`all_suites` line, read by `functional_suites()`
(`internal/le/rfc/rfc.go`). Skipping a suite per change would lower
that tier and fire `check_evidence_ratchet`. No suite is skipped by this spec,
so nothing here touches the derivation, and sub-spec 5 owns it as its AC-7.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` - the four carriers, and how a `.ci` earns `functional/verify`
  → Constraint: non-unit evidence is monotonic per requirement and per tier; no annotation satisfies `check_evidence_ratchet`
- [ ] `ai/rules/rfc-compliance.md` - the eight ratchets
  → Constraint: proof is monotonic. A requirement may not lose a polarity or a tier it held at HEAD
- [ ] `docs/functional-tests.md` - the suites and the runner

**Key insights:**
- The tier is DERIVED, never declared. That is what would make per-change suite selection safe, and it is now sub-spec 5's to use.
- A row filter must SUBTRACT from the derived rows. An allowlist of row names stops covering a tag the day one is added to `feature-gates.txt`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/staticcheckmatrix/staticcheckmatrix.go` - `deriveFeatureMatrix`, `matrixRowsForTags`, `judgeStaticcheckFeatureMatrix`
- [ ] `internal/le/changed/selector.go` - `reachedTags`, `tagForPackage`, `emit`
- [ ] `internal/le/verify/run.go` - `execStage`, `selectChangeSet`, and the scope environment it exports
- [ ] `internal/le/functional/suites.go` - `all_suites`, `run_suite`, `ZE_SKIP_SUITES`, `SUITE_RUN` (read while the functional half was in scope; it moved to sub-spec 5 unchanged)
- [ ] `internal/le/rfc/rfc.go` - `functional_suites`, `_suite_carriers`, `check_evidence_ratchet` (same; no edit is made here)

**Behavior to preserve:**
- `all_suites` stays the single source of truth for which suites are gating, and this spec does not read or rewrite it.
- Every `.ci` file that runs today still runs, in every run.
- The matrix keeps `all_features` and `core_only` in every run: those two judge the shipped combinations.

**Behavior to change:**
- The matrix judges a subset of rows, chosen by the selector's feature-tag answer.

## Data Flow (MANDATORY)

### Entry Point
- A verify run starts, and the sub-spec 2 selector has already written its answer.

### Transformation Path
1. The run writes the feature-tag answer once (`selectChangeSet`) and names it to every stage in `ZE_VERIFY_SCOPE_TAGS`.
2. The matrix check reads that answer and emits only the rows the answer can move.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Selector ↔ verify runner | `--print=both`, one run, two answers | Yes -- `TestSelectScopePackagesRunsTheRealSelector` |
| Verify runner ↔ matrix check | the feature-tag answer file named by `ZE_VERIFY_SCOPE_TAGS` | Yes -- `TestVerifyRunNamesTheFeatureScopeToEveryStage`, `TestTheStaticcheckMatrixReadsTheFeatureScopeVariable`, `TestMatrixRowsScopeToChangedTags` |

### Integration Points
- `deriveFeatureMatrix` - gains a row filter, keeps its manifest source.
- `reachedTags` - unions the tags a changed file NEGATES, so the only row that compiles such a file survives the filter.
- `execStage` - exports the answer to every stage of the run.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The matrix reads the answer file named by `ZE_VERIFY_SCOPE_TAGS` and never runs the selector itself; `selectChangeSet` is the only producer |
| No unintended coupling (components stay isolated) | Yes | Two `//go:build ignore` programs share one FILE FORMAT, one tag per line, and no symbol. `scopeTagsEnvName` in the test states that the string is the contract |
| No duplicated functionality (extends existing, does not recreate) | Yes | `matrixRowsForTags` still derives the rows; `scopeFeatureMatrix` subtracts from what it returns and builds no second row list |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Build tooling, off any data path. The answer file is a few dozen bytes read once per stage |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No row, tag or feature is named anywhere. `feature-gates.txt` stays the one inventory, and a tag added there gains its row and its scoping with no second edit |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `without_ze_X` row's verdict is unchanged by a change to a package that neither is gated by X nor imports one that is, and that negates X in no file | The row differs from `all_features` only by X's packages, and a file constrained `!ze_X` is compiled by that row alone | A skipped row hides a type error | A self-test that introduces a break only one row compiles, and drives the matrix from the answer the selector really gives | confirmed, and the second clause was ADDED by review: `TestMatrixRowFilterCatchesAGatedBreak` (`internal/le/repository/`) builds a module whose only type error compiles under `ze_web && !ze_ssh`. Scoped to `ze_ssh` the matrix exits 1, and the answer the selector produces for that changed FILE names `ze_ssh` as well as `ze_web`, so the row survives. `reachedTags` unioning the negated tags is what makes the assumption hold |

A-2 and A-3 were about the package-to-suite map and about the tier derivation
reading it. Both moved with that work to
`plan/spec-verify-scope-5-suite-coverage-map.md`, which carries them as its own
A-1 to A-3 and its AC-7. Nothing in this spec reads or writes a suite.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A subtracted row would have caught the change | CI red on a locally green change | Every doubt widens (`readChangeScope`), and the tag answer unions what a changed file negates. R-1's suite half moved to sub-spec 5 |
| R-2 | The matrix row filter is written as an allowlist and rots | A new tag is added and no row covers it | The filter subtracts from the derived 38, and never enumerates rows by name |
| R-3 | A test in `./scripts/checks` inherits the run's own scope answer and judges the machine | The package is green in a shell and red inside `./le verify current mode full` | `overriddenEnvironment` subtracts every `ZE_VERIFY_` variable from each child, keyed on the prefix so a variable added to `execStage` later is covered |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A change is under-analysed or under-tested. Nothing at runtime |
| How is it reverted? | Single commit revert; the matrix falls back to judging every row |
| Who else touches this path? | Every stage of a verify run inherits `ZE_VERIFY_SCOPE_TAGS`, so a test starting a child process must subtract it. No `.ci` file and no RFC requirement changes tier: the ledger is untouched |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le staticcheck-feature-matrix check` with a scoped selector answer | → | `deriveFeatureMatrix` row filter | `TestMatrixRowsScopeToChangedTags` |
| `./le verify current mode full` running the matrix stage | → | `execStage` exporting `ZE_VERIFY_SCOPE_TAGS` | `TestVerifyRunNamesTheFeatureScopeToEveryStage` |
| A changed file constrained `!ze_X` | → | `reachedTags` unioning the negated tag | `TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates` |

The two rows this table held for the functional stage moved to
`plan/spec-verify-scope-5-suite-coverage-map.md` with the work: the computed
`ZE_SKIP_SUITES` and `functional_suites()` reading the map.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The changed set is `internal/component/ssh/ssh.go` | The matrix judges at most 4 rows, and `all_features` and `core_only` are among them |
| AC-2 | The changed set touches an always-on package | The matrix judges all 38 rows |
| AC-3 | A type error is introduced that only `without_ze_ssh` catches, in an SSH-gated package | The scoped matrix still catches it |
| AC-4 | A changed file is constrained `!ze_X`, in a package gated by another tag | The answer names `ze_X` too, so the one row that compiles the file is judged |
| AC-5 | A test in `./scripts/checks` runs inside a scoped verify run | It judges its fixture, not the run: no `ZE_VERIFY_` variable reaches a child process it starts |

AC-4 to AC-7 of the approved spec were the functional stage, the fail-open
branch, and the tier derivation. All three moved to
`plan/spec-verify-scope-5-suite-coverage-map.md` (its AC-3 to AC-7). The two ACs
above are new, and both come from the review of the implementation: the negated
constraint and the inherited environment.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMatrixRowsScopeToChangedTags` | `internal/le/repository/` | AC-1, AC-2: the row filter subtracts correctly | pass |
| `TestMatrixRowFilterCatchesAGatedBreak` | `internal/le/repository/` | AC-3, AC-4: the retained rows still catch a real break, driven by the selector's own answer | pass |
| `TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates` | `internal/le/changed/selector_test.go` | AC-4: a negated tag joins the answer, and an unreadable changed file widens | pass |
| `TestMatrixTestsJudgeTheFixtureNotTheRunThatStartedThem` | `internal/le/repository/` | AC-5: no `ZE_VERIFY_` variable reaches a child | pass |
| `TestVerifyRunNamesTheFeatureScopeToEveryStage` | `internal/le/verify/verify_test.go` | the runner publishes the answer this spec's consumer reads | pass |

`TestFunctionalSuitesScopeToChangedPackages` and
`test_functional_tier_reads_the_suite_map` moved to
`plan/spec-verify-scope-5-suite-coverage-map.md`, which names them
`TestSuiteSelectionSkipsOnlyUnreachedSuites` and
`test_functional_tier_is_unchanged_by_selection`.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| matrix rows judged | 2-38 | 38 | 1 (drops `core_only` or `all_features`) | N/A |

<!-- The matrix has a real low boundary: `all_features` and `core_only` judge the
     shipped combinations and must never be filtered out, so 2 is the floor and
     1 must be refused. The suites-run row moved to sub-spec 5 with the stage. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | - | No `.ci` can drive this: the subject is which build combinations a verify STAGE judges, and the functional runner starts no verify run. `verify-scope-suite-selection.ci` moved with the stage it tested, and `plan/spec-verify-scope-5-suite-coverage-map.md` carries it as `test/runner/verify-scope-suite-map.ci` | <!-- doc-links: ignore (artifact a later phase of this spec will create) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling. No wire-visible behavior changes. The RFC ledger is regenerated, not re-judged | |

## Files to Modify
- `internal/le/verify/run.go` - the largest edit: run the selector once, write both answers, and name the tag answer to every stage (`selectChangeSet`, `execStage`)
- `internal/le/staticcheckmatrix/staticcheckmatrix.go` - the row filter and its floor
- `internal/le/changed/selector.go` - `reachedTags` unions the tags a changed file negates
- `internal/le/repository/`, `internal/le/changed/selector_test.go`, `internal/le/verify/verify_test.go` - the tests for all three
- `ai/INDEX.md`, `docs/functional-tests.md`, `docs/contributing/testing.md`, `docs/architecture/testing/tracked-build-gate.md`, `docs/architecture/testing/verify-freshness-scope.md` - the row count is no longer unconditional

The suite half's files moved to `plan/spec-verify-scope-5-suite-coverage-map.md`:
`internal/le/functional/suites.go`, `internal/le/rfc/rfc.go`, `ai/rules/testing.md`
and the regenerated ledger. `FUNCTIONAL_SUITE_BY_AREA`
(`internal/le/docwiring/wiring.go`) stays exactly as it is, four advisory
entries used by no selection: superseding it needs the derived map, which is
sub-spec 5's deliverable, and no spec supersedes it before that map exists.

## Files to Create
- None. The suite map (`internal/le/changed/scope.go`, its test, and the `.ci`) is created by `plan/spec-verify-scope-5-suite-coverage-map.md` <!-- doc-links: ignore (artifact a later phase of this spec will create) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Build and test tooling |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | Make targets only |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No `ze` CLI output added |
| Env var registration | N-A | `ZE_VERIFY_SCOPE_TAGS` is a build-tooling variable one process writes and one stage reads, not a `ze.*` leaf |
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
| 9 | RFC behavior implemented, changed, or newly proven? | No | No suite is skipped by this spec, so no `.ci` loses its `functional/verify` tier and no `rfc/requirements/` shard changes. The tier derivation moved with the suite map to `plan/spec-verify-scope-5-suite-coverage-map.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, under Release Gate Coverage: a scoped run judges fewer Staticcheck matrix rows, and suite selection is NOT scoped |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md`, "Which matrix rows a scoped run judges": the filter, the 2-row floor, the four inputs that widen, and where the run publishes the tag answer |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Two pages anchored `staticcheck_feature_matrix.go` and stated the N+2 row count unconditionally: `docs/contributing/testing.md` and `docs/architecture/testing/tracked-build-gate.md`. Each now says a verify run scopes the rows and links the contract page. New anchors added to `docs/functional-tests.md` and `docs/architecture/testing/verify-freshness-scope.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ZE_VERIFY_SCOPE_TAGS` is documented in `ai/INDEX.md`, `ai/rules/commands.md` and `docs/architecture/testing/verify-freshness-scope.md`. `ZE_SKIP_SUITES` is unchanged by this spec |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the run publishes the answer and the matrix reads it
   - Tests: `TestMatrixRowsScopeToChangedTags`, `TestVerifyRunNamesTheFeatureScopeToEveryStage`
   - Files: `internal/le/verify/run.go`, `internal/le/staticcheckmatrix/staticcheckmatrix.go`
   - Verify: the answer reaches the stage, and the matrix judges every row while it is discarded
2. **Phase: Matrix row filter** -- subtract the rows the change cannot move
   - Tests: `TestMatrixRowsScopeToChangedTags`, `TestMatrixRowFilterCatchesAGatedBreak`
   - Files: `internal/le/staticcheckmatrix/staticcheckmatrix.go`
   - Verify: AC-1, AC-2, AC-3 hold, and the floor of 2 rows is enforced
3. **Phase: Review fixes** -- close the two holes the review found in phase 2
   - Tests: `TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates`, `TestMatrixTestsJudgeTheFixtureNotTheRunThatStartedThem`
   - Files: `internal/le/changed/selector.go`, both test files
   - Verify: AC-4 and AC-5 hold, and each fix is red when it is reverted

The suite map and the tier derivation were phases 3 and 4 of the approved spec.
Both moved to `plan/spec-verify-scope-5-suite-coverage-map.md`, whose phase 1
measures the assumption they rested on before anything is built.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | The matrix reads the selector's answer and keeps no private copy of the manifest |
| Correctness | The matrix floor holds: `all_features` and `core_only` are never filtered out |
| Correctness | A row is subtracted only when NO changed file compiles in it. A file constrained `!ze_X` compiles in `without_ze_X` alone |
| Data flow | One producer writes the tag answer, and the consumer reads that file. A test that starts a child inherits it, so the test subtracts it |
| Rule: `ai/rules/evidence.md` | Every widening states its reason on stderr, and no doubt returns a narrow answer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The matrix scopes | `./le staticcheck-feature-matrix check --print-matrix` under a scoped answer |
| Every widening states its reason | the same command with an unreadable, over-wide, or undeclared answer prints the reason on stderr and judges 38 rows |
| The scoping is sound for a negated constraint | `TestMatrixRowFilterCatchesAGatedBreak` drives the matrix from the selector's own answer and still catches the break |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The tag answer is a file path from the environment and its lines become build tags. A line the manifest does not declare must widen rather than reach a `-tags` list, and `matrixNamePattern` / `featureTagPattern` still bound what a row can be called |

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

- Which rows a change can move is decided by TWO facts about the changed file, not one. The tag that GATES its package says which row drops it; the tags it NEGATES say which rows are the only ones that compile it. An answer built from the first alone subtracts the row that a `!ze_X` file's break lives in, and the fixture in `TestMatrixRowFilterCatchesAGatedBreak` is exactly that file.
- A test that starts a child process inherits the run that started IT. Inside `./le verify current mode full` the `./scripts/checks` unit tests execute with the run's own `ZE_VERIFY_SCOPE_TAGS` set, so a test that does not subtract it judges the machine. This is the second instance of the shape in one session (`ZE_VERIFY_MODE` did it to `internal/le/`), which is why the subtraction keys on the `ZE_VERIFY_` prefix rather than on the names known today.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Scope the matrix by row count | Parallelise the 38 rows | Owner decision 2026-08-19: the box is partitioned so concurrent sessions coexist |
| Subtract rows from the derived 38 | Enumerate the rows to run | An allowlist rots when a tag is added. Subtraction inherits every new tag automatically |
| Union the tags a changed file NEGATES into the answer | Read the package's manifest gate alone | A file constrained `!ze_X` compiles in `without_ze_X` and in no other row, so the gate alone subtracts the only row that can see it |
| A changed Go file that cannot be read widens to every feature | Skip it, and answer from the files that remain | Its constraint is what cannot be known, and a deleted `!ze_X` file breaks `without_ze_X` alone. A guard that cannot read its input must not answer narrow (`ai/rules/evidence.md`) |
| The test-side subtraction keys on the `ZE_VERIFY_` prefix | Name the two scope variables | `execStage` exports that whole family, and the pair known today is not the pair a later spec adds |

## Known Limitations
- The row filter is coarser than the package graph: a row is judged over the WHOLE module or not at all. Staticcheck's `-matrix` takes one package pattern per run, so judging a row over a subset would be a second invocation shape, and this spec does not add one.
- A commit that DELETES a Go file widens the tag answer to every feature, because the deleted file's build constraint is what cannot be read. That is the safe direction and it is the whole cost: a change set holding no deletion is unaffected.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/speclifecycle/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
