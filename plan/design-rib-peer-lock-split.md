# Design: split RIBManager.mu and drop r.mu from the best-path hot path

Implemented 2026-04-20 (commit 51c883952). Document updated 2026-05-25 to
reflect the final state.

## Why

Phase 4 sharding (landed 2026-04-20) moved `bestPrev` out from under
`RIBManager.mu` onto per-prefix shard locks, and gave the
`bestPathInterner` its own per-table mutexes. But `checkBestPathChange`
still ran inside the caller's `r.mu.Lock()` region in
`rib_structured.go::handleReceivedStructured`, because two helpers
reached from inside it still needed `r.mu`:

- `gatherCandidates` iterates `r.bgpPeers` (cached subset of ribInPool for BGP protocol peers).
- `bestCandidateNextHopAddr` reads `r.bgpPeers[best.PeerAddr]`.

So in production every UPDATE from any peer serialized on `r.mu` for
the whole `peerRIB.Insert` + `checkBestPathChange` block. The new
per-shard locks never saw contention -- a single full-feed peer drove
the outer lock 100% and the shards stayed idle behind it.

The goal: two peers' UPDATEs for different prefixes (and typically
different shards) should process concurrently. We only need mutual
exclusion over the peer-keyed **map** operations, not over PeerRIB
content (every PeerRIB has its own `sync.RWMutex` internally already,
per `internal/component/bgp/plugins/rib/storage/peerrib.go:16`).

## Current state (verified against code, 2026-05-25)

`RIBManager.peerMu` (rib.go:307) is a `sync.RWMutex` that guards
the peer-keyed maps:

| Field | Shape | Line |
|-------|-------|------|
| `ribInPool` | `map[ProtocolID]map[string]*storage.PeerRIB` | rib.go:215 |
| `bgpPeers` | `map[string]*storage.PeerRIB` (cached ribInPool[bgpProtocolID]) | rib.go:220 |
| `ribOut` | `map[string]map[Family]map[string]*Route` | rib.go:224 |
| `peerUp` | `map[string]bool` | rib.go:227 |
| `peerMeta` | `map[string]*PeerMeta` | rib.go:230 |
| `retainedPeers` | `map[string]bool` | rib.go:237 |
| `grState` | `map[string]*peerGRState` | rib.go:242 |

`bestPrev` (rib.go:249) is **not** protected by `peerMu`; it uses per-shard locks.
`bestPathInterner` (rib.go:258) owns its own per-table `sync.RWMutex`.

PeerRIB internals are self-synchronized: a single `sync.RWMutex` inside
the PeerRIB struct (`storage/peerrib.go:16`) protects every `Insert` /
`Remove` / `Lookup` / `Iterate` / `Range` operation. No outer lock is
required to work with a PeerRIB pointer once the map-lookup hand-off
has happened.

## Constraints

- `PeerRIB` self-synchronizes. We can hand a `*PeerRIB` pointer to a
  reader and let that reader work with no outer lock.
- `ribInPool` / `bgpPeers` / `ribOut` / `peerUp` / `peerMeta` /
  `retainedPeers` / `grState` are all peer-keyed maps. Creation,
  deletion, and iteration of keys still need mutual exclusion.
- `gatherCandidates` iterates `bgpPeers` and for each peer reads
  that peer's adj-rib-in. The iteration itself needs a stable map
  view; the PeerRIB reads inside the loop are self-synchronized.
  `extractCandidate` (called from within `gatherCandidatesLocked`)
  also reads `r.peerMeta[peerAddr]` under the same RLock.
- A peer coming up or going down while an UPDATE is in flight is
  already a race today (the current `r.peerMu.Lock()` does not prevent a
  peer teardown from another goroutine -- the peer-up/down handler
  just queues on the same lock). The new design has the same
  semantics: `gatherCandidates` snapshots who was up at snapshot time.
- `rib_structured.go::handleReceivedStructured` runs on one peer
  goroutine. Mutations it performs on `bgpPeers` are limited to that
  one peer's slot (`bgpPeers[peerAddr]`).

## Shape

Renamed `RIBManager.mu` to `RIBManager.peerMu`. Its scope narrowed to
"the peer-keyed maps". The contract:

| Access | Lock |
|--------|------|
| Read `r.bgpPeers[k]` (pointer) | `r.peerMu.RLock()` |
| Iterate `r.bgpPeers` keys | `r.peerMu.RLock()` |
| Write `r.bgpPeers[k] = ...` or `delete()` | `r.peerMu.Lock()` |
| Same rule for `ribOut`, `peerUp`, `peerMeta`, `retainedPeers`, `grState` | `peerMu.Lock()` for writes, `peerMu.RLock()` for reads |
| Read/write PeerRIB content | `peerRIB.mu` (already owned) |
| Read/write bestPrev | per-shard lock (already owned, unchanged) |
| Read/write bestPathInterner | per-table lock (already owned, unchanged) |

`checkBestPathChange`, `gatherCandidates`, and
`bestCandidateNextHopAddr` acquire `r.peerMu.RLock()` internally for
the brief map read, then release it before working on PeerRIB content.
None of them call sh.mu.Lock or the interner under peerMu -- lock
ordering stays `peerMu -> shard.mu` (peerMu is always outer when held
together).

**Exception:** `handleStructuredState` calls `purgeBestPrevForPeer`
while still holding `peerMu.Lock()`. `purgeBestPrevForPeer` acquires
shard locks internally. This is correct ordering (`peerMu` outer,
`shard.mu` inner) but is the one path where `peerMu` is held while
touching shard locks.

Call-site surgery in `rib_structured.go`:

- Before: `r.mu.Lock()` wrapped the entire UPDATE processing block
  (peerRIB init + peerRIB.Insert/Remove loop + checkBestPathChange
  loop).
- After: the peerRIB-init step takes `r.peerMu.Lock()` briefly to
  create the PeerRIB if absent (rib_structured.go:104-124), releases,
  then the peerRIB.Insert / peerRIB.Remove loop and the
  checkBestPathChange loop run lock-free with respect to `r.peerMu`.
  Each of those helpers acquires `r.peerMu.RLock()` internally for the
  O(peers) map read.

Two peer goroutines processing UPDATEs for different peers now run in
parallel: the brief `peerMu.Lock()` on peerRIB init is taken only when
creating a new peer (O(few) per peer lifetime), and the
`peerMu.RLock()` readers in the hot path share the lock.

## What was done

1. **Mechanical rename** `r.mu` -> `r.peerMu` across
   `internal/component/bgp/plugins/rib/*.go`. Updated the field doc
   at rib.go:300-306 to state: "protects ribInPool, ribOut, peerUp,
   peerMeta, retainedPeers, grState -- PEER-KEYED maps only."
2. **Pushed peerMu RLock into the helpers.** `gatherCandidates`
   (rib_commands.go:953) and `bestCandidateNextHopAddr`
   (rib_bestchange.go:977) each take `r.peerMu.RLock()` for the brief
   map access, snapshot what they need (the `*PeerRIB` pointer plus
   `peerMeta` value via `extractCandidate`), release, then work.
3. **Relaxed the caller in `rib_structured.go`.** Replaced the big
   `r.mu.Lock() ... r.mu.Unlock()` block with: a brief
   `r.peerMu.Lock()` to lazy-init the peer's PeerRIB + peerMeta slot
   (lines 104-124), release, then process. `peerRIB.Insert` /
   `peerRIB.Remove` run under PeerRIB's own lock. Best-path work
   runs with no outer lock held.
4. **Audited every `r.peerMu.Lock()` site** in the rib plugin.
   13 write-lock sites and 13 read-lock sites verified (see review).
5. **Parallel-UPDATE stress tests.** Three tests validate the split:
   - `TestParallelCheckBestPathChangeNoLostWrites` (bestprev_shard_test.go:62):
     N goroutines, single peer, M prefixes each.
   - `TestParallelMultiPeerNoLostWrites` (bestprev_shard_test.go:186):
     N peer goroutines each with their own UPDATE stream.
   - `TestConcurrentDownVsUpdate` (bestprev_shard_test.go:114):
     races DOWN against in-flight UPDATEs.

## Known serialization points

- `updateMetrics` (rib.go:453) takes `peerMu.Lock()` (write lock) to
  iterate peers for gauge updates. If metrics collection runs
  frequently, it serializes against UPDATE processing. May warrant
  investigation whether `peerMu.RLock()` suffices.
- `handleSentStructured` (rib_structured.go:293) holds
  `peerMu.Lock()` + `defer Unlock()` for the duration of sent-UPDATE
  processing. Outbound processing is less latency-sensitive than
  inbound, so this is acceptable.

## Risks

- **Lock ordering.** Rule: `peerMu` is outer, `shard.mu` is inner,
  interner per-table locks are independent. `handleStructuredState`
  is the one path that holds peerMu while acquiring shard locks (via
  `purgeBestPrevForPeer`); this is correct ordering. No code path
  acquires shard.mu then later acquires peerMu.
- **PeerRIB pointer safety.** After `r.peerMu.RUnlock()`, a goroutine
  may still hold a `*PeerRIB` pointer to work with. A concurrent peer
  teardown (`SetPeerDown`) takes `r.peerMu.Lock()` to delete the map
  entry, but the PeerRIB struct stays allocated as long as the
  pointer holder has it. No use-after-free. Garbage collected only
  when the last pointer holder drops it.
- **`gatherCandidates` snapshot staleness.** Between the RLock
  release at the end of gatherCandidates and the interner mutation in
  checkBestPathChange, the peer set could change. The caller already
  captured the candidates list by value, so this is benign (same
  staleness window as before, just shifted a hair earlier).
- **`SetPeerDown`** deletes `bgpPeers[peerAddr]` under peerMu.Lock.
  If that races with a `gatherCandidates` that already snapshotted
  the peer pointer, the candidate record persists for this one
  best-path computation. Next computation will not see the peer.
  Same eventually-consistent semantics as before.

## Out of scope

- Sharding `bgpPeers` itself by peer-hash. Peer count is O(100s) at
  most; a single RWMutex around the map is fine.
- Per-peer goroutines for best-path computation. Each peer goroutine
  already drives its own UPDATE processing.
- `SetLocRIB` / the forward-handle observer lifecycle. Touches locRIB
  only, not peerMu.
