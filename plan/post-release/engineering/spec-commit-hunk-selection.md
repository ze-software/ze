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
| Updated | 2026-10-25 |

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
| `TestCreateHunksCommitsOnlySelectedHunks` | `internal/le/commit/hunks_create_test.go` | AC-1 | done |
| `TestCreateHunksRefusesPatchThatDoesNotApply` | `internal/le/commit/hunks_create_test.go` | AC-2 | done |
| `TestApplyRefusesAnotherPath` | `internal/le/commit/hunks/hunks_test.go` | AC-3 | done |
| `TestCreateHunksRefusesDuplicatePath` | `internal/le/commit/hunks_create_test.go` | AC-4 | done |
| `TestCreateHunksGatesJudgeSelectedBlob` | `internal/le/commit/hunks_create_test.go` | AC-5: the weakening gate sees the selected test content | done |
| `TestCoverageHunksMalformedEntryFailsClosed` | `internal/le/commit/hunks_gates_test.go` | AC-5: the test-coverage gate itself returns the read error for a malformed entry, on a code file and on a test file. This is the test that pins the coverage read | done |
| `TestSourceGatesNoTestCannotWaiveMalformedEntry` | `internal/le/commit/hunks_gates_test.go` | AC-5: the entry point `checkSourceGates`, with `no-test` set, refuses a malformed entry. It proves only that the entry point refuses: the prospective commit reads the entry first, so it does not show which gate refused, and the coverage read is pinned by `TestCoverageHunksMalformedEntryFailsClosed` | done |

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
- The review pin compares like with like. The reviewer vouches for the bytes that land, so `./le spec review record ... hunks <path> <patch-file>` pins HEAD's blob with the patch applied (`spec.recordedHash` through `hunks.Content`), and `checkReviewSelected` hashes the selected blob. Pinning the working-tree file and comparing it with the working tree was rejected: a patch can carry lines the working tree never held, which no reviewer read. A pin over the working-tree file is stale, and a hunks path the artifact does not pin is unreviewed. Either way the one fix is a new review of the content the commit carries, so the refusal says that and names the record verb's usage shape with this commit's values filled in, `./le spec review record spec <stem> ... hunks <path> <patch-file>`, the reviewer's own words left as `...`. It prints no command that pins anything by itself. A printed re-pin command was rejected after three review rounds each found a way it cleared the gate with no review: a pin nobody reviewed has no correct automatic fix (round 4 below).
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
- `internal/le/commit/hunks` (`Apply`, `Selection`): `hunks <path> <patch>` applies the patch to HEAD's blob in a scratch index, refuses a patch that does not apply, touches another path, deletes, is binary or names a quoted path, and hands every gate the selected blob through `Selection.Read`.
- `./le commit create` keyword `hunks` (`internal/le/commit/actions.go`, `prepare.go`, `hunks_select.go`): a path named by `file` and `hunks`, or by `hunks` twice, is refused; the generated script (`script.go`) writes the selected blob into the commit's index and leaves the working-tree file untouched, then the shared-index repair writes HEAD's entry back.
- Gates on the selected blob: the weakening gate and its prospective index (`internal/le/test/weakened`), the RFC-tagged change gate (`rfcchange.go`), the test-coverage gate (`testCoverageProblemsSelected`), the journal shape check and owned-row guard (`specjournal.Reader`), the spec-closure sides and the review pin (`checkReviewSelected`). A malformed selection entry is an error at each, never a fall back to the working tree.
- `./le spec review record ... hunks <path> <patch>` (`internal/le/spec/review.go`, `recordedHash`) pins the selected content, so the review pin compares like with like; a stale or unreviewed hunks pin is refused with a re-review instruction and no printable re-pin command (`hunksReviewRefusal`).
- Verification freshness is reported stale for a hunks path; the structural gate and the drift note say they do not read the selected blob (committing.md).

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
| approach | Rounds 2 and 3 patched a generated re-pin command three times | Any command that re-pins a review without a new review clears the gate's own refusal; the only correct fix is a re-review | Independent review, round 4 (BLOCKER) | Command deleted in a215637558; the refusal names the record verb as a usage shape only |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Commit HEAD's blob plus only the named hunks of one file | done | `internal/le/commit/hunks/hunks.go` Apply; `internal/le/commit/script.go` | |
| Leave the working-tree file untouched | done | `internal/le/commit/script.go` | `TestCreateHunksCommitsOnlySelectedHunks` reads the file after the script |
| Every create-time gate judges the blob that will be committed | done | `hunks.Selection.Read` callers listed in What Was Implemented | The three judgements that cannot (verification state, structural gate, drift note) say so in committing.md |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestCreateHunksCommitsOnlySelectedHunks`, `TestCreateHunksPathWithSpace` | runs the generated script over a real repository |
| AC-2 | done | `TestCreateHunksRefusesPatchThatDoesNotApply`, `TestApplyRefusals` | |
| AC-3 | done | `TestApplyRefusesAnotherPath` | |
| AC-4 | done | `TestCreateHunksRefusesDuplicatePath` | |
| AC-5 | done | `TestCreateHunksGatesJudgeSelectedBlob`, `TestRFCChangeHunksJudgesSelectedBlob`, `TestCoverageHunksJudgesSelectedBlob`, `TestCoverageHunksMalformedEntryFailsClosed`, `TestClosureStemHunksJudgesSelectedOwner`, `TestClosureSidesHunksReadSelectedBlob`, `TestReviewHunksComparesLikeWithLike`, `TestWeakenedTestsRefusesAMalformedHunksEntry` | freshness reported stale: committing.md, "Three judgements do not read the selected blob" |
| AC-6 | done | `docs/contributing/committing.md` "Committing some hunks of a shared file"; `ai/rules/points/git-safety/directives/never-defer-a-shared-file-to-an-absent-session.md` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestCreateHunksCommitsOnlySelectedHunks` | done | `internal/le/commit/hunks_create_test.go` | |
| `TestCreateHunksRefusesPatchThatDoesNotApply` | done | `internal/le/commit/hunks_create_test.go` | |
| `TestApplyRefusesAnotherPath` | done | `internal/le/commit/hunks/hunks_test.go` | |
| `TestCreateHunksRefusesDuplicatePath` | done | `internal/le/commit/hunks_create_test.go` | |
| `TestCreateHunksGatesJudgeSelectedBlob` | done | `internal/le/commit/hunks_create_test.go` | |
| review-round tests | done | `internal/le/commit/hunks_gates_test.go`, `internal/le/spec/review_hunks_test.go`, `internal/le/test/weakened/prospective_test.go` | listed per round in the Review Gate |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/commit/actions.go`, `prepare.go`, `rfcchange.go`, `review.go`, `filestat.go`, `script.go` | done | plus `hunks_select.go` |
| `internal/le/test/weakened/prospective.go`, `testweakened.go` | done | |
| `internal/le/commit/hunks/hunks.go`, `hunks_test.go`, `internal/le/commit/hunks_create_test.go` | done | |
| `docs/contributing/committing.md`, the git-safety rule point | done | |
| `internal/le/spec/review.go`, `actions.go`, `internal/le/spec/journal/owner.go`, `validate.go` | changed | added by round 1 (Deviations) |

### Audit Summary
- **Total items:** 6 ACs, 3 requirements, 5 planned tests
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** the review pin's `hunks` keyword and the journal `Reader` (Deviations, round 1)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Commit HEAD's blob plus only the named hunks, working tree untouched | functional (package test over a real repository and the generated script) | `TestCreateHunksCommitsOnlySelectedHunks` |
| Every create-time content gate judges the blob that will be committed | package tests, each forced red | `TestCreateHunksGatesJudgeSelectedBlob`, `TestRFCChangeHunksJudgesSelectedBlob`, `TestCoverageHunksJudgesSelectedBlob`, `TestClosureStemHunksJudgesSelectedOwner`, `TestClosureStemHunksIgnoresForeignRow`, `TestClosureSidesHunksReadSelectedBlob`, `TestReviewHunksComparesLikeWithLike`, `TestReviewHunksRefusalPrintsNoRecordCommand`, `TestCoverageHunksMalformedEntryFailsClosed`, `TestWeakenedTestsRefusesAMalformedHunksEntry`; all five packages `ok` with `-count=1` (Pre-Commit Verification) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/commit-hunk-selection-918acecf-5576-405e-84bf-f8b106e9f9a4.md` (22 code files of the five commits, each by `file`: none is dirty, so no `hunks` path) |
| `./le spec review check` | `review_gate: OK (22 code files, clean, hashes match ...)` |
| Rounds | 5: round 1 over 322391f9eb; round 2 over 322391f9eb and f442e95663; round 3 over those and e77cbf6d17; round 4 over those and 1c6784bbc8; round 5 over those and a215637558, clean (0 BLOCKER, 0 ISSUE, 3 NOTE) |
| Reviewer lenses used | rounds 1 to 5: independent ze-read subagent run by the parent session |

The agents that wrote each round's fixes did not review them: every round ran in
an independent reviewer launched by the parent session. Round 5 was clean, and
the closing agent recorded it with `./le spec review record` over the code
files the five commits carry.

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

### Round 4

Round 4 ran in the parent session over 322391f9eb, f442e95663, e77cbf6d17 and
1c6784bbc8. The fixes below are in the round 4 fix commit; round 5 is owed over
it, run by the parent session.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The generated re-pin command had a defect again, as in rounds 2 and 3 | `internal/le/commit/review.go` hunksReviewRepin, parseRecordedReview | Deleted `hunksReviewRepin` and `parseRecordedReview` (round 3's replacement for `hunksReviewRecord`), their constants and their tests (`TestReviewHunksStaleNamesExactRecord`, `TestReviewHunksRepinRefusesWhatNobodyReviewed`). A stale or unreviewed pin has one correct fix, a new review of the content, so any command that re-pins without one is a way around the review gate, and patching it a fourth time would not change that (`ai/rules/simplicity.md`). `hunksReviewRefusal` says what is wrong, why, and that the content must be re-reviewed, and names `./le spec review record spec <stem> ... hunks <path> <patch>` with this commit's values; `TestReviewHunksRefusalPrintsNoRecordCommand` (stale pin, unreviewed path) runs every record line of the refusal through the real recorder and expects a refusal and an unchanged artifact; `TestReviewHunksComparesLikeWithLike` kept |
| 2 | ISSUE | "no-test cannot waive a malformed entry" was tested only through the helper | `internal/le/commit/prepare.go` checkSourceGates | `TestSourceGatesNoTestCannotWaiveMalformedEntry` drives `checkSourceGates` with `NoTest` set and a malformed entry on a code file and on a test file; both are refused, by the prospective commit, which reads the entry first |
| 3 | ISSUE | A malformed entry on a `_test.go` hunks path counted as the commit's test in `testCoverageProblemsSelected` without being read | `internal/le/commit/prepare.go` testCoverageProblemsSelected | The entry point already refused it (row 2), but the gate itself did not; a selected test path is now read before it counts, and the read error is returned; `TestCoverageHunksMalformedEntryFailsClosed` (test-file case) |

Row 1's test was forced red by restoring a printed `record ... verdict 'clean'
rounds 1 hunks ...` line in the refusal: the recorder accepted it and the test
failed on that line. Row 3's test was forced red by skipping the new read.
Both were restored green.

### Round 5

Round 5 ran as an independent ze-read subagent of the parent session over
322391f9eb, f442e95663, e77cbf6d17, 1c6784bbc8 and a215637558: 0 BLOCKER,
0 ISSUE, 3 NOTE. Clean. The notes are addressed in the closing commit and owe
no further round (record and prose only).

| # | Severity | Finding | Location | Addressed by |
|---|----------|---------|----------|--------------|
| 1 | NOTE | committing.md said the refusal names "each hunks path", but it names only the stale or unreviewed ones, while a record replaces the artifact | `docs/contributing/committing.md`; `internal/le/commit/review.go` hunksReviewRefusal | Doc chosen (no code change): it says the refusal names each stale or unreviewed hunks path, and that a record replaces the artifact (`recordReview`, `writeAtomic`), so the new record names every code file of the commit, each hunks path with `hunks` and every other with `file` |
| 2 | NOTE | `TestSourceGatesNoTestCannotWaiveMalformedEntry` proves only that the entry point refuses | TDD Test Plan | The test table says so, and names `TestCoverageHunksMalformedEntryFailsClosed` as the test that pins the coverage read |
| 3 | NOTE | Round 4 row 1 was labelled ISSUE; it was a BLOCKER | Review Gate, Round 4 | Label corrected |

## Pre-Commit Verification

Scoped evidence only. The full release check (`./le verify current`) is
Thomas's to run and is not owed by closure (`ai/META-DECISIONS.md`, "Ze is
pre-release"); `./le verify changed` over the closing commits is owed to the
main thread.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/commit/hunks/hunks.go`, `hunks_test.go` | yes | tracked since 322391f9eb; package `ok` below |
| `internal/le/commit/hunks_create_test.go`, `hunks_gates_test.go`, `hunks_select.go` | yes | tracked since 322391f9eb / f442e95663 |
| `internal/le/spec/review_hunks_test.go` | yes | tracked since f442e95663 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-5 | the tests in the Implementation Audit pass | `./le job run label unit-pkg quiet command go test -count=1 ./internal/le/commit/ ./internal/le/commit/hunks/ ./internal/le/spec/ ./internal/le/spec/journal/ ./internal/le/test/weakened/`: `ok internal/le/commit 67.200s`, `ok internal/le/commit/hunks 0.876s`, `ok internal/le/spec 0.495s`, `ok internal/le/spec/journal 7.475s`, `ok internal/le/test/weakened 7.744s` (same five packages `ok` with `-count=1` in round 5) |
| AC-6 | the keyword and the foreign-hunk rule are documented | committing.md "Committing some hunks of a shared file"; the git-safety point names `hunks` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le commit create ... hunks <path> <patch>` | none (tooling): `TestCreateHunksCommitsOnlySelectedHunks` drives `parseCreate` -> `Create` -> the generated script over a real repository | yes, `ok internal/le/commit` above |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 (`git diff -- <path>` with foreign hunks deleted is a usable patch) | confirmed | `TestCreateHunksCommitsOnlySelectedHunks` builds its patch that way and the script commits it |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| committing.md, the refusal and the record | `internal/le/commit/review.go` hunksReviewRefusal; `internal/le/spec/review.go` recordReview | yes, `./le doc check verify`: "Documentation tests PASSED", exit 0 |
| committing.md, the gates that read the selected blob and the three that do not | `hunks.Selection.Read` callers; `structuralGateReds`; `renderSharedIndexRepair` | reviewed in rounds 1 to 5 |
