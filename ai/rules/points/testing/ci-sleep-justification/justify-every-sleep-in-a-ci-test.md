---
kind: directive
level: MUST
stage:
---
- **Blocking gate:** `check_ci_sleep_justification` in
  `scripts/dev/verify_wiring_docs.py`, run by `make ze-doc-wiring-check` (and the
  inventory make gate). Scoped to CHANGED `.ci` files: a session MUST justify
  the sleeps in the tests it touches. Fails (exit 1) listing every unjustified
  `file:line`.
- **Edit-time nudge:** `c_ci_sleep_justification` in
  `.claude/hooks/pretool-writeedit.py` warns (non-blocking) when a Write/Edit of a
  `.ci` introduces a `time.sleep(` with no comment on the line above or trailing it.
