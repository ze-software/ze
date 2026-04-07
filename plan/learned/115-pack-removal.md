# 115 — Pack() Removal from Message Types

## Objective

Remove deprecated `Pack()` method from all BGP message types and the `message.Negotiated` shim struct. All callers migrate to `WriteTo(buf, off, ctx)` with `EncodingContext`.

## Decisions

- `PackTo(msg, ctx)` convenience helper retained — allocates a buffer and calls `WriteTo`, useful for callers that don't pre-allocate (tests, one-off sends). Not in hot path.
- `message.Negotiated` struct deleted — was an ephemeral conversion shim created at every `writeMessage()` call.
- `message.Family` struct deleted — duplicate of `family.Family`.
- `rib/commit.go` migrated from `*message.Negotiated` to `*bgpctx.EncodingContext` — uses `ctx.AddPath(family)`, `ctx.IsIBGP()`, `ctx.LocalASN()` directly.

## Patterns

- `testContext()` helper added to `commit_test.go` for constructing `EncodingContext` in tests without a full `Negotiated`.

## Gotchas

None.

## Files

- `internal/bgp/message/message.go` — `Pack` removed from interface, `Negotiated` and `Family` structs deleted
- `internal/bgp/message/{keepalive,open,update,notification,routerefresh}.go` — `Pack()` removed
- `internal/rib/commit.go` — migrated to `*bgpctx.EncodingContext`
- `internal/reactor/peer.go` — `messageNegotiated()` helper removed
