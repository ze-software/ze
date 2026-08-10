---
kind: directive
level: MUST
stage:
---
**`git rm` safety:** before using `git rm` in a commit script, you MUST verify
the file is tracked (`git ls-files --error-unmatch <file>`). For files
modified during implementation (specs, stubs), you MUST use `git rm -f` to avoid
"has local modifications" errors. You MUST NOT `git rm -f` without first
committing the file's current state (see Spec Closure in planning rules).
