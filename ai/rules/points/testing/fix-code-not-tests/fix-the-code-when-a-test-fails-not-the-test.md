---
kind: directive
level: MUST NOT
stage:
---
**When a test fails, the CODE MUST be fixed to make it pass, and the test's
expectations MUST NOT be weakened or simplified to match broken code.** Tests are
ground truth. When an underlying mechanism changes (Unix sockets replaced by SSH,
for instance), the expectations stay and the replacement mechanism satisfies
them.
