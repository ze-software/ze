# 783: RIB peerMu lock split

Spec: design-rib-peer-lock-split

## Context

Phase 4 sharding moved bestPrev onto per-prefix shard locks, but every UPDATE
still serialized on a single `RIBManager.mu` because `gatherCandidates` and
`bestCandidateNextHopAddr` iterated the peer-keyed maps from inside the locked
region. A full-feed peer held the outer lock 100% and the shards sat idle.

## Decisions

**Narrow the lock to peer-keyed maps only.** Renamed `r.mu` to `r.peerMu`. Its
scope covers `ribInPool`/`bgpPeers`, `ribOut`, `peerUp`, `peerMeta`,
`retainedPeers`, `grState`. PeerRIB content, bestPrev shards, and the
bestPathInterner each own their own locks.

**Push RLock into the helpers.** `gatherCandidates` and `bestCandidateNextHopAddr`
acquire `r.peerMu.RLock()` for a brief map read, snapshot what they need (the
`*PeerRIB` pointer and peerMeta), release, then work. `extractCandidate` reads
`r.peerMeta` under the same RLock. `checkBestPathChange` runs with no outer lock.

**Three-phase UPDATE handler.** Phase 1: brief `peerMu.Lock()` to lazy-init the
peer's PeerRIB + peerMeta slot. Phase 2: `peerRIB.Insert`/`Remove` under PeerRIB's
own lock, no peerMu held. Phase 3: `checkBestPathChange` with no outer lock; helpers
take RLock internally.

**bgpPeers fast-path cache.** The hot path uses `r.bgpPeers` (a cached reference to
`ribInPool[bgpProtocolID]`), not the full `ribInPool` map. Smaller map means shorter
RLock hold time.

## Gotchas

- `handleStructuredState` calls `purgeBestPrevForPeer` while holding `peerMu.Lock()`,
  and `purgeBestPrevForPeer` acquires shard locks. This is the one path where peerMu
  is held while touching shard locks. Lock ordering is correct (peerMu outer, shard.mu
  inner) but must be preserved by any future refactor.

- `updateMetrics` takes `peerMu.Lock()` (write lock) to iterate peers. If metrics
  collection frequency increases, this becomes a serialization point against UPDATEs.
  An RLock may suffice here but was not changed.

- After `r.peerMu.RUnlock()`, a goroutine still holds a `*PeerRIB` pointer. A
  concurrent peer-down deletes the map entry but the PeerRIB struct stays alive (GC).
  Writes to the orphan PeerRIB are wasted work, not lost state, because Phase 3's
  `checkBestPathChange` emits withdraws for every prefix whose best came from the
  now-absent peer.

## Test coverage

Three stress tests in `bestprev_shard_test.go`:
- `TestParallelCheckBestPathChangeNoLostWrites`: N goroutines, single peer, M prefixes.
- `TestParallelMultiPeerNoLostWrites`: N peer goroutines, each with own UPDATE stream.
- `TestConcurrentDownVsUpdate`: races DOWN against in-flight UPDATEs.

## Files

None recorded.
