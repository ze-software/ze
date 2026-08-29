---
kind: note
level:
stage:
---
`./le repository tracked-build check` (`internal/le/repository/trackedbuild/repositorytrackedbuild.go`) is the one
check that reads what git holds: it extracts the commit with `git archive` and
compiles six build flavors of the extracted tree. Three rules follow.
