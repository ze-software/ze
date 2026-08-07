---
kind: note
level:
stage:
---
`scripts/dev/stress-repro.py <suite>` recreates that pressure cheaply: CPU + GC
"burner" processes oversubscribe every core while many concurrent copies of one
suite loop, and it captures the FIRST failure's complete, untruncated output.
