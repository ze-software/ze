# 647 -- BMP Sender Compliance (RFC 7854 S4.10, S4.7)

## Context

The BMP sender (spec-bgp-4-bmp, #574) shipped with three known compliance gaps:
synthetic 29-byte OPENs in Peer Up (no capabilities), no Route Mirroring sender,
and broken ribout dedup (removed in #574). This spec closed three of four gaps
(Phase 4 Loc-RIB deferred pending UPDATE wire encoder).

## Decisions

- **Event-based OPEN caching**, over reconstructing from parsed Open struct.
  Subscribe to "open direction received/sent", cache raw OPEN body bytes per
  peer, correlate with subsequent SessionStateUp. Simpler, uses actual wire
  bytes, plugin-contained.
- **Cache and cleanup before senders check.** OPEN caching and peer-down
  cleanup (openCache + dedupState) run before checking if any collector is
  connected. Peers establish before collectors connect; without this, OPEN
  PDUs are lost (AC-3).
- **Skip Peer Up on OPEN cache miss**, over falling back to synthetic OPENs.
  The reactor always delivers OPEN events before state events, so a cache
  miss indicates a bug. Logging a warning and skipping is safer than sending
  wrong data.
- **Subscribe all message types unconditionally.** Subscriptions are set at
  startup before config loads. Notification/keepalive/refresh subscriptions
  cost one type-check per event when mirroring is disabled.
- **Whole-UPDATE-body FNV-64a hash for dedup**, over per-NLRI tracking. Hashing
  the complete RawBytes is simpler and avoids UPDATE parsing. Different
  attributes produce different hashes (AC-8). Pre-allocated hasher with
  Reset() avoids per-event allocation.
- **Dedup cap disables tracking, not forwarding.** When the per-peer hash set
  reaches 100k entries, new hashes are not tracked but UPDATEs are still
  forwarded. The first implementation silently dropped UPDATEs at the cap --
  caught in review.
- **Route Mirroring accepts nil body** (KEEPALIVE). handleSenderMirror checks
  RawMessage != nil but does not require RawBytes != nil. writeRouteMirroring
  synthesizes the 19-byte BGP header with no body.

## Consequences

- Peer Up now carries real OPEN PDUs with capabilities -- collectors can
  analyze negotiated features
- Route Mirroring streams verbatim copies of all BGP messages (UPDATE, OPEN,
  NOTIFICATION, KEEPALIVE, ROUTE-REFRESH) when enabled
- Ribout dedup suppresses redundant Route Monitoring with bounded memory
- BuildSyntheticOpen removed from production code
- RFC 7854 summary corrected: Route Mirroring has Per-Peer Header (was "No")
- Follow-up: Loc-RIB Route Monitoring (RFC 9069) requires wire UPDATE
  reconstruction from BestChangeBatch structured data

## Gotchas

- **handleStructuredEvent early return blocks housekeeping.** The original
  `if len(senders) == 0 { return }` prevented OPEN caching when no collector
  was connected. State cleanup (openCache, dedupState) has the same issue.
  Solution: move housekeeping before the senders check.
- **net.Pipe is synchronous.** Tests using net.Pipe that write multiple BMP
  messages from handleStructuredEvent (Route Monitoring + Route Mirroring)
  deadlock because the second write blocks until someone reads the first.
  Solution: run the handler in a goroutine and read sequentially.
- **KEEPALIVE has nil RawBytes.** rawUpdateBytes returns nil for nil body,
  which handleSenderMirror originally checked to bail out. KEEPALIVE is a
  valid 19-byte BGP message with no body. handleSenderMirror must check
  RawMessage != nil, not RawBytes != nil.
- **Dedup test passed by accident.** Without dedup logic, the duplicate
  UPDATE wrote to net.Pipe, hit the 10-second writeTimeout (no reader),
  returned an error, and the test continued. The test took 10s but "passed"
  because it only checked that a different UPDATE arrived afterward.
- **fnv.New64a() allocates per call.** Returns *sum64a that escapes to heap.
  Pre-allocate on the struct and Reset() per use. Safe because event handling
  is serial.

## Files

- `internal/component/bgp/plugins/bmp/bmp.go` -- openPair, openCache, dedupState, cacheOpenPDU, handleSenderMirror, dedup in handleSenderUpdate
- `internal/component/bgp/plugins/bmp/sender.go` -- writeRouteMirroring
- `internal/component/bgp/plugins/bmp/msg.go` -- BuildSyntheticOpen removed
- `internal/component/bgp/plugins/bmp/event_test.go` -- 8 new tests
- `internal/component/bgp/plugins/bmp/schema/ze-bmp-conf.yang` -- route-mirroring leaf
- `rfc/short/rfc7854.md` -- Route Mirroring Per-Peer Header correction
- `rfc/short/rfc8671.md` -- new RFC summary (Adj-RIB-Out)
- `rfc/short/rfc9069.md` -- new RFC summary (Loc-RIB)
- `test/plugin/bmp-sender-peer-up-open.ci` -- functional test
- `test/plugin/bmp-sender-route-mirroring.ci` -- functional test
