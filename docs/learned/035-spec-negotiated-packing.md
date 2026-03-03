# 035 — Negotiated Packing Pattern

## Objective

Audit and migrate all remaining `nlri.Bytes()` call sites in wire-encoding paths to use `nlri.Pack(ctx)` for consistent ADD-PATH awareness across all code paths.

## Decisions

- `Bytes()` is preserved for internal use (RIB indexing, dedup hashing) — only wire-encoding paths need `Pack(ctx)`.
- Two `Negotiated` structs coexist: `message.Negotiated` (wire encoding) and `capability.Negotiated` (full session state). They were not unified here; `PackContext` acts as the bridge.
- `Peer.packContext(family nlri.Family)` helper method converts `NegotiatedFamilies` to a `PackContext` for a specific family — only IPv4/IPv6 unicast ADD-PATH was initially supported; other families return nil.

## Patterns

- Sites that were "OK to keep Bytes()" are: `rib/outgoing.go` (indexing), `rib/store.go` (dedup hash), `api/commit_manager.go` (indexing) — none of these are wire encoding.
- Phase 2 (ASN4 in PackContext) and Phase 3 (other attributes) deferred to subsequent specs.

## Gotchas

- `reactor/reactor.go` API functions (`buildAnnounceUpdate`, `buildWithdrawUpdate`, `sendWithdrawals`) need access to peer negotiated state to build `PackContext` — required passing the peer or its context down to these functions.
- `rib/update.go:buildNLRIBytes` also needed a `ctx` parameter threaded through from the caller.

## Files

- `internal/bgp/nlri/pack.go` — `PackContext` with `AddPath` field
- `internal/reactor/peer.go` — `packContext()` helper, migrated builder calls
- `internal/reactor/reactor.go` — API route building migrated
- `internal/rib/update.go` — grouped update building migrated
