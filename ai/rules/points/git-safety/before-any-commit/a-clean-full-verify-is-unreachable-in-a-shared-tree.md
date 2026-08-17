---
kind: directive
level: MUST NOT
stage:
---
**Several agents work this checkout at once. `make ze-precommit-verify` reads the WORKING
TREE, so it reads their half-finished edits too, and a fully GREEN run is
unreachable by construction. You MUST NOT wait for one and you MUST NOT re-run for one: what is unreachable is the green bar, never the run.**
