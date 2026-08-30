---
kind: directive
level: SHOULD
stage:
---
- **A seam SHOULD be used ONLY when the listener construction registry genuinely cannot express the construction shape; the registry SHOULD be preferred.** ssh and gNMI are the two that qualify today. Both shapes, and what crosses the boundary in each, are in `docs/architecture/plugin/feature-gates.md`, "The registration shapes".
