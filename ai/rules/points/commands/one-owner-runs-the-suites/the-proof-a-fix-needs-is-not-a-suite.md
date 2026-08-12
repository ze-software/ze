---
kind: note
level:
stage:
---
This costs an agent almost nothing. The evidence a fix owes is a single-test
mutation: revert the change, watch one named test go red, restore. That is one
`-run` on one package.

A suite count proves the tree, never the fix. It is also the part that does not
survive contention.
