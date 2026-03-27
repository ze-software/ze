# 445 -- Forward Congestion Phases 4-5: Weight Sizing, Backpressure, Teardown

## Context

Phases 1-3 established the overflow pool, metrics, Send Hold Timer, and pool multiplexer. But the system had no enforcement -- one slow destination peer could pin every buffer in the shared pool, freezing all source peers' read loops. Phase 4 added weight-based dynamic sizing (burst fraction, peer prefix counts, auto-sized budget). Phase 5 added the two-threshold enforcement that prevents pool monopolisation: buffer denial for the worst offender at 80% usage, and forced GR-aware teardown at 95% after a grace period. Route superseding (AC-23) and withdrawal priority (AC-25) were also implemented as part of this work.

## Decisions

- Chose **two-threshold enforcement** (soft denial at 80%, hard teardown at 95% + 2x weight + 5s grace) over a single threshold. The soft threshold creates TCP backpressure via buffer denial; the hard threshold reclaims resources when backpressure is insufficient.
- Chose **worker-driven teardown check** (in `runWorker` after each batch) over a timer goroutine. No new goroutines needed -- the check piggybacks on the existing worker loop. The stuck worker detects its own situation.
- Chose **GR-aware teardown** (TCP close without NOTIFICATION for GR peers, Cease/OutOfResources for non-GR) over uniform NOTIFICATION. Per RFC 4724 Section 4, TCP failure without NOTIFICATION triggers route retention, which is strictly better for GR peers.
- Chose **content-hash superseding** (FNV-1a of raw body bytes) over per-prefix NLRI parsing. Per-prefix dedup would require NLRI parsing on the forward path, contradicting ze's UPDATE-first zero-copy design. Content-hash catches exact duplicates (the common case during re-advertisements) without parsing overhead.
- Chose **stable-partition reordering** for withdrawal priority over in-place sorting. Withdrawals move before announcements while preserving order within each group.
- Chose **headroom as additive** (`total = auto + headroom`) over multiplicative. Operators on large-memory machines set an absolute byte amount, not a percentage.

## Consequences

- One frozen peer can no longer freeze the system. After 30s write deadline + 5s grace, the worst peer is torn down and all its buffers reclaimed.
- The Send Hold Timer (8min+) remains as a second safety net for scenarios the congestion logic doesn't catch.
- Route superseding bounds overflow growth to unique UPDATE content, not total update count. This matters during convergence events where the same route is re-advertised multiple times.
- The `peerGRCapable` callback in the congestion controller iterates all peers by string comparison -- O(N) per teardown check. Acceptable because the check only fires when pool > 95% (rare path).

## Gotchas

- **Pre-existing race in `updateBufMuxBudget`:** Reading the `budget` pointer field of `bufMux4K.mux` without holding the mutex raced with `SetBudget` writes from concurrent reactor initialization in tests. Fixed by acquiring `mu` for the pointer read.
- **Supersede token accounting:** When both old and new items are pooled, releasing the old token and keeping the new is correct. An initial implementation released both (double-release), leaving the surviving item with no token.
- **`fwdIsWithdrawal` raw body parsing:** The UPDATE wire format has Unfeasible Routes Length at offset 0. A withdrawal-only UPDATE has non-zero withdrawn length AND no NLRI section after attributes. Must check both conditions -- a combined announce+withdraw UPDATE is NOT a withdrawal for priority purposes.

## Files

- `internal/component/bgp/reactor/forward_pool_congestion.go` (NEW) -- congestionController, ShouldDeny, CheckTeardown, congestionTeardownPeer
- `internal/component/bgp/reactor/forward_pool_congestion_test.go` (NEW) -- 11 tests for buffer denial, teardown, GR, grace period
- `internal/component/bgp/reactor/forward_pool_supersede_test.go` (NEW) -- 9 tests for superseding, withdrawal detection, batch reordering
- `internal/component/bgp/reactor/forward_pool.go` -- supersedeKey/withdrawal fields, superseding in DispatchOverflow, reordering in safeBatchHandle, fwdSupersedeKey/fwdIsWithdrawal/fwdReorderWithdrawalsFirst helpers
- `internal/component/bgp/reactor/forward_pool_weight_tracker.go` -- WorstPeerRatio method
- `internal/component/bgp/reactor/reactor.go` -- congestion controller wiring, headroom, teardown grace env vars
- `internal/component/bgp/reactor/reactor_metrics.go` -- buffer denied + teardown counters
- `internal/component/bgp/reactor/reactor_api_forward.go` -- supersedeKey and withdrawal set on fwdItems
- `internal/component/bgp/reactor/session.go` -- race fix in updateBufMuxBudget
- `test/plugin/forward-congestion-teardown-metrics.ci` (NEW) -- functional test for Phase 5 metrics
- `docs/architecture/forward-congestion-pool.md` -- enforcement design, headroom config
- `docs/architecture/congestion-industry.md` -- updated Ze status
- `docs/features.md` -- congestion backpressure, GR teardown, headroom features
