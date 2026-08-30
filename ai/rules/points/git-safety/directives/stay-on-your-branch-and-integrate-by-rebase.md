---
kind: directive
level: MUST NOT
stage:
---
**A branch MUST NOT be changed, created, deleted, renamed or integrated from a tool call: stay on the branch you started on and ask the user to move it.** When the user integrates a worktree branch it lands on main via `git rebase <branch>`, never `git merge`, so history stays linear.
