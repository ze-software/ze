---
kind: directive
level: MUST
stage:
---
**When `./le verify worktree` is known-red from failures this session did not
cause, a commit carrying NO Go MUST be gated on changed scope alone.** Re-running
the full gate for it re-surfaces other sessions' noise that is not your
regression. **A commit carrying Go still owes the run named above: a known red is
ATTRIBUTED there and is never a reason to skip it.**
