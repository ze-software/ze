# 034 — ADD-PATH Encoding Fix

## Objective

Fix RFC 7911 ADD-PATH encoding so NLRI wire format includes the 4-byte Path Identifier when ADD-PATH is negotiated, fixing test R (path-information).

## Decisions

- Introduced `Pack(ctx *PackContext) []byte` method on the `NLRI` interface alongside the existing `Bytes() []byte`. `Bytes()` is kept for internal use (RIB keys, dedup hashing); `Pack` is for wire encoding.
- `PackContext` is defined in the `nlri` package (not `message`) to avoid circular imports — `message` imports `nlri` for EOR building.
- Chose `Pack(ctx *PackContext)` over `PackNLRI(addpath bool)` because `PackContext` is extensible (ASN4, ExtendedNextHop can be added later without changing signatures).
- `Pack(nil)` behaves identically to `Bytes()` for safety.

## Patterns

- ADD-PATH send=true, NLRI has path ID → include the existing path ID. ADD-PATH send=true, NLRI has NO path ID → prepend NOPATH (4 zero bytes). ADD-PATH not negotiated → strip path ID if present.
- `CommitService.packContext(family)` builds a `PackContext` from the per-peer `Negotiated` struct.

## Gotchas

- Cannot use `*message.Negotiated` directly in `nlri` package — circular import. This forced the `PackContext` intermediary struct in the `nlri` package.
- `Bytes()` must remain for non-wire uses (RIB dedup, indexing) — do not replace all call sites.

## Files

- `internal/bgp/nlri/pack.go` — `PackContext` struct (new file)
- `internal/bgp/nlri/nlri.go` — `Pack(ctx *PackContext)` added to NLRI interface
- `internal/bgp/nlri/inet.go` — `INET.Pack()` implementation
- `internal/rib/commit.go` — migrated to use `Pack(ctx)`
