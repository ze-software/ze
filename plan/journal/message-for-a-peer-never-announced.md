# A message is sent about a peer its receiver was never told exists

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-13 | spec-bgp-live-receive-binding-and-bfd-keep-session | bmp `handleSenderUpdate` (bmp_events.go) | A bmp binding without `open` never caches OPENs, so `recordPeerUp` records nothing and no Peer Up is sent, yet updates still stream as Route Monitoring to every collector: the collector gets routes for a peer it was never told is up (RFC 7854 Section 3.3) | Open |
