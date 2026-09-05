---
name: ze-progress
description: Progress
---

# Progress

Check the lifecycle stage of the currently selected spec and recommend the single next action.

Answers: **"Where am I in the spec lifecycle, and what do I do next?"**

See also: `/ze-status` (cross-project dashboard), `/ze-debrief` (session summary), `/ze-audit` (pre-impl), `/ze-review-spec` (post-impl verification), `/ze-commit` (prepare commit)

## Lifecycle Stages

Checked in order. The report stops at the first stage that is NOT satisfied.

| # | Stage | Satisfied when |
|---|-------|----------------|
| 1 | Implementation | Every AC names the producing function, every TDD test exists, every "Files to Modify/Create" entry was touched, and the Wiring Test table rows all have a real `.ci` test |
| 2 | Work Not Done | Every row in this spec's **Work Not Done** table names a spec that exists on disk, and no other spec's table routes unfinished work here. See step 5 for the two-class test |
| 3 | Review | A review (`/ze-review`, `/ze-review-spec`, or `/ze-review-deep`) has run AFTER the most recent spec-related code edit, and every finding was fixed |
| 4 | Commit A | `./le verify worktree` passed AND all spec-scoped changes (code + tests + docs + completed spec file) are committed |
| 5 | Commit B (closure) | A journal row in `plan/journal/<class>.md` naming this spec is committed AND `plan/spec-<name>.md` has been removed via `git rm` in the same commit |

A spec is **done** only when stage 5 is complete. Stages 1 through 4 are checkpoints, not endpoints.

## Steps

1. **Run `./le spec session current`.** If empty: report "No spec selected. Run `/ze-spec` to pick or create one." and stop.
2. **Read `plan/<spec-name>`.** Extract the metadata table (`Status`, `Updated`), the Acceptance Criteria table, the TDD Test Plan, the Files to Modify / Files to Create lists, and the Wiring Test table.
3. **Check git state:** `git status`, `git log --oneline -20`, and `git log --oneline -- plan/<spec-name>`. Record the timestamp of the most recent commit touching any spec-scoped file.
4. **Stage 1 -- Implementation:** Build the AC / TDD / Files table below by checking each row against the code:
   - For every AC row in the spec: grep for the feature or the test that covers it. Mark `Done` only when the producing function exists; otherwise `Partial` or `Missing`.
   - For every TDD test name: `grep -rn "<TestName>" internal/ test/`. Mark `Present` or `Missing`. A renamed test is `Missing` unless the spec lists the new name.
   - For every "Files to Modify" entry: run `git log --oneline -- <file>` to confirm the file was touched during this spec's work window. Missing entry = `Missing`.
   - For every Wiring Test row: check that the named `.ci` or Go test exists and exercises the path.
   - If anything is `Partial` or `Missing`: STAGE = 1. Go to step 9.
5. **Stage 2 -- Work Not Done:** Read this spec's **Work Not Done** table. Then run `grep -rln "spec-<this-stem>" plan/ --include="*.md"` to find the tables that route work here. Two classes hold this stage, and BOTH are checked:
   - **Owed BY this spec:** a row whose destination cell is empty, holds prose, names a path that is not on disk, or names this spec itself. That item is in no count, so nobody schedules it. Write the spec now, in the bucket the item belongs to (`plan/README.md`). Name it by path.
   - **Owed TO this spec:** another spec's **Work Not Done** row names this spec. Someone routed that work here, so it is this spec's to finish whatever its origin.
   - **Why the destination test.** A destination that is prose is a deletion with a polite name. A row pointing at this spec is satisfied by a closure that deletes its own destination. The count is the point. The row mechanism this replaced held 103 live rows, and 29 named no destination, so that work was invisible to every backlog count (`ai/rationale/unfinished-scope-becomes-a-spec.md`).
   - If either class holds: STAGE = 2. Before reporting, grep the repo for the item itself. Code that already exists makes the row a bookkeeping error rather than scope. Go to step 9.
6. **Stage 3 -- Review:** Determine whether a review has been run since the most recent code change:
   - Most recent code change: `git diff --name-only HEAD~1 HEAD` plus any uncommitted changes from `git status`.
   - Most recent review: look for a review artifact in this session (conversation history) or in recent commits/messages mentioning `/ze-review`, `/ze-review-spec`, `/ze-review-deep`.
   - If uncommitted code exists AND no review has run since it was written: STAGE = 3. Go to step 9.
   - If a review ran but reported unresolved BLOCKER or ISSUE items that are not yet fixed: STAGE = 3.
7. **Stage 4 -- Commit A:** Check:
   - Did `./le verify worktree` pass recently? Check `tmp/ze-verify.log` (<1h old) or a documented pass in session state.
   - Are there uncommitted files in the spec scope (code, tests, docs, or the spec file itself)?
   - If uncommitted spec-scoped files remain: STAGE = 4. Go to step 9.
8. **Stage 5 -- Commit B (closure):** Check:
   - Does a committed journal row in `plan/journal/<class>.md` name this spec stem in its Spec column? If not, append a row to the matching class file.
   - Is `plan/spec-<name>.md` still tracked by git (`git ls-files plan/spec-<name>.md`)?
   - If no journal row names this spec OR the spec file is still tracked: STAGE = 5. Go to step 9.
   - Otherwise: STAGE = done.
9. **Report** using the format below.

## Report Format

```
## Progress: <spec-name>

**Stage:** [1-5 / done] -- [Implementation / Work Not Done / Review / Commit A / Commit B / Complete]
**Spec status field:** [skeleton / design / in-progress / blocked / deferred]
**Last spec-related commit:** [sha subject] ([age])

### Implementation
| # | Requirement | Status | Evidence |
|---|-------------|--------|----------|
| AC-1 | <short text> | Done | file.go:42 -- TestFoo |
| AC-2 | <short text> | Missing | - |
| TDD | TestBar | Present | internal/foo/bar_test.go |
| File | internal/foo.go | Touched | commit 7fcc8083 |
| Wire | test/foo/bar.ci | Missing | - |

Counts: N done / M partial / K missing

### Work Not Done
| # | What | Why | The spec that owns it | Already in code? |
|---|------|-----|-----------------------|------------------|
| 1 | ... | ... | `plan/<bucket>/spec-<stem>.md` | yes -- delete the row |

Or: "Every item this spec did not do names a spec that exists."

### Review
- Most recent code edit: [commit sha OR "uncommitted"]
- Most recent review: [command + timestamp OR "none this session"]
- Unresolved findings: [count, or "none"]
- Verdict: [current / due / findings outstanding]

### Commit A (code + completed spec)
- Uncommitted files in spec scope: [list, or "none"]
- `./le verify worktree`: [PASS (Nh ago) / FAIL / not run recently]

### Commit B (closure -- spec -> journal)
- Journal row naming this spec: [present in plan/journal/<class>.md / missing]
- `plan/spec-<name>.md`: [still tracked / removed]

### Next Action
**Run:** `/ze-<command>`
**Why:** [one sentence tying the command to the stage above]
```

## Next Action Decision Table

Pick exactly ONE action based on the reported stage. Do not chain recommendations.

| Stage | Next Action | Rationale |
|-------|-------------|-----------|
| 1 | `/ze-implement` | Finish the missing ACs, tests, or wiring. A unit test in isolation is not wiring -- name the user entry point. |
| 2 (resolvable) | Delete the row: the code is already there | An item that already exists in code is a bookkeeping error, not scope |
| 2 (genuine) | Write the destination spec in the right bucket and name it by path, or ask the owner to drop the item | The item survives this spec's closure as a spec of its own, which is the only form a count can see. Dropping it is the owner's decision, never the author's (`ai/rules/completion.md`). A row that names a spec on disk clears stage 2 |
| 3 | `/ze-review` (or `/ze-review-spec` for conformance, `/ze-review-deep` for exhaustive) | Uncommitted code without a post-edit review is a known failure mode |
| 4 | `/ze-verify` then `/ze-commit` | Commit A must include the completed spec file with its audit tables filled -- this preserves it in git history |
| 5 | Append a journal row to `plan/journal/<class>.md` naming this spec, stage `git rm plan/spec-<name>.md` + the journal file, then `/ze-commit` | Two-commit rule (`ai/rules/planning.md`): never delete a spec without committing it first |
| done | "Spec complete. `/ze-spec` to pick the next one." | Nothing pending |

## Rules

- **Read-only.** Do NOT run `/ze-implement`, `/ze-review`, or `/ze-commit`. Report the stage and recommend the command.
- **One stage at a time.** Stop at the first unsatisfied stage. Do not preview later stages or recommend batched actions.
- **No optimism.** A missing test is Missing, even if "the code obviously works." A missing .ci wiring test means stage 1 is incomplete, even if unit tests pass.
- **Verify before homing.** Before reporting a **Work Not Done** row as stage 2, grep for the item. Code that already exists makes the row `resolvable-now`: recommend deleting the row rather than writing a spec for work that is done.
- **Honest evidence.** Every `Done` row MUST name the producing function or a test name. "Probably done" is `Partial`.
- **Never tick `[ ]` to `[x]`** in the spec file. Checkbox state is not a truth source; grep the code.
- **Do not edit the spec.** If the Status field is wrong, note it in the report but do not change it -- the user decides when to update spec metadata.
- **Respect the two-commit rule.** Stage 4 and stage 5 are separate commits. Never recommend squashing them.
