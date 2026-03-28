# 450 — Generic Element Iterator

## Objective

Create a generic zero-allocation element iterator at `internal/iter/` for walking variable-length records in byte slices, and adopt it consistently across chunking/splitting code and inline attribute TLV walks.

## Decisions

- `iter.Elements` is a value type (not pointer) — stays on stack, zero allocations
- `maxSize` is NOT a parameter of the iterator — slice length is the only boundary; chunking is caller's responsibility
- `ChunkMPNLRI` keeps `[][]byte` return type — uses `iter.Elements` internally but preserves API to avoid 40+ caller changes
- `NLRISizeFunc` stays in BGP package — domain-specific, cast to `iter.SizeFunc` at call site via `NewNLRIElements`
- Dead `ChunkNLRI` (copy-based IPv4-only) deleted — no production callers, superseded by `ChunkMPNLRI`
- Existing domain iterators (`NLRIIterator`, `AttrIterator`, `ASPathIterator`) NOT replaced — they return parsed fields, serve different purpose

## Patterns

- **Offset tracking for AttrIterator**: capture `iter.Offset()` before and after `Next()` to get `[start:end]` byte ranges without re-implementing TLV parsing. Enables in-place mutation, byte range removal, and two-pass measure+copy.
- **Zero-copy mutation through subslices**: `AttrIterator.Next()` returns `value` as a subslice of the original buffer. Writing to `value[i]` writes directly to the underlying buffer — no offset arithmetic needed.
- **Auto-linter cooperation**: goimports adds imports when usage is introduced. Change function body to use `attribute.NewAttrIterator` and goimports adds the import automatically.

## Gotchas

- Auto-linter removes unused imports — must add import and usage in same edit or rely on goimports to add it
- `ChunkNLRI` test deletion: first pass missed `TestChunkNLRI_VariablePrefixLengths` at line 312 — caught by lint error
- `attr_discard.go` functions use `uint8` for type codes while `AttrIterator` uses `AttributeCode` — need type casts at comparison points

## Files

- `internal/iter/iter.go` — created (generic element iterator)
- `internal/iter/iter_test.go` — created (10 tests)
- `internal/component/bgp/message/chunk_mp_nlri.go` — modified (uses iter.Elements internally)
- `internal/component/bgp/message/update_split.go` — modified (findMPAttribute uses AttrIterator)
- `internal/component/bgp/message/attr_discard.go` — modified (5 functions use AttrIterator, −126 lines)
- `internal/component/bgp/message/update.go` — modified (ChunkNLRI deleted)
- `internal/component/bgp/message/fuzz_test.go` — modified (FuzzChunkNLRI → FuzzChunkMPNLRI)
