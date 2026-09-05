# Spec: verify-scope-3-selector-consumers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | spec-verify-scope-2-change-set-selector (closed 2026-09-05; the selector is `internal/le/changed/selector.go`, and the run publishes its answers through `publishChangeScope`, `internal/le/verify/engine/scope.go`) |
| Phase | 3/3 -- the staticcheck half is built, green, documented and review-fixed. The approved phases 3 and 4 (the suite map and the tier derivation) moved to `plan/spec-verify-scope-5-suite-coverage-map.md`, because no static signal attributes a `.ci` file to a Go package |
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
(`internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go`) emits `all_features`,
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
- [ ] `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go` - `deriveFeatureMatrix`, `matrixRowsForTags`, `judgeStaticcheckFeatureMatrix`
- [ ] `internal/le/changed/selector.go` - `reachedTags`, `tagForPackage`, `emit`
- [ ] `internal/le/verify/engine/run.go` and `scope.go` - `runMode`, `publishChangeScope`, `selectChangeSet`, and the scope environment they name
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
1. The run writes the feature-tag answer once (`selectChangeSet`, `internal/le/verify/engine/scope.go`) and names it to every stage in `ZE_VERIFY_SCOPE_TAGS`.
2. The matrix check reads that answer and emits only the rows the answer can move.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Selector ↔ verify runner | `--print=both`, one run, two answers | Yes -- `selectChangeSet` (`internal/le/verify/engine/scope.go`) calls the production selector, proved by `TestVerifyRunPublishesTheScopedAnswerAGatedChangeProduces` |
| Verify runner ↔ matrix check | the feature-tag answer file named by `ZE_VERIFY_SCOPE_TAGS` | Yes -- `TestVerifyRunNamesTheFeatureScopeToEveryStage`, `TestDeriveReadsTheAnswerTheRunPublished`, `TestTheSubtractionNeverDropsAShippedCombination` |

### Integration Points
- `deriveFeatureMatrix` - gains a row filter, keeps its manifest source.
- `reachedTags` - unions the tags a changed file NEGATES, so the only row that compiles such a file survives the filter.
- `publishChangeScope` (`internal/le/verify/engine/scope.go`) - names the answer to every stage of the run.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The matrix reads the answer file named by `ZE_VERIFY_SCOPE_TAGS` and never runs the selector itself; `selectChangeSet` (`internal/le/verify/engine/scope.go`) is the only producer |
| No unintended coupling (components stay isolated) | Yes | Producer and consumer share one FILE FORMAT, one tag per line, and one key name declared once as `changed.ScopeTagsKey`. The matrix imports that name from the package that produces the answer and holds no copy of it |
| No duplicated functionality (extends existing, does not recreate) | Yes | `matrixRowsForTags` still derives the rows; `scopeFeatureMatrix` subtracts from what it returns and builds no second row list |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Build tooling, off any data path. The answer file is a few dozen bytes read once per stage |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No row, tag or feature is named anywhere. `feature-gates.txt` stays the one inventory, and a tag added there gains its row and its scoping with no second edit |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `without_ze_X` row's verdict is unchanged by a change to a package that neither is gated by X nor imports one that is, and that negates X in no file | The row differs from `all_features` only by X's packages, and a file constrained `!ze_X` is compiled by that row alone | A skipped row hides a type error | A self-test that introduces a break only one row compiles, and drives the matrix from the answer the selector really gives | confirmed, and the second clause was ADDED by review: `TestMatrixRowFilterCatchesAGatedBreak` (`internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix_test.go`) builds a module whose only type error compiles under `ze_web && !ze_ssh`. Scoped to `ze_ssh` the matrix exits 1, and the answer the selector produces for that changed FILE names `ze_ssh` as well as `ze_web`, so the row survives. `reachedTags` unioning the negated tags is what makes the assumption hold |

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
| `./le staticcheck-feature-matrix check` with a scoped selector answer | → | `scopeMatrix` row filter | `TestTheSubtractionNeverDropsAShippedCombination`, `TestEveryDoubtJudgesTheWholeMatrix` |
| `./le verify current mode full` running the matrix stage | → | `publishChangeScope` naming `ZE_VERIFY_SCOPE_TAGS` to every stage | `TestVerifyRunNamesTheFeatureScopeToEveryStage`, `TestDeriveReadsTheAnswerTheRunPublished` |
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
| `TestTheSubtractionNeverDropsAShippedCombination`, `TestEveryDoubtJudgesTheWholeMatrix` | `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix_test.go` | AC-1, AC-2: the row filter subtracts correctly, and every doubt widens | PASS |
| `TestMatrixRowFilterCatchesAGatedBreak` | `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix_test.go` | AC-3, AC-4: the retained rows still catch a real break, and the gate-only answer misses it | PASS |
| `TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates` | `internal/le/changed/selector_test.go` | AC-4: a negated tag joins the answer, and an unreadable changed file widens | PASS |
| `TestDeriveReadsTheAnswerTheRunPublished` | `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix_test.go` | AC-5: an answer planted in the environment is visible to `Derive` and invisible to a caller that names its scope | PASS |
| `TestVerifyRunNamesTheFeatureScopeToEveryStage`, `TestVerifyRunWidensWhenTheChangeSetCannotBeSelected` | `internal/le/verify/engine/scope_test.go` | the runner publishes the answer this spec's consumer reads, and a refused selection adds nothing to what the process already held | PASS |

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
- `internal/le/verify/engine/run.go` - the largest edit: run the selector once, write both answers, and name the tag answer to every stage (`selectChangeSet`, `execStage`)
- `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go` - the row filter and its floor
- `internal/le/changed/selector.go` - `reachedTags` unions the tags a changed file negates
- `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix_test.go`, `internal/le/changed/selector_test.go`, `internal/le/verify/engine/scope_test.go` - the tests for all three
- `ai/INDEX.md`, `docs/functional-tests.md`, `docs/contributing/testing.md`, `docs/architecture/testing/tracked-build-gate.md`, `docs/architecture/testing/verify-freshness-scope.md` - the row count is no longer unconditional

The suite half's files moved to `plan/spec-verify-scope-5-suite-coverage-map.md`:
`internal/le/functional/suites.go`, `internal/le/rfc/rfc.go`, `ai/rules/testing.md`
and the regenerated ledger. `FUNCTIONAL_SUITE_BY_AREA`
(`internal/le/doc/wiring/wiring.go`) stays exactly as it is, four advisory
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
   - Tests: `TestDeriveReadsTheAnswerTheRunPublished`, `TestVerifyRunNamesTheFeatureScopeToEveryStage`
   - Files: `internal/le/verify/engine/run.go`, `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go`
   - Verify: the answer reaches the stage, and the matrix judges every row while it is discarded
2. **Phase: Matrix row filter** -- subtract the rows the change cannot move
   - Tests: `TestTheSubtractionNeverDropsAShippedCombination`, `TestMatrixRowFilterCatchesAGatedBreak`
   - Files: `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go`
   - Verify: AC-1, AC-2, AC-3 hold, and the floor of 2 rows is enforced
3. **Phase: Review fixes** -- close the two holes the review found in phase 2
   - Tests: `TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates`, `TestDeriveReadsTheAnswerTheRunPublished`
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

---

## Implementation Summary

### What Was Implemented
- `readChangeScope`, `scopeMatrix`, `validateScoped`, `minMatrixRows` (`internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go`): the row filter that SUBTRACTS from the derived rows, its two-row floor, and the four doubts that widen it back to every row.
- `reachedTags`, `negatedTags`, `tagsUnderNot`, `everyTagIn` (`internal/le/changed/selector.go`): the tag answer unions the features a changed file NEGATES, so the one row that compiles a `!ze_X` file survives the subtraction.
- `DeriveScoped` (matrix): the derivation with the answer named explicitly, which is what lets a test judge its fixture rather than the run that started it.
- The producer half landed at the closure of spec-verify-scope-2-change-set-selector, on the same day and for the same reason: `publishChangeScope` (`internal/le/verify/engine/scope.go`) names `ZE_VERIFY_SCOPE_TAGS` to every stage.

### Bugs Found/Fixed
- **The soundness proof was gone.** `TestMatrixRowFilterCatchesAGatedBreak` is what makes A-1 more than an argument: it builds a module whose only type error compiles under `ze_web && !ze_ssh` and drives the real Staticcheck matrix over the answer the selector gives for that file. It went with the shell it was written against. Restored in `staticcheckfeaturematrix_test.go`, and strengthened: it now also asserts that the GATE-ONLY answer misses the break, so the negation union is proved load-bearing rather than merely present.
- **AC-5's failure mode was reintroduced by the producer fix, and this closure found it.** `TestTheRealManifestDerivesEveryRow` called `Derive(tree)`, which reads the ambient answer. With `publishChangeScope` restored, a verify run publishes an answer that its unit stage's child `go test` inherits, so that test would have judged the run instead of the manifest, and only from inside a verify. It now names an empty scope, with the reason on the line above it. `TestVerifyRunWidensWhenTheChangeSetCannotBeSelected` had the same shape and now asserts against what the process already held rather than against the empty string. Both were measured: `ZE_VERIFY_SCOPE_TAGS=<a one-tag answer> go test ./internal/le/staticcheckfeaturematrix/ ./internal/le/changed/... ./internal/le/verify/engine/` was RED before the fix and is green after.
- **`Derive`'s environment read had no test at all.** Every other caller names its scope, which is correct for them and left the one env-reading line uncovered. `TestDeriveReadsTheAnswerTheRunPublished` plants an answer through `env.Set` and asserts both halves: `Derive` narrows, and a caller naming its own scope does not.

### Documentation Updates
- None needed. `docs/functional-tests.md`, `docs/contributing/testing.md`, `docs/architecture/testing/tracked-build-gate.md` and `docs/architecture/testing/verify-freshness-scope.md` each already state that a verify run judges only the rows the change set can move and that typing the target yourself judges every row. Those sentences were FALSE while the producer was missing and are true again; the producer's own anchor landed with spec-verify-scope-2-change-set-selector.

### Deviations from Plan
- The spec named five tests in `internal/le/repository/` and `verifyengine_test.go`. Three of the five did not exist under those names, and the surviving behaviour lives in `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix_test.go`. Every row now names a test this closure ran.
- The Architectural Verification row citing "two `//go:build ignore` programs" and `scopeTagsEnvName` described the shell era. The contract is now one Go constant, `changed.ScopeTagsKey`, imported by its consumer.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The producer fix looked complete when its own tests passed | Restoring the producer re-armed AC-5's failure mode in a package the fix never touched: a test that reads the ambient answer judges the run that started it | Running the three packages with `ZE_VERIFY_SCOPE_TAGS` planted, which is what a verify run does to its unit stage's child | Both ambient reads removed, and the planted-answer run recorded as the evidence |
| assumption | AC-3 looked covered because the subtraction has tests | The subtraction's tests are arithmetic over a synthetic matrix. The only test that proved it SOUND, by compiling a real break in one row, had been deleted | Grepping each test name in the spec before pasting it forward | `TestMatrixRowFilterCatchesAGatedBreak` restored and strengthened |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The matrix reads the selector's answer | Done | `Derive` -> `env.Get(changed.ScopeTagsKey)` -> `DeriveScoped` -> `readChangeScope` | proved end to end by `TestDeriveReadsTheAnswerTheRunPublished` |
| It judges only the combinations the change can move | Done | `scopeMatrix` | subtracts from the derived rows; names none of them |
| `all_features` and `core_only` survive every scope | Done | `validateScoped`, `minMatrixRows` | refuses a scope that drops either |
| Every doubt widens | Done | `readChangeScope` | no answer, unreadable answer, undeclared tag, every declared tag |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestTheSubtractionNeverDropsAShippedCombination` | a one-tag answer leaves `all_features`, `core_only` and the one omission row |
| AC-2 | Done | `TestEveryDoubtJudgesTheWholeMatrix` (case "answer names every declared tag"), and `reachedTags` returning `everyTag` for an always-on package | an always-on change reaches every tag, so no row is subtracted |
| AC-3 | Done | `TestMatrixRowFilterCatchesAGatedBreak` | the scoped matrix exits 1 over a break only `without_ze_ssh` compiles |
| AC-4 | Done | `TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates`, and the second half of `TestMatrixRowFilterCatchesAGatedBreak` | the gate-only answer misses the break, so the union is load-bearing |
| AC-5 | Done | `TestDeriveReadsTheAnswerTheRunPublished`, `TestVerifyRunWidensWhenTheChangeSetCannotBeSelected` | a planted answer is visible to `Derive` and invisible to every caller that names its scope |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestTheSubtractionNeverDropsAShippedCombination`, `TestEveryDoubtJudgesTheWholeMatrix` | PASS | `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix_test.go` | |
| `TestMatrixRowFilterCatchesAGatedBreak` | PASS | same file | restored by this closure |
| `TestDeriveReadsTheAnswerTheRunPublished` | PASS | same file | written by this closure |
| `TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates` | PASS | `internal/le/changed/selector_test.go` | |
| `TestVerifyRunNamesTheFeatureScopeToEveryStage` | PASS | `internal/le/verify/engine/scope_test.go` | landed with spec-verify-scope-2-change-set-selector |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/verify/engine/run.go` | Done | the publication landed with sub-spec 2's closure, which is the spec whose Files to Modify named it |
| `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go` | Done | the row filter and its floor |
| `internal/le/changed/selector.go` | Done | `reachedTags` unions the negated tags |
| the three test files | Done | paths corrected in the table above |
| the five doc pages | Done | each already states the scoping; nothing was stale once the producer existed |

### Audit Summary
- **Total items:** 19
- **Done:** 19
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The matrix judges only the build combinations the change can move | unit, discriminated | `TestMatrixRowFilterCatchesAGatedBreak` runs the real Staticcheck over a fixture module twice: the selector's own answer for the changed file catches a break only `without_ze_ssh` compiles, and the gate-only answer does not. Neither half passes if the subtraction is a no-op |
| A skipped row never hides a type error (A-1) | unit, discriminated | the same test. Its second assertion is the failure mode, so a change making the union redundant turns the test red rather than leaving it green and meaningless |
| The consumer really reads what the producer publishes | unit | `TestDeriveReadsTheAnswerTheRunPublished` plants the answer through the same door `publishChangeScope` uses and asserts `Derive` narrows to it |
| A test judges its fixture, not the run that started it (AC-5) | measurement | `ZE_VERIFY_SCOPE_TAGS=<one-tag answer> ZE_VERIFY_SCOPE_PACKAGES=<answer> go test ./internal/le/staticcheckfeaturematrix/ ./internal/le/changed/... ./internal/le/verify/engine/` exits 0. The same command was RED on two tests before this closure fixed them |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Functional suite selection from the change set | No static signal attributes a `.ci` file to a Go package; measured at 423 of 646 packages in the intersection | `plan/spec-verify-scope-5-suite-coverage-map.md` |
| The `functional/verify` tier derivation reading a suite map | It only matters once a suite can be skipped, which this spec does not do | `plan/spec-verify-scope-5-suite-coverage-map.md`, AC-7 |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/verify-scope-3-selector-consumers-zeclose-vs3.md` |
| `review check` | clean |
| Rounds | 3. Round 1 found the missing soundness proof and the missing boundary test; round 2 found the ambient read the producer fix had re-armed; round 3 found three `t.Skip` calls in the new and restored tests and was clean after they became failures |
| Reviewer lenses used | assumption validation (A-1's soundness); test vacuity and discrimination; ambient-environment inheritance; guard-fails-closed; documentation against producer |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | AC-3 and A-1 have no soundness proof: every surviving test is arithmetic over a synthetic matrix | `scopeMatrix`, `internal/le/staticcheckfeaturematrix` | `TestMatrixRowFilterCatchesAGatedBreak` restored, with the gate-only answer asserted to MISS the break |
| 2 | ISSUE | `TestTheRealManifestDerivesEveryRow` reads the ambient feature-tag answer, so it judges the run that started it | the same file | it names an empty scope, with the reason stated on the line above |
| 3 | ISSUE | `TestVerifyRunWidensWhenTheChangeSetCannotBeSelected` asserts the empty string rather than what the process already held | `internal/le/verify/engine/scope_test.go` | the assertion is now against the ambient values captured before the run |
| 4 | ISSUE | `Derive`'s environment read, the one line joining producer to consumer, has no test | `Derive`, `internal/le/staticcheckfeaturematrix` | `TestDeriveReadsTheAnswerTheRunPublished` |
| 5 | ISSUE | Three `t.Skip` calls would let the soundness test pass without making its claim: staticcheck absent, the fixture matrix unjudged, and git unable to build the fixture checkout | `staticcheckfeaturematrix_test.go`, `scope_test.go` | all three are now `t.Fatalf`. Each names a broken machine rather than a case the test may decline |
| 6 | NOTE | Three of five TDD rows and three Data Flow cells name tests, files and a mechanism the tree no longer holds | this spec | every row and cell rewritten against the tree as it stands |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix.go` | Yes | holds `readChangeScope`, `scopeMatrix`, `validateScoped` |
| `internal/le/staticcheckfeaturematrix/staticcheckfeaturematrix_test.go` | Yes | holds the restored `TestMatrixRowFilterCatchesAGatedBreak` |
| `internal/le/staticcheckfeaturematrix/judge.go` | Yes | holds `Judge`, which the restored test drives |
| `internal/le/changed/selector.go` | Yes | holds `reachedTags` and `negatedTags` |
| `internal/le/verify/engine/scope.go` | Yes | holds `publishChangeScope` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | one tag leaves three rows | `TestTheSubtractionNeverDropsAShippedCombination` PASS |
| AC-2 | an always-on change judges every row | `TestEveryDoubtJudgesTheWholeMatrix` PASS |
| AC-3, AC-4 | the scoped rows still catch a gated break, and the gate-only answer misses it | `TestMatrixRowFilterCatchesAGatedBreak` PASS (0.39s, two real Staticcheck runs over the fixture) |
| AC-5 | no ambient answer reaches a fixture-driven assertion | the planted-answer run exits 0 over all three packages |
| all | the packages are green | `go test ./internal/le/staticcheckfeaturematrix/` exit 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le staticcheck-feature-matrix check` under a scoped answer | none; no `.ci` can drive it | `TestDeriveReadsTheAnswerTheRunPublished` covers the action's own boundary read, and `TestMatrixRowFilterCatchesAGatedBreak` covers the judgement |
| `./le verify current mode full` running the matrix stage | none | `TestVerifyRunNamesTheFeatureScopeToEveryStage` proves every stage reads the run's answer |
| a changed file constrained `!ze_X` | none | `TestSelectorTagAnswerHoldsTheFeaturesAChangedFileNegates` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, with the second clause proved rather than argued | `TestMatrixRowFilterCatchesAGatedBreak` compiles a real break under `ze_web && !ze_ssh`, catches it with the selector's answer, and misses it with the gate-only answer |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #10 test infrastructure | `docs/functional-tests.md` states the scoped rows and the six-way cut | Yes, unchanged and true |
| #12 internal architecture | `docs/architecture/testing/verify-freshness-scope.md` states the filter, the floor and the widening inputs | Yes, unchanged and true |
| #16 anchored pages | `docs/contributing/testing.md` and `docs/architecture/testing/tracked-build-gate.md` both say a verify run judges fewer rows and typing the target judges every row | Yes, read both; neither is stale now that a run publishes the answer |
| #17 examples | `ai/INDEX.md` and `docs/architecture/testing/verify-freshness-scope.md` name `ZE_VERIFY_SCOPE_TAGS` | Yes |
| every other row | No | Build tooling: no YANG, CLI verb, RPC, plugin, wire format, or RFC tier |

## Core Insight

Restoring a producer re-arms every failure mode its absence had suppressed. `ZE_VERIFY_SCOPE_TAGS` was unset for weeks, so a test reading it ambiently could not fail, and the two such tests looked correct for exactly as long as the feature was broken. The check that finds them is one command, not a review pass: run the packages with the variable planted, which is what the run does to its own children.
