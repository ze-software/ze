---
kind: directive
level: MUST
stage:
---
**A test in the scoped proof that passes on a quiet host and fails on a busy one MUST be diagnosed and fixed.** Host load does not turn a failure into a pass or satisfy an acceptance criterion. Record the observed failure and its cause; the record does not replace the fix.
**You MUST find what the test waits ON and make it wait for that thing.** Poll the condition, or wait on the readiness signal the daemon emits, and ADD that signal when none exists, because a missing one is a product gap. Raising a timeout only moves the load level at which the test lies. `checkLoadExcuses` (`internal/le/doc/wiring/docwiring.go`) fails a changed `plan/known-failures/` shard carrying "passes in isolation" or any of its synonyms.
