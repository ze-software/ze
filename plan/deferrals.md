# Deferrals

Tracked deferrals from implementation sessions. Every decision to not perform in-scope work
must be recorded here with a destination (receiving spec or explicit cancellation).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-03-23 | spec-forward-congestion Phase 2 | `updatePeriodicMetrics()` unit test with mock registry verifying gauge values are set | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | reactor_metrics_test.go:TestUpdatePeriodicMetrics_SetsOverflowGauges | done |
| 2026-03-23 | spec-forward-congestion Phase 2 | ForwardUpdate -> RecordForwarded/RecordOverflowed end-to-end wiring test | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | reactor_metrics_test.go:TestForwardDispatch_RecordForwarded_UpdatesMetrics | done |
| 2026-03-23 | spec-forward-congestion Phase 2 | .ci functional test for metrics endpoint showing ze_bgp_overflow_* metrics | Pre-existing peer.go/types.go build break blocks functional test build | test/plugin/forward-congestion-overflow-metrics.ci | done |
| 2026-03-23 | spec-forward-congestion Phase 2 | Test verifying exact Prometheus metric names match registration | Pre-existing peer.go/types.go build break blocks reactor-level test compilation | reactor_metrics_test.go:TestMetricNames_MatchRegistration | done |
| 2026-03-23 | spec-forward-congestion Phase 3 | Wire ReadThrottle.ThrottleSleep into session read loop | Superseded: spec replaced sleep-between-reads with buffer denial (2026-03-23) | cancelled | cancelled |
| 2026-03-23 | spec-forward-congestion Phase 3 | Register ze.fwd.throttle.enabled env var in reactor.go | Superseded: ReadThrottle removed, buffer denial is the backpressure mechanism | cancelled | cancelled |
| 2026-03-24 | CLI autocomplete session | Sanitize non-printable characters (ANSI escapes) in config values before terminal rendering | Low severity: attacker must be authenticated user editing config; lipgloss passes through raw escape codes | ad-hoc | open |
| 2026-03-24 | CLI autocomplete session | Clear searchCache on commit/discard/rollback to prevent stale search results | Info severity: cache rebuilds on next Dirty() change, staleness window is between commit and next edit | ad-hoc | open |
| 2026-03-25 | spec-role-otc (learned/401) | OTC egress stamping: add OTC = local ASN when route without OTC sent to Customer/Peer/RS-Client | EgressFilterFunc returns bool only, cannot return modified payload; needs applyMods framework | spec-apply-mods | done |
| 2026-03-25 | spec-role-otc (learned/401) | Unicast-only scope enforcement: `isUnicastFamily` defined but not called from OTC filters | Needs family extraction from payload (MP_REACH_NLRI parsing) | spec-apply-mods | done |
| 2026-03-25 | spec-role-otc (learned/401) | `resolveExport` hot-path allocation: allocates per UPDATE per peer, should pre-compute at config time | Performance optimization, not correctness | spec-apply-mods | done |
| 2026-03-26 | spec-peer-local-remote | AC-5: Validate duplicate `remote > ip` across peers in same list | goyang does not support YANG `unique` constraint; needs custom post-parse validation | ad-hoc | open |
| 2026-03-26 | make ze-verify | Editor `.et` test race conditions: 8 tests fail with `race detected` (lifecycle, session, pipe categories) | Pre-existing race in TUI framework, not related to config changes | ad-hoc | open |
| 2026-03-26 | make ze-verify | Perf benchmark failures: TestBenchmarkEndToEnd, TestRunSmallBenchmark, TestBenchmarkIterDelayZero (bind port 179 permission denied, dial timeout) | Pre-existing infrastructure issue, requires privileged port or test environment changes | ad-hoc | open |
| 2026-03-26 | spec-monitor-1-event-stream review | FormatMonitorLine in `bgp/plugins/cmd/monitor/format.go` is dead code: defined and tested but never called from production code. Decide whether to wire it into a visual/text output mode or remove it | Discovered during spec review against current code; not blocking monitor-1 implementation | spec-monitor-1-event-stream | open |
