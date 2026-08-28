---
kind: note
level:
stage:
---
`internal/le/functional/binaries.go` builds an isolated bare-named pair into the
session scratch directory. The daemon carries the test-only tag set, and the
suite runs with `ZE_TEST_NO_BUILD=1`, `ZE_BIN`, and `ZE_TEST_BIN` set to that
pair. A directly launched runner can rebuild a daemon without the test-only
surface, so a fixture can time out for a build-population error.
