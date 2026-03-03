# 037 — ASN4 in PackContext

## Objective

Add `ASN4 bool` to `PackContext` so AS_PATH encoding can use the same unified context as NLRI encoding, completing Phase 2 of the negotiated packing pattern.

## Decisions

- `PackContext` stays in the `nlri` package even though it now carries `ASN4` (an attribute concern), because moving it to a shared `encoding` package would require more churn for marginal benefit.
- Caller refactoring (removing the separate `asn4 bool` parameter from `buildRIBRouteUpdate` etc.) is deferred as optional cleanup — the field is available but callers can adopt it incrementally.

## Patterns

- `Peer.packContext()` and `CommitService.packContext()` both populate `ASN4` from their respective `Negotiated` structs. These two sites are the canonical source of pack context.

## Gotchas

- ZeBGP already implements AS_TRANS conversion in `aspath.go:169-174` via `PackWithASN4(bool)`. The `ASN4` field in `PackContext` is a convenience wrapper, not new logic.

## Files

- `internal/bgp/nlri/pack.go` — added `ASN4 bool` field to `PackContext`
- `internal/reactor/peer.go` — `packContext()` updated to set `ASN4`
- `internal/rib/commit.go` — `packContext()` updated to set `ASN4`
