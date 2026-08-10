---
kind: directive
level: MUST
stage:
---
**`refine` MUST be used to set per-service defaults for ip and port.** The `ip` leaf MUST be `zt:ip-address` (numeric, not hostname) because listeners bind to local interfaces.
