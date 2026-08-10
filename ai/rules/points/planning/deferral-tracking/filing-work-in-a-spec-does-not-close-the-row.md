---
kind: directive
level: MUST NOT
stage:
---
**Filing work in a spec MUST NOT be treated as a close.** Moving work into a spec gives the row its
Destination; the row then stays `deferred` until the work lands. This table's
predecessor said "moved to another spec -> `done`", which read as "filing closes the
row" and cost real coverage: 13 rows were closed on filing in one session, hiding
their work from the gate while none of it had been done. If the work is not in the
tree, the row MUST NOT be `done`.
