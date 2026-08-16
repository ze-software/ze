---
kind: note
level:
stage:
---
This is a MANUAL discipline. `make ze-mutation-test` / `ze-mutation-test-changed` (gomu,
see "Mutation Testing" below) mutates Go source and runs only `go test` UNIT tests: it
never executes `.ci`/`.et`, so it cannot catch a functional false-pass. Nothing else in
the pipeline does either.
