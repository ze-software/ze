---
kind: directive
level: MUST
stage:
---
- **Blocking gate:** `check_ci_sleep_justification` in
  `internal/le/docwiring.CheckCISleepJustifications`, run by `./le doc-wiring`.
  Scoped to changed `.ci` files, it lists every unjustified `file:line` and
  returns exit 1.
- **Edit-time nudge:** `writeCISleep` in `internal/le/hookruntime/writeedit.go`
  blocks a Write/Edit that introduces `time.sleep(` with no recognised
  justification.
