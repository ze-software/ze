---
kind: table
level:
stage:
---
| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Struct tags + `json.Unmarshal` | YANG schema as sole source of truth | `ai/rules/config.md` | Schema-driven validation, migration, completion, diff |
| Config version field | No version numbers; machine-transformable migration | `ai/rules/config.md` | YANG evolution handles schema changes |
| Silent defaults for missing fields | Fail on unknown keys; suggest closest valid | `ai/rules/config.md` | Explicit > implicit |
| `interface{}` for flexible config | `map[string]any` through canonical pipeline | `ai/rules/repo-maintenance.md` | File -> Tree -> ResolveBGPTree -> map[string]any -> PeersFromTree |
