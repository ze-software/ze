# Quality Standards

**When:** before presenting any work as complete
**Severity:** blocking

## Directives

Rationale: `ai/rationale/quality.md`

## Linting

**MUST FIX lint issues. MUST NOT disable linters.** Only exclusions: `fieldalignment` (govet), test-file exclusions for `dupl`/`goconst`/`prealloc`/`gosec`.

## Self-Critical Review

The checks that cover the behavior you changed are run once and read. A check that stays red is NAMED in the done-claim, with the one-line reason it is scaffolding rather than a product defect (`ai/rules/pre-release.md`). A tree-wide green is not what "done" means here.

| Check | Question |
|-------|----------|
| Correctness | Actually works? Edge cases? |
| Simplicity | Is this the simplest FULLY CORRECT answer? Name every abstraction, option, layer, and parameter the problem in hand did not need (`ai/rules/simplicity.md`) |
| Modularity | Modified files still one-concern? Line count ok? (rules/file-modularity.md) |
| Consistency | Follows existing patterns? |
| Style | Every loop, queue, retry and cache bounded? Every name says what the value IS? Every lifecycle obligation in a comment? No `panic()` a peer can reach? (`docs/contributing/ze-go-style.md`) |
| Completeness | TODOs, FIXMEs, unfinished? |
| Quality | Debug statements removed? Errors clear? |
| Tests | Cover the change? Any flaky? |

Every check answered honestly. "Probably fine" is not a pass — run the code, read the diff. If any fails, fix before proceeding.

## Adversarial Self-Review (BLOCKING)

**Before presenting any work as complete**, MUST answer these questions. MUST fix what they reveal BEFORE presenting.

| # | Question | If the answer is bad |
|---|----------|---------------------|
| 1 | If a thorough code review ran right now, what would it find? | Fix those things first |
| 2 | What test cases did I skip because they seemed unlikely? | Write them |
| 3 | Is every new function reachable from a user entry point? Name the path. | Wire it or say "not yet wired" |
| 4 | If I doubled the test count, which tests would I add? | Add them now, not after being challenged |
| 5 | Did I ask questions earlier that went unanswered? | List them. Do not silently assume answers and proceed |
| 6 | If I deliberately broke the production code path, would the test catch it? | Re-run after breaking it. Observer-exit antipattern hides this (`ai/rules/testing.md`) |
| 7 | Did I rename a registered name (plugin / subsystem / log / dispatch key)? Did I grep every consumer? | `ai/rules/plugins.md` "Renaming a Registered Name" |
| 8 | Did I add a guard / fallback to a function? Did I check sibling call sites? | `ai/rules/architecture.md` "Sibling Call-Site Audit" |
| 9 | Did I touch reactor concurrency code? Did `go test -race -count=20 ./internal/component/bgp/reactor/...` pass? | `ai/rules/testing.md` "Reactor Concurrency Code" |

**MUST NOT present "version 1" knowing "version 2" is needed.** The first presentation SHOULD be the thorough one.

**Tests passing is not completion.** After tests pass, MUST continue to the next checklist item (docs, audit, journal row). MUST NOT stop at "tests pass" and wait for the user to say "continue." The Completion Checklist has 12 steps -- tests are step 10, not the finish line. MUST NOT stop unless blocked or every step is done.

**Unanswered questions block work.** If a question was asked and not answered, MUST re-state it before proceeding. MUST NOT silently pick an answer and keep going.

## Proof

Paste command output as evidence. "Should work" is not evidence.

The evidence a done-claim owes is the focused test for the behavior you changed, run once. `./le verify worktree` is the FULL gate, it costs 25 to 53 minutes, and it is owed before a push rather than before a done-claim (`ai/rules/pre-release.md`). When you do run it, run it in the foreground and wait: output lands in `tmp/ze-verify.log`, and killing it for being slow wastes the whole run. `ai/rules/precommit-verify.md` carries how to read its red.

Race coverage: `./le verify current mode full` runs `-race` on component groups with changed `.go` files. For reactor concurrency changes, also run `go test -race -count=20 ./internal/component/bgp/reactor/...`.

## Learned Summary Verification

Learned summaries can contain wrong claims about what is "deferred" or "requires X change."
When a summary says something is "deferred because X is missing" or "requires Y change,"
verify the claim against actual code before reporting it to the user. Read the function
signature, check the types. Do not parrot deferred-item descriptions from summaries.

## Critical Reviews

Validate understanding of existing architecture BEFORE proposing changes. Read code first. Check git history.
