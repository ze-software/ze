---
kind: table
level:
stage:
---
| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Direct imports between packages | `init()` + registry + blank import | `ai/patterns/registration.md` | Small core discovers components; never imports directly |
| Constructor injection | Registry lookup at runtime (`registry.NLRIDecoder(family)`) | `ai/rules/plugins.md` | Plugins are independently removable via blank import |
| `os.Getenv("FOO")` | `env.Get("ze.foo")` via `internal/core/env` | `ai/rules/go-standards.md` | Cache, registration, dot/underscore agnostic, secret clearing |
| `log.Printf` or `logrus` | `slog` via `slogutil.Logger("subsystem")` | `ai/rules/go-standards.md` | Hierarchical per-subsystem levels via env vars |
| Shared types via direct import | Cross-boundary payloads are value types only | `ai/rules/plugins.md` (Cross-Boundary Value Types) | No pointer fields across plugin/component boundaries |
