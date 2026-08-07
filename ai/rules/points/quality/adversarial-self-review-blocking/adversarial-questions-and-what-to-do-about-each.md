---
kind: table
level:
stage:
---
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
| 9 | Did I touch reactor concurrency code? Did `make ze-race-reactor` pass? | `ai/rules/testing.md` "Reactor Concurrency Code" |
