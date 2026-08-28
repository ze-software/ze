---
kind: note
level:
stage:
---
This is the same trap as bare `go test` above, one layer out: the invocation is
accepted and the failure looks like the code under test. `test/plugin/cos-external-warns.ci`
cost an hour of bisecting innocent changes this way; it passed in 2.0s through
the native action (`plan/learned/HOOK-FRICTION.md` F17).
