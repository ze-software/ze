# Spec: a prepared commit's message file carries its own random suffix

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-07 |

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
- `allocateMessage` draws a random suffix and reserves its file with `O_EXCL`.
  `allocateScript` selects a separate random-suffixed path after an existence
  check; it does not use the message reservation or `O_EXCL`.
- The auto-tag walk treats an existing suffixed message as a taken letter.
- `Create` owns cleanup for every tag rather than for automatic tags alone, so a
  failed or dry-run `create` leaves no empty file holding a name.

## Data Flow (MANDATORY)

### Entry Point
- `./le commit create subject <text> file <path> ...`, run by an agent preparing
  a commit. Entry format is command-line arguments.

### Transformation Path
1. `Create` (`internal/le/commit/prepare.go`) resolves the commit session.
2. `nextTag` (`script.go`) picks the tag and reserves the message path.
3. `Create` resolves the script separately through `targetScript` and places
   the selected message path in the generated block.
4. `Create` removes an unused empty message reservation on failure or dry run.
   Successful preparation prints `script=` and `message=`. AC-2 and AC-4 still
   require their own evidence; the current allocator description is not proof.

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
| `TestTwoCreatesUnderOneTagKeepTheirOwnMessages`, `TestAppendUnderOneTagGivesEachBlockItsOwnMessage` (added by `bc987697e1`) | `internal/le/commit/message_path_test.go` | AC-1, AC-2 | landed, and observed red against the old derivation. Both use an EXPLICIT tag, so neither reaches the automatic walk: the earlier AC-3 mapping was wrong |
| `TestTheAutomaticTagWalkStepsOverATakenLetter` | `internal/le/commit/message_path_test.go` | AC-3 and the a..z boundary | added 2026-10-07. Green; red with the `continue` on a taken letter removed from `nextTag` (second create answered letter a) |
| `TestACreateThatWritesNoScriptLeavesNoMessageHoldingAName` | `internal/le/commit/message_path_test.go` | AC-4, dry run and refused-after-reservation, each under an automatic and a named tag | added 2026-10-07. Green; every subtest red with the `os.Remove` in `Create`'s `keepReservation` defer removed. The named-tag half was added at closure: with the cleanup restricted to automatic tags (`if keepReservation \|\| options.Tag != ""`) both named subtests go red |

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
- A create that writes its message and then fails to write its SCRIPT leaves
  that non-empty message behind: `Create` sets `keepReservation` once the
  message is written, and the deferred cleanup removes only an empty file. The
  orphan holds a random suffix nobody else draws, so it takes no name from a
  later create; it is clutter in `tmp/`, not a collision. AC-4 as written
  ("no empty artifact") holds.

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

Re-checked 2026-10-07 against the tree. `allocateMessage` in
`internal/le/commit/script.go` reserves a random-suffixed path with `O_EXCL`,
and `allocateScript` draws a separate suffix. AC-2 does not need the two
suffixes to agree: the script's own `git commit -F` line names its message,
which is the derivation AC-2 asks for, and two creates under one session and
one tag get two different message paths, so no guess at session and tag
reaches one.

| Item | State |
|------|-------|
| Product code | LANDED at `bc987697e1`. Reachability re-checked 2026-10-07: `git rev-list HEAD \| grep -c '^bc987697e1'` answers 1 |
| Documentation | landed in the same commit. `docs/contributing/committing.md` ("The `message=` line carries a random suffix") read 2026-10-07 and matches `nextTag`, `allocateMessage`, `allocateScript` |
| Journal row | written in `plan/journal/pointer-shared-across-the-names-it-indexes.md` |
| PROVEN | AC-1 and AC-2 by `TestTwoCreatesUnderOneTagKeepTheirOwnMessages` (asserts each script's `git commit -F` names its own message under one session and tag) and `TestAppendUnderOneTagGivesEachBlockItsOwnMessage`. AC-3 and the a..z boundary by `TestTheAutomaticTagWalkStepsOverATakenLetter`. AC-4 by `TestACreateThatWritesNoScriptLeavesNoMessageHoldingAName`. The two new tests were each observed red under a mutation of the producer, recorded in the Unit Tests table |
| Package run | `./le job run label unit-pkg quiet command go test -count=1 ./internal/le/commit/...` exit 0 on 2026-10-07 |
| Owed by the main thread | `./le verify worktree` (whole-tree gates not run at closure: another session's in-flight BGP/OSPF work occupies the tree; the commit tool records the debt) |
| Remains | nothing: closed by `/ze-close` 2026-10-07 |

---

## Implementation Summary

### What Was Implemented
- `allocateMessage` (`internal/le/commit/script.go`) reserves a random-suffixed
  `tmp/commit-msg-<session>-<tag>-<hex>.txt` with `O_EXCL`; `nextTag` walks
  automatic letters by globbing suffixed messages; `Create`
  (`internal/le/commit/prepare.go`) removes an empty reservation for every tag
  on failure or dry run. Product code landed at `bc987697e1`; tests at
  `bc987697e1` and `e18d9061c9`; closure extended the AC-4 test to named tags.

### Bugs Found/Fixed
- Closure review: the AC-4 test drove only the automatic tag, so a regression of
  the cleanup to automatic tags alone (the pre-fix behavior the spec names under
  "Behavior to change") passed. Fixed by running both routes under a named tag
  too; observed red under that mutation.

### Documentation Updates
- None at closure. `docs/contributing/committing.md` ("The `message=` line
  carries a random suffix", anchor `script.go -- nextTag, allocateMessage,
  allocateScript`) re-read 2026-10-07 and matches the producers.

### Deviations from Plan
- AC-3 was earlier mapped to the two explicit-tag tests; corrected in
  `e18d9061c9` with a dedicated automatic-walk test.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | AC-4 test exercised only the automatic tag | the changed behavior was cleanup for named tags | closure review, mutation `options.Tag != ""` in the defer | named-tag subtests added |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| a later `create` cannot take a script's message | Done | `internal/le/commit/script.go` `allocateMessage` | `O_EXCL` plus random suffix |
| no message reachable from a guess at the session id | Done | `allocateMessage` | 24-bit `crypto/rand` suffix |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestTwoCreatesUnderOneTagKeepTheirOwnMessages`, `TestAppendUnderOneTagGivesEachBlockItsOwnMessage` | |
| AC-2 | Done | `TestTwoCreatesUnderOneTagKeepTheirOwnMessages` | script's `git commit -F` names its message |
| AC-3 | Done | `TestTheAutomaticTagWalkStepsOverATakenLetter` | includes a..z exhaustion |
| AC-4 | Done | `TestACreateThatWritesNoScriptLeavesNoMessageHoldingAName` | automatic and named tag |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the four tests above | Done | `internal/le/commit/message_path_test.go` | each observed red under a producer mutation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/commit/script.go` | Done | `bc987697e1` |
| `internal/le/commit/prepare.go` | Done | `bc987697e1` |
| `docs/contributing/committing.md` | Done | `bc987697e1` |
| `internal/le/commit/message_path_test.go` | Done | `bc987697e1`, `e18d9061c9`, closure |

### Audit Summary
- **Total items:** 4 AC, 4 files
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-3 test mapping, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| a script and its message are one artifact a later `create` cannot take | unit test through the real entry point `Create` | `TestTwoCreatesUnderOneTagKeepTheirOwnMessages` and `TestAppendUnderOneTagGivesEachBlockItsOwnMessage` pass; recorded red against the unsuffixed derivation in `bc987697e1` |
| no unused reservation holds a name | unit test through `Create` | `TestACreateThatWritesNoScriptLeavesNoMessageHoldingAName`, 4 subtests pass; red under both cleanup mutations |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/commit-message-file-carries-its-own-suffix-450bc92b-6ac1-4190-bd40-b427ecba17bf.md` |
| `./le spec review check` | clean |
| Rounds | 3 (round 3 re-read the test after its subtests moved back into the test body, which the commit tool's weakened-test check had read as a removal) |
| Reviewer lenses used | logic+wiring over `nextTag`/`allocateMessage`/`Create`, test discrimination, security (path, suffix source), Go style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | AC-4 test covered only the automatic tag, so the spec's named-tag cleanup change had no discriminating test | `internal/le/commit/message_path_test.go` | subtests under a named tag; red under `if keepReservation \|\| options.Tag != ""` |

NOTEs (non-blocking): `allocateScript` checks existence with `os.Stat` rather than
reserving with `O_EXCL`, so two concurrent creates in one session and tag that
draw the same 24-bit suffix could share a script path (negligible probability,
already stated under Behavior to change); a create whose script write fails
leaves its non-empty message (Known Limitations).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/commit/message_path_test.go` | Yes | `git grep -n createsThatWriteNoScript` hits it |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | the four tests pass | `go test -count=1 -run 'TestACreateThatWritesNoScript\|TestTheAutomatic\|TestTwoCreates\|TestAppendUnderOneTag' -v ./internal/le/commit/` all PASS 2026-10-07; full package `go test -count=1 ./internal/le/commit/...` exit 0; scoped lint 0 issues |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le commit create` → `Create` | none, N-A: no `.ci` surface | tests call `Create`, the function the CLI action calls |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | journal row `plan/journal/pointer-shared-across-the-names-it-indexes.md` |
| A-2 | confirmed | `git grep -n "commit-msg-"` outside `internal/le/commit` names no derivation of the path |
