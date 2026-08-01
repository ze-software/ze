# Learned: RIB Multi-Source Support (spec-rib-4-extraction)

## What
Changed `ribInPool` from `map[string]*PeerRIB` to `map[ProtocolID]map[string]*PeerRIB`.
Added `bgpPeers` as a cached reference to the BGP inner map for zero-cost hot-path access.

## Key decisions
- Two-level map with uint16 outer key beats string-prefix keys (no alloc per lookup) and composite struct keys (no struct hashing).
- `bgpPeers` cached at construction, not computed on access. The pointer never changes after `NewRIBManager`.
- `gatherCandidatesLocked` iterates only `bgpPeers`. Show commands and metrics iterate all protocol slots.
- No `protocolPeers(pid)` helper added. Direct `for _, protoPeers := range r.ribInPool` is simpler and sufficient until a third consumer appears.

## What went well
- Bulk `replace_all` of `r.ribInPool[` to `r.bgpPeers[` in test files was efficient for 60+ sites.
- Existing test suite caught zero regressions, confirming the refactor preserved all behavior.

## Mistakes
- Forgot `FamilyOps` field when constructing test `Event` structs for `handleReceived`. The function returns early on `len(FamilyOps) == 0`. Existing tests set it; new tests initially did not.
- Tried `require.Same` on Go maps. Maps are reference types but `require.Same` needs `reflect.Pointer` kind. Used value equality instead.

## Future work
- `newInboundSource` does not dedup peer addresses across protocol slots. When BMP lands, a peer address in multiple protocols would appear twice in show output. Needs a `seen` map.

## Files

None recorded.
