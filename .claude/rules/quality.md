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
| 1 | If `/ze-review-deep` ran right now, what would it find? | Fix those things first |
| 2 | What test cases did I skip because they seemed unlikely? | Write them |
| 3 | Is every new function reachable from a user entry point? Name the path. | Wire it or say "not yet wired" |
| 4 | If I doubled the test count, which tests would I add? | Add them now, not after being challenged |
| 5 | Did I ask questions earlier that went unanswered? | List them. Do not silently assume answers and proceed |
| 6 | If I deliberately broke the production code path, would the test catch it? | Re-run after breaking it. Observer-exit antipattern hides this (`rules/testing.md`) |
| 7 | Did I rename a registered name (plugin / subsystem / log / dispatch key)? Did I grep every consumer? | `rules/plugin-design.md` "Renaming a Registered Name" |
| 8 | Did I add a guard / fallback to a function? Did I check sibling call sites? | `rules/before-writing-code.md` "Sibling Call-Site Audit" |
| 9 | Did I touch reactor concurrency code? Did `make ze-race-reactor` pass? | `rules/testing.md` "Reactor Concurrency Code" |

**Never present "version 1" knowing "version 2" is needed.** The first presentation should be the thorough one.

**Tests passing is not completion.** After tests pass, continue to the next checklist item (docs, audit, learned summary). Never stop at "tests pass" and wait for the user to say "continue." The Completion Checklist has 12 steps -- tests are step 10, not the finish line. Only stop when blocked or when every step is done.

**Unanswered questions block work.** If a question was asked and not answered, re-state it before proceeding. Do not silently pick an answer and keep going.

## Proof

Paste command output as evidence. "Should work" is not evidence.

**BLOCKING:** `make ze-verify-fast` (timeout 240s) is the ONLY acceptable verification before claiming done. Output auto-captured to `tmp/ze-verify.log`. See `rules/git-safety.md` for the full pre-commit workflow.

Race coverage: `ze-verify-fast` does NOT run `-race` (it lives in `make ze-verify`, the full sequential variant). Run `make ze-verify` before commit when reactor concurrency or any race-sensitive code changed.

## Learned Summary Verification

Learned summaries can contain wrong claims about what is "deferred" or "requires X change."
When a summary says something is "deferred because X is missing" or "requires Y change,"
verify the claim against actual code before reporting it to the user. Read the function
signature, check the types. Do not parrot deferred-item descriptions from summaries.

## Critical Reviews

Validate understanding of existing architecture BEFORE proposing changes. Read code first. Check git history.
