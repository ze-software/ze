---
kind: table
level:
stage:
---
| Session | Skill | Commits it produces |
|---------|-------|---------------------|
| Implementation, any model | `/ze-implement` | ONE commit: code, tests, docs, and the spec at `Status: verification`. Then `./le spec session release`, report the SHA, stop |
| Review and closure, Opus 5 | `/ze-close` | commit A (journal row, spec, closure edits) and commit B (`git rm` the spec), after a Review Gate over that committed diff |
