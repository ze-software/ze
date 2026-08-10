---
kind: directive
level: MUST
stage:
---
1. Lossy field -> MUST the pre-check reject in verifier?
2. Bounded output structure -> MUST the capacity check reject when exceeded?
3. Truncated name -> MUST the length check reject before truncation?
4. Numeric narrowing -> MUST there be an explicit range check with the valid range in the error?
