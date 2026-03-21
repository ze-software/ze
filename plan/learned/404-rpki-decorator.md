# 404 -- rpki-decorator

## Objective

Enable plugins to register custom event types dynamically and implement the bgp-rpki-decorator plugin that merges UPDATE + RPKI validation events into a single `update-rpki` event for downstream consumers. Also implement auto-loading: `receive [ update-rpki ]` in config automatically starts the decorator and its dependencies without explicit plugin configuration.

## Decisions

- Dynamic event registration follows the family registration pattern: plugins declare `EventTypes` in Registration, engine calls `RegisterEventType` at startup via `RegisterPluginEventTypes()` (sync.Once, idempotent)
- Decorator is a plugin (not engine infrastructure) -- engine stays content-agnostic
- `update-rpki` event type owned by decorator's YANG schema (proximity principle)
- Graceful degradation: if rpki event times out, UPDATE emitted as update-rpki without rpki section
- YANG receive/send changed from enum to `type string` -- runtime validation via `parseReceiveFlags`/`parseSendFlags` instead of schema-level enums
- Auto-loading for event types mirrors the existing family auto-load pattern (Phase 3 in startup)
- `all` removed from receive/send parsers -- users must list event types explicitly

## Patterns

- **Event type registration**: `EventTypes: []string{"update-rpki"}` on Registration, engine calls `RegisterEventType(NamespaceBGP, et)` before any subscription validation
- **Auto-load chain**: config `receive [ update-rpki ]` -> `ConfiguredCustomEvents` -> `PluginForEventType()` -> `ResolveDependencies()` -> auto-start decorator + bgp-rpki (dependency)
- **File extraction**: auto-load logic in `startup_autoload.go` (separated from 5-stage protocol in `startup.go`)
- **Union correlator**: first production consumer. Primary=UPDATE, secondary=rpki, correlated by peer+msgID, merge callback emits via EmitEvent RPC
- **maps.Clone**: use for deep-copying map fields across type boundaries (ReceiveCustom)

## Gotchas

- `parseSendFlags` refactored to return errors (matching `parseReceiveFlags`) -- broke 2 `.ci` tests using `send [ update borr eorr ]` because `borr`/`eorr` never had bool fields and are now rejected
- `RegisterPluginEventTypes()` must be called before config parsing (in `PeersFromTree`) so `receive [ update-rpki ]` is valid during parse
- `hasConfiguredPlugin()` only checks explicit config, not Phase 2 auto-loaded plugins -- Phase 3 also checks `procManager.GetProcess()` for running processes
- `runPluginPhase` replaces `s.procManager` each time -- pre-existing architectural issue, Phase 3 doesn't worsen it since family plugins and event type plugins don't overlap

## Files

- `internal/component/plugin/events.go` -- RegisterEventType, CustomEventTypes, IsValidEvent (RWMutex)
- `internal/component/plugin/resolve.go` -- RegisterPluginEventTypes (sync.Once)
- `internal/component/plugin/registry/registry.go` -- EventTypes field, PluginForEventType lookup
- `internal/component/plugin/server/startup_autoload.go` -- family + event type auto-load
- `internal/component/plugin/server/startup.go` -- Phase 3 in runPluginStartup
- `internal/component/bgp/plugins/rpki_decorator/` -- decorator plugin (decorator.go, merge.go, register.go, schema/)
- `internal/component/bgp/config/loader.go` -- ConfiguredCustomEvents collection
- `test/plugin/rpki-decorator-{merge,register,timeout,autoload}.ci` -- 4 functional tests
