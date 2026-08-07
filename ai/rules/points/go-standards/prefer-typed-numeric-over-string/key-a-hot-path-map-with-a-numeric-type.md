---
kind: directive
level:
stage:
---
**BLOCKING on hot paths.** When a map is keyed by a value from a known set, use a numeric key type (`uint8`, `uint16`, `int`, typed enum), not `string`.
