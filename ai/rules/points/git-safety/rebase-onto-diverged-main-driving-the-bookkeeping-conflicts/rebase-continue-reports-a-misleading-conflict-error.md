---
kind: note
level:
stage:
---
Gotcha: `git rebase --continue` refuses with a MISLEADING "You must edit all
merge conflicts" whenever there are unstaged tracked changes, not only when
index entries are unmerged (`builtin/rebase.c` ACTION_CONTINUE checks
`has_unstaged_changes()`). Read `git status` for the unstaged tracked files and
stage or discard them; the message names conflicts you do not have.
Per "Branch Changes Are Forbidden" above, the AI never runs `git rebase`
itself; the script only resolves conflicts within a rebase the user started.
