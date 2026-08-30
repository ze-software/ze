---
kind: directive
level: MUST NOT
stage:
---
**The commit script MUST list ONLY this session's files, as repeated `file
<path>` pairs, so the commit never pulls in another session's working-tree
edits**, and other sessions' files MUST be excluded when reviewing `git diff`.
**This route MUST NOT be taken for a red your own change caused, which is fixed
rather than scoped around, and it MUST NOT be inferred from a red suite alone.**
It needs an explicit owner direction naming the other session's clearing run.
