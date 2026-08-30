---
kind: directive
level: SHOULD
stage:
---
**Each artifact SHOULD stay within its budget. No gate measures these: they are the standard a review applies, and the number to quote when a document is over:**

| Artifact | Contains | Budget |
|----------|----------|--------|
| Subagent report to the main thread | the conclusion, the evidence that would overturn it, open questions | under 40 lines |
| Review finding | the claim, where it lives, how it fails | 3 lines |
| Commit subject | what changed, imperative | one line |
| Commit body | the defect, its cause, what the fix does | under 15 lines |
| Known-failure shard | the failing output, the repro command, the next step | under 20 lines |
| Learned summary | what the code cannot tell a future reader | 25 to 35 lines (`ai/rules/planning.md`) |
| Index or pointer line | what the target answers | under 120 characters after the link |
| Rule file | trigger, directives, one example for each | under 150 lines. Above that, move the reference tables to `docs/` and link |
