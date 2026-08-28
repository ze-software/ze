---
kind: directive
level: MUST
stage:
---
**The pre-commit gate MUST run against a COMMIT in a throwaway worktree, never against the working tree (owner directive, 2026-08-21).** `./le verify worktree` adds a detached worktree at HEAD, runs every native verification stage, and removes it on every exit path. `commit <revision>` selects another commit; `keep` leaves the tree for inspection when it goes red.
