---
kind: directive
level: MUST
stage:
rationale: plan/journal/test-gate-repeats-expensive-work.md
---
- A known failing test MUST stay at the narrowest runnable scope until it passes. For Go tests, run `./le job run label unit-pkg command go test PKG=./path/to/package RUN='^TestName$' RACE=0`.
- Use `RACE=0` only for non-race iteration. A race or concurrency failure MUST keep race detection enabled.
- Run the required aggregate target, `./le verify worktree` or `./le verify worktree`, only once. Run it after focused tests pass and all edits are complete. You MUST NOT use either aggregate target to rerun one known failure.
