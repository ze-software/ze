# 128 — Graceful Restart Plugin

## Objective

Create a standalone `bgp-gr` plugin that injects GR capability (RFC 4724) per-peer, removing all GR knowledge from the engine. Phase 1 (plugin code) was implemented; Phase 2 (plugin-driven schema parsing) was designed but not yet implemented.

## Decisions

- Chose to separate GR into its own plugin rather than keeping it in RIB: GR is capability injection only, not route storage.
- Plugin is stateless after startup — no runtime state needed beyond config parsed in Stage 2.
- Phase 2 design requires two-phase config parsing: parse `plugin {}` blocks first, start plugins, collect schema declarations, then parse remaining config using the plugin-extended schema. Engine must have zero hardcoded knowledge of GR capability.

## Patterns

- `deliverConfig()` in the engine delivers config to ALL peers with matching capability, regardless of `process` bindings. Process bindings are only for runtime events. Capability-only plugins don't need a `process` binding.
- RFC 4724 wire: `[R:1][Reserved:3][RestartTime:12]` = 2 bytes. Capability code 64.

## Gotchas

- Phase 1 plugin code exists but cannot receive config until Phase 2 engine changes land. The engine had no mechanism to deliver `graceful-restart { restart-time X; }` to a plugin — the schema was hardcoded in engine with no delivery path.
- Missing capability creation: config was parsed but `GracefulRestart` capability was never created in the engine, so `ConfigValues()` was never called. Root cause: engine and plugin were separately aware of GR but never connected.

## Files

- `internal/plugin/gr/gr.go` — GR plugin, 5-stage startup, RFC 4724 wire encoding
- `internal/plugin/gr/gr_test.go` — config parsing, wire format, startup tests
- `cmd/ze/bgp/plugin_gr.go` — CLI dispatch
- `internal/plugin/rib/rib.go` — GR code removed
