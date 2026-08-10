---
kind: directive
level: MUST
stage:
excepted-by: completion/directives/spec-a-found-problem-close-then-ask, planning/critical-review-is-the-central-deliverable/a-defect-in-test-only-code-is-not-a-finding-in-the-product
---
**Recording a problem is not addressing it. You MUST fix the root cause, always.** Writing a failure down (in `plan/known-failures/`, a journal row, a deferral row, or a report to the user) changes nothing about the product. A record is a step *toward* a fix and never a substitute for one. When you find a red test, a hang, a wrong result, or a silent misbehavior, the deliverable is the FIX.
**The JOURNAL ROW is the one exception, and only on the route "A problem you FIND" (above) sets out: one row, close the work in hand, stop (owner directive, 2026-08-10).** This point governs the defect you were sent to fix, where a record instead of a fix is the failure. It does not govern the defect you merely walked into, which is not yours to fix and whose row is the whole obligation. You MUST NOT write a SPEC for a walked-into defect, and you MUST NOT ask Thomas whether to implement one.
