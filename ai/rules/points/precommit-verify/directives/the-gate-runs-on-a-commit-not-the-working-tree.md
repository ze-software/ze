---
kind: directive
level: MUST
stage:
---
**The verification gate MUST run against a COMMIT in a throwaway worktree, `./le verify worktree`, and MUST NOT run against the working tree (owner directive, 2026-08-21).** An in-place run is void the moment the tree moves under it, and it never says so: earlier stages judged a tree that no longer exists. A red from such a run MUST NOT be diagnosed as a defect, and its green MUST NOT be cited as evidence.
