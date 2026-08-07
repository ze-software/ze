---
kind: directive
level:
stage:
---
**Never `git rm -f` a spec without committing it first.** The `-f` flag silently
discards uncommitted edits. If the spec was modified during implementation (it
almost always is), those modifications must be committed before deletion.
