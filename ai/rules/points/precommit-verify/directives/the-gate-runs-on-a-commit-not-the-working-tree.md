---
kind: directive
level: MUST
stage:
rationale: ai/rationale/precommit-verify.md
---
**The verification gate MUST run against a commit in a throwaway worktree and MUST NOT run against the working tree: an agent's run is `./le verify changed`, and the full verification, `./le verify worktree`, is Thomas's release check, whose exact command an agent gives him.** An in-place run is void the moment the tree moves under it, and it never says so: earlier stages judged a tree that no longer exists. A red from such a run MUST NOT be diagnosed as a defect, and its green MUST NOT be cited as evidence.
