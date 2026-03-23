# Deferrals

Tracked deferrals from implementation sessions. Every decision to not perform in-scope work
must be recorded here with a destination (receiving spec or explicit cancellation).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-03-23 | spec-forward-congestion Phase 2 | `updatePeriodicMetrics()` unit test with mock registry verifying gauge values are set | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | spec-forward-congestion Phase 3 | open |
| 2026-03-23 | spec-forward-congestion Phase 2 | ForwardUpdate -> RecordForwarded/RecordOverflowed end-to-end wiring test | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | spec-forward-congestion Phase 3 | open |
| 2026-03-23 | spec-forward-congestion Phase 2 | .ci functional test for metrics endpoint showing ze_bgp_overflow_* metrics | Pre-existing peer.go/types.go build break blocks functional test build | spec-forward-congestion Phase 3 | open |
| 2026-03-23 | spec-forward-congestion Phase 2 | Test verifying exact Prometheus metric names match registration | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | spec-forward-congestion Phase 3 | open |
| 2026-03-23 | spec-forward-congestion Phase 3 | Wire ReadThrottle.ThrottleSleep into session read loop | Pre-existing peer.go/types.go build break blocks session.go compilation | spec-forward-congestion Phase 4 | open |
| 2026-03-23 | spec-forward-congestion Phase 3 | Register ze.fwd.throttle.enabled env var in reactor.go | Pre-existing peer.go/types.go build break blocks reactor.go compilation | spec-forward-congestion Phase 4 | open |
