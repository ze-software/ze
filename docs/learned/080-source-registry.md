# 080 — Source Registry

## Objective

Create a compact source registry assigning `uint32` IDs to message sources (peers, API processes, config) to replace 16-byte IP address storage and enable unified source tracking.

## Decisions

- Chose `uint32` self-describing ID layout (`0` = config, `1-99999` = peer, `100001+` = API, `MaxUint32` = invalid) over simple sequential allocation: makes `SourceID.String()` self-describing ("peer:42", "api:1") without a registry lookup.
- IDs are never reused: deactivated sources keep their slot to preserve historical source resolution.
- Reverse indexes (`peerIdx map[netip.Addr]SourceID`, `apiIdx map[string]SourceID`) for O(1) lookup in both directions.
- `Get()` returns `Source` by value (not pointer): safe for concurrent access without holding the lock.

## Patterns

- `SplitWireUpdate` preserves `sourceID` on all split chunks: the source of a split chunk is the same as the original.

## Gotchas

- API process registration and JSON/text formatter updates were deferred: the spec was scoped to just the registry infrastructure + peer registration. The `ReceivedUpdate.SourcePeerIP` field was also left for follow-up.

## Files

- `internal/source/source.go` — SourceID, SourceType, Source types
- `internal/source/registry.go` — thread-safe registry
- `internal/component/plugin/wire_update.go` — sourceID field, SetSourceID()
- `internal/reactor/peer.go` — peer registration at creation
