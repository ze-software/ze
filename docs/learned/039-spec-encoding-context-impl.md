# 039 — EncodingContext Implementation

## Objective

Create the `internal/bgp/context/` package implementing `EncodingContext`, `ContextID`, and `ContextRegistry` as designed in spec 038, as a prerequisite for zero-copy route forwarding.

## Decisions

- `ContextRegistry` uses dual-map deduplication: `byHash map[uint64]ContextID` for dedup lookup, `contexts map[ContextID]*EncodingContext` for retrieval. FNV-64a for hashing.
- Hash must sort map entries before hashing (AddPath map, ExtendedNextHop map) to be deterministic — iteration order of Go maps is not guaranteed.
- Global `var Registry = NewRegistry()` provided as convenience; callers that need isolation can construct their own.
- Peer integration (storing `recvCtxID`/`sendCtxID` on `Peer`) is explicitly out of scope — this spec delivers the package only.

## Patterns

- `EncodingContext.ToPackContext(family)` bridges from the context package back to `nlri.PackContext`, allowing incremental adoption.
- `FromNegotiated(neg *capability.Negotiated)` extracts encoding-relevant fields from the full session negotiation result.

## Gotchas

- Concurrent access requires `sync.RWMutex` in `ContextRegistry`: writes (Register) take a write lock, reads (Get) take a read lock.
- Hash collision is theoretically possible with FNV-64a — treated as acceptable given the small number of distinct contexts in production (typically <100).

## Files

- `internal/bgp/context/context.go` — `EncodingContext`, `Family`, `Hash()`, helpers
- `internal/bgp/context/registry.go` — `ContextRegistry`, `ContextID`, global `Registry`
- `internal/bgp/context/context_test.go`, `registry_test.go` — TDD tests
