| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-30 | spec-rfc-verdict-fix-bgp | rib `handleRefresh` vs reactor `sendRouteRefresh` and `peer.shouldQueue()` | SUSPECTED: BoRR/EoRR go out raw at once while the refresh's routes queue in `opQueue` during initial sync, so the EoRR can precede them (RFC 7313 §4 purge) | not fixed; next: whole-Peer test, refresh during initial sync, read wire order |
