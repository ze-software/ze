---
kind: directive
level: MUST
stage:
---
**BLOCKING on hot paths.** An IP address MUST NOT be stored as a string when it
will be compared. `netip.Addr` MUST be stored and `.Compare()` MUST be used
directly.
