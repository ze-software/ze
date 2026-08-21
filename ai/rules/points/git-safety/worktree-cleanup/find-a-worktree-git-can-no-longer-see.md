---
kind: directive
level: MUST
stage:
---
**A worktree audit MUST read the `.git` file of every directory under `.claude/worktrees/` and test that the gitdir path it names exists; `git worktree list` alone MUST NOT be treated as the answer.** A repository that moved leaves each worktree pointing at the old path, so git reports a clean tree while the checkout and its disk remain.
