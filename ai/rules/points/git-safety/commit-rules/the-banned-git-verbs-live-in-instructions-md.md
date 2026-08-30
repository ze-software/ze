---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/git-safety.md
---
**`git commit`, `git add`, `git rm`, `git restore --staged` and `git stash` MUST NOT be invoked as a direct Bash tool call.** `ai/INSTRUCTIONS.md` carries the ban into every session, so it is not restated here.
**The same verbs inside the generated commit script are ALLOWED.** The ban is on the direct tool call, not on what the script does when you run it. `docs/contributing/committing.md` describes what the command writes and what it refuses.
