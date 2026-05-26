# 793 -- show-bgp-peer-detail

## Context

Ze's `show bgp peer <x>` output had ~20 fields while production NOS implementations (JunOS, IOS-XR, EOS, FRR) show 50-60. Operators switching from other platforms expected full message counters, negotiated timers, capabilities, policy names, transport details, and session stability metrics inline in the peer detail view. The gap made Ze look incomplete for operational troubleshooting.

## Decisions

- Chose session callbacks (matching existing `onNotifSent`/`onNotifRecv` pattern) over direct peer references in Session, because Session deliberately has no Peer backpointer.
- Chose atomic counters over mutex-protected fields, consistent with existing `peerCounters` pattern. Lock-free on the hot path.
- Kept existing flat fields (`updates-received`, etc.) alongside new nested `messages` block for backward compatibility, over a breaking change to restructure only.
- Split counters into per-session (reset by ClearStats) and lifetime (connections, flaps, last notification survive across sessions), matching operator expectations from other NOS.
- Read TCP ports directly from `net.Conn` in `TCPPorts()` over storing in atomics, avoiding stale port data after connection changes.
- Computed `effectiveKeepalive` to report the actual timer value (configured or clamped), not always hold/3.

## Consequences

- `show bgp peer` is now at NOS parity for observability. Operators get messages, capabilities, timers, policy, stability metrics in one command.
- The `messages` block structure sets a convention for future per-type counters.
- Prefix counters (received/accepted/active/sent) remain the major gap, requiring a RIB plugin query bridge (separate effort).
- `PeerInfo` struct grew by ~25 fields. Any new consumer of `Peers()` gets the enriched snapshot automatically.

## Gotchas

- `processOpen()` (collision resolution path) and `handleOpen()` (normal path) both need the `onOpenRecv` callback. Missing one undercounts opens.
- `writeUpdate()` is a third write path alongside `writeMessage()` and `writeRawUpdateBody()`. All three need `onWrite` for accurate last-write timestamps.
- `negotiatedKeepalive` is not simply `hold/3` when a valid configured keepalive exists. The effective value depends on whether clamping was needed.
- `sessionHealth.flapTimes` is a sliding window (5-min, bounded). A separate `flapLifetime` counter was needed for the API's lifetime count.

## Files

- `internal/component/bgp/reactor/peer_stats.go` - new counters, Incr methods
- `internal/component/bgp/reactor/peer.go` - negotiated timer atomics, TCPPorts(), connection-dropped tracking
- `internal/component/bgp/reactor/reactor_api.go` - PeerInfo population with all new fields
- `internal/component/bgp/plugins/cmd/peer/peer.go` - enriched HandleBgpPeerDetail output
- `internal/component/plugin/types_bgp.go` - PeerInfo struct extensions
- `internal/component/bgp/reactor/session*.go` - callbacks for open/refresh/read/write/negotiated
- `internal/component/bgp/reactor/negotiated.go` - HoldTime and GR fields
- `internal/component/bgp/reactor/session_health.go` - lifetime flap counter
