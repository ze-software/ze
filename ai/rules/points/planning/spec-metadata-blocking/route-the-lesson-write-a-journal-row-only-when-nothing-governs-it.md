---
kind: directive
level: MUST
stage:
rationale: ai/rationale/planning.md
---
**We do not save a lesson, we update the system with it. You MUST route the lesson to the surface that governs the behavior, and write a journal row only when no surface governs it yet:** a recurring trap to a rule under `ai/rules/`, a design decision to `docs/architecture/`, a protocol obligation to `rfc/short/`, an abandoned approach to `plan/learned/DESIGN-HISTORY.md`, hook friction to `plan/learned/HOOK-FRICTION.md`.
**A row is written only when the work produced a lesson, MUST NOT be written as an artifact of closing a spec, and holds exactly five cells, `| Date | Spec | Surface | Symptom | Fix |`, in `plan/journal/<PROBLEM-class>.md`, never a file named for the subsystem.** No gate asks for a lesson and none MUST be added. `ValidateFile` (`internal/le/spec/journal/validate.go`) rejects malformed rows and unreadable Spec keys. The Spec cell attributes the occurrence; `closureStem` (`internal/le/commit/review.go`) runs `ValidateFile` over each journal shard it commits and reads no stem from it, and `closedSpecStem` recognises closure only from a removed spec that is not a relocation between release buckets. Commit B therefore owes the review artifact; a journal row never makes commit A a closure. None of this is permission to prune a defect record (`ai/rules/completion.md`).
