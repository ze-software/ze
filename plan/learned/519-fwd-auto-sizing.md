# Learned: fwd-auto-sizing

Replaced the forward pool's three disconnected allocation systems (global
bufMuxStd, global bufMuxExt, static chan struct{} overflow token pool) with
a two-tier model: per-peer pools + single shared overflow MixedBufMux.

## What Worked

- **Single-pool subdivision design**: one pool of 64K blocks, each subdivisible
  into 16 x 4K slices via bitmask. No memory partitioning between 4K/64K paths.
  Blocks return to a shared free list and can serve either mode next time.

- **Three-level slicing for zero per-item allocation**: chunk (make[]byte, N*64K),
  block (64K slice into chunk), slice (4K slice into block). Hot path is bitmask
  pop + slice arithmetic. Only chunk growth allocates.

- **Stable block IDs via tombstoning**: block ID = slot index in a slice, never
  moved. Collapsed blocks are nil'd, slot indices reused on growth. Avoids the
  swap-delete corruption bug that was caught in self-review.

- **Budget bounds live blocks, not bytes handed out**: UsedRatio = liveBlocks /
  maxBlocks reflects real memory pressure for the congestion controller's 80%
  denial and 95% teardown thresholds.

- **overflowPoolBudget() pure sizing formula**: restart-burst * capped fan-out +
  10% steady-state. Isolated, testable, wired into weightTracker callback.

## What Failed

- **First implementation was two separate pools with shared counter**: spec
  explicitly said "avoids maintaining two separate pools" and I built two
  independent BufMux instances. Agreed to the spec then silently substituted a
  different design. This is the spec's #1 documented failure mode.

- **First dispatch wiring left the new code unwired**: built the types and tests
  but the dispatch path still used the legacy chan struct{} pool. PoolUsedRatio
  read from the empty new pool (always 0.0), effectively disabling the
  congestion controller. Deep review caught this.

- **Swap-delete corrupted block IDs**: first single-pool implementation used
  swap-delete in tryCollapse, which moved blocks and changed their IDs.
  Outstanding BufHandles carried stale IDs pointing to wrong blocks. Self-review
  caught this before commit.

- **Chunk memory pinned forever**: kept a m.chunks slice holding every chunk ever
  allocated. Even when all blocks were tombstoned, the chunks slice prevented GC.
  Fix: delete the chunks slice entirely -- block backing slices pin the array via
  Go GC slice rules.

## Key Decisions

- Per-peer pool is a 64-slot atomic counter (not actual buffers). Gates
  TryDispatch. peerPoolRef stored directly on fwdItem for lock-free release.

- Block metadata allocated contiguously alongside backing memory:
  make([]overflowBlock, N) per chunk. Zero per-block heap allocation.

- minBlocks = 0 (collapse everything). Regrowth is amortized (16 blocks per
  chunk). Could set to overflowChunkBlocks for a warm pool if needed.

- ze.fwd.pool.size default changed from 100000 (legacy token count) to 0
  (auto-sized from peer prefix maximums).

## Deferred

- Global bufMuxStd/bufMuxExt (read-path pools in session.go) NOT deleted. These
  serve pre-OPEN reads and build buffers, not overflow dispatch. The spec
  conflated them with the overflow pool. They should remain as-is.

- No .ci functional test for two-tier dispatch. Needs ze-test infrastructure
  with slow-consumer simulation. Unit tests cover all paths.

- ExtMsg re-registration at session establishment fires RegisterPeerPool again
  with 64K, which replaces the 4K pool created at AddPeer. Items in flight with
  the old pool's peerPoolRef release correctly (atomic counter on orphaned
  struct, GC'd after all refs gone).
