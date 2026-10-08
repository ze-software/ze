# Spec: a prepared commit stages in a private index over a snapshot

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - (`spec-ledger-shards-per-commit-session`, which also touched `internal/le/commit/prepare.go`, closed 2026-10-07) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

Product code landed in `a9f2207a3` and `06f6185cb`. End-to-end proof and
closure remain outstanding.

<!-- Backfilled after work commissioned from a journal row. Product code
     landed in the commits named above; proof, the limitation decision and
     closure remain outstanding. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Several sessions share this checkout and therefore share one git index. The
generated commit script guarded against that by reading `git diff --cached
--name-only` before its own `git add`, and `git commit` then committed whatever
the index held when IT ran. Everything another session staged between the check
and the commit was carried, with the guard already passed.

Evidence is eight rows in `plan/journal/concurrent-session-corruption.md`. The
seventh happened AFTER the author was warned by name about the exact files, which
disproves the habit-based mitigation rather than merely finding it unreliable.
One occurrence left `cmd/ze/hub` uncompilable on `main` for eleven hours, because
a caller was swept in while the file defining its callee stayed uncommitted.
The rows are not restated here.

Goal: a block commits its OWN population and its own content, and the author's
care is irrelevant to whether that holds.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` - the commit route and what a prepared commit carries
  → Constraint: the generated script is the only route, so the mechanism has to
    live inside what `./le commit create` writes.
- [ ] `ai/rules/git-safety.md` - the banned verbs
  → Constraint: `git restore --staged` is banned for agents, so nothing may leave
    the SHARED index dirtier than it found it. That is why the block repairs the
    shared index after committing.
- [ ] `ai/rules/never-destroy-work.md` - uncommitted work belongs to whoever wrote it
  → Decision: an edit that arrives after preparation is LEFT in the working tree,
    never carried and never reverted.

**Key insights:**
- No ordering of check-then-add closes the window: it lies between the last add
  and the commit, where a per-block guard cannot see.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/script.go` - `renderBlock`, `renderPrivateIndex`, `renderDriftNote`, `renderSharedIndexRepair`, `indexFileFor`
- [ ] `internal/le/commit/snapshot.go` - `snapshotIndexEntries`, `checkSnapshot`, `gitIndexOutput`
- [ ] `internal/le/commit/prepare.go` - `Create`, which reads the snapshot at preparation time
- [ ] `internal/le/commit/filestat.go` - `FileStat` and `fileStats`, the per-path diffstat `create` prints
- [ ] `docs/contributing/committing.md` - the published contract

**Behavior to preserve:**
- `./le commit create` keeps its verbs, its `script=` line and its refusals.
- A peer's commit made between preparation and the run is KEPT: the block's tree
  is HEAD at run time plus this block's own paths.
- The shared index is left describing what was committed for this block's paths,
  and nothing else in it is touched.

**Behavior to change:**
- The concurrency guard, which read the shared index and aborted, is DELETED.
- `snapshotIndexEntries` reads one `git ls-files -s` line per path at PREPARATION
  time, in a private index under `tmp/`, so the blobs land in the object database
  and the shared index is never written.
- The generated block seeds a private index from HEAD at run time, feeds it the
  snapshot through `git update-index --index-info`, and commits with
  `GIT_INDEX_FILE` pointing at it.
- A path whose content changed after preparation produces a NOTE on stderr, not a
  refusal. The comparison runs against a COPY of the index, because
  `git update-index --refresh` in place rewrites the entry from the working tree.

## Data Flow (MANDATORY)

### Entry Point
- `./le commit create subject <text> file <path> ...`, run by an agent. Entry
  format is command-line arguments naming the population.

### Transformation Path
1. `Create` validates every path and measures it against HEAD (`fileStats`).
2. `snapshotIndexEntries` stages those paths into a throwaway index under `tmp/`
   and reads back one `ls-files -s` line each. `checkSnapshot` refuses a missing
   entry and a path git would quote.
3. `renderBlock` writes the block: seed a private index from HEAD, feed the
   snapshot, remove the `Removed` paths, print the drift note, commit.
4. `renderSharedIndexRepair` points the shared index at what was committed, for
   this block's paths only, and removes the private index file.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| preparation ↔ script run | blob hashes in the object database, carried in the script text | Yes, by `snapshot_test.go` |
| this session ↔ every other session | the shared index is never written before the commit | Yes, by construction: the commit reads `GIT_INDEX_FILE` |
| script ↔ `git status` for other sessions | `renderSharedIndexRepair` | Yes, the repair names only this block's paths |

### Integration Points
- `internal/le/commit/prepare.go` - `Create` calls `snapshotIndexEntries` and
  carries the result into `commitBlock.IndexEntries`.
- The private index file is named beside the script, sharing its random suffix,
  so a reader who has the script path has the index path.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the block is still the only thing that commits |
| No unintended coupling | Yes | confined to `internal/le/commit` |
| No duplicated functionality | Yes | the old guard is DELETED, not kept beside the new mechanism (`ai/rules/no-layering.md`) |
| Zero-copy preserved where applicable | N-A | tooling path |
| Registration over hardcoding | N-A | no registry involved |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `git ls-files -s` line is a stable interchange for `update-index --index-info` | git's own documented format | the script cannot rebuild the index | `snapshot_test.go` round-trips it | confirmed |
| A-2 | The object database keeps the snapshot blob alive between preparation and the run | blobs are unreferenced until the commit, so `gc --prune=now` could collect them | a script prepared long ago fails with an unknown object | `TestAPreparedBlobSurvivesAnOrdinaryGC` (2026-10-07): under `git gc` with its default expiry the script commits the prepared content; under `git gc --prune=now` the blob is collected and the run fails over `invalid object ... for 'mine.txt'` with HEAD and the shared index unchanged. Measured: `update-index --index-info` ACCEPTS the missing object, and `git commit` is the step that refuses | confirmed (fail-closed bound: a script older than `gc.pruneExpire` can fail, never mis-commit) |
| A-3 | Repairing the shared index for this block's paths cannot destroy another session's staged entry | the repair names only `block.Paths` | a peer's staged path is reset | foreign path: `TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit`. Same path: `TestAPeerStagedEntryForANamedPathIsResetToTheCommit` (2026-10-07) shows the repair REPLACES a peer's staged entry for a path this block names, while the peer's content stays in the working tree and the drift note names the path | confirmed for a foreign path; QUALIFIED for the same path: the entry is reset, the content survives on disk. A peer that staged a path and then edited it again keeps only the newer edit on disk. Staging in the shared index is itself banned for every session (`ai/rules/git-safety.md`), so that entry has no sanctioned writer |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The drift note is read as a refusal and an author re-prepares needlessly | authors report churn | the note states plainly that the commit carries the prepared content and the difference stays in the tree |
| R-2 | A path git must quote reaches the heredoc | `checkSnapshot` refuses it | refusal at preparation, never a malformed script |
| R-3 | The delimiter `ZE_INDEX_INFO` appears in an entry | impossible: an entry starts with a six-digit mode | none needed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | every commit in this repository. A wrong index rebuild commits the wrong tree |
| How is it reverted? | not yet landed. Once landed, a single commit revert restores the old guard |
| Who else touches this path? | every session in this checkout, and the ledger-shard spec, which edits `prepare.go` too |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit create` with a path list | → | `snapshotIndexEntries` (`internal/le/commit/snapshot.go`) | `internal/le/commit/snapshot_test.go` |
| the generated script, run while a foreign path is staged | → | `renderPrivateIndex` (`internal/le/commit/script.go`) | `internal/le/commit/commit_test.go`, the block-rendering cases |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | another session has a foreign path staged in the shared index when the script runs | the commit carries this block's paths only, and does not abort |
| AC-2 | a peer lands a commit between preparation and the run | the block's tree is the NEW HEAD plus this block's paths; the peer's commit is kept |
| AC-3 | a named path is edited after preparation | the commit carries the PREPARED content, the working tree keeps the newer edit, and stderr says so |
| AC-4 | a named path is removed by `remove` | the commit deletes it, and the shared index agrees afterwards |
| AC-5 | after the script runs | `git status` for this block's paths shows them committed, and no other entry in the shared index moved |
| AC-6 | a path git would quote, or one git stages no content for | `create` refuses at preparation, naming the path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| snapshot round-trip and refusal cases | `internal/le/commit/snapshot_test.go` | AC-6, A-1 | landed `a9f2207a3` |
| rendered block shape | `internal/le/commit/commit_test.go` | AC-1, AC-3, AC-4 | landed `a9f2207a3` |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| paths per block | 0..n | n | N/A, an empty block writes no `--index-info` heredoc | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit` | `internal/le/commit/snapshot_test.go` | an agent commits while a peer holds the index, moves HEAD and edits a named path (AC-1, AC-2, AC-3, AC-5) | PASS 2026-10-07 |
| `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex` | `internal/le/commit/snapshot_test.go` | a removal reaches the commit and the shared index agrees (AC-4) | PASS 2026-10-07 |
| `TestABlockLeavesTheSharedIndexAloneWhenItsCommitFails` | `internal/le/commit/snapshot_test.go` | a failed block changes neither HEAD nor the shared index | PASS 2026-10-07 |
| `TestAPreparedBlobSurvivesAnOrdinaryGC`, `TestAPeerStagedEntryForANamedPathIsResetToTheCommit` | `internal/le/commit/snapshot_test.go` | A-2 and A-3 | added and PASS 2026-10-07 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | - | - | tooling, no wire-visible behavior | N-A |

## Files to Modify
- `internal/le/commit/script.go` - `renderBlock` and its three new renderers; the old guard deleted
- `internal/le/commit/prepare.go` - `Create` carries the snapshot into the block
- `internal/le/commit/commit_test.go` - the block-rendering cases
- `docs/contributing/committing.md` - what a prepared commit carries

## Files to Create
- `internal/le/commit/snapshot.go` - `snapshotIndexEntries`, `checkSnapshot`, `gitIndexOutput`
- `internal/le/commit/snapshot_test.go` - its tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | development tooling, no operator config |
| CLI commands/flags | No | `./le commit create` keeps its verbs and flags |
| Functional test for new RPC/API | N-A | no RPC |
| Doctor check for runtime dependencies | N-A | `git` was already required |
| Env var registration | N-A | `GIT_INDEX_FILE` is set for a child process, not read as ze config |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/contributing/committing.md`, which must state that the commit carries the PREPARED content |
| 10 | Test infrastructure changed? | No | the test runner is untouched |
| 16 | Any changed source file referenced by existing doc anchors? | Yes | `docs/contributing/committing.md` is the `// Design:` anchor of `script.go` and `snapshot.go`. `docs/features/ai-first.md` is the anchor of `prepare.go` and is UNAFFECTED: it describes the per-session commit identity, which this change does not alter |
| 1, 2, 4-9, 11-15, 17 | - | No | no operator-facing surface changed |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - `snapshotIndexEntries` and its test,
   proving a path list becomes index entries and a quoted path is refused.
2. **Phase: Block rendering** - `renderPrivateIndex` and `renderSharedIndexRepair`;
   DELETE `renderStagingGuard` and `renderAdd` in the same step.
3. **Phase: Drift note** - `renderDriftNote` against a COPY of the index.
4. **Phase: End-to-end** - a test that runs a generated script in a throwaway
   repository with a foreign path staged.
5. **Phase: Documentation** - `docs/contributing/committing.md`.
6. **Phase: Land it** - DONE, `a9f2207a3` and `06f6185cb` on 2026-09-06.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | the old guard is gone, not disabled |
| Correctness | the drift comparison never rewrites the snapshot entry |
| Data flow | the shared index is written only AFTER the commit, and only for this block's paths |
| Rule: `ai/rules/never-destroy-work.md` | a post-preparation edit is left in the tree, never carried and never reverted |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no `git add` in a generated block | `grep -n 'git add' tmp/commit-*.sh` over a fresh `create` |
| the commit's population equals the block's paths | run a script with a foreign path staged and read `git show --name-only` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Injection into the heredoc | an `ls-files -s` entry cannot spell the delimiter; a quoted path is refused before it reaches the script |
| Temporary file placement | the private index sits beside the script under `tmp/`, sharing its random suffix, so no other session can take the name |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The commit carries a path the block did not name | `checkSnapshot` or the repair is wrong; re-read the producer |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- A guard that runs at the wrong instant cannot be repaired by moving it. The
  window is between the last add and the commit, so the fix is to stop sharing
  the index rather than to check it more carefully.
- `git update-index --refresh` REWRITES an entry whose file changed. Measured on
  2026-09-06: a refresh in place replaced the snapshot blob with the drifted one
  and committed exactly what this design exists to leave behind.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Private index over `GIT_INDEX_FILE` | `git commit -- <pathspec>` | the pathspec form still reads the WORKING TREE at commit time, so it fixes the population and not the content |
| Snapshot at preparation, commit at run | snapshot at run time | `create`'s gates judge the content they read; committing something else makes those gates advisory |
| Drift is a NOTE, not a refusal | refuse on drift | the commit is already safe, and refusing blocks an author whose only offense is that a peer touched a shared file |

## Known Limitations
- **An interloper's edit ALREADY in the working tree when preparation runs is
  still carried.** The snapshot binds the content to preparation time, which
  closes the window AFTER it; nothing here can tell whose hunks a file held
  BEFORE it. The per-path diffstat `create` prints (`FileStat`,
  `internal/le/commit/filestat.go`) is the only signal for that case, and its own
  comment says why it is a print rather than a refusal: nothing here knows which
  hunks this author wrote. A signal is not a mechanism, and the seventh journal
  row is the measurement of what a signal is worth against habit.
- The September 6 evidence record below does not establish the end-to-end
  obligations. Reconciled at closure (2026-10-08): the end-to-end obligations
  are proved by the script tests named in the Functional Tests table.
- A `.git/index.lock` held past `indexLockWaitSecondsMax` (30 seconds) still
  leaves the shared index stale after the commit. The script then fails loudly
  and prints the exact repair; waiting cannot clear an orphaned lock.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean

## Current Condition and What Remains

The September 6 record says the product code, two unit tests and
`docs/contributing/committing.md` edits landed as `a9f2207a3`, followed by
`06f6185cb` for the drift note. `renderBlock` and `renderPrivateIndex` in
`internal/le/commit/script.go` still implement the private-index path. The
evidence rows below describe that dated record; they are not a fresh test run.

| Item | State |
|------|-------|
| Product code | `internal/le/commit/snapshot.go` (new), `script.go`, `prepare.go`. Landed `a9f2207a3`, with `06f6185cb` on top |
| Documentation | `docs/contributing/committing.md` reconciled with the new contract in `a9f2207a3` and `06f6185cb` |
| Journal row | written, `plan/journal/concurrent-session-corruption.md`, seventh occurrence |
| PROVEN | the snapshot round-trip and the rendered block shape, by the two unit tests. The drift-rewrite trap was measured directly on 2026-09-06 |
| Current proof source | `TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit` in `internal/le/commit/snapshot_test.go` now runs the generated script after a peer commit, a foreign staged path and a later edit. It asserts the committed population, peer ancestry, preserved working-tree edit, shared index and drift report. This supersedes the September 6 statement that no end-to-end test existed; no current pass is claimed |
| Done 2026-10-07 | (1) the script regressions run green and are mapped in the Functional Tests table: AC-1, AC-2, AC-3, AC-5 by `TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit`, AC-4 by `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex`, AC-6 by `TestSnapshotRefusesAPathGitStagedNothingFor`. (2) A-2 confirmed with a fail-closed bound and A-3 confirmed with the same-path qualification, see the Assumptions table. Package run: `./le job run label unit-pkg quiet command go test -count=1 ./internal/le/commit/...` exit 0 |
| Observed live 2026-10-07 | commit `c14ec85d74` succeeded and its shared-index repair then failed on `.git/index.lock` held by another process, leaving three paths `MM`. Re-running the two repair lines cleared it. Already journaled twice in `plan/journal/concurrent-session-corruption.md` (2026-10-04, 2026-10-05); not in this spec's ACs, but AC-5 ("no other entry in the shared index moved" and this block's paths show committed) does not hold on that path |
| OWNER DECISION | the Known Limitation "an interloper's edit ALREADY in the working tree when preparation runs is still carried" is a gap against the stated Goal ("the author's care is irrelevant"). The spec cannot close without the owner choosing: accept it as a recorded limitation (goal narrowed to edits after preparation), or scope a follow-up mechanism |
| OWNER DECISION, resolved 2026-10-08 | Accepted as a recorded limitation: the goal is narrowed to edits made after preparation. A foreign edit already in the working tree at preparation stays governed by the git-safety rule (judge foreign hunks against HEAD, carry only what is safe, name whose hunks rode along). No follow-up per-hunk ownership mechanism |
| Remains | closure: `/ze-close` by an independent reviewer. Owed by the main thread: `./le go lint run`, `./le verify worktree` |
| Closed 2026-10-08 | `/ze-close` by an independent reviewer. The `.git/index.lock` failure of the shared-index repair was judged a defect against AC-5 in this spec's own code and fixed (`renderSharedIndexRepair`, bounded wait plus a loud failure with the exact repair). See the closure sections below |

---

## Implementation Summary

### What Was Implemented
- Before this closure: the private index (`renderPrivateIndex`), the preparation-time snapshot (`snapshotIndexEntries`, `checkSnapshot`), the drift note (`renderDriftNote`) and the shared-index repair (`renderSharedIndexRepair`), landed in `a9f2207a3` and `06f6185cb`. The end-to-end script tests and the A-2/A-3 validations landed in `a0226dd2ab`.
- In this closure: `renderSharedIndexRepair` (`internal/le/commit/script.go`) now retries while git's refusal names `.git/index.lock`, once a second, up to `indexLockWaitSecondsMax` (30). On any other failure, or a lock still held at the bound, it stops the script with exit 1, says the commit above landed, lists the paths the shared index still holds stale, and prints the exact repair commands, private index removal included. The repair moved to the block's last step (after the working-tree removal and the approval prune), so a give-up leaves only the shared index to repair.

### Bugs Found/Fixed
- AC-5 did not hold when a peer held `.git/index.lock` after the commit: the repair refused at once and `set -e` ended the script, leaving the block's paths staged in reverse for every session (three live runs, `plan/journal/concurrent-session-corruption.md`, 2026-10-04 and 2026-10-05; a fourth on 2026-10-07, `c14ec85d74`). Covered by `TestTheRepairWaitsForAPeersIndexLock` and `TestARepairThatCannotTakeTheLockNamesThePathsAndTheRepair` (`internal/le/commit/snapshot_test.go`).
- Found in review: with two repair commands joined by `&&`, a trailing `2>&1` captured only the second command's stderr, so a lock refusal of the first was not seen and not retried. Fixed by grouping (`{ ...; } 2>&1`). Discrimination: removing the grouping reddens `TestTheRepairWaitsForAPeersIndexLock` (observed, `-count=1`, then restored from a pristine copy).
- Found in review: the failure message named `$(git rev-parse --short HEAD)`, which names a peer's commit if one landed during the wait. It now points at the line `git commit` printed.

### Documentation Updates
- `docs/contributing/committing.md`, "What the generated script contains": the removal is step 8 (with the approval prune), the shared-index repair is step 9 and states the wait, its bound, the failure message and the printed repair. Anchor: `<!-- source: internal/le/commit/script.go -- renderPrivateIndex, renderWorkingTreeRemoval, renderSharedIndexRepair, indexLockWaitSecondsMax -->`. Step 5's cross-reference moved from step 9 to step 8, and the three test comments citing step 9 for the removal now cite step 8.
- `./le doc check verify`: source anchors, drift and rules stages pass; the run ends FAILED on the command help shape stage (one RPC leaf text of 292 without a summary), which no file in this change touches.

### Deviations from Plan
- The shared-index repair is no longer the step right after `git commit`; it is the block's last step. The plan's Data Flow step 4 described it after the commit; it still is, with the removal and the approval prune now between them.
- The goal is narrowed, by the owner's decision of 2026-10-08, to edits made after preparation (see Current Condition).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first version of the lock fix read the repair entries from the private index, to avoid a race with a peer's commit during the wait | The shared index has to describe HEAD as it stands, or a peer's commit to a shared path reads as staged in reverse; `git ls-tree HEAD` at run time was already right | Review of the draft before any commit | Reverted to `git ls-tree HEAD`, with a comment saying why |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A block commits its own population | Done | `internal/le/commit/script.go` `renderPrivateIndex` | the commit reads `GIT_INDEX_FILE` |
| A block commits its own content | Done | `internal/le/commit/snapshot.go` `snapshotIndexEntries` | blobs fixed at preparation |
| The author's care is irrelevant | Changed | Current Condition, OWNER DECISION resolved 2026-10-08 | narrowed to edits made after preparation |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit` | foreign staged path not carried |
| AC-2 | Done | same test | parent subject is the peer commit |
| AC-3 | Done | same test | prepared content committed, edit on disk, drift NOTE |
| AC-4 | Done | `TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex`, `TestTheRepairWaitsForAPeersIndexLock` | removal committed, shared index clean |
| AC-5 | Done | `TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit`, `TestTheRepairWaitsForAPeersIndexLock`, `TestARepairThatCannotTakeTheLockNamesThePathsAndTheRepair` | now also under a held `.git/index.lock`; a lock past the bound fails loudly with the exact repair |
| AC-6 | Done | `TestSnapshotRefusesAPathGitStagedNothingFor` | missing entry and unrequested quoted entry refused |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| snapshot round-trip and refusal cases | Done | `internal/le/commit/snapshot_test.go` | |
| rendered block shape | Done | `internal/le/commit/commit_test.go` `TestGeneratedBlockQuotesPathsAndCommitsFromItsOwnIndex`, `TestARemovalRendersTheWorkingTreeDeletionAfterTheCommit` | the latter's order list moved the repair after the deletion |
| end-to-end script runs | Done | `internal/le/commit/snapshot_test.go` | the Functional Tests rows, plus the two lock tests |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/commit/script.go` | Done | plus the lock wait in this closure |
| `internal/le/commit/prepare.go` | Done | unchanged in this closure |
| `internal/le/commit/commit_test.go` | Done | |
| `internal/le/commit/snapshot.go` | Done | |
| `internal/le/commit/snapshot_test.go` | Done | |
| `docs/contributing/committing.md` | Done | |

### Audit Summary
- **Total items:** 15
- **Done:** 14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the goal narrowing, owner decision 2026-10-08)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A block commits its OWN population and its own content, for every edit made after preparation | functional (generated script run in a throwaway repository) | `TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit`: a peer commit, a foreign staged path and a later edit of the named path; asserts `git show HEAD:mine.txt` is the prepared content, the population equals the prepared paths, the parent is the peer commit, and the shared index holds only the peer's path. PASS 2026-10-08 |
| The shared index is left describing the commit, even with a peer holding the lock | functional | `TestTheRepairWaitsForAPeersIndexLock` (lock released after 2 s: `git diff --cached` empty) and `TestARepairThatCannotTakeTheLockNamesThePathsAndTheRepair` (lock held: exit 1, paths named, printed repair run as printed leaves `git diff --cached` empty and the private index gone). Both red before the fix with git's `index.lock` refusal |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | the one open item, the pre-preparation interloper, was resolved by the owner on 2026-10-08 as a recorded limitation with no follow-up mechanism | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/commit-stages-in-a-private-index-450bc92b-6ac1-4190-bd40-b427ecba17bf.md` |
| `./le spec review check` | clean: `review_gate: OK (3 code files, clean, hashes match ...)` |
| Rounds | 2 |
| Reviewer lenses used | logic and shell semantics under `set -euo pipefail`, concurrency (peer commit and peer lock during the wait), removed-behavior audit, tests and discrimination, documentation drift, Go style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A peer's `.git/index.lock` after the commit made the repair refuse at once and leave the shared index stale (AC-5) | `internal/le/commit/script.go` `renderSharedIndexRepair` | bounded wait on the lock, loud failure with the exact repair; two tests red before, green after |
| 2 | ISSUE | `A | B && C 2>&1` captured only `C`'s stderr, so a lock refusal of `B` was not retried | same | `{ ...; } 2>&1`; mutation reddens `TestTheRepairWaitsForAPeersIndexLock` |
| 3 | ISSUE | The failure message named HEAD, which can be a peer's commit after the wait | same | the message points at the `git commit` line above |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/commit/snapshot.go` | Yes | `ls -l`: 4029 bytes |
| `internal/le/commit/snapshot_test.go` | Yes | `ls -l`: 33112 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3 | prepared population and content, peer commit kept | `--- PASS: TestTheCommitCarriesThePreparedContentAndNotAConcurrentSessionsEdit` (2026-10-08, `-count=1`) |
| AC-4 | removal committed and the shared index agrees | `--- PASS: TestATwoBlockScriptCommitsEachBlockFromItsOwnIndex`, `--- PASS: TestTheRepairWaitsForAPeersIndexLock` |
| AC-5 | shared index describes the commit, under a held lock too | `--- PASS: TestTheRepairWaitsForAPeersIndexLock`, `--- PASS: TestARepairThatCannotTakeTheLockNamesThePathsAndTheRepair`, `--- PASS: TestABlockLeavesTheSharedIndexAloneWhenItsCommitFails` |
| AC-6 | refusal at preparation naming the path | `--- PASS: TestSnapshotRefusesAPathGitStagedNothingFor` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le commit create` then the generated script | none: tooling, the end-to-end proof is `Create` plus `bash <script>` in `snapshot_test.go` | Yes, every end-to-end test calls `Create` and runs the script it wrote |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `snapshot_test.go` round-trip; every end-to-end test commits through `--index-info` |
| A-2 | confirmed | `--- PASS: TestAPreparedBlobSurvivesAnOrdinaryGC` |
| A-3 | confirmed (qualified for the same path) | `--- PASS: TestAPeerStagedEntryForANamedPathIsResetToTheCommit` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `committing.md` step 9: wait, 30 s bound, failure message, printed repair | `renderSharedIndexRepair`, `indexLockWaitSecondsMax` in `internal/le/commit/script.go` | Yes |
| `committing.md` step 8: removal, then approval prune | `renderBlock` order in `internal/le/commit/script.go` | Yes |
| `docs/features/ai-first.md` (anchor of `prepare.go`) | `prepare.go` unchanged in this closure | No update needed |
