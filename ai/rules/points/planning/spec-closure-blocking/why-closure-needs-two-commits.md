---
kind: directive
level: MUST
stage:
---
**Closure MUST use TWO commits, ONE script.** The spec is edited during implementation (design notes,
status updates, corrected assumptions). Those edits are valuable design history.
`git rm` destroys the working copy. If the edited spec is never committed before
deletion, the design work is lost from git history forever.
