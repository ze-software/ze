---
kind: table
level:
stage:
---
| Setters | Use |
|---------|-----|
| `env.Set("ze.foo", "val")` | String (updates cache + os env) |
| `env.SetInt("ze.foo", 42)` | Integer |
| `env.SetBool("ze.foo", true)` | Boolean ("true"/"false") |
