# Spec: commit-hunk-selection

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Commit tooling for the shared checkout; nothing in the shipped binary changes |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-24 |

Approved by Thomas on 2026-10-24 (approach approved, design delegated), to break
the commit deadlock between the verify-release-check-only and
le-build-cache-cap sessions.

## Task

`./le commit create` stages whole files only. Sessions share one checkout, so a
file often holds hunks from two sessions whose hunks each compile only with
that session's other files: neither side can commit the whole file. Add a
keyword that commits HEAD's blob plus only the named hunks of one file, leaves
the working-tree file untouched, and has every create-time gate judge the blob
that will be committed.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` -- keyword table, the generated script
  -> Constraint: the script commits the `git ls-files -s` line create wrote, so the content is fixed at preparation time.
- [ ] `ai/doors/le-tooling.md` -- the shared index is a contract with every session
- [ ] `docs/architecture/core-design.md` -- le registers its actions; no new command, so no change
- [ ] `docs/architecture/testing/test-health.md` -- the weakening gate; it judges the selected blob, which the page does not contradict

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/actions.go` - keyword table and `parseCreate`
- [ ] `internal/le/commit/prepare.go` - `Create` and its gates
- [ ] `internal/le/commit/snapshot.go` - the create-time snapshot
- [ ] `internal/le/commit/script.go` - `renderDriftNote`, the block layout
- [ ] `internal/le/test/weakened/prospective.go` - the prospective index
- [ ] `internal/le/test/weakened/testweakened.go` - `comparePath`

**Behavior to preserve:** every existing keyword, the snapshot guarantee, the script layout.

**Behavior to change:** a new `hunks` keyword.

- `internal/le/commit/actions.go`: `createKeywords`, `parseCreate`; `file`/`file-list` expand into `Options.Files`.
- `internal/le/commit/prepare.go`: `Create` runs `checkSourceGates`, `checkVerificationGates`, reads `preparedHead`, then `snapshotIndexEntries` (working tree into a private index).
- `internal/le/commit/snapshot.go`: one `git ls-files -s` line per path.
- Content readers of a named path at create time: `testweakened.ProspectiveCommit` (`update-index --add` from the working tree), `testweakened.Check` via `comparePath`/`worktreeText`, `rfcChangeProblems` (`os.ReadFile`), `testCoverageProblems` (`os.ReadFile`), `closureStem`/`CheckReview` via `workingTreeSides`, `fileStats` (`git diff --numstat HEAD`), `verificationState` (certificate manifest hashes the working tree).
- `renderDriftNote` reports a named path whose working-tree entry differs from the committed one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`./le commit create ... hunks <path> <patch-file>`.

### Transformation Path
1. `parseCreate` collects `(path, patch)` pairs into `Options.Hunks`.
2. `Create` reads HEAD once, then `hunks.Apply` builds each blob: a private index read from that HEAD, `git apply --cached` of the patch, and a check that the patch changed exactly that path. The answer is the path's `git ls-files -s` line.
3. Every gate reads a hunk path through `hunks.Selection.Read`, which answers the selected blob, never the working-tree file.
4. The block carries the selected entry lines beside the snapshot lines, records the HEAD the patch applied to as `PreparedHead`, and the drift note leaves hunk paths out.

### Boundaries Crossed
None outside `internal/le`.

### Integration Points
- `internal/le/test/weakened` gains a `Selected` field on `Request` and a parameter on `ProspectiveCommit`.

### Architectural Verification
- [ ] The working tree is never written: the patch applies to a private index only.
- [ ] HEAD is read once and is both the patch base and `PreparedHead`, so the script's existing peer-change refusal covers a moved HEAD.

## Risks & Assumptions

### Assumptions
- A session can produce the patch with `git diff -- <path>` and delete the foreign hunks.

### Risks
- A gate that reads a hunk path from the working tree would judge content the commit does not carry. Mitigation: every reader listed above takes the selection; verification freshness is reported stale for a hunk path because the manifest hashes the working tree.

## Blast Radius

`internal/le/commit`, `internal/le/test/weakened`, `docs/contributing/committing.md`, the git-safety foreign-hunk rule point.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| `./le commit create ... hunks <path> <patch>` | `parseCreate` -> `Create` -> `hunks.Apply` | `TestCreateHunksCommitsOnlySelectedHunks` runs the generated script |

## Acceptance Criteria

| AC | Criterion |
|----|-----------|
| AC-1 | A file holding two sessions' hunks commits HEAD's blob plus only the selected hunks; the working-tree file still holds both afterwards |
| AC-2 | A patch that does not apply to HEAD's blob is refused at create time, naming the path, the reason and the fix |
| AC-3 | A patch that touches a path other than the one named is refused |
| AC-4 | A path named by both `file` and `hunks`, or by `hunks` twice, is refused |
| AC-5 | The weakening, RFC-change, test-coverage and closure gates read the selected blob, and verification freshness is not claimed for a hunk path |
| AC-6 | committing.md documents the keyword; the git-safety foreign-hunk rule says when to use it |

## End-to-End User Stories

A session whose file also carries another session's hunks saves `git diff -- <path>`, deletes the foreign hunks, passes `hunks <path> <patch>`, and runs the printed script.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateHunksCommitsOnlySelectedHunks` | `internal/le/commit/hunks_create_test.go` | AC-1 | |
| `TestCreateHunksRefusesPatchThatDoesNotApply` | `internal/le/commit/hunks_create_test.go` | AC-2 | |
| `TestApplyRefusesAnotherPath` | `internal/le/commit/hunks/hunks_test.go` | AC-3 | |
| `TestCreateHunksRefusesDuplicatePath` | `internal/le/commit/hunks_create_test.go` | AC-4 | |
| `TestCreateHunksGatesJudgeSelectedBlob` | `internal/le/commit/hunks_create_test.go` | AC-5: the weakening gate sees the selected test content | |

### Functional Tests
None: `./le commit create` is driven by its package tests over a real repository, as the existing script tests are.

## Files to Modify
- `internal/le/commit/actions.go`, `prepare.go`, `rfcchange.go`, `review.go`, `filestat.go`, `script.go`
- `internal/le/test/weakened/prospective.go`, `testweakened.go`
- `docs/contributing/committing.md`, the git-safety rule point

## Files to Create
- `internal/le/commit/hunks/hunks.go`, `hunks_test.go`
- `internal/le/commit/hunks_create_test.go`

## Implementation Steps
1. Tests first, red.
2. `hunks` package, keyword, gate wiring, block wiring.
3. Docs and rule point; render and lint the rules.

## Owned journal rows

| Row | Status |
|-----|--------|
| - | - |

## Key Design Decisions

- One keyword with two values, `hunks <path> <patch-file>`, rather than an interactive selection: an agent writes a patch file, and the patch is the reviewable record of what it chose.
- The blob is computed at create time, against the same HEAD recorded as `PreparedHead`, so the existing guarantee holds unchanged: the script commits the blob create read and refuses when a peer changed the path since.
- Gates read through one `Selection.Read`, so a hunk path has exactly one content source at create time.
- The journal readers take the content the commit carries: `specjournal.Reader`, passed to `ValidateCarried` and `UnownedOpenRows` by `closureStemSelected` as `selection.Read`, so an owner entry or an open row only the working tree holds counts as absent. `ValidateFile` and the post-write hook keep the working tree (`specjournal.WorkingTree`).
- The review pin compares like with like. The reviewer vouches for the bytes that land, so `./le spec review record ... hunks <path> <patch-file>` pins HEAD's blob with the patch applied (`spec.recordedHash` through `hunks.Content`), and `checkReviewSelected` hashes the selected blob. Pinning the working-tree file and comparing it with the working tree was rejected: a patch can carry lines the working tree never held, which no reviewer read. A pin over the working-tree file is stale, and the refusal names the `hunks` record that clears it.
- A binary patch, an empty patch and a path git quotes are refused in `hunks.Apply` before any blob is built, each with its fix (`file <path>`; save the patch again; rename the path).

## Known Limitations

- A patch may not delete the file: a deletion is `remove`.
- The structural gate (`structuralGateReds`) charges reds by path from a failure index written over the working tree, and the verification state is reported stale, so neither judges the selected blob; `docs/contributing/committing.md` names both.
- `./le spec review check` (standalone, and the script's re-check) hashes the working tree, so the script's re-check leaves a hunks path out; the create-time pin judged it.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output): `./le job run label unit-pkg quiet command go test ./internal/le/commit/... ./internal/le/test/weakened/` and `./le test weakened selftest`
- [ ] Boundary tests for all numeric inputs (N-A: the feature takes no numeric input)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: tooling, driven by package tests over a real repository)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-commit-hunk-selection.md` only

## Implementation Summary

### What Was Implemented
- [filled at closure, after the round 2 review]

### Bugs Found/Fixed
- Round 1 review findings, each with its test: see Review Gate, "Findings fixed".

### Documentation Updates
- `docs/contributing/committing.md`, "Committing some hunks of a shared file"; `ai/rules/points/git-safety/directives/never-defer-a-shared-file-to-an-absent-session.md` (rendered, condensed, linted); `ai/skills/ze-close.md` step 5 names the `hunks` review record. `./le doc check verify`: exit 0.

### Deviations from Plan
- The review pin is taken over the selected content through a new `hunks` keyword of `./le spec review record` (Key Design Decisions).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Commit 322391f9eb threaded the selection into the closure sides but not into the journal readers it calls first | `specjournal.ValidateFile` and `UnownedOpenRows` read the working tree themselves | Independent review, round 1 | Reader parameter; tests forced red |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (each needs user approval)
- **Skipped:** (each needs user approval)
- **Changed:** (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Commit HEAD's blob plus only the named hunks, working tree untouched | functional (package test over a real repository and the generated script) | `TestCreateHunksCommitsOnlySelectedHunks` |
| Every create-time content gate judges the blob that will be committed | package tests, each forced red | `TestCreateHunksGatesJudgeSelectedBlob`, `TestRFCChangeHunksJudgesSelectedBlob`, `TestCoverageHunksJudgesSelectedBlob`, `TestClosureStemHunksJudgesSelectedOwner`, `TestClosureStemHunksIgnoresForeignRow`, `TestClosureSidesHunksReadSelectedBlob`, `TestReviewHunksComparesLikeWithLike` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | not yet recorded: the round 4 re-review is owed |
| `./le spec review check` | not run |
| Rounds | round 1 over 322391f9eb; round 2 over 322391f9eb and f442e95663; round 3 over those and e77cbf6d17 (findings below); round 4 owed over the round 3 fix commit, run by the parent session |
| Reviewer lenses used | rounds 1 to 3: independent reviewer in the parent session |

The agent that wrote the round 1 fixes holds no Agent tool and authored them, so
it cannot review them. The parent session runs the independent round 2 re-review
over 322391f9eb plus the fix commit and records it with `./le spec review
record` (a code file committed through `hunks` is recorded with `hunks <path>
<patch-file>`). Commits A and B are prepared only after a clean run.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The owned-row guard and the journal shape check read the working tree for a hunks path | `internal/le/commit/review.go` closureStemSelected; `internal/le/spec/journal/owner.go` UnownedOpenRows | `specjournal.Reader`; `TestClosureStemHunksJudgesSelectedOwner`, `TestClosureStemHunksIgnoresForeignRow` |
| 2 | ISSUE | The review pin compared the selected blob with a pin over the working-tree file, so a hunks code file was always stale | `internal/le/spec/review.go` reviewFileHash; `internal/le/commit/review.go` checkReviewSelected | `review record ... hunks` pins `hunks.Content`, the refusal names it; `TestReviewHunksComparesLikeWithLike`, `TestRecordReviewHunksPinsSelectedContent` |
| 3 | ISSUE | No test drove a gate with a hunks path | `internal/le/commit` | `TestRFCChangeHunksJudgesSelectedBlob`, `TestCoverageHunksJudgesSelectedBlob`, `TestClosureSidesHunksReadSelectedBlob`, `TestScriptHunksRefusesPeerCommitOnPath`, `TestApplyRefusals` (empty, deletion, binary, quoted) |
| 4 | ISSUE | committing.md and the git-safety point claimed every gate judges the selected blob | `docs/contributing/committing.md`; the git-safety point | Both name the gates that do, the structural gate and verification state that do not, and the shared-index repair |
| 5 | NOTE | Binary paths were not refused | `internal/le/commit/hunks/hunks.go` Apply | `binaryProblem` |
| 6 | NOTE | The quoted-path refusal gave no fix | `internal/le/commit/hunks/hunks.go` Apply | `mustQuote`, `quotedProblem` |

Each new test was forced red by breaking its producer (the journal reader, the
review hash, the RFC read, the sides' selection, the record hash, each Apply
refusal, the script's moved-path list, `Selection.Read`), then restored green.

### Round 2

Round 2 ran in the parent session over 322391f9eb and f442e95663: 0 BLOCKER,
1 ISSUE, 2 NOTE. The fixes below are in the round 2 fix commit; round 3 is owed
over it, run by the parent session.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `changedProblem` split `git diff --name-only` on whitespace, so a path with a space read as two names and the patch was refused for changing a path it never touched | `internal/le/commit/hunks/hunks.go` Apply, changedProblem | `diff --name-only -z`, split on NUL; the script already names each path in a single-quoted shell word and reads git's names NUL-terminated, proven end to end; `TestApplyAdmitsPathWithSpace`, `TestCreateHunksPathWithSpace`; committing.md says a space is not a quoted path |
| 2 | NOTE | `Selection.Blob` answered "not selected" for a malformed entry, so `Read` fell back to the working tree | `internal/le/commit/hunks/hunks.go` Selection | `Selected` answers membership; `Blob` answers an error for an entry that is not a `git ls-files -s` line of that path; `Read` and the review pin surface it, `fileStats` reports the counts unavailable; `TestSelectionRefusesMalformedEntry` |
| 3 | NOTE | The stale-pin hint held `...`, so it could not be pasted | `internal/le/commit/review.go` checkReviewSelected | `Selection` carries each path's patch; `hunksReviewRecord` prints the whole shell-quoted record from the artifact's rounds and reviewers, each hunks path with its patch and every other code file; `TestReviewHunksStaleNamesExactRecord`, `TestReviewHunksComparesLikeWithLike` |

Each round 2 test was forced red by breaking its producer (a whitespace split
of the changed names, a `Read` that falls back when `Blob` fails, the old `...`
hint), then restored green.

### Round 3

Round 3 ran in the parent session over 322391f9eb, f442e95663 and e77cbf6d17:
0 BLOCKER, 3 ISSUE, 1 NOTE. The fixes below are in the round 3 fix commit;
round 4 is owed over it, run by the parent session.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The printed record pinned every other code file as `file`, stale or unreviewed ones included, so pasting it cleared the gate's own refusal with no review | `internal/le/commit/review.go` hunksReviewRecord | `hunksReviewRepin` prints the command only when every path it would pin as `file` (each the artifact pins and each code file of the commit) still matches its pin; otherwise it prints none and names the paths that need a re-review first; `TestReviewHunksRepinRefusesWhatNobodyReviewed` (changed after the review, never reviewed) |
| 2 | ISSUE | `verdict clean` was hard-coded | `internal/le/commit/review.go` hunksReviewRecord | No command for a verdict that is not clean: the refusal says the verdict blocks first; `TestReviewHunksRepinRefusesWhatNobodyReviewed` (findings verdict) |
| 3 | ISSUE | A malformed selection entry was swallowed by the coverage gate (`continue`) and the weakening gate (empty text) | `internal/le/commit/prepare.go` testCoverageProblemsSelected; `internal/le/test/weakened/testweakened.go` worktreeText | The coverage gate returns the read error, which `no-test` cannot waive; `worktreeText` answers the read problem, reported as "check could not run"; committing.md and the `selectedFileStat` comment now hold; `TestCoverageHunksMalformedEntryFailsClosed`, `TestWeakenedTestsRefusesAMalformedHunksEntry` |
| 4 | NOTE | The command dropped findings and every reason line past the first, and its rounds placeholder failed when pasted | `internal/le/commit/review.go` hunksReviewRecord | `parseRecordedReview` reads the artifact in exactly the shape the recorder writes, every byte consumed, and carries the full reason and owner authorization; an artifact holding findings, or not in that shape, gets no command and the refusal says why; `TestReviewHunksStaleNamesExactRecord` records through the real recorder with a two-line reason holding a single quote, pastes the printed command into the recorder and finds the gate clean and the reason whole; the findings and hand-written cases are in `TestReviewHunksRepinRefusesWhatNobodyReviewed` |

Each round 3 test was forced red by breaking its producer (the freshness check,
the verdict check, a reason cut to its first line, the coverage `continue`, an
empty `worktreeText`), then restored green.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
