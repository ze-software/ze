# Spec: a prepared commit stages in a private index over a snapshot

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | `plan/spec-ledger-shards-per-commit-session.md` (both touch `internal/le/commit/prepare.go`) |
| Phase | product code landed `a9f2207a3` and `06f6185cb`; end-to-end proof and closure outstanding |
| Handoff | - |
| Updated | 2026-09-06 |

<!-- Backfilled. The work was commissioned straight from a journal row and
     skipped the spec step. Status is in-progress: the product code exists in
     the working tree, nothing has closed, and the landing is what it owes
     first. -->

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
| A-2 | The object database keeps the snapshot blob alive between preparation and the run | blobs are unreferenced until the commit, so `gc --prune=now` could collect them | a script prepared long ago fails at `update-index` with an unknown object | not validated by a test | UNVALIDATED |
| A-3 | Repairing the shared index for this block's paths cannot destroy another session's staged entry | the repair names only `block.Paths` | a peer's staged path is reset | asserted from the rendered text, no test | UNVALIDATED |

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
| an end-to-end run in a throwaway repository, with a foreign path staged | not written | an agent commits while a peer holds the index | MISSING. See "What Remains" |

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
- The end-to-end run of a generated script is untested. See "What Remains".

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

**The change is complete and LANDED.** The product code, the two unit tests and
the `docs/contributing/committing.md` reconciliation went in as `a9f2207a3` on
2026-09-06, and `06f6185cb` followed with the drift note fix. A fresh clone and
this tree now agree about how committing works. What this spec still owes is the
end-to-end proof, not the landing.

| Item | State |
|------|-------|
| Product code | `internal/le/commit/snapshot.go` (new), `script.go`, `prepare.go`. Landed `a9f2207a3`, with `06f6185cb` on top |
| Documentation | `docs/contributing/committing.md` reconciled with the new contract in `a9f2207a3` and `06f6185cb` |
| Journal row | written, `plan/journal/concurrent-session-corruption.md`, seventh occurrence |
| PROVEN | the snapshot round-trip and the rendered block shape, by the two unit tests. The drift-rewrite trap was measured directly on 2026-09-06 |
| ASSERTED, not proven | AC-1, AC-2 and AC-5. No test runs a generated script against a repository with a foreign path staged, so the property the whole change exists for is read off the rendered text rather than observed. A-2 and A-3 are unvalidated |
| Remains | (1) the end-to-end script run that proves AC-1, AC-2 and AC-5; (2) validate A-2 and A-3; (3) the residual gap in Known Limitations, which needs a decision rather than a repair; (4) closure sections |
