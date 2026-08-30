---
kind: directive
level: MUST
stage:
---
- **A copy on the UPDATE path MUST match one of the four deliberate copy triggers in `docs/architecture/buffer-architecture.md` ("When a copy is deliberate"). A copy that matches none of them SHOULD be treated as wrong, and the reason it is needed MUST be asked before the code lands.**
