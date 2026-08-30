---
kind: directive
level: MUST NOT
stage:
---
**A commit MUST NOT carry a lint issue its own diff caused**: lint costs seconds
and says the tree is broken. **The test evidence a commit owes is the focused
test for what it changed, run once.** A test that stays red is named in the
commit body, and it does not hold the commit (`ai/rules/pre-release.md`).
