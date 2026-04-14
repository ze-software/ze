---
name: Verify specs against code before reporting
description: Never report spec progress by reading the spec alone. Grep the codebase to verify claims.
type: feedback
originSessionId: 806802b1-7fdf-4255-8fb5-d9855a2c189c
---
Do not report on spec implementation progress by reading the spec's "What Remains" or "Implementation Summary" sections alone. Specs go stale. All four cmd-* specs (cmd-1, cmd-2, cmd-3, cmd-9) claimed features were unimplemented when they were fully done with tests.

**Why:** The 2026-04-14 audit found that every single "What Remains" item across all four specs was already implemented in code. The specs had not been updated. Reporting from the spec would have said "3 specs incomplete, significant work remains" when the truth was "all 4 specs are done."

**How to apply:** When asked about spec status, grep for the claimed-missing feature in the codebase before reporting it as missing. If the function/type/test exists, the spec is stale, not the code. Update the spec to match reality.
