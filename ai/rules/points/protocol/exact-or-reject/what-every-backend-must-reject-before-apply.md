---
kind: directive
level:
stage:
---
- [ ] Every accepted config path produces backend state matching EXACTLY. No approximation
- [ ] Every capacity/limit/bound is checked in the verifier BEFORE Apply time
- [ ] Every narrowing numeric input has an explicit range check naming the valid range
- [ ] Every name subject to truncation rejects when it would truncate (distinct inputs != same stored name)
- [ ] Not-yet-implemented feature rejects with "deferred" message, not quiet ignore
