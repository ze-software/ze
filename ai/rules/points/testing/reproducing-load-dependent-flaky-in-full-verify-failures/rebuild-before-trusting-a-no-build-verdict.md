---
kind: directive
level: MUST
stage:
---
**A no-build stress reproduction tests the isolated binary set it was given.** After
changing daemon source, MUST rebuild before trusting a verdict, otherwise a fixed
bug still "reproduces" against the stale binary. Run the owning
`./le functional <suite>` action once; `internal/le/functional.Prepare` rebuilds
the isolated daemon and runner pair.
