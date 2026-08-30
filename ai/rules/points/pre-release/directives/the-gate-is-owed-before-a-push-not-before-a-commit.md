---
kind: directive
level: MUST NOT
stage:
---
**A commit MUST NOT be held for a green gate.** A local commit reaches nobody, so it can cost nobody anything, and it protects finished work from the next crash or checkout.
**The gate is owed before a PUSH, which is the act that reaches a reader** (`ai/rules/git-safety.md` carries the verification-debt route). Verification debt is a record of what a push still owes, and it MUST NOT be read as a reason to stop committing.
