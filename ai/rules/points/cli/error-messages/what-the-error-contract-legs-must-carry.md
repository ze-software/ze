---
kind: directive
level: MUST
stage:
---
An error MUST carry these three legs:

1. **What failed** -- the specific operation plus the identifying subject:
   `file:line`, config key, field name, command, NLRI, port, diagnostic code.
   MUST NOT be a bare "operation failed" or "invalid input".
2. **Why -- the evidence** -- the offending value AND the expected one. MUST quote
   values with `%q` so empty, whitespace, or look-alike values are visible:
   `expected exit code %d, got %d`, `unknown field %q (want one of ...)`.
3. **What to do next** -- the corrective action, or a stable handle the reader
   can act on: a directive to add, a flag to set, a native action to run, or a
   registered `doctor-*` diagnostic code that `ze explain` expands.
