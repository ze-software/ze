# 100 — ADD-PATH Support in Plugin System

## Objective

Propagate RFC 7911 ADD-PATH path-id through the plugin event format so RIB and other plugins can distinguish multiple paths to the same prefix.

## Decisions

- Context propagation reuses existing infrastructure (`AttributesWire.SourceContext()` → `EncodingContext`) rather than adding a new `AddPathReceive` map field to `RawMessage`. The `EncodingContext` already knows per-family ADD-PATH state.
- RIB route key extended to `family:prefix:path-id` when path-id > 0, enabling separate storage of multiple paths.
- JSON format: ADD-PATH NLRI becomes object `{"prefix":"...","path-id":N}` instead of bare string.

## Patterns

- Context flows: `Reactor.FromNegotiatedRecv(neg)` → ctx stored with attributes as `sourceCtxID` → plugin system calls `wire.SourceContext()` → `ctx.AddPathFor(family)` → `NLRIs(hasAddPath)`. No new fields needed anywhere.

## Gotchas

- Original spec proposed adding `AddPathReceive map[string]bool` to `RawMessage`. This was rejected — existing `SourceContext()` path already carries this information end-to-end.

## Files

- `internal/plugin/mpwire.go` — `MPReachWire.NLRIs(hasAddPath)`, `IPv4Reach.NLRIs(hasAddPath)`, etc.
- `internal/plugin/rib/rib.go` — route key with path-id, `parseNLRIValue` handles both formats
