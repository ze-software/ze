---
kind: note
level:
stage:
---
Closure once depended on remembering the two-commit step, so it was routinely
dropped and specs piled up in `plan/` as false "open work". Three mechanical
gates exist for it, and all three run. The `Stop` array in
`.claude/settings.json` is the authority on the hook gate:
