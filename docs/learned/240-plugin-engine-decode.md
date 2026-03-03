# 240 — Plugin Engine Decode

## Objective

Add `ze-plugin-engine:decode-nlri` and `ze-plugin-engine:encode-nlri` RPCs so plugins can ask the engine to decode/encode NLRI hex strings.

## Decisions

- `handleCodecRPC()` shared helper avoids `dupl` lint violations for structurally similar decode/encode handlers.
- Registry `Snapshot()`/`Restore()` is the correct test isolation pattern for global compile-time registrations.
- `t.Cleanup(registry.Reset)` was wrong — leaves an empty registry, breaking tests that rely on registered decoders.

## Patterns

- Snapshot/Restore for global registry: `snap := registry.Snapshot(); t.Cleanup(func() { registry.Restore(snap) })`.
- `handleCodecRPC()` helper: shared RPC handler body accepting a function parameter for the codec operation.

## Gotchas

- `t.Cleanup(registry.Reset)` empties the registry entirely — all globally registered decoders disappear for subsequent tests in the same binary run.
- Always Snapshot before test modifications, Restore on cleanup.

## Files

- `internal/plugins/bgp/handler/` — decode-nlri, encode-nlri RPC handlers
- `internal/plugin/registry/` — Snapshot/Restore for test isolation
