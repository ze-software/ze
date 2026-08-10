---
kind: directive
level: MUST NOT
stage:
---
**Several agents work this checkout at once. `make ze-verify` reads the WORKING
TREE, so it reads their half-finished edits too, and a fully green run is
unreachable by construction. You MUST NOT wait for one: it is a strategy that cannot terminate.**
