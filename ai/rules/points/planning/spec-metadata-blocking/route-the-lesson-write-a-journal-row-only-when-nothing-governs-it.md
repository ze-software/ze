---
kind: directive
level: MUST
stage:
---
**We do not SAVE a lesson, we UPDATE the system with it (owner directive, 2026-08-10). You MUST ROUTE the lesson to the surface that governs the behavior, and write a journal row only when no surface governs it yet:** a recurring trap to a rule under `ai/rules/`, a design decision to `docs/architecture/`, a protocol obligation to `rfc/short/`, an abandoned approach to `plan/learned/DESIGN-HISTORY.md`, hook friction to `plan/learned/HOOK-FRICTION.md`.
**A row is written only when the work produced a lesson, MUST NOT be written as an artifact of closing a spec, and holds exactly five cells, `| Date | Spec | Surface | Symptom | Fix |`, in `plan/journal/<PROBLEM-class>.md`, never a file named for the subsystem.** No gate asks for a lesson and none MUST be added. `journal_row_cells` (`internal/le/journal/journal.go`) names a malformed row, and `spec_closure_stem` (`internal/le/commit`) reads the `Spec` cell to recognise commit A as a closure, so an empty one drops the review gate off the commit carrying the code. None of this is permission to prune a defect record (`ai/rules/completion.md`).
