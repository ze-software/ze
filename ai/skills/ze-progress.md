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
| 2 | Deferrals | Every LIVE row (Status not `done`/`cancelled`/`resolved`) that this spec still owes is homed at a spec that exists. A live row homed ELSEWHERE is not owed and does not hold the gate: see step 5 for the exact two-class test |
| 3 | Review | A review (`/ze-review`, `/ze-review-spec`, or `/ze-review-deep`) has run AFTER the most recent spec-related code edit, and every finding was fixed |
| 4 | Commit A | `./le verify worktree` passed AND all spec-scoped changes (code + tests + docs + completed spec file) are committed |
| 5 | Commit B (closure) | A journal row in `plan/journal/<class>.md` naming this spec is committed AND `plan/spec-<name>.md` has been removed via `git rm` in the same commit |

A spec is **done** only when stage 5 is complete. Stages 1 through 4 are checkpoints, not endpoints.

## Steps

1. **Run `./le spec-session current`.** If empty: report "No spec selected. Run `/ze-spec` to pick or create one." and stop.
2. **Read `plan/<spec-name>`.** Extract the metadata table (`Status`, `Updated`), the Acceptance Criteria table, the TDD Test Plan, the Files to Modify / Files to Create lists, and the Wiring Test table.
3. **Check git state:** `git status`, `git log --oneline -20`, and `git log --oneline -- plan/<spec-name>`. Record the timestamp of the most recent commit touching any spec-scoped file.
4. **Stage 1 -- Implementation:** Build the AC / TDD / Files table below by checking each row against the code:
   - For every AC row in the spec: grep for the feature or the test that covers it. Mark `Done` only when the producing function exists; otherwise `Partial` or `Missing`.
   - For every TDD test name: `grep -rn "<TestName>" internal/ test/`. Mark `Present` or `Missing`. A renamed test is `Missing` unless the spec lists the new name.
   - For every "Files to Modify" entry: run `git log --oneline -- <file>` to confirm the file was touched during this spec's work window. Missing entry = `Missing`.
   - For every Wiring Test row: check that the named `.ci` or Go test exists and exercises the path.
   - If anything is `Partial` or `Missing`: STAGE = 1. Go to step 9.
5. **Stage 2 -- Deferrals:** Read every shard under `plan/deferrals/` (one file per source). A row is LIVE when its `Status` is not `done`, `cancelled`, or `resolved`. Two classes of live row hold this stage, and BOTH must be checked -- scan every shard, not only `plan/deferrals/<spec-stem>.md`:
   - **Owed TO this spec:** any live row whose `Destination` names this spec. Someone routed that work here. It is this spec's to finish, wherever it came from.
   - **Owed BY this spec:** a live row whose `Source` names this spec, whose `Destination` does NOT name some OTHER `plan/spec-*.md` that exists on disk. Unhomed, or pointing at prose, or pointing back at this spec.
   - **Why the destination test, and why "some OTHER".** Status alone never terminates: homing a row is the correct resolution and leaves the row LIVE, so a spec with a homed row would re-enter stage 2 forever and could never be reported closeable. Without "OTHER", a row homed at this very spec reads as satisfied, and closure then deletes its destination. Status alone also under-fires: measured 2026-08-03, 127 live rows carry `deferred` and only 12 carry `open`, so a filter on `open` misses most of the corpus.
   - If any such row exists: STAGE = 2. Before reporting, apply the **Verify Before Deferring** rule (`ai/rules/planning.md`): grep the repo for the deferred thing. If it already exists, flag the deferral as resolvable-now. Go to step 9.
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

**Stage:** [1-5 / done] -- [Implementation / Deferrals / Review / Commit A / Commit B / Complete]
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

### Deferrals
| # | What | Reason | Destination | Verify-before-deferring |
|---|------|--------|-------------|-------------------------|
| 1 | ... | ... | spec-bar.md | already implemented -- close as done |

Or: "No deferral is owed by this spec: every live row is homed elsewhere."

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
| 2 (resolvable) | Close the deferral: implement it or mark `done` in its `plan/deferrals/<source>.md` shard | A deferral that already exists in code is a bookkeeping bug, not scope |
| 2 (genuine) | Ask user: implement now, move to another spec, or drop with `user-approved-drop` | A deferral cannot silently VANISH at closure. It may survive it: a row homed at another spec stays live, and its shard outlives this spec (`ai/rules/planning.md`). Homing it clears stage 2 -- do not treat a live homed row as unfinished closure work |
| 3 | `/ze-review` (or `/ze-review-spec` for conformance, `/ze-review-deep` for exhaustive) | Uncommitted code without a post-edit review is a known failure mode |
| 4 | `/ze-verify` then `/ze-commit` | Commit A must include the completed spec file with its audit tables filled -- this preserves it in git history |
| 5 | Append a journal row to `plan/journal/<class>.md` naming this spec, stage `git rm plan/spec-<name>.md` + the journal file, then `/ze-commit` | Two-commit rule (`ai/rules/planning.md`): never delete a spec without committing it first |
| done | "Spec complete. `/ze-spec` to pick the next one." | Nothing pending |

## Rules

- **Read-only.** Do NOT run `/ze-implement`, `/ze-review`, or `/ze-commit`. Report the stage and recommend the command.
- **One stage at a time.** Stop at the first unsatisfied stage. Do not preview later stages or recommend batched actions.
- **No optimism.** A missing test is Missing, even if "the code obviously works." A missing .ci wiring test means stage 1 is incomplete, even if unit tests pass.
- **Verify before deferring.** Before reporting a live deferral as stage 2, grep for the thing being deferred. If it already exists in code, flag it as `resolvable-now` and recommend closing it.
- **Honest evidence.** Every `Done` row MUST name the producing function or a test name. "Probably done" is `Partial`.
- **Never tick `[ ]` to `[x]`** in the spec file. Checkbox state is not a truth source; grep the code.
- **Do not edit the spec.** If the Status field is wrong, note it in the report but do not change it -- the user decides when to update spec metadata.
- **Respect the two-commit rule.** Stage 4 and stage 5 are separate commits. Never recommend squashing them.
