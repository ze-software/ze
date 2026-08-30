---
kind: directive
level: MUST
stage:
---
**Existing cold-path concatenation is cleanup-on-touch, not a sweep target: a legacy `+` site MUST be converted when the surrounding code is edited.** A `+` between strings MUST NOT survive on a hot path, where the Hot Path Rule below carries no legacy carve-out.
