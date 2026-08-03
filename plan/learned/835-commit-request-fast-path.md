# 835 -- Commit Request Fast Path

## Context

The old `/ze-commit` and `/ze-commit-check` flows treated a user request for a commit as permission to re-open review: scope audits, completeness tables, recent-commit style checks, health checks, and sometimes another full verification run. That is wrong when the implementation phase has already finished. At commit time the user wants the safe user-run script, not another review cycle.

## Decisions

- Chose an explicit fast-path rule in `ai/rules/git-safety.md`: when the user asks for a commit, prepare the helper script immediately and inspect only enough state to avoid staging unrelated, ignored, generated, or out-of-scope paths.
- Chose to remove late completeness and remaining-work tables from `/ze-commit` and `/ze-commit-check`; those checks belong before reporting implementation work, not after a commit request.
- Chose `scripts/dev/verify-status.sh check` as the mandatory gate before any verify rerun. A FRESH result means the current tree is byte-identical to the last passing `ze-verify` run, so rerunning `make ze-verify` or `make ze-verify-changed` is forbidden.
- Kept `/ze-commit` unchecked by design. `/ze-commit-check` still verifies when needed, but only after the freshness check says STALE and `ai/rules/git-safety.md` says verification applies.

## Consequences

- A plain `/ze-commit` produces only the commit scope, message summary, message file path, and script path.
- `/ze-commit-check` reports verification evidence or skip reason, then produces the same helper-generated script.
- Agents must not rerun expensive verification for a byte-identical tree covered by a passing status file.

## Gotchas

- Protecting unrelated work still matters. The fast path may inspect a concise changed-file list, but it must not turn into a full audit.
- A STALE status is not itself a reason to run verify for docs, plans, or agent-only changes covered by the NO row in `ai/rules/git-safety.md`.
- Direct `git add` and `git commit` remain forbidden from AI tool calls. The helper-generated user-run script is still the commit boundary.

## Files

- `ai/rules/git-safety.md`
- `ai/rules/cli.md`
- `ai/skills/ze-commit.md`
- `ai/skills/ze-commit-check.md`
- `ai/INSTRUCTIONS.md`
- `ai/INDEX.md`
- `ai/NAVIGATION.md`
