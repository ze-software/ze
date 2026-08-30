---
kind: directive
level: MUST NOT
stage:
---
**A push MUST be ordered by the owner and MUST NOT be added on your own initiative; it MUST go through `./le commit create ... push "<owner authorisation>"`, which the generated script performs after every commit succeeds (owner amendment, 2026-08-05).** A bare `git push` from a Bash call stays forbidden and the hook enforces it, `--force` and `-f` are never used, and a worktree agent never pushes at all.
