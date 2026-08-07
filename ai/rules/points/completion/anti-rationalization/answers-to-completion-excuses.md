---
kind: table
level:
stage:
---
| Excuse | Answer |
|--------|--------|
| "Should work" / "Probably fine" | Run it, paste output |
| "Tests passed earlier" | Run again now |
| "Only cosmetic differences" | Show diff, let user decide |
| "Library and interface only" | Feature is not done: library without wiring is dead code |
| "Wiring will be done in next commit" | One commit = code + tests + wiring + summary. No partial deliveries |
| "The .ci test requires infrastructure" | Then the feature is blocked, not done |
| "Unit tests prove it works" | Unit tests prove the algorithm. .ci tests prove the user can reach it |
| "SetAuthorizer is called somewhere" | Show the .ci test where a user command is denied. No test = no proof |
| "Consistent with how other plugins do it" | Other plugins missing tests is a gap, not a precedent |
| "No test infrastructure for this path" | Build the infra or flag as BLOCKER. Never downgrade to NOTE |
| "Out of scope for this review" | Missing coverage is never out of scope. Report as ISSUE |
