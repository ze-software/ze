---
kind: directive
level: MUST
stage:
---
- **The `functional/verify` tier MUST be read as "this suite runs when the change set reaches it", and MUST NOT be read as "this `.ci` runs on every gating run".** A gating run selects its suites from the recorded suite map, in full mode as well as changed mode (`docs/architecture/testing/verify-freshness-scope.md`), so a green functional stage is not evidence that one tagged `.ci` executed, and the suite that carries the tag is what you run when you need that evidence. The tier itself stays derived from the `Gating` list in `internal/le/functional/suites.go` and never from the map, so no requirement loses a tier when a run rules its suite out.
