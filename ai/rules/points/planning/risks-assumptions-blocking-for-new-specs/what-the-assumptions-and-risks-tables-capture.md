---
kind: directive
level: MUST
stage:
---
**Both tables MUST be filled, and each row MUST reach a terminal state before the spec closes.**

| Table | Captures | Lifecycle |
|-------|----------|-----------|
| Assumptions (A-N) | Beliefs the design depends on, with Basis and a validation method | `unvalidated` → `confirmed` or `broken`. Validate cheap ones (grep/read) during the /ze-implement audit, before coding. |
| Risks (R-N) | Failure modes that exist even if assumptions hold, with early signal + mitigation | Reviewed at each phase; surviving risks copy forward to the Executive Summary and, when one is owed, to the journal row. |
