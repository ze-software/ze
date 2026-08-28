---
kind: directive
level: MUST
stage:
rationale: ai/rationale/git-safety.md
---
**Owner directive, 2026-08-17: a commit carrying `.go`, `go.mod`, `go.sum` or `vendor/` MUST be preceded by a full `./le verify worktree` whose run STARTED after your last Go edit. You MUST NOT reach such a commit on scoped gates alone, and you MUST NOT re-run the gate to watch somebody else's red clear.** What the commit owes is the run's COVERAGE, never its exit code: the exit code is read through the attribution table below. `./le commit create` enforces the coverage and names the owner-only escape when no such run exists.
