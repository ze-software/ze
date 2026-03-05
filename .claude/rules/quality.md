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

## Proof

Paste command output as evidence. "Should work" is not evidence.

**BLOCKING:** `make ze-verify` (timeout 120s) is the ONLY acceptable verification before committing or claiming done. `go test` alone is for development iterations only — never sufficient for commit readiness.

## Critical Reviews

Validate understanding of existing architecture BEFORE proposing changes. Read code first. Check git history.
