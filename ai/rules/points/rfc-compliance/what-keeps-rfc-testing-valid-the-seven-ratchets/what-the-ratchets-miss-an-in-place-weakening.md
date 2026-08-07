---
kind: directive
level:
stage:
---
**What none of this catches: a tagged test whose assertions are weakened *in place* while keeping the same shape.** That is `c_test_weakening` and `scripts/dev/audit-test-relaxation.py`, plus the SHA ratchet (`check_audit_freshness`) wherever `/ze-rfc-audit` has recorded a verdict. The SHA ratchet is armed only for RFCs that have an `rfc/audit/<rfc>.json`.
