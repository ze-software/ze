---
kind: table
level:
stage:
---
| Flag | Meaning |
|------|---------|
| `Private: true` | Hidden from `ze env list` and autocomplete |
| `Secret: true` | Cleared from OS environment after first `Get()` (value stays in cache) |
