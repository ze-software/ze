---
kind: note
level:
stage:
---
Test binaries take the opposite trade-off -- a private `bin/` subdir under
`tmp/s/<id>/` -- because `.ci` tests exec them by bare name and an isolated
`etc/ze` is what a test wants (`mk/test-functional.mk`, `internal/test/sessionpath`).
