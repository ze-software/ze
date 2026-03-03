# 187 — Family Plugin Infrastructure

## Objective

Add a registry mapping AFI/SAFI families to the plugin that claims them, enabling the engine to auto-load plugins for unknown families and route decode/encode to the correct handler.

## Decisions

- Case normalization happens at storage time, not lookup time — `RegisterFamily()` lowercases before inserting so callers never need to normalize.
- Separate `RegisterFamily()` function was dropped; family registration integrated directly into `Register()` to keep one call site per plugin.
- Family format validated (must contain "/", both parts non-empty) at registration; never enumerate all known families — plugins declare what they handle.

## Patterns

- `PluginRegistry.families` maps `"afi/safi"` string to plugin name; `LookupFamily()` returns name or empty string.
- `parseFamily()` extended to handle optional `decode` keyword from `declare family <afi> <safi> decode`.

## Gotchas

- Original implementation stored family strings as-is but looked them up lowercase — caused silent misses. Always normalize at write time, not read time.

## Files

- `internal/plugin/registry/registry.go` — families map, LookupFamily, RegisterFamily
- `internal/plugin/registry/parse.go` — parseFamily with decode keyword
