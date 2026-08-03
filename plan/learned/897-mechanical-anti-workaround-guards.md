# 897 -- mechanical-anti-workaround-guards

## Context

The user was tired of a recurring agent failure: when a test went red or a check
rejected an input, the agent would weaken the test (skip it, drop assertions,
change the expected value) or work around the problem (rename a command to dodge
a validation gap) instead of fixing the source. The repo already had strong prose
rules (`testing.md`, `completion.md`) but they
were enforced only by in-the-moment discipline, which is exactly what fails under
completion pressure. The goal was to convert those rules from hope into mechanical
guards at the points where the failure actually happens.

## Decisions

- Enforce at three distinct points rather than one, because the failure happens at
  three moments: as the edit is typed (hook), when the diff is reviewed (review
  pass), and before a fix is even chosen (diagnosis gate). Chose layered guards
  over a single check because each catches what the others miss.
- Broadened the existing `c_test_deletion_edit` hook into `c_test_weakening` over
  adding a new check, to keep the `CHECKS` tuple stable and the parity golden
  intact. Detects skip-injection, partial assertion removal, `require`->`assert`
  downgrade, comment-out, `ignore` build tag, and covers `Write`/`MultiEdit`.
- Escape hatch is a documented, greppable token (`// test-relax: <reason>`) over a
  silent allow, so legitimate relaxations leave an audit trail
  (`grep -rn 'test-relax:'`).
- The review audit (`scripts/dev/audit-test-relaxation.py`) IMPORTS the hook's
  `_test_weakening_errs` via importlib over reimplementing it, so the edit-time
  guard and the review pass can never drift apart.
- Deliberately did NOT detect expected-value-in-place mutation (`Equal(t,1,x)` ->
  `Equal(t,2,x)`): too many false positives on legitimate golden updates. Documented
  it as the one weakening the hook cannot see; the deep-review Agent 4 prompt covers
  it manually instead.

## Consequences

- A failing test can no longer be silently neutered: the hook blocks it at edit
  time, and the review surfaces anything that slipped past (relax token, out-of-band
  commit) across a whole branch.
- New canonical rule `completion.md` changes the success criterion for any
  fix from "symptom gone" to "root cause traced to file:line and fixed at the owning
  layer", enforced in `/ze-debug` and `/ze-implement`.
- The hook detection logic is now load-bearing for two consumers (the hook and the
  audit). Changing the regex set means re-running `hook-parity-check.py` (locks the
  hook) AND `audit-test-relaxation.py --selftest` (locks the audit).
- `ai/rules/INDEX.md` is generated; a new rule requires `make ze-rules-index`, and
  any `ai/skills/*.md` edit requires `make ze-ai-sync` (the `.claude`/`.codex`/
  `.agents` copies are gitignored, so drift never shows in `git diff`).

## Gotchas

- `hook-parity-check.py` runs the hook in a FRESH temp `CLAUDE_PROJECT_DIR`, so any
  hook code that imports relative to `PROJECT_DIR` would fail-open and silently
  no-op the whole hook in the parity harness. Hook imports must resolve relative to
  the hook file's own dir, not the project dir.
- In the parity harness almost every `internal/*.go` edit returns exit 2 from
  `c_pre_write_go` (no session state in the temp dir), which masks every other
  check. To isolate `c_test_weakening` in a test, put the fixture under `pkg/`, not
  `internal/`.
- Importing the hook module by path for reuse is safe ONLY because its `main()` is
  guarded by `if __name__ == "__main__"`; module-level code is side-effect-free.
- `git diff <base>` never shows untracked files, so brand-new test files do not
  appear in the audit (correct: a new test cannot be a weakening).

## Files

- `.claude/hooks/pretool-writeedit.py` -- `c_test_deletion_edit` -> `c_test_weakening` (broadened) + `_test_weakening_errs` helper
- `scripts/dev/hook-parity-check.py` -- WEAKEN_CASES + WEAKEN_GOLDEN corpus (10 new cases)
- `scripts/dev/audit-test-relaxation.py` -- new branch/diff audit, reuses hook logic, `--selftest`
- `ai/rules/completion.md` -- new BLOCKING rule
- `ai/rules/testing.md` -- broadened to cover weakening + escape token
- `ai/rules/repo-maintenance.md` -- updated check entry
- `ai/rules/INDEX.md` -- regenerated
- `ai/skills/ze-review.md`, `ze-review-deep.md` -- wired in the audit pass
- `ai/skills/ze-debug.md`, `ze-implement.md` -- wired in the diagnosis gate
