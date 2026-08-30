---
kind: directive
level: MUST
stage:
---
- **A contract package other features consume, a nil-able seam or the value types a gated feature exposes, MUST stay OFF the manifest and always-on; only the machinery gates.** Every consumer of such a seam MUST already handle the absent case, so verify each call site before choosing this shape, and make the absent-build `nm` needles NAME the gated sub-packages instead of using the subtree prefix.
