---
kind: directive
level: MUST
stage:
rationale: plan/learned/012-fix-the-question-not-the-site.md
---
**You MUST fix defects in existing capabilities exposed by the scoped proof, including defects in the tests used to establish that proof.** Diagnose the operation that failed against the agreed acceptance criteria, including its inverse, undo and disable paths. Recording a finding does not satisfy a required fix or permit a completion claim while the defect still blocks those criteria.
**A logic bug or misbehavior MUST be fixed, and whether to fix it MUST NOT be put to the owner (`ai/META-DECISIONS.md`).** An absent requirement of an implemented RFC is owed work under `ai/rules/rfc-compliance.md`; when it lies outside the work in hand it gets its gap record, not a question. Where scope stays ambiguous after the owner's standing decisions, the RFC and the agreed design are read, ask which way, never whether.
**You MUST NOT offer the user a reduction in coverage as a way out of a red.** Dropping an interop or functional test, weakening an assertion, and marking a goal-validation row "N/A" are the failure, never a choice to put on the table.
