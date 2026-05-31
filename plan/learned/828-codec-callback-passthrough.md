# 828 -- codec-callback-passthrough

## Context

NLRI decode callbacks (`DecodeNLRIHex`, `OnDecodeNLRI`) followed the same double-marshal pattern fixed in 827 for `OnExecuteCommand`: each handler built a Go data structure, `json.Marshal`ed it to a string, returned the string. The SDK/registry then wrapped that string in `{"json":"escaped_string"}` and marshaled again, producing JSON-string-inside-JSON. This spec moved the single marshal point from N handlers to the registry/SDK wrapper.

## Decisions

- `InProcessNLRIDecoder` returns `(any, error)` over `(json.RawMessage, error)`: keeps handlers marshal-free while `DecodeNLRIByFamily` provides the single marshal point. `format/text_json.go` already treats the result as raw bytes via `append(buf, decoded...)`.
- `DecodeNLRIOutput.JSON` changed from `string` to `json.RawMessage` over keeping string: eliminates the double-encoding at the wire level.
- `OnDecodeCapability` updated to `(any, error)` for API consistency over leaving it unchanged: zero registrations exist, so zero cost.
- `OnEncodeNLRI` kept as `(string, error)`: it returns hex, not JSON, so no double-encoding exists.
- `RunCLIDecode`/`RunDecodeMode` callers now marshal the `any` result themselves: this is the output boundary marshal, not a redundant one.

## Consequences

- All NLRI decode output is now single-marshal: the registry or SDK wrapper is the only place `json.Marshal` runs for decode results.
- Plugin SDK is now consistent: `OnExecuteCommand` (827), `OnDecodeNLRI`, `OnDecodeCapability` all return `any`; only `OnEncodeNLRI` returns `string` (hex).
- `DecodeNLRIByFamily` returns `json.RawMessage`, which is `[]byte`. Callers that previously did `json.RawMessage(result)` conversion (like `decodeMPNLRIs`) now use the return value directly.
- `format/text_json.go` fallback path `append(buf, decoded...)` works unchanged since `json.RawMessage` is `[]byte`.

## Gotchas

- Changing `DecodeNLRIHex` signature ripples to `RunCLIDecode`/`RunDecodeMode` functions that call it for CLI stdout output. These are NOT the SDK callback path but they share the same function. Four plugins (rtc, mup, mvpn, vpls) plus labeled had internal callers that needed `json.Marshal` added at the output boundary.
- `update_text_test.go` calls `registry.DecodeNLRIByFamily` directly and uses `assert.Contains` on the result. Since the return type changed from `string` to `json.RawMessage` (`[]byte`), `Contains` silently fails without a type error. Fix: `string(decoded)` conversion.
- labeled/encode.go used `strings.Builder` to construct JSON manually (not `json.Marshal`). The replacement returns `map[string]any{"prefix": ..., "labels": ...}`, which changes key ordering in output. Tests needed updating from substring matching to marshal-then-unmarshal.

## Files

- `internal/component/plugin/registry/registry.go` - InProcessNLRIDecoder type, DecodeNLRIByFamily return type
- `pkg/plugin/rpc/types.go` - DecodeNLRIOutput.JSON type
- `pkg/plugin/sdk/sdk_callbacks.go` - OnDecodeNLRI, OnDecodeCapability signatures
- `pkg/plugin/sdk/sdk_engine.go` - DecodeNLRI return type
- `internal/component/bgp/server/codec.go` - handleDecodeNLRI, decodeMPNLRIs
- `internal/component/bgp/plugins/nlri/{vpn,evpn,flowspec,rtc,mup,mvpn,vpls}/` - DecodeNLRIHex
- `internal/component/bgp/plugins/nlri/labeled/encode.go` - DecodeNLRIHex (strings.Builder to map)
- `internal/component/bgp/plugins/nlri/labeled/labeled.go` - RunCLIDecode, RunDecodeMode callers
- `internal/component/bgp/plugins/nlri/ls/plugin.go` - inline OnDecodeNLRI lambda
- `internal/component/bgp/plugins/cmd/update/update_text_test.go` - test helpers
- `docs/architecture/api/process-protocol.md` - wire format docs
- `docs/plugin-development/handlers.md` - SDK handler examples
