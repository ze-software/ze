---
kind: directive
level: MUST
stage:
---
**Before adding a `string` field that crosses a component seam, or that sits on a hot path, you MUST answer all three:**
- Is this value one of a closed set? Then it MUST be a typed enum
- Is it read more than once per message? Then it MUST be parsed at the boundary and carried typed
- Does the receiving side compare it against a literal? Then the literal MUST become a constant and the field MUST become its type
