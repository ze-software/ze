# 112 — Negotiated Composite Refactor

## Objective

Refactor `Negotiated` into a composite structure with reusable sub-components (`PeerIdentity`, `EncodingCaps`, `SessionCaps`) so that `WireContext` can reference them by pointer with zero data duplication.

## Decisions

- Sub-components are immutable after session creation and shared by pointer across `Negotiated` and both `WireContext` instances (send/recv).
- ADD-PATH requires direction-specific `addPath map[Family]bool` in each `WireContext` — derived from `EncodingCaps.AddPathMode` per direction.
- Hash includes direction so recv and send contexts get different registry IDs.
- Old `EncodingContext` and new `WireContext` coexist for gradual migration — backward compat maintained by keeping existing flat fields alongside new composite pointers.

## Patterns

- `WireContext` factory pattern: `Negotiated.RecvContext()` / `SendContext()` create direction-specific contexts sharing the same `*PeerIdentity` and `*EncodingCaps` refs.

## Gotchas

- Hash utilities were duplicated (`hashFamilyBoolMap` vs `hashFamilyBoolMapWire`) — acceptable because they work on different type parameters. Not unified.

## Files

- `internal/bgp/capability/identity.go` — `PeerIdentity`
- `internal/bgp/capability/encoding.go` — `EncodingCaps`
- `internal/bgp/capability/session.go` — `SessionCaps`
- `internal/bgp/context/wire.go` — `WireContext` (references sub-components, derives addPath per direction)
