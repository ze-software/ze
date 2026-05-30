# 823 -- rib-show-bounded-dump

## Context

`show bgp rib` materialized the entire Adj-RIB-Out into `[]RouteItem` at construction and held `peerMu.RLock` across the full pipeline build + drain (source creation, filter application, terminal serialization, json.Marshal). For route-server-scale tables (hundreds of thousands of routes), this caused peak memory proportional to the full table and starved UPDATE writers for the dump's duration. The same lock-hold-across-drain pattern existed in `bestPipeline`.

## Decisions

- Chose per-peer lazy buffering for outboundSource over eager full-table materialization, because the pipeline is already iterator-based (per-peer loading fits the `Next()` pull model) and peak memory is bounded to one peer's routes instead of the whole table.
- Chose per-peer peerMu snapshots over holding peerMu for the full pipeline, because PeerRIB has its own `sync.RWMutex` (IterateSorted, Lookup, IsAddPath all self-lock), so peerMu is only needed to read the peer maps, not for PeerRIB iteration or pool reads.
- Kept bestSource eager (not lazy) over per-peer lazification, because best-path selection needs all candidates across all peers for a given prefix before selecting a winner; the cross-peer aggregation is fundamental.
- Chose bestPipeline lock boundary at newBestSource return (before filter/terminal drain) over holding lock for full pipeline, because filters and terminals only read pool handles and RouteEntry values (captured by value), which are safe without peerMu.
- Kept textbuf.Hex for ext-community formatting (no pooled buffer) over pooled textbuf.Get/Release, because both paths produce one heap allocation per string (the pool overhead provides no measurable alloc reduction since stack allocations are free). Community.String() already uses stack buffers efficiently; textbuf cannot improve on it without modifying the attribute package.

## Consequences

- Mid-dump mutations are visible at per-peer granularity (not whole-table atomic). This matches typical `show` semantics and is not a regression (no whole-table atomicity existed before either).
- Terminal accumulation (jsonTerminal.drain building the full map before json.Marshal) is unchanged. Streaming marshal is a separate optimization.
- Future lock-scope reductions should follow the same pattern: snapshot peerMu-protected references, then operate on them via their own locks (PeerRIB) or thread-safe pool reads.

## Gotchas

- Go's sync.RWMutex allows concurrent RLock from different goroutines, but recursive RLock from the same goroutine can deadlock if a writer is waiting. The existing code (IterateSorted callback calling IsAddPath) already does this. The new design avoids the issue by releasing peerMu before PeerRIB iteration.
- The outbound source snapshot must capture both ribOutEntry values AND source peer strings under peerMu, because ribOutSourcePeer reads from ribOutSource which is peerMu-protected.
- Community.String() already uses `appendCommunityText` with a stack [11]byte buffer, so pooled textbuf cannot reduce per-community allocations without modifying the attribute package.

## Files

- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- lazy outboundSource, inboundSource snapshots PeerRIB refs, showPipeline lock removed
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` -- bestPipeline lock held only for newBestSource
- `internal/component/bgp/plugins/rib/rib_attr_format.go` -- unchanged (textbuf pooling reverted: no measurable benefit)
- `internal/component/bgp/plugins/rib/rib_pipeline_test.go` -- TestShowOutboundSourceLazy, TestShowJSONContractUnchanged, TestShowPipesUnchanged, BenchmarkShowLargeTable
- `internal/component/bgp/plugins/rib/rib_pipeline_best_test.go` -- TestBestPipelineLockReduced
