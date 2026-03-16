# 185 — Config JSON Delivery

## Objective

Replace pattern-based config delivery with full JSON delivery: plugins declare which config
roots they want (`declare wants config bgp`), receive JSON at Stage 2 and on reload.

## Decisions

- Declaration is opt-in per root: `declare wants config <root>`. No config sent unless declared.
- Delivery format: `config json <root> <json>` per root, then `config done`.
- Reload format: `config reload json <root> <json>` then `config reload done`.
- Config tree stored in `BGPConfig.ParsedTree`, accessed via `ReactorInterface.GetConfigTree()`.
- `DiffMaps()` (VyOS-style) for deep diff — `ConfigDiff` with Added/Removed/Changed.
- Capability decoding is plugin-gated: `ze bgp decode --plugin <name>` spawns plugin with `--decode` flag. Without plugin, unknown capabilities show `{"name":"unknown","code":N,"raw":"..."}`.
- `CapabilityConfigJSON` / `RawCapabilityConfig` fields deferred for cleanup — existing code paths still use them.

## Patterns

- Plugins extract what they need from JSON using their own YANG knowledge — core never needs to know plugin config structure.
- `ze bgp plugin-test` debug command dumps schema fields, config tree, and JSON that would be delivered to plugins.

## Gotchas

- Config tree was delivered wrapped as `{"bgp":{...}}` — plugins needed unwrapping logic to access peer data directly.
- `Internal` flag not copied in plugin config conversion in `reactor.go` — field was missing from conversion code.

## Files

- `internal/component/plugin/registration.go` — `WantsConfigRoots`, `DecodeCapabilities`, parsing
- `internal/component/plugin/server.go` — `deliverConfig()` rewritten, `notifyConfigReload()`
- `internal/component/config/diff.go` — `DiffMaps()`, `ConfigDiff`, `DiffPair`
- `internal/component/config/diff_test.go` — full diff coverage
- `internal/component/config/bgp.go` — `ParsedTree` field added
- `cmd/ze/bgp/decode.go` — `--plugin` flag, plugin decode invocation
