# Quality Standards

Rationale: `.claude/rationale/quality.md`

## Linting

**FIX lint issues. Never disable linters.** Only allowed exclusions: `fieldalignment` (govet), test-file exclusions for `dupl`/`goconst`/`prealloc`/`gosec`.

## Self-Critical Review

**BLOCKING:** All checks must pass before claiming "done." A single failing check = work is not complete.

After each step and before claiming "done":

| Check | Question | Pass? |
|-------|----------|-------|
| Correctness | Actually works? Edge cases? | [ ] |
| Simplicity | Simplest solution? Over-engineered? | [ ] |
| Consistency | Follows existing patterns? | [ ] |
| Completeness | TODOs, FIXMEs, unfinished? | [ ] |
| Quality | Debug statements removed? Errors clear? | [ ] |
| Tests | Cover the change? Any flaky? | [ ] |

Every check must be answered honestly. "Probably fine" is not a pass — run the code, read the diff. If any check fails, fix it before proceeding.

## Critical Reviews

Validate understanding of existing architecture BEFORE proposing changes. Read code first. Check git history.

## Proof

Paste command output as evidence. `make ze-verify` required before claiming done. "Should work" is not evidence.
