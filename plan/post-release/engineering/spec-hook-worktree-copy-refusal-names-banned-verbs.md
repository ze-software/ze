# Spec: hook-worktree-copy-refusal-names-banned-verbs

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Agent tooling: a refusal that recommends two banned git verbs steers a worktree agent into the shared checkout, but no shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom (2026-11-12).** The Bash hook refuses a file copy out of a worktree
into the main tree, and its refusal says "Use git merge or cherry-pick". Both
are banned for a worktree agent (`AGENTS.md`, "Worktrees": it never merges,
cherry-picks, rebases or copies into main). The hook refuses `git merge` and
does not refuse `git cherry-pick`, so the message steers the agent onto the
banned verb the hook lets through.

**Cause, read at the producer.** `bashWorktreeCopy`
(`internal/le/hookruntime/bash.go`) returns a literal text whose remedy
predates the worktree rule.

**Goal.** The refusal names the route that works: the agent commits on its own
branch through `./le commit create`, and the user lands that branch on main
with `git rebase <branch>`. It names no banned verb as a remedy. What the
guard refuses does not change.

**Split (2026-11-12).** This was the second defect recorded in
`plan/post-release/engineering/spec-hook-git-guard-misses-index-writers.md`.
It depends on nothing, so it runs alone; the guard gap stays there.

**WIP cap (2026-11-12).** Thomas ordered this fix run at once with 18 specs in
progress against a cap of 12. The claim is taken one above the count for this
spec alone; it is a permission for this work, not a change of the cap.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the doors this work crosses
  → Constraint: a hook that refuses wrongly blocks every session at once; this change touches a refusal's text only, never its condition
- [ ] `ai/rules/user-facing-errors.md` - what a refusal owes its reader
  → Constraint: the problem first, then why it matters, then the command that fixes it on a line of its own, last, with nothing after it
- [ ] `AGENTS.md`, "Worktrees"; `ai/rules/git-safety.md` - the route a worktree agent has
  → Constraint: a worktree agent commits on its own branch through `./le commit create` and never merges, cherry-picks, rebases or copies into main; the user lands the branch with `git rebase <branch>`
- [ ] `.claude/hooks/README.md`, the `bashWorktreeCopy` row - the page that describes the guard
  → Decision: the row says a worktree agent lands its work by committing and names no git verb, so it stays true and is not edited

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/bash.go` - `bashWorktreeCopy`
  → Constraint: the guard fires when the command names `.claude/worktrees` and holds a copy word (`cp`, `mv`, `rsync`, `install`) or a redirect; the verdict is exit 2 with one literal text. Only the text changes
- [ ] `internal/le/hookruntime/refusal_messages_test.go` - the tests that pin refusal advice
  → Decision: the new test lives here, beside the other advice pins, and drives the real `pretool-bash` action through `runHook`
- [ ] `internal/le/ai/hooks/parity_catalog.go` - the golden row `cp .claude/worktrees/a internal/x`
  → Constraint: the row pins exit 2 and no text, so it is untouched and its digest does not move
- [ ] `internal/le/ai/hooks/fixtures.go` - `hookSourcesDigest`
  → Constraint: the digest covers every non-test Go file of `internal/le/hookruntime`, so it is re-sealed with the bytes its test prints, after checking that no other session has a dirty file in the covered packages

**Behavior to preserve:**
- The condition under which a worktree copy is refused, and exit code 2
- The first line of the refusal, which names the problem

**Behavior to change:**
- The remedy the refusal gives

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Worktree copy guard | `internal/le/hookruntime/bash.go` `bashWorktreeCopy` | Refuses a copy out of a worktree with a text that recommends `git merge` or cherry-pick | Extends: the same verdict carries a corrected text. No new guard |
| Commit route advice | `internal/le/hookruntime/bash.go` `bashDestructiveGit` | Names `./le commit create` as the route that works for staging and committing | Extends by reuse of the same route name; the worktree refusal now points at it too |
| Refusal advice pins | `internal/le/hookruntime/refusal_messages_test.go` | Tests that a refusal's advice is the command that works | Extends: one more pin, for this refusal |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A Bash tool call, through the PreToolUse hook `le ai hooks pretool-bash`

### Transformation Path
1. The action reads the `command` string from the payload
2. `bashWorktreeCopy` runs first in the check list and matches a copy out of `.claude/worktrees`
3. It returns exit code 2 and the refusal text. The text is the only thing this spec changes

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ hook | JSON payload in; exit code and refusal text back | No |

### Changed-Wire Census

The exit code and the payload are unchanged. The refusal text changes, and its
only reader is the agent that receives it.

**Search evidence:** the phrases `cherry-pick` and `copying files from
worktree` searched on 2026-11-12 across `internal/le`, `.claude/hooks`,
`docs`, `ai/rules` and `test` (Go, Markdown, `.ci`, JSON). One producer,
`bashWorktreeCopy`; no test, fixture or page holds the text. Control: the
search finds the producer line.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Refusal text of the worktree copy guard | writer | `internal/le/hookruntime/bash.go` `bashWorktreeCopy` | the literal in its `verdict` | Changed: new remedy lines |
| Refusal text of the worktree copy guard | reader | the agent, through the harness, outside the repository | the harness shows the text of an exit 2 hook | No migration: prose read by a model; no program parses it |

### Integration Points
- `bashWorktreeCopy` - the one literal
- `hookSourcesDigest` - re-sealed

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No test or fixture pins the old text | The search in the census | A pinned text goes red and names itself | `./le test unit pkg path ./internal/le/hookruntime/` and `./internal/le/ai/hooks/` after the change | confirmed 2026-11-12: both package runs exit 0 after c775845457, so no pinned text went red |
| A-2 | `./le commit create` is the commit route inside a worktree too | `AGENTS.md`, "git commit, add, rm, mv, and push only through `./le commit`", which makes no exception for a worktree | The advice would name a route that does not work there | Reading `AGENTS.md` and `ai/rules/git-safety.md`: the ban on direct git verbs is unconditional | confirmed 2026-11-12 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The new text itself trips a guard when an agent pastes its last line | The pasted line is `./le commit create ...`, which no guard refuses | The golden row `./le commit no-such-action` already shows `./le commit` lines pass |
| R-2 | Another session edits `bash.go` or a covered package while the digest is sealed | `git status` shows a dirty non-test Go file in a covered package | Seal only after the check; commit by `hunks` if a foreign hunk is unsafe |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A refused worktree copy shows a wrong or unclear remedy. No verdict changes |
| How is it reverted? | One commit reverted by the owner |
| Who else touches this path? | `spec-hook-git-guard-misses-index-writers.md` and `spec-gates-share-one-semantic-population.md` both plan work in `bash.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A Bash call `cp .claude/worktrees/a/x.go internal/x.go` through the `pretool-bash` action | → | `bashWorktreeCopy` | `TestWorktreeCopyRefusalNamesTheRouteThatWorks` (`refusal_messages_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A copy out of a worktree into the main tree | The hook exits 2, and the refusal says a worktree agent commits on its own branch, that the user lands the branch with `git rebase <branch>`, and ends with a `./le commit create` line alone | The first line naming the problem stays as it is: it was correct |
| AC-2 | The same refusal | It names `./le commit create` as the remedy and holds neither "cherry-pick" nor "Use git merge" | `git rebase <branch>` stays in the text as what the USER runs, because that is the rule (`ai/rules/git-safety.md`) |
| AC-3 | A command that names no worktree, or a worktree with no copy | The guard returns no verdict, as today | Every command the guard passes today: its condition is untouched |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWorktreeCopyRefusalNamesTheRouteThatWorks` | `internal/le/hookruntime/refusal_messages_test.go` | AC-1, AC-2: exit 2, the text holds `./le commit create` and `git rebase <branch>`, its last line starts with `./le commit create`, and it holds no "cherry-pick" | |
| `BASH_GOLDEN` row `cp .claude/worktrees/a internal/x` (existing) | `internal/le/ai/hooks/parity_catalog.go` | AC-3's refused half: still exit 2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A: no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestWorktreeCopyRefusalNamesTheRouteThatWorks` | `internal/le/hookruntime/refusal_messages_test.go` | A worktree agent that tries to copy its files into main is told the route that works. The test drives the real hook action, which is the entry point; `./le test unit pkg path ./internal/le/hookruntime/` runs it | |

## Files to Modify
- `internal/le/hookruntime/bash.go` - the refusal text of `bashWorktreeCopy`
- `internal/le/hookruntime/refusal_messages_test.go` - the new test, and its `Related:` header line
- `internal/le/ai/hooks/fixtures.go` - `hookSourcesDigest` re-sealed
- `plan/journal/guard-message-teaches-the-violation.md` - the 2026-11-12 row marked fixed
- `plan/post-release/engineering/spec-hook-git-guard-misses-index-writers.md` - the second defect and its owned row leave it

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | A development hook |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | None added |
| CLI grammar (keyword before value) | N-A | None added |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | No variable |
| Doctor check for runtime dependencies | N-A | No runtime dependency; `le` is not the product |
| Doctor code per problem class | N-A | A development hook's refusal, read by an agent |
| Prometheus counters/metrics | N-A | No observable state |
| BGP family surface | N-A | Not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Development tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | `bash.go`, `refusal_messages_test.go` and `fixtures.go` declare `docs/architecture/core-design.md`; that page describes how le runs native gates, not what a refusal says, so it is unaffected |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `.claude/hooks/README.md` anchors `bash.go` and its `bashWorktreeCopy` row names no git verb, so it stays true. `docs/contributing/running-commands.md` anchors `gitVerbRun`, untouched here |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No page quotes this refusal |

## Implementation Steps

1. **Phase: Red test through the real action** -- the new test, before the text changes
   - Tests: `TestWorktreeCopyRefusalNamesTheRouteThatWorks`
   - Files: `refusal_messages_test.go`
   - Verify: red, because the text holds no `./le commit create`
2. **Phase: The text** -- the literal in `bashWorktreeCopy`, then the digest
   - Files: `bash.go`, `fixtures.go`
   - Verify: `./le test unit pkg path ./internal/le/hookruntime/` and `./internal/le/ai/hooks/` green
3. **Phase: Records** -- the journal row, the skeleton spec
   - Files: the journal shard, `spec-hook-git-guard-misses-index-writers.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 to AC-3 each have a test that fails when broken |
| Correctness | The condition of `bashWorktreeCopy` is byte-identical; only the literal changed |
| Message shape | Problem first, why second, the fixing command alone on the last line (`ai/rules/user-facing-errors.md`) |
| Rule: stale comments | The comment above `bashWorktreeCopy` still matches |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The refusal names the working route | `./le test unit pkg path ./internal/le/hookruntime/ run TestWorktreeCopyRefusalNamesTheRouteThatWorks` |
| No verdict moved | `./le test unit pkg path ./internal/le/hookruntime/`, `./le ai hooks unit` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Guard condition | Any change to when `bashWorktreeCopy` refuses: there must be none |
| Advice safety | The advised command is one every guard allows and that touches only the agent's own branch |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| A pinned text goes red | A-1 is broken: update that pin in the same change and say so |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- A guard's remedy is part of the guard: the rule changed and the refusal kept recommending what the rule now bans

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-message-teaches-the-violation` | 2026-11-12 | `bashWorktreeCopy`: the refusal tells a worktree agent to use `git merge` or `git cherry-pick` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Name `./le commit create` and the user's `git rebase <branch>` | Drop the remedy and state the problem only | A refusal owes the command that fixes it (`ai/rules/user-facing-errors.md`) |
| Fix the text now, alone | Wait and fix it with the guard gap | The text depends on nothing, and it is the one thing that points an agent at the unguarded verb |

## Known Limitations
- `git cherry-pick` itself stays unrefused: `plan/post-release/engineering/spec-hook-git-guard-misses-index-writers.md`

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass, run in the main checkout

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test unit pkg path ./internal/le/hookruntime/`, `./le test unit pkg path ./internal/le/ai/hooks/`, `./le ai hooks unit`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + edited spec + the journal row
- [ ] **Commit B:** the spec removed

## Implementation Summary

### What Was Implemented
- The refusal of `bashWorktreeCopy` (`internal/le/hookruntime/bash.go`) keeps its first line, says why a copy is harmful, names the worktree branch and the user's `git rebase <branch>`, and ends with the `./le commit create` line (c775845457)
- `TestWorktreeCopyRefusalNamesTheRouteThatWorks` pins it through the real `pretool-bash` action; `hookSourcesDigest` re-sealed

### Bugs Found/Fixed
- The refusal recommended `git merge` and cherry-pick, both banned for a worktree agent; covered by the new test, red on all five assertions before the change

### Documentation Updates
- None: `.claude/hooks/README.md`'s `bashWorktreeCopy` row names no git verb and stays true; no page quotes the refusal (search in the census)

### Deviations from Plan
- None

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The refusal names the route that works and no banned verb | Done | `bash.go` `bashWorktreeCopy` | |
| What the guard refuses does not change | Done | `bash.go` `bashWorktreeCopy` | the diff touches only the returned literal |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestWorktreeCopyRefusalNamesTheRouteThatWorks` | exit 2, rebase line, last line the commit route |
| AC-2 | Done | same test | neither "cherry-pick" nor "Use git merge" |
| AC-3 | Done | `git show c775845457 -- internal/le/hookruntime/bash.go`: only the `return` literal changes; `BASH_GOLDEN` row `cp .claude/worktrees/a internal/x` still exit 2 under `./le ai hooks unit` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestWorktreeCopyRefusalNamesTheRouteThatWorks` | Done | `internal/le/hookruntime/refusal_messages_test.go` | red before, green after |
| `BASH_GOLDEN` row `cp .claude/worktrees/a internal/x` | Done | `internal/le/ai/hooks/parity_catalog.go` | unchanged, still exit 2 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/hookruntime/bash.go` | Done | |
| `internal/le/hookruntime/refusal_messages_test.go` | Done | |
| `internal/le/ai/hooks/fixtures.go` | Done | |
| `plan/journal/guard-message-teaches-the-violation.md` | Done | row `fixed:` |
| `plan/post-release/engineering/spec-hook-git-guard-misses-index-writers.md` | Done | defect moved out; at closure its citation of this spec restated by bare stem |

### Audit Summary
- **Total items:** 11
- **Done:** 11
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The refusal names the route that works and no banned verb | test and live hook | `TestWorktreeCopyRefusalNamesTheRouteThatWorks` red on all five assertions before, green after; live on the build of c775845457 a `cp .claude/worktrees/...` line was refused with the new five-line text ending in `./le commit create subject "<subject>" file <path>` |
| The guard's condition does not change | diff and golden row | only the literal moved; `./le ai hooks unit` exit 0 on that build |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none: every item of the Task is done; `git cherry-pick` staying unrefused was never this spec's | - | `plan/post-release/engineering/spec-hook-git-guard-misses-index-writers.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/hook-worktree-copy-refusal-names-banned-verbs-e82b0d47-98d2-4083-b79e-5838e8b12dd5.md` (5 files, verdict clean) |
| `./le spec review check` | clean (run before commit A) |
| Rounds | 1 |
| Reviewer lenses used | round 1: correctness and guard audit (the condition, the verdict, the advice's safety); message shape and docs (`ai/rules/user-facing-errors.md`, the pages that anchor `bash.go`, citers of this spec) |

### Run 1
Scope, written before the round: commit c775845457 whole, which is the
literal in `bashWorktreeCopy` (`internal/le/hookruntime/bash.go`),
`TestWorktreeCopyRefusalNamesTheRouteThatWorks` and the `Related:` line in
`refusal_messages_test.go`, the `hookSourcesDigest` re-seal in `fixtures.go`,
the journal row in `guard-message-teaches-the-violation.md`, the edit to
`spec-hook-git-guard-misses-index-writers.md`, and this spec; plus the sibling
surfaces the text reaches: the `bashWorktreeCopy` row of `.claude/hooks/README.md`,
the `BASH_GOLDEN` row `cp .claude/worktrees/a internal/x`, and `Run`'s joining
of every refusing check's text (`internal/le/hookruntime/runtime.go`).

Result: 0 BLOCKER, 0 ISSUE, 2 NOTE. The condition of `bashWorktreeCopy` is
byte-identical and only the returned literal changed, so no verdict moved; the
golden row still pins exit 2. The text follows `ai/rules/user-facing-errors.md`:
the problem, why, and the fixing command alone on the last line. The advised
command is one no guard refuses and it commits only the named paths on the
caller's own branch. The test drives the real action, asks one question, and
was red on every assertion before the change; `Run` joins every refusing
check's text, and on the test's line no later check refuses, so the last line
is this guard's. The README row stays true, and the only citer that names this
spec by path, the skeleton, is restated by bare stem in commit A.
- NOTE: the command text names `.claude/worktrees`, so the usual caller is a
  main-checkout session copying out of a worktree rather than the worktree
  agent itself. For that caller the remedy is the user's rebase, which line 3
  states; the last line addresses the worktree agent. No change needed.
- NOTE: the test refuses the exact phrase "Use git merge", as AC-2 states it;
  a reworded merge remedy would pass. Test-only, AC-2 met as written.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | - | round 1 found no BLOCKER or ISSUE | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| none to create | N-A | Files to Create is `none`; every modified file is in HEAD (c775845457) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | the new test passes | `./le test unit pkg path ./internal/le/hookruntime/` exit 0 after c775845457 (main thread) |
| AC-3 | no verdict moved | `./le ai hooks unit` and `./le test unit pkg path ./internal/le/ai/hooks/` exit 0 (main thread); `bash.go` lines 66-78 unchanged by the diff |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Bash call through `pretool-bash` | none: the entry point is the hook action, driven by `runHook` | yes, and live through the real hook on the build of c775845457 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | both package runs green after the change |
| A-2 | confirmed | `AGENTS.md` and `ai/rules/git-safety.md`: the ban on direct git verbs has no worktree exception |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `.claude/hooks/README.md`, `bashWorktreeCopy` row | names no git verb; matches the comment above `bashWorktreeCopy` | yes |
| No page quotes the refusal | `git grep` for "merge or cherry-pick" and "copying files from worktree": only `bash.go`, the test comment and this spec | yes |
