---
kind: directive
level: MUST
stage:
---
**A defect in an implemented capability MUST be distinguished from an absent feature and an explicitly selected extension.** Behavior that violates an applicable RFC requirement on the default configuration path is a defect; a capability Ze does not provide is an implementation gap, which also remains a conformance gap where the requirement applies. Tests, golden files, comments and audit verdicts MUST NOT declare defective default behavior correct. Fix defects exposed while verifying the default path and correct the affected tests; an absent requirement of an implemented RFC is owed work under the full-coverage directive below. An explicitly selected extension that differs from an RFC MUST retain its RFC section, configuration and reason in the scope record. Its behavior MUST NOT be counted as conformant, and its availability MUST NOT count against the proven default path.
