---
kind: directive
level: MUST
stage:
---
- **An always-on importer that blocks a gate MUST be cleared by transitive package drop first, then core-leaf move, then inversion-of-control seam, and implementers SHOULD aim for the FEWEST source-tagged files rather than the fewest edits.** A core-leaf move MUST move the LEAF rather than the package, or it becomes a core-tier violation. A nil seam MUST have a CORRECT no-feature behaviour, not just a nil check. What each technique looks like at subsystem scale is `docs/architecture/plugin/feature-gates.md`.
