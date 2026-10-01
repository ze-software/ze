---
kind: directive
level: MUST NOT
stage:
excepted-by: git-safety/directives/pull-before-the-first-edit
---
**A branch MUST NOT be changed, created, deleted, renamed or integrated from a tool call, except for the same-branch upstream sync required by "Pull before the first edit": stay on the branch you started on and ask the user to move it.** When the user integrates a worktree branch it lands on main via `git rebase <branch>`, never `git merge`, so history stays linear.
