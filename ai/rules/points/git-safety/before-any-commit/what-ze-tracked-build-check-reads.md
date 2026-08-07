---
kind: note
level:
stage:
---
`make ze-tracked-build-check` (`scripts/checks/tracked_build.go`) is the one
check that reads what git holds: it extracts the commit with `git archive` and
compiles six build flavors of the extracted tree. Three rules follow.
