---
kind: note
level:
stage:
---
Never commit with lint issues your own diff caused: lint costs seconds and says the tree is broken. Test evidence is the focused test for what you changed, run once. A test that stays red is named in the commit body, and it does not hold the commit (`ai/rules/pre-release.md`).
