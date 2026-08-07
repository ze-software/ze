---
kind: table
level:
stage:
---
| Step | What |
|------|------|
| Plugin declares | `PeerUpBarrier: true` in its `registry.Registration` |
| Engine counts | The barrier-declaring plugins among those the peer-up event is ACTUALLY delivered to (`countPeerUpBarrier`, `internal/component/bgp/server/events.go`), before the first delivery |
| Engine acknowledges | Each successful delivery result signals the peer's barrier: the result IS the plugin's acknowledgement that its handler ran and returned |
| Peer waits | `Peer.waitPeerUpBarrier` before the initial-sync End-of-RIB (`internal/component/bgp/reactor/peer_initial_sync.go`) |
