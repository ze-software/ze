---
kind: directive
level: MUST
stage:
---
- [ ] Every accepted config path MUST produce backend state matching EXACTLY. No approximation
- [ ] Every capacity/limit/bound MUST be checked in the verifier BEFORE Apply time
- [ ] Every narrowing numeric input MUST have an explicit range check naming the valid range
- [ ] Every name subject to truncation MUST reject when it would truncate (distinct inputs != same stored name)
- [ ] Not-yet-implemented feature MUST reject with a "deferred" message, not quiet ignore
