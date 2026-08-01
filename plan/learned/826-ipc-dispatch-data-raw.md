# 826 -- ipc-dispatch-data-raw

## Context

The plugin-to-engine `dispatch-command` RPC carried its payload in `DispatchCommandOutput.Data string`, where the string was already a JSON document from `json.Marshal`. When the whole struct was marshaled onto the wire, `Data` became a JSON-string-inside-string, forcing every consumer to decode twice. The same field was overloaded to carry plain error text on the error path, making it ambiguous. Goal: single-decode raw JSON in `Data`, separate `Error` field, and update every consumer.

## Decisions

- Changed `Data` from `string` to `json.RawMessage` over keeping `string` with documentation, because single-decode is the whole point; documenting the footgun does not remove it
- Added a dedicated `Error string` field over keeping one overloaded field, because one field should have one meaning; the overload forced every consumer to branch on Status before interpreting Data
- Changed `dispatchCommand` to return `*rpc.DispatchCommandOutput` over keeping `(status, data string, err)`, because the struct passthrough eliminates field decomposition/recomposition at both the direct handler and bridge wiring sites
- Changed the bridge `DispatchCommandHandler` to return `(*DispatchCommandOutput, error)` over keeping `(status, data string, err)`, because the struct flows through without re-encoding
- SDK `DispatchCommand` folds `out.Error` into a Go error over exposing it as a separate return value, because plugin code should check `err != nil` rather than inspecting a field
- Scope limited to `DispatchCommandOutput` (plugin-to-engine direction) over also fixing `ExecuteCommandOutput` (engine-to-plugin direction), because the latter is a separate spec (spec-dispatch-response-passthrough)

## Consequences

- All callers of `Plugin.DispatchCommand` receive `json.RawMessage` for `data` instead of `string`; callers that passed `data` to `string`-typed parameters need `string(data)` conversion
- `parseReplayResponse` in RS and RR plugins now takes `json.RawMessage` directly, removing the `[]byte(data)` conversion that was the consumer-side symptom of double-encoding
- The bridge fast path returns `*DispatchCommandOutput` struct directly, maintaining zero-serialization semantics while sharing the same type as the slow path
- Pre-release IPC contract change: external plugins using `dispatch-command` need to read `data` as embedded JSON and `error` as a separate field

## Gotchas

- Changing `DispatchCommandOutput.Data` type triggers a wide ripple: every test hook, wrapper function, and `parseReplayResponse` call across RS, RR, healthcheck, BMP plugins needed updating
- `assert.Contains(t, output.Data, "substring")` silently does byte-membership on `json.RawMessage` (a `[]byte`) instead of substring search; must use `string(output.Data)`
- Concurrent agent work on the same wire types (`ExecuteCommandOutput`) creates merge conflicts in `types.go` and cascading build failures through `all_import_test.go`

## Files

- `pkg/plugin/rpc/types.go` -- wire type: Data `json.RawMessage`, added Error field
- `pkg/plugin/rpc/bridge.go` -- DispatchCommandHandler returns `*DispatchCommandOutput`
- `internal/component/plugin/server/dispatch.go` -- responseToDispatchOutput, dispatchCommand
- `pkg/plugin/sdk/sdk_engine.go` -- DispatchCommand returns `json.RawMessage`
- `internal/component/bgp/cli/cmd_plugin.go` -- print adaptation
- `internal/component/bgp/plugins/rr/rr.go` -- dispatchCommand, parseReplayResponse
- `internal/component/bgp/plugins/rs/server.go` -- dispatchCommand, dispatchCommandHook
- `internal/component/bgp/plugins/rs/server_handlers.go` -- parseReplayResponse
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` -- dispatchFn type
- `internal/component/plugin/server/dispatch_test.go` -- new tests
- `test/plugin/dispatch-command-single-decode.ci` -- functional test
