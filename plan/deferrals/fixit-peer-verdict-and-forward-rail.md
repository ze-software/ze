# Deferrals: fixit-peer-verdict-and-forward-rail

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-22 | spec-fixit-peer-verdict-and-forward-rail | The BGP forwarding rail consults `Peer.ShouldQueue()` nowhere (`peer.go:899-906` is called only from `reactor_api_batch.go:106`, `:235` and `reactor_api_forward.go:58`, all on the route-injection rail), so a forwarded withdraw can overtake an announce already queued for the same prefix and leave the peer holding a stale route | Distinct defect with a distinct failure mode from the End-of-RIB ordering this spec corrected, and the fix needs its own design: `opQueue` holds structured `PeerOp` values (`peer.go:111-118`) not wire bodies, blocking the fast path would stall the source peer's whole read loop, and widening `HoldWrites` would delay KEEPALIVE. Requires `make ze-race-reactor` | `plan/spec-fixit-forward-rail-initial-sync-ordering.md` | deferred |
