---
kind: directive
level: MUST
stage:
---
**When the user integrates a worktree branch manually, it MUST land on main via `git rebase <branch>`, never `git merge`.** History stays linear.
