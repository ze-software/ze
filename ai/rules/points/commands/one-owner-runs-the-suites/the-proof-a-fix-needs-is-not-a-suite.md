---
kind: directive
level: MUST NOT
stage:
---
**The evidence a fix owes is a single-test mutation, and a suite count MUST NOT
be offered in its place.** Revert the change, watch one named test go red,
restore. That is one `-run` on one package, and it costs an agent almost
nothing. A suite count proves the tree, never the fix, and it is the part that
does not survive contention.
