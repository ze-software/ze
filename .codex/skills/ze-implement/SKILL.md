---
name: ze-implement
description: Use when working in the Ze repo and the user asks for ze-implement or wants the selected spec carried through implementation. Update the spec status, audit what already exists, implement phase by phase with tests first, then verify, review, and finish the documentation updates.
---

# Ze Implement

This skill runs the Ze spec implementation workflow end to end.

## Workflow

1. Read `tmp/session/selected-spec` and the spec file.
2. Immediately update the spec metadata to `in-progress`, set the current phase, and stamp today's date.
3. Audit the planned files and tests before building anything new.
4. Implement phase by phase, writing the listed tests first and keeping each step minimal.
5. Run the appropriate verification targets, then review the result against the spec, quality rules, and security concerns.
6. Fix findings and repeat until verification is green and the review finds no open issues.
7. Finish the documentation checklist and name the exact docs that changed.
8. Deliver a concise summary of code, tests, docs, and resolved issues.

## Rules

- Stop if the spec is missing critical review, deliverables, security, or documentation checklists.
- Do not silently defer work that the spec still claims is in scope.
