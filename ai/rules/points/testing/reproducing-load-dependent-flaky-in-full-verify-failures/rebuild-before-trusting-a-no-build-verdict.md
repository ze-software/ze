---
kind: directive
level:
stage:
---
**`ZE_TEST_NO_BUILD=1` means the run tests whatever `bin/ze` already is.** After
changing daemon source, rebuild before you trust a verdict, otherwise a fixed
bug still "reproduces" against the stale binary. `bin/ze-test <suite> <test>`
once (no `ZE_TEST_NO_BUILD`) rebuilds both binaries.
