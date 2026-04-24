# Progress

Check the lifecycle stage of the currently selected spec and recommend the single next action.

Answers: **"Where am I in the spec lifecycle, and what do I do next?"**

See also: `/ze-status` (cross-project dashboard), `/ze-recap` (session summary), `/ze-audit` (pre-impl), `/ze-review-spec` (post-impl verification), `/ze-commit` (prepare commit)

## Lifecycle Stages

Checked in order. The report stops at the first stage that is NOT satisfied.

| # | Stage | Satisfied when |
|---|-------|----------------|
| 1 | Implementation | Every AC has file:line evidence, every TDD test exists, every "Files to Modify/Create" entry was touched, and the Wiring Test table rows all have a real `.ci` test |
| 2 | Deferrals | No `open` row in `plan/deferrals.md` references the spec, OR every open row is explicitly user-approved to remain open |
| 3 | Review | A review (`/ze-review`, `/ze-review-spec`, or `/ze-review-deep`) has run AFTER the most recent spec-related code edit, and every finding was fixed |
| 4 | Commit A | `make ze-verify` passed AND all spec-scoped changes (code + tests + docs + completed spec file) are committed |
| 5 | Commit B (closure) | `plan/learned/NNN-<spec-stem>.md` is committed AND `plan/spec-<name>.md` has been removed via `git rm` in the same commit |

A spec is **done** only when stage 5 is complete. Stages 1 through 4 are checkpoints, not endpoints.

## Steps

1. **Read `tmp/session/selected-spec`.** If empty or missing: report "No spec selected. Run `/ze-spec` to pick or create one." and stop.
2. **Read `plan/<spec-name>`.** Extract the metadata table (`Status`, `Updated`), the Acceptance Criteria table, the TDD Test Plan, the Files to Modify / Files to Create lists, and the Wiring Test table.
3. **Check git state:** `git status`, `git log --oneline -20`, and `git log --oneline -- plan/<spec-name>`. Record the timestamp of the most recent commit touching any spec-scoped file.
4. **Stage 1 -- Implementation:** Build the AC / TDD / Files table below by checking each row against the code:
   - For every AC row in the spec: grep for the feature or the test that covers it. Mark `Done` only if a file:line exists; otherwise `Partial` or `Missing`.
   - For every TDD test name: `grep -rn "<TestName>" internal/ test/`. Mark `Present` or `Missing`. A renamed test is `Missing` unless the spec lists the new name.
   - For every "Files to Modify" entry: run `git log --oneline -- <file>` to confirm the file was touched during this spec's work window. Missing entry = `Missing`.
   - For every Wiring Test row: check that the named `.ci` or Go test exists and exercises the path.
   - If anything is `Partial` or `Missing`: STAGE = 1. Go to step 9.
5. **Stage 2 -- Deferrals:** Read `plan/deferrals.md`. Filter rows where `Source` references the spec filename (or a task from this spec) AND `Status` is `open`.
   - If any open row exists: STAGE = 2. Before reporting, apply the **Verify Before Deferring** rule (`ai/rules/deferral-tracking.md`): grep the repo for the deferred thing. If it already exists, flag the deferral as resolvable-now. Go to step 9.
6. **Stage 3 -- Review:** Determine whether a review has been run since the most recent code change:
   - Most recent code change: `git diff --name-only HEAD~1 HEAD` plus any uncommitted changes from `git status`.
   - Most recent review: look for a review artifact in this session (conversation history) or in recent commits/messages mentioning `/ze-review`, `/ze-review-spec`, `/ze-review-deep`.
   - If uncommitted code exists AND no review has run since it was written: STAGE = 3. Go to step 9.
   - If a review ran but reported unresolved BLOCKER or ISSUE items that are not yet fixed: STAGE = 3.
7. **Stage 4 -- Commit A:** Check:
   - Did `make ze-verify-fast` pass recently? Check `tmp/ze-verify.log` (<1h old) or a documented pass in session state.
   - Are there uncommitted files in the spec scope (code, tests, docs, or the spec file itself)?
   - If uncommitted spec-scoped files remain: STAGE = 4. Go to step 9.
8. **Stage 5 -- Commit B (closure):** Check:
   - Does `plan/learned/NNN-<spec-stem>.md` exist? Compute the next `NNN` from `ls plan/learned/ | sort | tail -1` if it does not.
   - Is `plan/spec-<name>.md` still tracked by git (`git ls-files plan/spec-<name>.md`)?
   - If the learned summary is missing OR the spec file is still tracked: STAGE = 5. Go to step 9.
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

Or: "No open deferrals reference this spec."

### Review
- Most recent code edit: [commit sha OR "uncommitted"]
- Most recent review: [command + timestamp OR "none this session"]
- Unresolved findings: [count, or "none"]
- Verdict: [current / due / findings outstanding]

### Commit A (code + completed spec)
- Uncommitted files in spec scope: [list, or "none"]
- `make ze-verify`: [PASS (Nh ago) / FAIL / not run recently]

### Commit B (closure -- spec -> learned)
- `plan/learned/NNN-<stem>.md`: [present / missing (next NNN = ###)]
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
| 2 (resolvable) | Close the deferral: implement it or mark `done` in `plan/deferrals.md` | A deferral that already exists in code is a bookkeeping bug, not scope |
| 2 (genuine) | Ask user: implement now, move to another spec, or drop with `user-approved-drop` | Open deferrals cannot silently survive spec closure |
| 3 | `/ze-review` (or `/ze-review-spec` for conformance, `/ze-review-deep` for exhaustive) | Uncommitted code without a post-edit review is a known failure mode |
| 4 | `/ze-verify` then `/ze-commit` | Commit A must include the completed spec file with its audit tables filled -- this preserves it in git history |
| 5 | Write `plan/learned/NNN-<stem>.md`, stage `git rm plan/spec-<name>.md` + the new learned file, then `/ze-commit` | Two-commit rule (`ai/rules/spec-preservation.md`): never delete a spec without committing it first |
| done | "Spec complete. `/ze-spec` to pick the next one." | Nothing pending |

## Rules

- **Read-only.** Do NOT run `/ze-implement`, `/ze-review`, or `/ze-commit`. Report the stage and recommend the command.
- **One stage at a time.** Stop at the first unsatisfied stage. Do not preview later stages or recommend batched actions.
- **No optimism.** A missing test is Missing, even if "the code obviously works." A missing .ci wiring test means stage 1 is incomplete, even if unit tests pass.
- **Verify before deferring.** Before reporting an open deferral as stage 2, grep for the thing being deferred. If it already exists in code, flag it as `resolvable-now` and recommend closing it.
- **Honest evidence.** Every `Done` row MUST have a `file:line` or a test name. "Probably done" is `Partial`.
- **Never tick `[ ]` to `[x]`** in the spec file. Checkbox state is not a truth source; grep the code.
- **Do not edit the spec.** If the Status field is wrong, note it in the report but do not change it -- the user decides when to update spec metadata.
- **Respect the two-commit rule.** Stage 4 and stage 5 are separate commits. Never recommend squashing them.
