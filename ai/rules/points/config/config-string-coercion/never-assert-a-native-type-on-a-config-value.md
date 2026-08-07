---
kind: directive
level:
stage:
---
**Never `v.(bool)` / `v.(float64)` directly on a config value, and never a numeric/bool type switch without a `case string:` arm.**
