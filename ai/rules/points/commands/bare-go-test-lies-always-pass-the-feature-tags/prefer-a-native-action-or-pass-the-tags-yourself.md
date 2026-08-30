---
kind: directive
level: MUST
stage:
---
**A registered native action (`./le test-unit`, `./le verify worktree`) is the
route, and a run scoped to packages MUST carry the feature build tags itself.**
A bare `go test` omits them, so plugins never register and unrelated tests fail
with a phantom red. The tag list, the symptom, and the `git archive` variant of
the same trap are `docs/contributing/running-commands.md`.
