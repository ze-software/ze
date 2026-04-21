---
name: ze-progress
description: Use when working in the Ze repo and the user asks for ze-progress or wants to know the current lifecycle stage of the selected spec. Check implementation, deferrals, review state, verification, and closure, then recommend exactly one next action.
---

# Ze Progress

This skill answers where the current spec sits in the Ze workflow.

## Workflow

1. Read `tmp/session/selected-spec`; stop if none is selected.
2. Read the spec and extract ACs, tests, file lists, wiring checks, and status metadata.
3. Check the lifecycle stages in order:
   - implementation evidence
   - open deferrals
   - post-edit review status
   - verification and commit readiness
   - learned-summary closure
4. Stop at the first unsatisfied stage.
5. Report the evidence and recommend one next command only.

## Rules

- Read-only.
- Every `Done` claim needs concrete evidence such as `file:line`, a named test, or a commit.
- Do not jump ahead to later stages once an earlier stage is incomplete.
