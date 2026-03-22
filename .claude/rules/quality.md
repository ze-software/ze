# Quality Standards

Rationale: `.claude/rationale/quality.md`

## Linting

**FIX lint issues. Never disable linters.** Only exclusions: `fieldalignment` (govet), test-file exclusions for `dupl`/`goconst`/`prealloc`/`gosec`.

## Self-Critical Review

**BLOCKING:** All checks must pass before claiming "done."

| Check | Question |
|-------|----------|
| Correctness | Actually works? Edge cases? |
| Simplicity | Simplest solution? Over-engineered? |
| Modularity | Modified files still one-concern? Line count ok? (rules/file-modularity.md) |
| Consistency | Follows existing patterns? |
| Completeness | TODOs, FIXMEs, unfinished? |
| Quality | Debug statements removed? Errors clear? |
| Tests | Cover the change? Any flaky? |

Every check answered honestly. "Probably fine" is not a pass — run the code, read the diff. If any fails, fix before proceeding.

## Adversarial Self-Review (BLOCKING)

**Before presenting any work as complete**, answer these questions. Fix what they reveal BEFORE presenting.

| # | Question | If the answer is bad |
|---|----------|---------------------|
| 1 | If `/deep-review` ran right now, what would it find? | Fix those things first |
| 2 | What test cases did I skip because they seemed unlikely? | Write them |
| 3 | Is every new function reachable from a user entry point? Name the path. | Wire it or say "not yet wired" |
| 4 | If I doubled the test count, which tests would I add? | Add them now, not after being challenged |
| 5 | Did I ask questions earlier that went unanswered? | List them. Do not silently assume answers and proceed |

**Never present "version 1" knowing "version 2" is needed.** The first presentation should be the thorough one.

**Unanswered questions block work.** If a question was asked and not answered, re-state it before proceeding. Do not silently pick an answer and keep going.

## Proof

Paste command output as evidence. "Should work" is not evidence.

**BLOCKING:** `make ze-verify` (timeout 120s) is the ONLY acceptable verification before committing or claiming done. `go test` alone is for development iterations only — never sufficient for commit readiness.

## Critical Reviews

Validate understanding of existing architecture BEFORE proposing changes. Read code first. Check git history.
