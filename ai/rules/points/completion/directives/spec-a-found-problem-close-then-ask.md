---
kind: directive
level: MUST
stage:
rationale: ai/rationale/found-problem-spec-first.md
---
**A problem you FIND while working on something else gets a JOURNAL ROW, not a spec (owner directive, 2026-08-10).** You MUST append one row to `plan/journal/<class>.md`, close the work in hand, and stop. No spec, no deferral row, no question to Thomas, no report paragraph. Rows accumulate by problem class, and a class that collects rows earns a fix later, in a deliberate pass over the journal rather than by whoever tripped over it.
**Three finds are FIXED on the spot, and they are the only three.** A defect that stops a test or a gate from passing is fixed now. A test that is wrong about what it asserts is fixed now. Code related to the problem in hand is fixed now, edited or not, tests included ("The unit you fix is the PROBLEM", above). Everything else is one row.
**Fix it anyway when the fix is small, and still write the row.** A five-line correction needs no spec to license it, and `simplicity.md` governs its shape. Opening a spec to authorise a small fix is the overhead this directive removes.
**The cut is the goal, unchanged from `rule-precedence.md`: does the goal this work exists to achieve still hold if I leave this?** If it does not hold, the defect BLOCKS you and "Fix a defect that blocks your goal" (above) governs. If it holds, this point governs.
**You MUST NOT characterise the find beyond the row.** Five columns, one line each: `| Date | Spec | Surface | Symptom | Fix |`. Reproducing it, tracing its producer, sizing its blast radius and drafting its options are work nobody commissioned, and they cost the session that found it and every session that reads what it wrote.
**Before writing a row, grep `plan/journal/` for the same symptom.** Many sessions run this checkout at once and meet the same defect. A second row for a find already recorded adds nothing; a row in a class file that already holds three is the pattern that earns the fix.
