---
kind: directive
level: MUST
stage:
---
**The pre-commit gate MUST run against a COMMIT in a throwaway worktree, never against the working tree (owner directive, 2026-08-21).** `make ze-verify-worktree` does it: it adds a detached worktree at the commit, runs `ze-precommit-verify` there, and removes it on every exit path. `COMMIT=<rev>` picks the commit and defaults to HEAD; `KEEP=1` leaves the tree for inspection when it goes red.
