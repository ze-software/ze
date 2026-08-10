---
kind: directive
level: MUST
stage:
---
- **A bare `git push` from a Bash call stays forbidden; the hook enforces it.**
- **You MUST push only by passing `--push` to `scripts/dev/commit_helper.py create` (step 2); it runs from the script you run at step 7.**
- **The owner orders a push; you MUST NOT decide one yourself. `--push` on your own initiative is a push without authority.**
- **`git push --force` and `-f` stay forbidden; `--push` is no route to them.**
