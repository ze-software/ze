# 173 — Plugin RIB Pool Storage

## Objective

Add raw wire bytes (`raw-attributes`, `raw-nlri`) to engine JSON events, then migrate plugin RIB to pool-based storage for memory-efficient deduplication of received routes.

## Decisions

- `DirectNLRISet` for IPv4 unicast: 1–5 bytes per prefix < 4-byte pool handle overhead — direct storage is cheaper at this scale.
- `PooledNLRISet` for IPv6+, VPN, EVPN: wire bytes large enough that pool dedup handles save memory; uses local `map[pool.Handle]int` index for O(1) lookup instead of `pool.Lookup()`.
- Dual storage retained: engine sends `format=full` for pool storage, `format=short` for legacy. Pool storage only activates when `raw-attributes` field is present in the event; absent → fall back to legacy `Route` storage.
- `ribOut` stays with legacy Route storage: pool storage benefits Adj-RIB-In (received); outbound replay is simpler with text command reconstruction.
- `PeerRIB` wrapper added for thread-safe multi-family access — not in original spec but necessary given concurrent event handling.

## Patterns

- `FamilyRIB` maintains single pool ref per unique attr handle — refcount invariant: each unique attribute blob is referenced exactly once per NLRISet, not once per prefix.
- Cross-storage sync: when the same prefix arrives in a different format (pool vs. legacy), the old storage is cleared before inserting into the new one.

## Gotchas

- ADD-PATH hardcoded to false: pool storage assumes no path-id prefix. ADD-PATH peers silently fall back to legacy storage until proper per-peer/family negotiation state is tracked.
- Non-unicast families (EVPN, VPN, FlowSpec) skipped by `splitNLRIs()`: these families continue using legacy storage; pool storage only processes simple `[prefix-len][prefix-bytes]` format.
- `DirectNLRISet.nlriLen()` buffer overflow: computed length must be validated to fit in buffer before writing. Boundary tests added.

## Files

- `internal/plugin/wire_extract.go` — `ExtractRawAttributes`, `ExtractRawNLRI`, `ExtractRawWithdrawn`
- `internal/plugin/text.go` — raw fields added to JSON (format=full)
- `internal/plugin/rib/storage/nlriset.go` — `NLRISet` interface, `DirectNLRISet`, `PooledNLRISet`
- `internal/plugin/rib/storage/familyrib.go` — forward + reverse index
- `internal/plugin/rib/storage/peerrib.go` — thread-safe multi-family wrapper
- `internal/plugin/rib/event.go` — `RawAttributes`, `RawNLRI`, `RawWithdrawn` fields
