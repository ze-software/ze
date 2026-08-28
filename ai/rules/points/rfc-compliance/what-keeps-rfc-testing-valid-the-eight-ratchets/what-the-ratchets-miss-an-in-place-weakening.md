---
kind: directive
level: MUST NOT
stage:
---
**A tagged test's assertions MUST NOT be weakened *in place* while keeping the same shape.** None of the eight ratchets catches that: `c_test_weakening` and `./le commit audit`, plus the SHA ratchet (`check_audit_freshness`), catch it instead, wherever `/ze-rfc-audit` has recorded a verdict. The SHA ratchet is armed only for RFCs that have an `rfc/audit/<rfc>.json`.
