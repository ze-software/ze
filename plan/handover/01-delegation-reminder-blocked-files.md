# 01 -- Land the two files held back from `7dbdc1bbf`

**Status:** blocked on a concurrent session, not on a decision.
**Owner action needed:** none. Pick this up when the contended files are free.

## Rationale

- `7dbdc1bbf` added `.claude/hooks/delegation-reminder.sh`, a `UserPromptSubmit`
  hook that repeats the standing request to delegate. Background:
  `plan/learned/1306-delegation-reminder-position.md`.
- Two files that belong with that commit stayed out of it. Both carried a
  concurrent session's uncommitted hunks, and `git add` stages whole files, so
  including either one commits their unfinished work.
- The result is a hook in HEAD with **no committed test**, and a documentation
  row that still describes a dead gate as live.
- The prose gate makes this worse in one direction: `ste_problems`
  (`scripts/dev/commit_helper.py:1473`) compares each committed file against its
  own HEAD version, so a colleague's habit growth in a shared file blocks YOUR
  commit of that file. Recorded as `F-ste-3` in `plan/learned/HOOK-FRICTION.md`.

## Work outstanding

| # | File | What is in the working tree, uncommitted | Why it matters |
|---|------|------------------------------------------|----------------|
| 1 | `scripts/dev/hook-fixture-check.py` | A `delegation-reminder` section (`run_delegation_reminder` plus its `SECTIONS` entry) with 7 assertions, and a docstring section list replaced by a pointer to `--help` | HEAD has the hook and NOT its test. `git show HEAD:scripts/dev/hook-fixture-check.py \| grep -c delegation-reminder` returns 0 |
| 2 | `ai/rules/hook-mapping.md` | Rows for `verify-claim-reminder.sh` and `delegation-reminder.sh` in the Session Lifecycle table, plus the corrected `block-premature-stop.sh` row | `ai/rules/discovery-updates.md` names this file as the required discovery artifact for a new hook. Its row still calls the dead hook a live BLOCKING gate |
| 3 | `ai/INDEX.md:201` | NOT YET EDITED | The `spec-closure-check.py` row still says "Backs the Stop-hook closure gate". That hook is registered on no event. This is the tenth stale surface, and the other nine are corrected |

## Steps

1. Confirm the contention is gone: `git status --short ai/rules/hook-mapping.md
   scripts/dev/hook-fixture-check.py ai/INDEX.md`. If another session still holds
   hunks there, stop and wait. Check ownership with `git diff -U0 <file>` and
   compare hunk line numbers against the ones named above.
2. Make the `ai/INDEX.md:201` edit. Replace "Backs the Stop-hook closure gate."
   with a statement that the Stop hook is registered on no event since
   `41e5fa44f` (2026-06-29). Say that only the commit-time reminder in
   `scripts/dev/commit_helper.py` consumes the detector.
3. Restore the fixture citation. `ai/rules/spec-delegation.md`'s
   `delegation-reminder.sh` Enforcement bullet had its
   `Fixtures: ... --only delegation-reminder` sentence REMOVED before
   `7dbdc1bbf`, because that section was not in the committed tree. Once item 1
   lands, add it back.
4. Regenerate the digest. `ai/rules/hook-mapping.md` and
   `ai/rules/spec-delegation.md` are both rules, so `ai/rules/CONDENSED.md` is
   owed in the same commit. Build it from HEAD plus your own edited rules per
   `ai/rules/rule-format.md:67-72`, never with a plain `make ze-rules-condensed`
   while any other session has an uncommitted rule.
5. Verify: `make ze-hook-test`, `python3 scripts/dev/rules_lint.py`, and
   `python3 scripts/dev/ste_check.py --check <each committed file>`.

## What this closes

`ai/rules/CONDENSED.md` currently disagrees with itself about
`block-premature-stop.sh`. Its hook-mapping section calls the hook a live
BLOCKING gate. Its spec-delegation and planning sections call it inert.
`CLAUDE.md` imports the digest eagerly, so every session holds both statements.
Item 2 removes that contradiction. `plan/learned/1306-delegation-reminder-position.md`
discloses it in the meantime, and points readers at the `Stop` array in
`.claude/settings.json`, which settles it.

Delete this file in the commit that completes the last item.
