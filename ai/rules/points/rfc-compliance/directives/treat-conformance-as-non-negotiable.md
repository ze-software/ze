---
kind: directive
level: MUST
stage:
---
**A defect in an implemented capability MUST be distinguished from an absent feature.** Behavior that violates an applicable RFC requirement in a capability Ze implements is a defect; a capability Ze does not provide is an implementation gap, which also remains a conformance gap where the requirement applies. Tests, golden files, comments and audit verdicts MUST NOT declare defective behavior correct. Fix defects exposed while verifying the implemented baseline and correct the affected tests; an absent feature follows the separately agreed scope policy below. An owner-approved deviation MUST retain its RFC section and reason in the scope record and MUST NOT be counted as conformant.
