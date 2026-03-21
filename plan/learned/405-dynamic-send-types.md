# 405 -- Dynamic Send Types

## Objective

Make send type validation dynamic (plugin-registered) instead of hardcoded in the engine. Remove `SendEnhancedRefresh bool` from ProcessBinding, replacing it with `SendCustom map[string]bool`. Add auto-load for plugins providing send types.

## Decisions

- **SendTypes parallel to EventTypes**: same registration pattern -- plugins declare `SendTypes` in Registration, engine registers into `ValidSendTypes` at startup, config parser validates dynamically
- **Route-refresh registers enhanced-refresh**: no separate plugin created. `bgp-route-refresh` already owns RFC 7313 (BORR/EORR, capability code 70). Adding `SendTypes: ["enhanced-refresh"]` to it follows the proximity principle. A hollow new plugin would violate single-responsibility.
- **SendCustom map for plugin types**: base types (update, refresh) keep dedicated bool fields for efficiency. Plugin-registered types go into `SendCustom map[string]bool`, mirroring `ReceiveCustom`.
- **Four-phase startup**: Phase 4 added for send type auto-load, parallel to Phase 3 (event type auto-load). Extracted `getUnclaimedPluginsForTokens` helper to deduplicate Phase 3 and 4 logic.

## Patterns

- **Dynamic type registration**: `Registration.SendTypes` -> `RegisterPluginSendTypes()` (sync.Once) -> `ValidSendTypes` map -> `IsValidSendType()` in config parser. Exact mirror of EventTypes flow.
- **Auto-load chain**: config `send [ enhanced-refresh ]` -> `ConfiguredCustomSendTypes` -> `PluginForSendType()` -> `ResolveDependencies()` -> auto-start route-refresh
- **Token-based auto-load helper**: `getUnclaimedPluginsForTokens(tokens, lookupFn, kind)` eliminates duplication between event type and send type auto-load functions
- **maps.Clone for SendCustom**: same pattern as ReceiveCustom when copying across type boundaries (reactor ProcessBinding -> plugin PeerProcessBinding)

## Gotchas

- **sync.Once for registration**: `RegisterPluginSendTypes` must be called both in `PeersFromTree` (config parsing happens before server startup) and `NewServer`. The sync.Once ensures idempotency.
- **Error message completeness**: when rejecting unknown send types, the error message must include both base types (update, refresh) AND dynamically registered types. Concatenating `ValidSendTypeNames()` after the base list.
- **dupl linter**: event type and send type auto-load functions have identical structure. Extracting a shared helper with a lookup function parameter avoids the dupl lint violation.

## Files

- `internal/component/plugin/events.go` -- ValidSendTypes map, RegisterSendType, IsValidSendType, ValidSendTypeNames
- `internal/component/plugin/registry/registry.go` -- SendTypes field on Registration, PluginForSendType
- `internal/component/plugin/resolve.go` -- RegisterPluginSendTypes (sync.Once)
- `internal/component/plugin/inprocess.go` -- GetPluginForSendType
- `internal/component/bgp/reactor/config.go` -- parseOneSendFlag validates against dynamic set
- `internal/component/bgp/reactor/peersettings.go` -- SendCustom map replaces SendEnhancedRefresh bool
- `internal/component/plugin/types.go` -- same on PeerProcessBinding
- `internal/component/bgp/reactor/reactor_api.go` -- maps.Clone(b.SendCustom)
- `internal/component/bgp/config/loader.go` -- derives ConfiguredCustomSendTypes
- `internal/component/bgp/reactor/reactor.go` -- ConfiguredCustomSendTypes on Config
- `internal/component/plugin/server/config.go` -- ConfiguredCustomSendTypes on ServerConfig
- `internal/component/plugin/server/startup_autoload.go` -- getUnclaimedSendTypePlugins + shared helper
- `internal/component/plugin/server/startup.go` -- Phase 4 in runPluginStartup
- `internal/component/bgp/plugins/route_refresh/register.go` -- SendTypes: ["enhanced-refresh"]
- `test/parse/send-enhanced-refresh.ci` -- dynamic send type accepted
- `test/parse/send-unknown-rejected.ci` -- unregistered send type rejected
