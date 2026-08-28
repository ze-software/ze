---
kind: note
level:
stage:
---
Then prepare the commit script with `./le commit create`, listing ONLY this
session's files as repeated `file <path>` pairs, so the commit never pulls in
another session's working-tree edits. Exclude other sessions' files when
reviewing `git diff`. This applies only when the global red is not yours -- a red
caused by your own change must be fixed, not scoped around. Activate it only on an
explicit owner direction (e.g. "another session is clearing ./le verify current mode full, check only
what we changed"), never inferred from a red suite alone.
