---
kind: directive
level: MUST
stage:
---
**`ZE_TEST_NO_BUILD=1` means the run tests whatever `bin/ze` already is.** After
changing daemon source, MUST rebuild before trusting a verdict, otherwise a fixed
bug still "reproduces" against the stale binary. `bin/ze-test <suite> <test>`
once (no `ZE_TEST_NO_BUILD`) rebuilds both binaries.
