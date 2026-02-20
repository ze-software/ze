# Quality Standards

Rationale: `.claude/rationale/quality.md`

## Linting

**FIX lint issues. Never disable linters.** Only allowed exclusions: `fieldalignment` (govet), test-file exclusions for `dupl`/`goconst`/`prealloc`/`gosec`.

## Self-Critical Review

After each step and before claiming "done":

| Check | Question |
|-------|----------|
| Correctness | Actually works? Edge cases? |
| Simplicity | Simplest solution? Over-engineered? |
| Consistency | Follows existing patterns? |
| Completeness | TODOs, FIXMEs, unfinished? |
| Quality | Debug statements removed? Errors clear? |
| Tests | Cover the change? Any flaky? |

## Critical Reviews

Validate understanding of existing architecture BEFORE proposing changes. Read code first. Check git history.

## Proof

Paste command output as evidence. `make ze-verify` required before claiming done. "Should work" is not evidence.
