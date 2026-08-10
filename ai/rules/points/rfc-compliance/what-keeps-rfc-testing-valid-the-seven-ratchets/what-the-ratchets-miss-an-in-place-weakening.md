---
kind: directive
level: MUST NOT
stage:
---
**A tagged test's assertions MUST NOT be weakened *in place* while keeping the same shape.** None of the seven ratchets catches that: `c_test_weakening` and `scripts/dev/audit-test-relaxation.py`, plus the SHA ratchet (`check_audit_freshness`), catch it instead, wherever `/ze-rfc-audit` has recorded a verdict. The SHA ratchet is armed only for RFCs that have an `rfc/audit/<rfc>.json`.
