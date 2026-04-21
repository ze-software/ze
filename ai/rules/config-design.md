# Config Design

Rationale: `ai/rationale/config-design.md`
Structural template: `ai/patterns/config-option.md`

- No version numbers in config. Design for machine-transformable migration.
- Fail on unknown keys at any level. No silent ignore. Suggest closest valid key.
- Every YANG `environment/<name>` leaf MUST have a matching `ze.<name>.<leaf>` env var registered via `env.MustRegister()`. Env vars are part of the config interface, not follow-up work.

## YANG Structure

| Pattern | Use |
|---------|-----|
| `grouping` + `uses` | Shared structure within or across components |
| `augment` | Only when a plugin extends another component's YANG |

Augment is for cross-component plugin extensions only. Same-component shared
structure uses grouping. If you are writing an augment and both the source and
target are in the same component, use a grouping instead.

## Listeners

All network listener endpoints use the `zt:listener` grouping (`ip` + `port` from ze-types.yang)
and the `ze:listener` extension (ze-extensions.yang) for port conflict detection.

| Pattern | When |
|---------|------|
| `container` + `ze:listener` + `uses zt:listener` | Single-endpoint services (web, SSH, MCP, LG, telemetry, BGP global listen) |
| `list` + `ze:listener` + `uses zt:listener` | Named multi-instance listeners (plugin hub server) |
| `container` + `ze:listener` + manual ip/port | When ip type differs from standard (BGP peer local: union with auto enum) |

Use `refine` to set per-service defaults for ip and port. The `ip` leaf is always
`zt:ip-address` (numeric, not hostname) because listeners bind to local interfaces.
