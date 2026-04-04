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
| 2026-03-24 | CLI autocomplete session | Sanitize non-printable characters (ANSI escapes) in config values before terminal rendering | Low severity: attacker must be authenticated user editing config; lipgloss passes through raw escape codes | model_render.go:sanitizeForDisplay | done |
| 2026-03-24 | CLI autocomplete session | Clear searchCache on commit/discard/rollback to prevent stale search results | Info severity: cache rebuilds on next Dirty() change, staleness window is between commit and next edit | ad-hoc (fixed inline) | done |
| 2026-03-25 | spec-role-otc (learned/401) | OTC egress stamping: add OTC = local ASN when route without OTC sent to Customer/Peer/RS-Client | EgressFilterFunc returns bool only, cannot return modified payload; needs applyMods framework | spec-apply-mods | done |
| 2026-03-25 | spec-role-otc (learned/401) | Unicast-only scope enforcement: `isUnicastFamily` defined but not called from OTC filters | Needs family extraction from payload (MP_REACH_NLRI parsing) | spec-apply-mods | done |
| 2026-03-25 | spec-role-otc (learned/401) | `resolveExport` hot-path allocation: allocates per UPDATE per peer, should pre-compute at config time | Performance optimization, not correctness | spec-apply-mods | done |
| 2026-03-26 | spec-peer-local-remote | AC-5: Validate duplicate `remote > ip` across peers in same list | goyang does not support YANG `unique` constraint; needs custom post-parse validation | resolve.go:checkDuplicateRemoteIPs + cli/validator.go:checkDuplicateRemoteIPs | done |
| 2026-03-26 | make ze-verify | Editor `.et` test race conditions: 8 tests fail with `race detected` (lifecycle, session, pipe categories) | Pre-existing race in TUI framework, not related to config changes | Fixed: headless.go SettleWait snapshot isolation + 15s deadline (was 3s), settle snapshot isolation, stale comment fix in runner.go | done |
| 2026-03-26 | make ze-verify | Perf benchmark failures: TestBenchmarkEndToEnd, TestRunSmallBenchmark, TestBenchmarkIterDelayZero (bind port 179 permission denied, dial timeout) | Pre-existing infrastructure issue, requires privileged port or test environment changes | Fixed: PassiveListen flag gates port 179 listen, removed t.Parallel from heavy benchmark tests | done |
| 2026-03-26 | spec-monitor-1-event-stream review | FormatMonitorLine in `bgp/plugins/cmd/monitor/format.go` is dead code: defined and tested but never called from production code. Decide whether to wire it into a visual/text output mode or remove it | Discovered during spec review against current code; not blocking monitor-1 implementation | Wired into CLI streaming (ProcessPipesDefaultFunc) and TUI monitor (FormatFunc) via registered formatter | done |
| 2026-03-27 | spec-route-loop-detection | `allow-own-as N` configuration to skip AS loop detection for specific peers | Requires YANG schema for the config leaf and per-peer filter injection/suppression logic | spec-named-default-filters | open |
| 2026-03-27 | spec-route-loop-detection | Explicit `cluster-id` configuration (override Router ID as Cluster ID) | Requires YANG schema for the config leaf and adding ClusterID to PeerFilterInfo | spec-named-default-filters | open |
| 2026-03-27 | deep-review | Metrics behavioral tests: spy registry tests for RIB route churn counters, config reload counters, wire bytes counters | updatePeerStateMetric and IncrNotification* tests done (peer_stats_test.go); RIB, config reload, and wire counters still untested | spec-prometheus-plugin-health | open |
| 2026-03-27 | deep-review | notifSent/notifRecv per-peer Prometheus label cleanup on peer removal | Notification code/subcode combinations create per-peer labels that are never deleted in RemovePeer; needs per-peer tracking of observed (code, subcode) pairs | Fixed: reactor_peers.go enumerates notifCodeNames (1-7) x notifSubcodeNames (0-14) for cleanup | done |
| 2026-03-27 | deep-review | GR plugin per-peer metric label cleanup on peer removal (staleRoutes, timerExpired gauges) | GR plugin has no peer-removal callback; reactor cleans its own metrics but plugin metrics persist | spec-prometheus-plugin-health | open |
| 2026-03-27 | spec-prometheus-deep | Plugin health metrics (ze_plugin_status, ze_plugin_restarts_total, ze_plugin_events_delivered_total) | Requires changes to internal/plugin/ infrastructure, not just reactor wiring | spec-prometheus-plugin-health | done |
| 2026-03-27 | spec-prometheus-deep | `ze_rib_eor_received_total` per peer+family counter at RIB level | EOR already tracked at reactor level as type=eor on ze_peer_messages_received_total; per-family counter would require RIB to subscribe to EOR events | user-approved-drop | cancelled |
| 2026-03-28 | spec-redistribution-filter AC-18 | Default named filter `rfc:no-self-as` active by default for all peers | Current AS loop check is an in-process IngressFilterFunc (mandatory), not a named policy filter. Converting it to a named default requires refactoring the loop filter from IngressFilterFunc to the policy filter chain. Override infrastructure (applyOverrides, DefaultImportFilters) is built and tested. | spec-named-default-filters | open |
| 2026-03-28 | spec-redistribution-filter AC-19/20 | Override mechanism for default filters (per-peer and per-group) | applyOverrides() built and tested in redistribution.go but no default filters to override yet | spec-named-default-filters | open |
| 2026-03-28 | spec-redistribution-filter AC-21 | Mandatory filter cannot be overridden (override targeting mandatory is ignored) | Requires mandatory filter registry distinguishing mandatory from default | spec-named-default-filters | open |
| 2026-03-28 | spec-redistribution-filter AC-10 | Export filter modify dedicated functional test (.ci) | Export modify code wired in reactor_api_forward.go:288 but no .ci test exercises modify (only reject tested) | spec-redistribution-filter-phase2 | open |
| 2026-03-28 | spec-redistribution-filter AC-13 | Reject modify of undeclared attribute | Requires per-filter attribute registry to cross-check modify response against declared set | spec-redistribution-filter-phase2 | open |
| 2026-03-28 | spec-redistribution-filter AC-15 | raw=true delivers hex wire bytes in filter-update | FilterRegistration.Raw field stored but filter-update RPC does not include raw bytes | spec-redistribution-filter-phase2 | open |
| 2026-03-29 | spec-redistribution-filter | Wire-level dirty tracking: re-encode only modified attributes into wire UPDATE bytes | Text-level delta merge works (applyFilterDelta). Wire-level re-encoding requires integrating text delta back into wire bytes via attribute Builder + ModAccumulator. Current path: text delta applied, but wire bytes not rebuilt from modified text. | spec-redistribution-filter-phase2 | open |
| 2026-03-29 | spec-redistribution-filter | TestAttributeAccumulation, TestDirtyTracking, TestFilterModifyOnlyDeclared unit tests | Require reactor-level attribute parsing and wire re-encoding infrastructure | spec-redistribution-filter-phase2 | open |
| 2026-03-29 | spec-redistribution-filter | redistribution-override.ci functional test | Override resolution tested in unit tests but no .ci test; requires default filter registry | spec-named-default-filters | open |
| | | | | | |
| **2026-04-04 audit of deleted specs -- significant** | | | | | |
| 2026-04-04 | spec-rib-inject | Functional tests api-rib-inject.ci, api-rib-withdraw.ci | Spec deleted with TODO items unfinished; no .ci test proves feature reachable | spec-rib-inject-ci (to be created) | open |
| 2026-04-04 | spec-rib-inject | docs/features.md + docs/architecture/api/commands.md updates for RIB inject | Listed as TODO in spec, never done | spec-rib-inject-ci (to be created) | open |
| 2026-04-04 | spec-rib-inject | RFC 5549 extended next-hop for injected routes (PackContext.ExtendedNextHop check) | Noted as future work | spec-rib-inject-rfc5549 (to be created) | open |
| 2026-04-04 | spec-apply-mods | v2 ACs (AC-11 through AC-18): progressive build pipeline, AttrOp struct, pooled buffer | Core purpose of spec; status was "v2" not "done" when deleted | spec-apply-mods-v2 (to be created) | open |
| 2026-04-04 | spec-apply-mods | NLRI structural modification via ModAccumulator | Listed as future separate field | spec-apply-mods-v2 (to be created) | open |
| 2026-04-04 | spec-exabgp-bridge-muxconn | Bridge internal plugin registration for .ci tests and production | Bridge uses stdin/stdout only; cannot run as external plugin through process manager | spec-exabgp-bridge-internal (to be created) | open |
| 2026-04-04 | spec-exabgp-bridge-muxconn | SetWriteDeadline degradation on non-TCP transports (os.File, SSH) | Write deadline silently skipped; writes may block indefinitely | spec-exabgp-bridge-internal (to be created) | open |
| 2026-04-04 | spec-fleet-config | Config rollback mechanism | Deferred to "config-archive spec" which now exists as spec-config-archive-v2 | spec-config-archive-v2 | open |
| 2026-04-04 | spec-monitor-2-bgp-dashboard | All 17 ACs: entire BGP dashboard spec | Deleted as "complete" but nothing was implemented; audit was empty | spec-bgp-dashboard (to be created) | open |
| 2026-04-04 | spec-command-inventory | Entire spec: make ze-command-list, StreamingPrefixes accessor | Skeleton status when deleted | spec-command-inventory (to be created) | open |
| 2026-04-04 | spec-llgr-4-readvertisement | Route metadata infrastructure (meta map, ModAccumulator) | Dependency spec-route-metadata never created | spec-route-metadata (to be created) | open |
| 2026-04-04 | spec-llgr-4-readvertisement | Per-family ribOut infrastructure | Dependency spec-rib-family-ribout never created | spec-rib-family-ribout (to be created) | open |
| 2026-04-04 | spec-llgr-4-readvertisement | Multi-peer partial deployment .ci test for LLGR readvertisement | Requires multi-peer .ci infrastructure | spec-llgr-readvertisement (to be created) | open |
| 2026-04-04 | spec-filter-community | bgp-filter-prefix plugin | Planned as separate spec, never written | spec-filter-prefix (to be created) | open |
| 2026-04-04 | spec-filter-community | bgp-filter-irr plugin (library exists at internal/component/bgp/irr/) | Planned as separate spec, never written | spec-filter-irr (to be created) | open |
| | | | | | |
| **2026-04-04 audit of deleted specs -- medium** | | | | | |
| 2026-04-04 | spec-iface-0-umbrella | macOS/BSD interface plugins (_darwin.go, _bsd.go) | Platform-specific; Linux-only for now | spec-iface-darwin-bsd (to be created) | open |
| 2026-04-04 | spec-iface-0-umbrella | Phase 4: DHCP client, make-before-break, traffic mirroring, SLAAC | Later phase of iface umbrella | spec-iface-phase4 (to be created) | open |
| 2026-04-04 | spec-iface-5-vm-tests | Packet-level mirror verification, DHCPv6 PD test, SLAAC test | Advanced VM-based testing | spec-iface-phase4 (to be created) | open |
| 2026-04-04 | spec-looking-glass | Alice-LG e2e integration test, large RIB perf test, TLS, pagination | Advanced LG features | spec-looking-glass-v2 (to be created) | open |
| 2026-04-04 | spec-web-0-umbrella | RBAC, i18n, mobile layout, config upload/download, plugin web extensions | Later phases of web umbrella | spec-web-phase2 (to be created) | open |
| 2026-04-04 | spec-shell-completion-v2 | Flag-value completion (--family \<TAB\>), config section completion | Advanced completion features | spec-shell-completion-v3 (to be created) | open |
| 2026-04-04 | spec-port-defaults | Range-vs-single port conflict detection, YANG-default lint check | Edge case detection and tooling | spec-port-defaults-v2 (to be created) | open |
| 2026-04-04 | spec-multipeer-ci | LLGR egress suppress test, multi-peer route reflection test | Requires multi-peer .ci infrastructure | spec-multipeer-ci-v2 (to be created) | open |
| 2026-04-04 | spec-gr-marker | F-bit per-family signaling, Selection Deferral Timer, supervisor crash recovery | Advanced GR features | spec-gr-advanced (to be created) | open |
| 2026-04-04 | spec-prefix-limit | CLI warning count on login, ze bgp warnings command, staleness banner | Visibility features for prefix-limit | spec-prefix-limit-visibility (to be created) | open |
| 2026-04-04 | spec-cli-dispatch | Functional tests for ze set interface create, ze update peeringdb, ze validate config | .ci tests for CLI dispatch commands | spec-cli-dispatch-ci (to be created) | open |
| 2026-04-04 | spec-prometheus-deep | Process/runtime metrics (phase 6), bgp_as_path_loop_detected_total, RPKI/ASPA metrics | Later phases of prometheus work | spec-prometheus-phase6 (to be created) | open |
| 2026-04-04 | spec-prometheus-deep | Max-prefix metrics (may now be unblocked since prefix-limit is implemented) | Was blocked on prefix-limit; may now be feasible | spec-prometheus-plugin-health | open |
| 2026-04-04 | spec-llgr-0-umbrella | Restarting Speaker procedures (RFC 9494/4724), VPN ATTR_SET (RFC 6368), full hard-reset N-bit | Advanced LLGR/GR features | spec-gr-advanced (to be created) | open |
| 2026-04-04 | spec-dns | DNS-over-TLS, DNS-over-HTTPS, DNSSEC validation | Secure DNS transport features | spec-dns-secure (to be created) | open |
| 2026-04-04 | spec-role-otc | Private AS removal filter, AS Confederation OTC (RFC 9234 Section 5) | Advanced OTC features | spec-role-otc-v2 (to be created) | open |
| 2026-04-04 | spec-decorator | Reverse DNS, community name, RPKI status decorators | Additional decorator types | spec-decorator-v2 (to be created) | open |
| | | | | | |
| **2026-04-04 audit of deleted specs -- low (property/chaos/perf tests)** | | | | | |
| 2026-04-04 | spec-listener-0-umbrella | Property test: random listener conflict detection symmetric+transitive | Property testing | spec-property-tests (to be created) | open |
| 2026-04-04 | spec-listener-7-migrate | Property test: round-trip migration | Property testing | spec-property-tests (to be created) | open |
| 2026-04-04 | spec-forward-backpressure | Property tests for overflow ordering under concurrent access | Property testing | spec-property-tests (to be created) | open |
| 2026-04-04 | multiple iface specs | Chaos tests: rapid flapping, DHCP server failure | Chaos testing | spec-chaos-iface (to be created) | open |
| 2026-04-04 | spec-redistribution-filter | Property test: random UPDATEs through filter chains; perf benchmarks | Property testing and benchmarks | spec-property-tests (to be created) | open |
| 2026-04-04 | spec-web-3-config-edit | Concurrent multi-user editing stress test | Stress testing | spec-web-stress (to be created) | open |
| 2026-04-04 | spec-fleet-config | Performance test with >100 concurrent clients | Performance testing | spec-fleet-config-perf (to be created) | open |
