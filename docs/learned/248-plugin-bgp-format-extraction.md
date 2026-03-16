# 248 — Plugin BGP Format Extraction

## Objective

Extract `text.go`, `json.go`, `server_bgp.go`, `validate_open.go` from `internal/component/plugin/` into `internal/component/bgp/format/` and `internal/component/bgp/server/` using a BGPHooks callback struct pattern to break the circular import.

## Decisions

- BGPHooks as a struct of function callbacks (not interface): avoids forcing a single implementor; generic server calls hooks when set, no-ops when nil.
- Codec handler tests moved to `bgp/server/codec_test.go` as direct function tests — dispatch tests would create import cycles since they need the `format` package.
- BGP code registers hooks at server creation via `ServerConfig.BGPHooks`; reactor wires `NewBGPHooks()` at startup.

## Patterns

- Import cycle resolution: test files must live with the functions they test, not with the types they use.
- Hook-based DI: generic infra defines callback types, BGP layer registers implementations at startup via `ServerConfig`.

## Gotchas

- `writeJSONEscapedString` bug: missing `continue` after switch cases caused special characters to be double-written (once in case, once in default).
- `onMessageSent` had an unused `encoder` parameter that lint caught — remove unused params immediately.

## Files

- `internal/component/plugin/types.go` — `BGPHooks` struct + 6 callback function types
- `internal/component/plugin/server.go` — `bgpHooks` field + dispatch through hooks
- `internal/component/bgp/format/text.go`, `json.go`, `codec.go` — all Format* functions and JSONEncoder
- `internal/component/bgp/server/events.go`, `codec.go`, `validate.go`, `hooks.go` — event dispatch, codec RPCs, OPEN validation, `NewBGPHooks()` factory
