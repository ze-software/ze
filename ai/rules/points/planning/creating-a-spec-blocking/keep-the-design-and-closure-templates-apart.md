---
kind: directive
level: MUST
stage:
---
**Two templates, one per lifecycle half.** `plan/TEMPLATE.md` is design-time:
everything that MUST exist BEFORE code. The closure half lives in
`plan/TEMPLATE-CLOSURE.md` and is appended by `/ze-close` at step 1. MUST
NOT copy the closure sections into a new spec: measured across 161 specs,
sections copied at creation but used only at closure arrived there untouched in
65-75% of in-progress specs, while the sections authors added when they needed
them were untouched in 0%. Distance from use is what empties a section.
