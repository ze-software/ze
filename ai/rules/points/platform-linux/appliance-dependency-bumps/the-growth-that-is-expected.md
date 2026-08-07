---
kind: directive
level:
stage:
---
**Expected.** Superseded versions after a pin bump (runbook step 5 tells you to
`rm -rf` the old dir; do it, or every bump leaves 15-50 MB behind), and the breadth
of `go mod download all` (`mk/gokrazy.mk`), which is the whole module graph
including test-only deps and their fixtures: `pierrec/lz4` is 75 MB of `testdata/`,
`klauspost/compress` 46 MB. A second Go toolchain also lands here
(`golang.org/toolchain@...`, ~310 MB with its zip) whenever a builddir `go`
directive is newer than the host toolchain and `GOTOOLCHAIN=auto`.
