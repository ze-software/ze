---
kind: directive
level:
stage:
---
**A present-but-empty value passes `ok`.** `ok` proves the key exists, not that
the value is usable. When empty is also wrong, check `!ok || len(v) == 0`.
