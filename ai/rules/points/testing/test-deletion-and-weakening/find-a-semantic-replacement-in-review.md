---
kind: directive
level: MUST
stage:
---
**Detection:** `/ze-review` step 0 (`audit-test-relaxation.py`) flags structural
changes. For semantic replacement, `/ze-review` step 7 (removed-behavior audit)
MUST verify that every assertion the diff replaces still has coverage elsewhere.
When reviewing a test edit that changes WHAT is asserted (not just adding new
assertions), ask: "is the old behavior still tested?"
