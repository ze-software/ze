---
kind: note
level:
stage:
---
That is the whole list. Step 3 (edit `feature-gates.txt`) is the ONLY manifest
declaration point. Everything else follows: the Makefile, the runner, the generators,
dep_audit, and stress-repro all derive from it, and `feature_tags.go` (via
`make generate`) regenerates the three static tag lists. There is nothing to hand-sync.
