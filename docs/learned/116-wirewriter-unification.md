# 116 — WireWriter Unification

## Objective

Unify Message and Attribute interfaces around a common `WireWriter` interface (`Len(ctx) int`, `WriteTo(buf, off, ctx) int`) using `*EncodingContext` for all encoding decisions. Move `ExtendedMessage` from `SessionCaps` to `EncodingCaps` so `EncodingContext` can expose it.

## Decisions

- `ExtendedMessage` moved to `EncodingCaps` because it affects wire encoding (max message size 4096 vs 65535), not session negotiation metadata. `EncodingContext` already references `EncodingCaps` so it gains `ExtendedMessage()` and `MaxMessageSize()` automatically.
- `WireWriter` placed in `internal/bgp/context` package, NOT `internal/bgp/wire` — import cycle prevented: `wire → context → nlri → wire`.
- Attribute interface update deferred — attributes already had `WriteTo` methods; full unification left to a follow-up.
- Pack() kept on Message interface in this spec but removed in spec 115 (pack-removal).

## Patterns

- Context-independent types (Keepalive) accept `ctx *EncodingContext` and ignore it — uniform interface enables generic handling.
- `Transcoder` interface added for AS_PATH and Aggregator which need source context for ASN2↔ASN4 transcoding.

## Gotchas

- Import cycle: `WireWriter` cannot live in the `wire` package due to `wire → context → nlri → wire`. Discovered during implementation; moved to `context` package.

## Files

- `internal/bgp/context/context.go` — `WireWriter` interface, `ExtendedMessage()`, `MaxMessageSize()` added
- `internal/bgp/capability/encoding.go` — `ExtendedMessage bool` added, moved from `session.go`
- All 5 message types — `Len(ctx)` and `WriteTo(buf, off, ctx)` implemented
