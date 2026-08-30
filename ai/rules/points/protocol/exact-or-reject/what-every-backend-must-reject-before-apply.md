---
kind: directive
level: MUST
stage:
---
**Before a backend or translator spec is marked done, every path that accepts config and writes state MUST satisfy all of these:**
- Every accepted config path MUST produce backend state matching EXACTLY. No approximation
- Every capacity, limit, and bound MUST be checked in the verifier BEFORE Apply time
- Every narrowing numeric input MUST have an explicit range check naming the valid range
- Every name subject to truncation MUST reject when it would truncate, so distinct inputs never become the same stored name
- A not-yet-implemented feature MUST reject with a "deferred" message, never a quiet ignore
