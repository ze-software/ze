---
kind: directive
level: MUST
stage:
---
**MUST start from `plan/TEMPLATE.md`.** Read the template, copy its full
content, then fill in relevant sections and leave others as `(fill during
design)` placeholders. MUST NOT write a spec from memory -- the `validate-spec`
hook rejects files missing required section headers, and writing from scratch
always misses some. One read of the template before the first Write avoids the
rejected-then-rewrite cycle.
