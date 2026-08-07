---
kind: directive
level:
stage:
---
**BLOCKING on hot paths.** Never store an IP address as a string when it will be
compared. Store `netip.Addr` and use `.Compare()` directly.
