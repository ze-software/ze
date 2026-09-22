---
kind: directive
level: MUST
stage:
rationale: plan/learned/012-fix-the-question-not-the-site.md
---
**You MUST fix defects in existing capabilities exposed by the scoped proof, including defects in the tests used to establish that proof.** Diagnose the operation that failed against the agreed acceptance criteria, including its inverse, undo and disable paths. Recording a finding does not satisfy a required fix or permit a completion claim while the defect still blocks those criteria.
**An absent RFC feature MUST be classified under `ai/rules/rfc-compliance.md` before implementation is considered.** Shared protocol names, files or an RFC's full requirement list do not put an unrelated absent feature into the session's scope. If the agreed scope is genuinely ambiguous, resolve it with the user rather than assuming permission to expand it.
**You MUST NOT offer the user a reduction in coverage as a way out of a red.** Dropping an interop or functional test, weakening an assertion, and marking a goal-validation row "N/A" are the failure, never a choice to put on the table.
