---
kind: directive
level: MUST NOT
stage:
---
**A tagged test's assertions MUST NOT be weakened IN PLACE while the shape stays the same.** No ratchet catches that. The write-time guard, the commit audit and the audit-freshness SHA ratchet do, and `docs/contributing/rfc-conformance-gates.md` says which does what.
**A `test/weakened.md` row is self-service and MUST NOT be treated as authorizing the weakening of an RFC-tagged test.**
