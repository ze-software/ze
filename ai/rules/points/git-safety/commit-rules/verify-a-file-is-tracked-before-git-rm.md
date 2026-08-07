---
kind: directive
level:
stage:
---
**`git rm` safety:** before using `git rm` in a commit script, verify
the file is tracked (`git ls-files --error-unmatch <file>`). For files
modified during implementation (specs, stubs), use `git rm -f` to avoid
"has local modifications" errors. Never `git rm -f` without first
committing the file's current state (see Spec Closure in planning rules).
