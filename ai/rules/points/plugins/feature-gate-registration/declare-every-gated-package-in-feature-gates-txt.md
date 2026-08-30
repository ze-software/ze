---
kind: directive
level: MUST
stage:
---
- **Every gated package MUST be declared in the repo-root `feature-gates.txt`, and that file is the ONLY declaration point.** Every other consumer derives from it, so there is nothing to hand-sync. The line format, the consumer list and the current tag inventory are `docs/architecture/plugin/feature-gates.md` and the manifest itself.
