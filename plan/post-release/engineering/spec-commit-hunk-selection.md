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

## Known Limitations

- A patch may not delete the file: a deletion is `remove`.

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
