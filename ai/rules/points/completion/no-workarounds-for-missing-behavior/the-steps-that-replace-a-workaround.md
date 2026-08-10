---
kind: directive
level: MUST
stage:
---
**You MUST follow these steps to replace a workaround:**
1. Name the user goal the missing behavior is meant to satisfy.
2. Trace the code path meant to provide it.
3. Implement the missing behavior at the owning layer.
4. Update affected callers and tests.
5. Verify the user-visible goal directly.
