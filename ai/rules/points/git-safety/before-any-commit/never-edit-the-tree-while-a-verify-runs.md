---
kind: directive
level: MUST NOT
stage:
---
**You MUST NOT edit the tree while a verify runs**, yours or anybody's. Regenerating an
index or touching a rule mid-run invalidates every stage that already read it, and
the failures it produces look exactly like real ones. Measured: one such run
reported five failing stages, all five self-inflicted, and none reproduced on the
settled tree.
