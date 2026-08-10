---
kind: directive
level: MUST
stage:
---
**Existing cold-path concatenation is cleanup-on-touch**, not a sweep target:
the tree carries ~300 legacy cold-path `+` sites (web page rendering, one-shot
CLI output). They MUST be converted when the surrounding code is edited; one
MUST NOT survive on a hot path (the Hot Path Rule below has no legacy carve-out).
