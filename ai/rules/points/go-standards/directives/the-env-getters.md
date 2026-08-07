---
kind: table
level:
stage:
---
| Getters | Use |
|---------|-----|
| `env.Get("ze.foo.bar")` | String lookup (case-insensitive, dot/underscore agnostic) |
| `env.GetInt("ze.foo", 0)` | Integer with default |
| `env.GetInt64("ze.foo", 0)` | Int64 with default |
| `env.GetBool("ze.foo", false)` | Boolean (true/false/1/0) with default |
| `env.IsEnabled("ze.foo")` | Enabling check (1/true/yes/on/enable/enabled) |
| `env.GetDuration("ze.foo", 5*time.Second)` | Duration with default |
