---
kind: directive
level: MUST
stage:
---
- **A no-build stress reproduction tests the isolated binary set it was given, so after changing daemon source you MUST rebuild before trusting its verdict**, otherwise a fixed bug still "reproduces" against the stale binary. Run the owning `./le functional <suite>` action once; `internal/le/functional.Prepare` rebuilds the isolated daemon and runner pair.
- **A flake MUST NOT be hunted by looping `./le functional` or `./le verify worktree`**: use `./le stress-repro` against the suspected suite.
