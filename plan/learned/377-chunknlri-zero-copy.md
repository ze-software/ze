# 377 — ChunkMPNLRI Zero-Copy

## Objective

Convert `ChunkMPNLRI` from copy-based chunking (`append()`) to zero-copy subslice-based chunking, eliminating allocations in the hot path.

## Decisions

- Subsumed into `spec-iter-elements.md` — `ChunkMPNLRI` now uses `iter.Elements` internally, which yields subslices of the original buffer
- Return type stays `[][]byte` — each element is a subslice, not a copy
- The `chunks` slice header allocation is acceptable (16 bytes per entry, not wire data)

## Patterns

- **Subslicing is the correct buffer-first approach for splitting**: when data is already in wire format, return views into the existing buffer instead of copying into new ones
- **Offset tracking replaces append accumulator**: track `chunkStart` and current `offset`, emit `data[chunkStart:offset]` subslices
- `SplitMPNLRI` already demonstrated this pattern — `ChunkMPNLRI` now matches it

## Gotchas

- Original spec proposed `WriteTo(buf, off) int` — wrong approach for splitting (that's for encoding). Subslicing is simpler and faster when data is already in a buffer.

## Files

- `internal/component/bgp/message/chunk_mp_nlri.go` — modified (zero-copy via iter.Elements)
- `internal/component/bgp/message/chunk_mp_nlri_test.go` — added ZeroAlloc and SubsliceVerification tests
