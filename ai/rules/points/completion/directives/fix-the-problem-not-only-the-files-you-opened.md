---
kind: directive
level: MUST
stage:
---
**The unit you fix is the PROBLEM, not the files you happened to open (owner directive, 2026-08-10).** Fix the code you are editing AND the code related to the problem that you are not editing, its tests included. A related defect living in a file nobody opened is part of the work, and "I was not in that file" is not a boundary.
**Related means it shares the problem, not the diff.** The other call site of the function you corrected, the sibling path that carries the same defect, the test that asserts the behavior you just changed, the fixture that encodes the old shape: each one leaves the problem half-fixed if you leave it, so each one is in scope now.
**Everything else you notice gets ONE journal row, for later analysis.** A defect or a missing feature that belongs to a different problem is recorded in `plan/journal/<class>.md` and nothing more: no spec, no deferral row, no question, no report paragraph ("A problem you FIND", below). Rows accumulate by class, and a class that collects rows earns a deliberate pass of its own.
