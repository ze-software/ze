---
kind: directive
level: MUST
stage:
rationale: ai/rationale/found-problem-spec-first.md
---
**A problem you FIND while working on something else gets a JOURNAL ROW, not a spec (owner directive, 2026-08-10).** You MUST append one row to `plan/journal/<class>.md`, close the work in hand, and stop. No spec, no row anywhere else, no question to Thomas, no report paragraph. A class that collects rows earns its fix in a deliberate pass over the journal.
**Three finds are FIXED on the spot, and they are the only three:** a defect that stops a test or a gate from passing, a test that is wrong about what it asserts, and code related to the problem in hand, edited or not, its tests included. Fix a small one anyway, and still write the row.
**You MUST NOT characterise the find beyond the row's five columns, `| Date | Spec | Surface | Symptom | Fix |`.** Reproducing it, tracing its producer, sizing its blast radius and drafting its options are work nobody commissioned. Grep `plan/journal/` for the symptom first, because many sessions share this checkout, and COMMIT the row: an uncommitted one dies at the next clean or checkout.
