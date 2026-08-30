---
kind: directive
level: MUST
stage:
---
**A test that passes on a quiet host and fails on a busy one is a BROKEN TEST. Load did not break it: load revealed that it asserts on elapsed time instead of on state. You MUST fix the test so load cannot reach it.**
