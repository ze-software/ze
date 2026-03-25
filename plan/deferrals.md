# Deferrals

Tracked deferrals from implementation sessions. Every decision to not perform in-scope work
must be recorded here with a destination (receiving spec or explicit cancellation).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-03-23 | spec-forward-congestion Phase 2 | `updatePeriodicMetrics()` unit test with mock registry verifying gauge values are set | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | spec-forward-congestion Phase 3 | open |
| 2026-03-23 | spec-forward-congestion Phase 2 | ForwardUpdate -> RecordForwarded/RecordOverflowed end-to-end wiring test | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | spec-forward-congestion Phase 3 | open |
| 2026-03-23 | spec-forward-congestion Phase 2 | .ci functional test for metrics endpoint showing ze_bgp_overflow_* metrics | Pre-existing peer.go/types.go build break blocks functional test build | spec-forward-congestion Phase 3 | open |
| 2026-03-23 | spec-forward-congestion Phase 2 | Test verifying exact Prometheus metric names match registration | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | spec-forward-congestion Phase 3 | open |
| 2026-03-23 | spec-forward-congestion Phase 3 | Wire ReadThrottle.ThrottleSleep into session read loop | Superseded: spec replaced sleep-between-reads with buffer denial (2026-03-23) | cancelled | cancelled |
| 2026-03-23 | spec-forward-congestion Phase 3 | Register ze.fwd.throttle.enabled env var in reactor.go | Superseded: ReadThrottle removed, buffer denial is the backpressure mechanism | cancelled | cancelled |
| 2026-03-24 | CLI autocomplete session | Sanitize non-printable characters (ANSI escapes) in config values before terminal rendering | Low severity: attacker must be authenticated user editing config; lipgloss passes through raw escape codes | ad-hoc | open |
| 2026-03-24 | CLI autocomplete session | Clear searchCache on commit/discard/rollback to prevent stale search results | Info severity: cache rebuilds on next Dirty() change, staleness window is between commit and next edit | ad-hoc | open |
| 2026-03-25 | spec-role-otc (learned/401) | OTC egress stamping: add OTC = local ASN when route without OTC sent to Customer/Peer/RS-Client | EgressFilterFunc returns bool only, cannot return modified payload; needs applyMods framework | spec-apply-mods | done |
| 2026-03-25 | spec-role-otc (learned/401) | Unicast-only scope enforcement: `isUnicastFamily` defined but not called from OTC filters | Needs family extraction from payload (MP_REACH_NLRI parsing) | spec-apply-mods | done |
| 2026-03-25 | spec-role-otc (learned/401) | `resolveExport` hot-path allocation: allocates per UPDATE per peer, should pre-compute at config time | Performance optimization, not correctness | ad-hoc | open |
