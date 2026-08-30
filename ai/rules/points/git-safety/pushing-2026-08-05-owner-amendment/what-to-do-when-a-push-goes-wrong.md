---
kind: directive
level: MUST
stage:
---
**When a push goes wrong, you MUST take the action its row names:**

| Situation | Do |
|-----------|-----|
| The script's commit step failed | Nothing is pushed and nothing SHOULD be: `set -euo pipefail` stops the script before the push. Fix the cause (staged index, GPG), then re-run the script |
| The commits are made and no push was ordered | Stop and report the SHAs. A push nobody asked for is a push without authority, whatever the branch looks like |
| You are a worktree agent | Never push. Work on your branch and stop there (`ai/INSTRUCTIONS.md`) |
| The owner orders a push after your script already ran | Say so and let him push, or carry `push "<owner authorisation>"` on the next commit you prepare. Do not type the command to close the gap |
