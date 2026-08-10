---
kind: directive
level: MUST
stage:
---
**A present-but-empty value passes `ok`.** `ok` proves the key exists, not that
the value is usable. When empty is also wrong, you MUST check `!ok || len(v) == 0`.
