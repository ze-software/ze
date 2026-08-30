---
kind: directive
level: MUST NOT
stage:
---
- **The static files derived from the manifest MUST NOT have their tag lists hand-edited.** Add the gate to `feature-gates.txt`, run `./le feature-tags write`, then `./le feature-tags check`.
