---
kind: note
level:
stage:
---
Then prepare the user-run commit script listing ONLY this session's files
explicitly in `commit_helper.py create --file ...`, so the commit never pulls in
another session's working-tree edits; exclude other sessions' files when
reviewing `git diff`. This applies only when the global red is not yours -- a red
caused by your own change must be fixed, not scoped around. Activate it only on an
explicit owner direction (e.g. "another session is clearing ze-verify, check only
what we changed"), never inferred from a red suite alone.
