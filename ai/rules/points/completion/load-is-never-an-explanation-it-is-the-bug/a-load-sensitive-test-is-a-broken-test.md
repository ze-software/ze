---
kind: note
level:
stage:
---
A test that passes on a quiet host and fails on a busy one is a BROKEN TEST. Load
did not break it. Load revealed that it asserts on elapsed time instead of on
state. **Fix the test so load cannot reach it.** Owner directive, 2026-07-26.
