---
kind: note
level:
stage:
---
That is the whole list. Step 3 (edit `feature-gates.txt`) is the ONLY manifest
declaration point. The native runner, `internal/le/pluginimports`,
`internal/le/featuretags`, tier checks, and stress tooling all derive from it.
`./le repository generate` refreshes every generated consumer. There is nothing to hand-sync.
