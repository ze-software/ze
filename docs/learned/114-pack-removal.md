# 114 — PackContext and Pack() Removal from NLRI

## Objective

Remove `PackContext` and the `Pack(ctx *PackContext) []byte` method from NLRI types, replacing them with the existing `WriteTo(buf, off) int` pattern. Net result: -486 lines across 44 files.

## Decisions

- `PackContext` (containing `AddPath bool`, `ASN4 bool`) was a simplified duplicate of `EncodingContext`. Removed entirely.
- `WriteTo(buf, off, ctx)` → `WriteTo(buf, off)` — ctx parameter removed from NLRI WriteTo; callers pass `addPath bool` directly.
- `ToPackContext(family)` removed from `EncodingContext`.

## Patterns

- `supportsAddPath(n)` check added to `WriteNLRI`: FlowSpec and BGPLS do not support ADD-PATH even when `addPath=true` is passed — per-type guard required.

## Gotchas

- `WriteNLRI` had a bug: it was adding path-id for ALL NLRI types when `addPath=true`, but FlowSpec and BGPLS cannot carry path-id. Fixed by adding `supportsAddPath(n)` check.
- `TestWireNLRI_Len`: test expected `Len()` to return full data length (8) but interface contract says payload only (4). Test expectation was wrong — fixed.
- `TestBuildGroupedUnicast_WithAddPath`: had `AddPath=false` but expected ADD-PATH behavior — wrong parameter in test.

## Files

- `internal/bgp/nlri/pack.go` — deleted
- `internal/bgp/nlri/nlri.go` — `WriteNLRI` fixed with `supportsAddPath` guard
- All 8 NLRI type files — `Pack()` removed, `WriteTo` signature updated
