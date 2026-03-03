# 078 — WireUpdate Split

## Objective

Implement wire-level UPDATE splitting (`SplitWireUpdate`) without parsing to Route objects, replacing the broken `ConvertToRoutes()` path that required never-populated fields.

## Decisions

- Chose simplicity over performance for splitting: splitting is a rare operation (only when UPDATE exceeds peer's max size), so correctness and readability take priority.
- "Accept invalid, emit valid" semantics: API may send UPDATEs with multiple MP_REACH attributes for different AFI/SAFIs (technically invalid), splitter normalises output to at most one MP_REACH + one MP_UNREACH per output UPDATE.
- Changed signature from `SplitUpdate(wu, maxBodySize, addPath bool)` to `SplitWireUpdate(wu, maxBodySize, srcCtx *EncodingContext)`: ADD-PATH is negotiated per AFI/SAFI, not globally; the full context enables per-family lookup.
- IPv4 routes are included only in the first iteration when multiple MP_* attributes exist — this is the only semantically correct approach since the NEXT_HOP attribute applies to IPv4 unicast.

## Patterns

- Attribute separation passes through the raw attribute bytes once, returning `baseAttrs`, `[]mpReach`, `[]mpUnreach` as subslices.
- NLRI splitters (`prefixNLRISplitter`, `vpnNLRISplitter`, `flowspecNLRISplitter`) dispatched by AFI/SAFI key — existing `ChunkMPNLRI` code reused.

## Gotchas

- Initial spec assumed `addPath bool` was sufficient; changed to `*EncodingContext` because ADD-PATH is per-family asymmetric.
- Added infinite loop guard (`madeProgress` tracking) when `baseAttrs > maxSize` — without it, the slow path loops forever.
- FlowSpec length encoding uses 2-byte format when length >= 240 (not 256): easy to misread the RFC.

## Files

- `internal/plugin/wire_update_split.go` — SplitWireUpdate, separateMPAttributes, buildCombinedUpdates
- `internal/reactor/reactor.go` — ForwardUpdate uses SplitWireUpdate
