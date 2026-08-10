---
kind: note
level:
stage:
---
Test binaries take the same shape one level in -- a private `bin/` subdir of a
throwaway directory under `$(ZE_SCRATCH_DIR)` -- because `.ci` tests exec them by
bare name and an isolated `etc/ze` is what a test wants
(`mk/test-functional.mk`, `internal/test/sessionpath`).
