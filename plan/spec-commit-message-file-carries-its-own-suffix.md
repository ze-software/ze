# Spec: a prepared commit's message file carries its own random suffix

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | implemented and landed at `bc987697e1`; closure sections unwritten |
| Handoff | - |
| Updated | 2026-09-06 |

<!-- Backfilled. The work was commissioned straight from a journal row and
     skipped the spec step, so this records what the work IS, what its evidence
     proves, and what it still owes. Status is in-progress rather than ready:
     the product code exists, so the spec is past design, and closure has not
     run. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`./le commit create` writes two artifacts per commit: a script and a message
file. The script path drew a random suffix and the message path did not, so a
second `create` under one tag wrote a second script while overwriting the first
script's message. Both scripts stayed runnable, and running the first made its
commit under the second one's subject with nothing printed to say so.

Evidence is in `plan/journal/pointer-shared-across-the-names-it-indexes.md`,
three occurrences, one of which put the subject `x` on `main`. The commit
message of `bc987697e1` carries the mechanism. Neither is restated here.

Goal: a script and its message are ONE artifact. Neither can be taken by a later
`create`, and neither can be reached from a guess at the session id.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` - the commit route and the artifacts it writes
  → Constraint: the printed `script=` line is the only authoritative path, so an
    author who loses it re-runs `create` to recover it. That is the trigger, and
    it is ordinary rather than careless.
- [ ] `ai/rules/git-safety.md` - the native commit route
  → Decision: the message file is part of the prepared commit, not a scratch file,
    so it takes the same identity discipline the script takes.

**Key insights:**
- The defect lives in the disagreement between two derivations of one identity.
  A test over either derivation alone cannot see it, so the test drives `Create`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/script.go` - `nextTag` (`:264`), `allocateMessage` (`:300`), `allocateScript` (`:323`)
- [ ] `internal/le/commit/prepare.go` - `Create`, which owns both artifacts and their cleanup
- [ ] `docs/contributing/committing.md` - the published contract for the two paths

**Behavior to preserve:**
- The printed `script=` line stays the authoritative path an author copies.
- Automatic tag allocation still walks letters in order.

**Behavior to change:**
- `allocateMessage` draws a random suffix and creates its file with `O_EXCL`,
  exactly as `allocateScript` does.
- The auto-tag walk reserves a letter by globbing the suffixed names rather than
  by creating an unsuffixed file.
- `Create` owns cleanup for every tag rather than for automatic tags alone, so a
  failed or dry-run `create` leaves no empty file holding a name.

## Data Flow (MANDATORY)

### Entry Point
- `./le commit create subject <text> file <path> ...`, run by an agent preparing
  a commit. Entry format is command-line arguments.

### Transformation Path
1. `Create` (`internal/le/commit/prepare.go`) resolves the commit session.
2. `nextTag` (`script.go`) picks the tag and allocates both artifact paths.
3. `allocateMessage` and `allocateScript` each draw a random suffix and create
   their file with `O_EXCL`.
4. `Create` prints `script=` and `message=`, and removes both on failure.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `le` process ↔ the shell that runs the script | the generated `.sh` path, printed | Yes, by the tests in `message_path_test.go` |
| Session ↔ session, over `tmp/` | eight-hex commit namespace plus a random suffix | Yes, no second `create` can take a name |

### Integration Points
- `internal/le/lepath` answers the commit session both artifacts are keyed on.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | both paths come from one allocator family in `script.go` |
| No unintended coupling | Yes | the change is confined to `internal/le/commit` |
| No duplicated functionality | Yes | `allocateMessage` takes `allocateScript`'s shape rather than a second one |
| Zero-copy preserved where applicable | N-A | tooling path, not an encoder |
| Registration over hardcoding | N-A | no registry involved |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An author who loses the printed path re-runs `create` rather than reconstructing it | the journal row records exactly that | the fix removes a collision nobody meets | the journal row's own measurement | confirmed |
| A-2 | No caller outside `internal/le/commit` derives the message path | grep over the tree at the time of the fix | an external caller breaks | grep | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stale unsuffixed message file from before the fix is left in `tmp/` | a `tmp/commit-msg-*` name with no suffix | harmless: nothing reads it, and `Create` no longer writes it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a commit lands under another commit's subject, silently |
| How is it reverted? | single commit revert; `bc987697e1` touches five files |
| Who else touches this path? | every session in this checkout prepares its commits through it |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| two `./le commit create` runs under one session and one tag | → | `nextTag` and `allocateMessage` (`internal/le/commit/script.go`) | the two `Create`-driving tests in `internal/le/commit/message_path_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | two `create` runs under one session and one tag | each run owns a distinct message file; neither run's message is rewritten |
| AC-2 | a script path is known | the message path that script commits is derivable from it, and from nothing else |
| AC-3 | automatic tag allocation runs while a suffixed script already exists | the walk sees the taken letter and picks the next one |
| AC-4 | `create` fails, or runs as a dry run | no empty artifact is left holding a name |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the two `Create`-driving tests added by `bc987697e1` | `internal/le/commit/message_path_test.go` | AC-1, AC-3 | landed, and observed red against the old derivation |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| automatic tag letter | a..z | z | N/A | N/A, `create` reports exhaustion |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| none | - | the commit route has no `.ci` surface. `Create` IS the entry point an agent calls, and the unit tests drive it | N-A |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | - | - | tooling, no wire-visible behavior | N-A |

## Files to Modify
- `internal/le/commit/script.go` - `nextTag`, `allocateMessage`
- `internal/le/commit/prepare.go` - cleanup ownership in `Create`
- `docs/contributing/committing.md` - the two artifact paths

## Files to Create
- `internal/le/commit/message_path_test.go` - the two `Create`-driving tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | development tooling, no operator config |
| CLI commands/flags | No | `./le commit create` keeps its verbs and its flags |
| Functional test for new RPC/API | N-A | no RPC |
| Doctor check for runtime dependencies | N-A | no new file path, socket or binary |
| Env var registration | N-A | none added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/contributing/committing.md`, edited in `bc987697e1` |
| 16 | Any changed source file referenced by existing doc anchors? | Yes | `docs/contributing/committing.md` is the `// Design:` anchor of `script.go`, and it was updated. `docs/features/ai-first.md` is the `// Design:` anchor of `prepare.go` and is UNAFFECTED: it describes the per-session commit identity, and this change alters neither that identity nor any behavior the page names. Only cleanup ownership inside `Create` moved |
| 1,2,4-15,17 | - | No | no operator-facing surface changed |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - write the two tests against `Create` and
   observe them red against the unsuffixed derivation.
   - Files: `internal/le/commit/message_path_test.go`
   - Verify: red for the stated reason, not for a setup fault.
2. **Phase: Allocation** - give `allocateMessage` the suffix and the `O_EXCL`
   creation `allocateScript` has; move the auto-tag reservation onto a glob.
   - Files: `internal/le/commit/script.go`
3. **Phase: Cleanup** - `Create` removes both artifacts for every tag.
   - Files: `internal/le/commit/prepare.go`
4. **Phase: Documentation** - state the two paths in `docs/contributing/committing.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC has a test that drives `Create`, never a path helper alone |
| Correctness | a failed or dry-run `create` leaves no file behind |
| Data flow | one place derives both paths, so they cannot disagree again |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the two derivations agree | `go test ./internal/le/commit/` |
| no unsuffixed message file is written | `ls tmp/commit-msg-*` after a `create` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path traversal | the suffix is hex from `crypto/rand`, and `lepath` validates the session component |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | re-read `nextTag` in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- A defect living in the disagreement between two derivations of one identity is
  invisible to a test over either derivation. The test must drive the caller
  that uses both.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The message draws its own suffix | derive the message name from the script name | one allocator shape for both; a second derivation is what caused the defect |
| Auto-tag reserves by globbing suffixed names | keep creating an unsuffixed placeholder | the placeholder was itself an unsuffixed shared name, so it carried the same defect |

## Known Limitations
- The class this belongs to, a shared artifact whose identity omits the writer,
  is not closed here. Its ledger half is
  `plan/spec-ledger-shards-per-commit-session.md` and its index half is
  `plan/spec-commit-stages-in-a-private-index.md`.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
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

| Item | State |
|------|-------|
| Product code | LANDED at `bc987697e1`. Reachability checked: `git rev-list HEAD \| grep -c '^bc987697e1'` answers 1 |
| Documentation | landed in the same commit |
| Journal row | written in `plan/journal/pointer-shared-across-the-names-it-indexes.md` |
| PROVEN | AC-1 and AC-3. Both tests drive `Create` and both were observed red against the old derivation |
| ASSERTED, not proven | AC-2 and AC-4. They are read off the code and the commit message; no test names either one |
| Remains | a test for AC-4 (a failed and a dry-run `create` leave no artifact), a test for AC-2, and the closure sections |
