---
kind: note
level:
stage:
---
When `make ze-verify` runs on a loaded machine, the failure index may show
`VERIFY FAILURE INDEX (CONTENDED RUN)` with host load details. This means the
system had load > CPU count with concurrent ze-test or go-test processes.
