# 113 — EncodingContext Consolidation

## Objective

Consolidate `WireContext` (new, references sub-components) and `EncodingContext` (old, flat struct) into a single type named `EncodingContext`, keeping the WireContext implementation.

## Decisions

- Keep the name `EncodingContext` over `WireContext` — it was the established name and most consumers used it.
- Field access converted to method calls: `ctx.ASN4` → `ctx.ASN4()`, `ctx.IsIBGP` → `ctx.IsIBGP()`, `ctx.LocalAS` → `ctx.LocalASN()`, `ctx.AddPath[f]` → `ctx.AddPath(f)`.
- Added helper constructors `EncodingContextForASN4(bool)` and `EncodingContextWithAddPath(bool, map)` for tests that don't have a full `Negotiated`.
- `AddPathFor()` kept as alias for `AddPath()` for API compatibility.

## Patterns

- Merging files: `wire.go` content moved into `context.go`, `wire_test.go` merged into `context_test.go`, then originals deleted. Clean merge with no semantic change.

## Gotchas

- Import cycle prevented placing `WireWriter` in `internal/bgp/wire` package: `wire → context → nlri → wire`. Moved to `context` package instead.

## Files

- `internal/bgp/context/context.go` — renamed from `WireContext`, removed old flat struct
- `internal/bgp/context/negotiated.go` — `FromNegotiatedRecvWire` → `FromNegotiatedRecv`, `FromNegotiatedSendWire` → `FromNegotiatedSend`
- `internal/bgp/context/wire.go` — deleted (merged into context.go)
