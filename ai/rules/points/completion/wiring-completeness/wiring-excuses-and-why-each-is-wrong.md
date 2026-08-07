---
kind: table
level:
stage:
---
| Pattern | Why it's wrong |
|---------|----------------|
| "The caller will wire it later" | Later never comes. Other sessions see it as done. |
| "It's available for callers" | Available is not wired. No caller means no effect. |
| "The review said NOTE" | Reviews must flag unwired code as BLOCKER. |
| "The web UI doesn't need it" | If the feature produces data that a UI page renders, the UI must show it. |
