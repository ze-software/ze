---
kind: directive
level: MUST
stage:
---
**LOAD IS NEVER AN EXPLANATION. IT IS THE BUG.** A test that passes on a quiet host and fails on a busy one is a BROKEN TEST: load did not break it, load REVEALED that it asserts on elapsed time instead of on state. Naming the host's load is therefore the diagnosis, not an excuse and not a non-deterministic hatch, so you MUST fix the test rather than record it.
**You MUST find what the test waits ON and make it wait for that thing.** Poll the condition, or wait on the readiness signal the daemon emits, and ADD that signal when none exists, because a missing one is a product gap. Raising a timeout only moves the load level at which the test lies. `checkLoadExcuses` (`internal/le/doc/wiring/docwiring.go`) fails a changed `plan/known-failures/` shard carrying "passes in isolation" or any of its synonyms.
