---
kind: note
level:
stage:
---
`./le job run label unit-pkg command go test PKG=<pattern>` is the supported way to test ONE package while
you develop it. It carries all of the above. Add `RUN=<regexp>` to narrow, and
`RACE=0` to drop `-race` while iterating -- but a package tested without `-race`
has not been tested the way `./le verify current mode full` tests it, so put it back before the end.
