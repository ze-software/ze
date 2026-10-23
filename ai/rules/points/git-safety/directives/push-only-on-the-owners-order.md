---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/git-safety.md
---
**A push MUST be ordered by the owner and MUST NOT be added on your own initiative; it MUST go through `./le commit push "<owner authorisation>"`, which writes a push-only script that re-checks the verification debt and pushes the verified commit by SHA.** A bare `git push` from a Bash call stays forbidden and the hook enforces it, `--force` and `-f` are never used, and a worktree agent never pushes at all.
