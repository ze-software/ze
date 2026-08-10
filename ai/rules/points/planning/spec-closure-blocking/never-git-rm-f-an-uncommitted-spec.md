---
kind: directive
level: MUST
stage:
---
**MUST NOT `git rm -f` a spec without committing it first.** The `-f` flag silently
discards uncommitted edits. If the spec was modified during implementation (it
almost always is), those modifications MUST be committed before deletion.
