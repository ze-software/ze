---
kind: directive
level:
stage:
---
**Existing cold-path concatenation is cleanup-on-touch**, not a sweep target:
the tree carries ~300 legacy cold-path `+` sites (web page rendering, one-shot
CLI output). Convert them when you edit the surrounding code; never let one
survive on a hot path (the Hot Path Rule below has no legacy carve-out).
