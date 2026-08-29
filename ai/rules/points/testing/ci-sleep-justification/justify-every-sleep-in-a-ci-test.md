---
kind: directive
level: MUST
stage:
---
Every sleep in a `.ci` test MUST carry its justification marker. Two producers
enforce it, at different moments.

- **Blocking gate:** `checkSleepJustification` in
  `internal/le/doc/wiring`, run by `./le doc wiring`.
  Scoped to changed `.ci` files, it lists every unjustified `file:line` and
  returns exit 1.
- **Edit-time nudge:** `writeCISleep` in `internal/le/hookruntime/writeedit.go`
  blocks a Write/Edit that introduces `time.sleep(` with no recognised
  justification.
