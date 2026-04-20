# Design: split RIBManager.mu and drop r.mu from the best-path hot path

Working design document. Implementation follows.

## Why

Phase 4 sharding (landed 2026-04-20) moved `bestPrev` out from under
`RIBManager.mu` onto per-prefix shard locks, and gave the
`bestPathInterner` its own per-table mutexes. But `checkBestPathChange`
still runs inside the caller's `r.mu.Lock()` region in
`rib_structured.go::handleReceivedStructured`, because two helpers
reached from inside it still need `r.mu`:

- `gatherCandidates` iterates `r.ribInPool` (map of peer -> PeerRIB).
- `bestCandidateNextHopAddr` reads `r.ribInPool[best.PeerAddr]`.

So in production every UPDATE from any peer serializes on `r.mu` for
the whole `peerRIB.Insert` + `checkBestPathChange` block. The new
per-shard locks never see contention -- a single full-feed peer drives
the outer lock 100 % and the shards stay idle behind it.

The goal: two peers' UPDATEs for different prefixes (and typically
different shards) should process concurrently. We only need mutual
exclusion over the peer-keyed **map** operations, not over PeerRIB
content (every PeerRIB has its own `sync.RWMutex` internally already,
per `internal/component/bgp/plugins/rib/storage/peerrib.go:16`).

## Current state (verified against code)

`RIBManager.mu` (rib.go:279) is a single `sync.RWMutex` that today
guards seven fields per the field doc:

| Field | Shape | Line |
|-------|-------|------|
| `ribInPool` | `map[string]*storage.PeerRIB` | rib.go:210 |
| `ribOut` | `map[string]map[string]map[string]*Route` | rib.go:214 |
| `peerUp` | `map[string]bool` | rib.go:217 |
| `peerMeta` | `map[string]*PeerMeta` | rib.go:220 |
| `retainedPeers` | `map[string]bool` | rib.go:227 |
| `grState` | `map[string]*peerGRState` | rib.go:232 |
| `bestPrev` (removed as of Phase 4) | `*bestPrevShards` | rib.go:239 |

The bestPrev field is still listed in the doc of `r.mu` (rib.go:279)
but no longer protected by it -- Phase 4 gave it per-shard locks. That
comment is stale and needs updating in step 1.

PeerRIB internals are already self-synchronized: a single
`sync.RWMutex` inside the PeerRIB struct
(`storage/peerrib.go:16`) protects every `Insert` / `Remove` /
`Lookup` / `Iterate` / `Range` operation. No outer lock is required
to work with a PeerRIB pointer once the map-lookup hand-off has
happened.

## Constraints

- `PeerRIB` already self-synchronizes. We can hand a `*PeerRIB`
  pointer to a reader and let that reader work with no outer lock.
- `ribInPool` / `ribOut` / `peerUp` / `peerMeta` / `retainedPeers` /
  `grState` are all peer-keyed maps. Creation, deletion, and
  iteration of keys still need mutual exclusion.
- `gatherCandidates` iterates `ribInPool` and for each peer reads
  that peer's adj-rib-in. The iteration itself needs a stable map
  view; the PeerRIB reads inside the loop are self-synchronized.
- A peer coming up or going down while an UPDATE is in flight is
  already a race today (the current `r.mu.Lock()` does not prevent a
  peer teardown from another goroutine -- the peer-up/down handler
  just queues on the same lock). The new design has the same
  semantics: `gatherCandidates` snapshots who was up at snapshot time.
- `rib_structured.go::handleReceivedStructured` runs on one peer
  goroutine. Mutations it performs on `ribInPool` are limited to that
  one peer's slot (`ribInPool[peerAddr]`).

## Shape

Rename `RIBManager.mu` to `RIBManager.peerMu`. Its scope narrows to
"the peer-keyed maps". The new contract:

| Access | Lock |
|--------|------|
| Read `r.ribInPool[k]` (pointer) | `r.peerMu.RLock()` |
| Iterate `r.ribInPool` keys | `r.peerMu.RLock()` |
| Write `r.ribInPool[k] = ...` or `delete()` | `r.peerMu.Lock()` |
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

Call-site surgery in `rib_structured.go`:

- Today: `r.mu.Lock()` wraps the entire UPDATE processing block
  (peerRIB init + peerRIB.Insert/Remove loop + checkBestPathChange
  loop).
- After: the peerRIB-init step takes `r.peerMu.Lock()` briefly to
  create the PeerRIB if absent, releases, then the peerRIB.Insert /
  peerRIB.Remove loop and the checkBestPathChange loop run lock-free
  with respect to `r.peerMu`. Each of those helpers acquires
  `r.peerMu.RLock()` internally for the O(peers) map read.

Two peer goroutines processing UPDATEs for different peers will now
run in parallel: the brief `peerMu.Lock()` on peerRIB init is taken
only when creating a new peer (O(few) per peer lifetime), and the
`peerMu.RLock()` readers in the hot path share the lock.

## Migration steps

1. **Mechanical rename** `r.mu` -> `r.peerMu` across
   `internal/component/bgp/plugins/rib/*.go`. Update the field doc
   at rib.go:279 to drop `bestPrev` from the protected list (it is
   sharded) and to state: "protects ribInPool, ribOut, peerUp,
   peerMeta, retainedPeers, grState -- PEER-KEYED maps only."
2. **Push peerMu RLock into the helpers.** `gatherCandidates`
   (rib_commands.go:791) and `bestCandidateNextHopAddr`
   (rib_bestchange.go:646) today assume the caller holds `r.mu` -- both
   read `r.ribInPool`. Update each to take `r.peerMu.RLock()` for the
   brief map access, snapshot what they need (the `*PeerRIB` pointer
   plus `peerMeta` value), release, then work. Update each helper's
   godoc.
3. **Relax the caller in `rib_structured.go`.** Replace the big
   `r.mu.Lock() ... r.mu.Unlock()` block around
   `handleReceivedPool` / `handleReceivedStructured` with: a brief
   `r.peerMu.Lock()` to lazy-init the peer's PeerRIB + ribOut +
   peerMeta slot, release, then process. `peerRIB.Insert` /
   `peerRIB.Remove` run under the PeerRIB's own lock. Best-path work
   runs with no outer lock held (the helpers take peerMu.RLock
   internally).
4. **Audit every other `r.mu.Lock()` site** in the rib plugin. Each
   site either: (a) still needs the peerMu.Lock -- keep it; (b) was
   holding it as a shield for `bestPrev` -- can now rely on shard
   locks; (c) was holding it for the interner -- can drop (interner
   is self-locking).
5. **Parallel-UPDATE stress test.** Add a `test/plugin/bgp-rib-
   parallel-updates.ci` functional test or a go-level
   `TestParallelUpdateNoLostBest` that fires UPDATEs from N peer
   goroutines and asserts the best-path delivery count matches the
   insert count with no drops. Run `make ze-race-reactor` for race
   coverage on the new peerMu path.

## Risks

- **Lock ordering.** New rule: `peerMu` is outer, `shard.mu` is
  inner, interner per-table locks are independent (nobody holds
  peerMu while going to interner except via shard dispatch which does
  not touch peerMu). Document and enforce via review.
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
  staleness window as today, just shifted a hair earlier).
- **`SetPeerDown`** deletes `ribInPool[peerAddr]` under peerMu.Lock.
  If that races with a `gatherCandidates` that already snapshotted
  the peer pointer, the candidate record persists for this one
  best-path computation. Next computation will not see the peer.
  Same eventually-consistent semantics as today.

## Out of scope

- Sharding `ribInPool` itself by peer-hash. Peer count is O(100s) at
  most; a single RWMutex around the map is fine.
- Per-peer goroutines for best-path computation. Each peer goroutine
  already drives its own UPDATE processing.
- `SetLocRIB` / the forward-handle observer lifecycle. Touches locRIB
  only, not peerMu.
