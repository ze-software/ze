---
kind: note
level:
stage:
---
Gotcha: `git rebase --continue` refuses with a MISLEADING "You must edit all
merge conflicts" whenever there are unstaged tracked changes, not only when
index entries are unmerged (`builtin/rebase.c` ACTION_CONTINUE checks
`has_unstaged_changes()`). rebase_learned.py detects this and names the real
files. The behavior is pinned by `TestRebaseContinueMessageIsMisleading` in
`scripts/dev/rebase_learned_test.py`, so believe the test, not the message.
Per "Branch Changes Are Forbidden" above, the AI never runs `git rebase`
itself; the script only resolves conflicts within a rebase the user started.
