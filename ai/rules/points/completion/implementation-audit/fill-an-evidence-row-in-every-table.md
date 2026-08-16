---
kind: directive
level: MUST
stage:
---
**EVERY table MUST have at least one evidence row.** `pre_commit_verification_gaps`
(`scripts/dev/commit_helper.py`) checks them one at a time and names the empty
ones on the closure commit. Each table is a separate obligation: a row in
`Files Exist` is not evidence for `AC Verified`.
