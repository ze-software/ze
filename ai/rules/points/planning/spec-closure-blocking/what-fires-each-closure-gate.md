---
kind: table
level:
stage:
---
| Gate | Where | Fires when |
|------|-------|-----------|
| Detector | `./le spec status closure` | `--list` reports completed-but-not-closed specs in two tiers; `--spec <s>` exits 3 only for a high-confidence one. High confidence = a **committed** journal row in `plan/journal/*.md` whose Spec cell exactly equals the spec stem, or a `plan/learned/NNN-<slug>.md` whose slug exactly equals the stem, while the spec is still `in-progress` and is **not an umbrella** (commit A ran, commit B did not). Weaker `[umbrella]` / `[weak-match]` candidates are listed under NEEDS VERIFICATION. Only the high-confidence set triggers the `--spec` block. |
| Stop-hook block | native `block-premature-stop` action in `internal/le/hookruntime/lifecycle.go` | This session claimed a spec, the detector exits 3 for it, and no acknowledgement exists. The hook refuses the stop. |
| Commit reminder | `internal/le/commit` | A commit adds a journal row or learned summary but removes no spec: it prints the closure-commit reminder to stderr. |
