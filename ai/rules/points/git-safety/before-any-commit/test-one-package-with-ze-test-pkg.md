---
kind: note
level:
stage:
---
`make ze-test-pkg PKG=<pattern>` is the supported way to test ONE package while
you develop it. It carries all of the above. Add `RUN=<regexp>` to narrow, and
`RACE=0` to drop `-race` while iterating -- but a package tested without `-race`
has not been tested the way `ze-verify` tests it, so put it back before the end.
