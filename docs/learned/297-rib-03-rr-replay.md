# 297 — RIB-03: Route Reflector Replay Integration

## Objective

Fix the data loss bug where peers connecting late miss routes forwarded before they reached Established, by replacing ROUTE-REFRESH with RIB replay via dispatch-command and deleting bgp-rr's local RIB.

## Decisions

- Replay-first, then add to forward targets, then delta replay — ghost route problem: if peer is in forward targets during replay, a withdrawal followed by replay re-announces a deleted route; replay MUST complete before forwarding starts
- Delta replay covers the gap: routes arriving during full replay get higher sequence indices, caught up with a second small replay from `last-index`
- Withdrawal map (family+prefix per source peer) replaces full local RIB — bgp-rr only needs prefix/family for withdrawal commands on peer-down; full route state lives in bgp-adj-rib-in
- Per-peer lifecycle goroutine for replay — event loop stays free; goroutine is per-peer lifecycle (allowed by goroutine-lifecycle rule), not per-event
- `dispatchCommandHook` test seam — allows unit tests to mock DispatchCommand response without real plugin infrastructure
- ROUTE-REFRESH thundering herd: N peers × M families = N×M simultaneous re-advertisements; unusable for production route servers

## Patterns

- `Replaying` field on `PeerState` — excluded from `selectForwardTargets` until replay response received
- Peer-down race resolved by design: bgp-rr reads its own withdrawal map (populated from processForward), independent of bgp-adj-rib-in's cleanup timing
- Functional test deferred again (rib-03): requires full engine+plugin integration harness not yet available

## Gotchas

- None recorded.

## Files

- `internal/component/bgp/plugins/rs/server.go` — handleStateUp (replay+delta via DispatchCommand in lifecycle goroutine), replayForPeer, withdrawal map, handleStateDown (reads withdrawal map)
- `internal/component/bgp/plugins/rs/peer.go` — Replaying field added
- `internal/component/bgp/plugins/rs/rib.go` — deleted (local RIB replaced by withdrawal map + bgp-adj-rib-in)
- `internal/component/bgp/plugins/rs/rib_test.go` — deleted (tests for deleted RIB)
