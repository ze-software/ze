# 827 -- dispatch-response-passthrough

## Context

Every `OnExecuteCommand` handler in the plugin SDK followed a double-marshal pattern: build a Go data structure, `json.Marshal` it to a string, return the string. The SDK then wrapped that string in `ExecuteCommandOutput{Data: string}` and marshaled the whole struct for the RPC wire, producing JSON-string-inside-JSON. With 22 registrations and ~65 handler functions, this wasted work on every command response. The goal was to change the handler signature from `(string, string, error)` to `(string, any, error)` so handlers return Go values directly, and the SDK marshals once.

## Decisions

- Changed `OnExecuteCommand` data return from `string` to `any` over adding a new `OnExecuteCommandTyped` method, because all 22 call sites are internal and pre-release.
- Changed `ExecuteCommandOutput.Data` from `string` to `json.RawMessage` over keeping it as `string`, because `json.RawMessage` is the standard Go type for pre-marshaled JSON and embeds without double-encoding.
- Changed RIB `CommandHandler` type to match `(string, any, error)` over keeping a separate type, because it's the same data flow.
- Renamed handler functions to drop `JSON` suffix (e.g., `statusJSON` -> `status`) because they no longer produce JSON.
- Pipeline terminal `Meta().JSON` fields kept as `json.Marshal`-produced strings, wrapped in `json.RawMessage` at the pipeline boundary, because changing the pipeline internals was out of scope.
- Event bus payloads (sysctl `appliedJSON`, event subscribers) kept as marshal-to-string because the event bus expects string payloads, not the command response path.

## Consequences

- Handler functions return Go values (`map[string]any`, structs, slices). Test code that parsed JSON strings now uses type assertions or `mustMarshal` helpers.
- Engine-side consumers (`command.go`, `system.go`, `subsystem.go`) use `string(rpcOut.Data)` on error paths and `plugin.RawJSON(rpcOut.Data)` on success paths.
- Non-BGP handler helper functions that previously returned `string` now return `any`. Callers (both OnExecuteCommand closures and event bus subscribers) must be aware of the type.
- This spec and `spec-ipc-dispatch-data-raw` (which changes `DispatchCommandOutput.Data` on the other RPC path) are independent and can land in any order.

## Gotchas

- Pipeline terminals produce JSON strings internally via `Meta().JSON`. Returning these directly as `any` (a Go `string`) causes double-encoding when the SDK marshals. Must wrap in `json.RawMessage(meta.JSON)` at pipeline boundary functions.
- Functions used by both command handlers AND event bus subscribers (sysctl's `showEntries()`, `listKnownKeys()`, `describeKey()`) need the event bus callers to marshal the result themselves, since the function now returns `any` instead of a JSON string.
- `replace_all` on `(string, string, error) {` is safe for handler closures but can accidentally change non-handler functions in the same file. The `appliedJSON` function was incorrectly transformed and had to be restored.
- Pre-existing changes to `DispatchCommandOutput.Data` (from spec-ipc-dispatch-data-raw) were already in the working tree, causing cascading compilation errors in `dispatch.go` and `sdk_engine.go` that had to be fixed alongside this spec's changes.

## Files

- `pkg/plugin/sdk/sdk_callbacks.go` - OnExecuteCommand signature change
- `pkg/plugin/rpc/types.go` - ExecuteCommandOutput.Data -> json.RawMessage
- `internal/component/bgp/plugins/rib/rib_commands.go` - CommandHandler type + all handlers
- `internal/component/bgp/plugins/rib/rib_commands_community.go` - community handlers
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - showPipeline return type
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - bestPipeline return type
- `internal/component/bgp/plugins/rib/rib_inject.go` - inject handlers
- `internal/component/plugin/server/command.go` - engine consumer
- `internal/component/plugin/server/system.go` - engine consumer
- `internal/component/plugin/server/subsystem.go` - engine consumer
- 18 plugin register/handler files (BGP, non-BGP, test plugins)
- 15 test files updated
