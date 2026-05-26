# 789 -- Adj-RIB-Out Compact Storage (Phase 2)

## Context

The plugin ribOut stored a full `*bgp.Route` (385 B) per peer per route. At 1M routes / 10 peers, this consumed 3.7 GB. The engine OutgoingRIB (478 B/route) was identified as a target but turned out to have zero production callers (test-only code). Phase 2 replaced the plugin ribOut storage with a 16 B `ribOutEntry` + shared pool handle for wire attribute bytes.

## Decisions

- Dropped Phase 1 (engine OutgoingRIB): zero production callers. `NewOutgoingRIB()` only appears in test files. The reactor uses `rib.Route` through `Transaction` and `Peer.QueueAnnounce`, not through `OutgoingRIB`.
- Chose 16 B entry (`MsgID` + `AttrHandle` + `StaleLevel`) over 4 B (handle-only): MsgID is needed for replay ordering, StaleLevel for GR/LLGR. NextHop parsed from wire bytes on demand since reconstruction is infrequent.
- Separated `ribOutSource` from per-peer entries: SourcePeer is per-route (not per-peer), so a shared map with refcounting avoids N copies. Refcount tracks destination peers; cleaned up at zero.
- Created `pool.RibOut` (idx 16) as a new `attrpool.Pool` instance rather than reusing per-attribute pools, because ribOut stores the full wire blob (all attributes concatenated), not split per type.
- `packEventAttrs` (JSON fallback) only encodes IPv4 NEXT_HOP. IPv6 next-hops live in MP_REACH_NLRI which the fallback does not construct. Accepted: the structured path (production) always has wire bytes.

## Consequences

- At 1M routes / 10 peers: projected ~230 MB vs 3.7 GB (94% reduction).
- Reconstruction cost (parsing wire bytes back to Route fields) is paid only on cold paths: replay, show, refresh, resend. Hot path (store, count) operates on 16 B value types.
- `FormatAnnounceCommand` still uses per-field text format (never uses RawAttrs despite the docstring claiming it does). Reconstruction must populate all fields.
- pool.RibOut is included in AllPools() for compaction scheduling but not in poolNames() for Prometheus metrics. Metrics can be added later if needed.

## Gotchas

- `handleSent` (JSON path) must call `pool.RibOut.Intern` BEFORE `peerMu.Lock` to match the lock ordering in `handleSentStructured`. Pool.mu and peerMu are non-nested (pool operations acquire and release atomically), so there is no actual deadlock risk, but consistent ordering prevents future regressions if the pool API changes.
- `release()` is a value receiver on a map value. Go copies the struct to call the method. Since `AttrHandle` is `uint32` (value type), the copy has the same handle value and `pool.RibOut.Release` works correctly. Do not add pointer fields to `ribOutEntry` without reconsidering this.
- `parseOutRouteKey` must find the last `:` after the last `/` to distinguish IPv6 prefix colons from the pathID separator. IPv6 prefix "2001:db8::/32:42" has the pathID separator at position after "/32".
- `setRibOutSource` uses an `isNew bool` parameter to avoid double-counting refCount on re-announcements. The caller checks `_, existed := ribOut[peer][fam][key]` before storing and passes `!existed`.

## Files

- `internal/component/bgp/plugins/rib/pool/ribout.go` -- pool.RibOut (idx 16)
- `internal/component/bgp/plugins/rib/ribout_entry.go` -- entry type, reconstruction, source tracking, wire parsers
- `internal/component/bgp/plugins/rib/rib.go` -- handleSent updated
- `internal/component/bgp/plugins/rib/rib_structured.go` -- handleSentStructured/storeSentEntries updated
- `internal/component/bgp/plugins/rib/rib_commands.go` -- resend/stale updated
- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- show pipeline updated
