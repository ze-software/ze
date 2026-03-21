# 111 — AttributesWire Simplification

## Objective

Eliminate the redundant `parsed map[AttributeCode]Attribute` from `AttributesWire` by embedding the cached `Attribute` directly into the `attrIndex` struct, reducing memory usage ~53%.

## Decisions

- Mechanical refactor, no design decisions.

## Patterns

- Added `getAndParse()` helper with hint-based optimization: avoids double search when upgrading from read lock to write lock (the index position found under read lock is passed as hint to avoid re-scanning under write lock).
- Linear scan over `attrIndex` slice is acceptable for BGP attributes (n≈15): O(n) comparable to O(1) map for small n, and wire order is preserved (map loses order).

## Gotchas

- Memory reduction: 768 → 360 bytes for 15 attributes (~53%) by eliminating the separate map with its key+value pairs and map overhead.

## Files

- `internal/bgp/attribute/wire.go` — embedded `parsed Attribute` in `attrIndex`, removed `parsed map`, added `getAndParse()` helper
