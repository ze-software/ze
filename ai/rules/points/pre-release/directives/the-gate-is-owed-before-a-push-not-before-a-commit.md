---
kind: directive
level: MUST NOT
stage:
---
**A commit MUST NOT be held for a green gate.** A local commit reaches nobody, so it can cost nobody anything, and it protects finished work from the next crash or checkout.
**The gate is GitHub CI's, on the pushed commit, and no agent runs it locally** (`ai/rules/git-safety.md` carries the push route). The push debt is the person acts a push still owes, derived from git and the commit trailers, and it MUST NOT be read as a reason to stop committing.
