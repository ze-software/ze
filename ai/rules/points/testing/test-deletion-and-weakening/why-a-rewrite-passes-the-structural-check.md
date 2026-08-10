---
kind: directive
level: MUST NOT
stage:
---
**Why the hook cannot catch this:** the rewrite maintains the same structural
shape (same function count, same assertion count), so the mechanical check sees
no weakening. The coverage loss is semantic, not structural, so a passing
structural check MUST NOT be read as proof that nothing was weakened.
