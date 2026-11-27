# Spec: dropped-error-2-bgp-plugins

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | 63 errors in the BGP plugins (BMP, graceful restart, the NLRI decoders, route refresh, IRR, adj-rib-in) are caught and logged only below WARN: a malformed GR capability is ignored, a BMP collector write or peer-bounce Peer Down fails without aborting replay or alerting, a decode request answers nothing or discards stdout write errors, and an IRR or Adj-RIB-In failure vanishes below WARN |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/bgp-plugins.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/component/bgp/plugins/`.
At HEAD on 2026-11-27: **63 instances in 24 files** (`adj_rib_in` 2 in 1 file,
`bmp` 20 in 7 files, `filter_irr` 3 in 3 files, `gr` 12 in 2 files, `nlri/*` 19
in 9 files, `route_refresh` 7 in 2 files). Since 2026-11-02,
`bmp/bmp_locrib.go` (`handleBestChange`) was already resolved and
`filter_irr/stored.go:103` (`storeReader.notice`) entered the population.

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid. A decode request (`handleDecodeNLRI`, `RunDecode`, `run*Decode`, `decodeFlowSpecNLRI`, `RunDecodeMode`, `RunCLIDecodeWithCode`) surfaces the failure to its caller (non-zero exit or wrapped error); BMP `bounceMonitoredPeers` aborts the Adj-RIB replay on a non-disconnected Peer Down write failure |
| 2 | Going on is right and an operator can act | WARN with what, why, and the fix (`ai/rules/user-facing-errors.md`), carrying a registered doctor event `"code"` attribute and raising on the report bus via `internal/component/plugin/issuequeue` over `*sdk.Plugin` (never importing `internal/core/report` directly in any forkable BGP plugin); on a per-route or per-message path, count it in Prometheus and rate-limit WARN per peer/collector and condition in a bounded map |
| 3 | Going on is right and nobody can act (teardown close, RFC 7854 Section 4.2 unknown-AS OPEN mirror skip, `readEvery` reconciliation fallback, `parsed: false` decode contract, broken stderr when already returning exit 1) | `// refuse-by-default: guards <Symbol> -- <why>` on the shape's line or the line above, where `<Symbol>` is visible in the file/package and does not name the enclosing function |

### Complete 63-Site Classification (HEAD 2026-11-27)

Counts across all 24 files: **21 Answer 1**, **30 Answer 2**, **12 Answer 3** (total **63**).

| # | File:Line | Function | Answer | Resolution |
|---|-----------|----------|--------|------------|
| 1 | `adj_rib_in/rib_complex.go:55` | `AdjRIBInManager.installComplexNLRIs` | 2 | Count in `ze_adjribin_nlri_key_errors_total{peer,family,operation="install"}`; rate-limit 1 min per `(peer, family, op)` (bounded 1024, pruned on peer down/removal); WARN + `issuequeue.RaiseError` under `doctor-bgp-adj-rib-in-nlri-key-failed` |
| 2 | `adj_rib_in/rib_complex.go:113` | `AdjRIBInManager.removeComplexNLRIs` | 2 | Same counter with `operation="remove"`, same rate-limit/bounds, WARN + `issuequeue.RaiseError` under `doctor-bgp-adj-rib-in-nlri-key-failed` |
| 3 | `bmp/bmp.go:684` | `closeLog` | 3 | `// refuse-by-default: guards BMPPlugin.handleSession -- teardown close on an ending socket, listener, or plugin connection has no caller recovery` |
| 4 | `bmp/bmp.go:725` | `BMPPlugin.handleSession` defer | 2 | Skip if `bp.isStopping()`; otherwise WARN + `issuequeue.RaiseError` under `doctor-bmp-rib-sync-failed` with router address, cause, and fix `ze show bmp peers` |
| 5 | `bmp/bmp.go:768` | `BMPPlugin.handleSession` | 2 | Skip if `bp.isStopping()`; otherwise WARN + `issuequeue.RaiseError` under `doctor-bmp-receiver-session-error` with router address, cause, and fix `ze show bmp sessions` |
| 6 | `bmp/bmp.go:881` | `BMPPlugin.processTermination` | 2 | Skip if `bp.isStopping()`; otherwise WARN + `issuequeue.RaiseError` under `doctor-bmp-rib-sync-failed` with router address, cause, and fix `ze show bmp peers` |
| 7 | `bmp/bmp.go:1030` | `BMPPlugin.processPeerDown` | 2 | Skip if `bp.isStopping()`; otherwise WARN + `issuequeue.RaiseError` under `doctor-bmp-rib-sync-failed` with router/peer key, cause, and fix `ze show bmp peers` |
| 8 | `bmp/bmp.go:1048` | `BMPPlugin.processRouteMonitoring` | 2 | Skip if `bp.isStopping()`; count in `ze_bmp_route_inject_errors_total{peer}`; rate-limit 1 min per `peerKey` (bounded 1024, pruned on `removeRouter`/`peerDown`); WARN + `issuequeue.RaiseError` under `doctor-bmp-rib-sync-failed` |
| 9 | `bmp/bmp_events.go:462` | `BMPPlugin.bounceMonitoredPeers` | 1 | Fix hidden RFC 8671 Section 7.2 defect: on `errors.Is(err, errNotConnected)` count drop and `break`; on any other write error call `ss.abortAdjReplay(err)`, report via `bp.reportSenderWriteFailure` (`doctor-bmp-sender-delivery-failed`), and `break` |
| 10 | `bmp/bmp_events.go:507` | `BMPPlugin.handleSenderState` (Peer Up) | 2 | Report via `bp.reportSenderWriteFailure(ss.name, "peer-up", err)` (counts `ze_bmp_message_drops_total`, rate-limited 1 min per `(collector, message)`, WARN + `issuequeue.RaiseError` under `doctor-bmp-sender-delivery-failed`) |
| 11 | `bmp/bmp_events.go:531` | `BMPPlugin.handleSenderState` (Peer Down) | 2 | Report via `bp.reportSenderWriteFailure(ss.name, "peer-down", err)` under `doctor-bmp-sender-delivery-failed` |
| 12 | `bmp/bmp_events.go:557` | `BMPPlugin.handleSenderMirror` | 3 | `// refuse-by-default: guards peerHeaderFromEvent -- dynamic peer OPEN precedes AS negotiation; RFC 7854 Section 4.2 has no unknown-AS encoding so mirror skips until negotiated` |
| 13 | `bmp/bmp_events.go:564` | `BMPPlugin.handleSenderMirror` | 2 | Report via `bp.reportSenderWriteFailure(ss.name, "route-mirroring", err)` under `doctor-bmp-sender-delivery-failed` |
| 14 | `bmp/bmp_events.go:628` | `BMPPlugin.handleSenderUpdate` | 2 | Report via `bp.reportSenderWriteFailure(ss.name, "route-monitoring", err)` under `doctor-bmp-sender-delivery-failed` |
| 15 | `bmp/bmp_locrib.go:855` | `BMPPlugin.ensureLocRIBPeerUp` | 2 | Preserve `ss.locRIBUpSent.Store(false)` reset; report via `bp.reportSenderWriteFailure(ss.name, "loc-rib-peer-up", err)` under `doctor-bmp-sender-delivery-failed` |
| 16 | `bmp/bmp_locrib.go:908` | `BMPPlugin.primeLocRIBPeerUp` | 2 | Preserve `ss.locRIBUpSent.Store(false)` reset; report via `bp.reportSenderWriteFailure(ss.name, "loc-rib-peer-up", err)` under `doctor-bmp-sender-delivery-failed` |
| 17 | `bmp/bmp_locrib.go:926` | `BMPPlugin.sendLocRIBEndOfRIB` | 2 | Report via `bp.reportSenderWriteFailure(ss.name, "loc-rib-eor", err)` under `doctor-bmp-sender-delivery-failed` |
| 18 | `bmp/bmp_locrib.go:1023` | `BMPPlugin.sendLocRIBPeerDown` | 2 | Skip if `bp.isStopping()`; otherwise report via `bp.reportSenderWriteFailure(ss.name, "loc-rib-peer-down", err)` under `doctor-bmp-sender-delivery-failed` |
| 19 | `bmp/receiver.go:148` | `BMPPlugin.stopListeners` | 3 | `// refuse-by-default: guards applyReceiverConfig -- shutdown teardown closing active BMP receiver listeners has no recovery` |
| 20 | `bmp/sender.go:466` | `senderSession.writeTerminationLocked` | 3 | `// refuse-by-default: guards terminateAndClose -- best-effort Termination write on a dying collector socket before close` |
| 21 | `bmp/sender_drain.go:135` | `senderSession.drainLoop` | 2 | Preserve `closeLog` + `clearConnAndResetIf`; skip reporting if `ss.isStopping()`; otherwise report via `ss.reportWriteFailure("drain", err)` (wired to `bp.reportSenderWriteFailure`) under `doctor-bmp-sender-delivery-failed` |
| 22 | `bmp/statistics.go:145` | `BMPPlugin.sendStatisticsReports` | 2 | Report via `bp.reportSenderWriteFailure(ss.name, "statistics", err)` under `doctor-bmp-sender-delivery-failed` |
| 23 | `filter_irr/filter_irr.go:556` | `irrPlugin.trackVerdict` | 2 | Increment atomic `invalidPeerCount`; rate-limit 1 min per peer string (bounded 256); WARN + `issuequeue.RaiseError` under `doctor-bgp-irr-invalid-peer` with fix `ze show bgp peer list` |
| 24 | `filter_irr/readmit.go:350` | `readmitter.readmit` | 2 | Rate-limit/deduplicate once per peer condition (`shouldWarn(pf.peer, "confirm")`); WARN + `issuequeue.RaiseError` under `doctor-bgp-irr-readmit-confirm-failed` with fix `ze show bgp peer <peer> detail` |
| 25 | `filter_irr/stored.go:103` | `storeReader.notice` | 3 | `// refuse-by-default: guards readEvery -- unreadable notice already sets r.all so pass runs readEvery to reconcile every stored list and report any unreadable store copy` |
| 26 | `gr/gr.go:368` | `grPlugin.handleStructuredOpen` (code 64) | 2 | Call `gp.reportOpenDecodeFailure(peer, "gr-capability-64", detail, err)` — rate-limited 1 min per `(owner, kind)` (bounded 1024, pruned in `onPeerRemoved`), WARN + `issuequeue.RaiseError` under `doctor-bgp-gr-capability-decode` |
| 27 | `gr/gr.go:382` | `grPlugin.handleStructuredOpen` (code 71) | 2 | Call `gp.reportOpenDecodeFailure(peer, "llgr-capability-71", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 28 | `gr/gr.go:433` | `grPlugin.extractGRCaps` (code 64) | 2 | Call `gp.reportOpenDecodeFailure(peer, "gr-capability-64", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 29 | `gr/gr.go:450` | `grPlugin.extractGRCaps` (code 71) | 2 | Call `gp.reportOpenDecodeFailure(peer, "llgr-capability-71", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 30 | `gr/gr.go:692` | `grPlugin.handleOpenEvent` (code 64 hex) | 2 | Call `gp.reportOpenDecodeFailure(peer, "gr-capability-64-hex", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 31 | `gr/gr.go:697` | `grPlugin.handleOpenEvent` (code 64 parse) | 2 | Call `gp.reportOpenDecodeFailure(peer, "gr-capability-64", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 32 | `gr/gr.go:714` | `grPlugin.handleOpenEvent` (code 71 hex) | 2 | Call `gp.reportOpenDecodeFailure(peer, "llgr-capability-71-hex", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 33 | `gr/gr.go:719` | `grPlugin.handleOpenEvent` (code 71 parse) | 2 | Call `gp.reportOpenDecodeFailure(peer, "llgr-capability-71", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 34 | `gr/gr.go:1086` | `writeOut` | 1 | `writeOut(w, s) error` returns the write error; `RunDecodeMode`, `runCLIDecodeGR`, and `runCLIDecodeLLGR` (`gr/gr_llgr.go`) return exit code `1` on stdout/output write failure (error-path writes to stderr already returning `1` do not mask the error) |
| 35 | `gr/gr_llgr_exchange.go:105` | `grPlugin.handleStructuredSentOpen` | 2 | Call `gp.reportOpenDecodeFailure(peer, "sent-llgr-capability-71", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 36 | `gr/gr_llgr_exchange.go:165` | `grPlugin.handleSentOpenEvent` (hex) | 2 | Convert `addSentLLGRFamilies` / `addLLGRFamilies` to methods on `*grPlugin`; call `gp.reportOpenDecodeFailure(peer, "sent-llgr-capability-71-hex", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 37 | `gr/gr_llgr_exchange.go:177` | `addLLGRFamilies` (parse) | 2 | Call `gp.reportOpenDecodeFailure(peer, "sent-llgr-capability-71", detail, err)` under `doctor-bgp-gr-capability-decode` |
| 38 | `nlri/evpn/plugin.go:404` | `runEVPNDecode` (`write` closure) | 1 | `write` returns `error`; `runEVPNDecode` returns exit code `1` on stdout write failure |
| 39 | `nlri/evpn/plugin.go:489` | `handleDecodeNLRI` (unknown hex) | 1 | `handleDecodeNLRI` returns `error` from `write` and `runEVPNDecode` returns `1` |
| 40 | `nlri/evpn/plugin.go:500` | `handleDecodeNLRI` (JSON marshal) | 1 | `handleDecodeNLRI` returns `error` from `write` and `runEVPNDecode` returns `1` |
| 41 | `nlri/evpn/plugin.go:558` | `decodeEVPNNLRI` | 3 | `// refuse-by-default: guards DecodeNLRIHex -- unparseable EVPN NLRI is explicitly emitted with parsed:false and raw hex per the decode JSON contract` |
| 42 | `nlri/flowspec/plugin_decode.go:50` | `decodeFlowSpecNLRI` (VPN parse) | 1 | `decodeFlowSpecNLRI` returns `(map[string]any, error)`; `DecodeNLRIHex` wraps `fmt.Errorf("%w: %w", errNoValidFlowspecDecoded, err)`; `RunCLIDecode` prints the cause and returns `1`; `plugin_protocol.go` `handleDecodeNLRI` still writes `decoded unknown` on error to preserve `--decode` line-protocol callers |
| 43 | `nlri/flowspec/plugin_decode.go:60` | `decodeFlowSpecNLRI` (non-VPN parse) | 1 | Same as #42: return parse error from `decodeFlowSpecNLRI`, wrap with `errNoValidFlowspecDecoded` in `DecodeNLRIHex`, surface in `RunCLIDecode` |
| 44 | `nlri/labeled/labeled.go:149` | `RunDecode` (`write` closure) | 1 | `write` returns `error`; `RunDecode` returns exit code `1` on stdout write failure |
| 45 | `nlri/ls/plugin.go:189` | `runBGPLSDecode` (`write` closure) | 1 | `write` returns `error`; `runBGPLSDecode` returns exit code `1` on stdout write failure |
| 46 | `nlri/ls/plugin.go:270` | `handleDecodeNLRI` (unknown hex) | 1 | `handleDecodeNLRI` returns `error` from `write` and `runBGPLSDecode` returns `1` |
| 47 | `nlri/ls/plugin.go:287` | `handleDecodeNLRI` (JSON marshal) | 1 | `handleDecodeNLRI` returns `error` from `write` and `runBGPLSDecode` returns `1` |
| 48 | `nlri/ls/plugin.go:333` | `decodeBGPLSNLRI` | 3 | `// refuse-by-default: guards runBGPLSPlugin -- unknown or unparseable BGP-LS NLRI is explicitly emitted with parsed:false and raw hex per RFC 9552 Section 5.1` |
| 49 | `nlri/mup/mup.go:174` | `RunDecode` (`write` closure) | 1 | `write` returns `error`; `RunDecode` returns exit code `1` on stdout write failure |
| 50 | `nlri/mvpn/mvpn.go:176` | `RunDecode` (`write` closure) | 1 | `write` returns `error`; `RunDecode` returns exit code `1` on stdout write failure |
| 51 | `nlri/rtc/rtc.go:150` | `RunDecode` (`write` closure) | 1 | `write` returns `error`; `RunDecode` returns exit code `1` on stdout write failure |
| 52 | `nlri/vpls/vpls.go:140` | `RunDecode` (`write` closure) | 1 | `write` returns `error`; `RunDecode` returns exit code `1` on stdout write failure |
| 53 | `nlri/vpn/vpn.go:275` | `runVPNDecode` (`write` closure) | 1 | `write` returns `error`; `runVPNDecode` returns exit code `1` on stdout write failure |
| 54 | `nlri/vpn/vpn.go:361` | `handleDecodeNLRI` (unknown hex) | 1 | `handleDecodeNLRI` returns `error` from `write` and `runVPNDecode` returns `1` |
| 55 | `nlri/vpn/vpn.go:379` | `handleDecodeNLRI` (JSON marshal) | 1 | `handleDecodeNLRI` returns `error` from `write` and `runVPNDecode` returns `1` |
| 56 | `nlri/vpn/vpn.go:422` | `decodeVPNNLRI` | 3 | `// refuse-by-default: guards DecodeNLRIHex -- unparseable VPN NLRI is explicitly emitted with parsed:false and raw hex per the decode JSON contract` |
| 57 | `route_refresh/register.go:51` | `init` CLIHandler (`RunCLIWithCtx`) | 3 | `// refuse-by-default: guards RunCLIDecodeWithCode -- already returning exit code 1 for out-of-range capability code; a broken stderr write has no recovery` |
| 58 | `route_refresh/route_refresh.go:67` | `RunRouteRefreshPlugin` defer `p.Close()` | 3 | `// refuse-by-default: guards sdk.NewWithConn -- teardown close after p.Run returns has no caller recovery` |
| 59 | `route_refresh/route_refresh.go:97` | `writeOut` | 1 | `writeOut(w, s) error` returns the write error; `RunDecodeMode` returns exit code `1` on output write failure |
| 60 | `route_refresh/route_refresh.go:187` | `RunCLIDecodeWithCode` (unsupported code stderr) | 3 | `// refuse-by-default: guards capNames -- already returning exit code 1 for unsupported capability code; a broken stderr write has no recovery` |
| 61 | `route_refresh/route_refresh.go:194` | `RunCLIDecodeWithCode` (text stdout) | 1 | Check `fmt.Fprintln(stdout, ...)` error and return exit code `1` when writing decoded text to stdout fails |
| 62 | `route_refresh/route_refresh.go:204` | `RunCLIDecodeWithCode` (JSON marshal stderr) | 3 | `// refuse-by-default: guards capNames -- already returning exit code 1 after JSON marshal failure; a broken stderr write has no recovery` |
| 63 | `route_refresh/route_refresh.go:209` | `RunCLIDecodeWithCode` (JSON stdout) | 1 | Check `fmt.Fprintln(stdout, ...)` error and return exit code `1` when writing decoded JSON to stdout fails |

### Doctor Event Codes and `issuequeue` Wiring (All 4 Answer-2 Plugins)

Every new `Warn` under `internal/component/bgp/plugins/` sits inside
`doctor-coverage`'s `siteRoots` (`internal/component/`). Every Answer-2 plugin
registers its transient event codes in `init()` with BOTH `diagnostic.Register`
and `diagnostic.RegisterEventCode`, starts `issuequeue.Start(p)` over its
`*sdk.Plugin`, stops all plugin workers/goroutines BEFORE calling `issues.Stop()`
and BEFORE `p.Close()`, includes `issuequeue.Outcome(reportErr)` on every WARN
line, and logs queue-refusal callbacks at `Warn` under the same code constant.
No plugin imports `internal/core/report` directly.

| Doctor Code | Owner Package & Registration File | Severity & Class | Rate-Limit & Bound | Fix Command |
|-------------|-----------------------------------|------------------|--------------------|-------------|
| `doctor-bgp-adj-rib-in-nlri-key-failed` | `adj_rib_in` (`register_doctor_codes.go`, new) | Error (transient event, `recent-events`) | 1 min per `(peer, family, op)`, map capped at 1024, pruned on peer down/removal | `ze show bgp adj-rib-in` |
| `doctor-bmp-rib-sync-failed` | `bmp` (`register_doctor_codes.go`) | Error (transient event, `recent-events`) | 1 min per `peerKey` on `:1048`, map capped at 1024, pruned on `removeRouter`/`peerDown`; lifecycle-bounded on `:725,881,1030` | `ze show bmp peers` |
| `doctor-bmp-receiver-session-error` | `bmp` (`register_doctor_codes.go`) | Error (transient event, `recent-events`) | One per failing receiver session (`:768`), skipped when `bp.isStopping()` | `ze show bmp sessions` |
| `doctor-bmp-sender-delivery-failed` | `bmp` (`register_doctor_codes.go`) | Error (transient event, `recent-events`) | 1 min per `(collector, message)`, map capped at 256, pruned on collector removal/stop | `ze show bmp collectors` |
| `doctor-bgp-irr-invalid-peer` | `filter_irr` (`doctor.go` + `register.go`) | Error (transient event, `recent-events`) | 1 min per peer string, map capped at 256 | `ze show bgp peer list` |
| `doctor-bgp-irr-readmit-confirm-failed` | `filter_irr` (`doctor.go` + `register.go`) | Error (transient event, `recent-events`) | Deduplicated per peer (`shouldWarn(pf.peer, "confirm")`), map capped at 256, cleared on successful confirm or peer removal | `ze show bgp peer <peer> detail` |
| `doctor-bgp-gr-capability-decode` | `gr` (`doctor.go` + `register.go`) | Error (transient event, `recent-events`) | 1 min per `(owner, kind)`, map capped at 1024, pruned on `onPeerRemoved` (`gr_removal.go`) | `ze show bgp peer <peer> detail` |

Shutdown ordering invariant for `issuequeue`:
1. `adj_rib_in`: starts `issuequeue.Start(p)` inside `runAdjRIBIn`, stores on `AdjRIBInManager`, calls `issues.Stop()` after `p.Run` returns and before `p.Close()`.
2. `bmp`: starts `issuequeue.Start(p)` inside `RunBMPPlugin`, stores on `BMPPlugin`, stops receiver listeners, sender sessions (`drainLoop`), and statistics ticker BEFORE calling `issues.Stop()` and before `p.Close()`.
3. `filter_irr`: adds a `sync.WaitGroup` in `runFilterIRR` tracking `plug.readmit.run(stop)` and `plug.reader.run(ctx)`; after `p.Run` returns, calls `cancel()` and `close(stop)`, waits `wg.Wait()` for both workers to exit, then calls `issues.Stop()`, then `p.Close()`.
4. `gr`: starts `issuequeue.Start(p)` inside `RunGRPlugin`, stores on `grPlugin`, stops GR timers/workers after `p.Run` returns BEFORE calling `issues.Stop()` and before `p.Close()`.

### Prometheus Counters Added (Per-Message / Per-Route Answer-2 Paths)

All counters follow `docs/plugin-development/metrics.md` (`ze_{scope}_{subject}_{event}_total`, singular subject, registered via `ConfigureMetrics` + `atomic.Pointer` struct):

| Metric Name | Type | Labels | Owner & File | Counted Site(s) |
|-------------|------|--------|--------------|-----------------|
| `ze_adjribin_nlri_key_errors_total` | CounterVec | `peer`, `family`, `operation` | `bgp-adj-rib-in` (`adj_rib_in/metrics.go`) | `rib_complex.go:55` (`install`), `rib_complex.go:113` (`remove`) |
| `ze_bmp_route_inject_errors_total` | CounterVec | `peer` | `bgp-bmp` (`bmp/metrics.go`, new, wired in `bmp/register.go`) | `bmp.go:1048` (`processRouteMonitoring`) |
| `ze_bmp_message_drops_total` | CounterVec | `collector`, `message`, `reason` | `bgp-bmp` (`bmp/metrics.go`, new, wired in `bmp/register.go`) | `bmp_events.go:462,507,531,564,628`, `bmp_locrib.go:855,908,926,1023`, `sender_drain.go:135`, `statistics.go:145` |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`
  -> Constraint: repository boundaries, registration, no cross-component bypasses
- [ ] `ai/doors/bgp-plugins.md`
  -> Constraint: every BGP plugin can run forked; never import `internal/core/report` directly; raise issues through `internal/component/plugin/issuequeue` over `*sdk.Plugin`
  -> Constraint: stdout carries the protocol; a plugin logs to stderr only
  -> Constraint: a forwarder never drops a cached UPDATE (cumulative ack); no answer here may skip an ack to report an error
- [ ] `docs/architecture/doctor-and-health-checks.md`
  -> Constraint: every new `Warn`/`Error` under `internal/component/` carries a registered doctor `"code"` attribute; transient event codes register via both `diagnostic.Register` and `diagnostic.RegisterEventCode` and raise through `issuequeue`
- [ ] `docs/architecture/plugin/plugin-system.md` - the process boundary
  -> Constraint: an error returned to the engine travels as an RPC error or non-zero decode exit; its text reaches the operator through the dispatcher (`bgp/cli/decode_plugin.go`)
- [ ] `docs/plugin-development/metrics.md` - naming policy and `ConfigureMetrics` pattern
  -> Constraint: `ze_{scope}_{subject}_{event}_total` with singular subject, `atomic.Pointer` storage, nil-safe when metrics disabled
- [ ] `ai/rules/user-facing-errors.md`
  -> Constraint: every WARN names what failed, the peer/collector/object, why, and the operator fix command
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  -> Constraint: return, count + WARN, or annotate with a visible non-enclosing guard symbol

**Key insights:**
- `internal/component/bgp/cli/decode_plugin.go` already surfaces non-zero exit status via `formatPluginExitErr` for in-process, internal, and external `--decode` runners; returning exit code `1` on stdout write failure in the NLRI, GR, and route-refresh decoders reaches the CLI caller immediately.
- `evpn/plugin.go:558`, `ls/plugin.go:333`, and `vpn/vpn.go:422` already emit `{"parsed": false, "raw": "<HEX>"}` per existing decode JSON contracts (`test/decode/bgp-evpn-1.ci`, `ls/unknown_type_skip_test.go`, `vpn/addpath_decode_test.go`); altering those JSON keys would break pinned contracts, so those 3 sites are Answer 3.
- `flowspec/plugin_decode.go:50,60` must return `(map[string]any, error)` and wrap `errNoValidFlowspecDecoded` in `DecodeNLRIHex` (so `flowspec/plugin_test.go:815` continues to match `"no valid FlowSpec"` while exposing the real parse cause), while `plugin_protocol.go` keeps writing `decoded unknown` on line-protocol errors.
- `bmp/bmp_events.go:462` (`bounceMonitoredPeers`) hid a real RFC 8671 Section 7.2 defect: unlike `writePeerUpLocked` (`:466`) and `replayPeerLocked` (`:471`), a failed `writePeerDownLocked` did `continue` without `ss.abortAdjReplay(err)`.
- `doctor-coverage` treats any helper whose last two parameters are `(msg string, args ...any)` as a log wrapper requiring callers to pass `"code"`. Dedicated domain helpers (`reportSenderWriteFailure`, `reportOpenDecodeFailure`) call `logger().Warn(..., "code", codeConst)` directly so `doctor-coverage` sees every call site covered cleanly.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` and `annotation.go` - shape detection and guard symbol visibility/non-enclosing rules
- [ ] `internal/component/plugin/issuequeue/queue.go` and `raises.go` - async report queue over `*sdk.Plugin`
- [ ] `internal/component/bgp/cli/decode_plugin.go` - `--decode` runner error propagation (`formatPluginExitErr`)
- [ ] `internal/component/bgp/plugins/adj_rib_in/{rib_complex.go,compact_key.go,metrics.go,register.go,rib.go}` - complex NLRI key failures logged at Debug
- [ ] `internal/component/bgp/plugins/bmp/{bmp.go,bmp_events.go,bmp_locrib.go,bmp_replay.go,receiver.go,sender.go,sender_config.go,sender_drain.go,statistics.go,doctor.go,register.go,register_doctor_codes.go}` - receiver RIB sync, session errors, sender delivery failures, and `bounceMonitoredPeers` Peer Down failure logged below WARN
- [ ] `internal/component/bgp/plugins/filter_irr/{filter_irr.go,readmit.go,stored.go,doctor.go,register.go}` - invalid peer IP, readmission confirm failure, and notice decode failure logged at Debug
- [ ] `internal/component/bgp/plugins/gr/{gr.go,gr_llgr_exchange.go,gr_llgr.go,gr_removal.go,doctor.go,register.go,validation_cleanup.go}` - GR/LLGR capability decode failures and `writeOut` errors logged at Debug
- [ ] `internal/component/bgp/plugins/nlri/evpn/plugin.go`, `flowspec/{plugin_decode.go,plugin.go,plugin_protocol.go}`, `labeled/labeled.go`, `ls/plugin.go`, `mup/mup.go`, `mvpn/mvpn.go`, `rtc/rtc.go`, `vpls/vpls.go`, `vpn/vpn.go` - decode stdout writes and FlowSpec parse errors logged below WARN
- [ ] `internal/component/bgp/plugins/route_refresh/{register.go,route_refresh.go}` - decode stdout/stderr writes and teardown close logged below WARN

**Behavior to preserve:**
- Route selection, forwarding, cumulative ack order, GR/LLGR timer semantics, BMP RFC 7854/8671/9069 wire framing, EVPN/LS/VPN `{"parsed": false, "raw": "..."}` JSON decode output, FlowSpec `"no valid FlowSpec"` error prefix and `--decode` line-protocol `decoded unknown` output, and clean shutdown suppression (no spurious warnings during normal plugin stop).

**Behavior to change:**
- Every one of the 63 HEAD sites resolved by Answer 1, Answer 2 (with registered doctor event codes, `issuequeue`, bounded rate-limiting, and Prometheus counters on per-route/per-message paths), or Answer 3 (visible non-enclosing guard annotation).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` (`droppedError`) | Detects all 63 `error-dropped-below-warn` sites under `internal/component/bgp/plugins/` | Applies: acceptance gate AC-1 |
| refuse-by-default guard annotation | `internal/le/arch/refusebydefault/annotation.go` (`ParseGuardAnnotation`) | Accepts `// refuse-by-default: guards <Symbol> -- <why>` when `<Symbol>` is visible and not the enclosing function | Applies: the 12 Answer-3 sites |
| Plugin report queue (`issuequeue`) | `internal/component/plugin/issuequeue/queue.go` (`Start`, `Queue.RaiseError`, `Queue.Stop`, `Outcome`); `docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code" | Raises transient plugin issues over `*sdk.Plugin` (`report-issue` RPC) off a bounded 256-entry async queue with 1 s per-raise and 3 s stop-drain bounds, working identically internal and forked | Extends: wired into `adj_rib_in`, `bmp`, `filter_irr`, and `gr` for all 30 Answer-2 sites |
| Transient doctor event code registration | `internal/core/diagnostic/event_codes.go` (`RegisterEventCode`); `internal/core/diagnostic/diagnostic.go` (`Register`); `docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code" | Registers transient event codes rendered by `doctor-recent-events` (`show errors` / `show doctor`) and validated by `opReportIssue` and `./le arch doctor-coverage check` | Extends: registers the 7 new transient event codes across `adj_rib_in`, `bmp`, `filter_irr`, and `gr` |
| Plugin metrics registration | `docs/plugin-development/metrics.md` (`ConfigureMetrics` + `atomic.Pointer`); `internal/component/bgp/plugins/adj_rib_in/metrics.go` | Registers per-plugin Prometheus counters/gauges in the owning plugin process | Extends: adds `ze_adjribin_nlri_key_errors_total` in `adj_rib_in/metrics.go` and adds `bmp/metrics.go` with `ze_bmp_route_inject_errors_total` and `ze_bmp_message_drops_total` wired in `bmp/register.go` |
| Decode CLI runner error propagation | `internal/component/bgp/cli/decode_plugin.go` (`formatPluginExitErr`) | Converts any non-zero exit code from in-process, internal, or forked `--decode` plugin runners into a returned error | Applies: Answer-1 decode stdout write failures return exit code `1` and surface directly to CLI callers |
| Plugin stderr logger | `slogutil.Logger` per plugin | Structured stderr logging for both internal and forked plugin processes | Applies: all Answer-2 WARN lines carry `"code"` and `issuequeue.Outcome(reportErr)` |
| BMP Loc-RIB `handleBestChange` error reporting | `internal/component/bgp/plugins/bmp/bmp_locrib.go` (`handleBestChange`); `docs/guide/bmp.md`, "Sender Behavior" | Already warns and reports collector queue failures under `doctor-recent-events` | Extends: aligns the remaining 10 BMP sender sites with the same operator-facing model using `issuequeue` + `doctor-bmp-sender-delivery-failed` |

## Design documents

Every design page and decision row derived by `./le spec design documents` for
the modified and created files is evaluated below (20 design pages, 27 decision
rows). Verdict count: **0 departures**.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Every Answer-2 BGP plugin (`adj_rib_in`, `bmp`, `filter_irr`, `gr`) raises transient doctor events via `internal/component/plugin/issuequeue` over `*sdk.Plugin` (`report-issue` RPC), never importing `internal/core/report` directly, so internal and forked placements report identically. |
| `ai/doors/bgp-plugins.md` | Boundaries | the door covering all 39 modified and created files under `internal/component/bgp/plugins/` | "Reach the engine through the bus, DirectBridge or a text command, never a plain call into `reactor` or another component." | agrees | No BGP plugin adds a direct call into `reactor` or `internal/core/report`; runtime problems travel over `*sdk.Plugin` via `issuequeue`, decode failures return exit code `1` to `bgp/cli/decode_plugin.go`, and cumulative ack order is preserved. |
| `docs/architecture/plugin/rib-storage-design.md` | JSON received-route reconciliation | `// Design:` and source anchor of `adj_rib_in/rib_complex.go`, `adj_rib_in/rib.go` | "Its routes are not stored, selected or re-sent, and the RIB says so at Warn once per peer and family, naming the fix, on both receive paths (`reportUnstorableFamily`)." | agrees | `adj_rib_in/rib_complex.go:55,113` follows the same pattern when a complex NLRI key cannot be derived: increments `ze_adjribin_nlri_key_errors_total{peer,family,operation}`, warns at most once per minute per `(peer, family, op)` naming `ze show bgp adj-rib-in`, and raises `doctor-bgp-adj-rib-in-nlri-key-failed` via `issuequeue`. |
| `docs/architecture/api/commands.md` | Overview | anchored to `adj_rib_in/rib.go`, `gr/gr.go` | - | silent | Page documents the API command verb taxonomy and transport metadata; this spec adds or changes no CLI command, verb, pipe operator, or API answer shape. |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `adj_rib_in/rib.go` | "The protocol is the same for all invocation modes (internal goroutine, external subprocess)." | agrees | Raising runtime doctor events through `issuequeue` over `*sdk.Plugin` (`report-issue` RPC) uses the existing process protocol identically in internal and forked modes without adding a new RPC or changing `DirectBridge`. |
| `docs/plugin-development/metrics.md` | Naming Policy | `// Design:` of `adj_rib_in/metrics.go`; a page this spec modifies | "The subject is always a **singular noun** describing the category of thing measured." | agrees | All three new counters (`ze_adjribin_nlri_key_errors_total`, `ze_bmp_route_inject_errors_total`, `ze_bmp_message_drops_total`) follow `ze_{scope}_{subject}_{event}_total` with singular subjects and `ConfigureMetrics` + `atomic.Pointer` wiring, and are added to this page's scope table and inventory. |
| `docs/architecture/core-design.md` | Command engine and composition roots | `// Design:` of `bmp/{bmp,bmp_events,sender,sender_config,sender_drain}.go`, `gr/{gr,gr_llgr,gr_removal}.go`, `route_refresh/route_refresh.go`; anchored to `filter_irr/stored.go` | "Every personality enters through `main` in `cmd/ze/main.go`, which seals the plugin registry (`registry.Seal`) before it dispatches, so a registration made after the `init()` functions ran is refused." | agrees | All 7 new doctor event code registrations (`diagnostic.Register` + `diagnostic.RegisterEventCode`) and BMP's `ConfigureMetrics` field stay strictly in `init()` before `registry.Seal`, and per-message error handling preserves zero-copy `WireUpdate` and pool storage. |
| `docs/architecture/api/birdwatcher-compat.md` | 3. Endpoints | anchored to `bmp/bmp.go` | - | silent | Page documents the Birdwatcher-compatible Looking Glass JSON API (`internal/component/lg/`); this spec modifies no Looking Glass endpoint or JSON field. |
| `rfc/short/rfc9069.md` | Compliance Checklist | `// Design:` of `bmp/bmp_locrib.go` | "the Loc-RIB Peer Up is emitted once per RIB instance per collector session, behind the one-shot senderSession.locRIBUpSent.CompareAndSwap taken by ensureLocRIBPeerUp and primeLocRIBPeerUp" | agrees | `ensureLocRIBPeerUp` (`:855`) and `primeLocRIBPeerUp` (`:908`) preserve the `ss.locRIBUpSent.Store(false)` reset on write failure while reporting the delivery error under `doctor-bmp-sender-delivery-failed`. |
| `docs/guide/bmp.md` | Sender Behavior | `// Design:` of `bmp/receiver.go`, `bmp/statistics.go`; a page this spec modifies | "If a Loc-RIB update cannot be queued for a collector, Ze logs a warning with the collector and the original error, and continues sending to the other collectors. It also retains the failure as an error event under `doctor-recent-events`." | agrees | This spec extends that exact collector-failure reporting pattern to all remaining BMP sender delivery sites via `issuequeue` + `doctor-bmp-sender-delivery-failed`, fixes `bounceMonitoredPeers` (`:462`) to call `abortAdjReplay` on write error, and documents the new BMP counters and error reporting on this page. |
| `docs/architecture/doctor-and-health-checks.md` | Event codes | `// Design:` of `bmp/register_doctor_codes.go` | "A plugin raises through `sdk.Plugin.RaiseIssue`, which sends `report-issue` to the daemon, never through `internal/core/report` directly" | agrees | Every Answer-2 plugin (`adj_rib_in`, `bmp`, `filter_irr`, `gr`) registers its transient event codes with both `diagnostic.Register` and `diagnostic.RegisterEventCode`, tags every new `Warn` with the code constant and `issuequeue.Outcome`, and raises exclusively via `issuequeue` over `*sdk.Plugin`. |
| `docs/architecture/bgp/filter-irr.md` | The decisions | `// Design:` and source anchor of `filter_irr/{filter_irr,readmit,stored,doctor}.go` | "It reaches the owner only through stored keys and notices, so it works the same run internal or forked." | agrees | `filter_irr` keeps its stored-key/notice boundary untouched, raises `doctor-bgp-irr-invalid-peer` and `doctor-bgp-irr-readmit-confirm-failed` via `issuequeue` so internal and forked runs match, joins `readmit`/`reader` workers with a `WaitGroup` before `issues.Stop()`, and annotates `stored.go:103` (`guards readEvery`). |
| `docs/architecture/irr-owner.md` | What a consumer reads | anchored to `filter_irr/register.go` | "Every other error means a stored value exists and cannot be used: keep what is enforced, log a WARN with a doctor code, and report it (AC-22)." | agrees | `filter_irr/stored.go:103` handles an unreadable notice (not a stored dataset read) by keeping `r.all = true` and poking `storeReader.pass`, which runs `readEvery` and enforces the WARN + doctor-code contract on any unreadable stored copy; `filter_irr/register.go` adds only `diagnostic.RegisterEventCode` calls for the two new filter event codes. |
| `docs/architecture/api/architecture.md` | Message Bus and Participant Wiring | anchored to `gr/gr.go`, `nlri/flowspec/plugin_decode.go` | - | silent | Page describes the bus participant architecture and pipe transport; this spec adds no bus participant, route, or command grammar, and its 8 decision rows are evaluated individually below. |
| `docs/architecture/plugin/plugin-relationships.md` | 2. BGP Sub-Plugins | anchored to `gr/gr.go` | - | silent | Page catalogs inter-plugin subscriptions and dependencies; this spec adds no new plugin dependency, subscription, or inter-plugin command. |
| `docs/guide/graceful-restart.md` | What Ze Advertises | `// Design:` of `gr/gr_llgr_exchange.go`, `gr/doctor.go` | "A family an external plugin registers at runtime with no splitter is left out, and Ze logs a warning naming the peer and the family." | agrees | Malformed code 64 and code 71 OPEN capabilities similarly log a rate-limited WARN naming the peer, capability kind, cause, and fix `ze show bgp peer <peer> detail`, and raise `doctor-bgp-gr-capability-decode` via `issuequeue`. |
| `docs/architecture/wire/nlri-evpn.md` | Common Header | `// Design:` and source anchor of `nlri/evpn/plugin.go` | "All EVPN NLRIs share this structure:" | agrees | EVPN wire framing is unchanged; `evpn/plugin.go:558` preserves the `{"code", "parsed": false, "raw"}` JSON contract (Answer 3) while `runEVPNDecode` and `handleDecodeNLRI` (`:404,489,500`) surface stdout write errors as exit code `1` (Answer 1). |
| `docs/architecture/wire/nlri-flowspec.md` | A malformed NLRI resets the session | `// Design:` of `nlri/flowspec/{plugin_decode,plugin,plugin_protocol}.go` | "The receive path judges every FlowSpec NLRI of an MP_REACH_NLRI or MP_UNREACH_NLRI before any plugin sees it, with the checks of the parser the plugin's decoder uses (`flowspec.ValidateFlowSpec` and `ValidateFlowSpecVPN` share them with `ParseFlowSpec`)" | agrees | Session receive validation is untouched; `flowspec/plugin_decode.go:50,60` propagates the `ParseFlowSpec`/`ParseFlowSpecVPN` error out of `decodeFlowSpecNLRI`, wrapping `errNoValidFlowspecDecoded` in `DecodeNLRIHex` and printing it in `RunCLIDecode` while `plugin_protocol.go` keeps `decoded unknown` for line-protocol callers. |
| `docs/architecture/wire/nlri.md` | Overview | `// Design:` of `nlri/{labeled,mup,mvpn,rtc,vpls,vpn}`; anchored to `nlri/vpn/vpn.go` | "NLRI (Network Layer Reachability Information) represents route prefixes in BGP UPDATE messages." | agrees | Wire codecs and family registrations are unchanged; all six plugins return exit code `1` on decode stdout write failure (Answer 1) and `vpn/vpn.go:422` preserves its `{"parsed": false, "raw"}` JSON fallback (Answer 3). |
| `docs/architecture/wire/nlri-bgpls.md` | Wire Format Overview | `// Design:` and source anchor of `nlri/ls/plugin.go` | "Descriptors (variable)" | agrees | BGP-LS wire parsing is unchanged; `ls/plugin.go:333` preserves the `{"parsed": false, "raw"}` skip contract per RFC 9552 Section 5.1 (Answer 3) while `runBGPLSDecode` and `handleDecodeNLRI` (`:189,270,287`) return exit code `1` on stdout write failure (Answer 1). |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 on `docs/architecture/api/commands.md` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec adds or modifies no CLI command path, subcommand, or pipe operation. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 on `docs/architecture/api/commands.md` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec defines or changes no pipe override or command answer shape. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/commands.md` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec does not alter any pipe operation or command execution semantics. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/process-protocol.md` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no new engine operation and changes no plugin authorization profile; it uses the already-declared `report-issue` operation via `issuequeue`. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 on `docs/architecture/api/process-protocol.md` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec touches no engine-to-plugin event delivery queue; `issuequeue` is the plugin-to-daemon diagnostic report queue specified in `docs/architecture/doctor-and-health-checks.md`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec touches only BGP plugin runtime error handling and modifies no data-plane feature, `fib` configuration rule, or VPP commit validator. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec modifies no data-plane mismatch validation at start, commit, reload, or `ze config validate`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec modifies no firewall YANG schema or VPP firewall verifier. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec adds or renames no data-plane selector, backend registration, or `ze:backend` value. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches no BGP egress next-hop rewriting or export filter chain configuration. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec modifies no policy `modify` syntax or next-hop chain reference. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not touch `policychain.Chain` or `rpc.FilterInsertion`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not alter export-chain next-hop folding or `wireOverride` provenance. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec touches no import/export next-hop validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec modifies no route origination or forwarded next-hop policy. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec modifies no implied-filter seam or filter-list display. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec introduces no startup refusal; Answer-2 runtime failures keep plugins and `ze` running while reporting under registered doctor event codes via `issuequeue` so `ze doctor` and `ze show errors` surface them with their fixes. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec modifies no startup data-plane exclusion or mismatch check. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec does not touch `\| implied` output formatting. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This spec adds no bus participant or message registration vocabulary. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This spec modifies no bus `send > to` routing rules. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec modifies no bus `receive > from` rules. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This spec modifies no bus allow-list enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec modifies no bus filter compilation or conflict check. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This spec adds no bus arbitration or role exclusivity rule. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/api/architecture.md` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec modifies no bus display. |
| `docs/guide/graceful-restart.md` | Design decisions | decision of 2026-11-22 on `docs/guide/graceful-restart.md` | "Graceful Restart and Long-Lived Graceful Restart use symmetric `short-lived` and `long-lived` containers inside `graceful-restart`, both carrying keyed `list family` (`restart-time` inside `short-lived`; mandatory `stale-time` per family inside `long-lived`), with legacy `long-lived-stale-time`, top-level `restart-time`, and `family { name [...] }` removed" | agrees | This spec modifies no GR or LLGR configuration schema, capability advertisement syntax, or legacy fallback; it only upgrades malformed code 64 and code 71 OPEN capability decode errors from Debug to rate-limited WARN + `doctor-bgp-gr-capability-decode`. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call inside a BGP plugin: a complex NLRI key derivation (`adj_rib_in`), a BMP receiver RIB sync / session read / collector write / peer bounce (`bmp`), an IRR peer address parse or readmission confirmation (`filter_irr`), a GR/LLGR OPEN capability decode (`gr`), or a `--decode` stdout write / FlowSpec NLRI parse (`nlri/*`, `gr`, `route_refresh`).

### Transformation Path
1. **Answer 1 (21 sites):** Return the write or parse error up to the decode runner (returning exit code `1`, which `bgp/cli/decode_plugin.go` surfaces as a CLI error, or wrapping `errNoValidFlowspecDecoded` in `flowspec.DecodeNLRIHex`), or in `bmp/bmp_events.go:462` abort the Adj-RIB replay (`ss.abortAdjReplay(err)`) and report the collector write failure before breaking.
2. **Answer 2 (30 sites):** On per-route/per-message paths, increment the Prometheus counter (if metrics enabled); check the bounded per-key rate limiter; emit one structured stderr WARN naming the peer/collector, operation, cause, fix, registered doctor `"code"`, and `issuequeue.Outcome(reportErr)`; enqueue `issues.RaiseError(code, msg, fix, failedCb)` over `*sdk.Plugin` (`report-issue` RPC) so the daemon's `doctor-recent-events` surfaces it in `show errors` and `show doctor`.
3. **Answer 3 (12 sites):** Keep the deliberate guard/teardown behavior and document the visible, non-enclosing guard symbol with `// refuse-by-default: guards <Symbol> -- <why>`.

### Changed-Wire Census

**Search evidence:** Scope `internal/component/bgp/`, `pkg/plugin/`, `test/decode/`, `test/plugin/`; terms `formatPluginExitErr`, `decoded unknown`, `parsed.*false`, `MethodReportIssue`, `bounceMonitoredPeers`; positive controls `internal/component/bgp/cli/decode_plugin.go` (`formatPluginExitErr`), `internal/component/plugin/server/dispatch_report.go` (`opReportIssue`), `test/decode/bgp-evpn-1.ci`.

No wire format, RPC message schema, JSON output key, or CLI command grammar changes in this spec: `decode_plugin.go` already surfaces non-zero decode exit codes, `evpn`/`ls`/`vpn` preserve `{"parsed": false, "raw": "<HEX>"}`, `flowspec` preserves `decoded unknown` and `errNoValidFlowspecDecoded`, `issuequeue` uses the existing `report-issue` RPC, and `bmp/bmp_events.go:462` aligns Peer Down failure with existing RFC 8671 Section 7.2 replay-abort handling without changing BMP wire framing.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin <-> daemon (`report-issue` RPC) | `internal/component/plugin/issuequeue` over `*sdk.Plugin` (works internal and forked) | Yes (`./le plugin boundary check` OK; `TestComplexNLRIKeyErrorWarnsCountsAndRaisesDoctorCode`, `TestBounceMonitoredPeersAbortsReplayOnPeerDownError`, `TestBMPSenderDeliveryFailureWarnsCountsAndRateLimits`, `TestBMPReceiverRIBSyncAndSessionErrorsRaiseDoctorCodes`, `TestFilterIRRInvalidPeerAndReadmitConfirmRaiseDoctorCodes`, `TestGROpenUndecodableCapabilityWarnsAndRaisesDoctorCode`, `test/plugin/bmp-locrib.ci`) |
| Plugin <-> CLI decode caller | Non-zero exit code via `bgp/cli/decode_plugin.go` or wrapped Go error | Yes (`TestGRWriteOutReturnsErrorAndExitOne`, `TestFlowSpecDecodeSurfacesParseCause`, `TestEVPNDecodeReturnsExitOneOnWriteError`, `TestBGPLSDecodeReturnsExitOneOnWriteError`, `TestVPNDecodeReturnsExitOneOnWriteError`, `TestSimpleNLRIDecodeReturnsExitOneOnWriteError`, `TestRouteRefreshDecodeReturnsExitOneOnWriteError`, `test/decode/bgp-evpn-1.ci`, `test/decode/bgp-open-graceful-restart.ci`, `test/decode/capability-llgr.ci`) |
| Plugin <-> operator | Structured stderr log at WARN with doctor `"code"`, `show errors`/`show doctor` (`recent-events`), Prometheus counters | Yes (`./le arch doctor-coverage check` 0 findings in `internal/component/bgp/plugins/`; all 7 event codes registered via `diagnostic.Register` + `diagnostic.RegisterEventCode`; 3 Prometheus CounterVecs verified in unit tests and documented in `docs/plugin-development/metrics.md` + `docs/guide/bmp.md`) |

### Integration Points
- `internal/component/plugin/issuequeue` over `*sdk.Plugin` in `adj_rib_in`, `bmp`, `filter_irr`, and `gr`.
- `diagnostic.Register` + `diagnostic.RegisterEventCode` in `init()` of those 4 plugins.
- `ConfigureMetrics` + `atomic.Pointer` in `adj_rib_in/metrics.go` and new `bmp/metrics.go` (wired in `bmp/register.go`).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Runtime issues travel via `issuequeue` -> `*sdk.Plugin.RaiseIssue` -> `report-issue` RPC; decode errors exit `1` to `decode_plugin.go` |
| No unintended coupling (components stay isolated) | Yes | Zero imports of `internal/core/report` in any forkable BGP plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `issuequeue`, `diagnostic.RegisterEventCode`, `ConfigureMetrics`, and `decode_plugin.go` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-message paths update atomic counters and check bounded rate-limit maps without copying wire slices |
| Registration over hardcoding, outbound | Yes | All 7 transient doctor codes register via `diagnostic.Register` + `diagnostic.RegisterEventCode` in `init()` |
| Registration over hardcoding, inbound | Yes | BMP metrics register via `Registration.ConfigureMetrics` in `bmp/register.go` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 63 HEAD sites across 24 files are the complete population | `./le arch refuse-by-default report '\|' json` filtered to `rule == "error-dropped-below-warn"` under `internal/component/bgp/plugins/` on 2026-11-27 | Re-run report and reconcile any new/moved site | Re-run filtered report before and after edits (AC-1) | confirmed |
| A-2 | Every `--decode` caller handles exit code `1` cleanly without breaking existing decode tests | Read `internal/component/bgp/cli/decode_plugin.go` (`formatPluginExitErr`), `flowspec/plugin_test.go`, `test/decode/bgp-evpn-1.ci`, `ls/unknown_type_skip_test.go`, `vpn/addpath_decode_test.go` | A caller expected exit `0` on write failure or different JSON keys | Run all NLRI/GR/route_refresh unit tests and `test/decode/` `.ci` tests | confirmed |
| A-3 | Every Answer-2 plugin (`adj_rib_in`, `bmp`, `filter_irr`, `gr`) owns a `*sdk.Plugin` instance and cleanly joins all worker goroutines before `issues.Stop()` and `p.Close()` | Read `adj_rib_in/rib.go`, `bmp/bmp.go`, `filter_irr/filter_irr.go`, `gr/gr.go` (`filter_irr` needs an explicit `sync.WaitGroup` for `readmit` and `reader` workers) | Late worker raise hits a stopped queue or races with `p.Close()` | Race-detector unit tests (`go test -race`) for all 4 plugins | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per BMP route-monitoring message, complex NLRI, or malformed OPEN floods stderr or the 256-entry `issuequeue` under sustained failure | Log volume spikes or `report-error=queue full` under burst tests | Rate-limit 1 min per `(collector, message)`, `(peer, family, op)`, or `(owner, kind)` inside bounded maps (256 or 1024 entries, pruned on peer/collector teardown), and count every occurrence in Prometheus |
| R-2 | Spurious doctor warnings fire during normal daemon/plugin shutdown when pipes or sockets close | Shutdown tests record unexpected `doctor-bmp-*` warnings | Gate receiver/sender shutdown paths on `bp.isStopping()` / `ss.isStopping()`, and drain `issues.Stop()` only after worker goroutines have exited |
| R-3 | Unbounded peer-supplied bytes or strings echoed into WARN logs or doctor report messages | Security review or fuzz/long-input test shows huge log/report line | Log only fixed labels, capability/error codes, bounded hex prefixes or parsed `err.Error()`, never raw unbounded peer payloads |
| R-4 | A shared logging helper accidentally triggers `doctor-coverage`'s wrapper rule (`(msg string, args ...any)`) | `./le arch doctor-coverage check` refuses helper callers | Use dedicated domain helpers (`reportSenderWriteFailure`, `reportOpenDecodeFailure`) that pass explicit typed arguments and call `logger().Warn(..., "code", codeConst)` directly |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise or spurious `show errors` findings during shutdown, a race on `issuequeue.Stop()` during plugin exit, or a broken `--decode` test if JSON/line-protocol contracts are disturbed |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Specs touching BMP, GR, IRR filter, Adj-RIB-In, NLRI codecs, or `doctor-coverage` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | -> | `droppedError` over `internal/component/bgp/plugins/` | Report filtered to `internal/component/bgp/plugins/`: zero `error-dropped-below-warn` findings |
| `./le arch doctor-coverage check` | -> | Doctor code coverage over changed lines in `internal/component/bgp/plugins/` | Exit code `0`: every new `Warn` carries a registered doctor event code |
| An OPEN whose GR/LLGR capability hex or bytes fail decode | -> | `grPlugin.handleOpenEvent` / `handleStructuredOpen` / `handleSentOpenEvent` -> `reportOpenDecodeFailure` | `TestGROpenUndecodableCapabilityWarnsAndRaisesDoctorCode` (`internal/component/bgp/plugins/gr/gr_test.go`) |
| A BMP peer bounce where `writePeerDownLocked` fails with a write error | -> | `BMPPlugin.bounceMonitoredPeers` -> `abortAdjReplay` + `reportSenderWriteFailure` | `TestBounceMonitoredPeersAbortsReplayOnPeerDownError` (`internal/component/bgp/plugins/bmp/bmp_events_test.go`) |
| A decode request with a failing stdout writer or malformed FlowSpec NLRI | -> | `runEVPNDecode`/`runVPNDecode`/`runBGPLSDecode`/`RunDecode`/`RunDecodeMode`/`RunCLIDecodeWithCode`/`DecodeNLRIHex` | `TestVPNDecodeReturnsExitOneOnWriteError`, `TestFlowSpecDecodeSurfacesParseCause` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report '\|' json`, filtered to `internal/component/bgp/plugins/` | Zero `findings` entries with rule `error-dropped-below-warn` across all 24 files | The 12 Answer-3 sites listed in the 63-site table carry valid `// refuse-by-default: guards <Symbol> -- <why>` annotations (teardown closes, RFC 7854 §4.2 dynamic-peer OPEN mirror skip, `readEvery` reconciliation fallback, `parsed: false` decode JSON contract, broken stderr when already returning exit 1) |
| AC-2 | `./le arch doctor-coverage check` and `./le plugin boundary check` over the changed tree | Both exit `0`: every new `Warn` carries a registered `"code"` constant, all 7 new codes are registered with both `diagnostic.Register` and `diagnostic.RegisterEventCode`, and no forkable BGP plugin imports `internal/core/report` | None |
| AC-3 | Every Answer-1 site (all 21: `bmp_events.go:462`, `gr.go:1086`, `route_refresh.go:97,194,209`, `flowspec/plugin_decode.go:50,60`, and the 14 NLRI stdout write sites) | Unit tests prove the caller observes the failure: exit code `1` on stdout write failure, wrapped parse error preserving `errNoValidFlowspecDecoded` in `flowspec.DecodeNLRIHex`, and `abortAdjReplay` + error report in `bounceMonitoredPeers` | `flowspec/plugin_protocol.go` `handleDecodeNLRI` still writes `decoded unknown` on error to preserve existing `--decode` stdin/stdout line-protocol callers |
| AC-4 | Every Answer-2 plugin (`adj_rib_in`, `bmp`, `filter_irr`, `gr`) under each failure class | Unit tests prove each condition logs a WARN naming the peer/collector, operation, cause, fix, doctor `"code"`, and `issuequeue.Outcome`, enqueues a `report-issue` RPC over `*sdk.Plugin`, increments the Prometheus counter on per-route/per-message paths, enforces its 1-minute bounded rate limit, and stays silent during normal plugin shutdown | Rate-limited repeat occurrences within the 1-minute window suppress repeat WARN/raise lines while still incrementing Prometheus counters so hot paths cannot flood logs or the 256-entry `issuequeue` |
| AC-5 | Every Answer-3 annotation (all 12 sites) | `./le arch refuse-by-default check` accepts every annotation, confirming each `<Symbol>` is visible in the file/package and is not the enclosing function | None |
| AC-6 | Scoped Go package tests (`-race`) and affected `test/plugin` and `test/decode` `.ci` tests | All pass green with no regressions to route selection, forwarding, cumulative acks, `parsed: false` decode contracts, or clean plugin shutdown | None |

## End-to-End User Stories

1. **Operator debugging a malformed Graceful Restart capability:** A peer sends an OPEN carrying a truncated code 64 or code 71 capability. Instead of silently ignoring it at Debug, `bgp-gr` emits a rate-limited WARN naming the peer, capability code, parse error, and `ze show bgp peer <peer> detail`, and raises `doctor-bgp-gr-capability-decode` via `issuequeue` so `ze show errors` and `ze show doctor` surface it whether `bgp-gr` runs internal or forked.
2. **Operator running BMP monitoring to a stalled collector:** When a collector write fails during Peer Up, Peer Down, Route Monitoring, Mirroring, Statistics, Loc-RIB, drain, or a policy bounce, `bgp-bmp` aborts any in-flight Adj-RIB replay on bounce failure (RFC 8671 §7.2), increments `ze_bmp_message_drops_total{collector,message,reason}`, logs one rate-limited WARN per minute per `(collector, message)`, and raises `doctor-bmp-sender-delivery-failed` with fix `ze show bmp collectors`.
3. **Operator decoding a BGP NLRI or capability from the CLI:** Running `ze bgp decode` when stdout is broken or when FlowSpec hex fails parsing returns exit code `1` with the underlying error surfaced through `bgp/cli/decode_plugin.go`, rather than exiting `0` with truncated or blank output.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestComplexNLRIKeyErrorWarnsCountsAndRaisesDoctorCode` | `internal/component/bgp/plugins/adj_rib_in/rib_complex_test.go` | AC-2, AC-4 (`rib_complex.go:55,113`: counter, 1-min rate limit, map bound, `issuequeue` raise) | PASS (rerun owed after Run 1 fix) |
| `TestBounceMonitoredPeersAbortsReplayOnPeerDownError` | `internal/component/bgp/plugins/bmp/bmp_events_test.go` | AC-3, AC-4 (`bmp_events.go:462`: `abortAdjReplay` on write error, no abort on `errNotConnected`, `doctor-bmp-sender-delivery-failed`) | PASS |
| `TestBMPSenderDeliveryFailureWarnsCountsAndRateLimits` | `internal/component/bgp/plugins/bmp/bmp_events_test.go` | AC-2, AC-4 (`bmp_events.go:507,531,564,628`, `bmp_locrib.go:855,908,926,1023`, `sender_drain.go:135`, `statistics.go:145`, `locRIBUpSent` reset preserved, shutdown silent) | PASS |
| `TestBMPRateLimitMapsBoundedAndPruned` | `internal/component/bgp/plugins/bmp/bmp_events_test.go` | AC-4, Boundary Tests (receiver cap 1024, sender cap 256, peer/router/collector pruning) | PASS |
| `TestBMPDrainLoopWriteFailureReportsAndResets` | `internal/component/bgp/plugins/bmp/bmp_events_test.go` | AC-2, AC-4 (`sender_drain.go:135`: socket write error resets queue and reports via `reportWriteFailure`) | PASS |
| `TestBMPReceiverRIBSyncAndSessionErrorsRaiseDoctorCodes` | `internal/component/bgp/plugins/bmp/bmp_test.go` | AC-2, AC-4 (`bmp.go:725,768,881,1030,1048`, `ze_bmp_route_inject_errors_total`, shutdown silent) | PASS |
| `TestFilterIRRInvalidPeerAndReadmitConfirmRaiseDoctorCodes` | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | AC-2, AC-4 (`filter_irr.go:556`, `readmit.go:350`, `WaitGroup` worker join before `issues.Stop()`) | PASS (rerun owed after Run 1 fix) |
| `TestFilterIRRInvalidPeerBoundsLongAndControlInput` | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | Security checklist, AC-4 (Run 1 regression test: oversized/control-byte `peerAddr` passes `rpc.ValidateReportIssue` without echoing raw `netip.ParseAddr` input) | Added in Run 1 fix (rerun owed) |
| `TestGROpenUndecodableCapabilityWarnsAndRaisesDoctorCode` | `internal/component/bgp/plugins/gr/gr_test.go` | AC-2, AC-4 (`gr.go:368,382,433,450,692,697,714,719`, `gr_llgr_exchange.go:105,165,177`, 1-min rate limit, pruning on `onPeerRemoved`) | PASS (rerun owed after Run 1 fix) |
| `TestGRWriteOutReturnsErrorAndExitOne` | `internal/component/bgp/plugins/gr/gr_test.go` | AC-3 (`gr.go:1086`, `RunDecodeMode`, `runCLIDecodeGR`, `runCLIDecodeLLGR`) | PASS (rerun owed after Run 1 fix) |
| `TestFlowSpecDecodeSurfacesParseCause` | `internal/component/bgp/plugins/nlri/flowspec/plugin_test.go` | AC-3 (`plugin_decode.go:50,60`: `DecodeNLRIHex` wraps `errNoValidFlowspecDecoded` + parse cause; `RunCLIDecode` returns `1`; line protocol still writes `decoded unknown`) | PASS |
| `TestEVPNDecodeReturnsExitOneOnWriteError` | `internal/component/bgp/plugins/nlri/evpn/plugin_test.go` | AC-3 (`evpn/plugin.go:404,489,500`; `parsed: false` preserved for `:558`) | PASS |
| `TestBGPLSDecodeReturnsExitOneOnWriteError` | `internal/component/bgp/plugins/nlri/ls/plugin_test.go` | AC-3 (`ls/plugin.go:189,270,287`; `parsed: false` preserved for `:333`) | PASS |
| `TestVPNDecodeReturnsExitOneOnWriteError` | `internal/component/bgp/plugins/nlri/vpn/vpn_test.go` | AC-3 (`vpn/vpn.go:275,361,379`; `parsed: false` preserved for `:422`) | PASS |
| `TestSimpleNLRIDecodeReturnsExitOneOnWriteError` | `internal/component/bgp/plugins/nlri/{labeled,mup,mvpn,rtc,vpls}/*_test.go` | AC-3 (`labeled:149`, `mup:174`, `mvpn:176`, `rtc:150`, `vpls:140`) | PASS |
| `TestRouteRefreshDecodeReturnsExitOneOnWriteError` | `internal/component/bgp/plugins/route_refresh/route_refresh_test.go` | AC-3 (`route_refresh.go:97,194,209`) | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unannotated `error-dropped-below-warn` findings under `internal/component/bgp/plugins/` | 0 | 0 | N/A | 1 |
| Rate-limit map capacity (`adj_rib_in`, `bmp` receiver, `gr`: 1024; `bmp` sender, `filter_irr`: 256) | 0..cap | cap | N/A | cap+1 (evicted/bounded, never grows unbounded) |
| `issuequeue` queue depth (256) | 0..256 | 256 | N/A | 257 (non-blocking refusal logged via `issuequeue.Outcome`, never blocks packet/event loop) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/decode/bgp-evpn-1.ci`, `test/decode/bgp-open-graceful-restart.ci`, `test/decode/capability-llgr.ci` | `test/decode/` | EVPN/FlowSpec/LS/VPN/GR/Route-Refresh decode behavior preserved; malformed/write-error paths behave as specified | PASS |
| `test/plugin/bmp-receiver-messages.ci`, `test/plugin/bmp-sender-peer-up-open.ci`, `test/plugin/bmp-sender-route-monitoring.ci`, `test/plugin/bmp-sender-statistics.ci`, `test/plugin/bmp-locrib.ci` | `test/plugin/` | Normal sessions produce zero spurious warnings; collector/peer errors surface cleanly without flooding; `bmp-locrib.ci` proves `awaitingHandshake` / `OnStarted` gate | PASS |

## Files to Modify

- `internal/component/bgp/plugins/adj_rib_in/rib_complex.go` - Answer 2 at lines 55, 113 (`doctor-bgp-adj-rib-in-nlri-key-failed`, rate-limited, counted)
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - wire `issuequeue.Start(p)` / `issues.Stop()`, bounded rate-limit map, prune on peer down/removal
- `internal/component/bgp/plugins/adj_rib_in/metrics.go` - add `ze_adjribin_nlri_key_errors_total{peer,family,operation}`
- `internal/component/bgp/plugins/bmp/bmp.go` - Answer 3 at line 684; Answer 2 at lines 725, 768, 881, 1030, 1048; wire `issuequeue`, bounded rate-limit maps, `isStopping()` guards
- `internal/component/bgp/plugins/bmp/bmp_events.go` - Answer 1 at line 462 (`abortAdjReplay` + report); Answer 2 at lines 507, 531, 564, 628; Answer 3 at line 557
- `internal/component/bgp/plugins/bmp/bmp_locrib.go` - Answer 2 at lines 855, 908, 926, 1023 (preserving `locRIBUpSent.Store(false)` and shutdown suppression)
- `internal/component/bgp/plugins/bmp/receiver.go` - Answer 3 at line 148
- `internal/component/bgp/plugins/bmp/sender.go` - Answer 3 at line 466; wire `reportWriteFailure` callback on `senderSession`
- `internal/component/bgp/plugins/bmp/sender_config.go` - prune sender rate-limit entries on collector removal
- `internal/component/bgp/plugins/bmp/sender_drain.go` - Answer 2 at line 135 (preserving `closeLog` + `clearConnAndResetIf`, skipping during stop)
- `internal/component/bgp/plugins/bmp/statistics.go` - Answer 2 at line 145
- `internal/component/bgp/plugins/bmp/register.go` - wire `ConfigureMetrics: SetMetricsRegistry`
- `internal/component/bgp/plugins/bmp/register_doctor_codes.go` - register `doctor-bmp-rib-sync-failed`, `doctor-bmp-receiver-session-error`, `doctor-bmp-sender-delivery-failed` via `diagnostic.Register` + `diagnostic.RegisterEventCode`
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - Answer 2 at line 556; wire `issuequeue`, `sync.WaitGroup` for `readmit`/`reader` shutdown ordering, bounded rate-limit map, atomic `invalidPeerCount`
- `internal/component/bgp/plugins/filter_irr/readmit.go` - Answer 2 at line 350 (`doctor-bgp-irr-readmit-confirm-failed`, deduplicated per peer)
- `internal/component/bgp/plugins/filter_irr/stored.go` - Answer 3 at line 103 (`guards readEvery`)
- `internal/component/bgp/plugins/filter_irr/doctor.go` - define `codeIRRInvalidPeer` and `codeIRRReadmitConfirmFailed` diagnostic descriptors
- `internal/component/bgp/plugins/filter_irr/register.go` - register `codeIRRInvalidPeer` and `codeIRRReadmitConfirmFailed` via `diagnostic.Register` + `diagnostic.RegisterEventCode`
- `internal/component/bgp/plugins/gr/gr.go` - Answer 2 at lines 368, 382, 433, 450, 692, 697, 714, 719 (`reportOpenDecodeFailure`); Answer 1 at line 1086 (`writeOut` returns error); wire `issuequeue`
- `internal/component/bgp/plugins/gr/gr_llgr_exchange.go` - Answer 2 at lines 105, 165, 177 (convert `addSentLLGRFamilies`/`addLLGRFamilies` to `*grPlugin` methods calling `reportOpenDecodeFailure`)
- `internal/component/bgp/plugins/gr/gr_llgr.go` - check `writeOut` error in `runCLIDecodeLLGR` and return exit code `1`
- `internal/component/bgp/plugins/gr/gr_removal.go` - prune GR rate-limit entries in `onPeerRemoved`
- `internal/component/bgp/plugins/gr/doctor.go` - define `codeDoctorBGPGRCapabilityDecode` diagnostic descriptor
- `internal/component/bgp/plugins/gr/register.go` - register `codeDoctorBGPGRCapabilityDecode` via `diagnostic.Register` + `diagnostic.RegisterEventCode`
- `internal/component/bgp/plugins/nlri/evpn/plugin.go` - Answer 1 at lines 404, 489, 500; Answer 3 at line 558
- `internal/component/bgp/plugins/nlri/flowspec/plugin_decode.go` - Answer 1 at lines 50, 60 (`decodeFlowSpecNLRI` returns error, `DecodeNLRIHex` wraps `errNoValidFlowspecDecoded`)
- `internal/component/bgp/plugins/nlri/flowspec/plugin.go` - print wrapped parse cause and return exit code `1` in `RunCLIDecode`
- `internal/component/bgp/plugins/nlri/flowspec/plugin_protocol.go` - handle `(map[string]any, error)` from `decodeFlowSpecNLRI`, keeping `decoded unknown` on error
- `internal/component/bgp/plugins/nlri/labeled/labeled.go` - Answer 1 at line 149
- `internal/component/bgp/plugins/nlri/ls/plugin.go` - Answer 1 at lines 189, 270, 287; Answer 3 at line 333
- `internal/component/bgp/plugins/nlri/mup/mup.go` - Answer 1 at line 174
- `internal/component/bgp/plugins/nlri/mvpn/mvpn.go` - Answer 1 at line 176
- `internal/component/bgp/plugins/nlri/rtc/rtc.go` - Answer 1 at line 150
- `internal/component/bgp/plugins/nlri/vpls/vpls.go` - Answer 1 at line 140
- `internal/component/bgp/plugins/nlri/vpn/vpn.go` - Answer 1 at lines 275, 361, 379; Answer 3 at line 422
- `internal/component/bgp/plugins/route_refresh/register.go` - Answer 3 at line 51
- `internal/component/bgp/plugins/route_refresh/route_refresh.go` - Answer 3 at lines 67, 187, 204; Answer 1 at lines 97, 194, 209
- `docs/guide/bmp.md` - document the new BMP sender/receiver error counters and `doctor-bmp-*` reporting in "Sender Behavior" / "Receiver Behavior"
- `docs/plugin-development/metrics.md` - add `bgp-bmp` -> `bmp` to "Scope-to-Prefix Mapping" and add `ze_adjribin_nlri_key_errors_total`, `ze_bmp_route_inject_errors_total`, and `ze_bmp_message_drops_total` to "Full Inventory"

## Files to Create

- `internal/component/bgp/plugins/adj_rib_in/register_doctor_codes.go` - register `doctor-bgp-adj-rib-in-nlri-key-failed` via `diagnostic.Register` + `diagnostic.RegisterEventCode`
- `internal/component/bgp/plugins/bmp/metrics.go` - BMP Prometheus counters (`ze_bmp_route_inject_errors_total`, `ze_bmp_message_drops_total`) following `docs/plugin-development/metrics.md`
- Unit test additions across the packages named in the TDD Test Plan above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC schema changes |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | No command added |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No command added |
| Functional test for new RPC/API | N-A | Reuses existing `report-issue` RPC and existing `--decode` runner |
| Pipe completeness | N-A | No command output shape changes |
| Env var registration | N-A | No env var added |
| Doctor check for runtime dependencies | N-A | No new external runtime dependency |
| Doctor code per problem class | Yes | 7 transient event codes registered with `diagnostic.Register` + `diagnostic.RegisterEventCode` (`adj_rib_in/register_doctor_codes.go`, `bmp/register_doctor_codes.go`, `filter_irr/{doctor,register}.go`, `gr/{doctor,register}.go`) and raised via `issuequeue` over `*sdk.Plugin` |
| Prometheus counters/metrics | Yes | `ze_adjribin_nlri_key_errors_total` (`adj_rib_in/metrics.go`), `ze_bmp_route_inject_errors_total` and `ze_bmp_message_drops_total` (`bmp/metrics.go`, wired via `bmp/register.go`) |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md` (document BMP error counters and collector/receiver doctor event reporting) |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | `bounceMonitoredPeers` aligns Peer Down failure with existing RFC 8671 §7.2 replay-abort handling |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (Scope-to-Prefix Mapping + Full Inventory) and `docs/guide/bmp.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-2-bgp-plugins.md`: `docs/guide/bmp.md` and `docs/plugin-development/metrics.md` are updated directly; the 8 other anchored pages (`docs/DESIGN.md`, `docs/features/plugins.md`, `docs/guide/configuration.md`, `docs/guide/irr-filtering.md`, `docs/guide/plugins.md`, `docs/guide/route-reflection.md`, `docs/guide/status.md`, `docs/plugin-overview.md`) reference unchanged registration/config/command symbols and need no edit unless a referenced symbol moves |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes
- Resolves all 63 `error-dropped-below-warn` sites across 24 files in `internal/component/bgp/plugins/`: 21 Answer-1 error returns / bug fixes, 30 Answer-2 rate-limited WARN + `issuequeue` doctor-event raises + Prometheus counters, and 12 Answer-3 visible non-enclosing guard annotations.
- Registers 7 transient doctor event codes (`doctor-bgp-adj-rib-in-nlri-key-failed`, `doctor-bmp-rib-sync-failed`, `doctor-bmp-receiver-session-error`, `doctor-bmp-sender-delivery-failed`, `doctor-bgp-irr-invalid-peer`, `doctor-bgp-irr-readmit-confirm-failed`, `doctor-bgp-gr-capability-decode`) via `diagnostic.Register` + `diagnostic.RegisterEventCode` and wires `internal/component/plugin/issuequeue` into `adj_rib_in`, `bmp`, `filter_irr`, and `gr` so internal and forked plugin executions report identically without importing `internal/core/report`.
- Adds 3 Prometheus CounterVecs (`ze_adjribin_nlri_key_errors_total`, `ze_bmp_route_inject_errors_total`, `ze_bmp_message_drops_total`) following `docs/plugin-development/metrics.md`.
- Fixes `bmp/bmp_events.go:462` (`bounceMonitoredPeers`) so a non-disconnected `writePeerDownLocked` failure aborts the Adj-RIB replay (`ss.abortAdjReplay(err)`) consistently with `writePeerUpLocked` and `replayPeerLocked` (RFC 8671 Section 7.2).

### Before and after
- **Before:** 63 error branches in BGP plugins log below WARN and proceed silently — malformed GR/LLGR capabilities vanish at Debug, BMP collector delivery failures and receiver RIB sync errors leave no WARN/doctor trace or counter, `bounceMonitoredPeers` continues into Peer Up after a failed Peer Down write, `flowspec.DecodeNLRIHex` discards the parse cause behind a generic error, and 17 decode stdout write sites ignore write errors and exit `0`.
- **After:** Zero `error-dropped-below-warn` findings under `internal/component/bgp/plugins/`; every actionable runtime failure logs one rate-limited WARN with a registered doctor code, increments its Prometheus counter where per-message/per-route, and surfaces in `ze show errors` and `ze show doctor` via `issuequeue`; every decode write failure returns exit code `1` to `bgp/cli/decode_plugin.go`; FlowSpec exposes its real parse cause while preserving `errNoValidFlowspecDecoded`; and all 12 deliberate teardown/contract guards carry verified `// refuse-by-default: guards <Symbol> -- <why>` annotations.

### Alternatives set aside
- **Calling `internal/core/report` directly from BGP plugins:** Rejected because every BGP plugin can run forked (`ai/doors/bgp-plugins.md`); process-local `report` calls in a forked plugin are invisible to the daemon's `show doctor` and `show errors`. Using `issuequeue` over `*sdk.Plugin` works identically internal and forked and matches `bfd`, `ospf`, `isis`, and `ldp`.
- **Changing EVPN/LS/VPN `parsed: false` decode output to return hard errors:** Rejected because `test/decode/bgp-evpn-1.ci`, `ls/unknown_type_skip_test.go` (RFC 9552 Section 5.1), and `vpn/addpath_decode_test.go` pin the `{"parsed": false, "raw": "<HEX>"}` JSON contract; those 3 sites are genuine Answer-3 guards.
- **Unthrottled WARN logging on per-UPDATE / per-route paths:** Rejected because a lost BMP collector or malformed peer stream would flood stderr and saturate the 256-entry `issuequeue`; bounded 1-minute per-key rate-limiting plus atomic Prometheus counters gives operators both immediate visibility and exact counts.
- **Generic variadic log wrapper helper (`warnWithCode(msg string, args ...any)`):** Rejected because `./le arch doctor-coverage` treats variadic `(msg string, args ...any)` helpers as log wrappers and would flag every caller lacking a literal `"code"` argument; dedicated typed domain helpers call `logger().Warn(..., "code", codeConst)` directly.

### Documents departed from
none

### Choices for Thomas
none

## Implementation Steps

1. **Phase: Wiring & Doctor/Metrics Foundations** -- re-run `./le arch refuse-by-default report` filtered to `internal/component/bgp/plugins/` to confirm all 63 sites; add `adj_rib_in/register_doctor_codes.go`, `bmp/metrics.go`, `bmp/register.go` (`ConfigureMetrics`), doctor event code registrations in `bmp/register_doctor_codes.go`, `filter_irr/{doctor,register}.go`, and `gr/{doctor,register}.go`; wire `issuequeue` and clean worker-join shutdown order in `adj_rib_in`, `bmp`, `filter_irr`, and `gr`.
2. **Phase: Decoders (Answer 1 & Answer 3)** -- write red unit tests then resolve all 19 `nlri/*` sites (`evpn`, `flowspec`, `labeled`, `ls`, `mup`, `mvpn`, `rtc`, `vpls`, `vpn`), `gr/gr.go:1086` + `gr/gr_llgr.go`, and all 7 `route_refresh` sites.
3. **Phase: GR, IRR, Adj-RIB-In (Answer 2 & Answer 3)** -- write red unit tests then resolve the 11 GR OPEN capability decode sites (with `onPeerRemoved` pruning), the 3 `filter_irr` sites (including `WaitGroup` worker join and `stored.go:103` annotation), and the 2 `adj_rib_in/rib_complex.go` sites.
4. **Phase: BMP (Answer 1, Answer 2, Answer 3)** -- write red unit tests then fix `bmp_events.go:462` (`bounceMonitoredPeers` -> `abortAdjReplay` + report), resolve the remaining 19 BMP sites with `isStopping()` suppression and bounded rate-limiting, and update `docs/guide/bmp.md` and `docs/plugin-development/metrics.md`.
5. **Phase: Prove** -- run scoped `go test -race` over all modified plugin packages, `./le arch refuse-by-default report` / `check`, `./le arch doctor-coverage check`, `./le plugin boundary check`, `./le spec citation anchors`, and affected `test/decode` and `test/plugin` `.ci` tests.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 63 HEAD sites has its exact Answer 1 / 2 / 3 verified at file:line |
| Correctness | No ack skipped, no cached UPDATE dropped, `locRIBUpSent.Store(false)` preserved on Loc-RIB Peer Up errors, `WaitGroup` joins `filter_irr` workers before `issues.Stop()` |
| Forked-plugin safety | Zero imports of `internal/core/report` in any BGP plugin; all Answer-2 raises flow through `issuequeue` over `*sdk.Plugin` |
| Rule: user-facing-errors | Each WARN names the peer or collector, the operation, why, the registered doctor `"code"`, `issuequeue.Outcome`, and the fix command |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero `error-dropped-below-warn` findings under `internal/component/bgp/plugins/` | `./le arch refuse-by-default report '\|' json` filtered by path and `./le arch refuse-by-default check` |
| Doctor code coverage and plugin boundaries clean | `./le arch doctor-coverage check` and `./le plugin boundary check` |
| Package tests (with race detector) | `./le job run label bgp-plugins command go test -race -tags ze_bgp,ze_bmp ./internal/component/bgp/plugins/adj_rib_in/... ./internal/component/bgp/plugins/bmp/... ./internal/component/bgp/plugins/filter_irr/... ./internal/component/bgp/plugins/gr/... ./internal/component/bgp/plugins/nlri/... ./internal/component/bgp/plugins/route_refresh/...` |
| Functional tests | `./le test bgp decode <name>` and `./le test bgp plugin <name>` for each `.ci` the change reaches |
| Doc source anchors checked | `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-2-bgp-plugins.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | No decode error or WARN line echoes unbounded peer bytes or untrusted strings into stderr logs or `report-issue` payloads |
| Memory DoS bounds | Every rate-limit map has a hard cap (256 or 1024 entries) and prunes entries on peer/collector teardown; `issuequeue` is bounded at 256 pending raises |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood -> RESEARCH |
| Lint or arch gate failure | Fix inline. If architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **Why `issuequeue` instead of `internal/core/report`:** Every BGP plugin can run forked (`ai/doors/bgp-plugins.md`). Raising through `issuequeue` over `*sdk.Plugin` sends `report-issue` RPCs to the daemon off a bounded non-blocking worker queue (256 cap, 1 s per-RPC timeout, 3 s drain cap on `Stop()`), matching `bfd`, `ospf`, `isis`, and `ldp` (`docs/architecture/doctor-and-health-checks.md`).
- **Why `filter_irr` needs a `sync.WaitGroup`:** `runFilterIRR` starts `plug.readmit.run(stop)` and `plug.reader.run(ctx)` without waiting for them before `p.Close()`. Adding a `WaitGroup` guarantees both workers stop raising before `issues.Stop()` drains and before `p.Close()` tears down the SDK transport.
- **Why `bounceMonitoredPeers` (`bmp_events.go:462`) is Answer 1:** `writePeerUpLocked` (`:466`) and `replayPeerLocked` (`:471`) both call `ss.abortAdjReplay(err)` on non-`errNotConnected` errors to uphold RFC 8671 Section 7.2, whereas `:462` (`writePeerDownLocked`) logged at Debug and did `continue`, leaving the collector with an inconsistent peer state.
- **Why `evpn:558`, `ls:333`, and `vpn:422` are Answer 3:** All three already emit structured `{"parsed": false, "raw": "<HEX>"}` output covered by existing decode/RFC tests; converting them to hard errors would violate RFC 9552 Section 5.1 and pinned decode tests.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Decode stdout write failures and FlowSpec parse errors return to the caller (Answer 1) | WARN and exit `0` or return empty result | `bgp/cli/decode_plugin.go` already propagates non-zero exit codes; an empty `0` answer cannot be distinguished from valid empty output |
| All 4 Answer-2 BGP plugins raise registered doctor event codes via `issuequeue` over `*sdk.Plugin` | Import `internal/core/report` directly, or log WARN without a doctor code | Forked BGP plugins cannot reach the daemon's process-local `report` bus, and `doctor-coverage` requires every new `Warn` under `internal/component/` to carry a registered doctor code |
| Per-message / per-route Answer-2 paths count in Prometheus (`ze_{scope}_{subject}_{event}_total`) and rate-limit WARN 1 min in bounded maps | Unthrottled WARN per message | Prevents log floods and `issuequeue` saturation while preserving exact error counts and bounded memory |
| `bmp/bmp_events.go:462` aborts Adj-RIB replay on non-disconnected Peer Down failure | Log WARN and `continue` | Matches `:466` and `:471` and prevents RFC 8671 Section 7.2 peer-state corruption on the collector |
| Preserve `parsed: false` JSON contract on `evpn:558`, `ls:333`, `vpn:422` (Answer 3) | Return error instead of `parsed: false` | Preserves pinned decode tests and RFC 9552 Section 5.1 unknown-NLRI handling |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions and Permitted Survivors and Why
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped package tests (`-race`), architecture gates (`refuse-by-default`, `doctor-coverage`, `plugin boundary`), and reached functional `.ci` tests pass
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire format change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-2-bgp-plugins.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- Resolved all 63 `error-dropped-below-warn` sites across 24 files in `internal/component/bgp/plugins/`: 21 Answer-1 error returns and behavior fixes, 30 Answer-2 rate-limited WARN + `issuequeue` doctor-event raises + Prometheus counters, and 12 Answer-3 verified `// refuse-by-default: guards <Symbol> -- <why>` annotations.
- Registered 7 transient doctor event codes with both `diagnostic.Register` and `diagnostic.RegisterEventCode` (`doctor-bgp-adj-rib-in-nlri-key-failed`, `doctor-bmp-rib-sync-failed`, `doctor-bmp-receiver-session-error`, `doctor-bmp-sender-delivery-failed`, `doctor-bgp-irr-invalid-peer`, `doctor-bgp-irr-readmit-confirm-failed`, `doctor-bgp-gr-capability-decode`) and wired `internal/component/plugin/issuequeue` over `*sdk.Plugin` into `adj_rib_in`, `bmp`, `filter_irr`, and `gr` with zero new `internal/core/report` imports and clean worker-join shutdown before `issues.Stop()` and `p.Close()`.
- Added 3 Prometheus CounterVecs (`ze_adjribin_nlri_key_errors_total`, `ze_bmp_route_inject_errors_total`, `ze_bmp_message_drops_total`) via `ConfigureMetrics` and `atomic.Pointer`.
- Fixed `bmp/bmp_events.go` (`BMPPlugin.bounceMonitoredPeers`) so a non-disconnected `writePeerDownLocked` failure calls `ss.abortAdjReplay(err)` and `bp.reportSenderWriteFailure` before breaking (RFC 8671 Section 7.2).
- Propagated stdout write errors as exit code `1` in EVPN, BGP-LS, VPN, labeled, MUP, MVPN, RTC, VPLS, GR/LLGR, and Route Refresh decode runners, and propagated the underlying parse cause from `flowspec.decodeFlowSpecNLRI` through `DecodeNLRIHex` (preserving `errNoValidFlowspecDecoded`) and `RunCLIDecode`.
- Updated `docs/guide/bmp.md` and `docs/plugin-development/metrics.md` with source anchors, updated `plan/release/fixes/spec-dropped-error-0-umbrella.md` (child-2 row closed with 0 remaining), and added `website/changes/fragments/dropped-error-2-bgp-plugins.md`.

### Bugs Found/Fixed
- **BMP handshake race on Stage 2 `report-issue` (`test/plugin/bmp-locrib.ci`):** `OnConfigure` starts BMP sender sessions and Loc-RIB feeds during Stage 2 before Stage 5 Ready completes, and the engine refuses non-stage RPCs before Ready. Added `BMPPlugin.awaitingHandshake` (`bmp/bmp.go`, `bmp/error_report.go`), cleared in `p.OnStarted` after Stage 5 Ready, so pre-Ready failures still count metrics and log WARN without sending `report-issue` over the handshake wire. Verified by `test/plugin/bmp-locrib.ci`.
- **Pre-existing `annotation-names-no-guard` in `gr/gr_egress.go`:** `egressFilterState.hasLLGR` carried a prose guard comment (`guards capability lookup`). Updated to `// refuse-by-default: guards LLGREgressFilter -- missing original owner fails closed as LLGR-incapable instead of borrowing another peer's capability`.
- **Run 1 Review Gate finding — raw `netip.ParseAddr` error echo in `filter_irr/filter_irr.go` (`reportInvalidPeer`):** `netip.ParseAddr(peerAddr)` includes the full raw `peerAddr` in `err.Error()`, bypassing `sanitizePeerLabel` and causing `rpc.ValidateReportIssue` (`ReportMessageMax = 1024`) to refuse reports for oversized peer strings. Added `parseAddrReason` and `fitReportText` in `filter_irr/filter_irr.go` (and `fitReportText` bounds in `filter_irr/readmit.go`, `adj_rib_in/rib_complex.go`, and `gr/gr.go`) plus regression test `TestFilterIRRInvalidPeerBoundsLongAndControlInput`.
- **Run 1 Review Gate finding — style pass type-suffix variable names:** Renamed `peerStr`/`famStr` to `peer`/`familyName` in `adj_rib_in/rib_complex.go`, `filter_irr/readmit.go`, and `gr/gr.go` per `docs/contributing/ze-go-style.md`.

### Documentation Updates
- `docs/guide/bmp.md` (Receiver Behavior and Sender Behavior): documented `ze_bmp_route_inject_errors_total`, `ze_bmp_message_drops_total`, `doctor-bmp-receiver-session-error`, `doctor-bmp-rib-sync-failed`, `doctor-bmp-sender-delivery-failed`, bounce Peer Down replay abort, and clean-shutdown suppression, with source anchors `<!-- source: internal/component/bgp/plugins/bmp/error_report.go -- reportReceiverRIBSyncFailure, reportReceiverSessionError -->`, `<!-- source: internal/component/bgp/plugins/bmp/error_report.go -- reportSenderWriteFailure -->`, `<!-- source: internal/component/bgp/plugins/bmp/bmp_events.go -- BMPPlugin.bounceMonitoredPeers -->`, and `<!-- source: internal/component/bgp/plugins/bmp/metrics.go -- SetMetricsRegistry, incRouteInjectErrorMetric -->` / `incMessageDropMetric`.
- `docs/plugin-development/metrics.md` (Scope-to-Prefix Mapping and Full Inventory): added `bgp-bmp -> bmp` and all 3 new CounterVec rows with source anchors to `adj_rib_in/metrics.go` and `bmp/metrics.go`.
- `website/changes/fragments/dropped-error-2-bgp-plugins.md`: created operator-facing news fragment (`./le doc ste check file` OK).
- Verification: `./le doc ste check` OK on `docs/guide/bmp.md`, `docs/plugin-development/metrics.md`, and `website/changes/fragments/dropped-error-2-bgp-plugins.md`; `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-2-bgp-plugins.md` exited 0 with no stale anchors; `./le doc check all` reported 0 issues on any file in this spec (the 1 unrelated pre-existing catalog drift in `internal/le/cli/catalog/catalog.go` is outside this spec).

### Deviations from Plan
- **Created `internal/component/bgp/plugins/bmp/error_report.go`:** Split BMP's error reporting (`reportReceiverRIBSyncFailure`, `reportReceiverSessionError`, `reportSenderWriteFailure`, `allowBoundedWarn`, `issueQueue`) out of `bmp/bmp.go` to keep `bmp.go` focused and concise.
- **Added `awaitingHandshake` / `OnStarted` gate in `bmp/bmp.go` + `bmp/error_report.go`:** Caught by `test/plugin/bmp-locrib.ci` because BMP starts sender loops during Stage 2 `OnConfigure`; gating `issueQueue()` until `p.OnStarted` prevents pre-Ready `report-issue` RPCs.
- **Modified `adj_rib_in/source_state.go` (instead of pruning inside `rib.go`):** `recordSourceStateLocked` and `endSourceOwner` live in `source_state.go`, so `pruneNLRIKeyWarnedPeerLocked` and `pruneNLRIKeyWarnedOwnerLocked` were wired there.
- **Modified `gr/gr_state.go` (`stopAll()`):** Added `grStateManager.stopAll()` to stop all active GR/LLST timers before `issues.Stop()` during shutdown.
- **Modified `gr/gr_egress.go`:** Repaired pre-existing `annotation-names-no-guard` on `egressFilterState.hasLLGR` so `internal/component/bgp/plugins/` has 0 `annotation-names-no-guard` findings.
- **Left `gr/register.go` unmodified:** `gr/register.go`'s `init()` already calls `registerGRDoctorCodes()` defined in `gr/doctor.go`, so adding `diagnostic.RegisterEventCode` inside `gr/doctor.go` registered the new event code cleanly without editing `gr/register.go`.
- **Added `refuse-by-default: guards processPeerUp` on two `zero-on-error` guards in `bmp/bmp.go` (`openMultiprotocolFamilies`):** Preserved RFC 4760 Section 8 empty-family return on unparseable OPENs cleanly under the guard rule.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `bmp` initially let `issuequeue.Start(p)` send `report-issue` RPCs while Stage 2 `OnConfigure` was still running | `OnConfigure` starts BMP sender loops and Loc-RIB feeds before Stage 5 Ready completes, and the engine refuses non-stage RPCs during the handshake | `test/plugin/bmp-locrib.ci` failed during handshake | Added `BMPPlugin.awaitingHandshake` cleared in `p.OnStarted` (`bmp/bmp.go`, `bmp/error_report.go`) |
| approach | `filter_irr/filter_irr.go` (`reportInvalidPeer`) passed raw `netip.ParseAddr` `err` into `msg` and `logger().Warn` after sanitizing `peerAddr` | `netip.ParseAddr` embeds the full raw input string in `err.Error()`, defeating `sanitizePeerLabel` and exceeding `rpc.ReportMessageMax` on long inputs | Independent Review Gate Run 1 | Added `parseAddrReason` and `fitReportText` across `filter_irr`, `adj_rib_in`, and `gr`, plus regression test `TestFilterIRRInvalidPeerBoundsLongAndControlInput` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| All 21 Answer-1 sites return errors or fix hidden behavior (`bmp_events.go:462`, `gr.go:1086`, `route_refresh.go:97,194,209`, `flowspec/plugin_decode.go:50,60`, 14 NLRI write sites) | Done | `bmp/bmp_events.go:462-470`, `gr/gr.go:1009-1035,1127-1220`, `gr/gr_llgr.go:371-415`, `route_refresh/route_refresh.go:95-235`, `nlri/flowspec/{plugin_decode,plugin,plugin_protocol}.go`, `nlri/{evpn,labeled,ls,mup,mvpn,rtc,vpls,vpn}/*.go` | All 21 verified against source and unit tests |
| All 30 Answer-2 sites log rate-limited WARN with registered doctor `"code"`, fix, `issuequeue.Outcome`, raise via `issuequeue` over `*sdk.Plugin` (zero new `internal/core/report` imports), count in Prometheus on hot paths | Done | `adj_rib_in/{register_doctor_codes,metrics,rib,rib_complex,source_state}.go`, `bmp/{register_doctor_codes,metrics,error_report,bmp,bmp_events,bmp_locrib,sender,sender_config,sender_drain,statistics,register}.go`, `filter_irr/{doctor,register,filter_irr,readmit}.go`, `gr/{doctor,gr,gr_llgr_exchange,gr_removal,gr_state}.go` | All 7 event codes registered via `diagnostic.Register` + `diagnostic.RegisterEventCode`; all 3 CounterVecs registered via `ConfigureMetrics` |
| All 12 Answer-3 sites annotated with visible non-enclosing guard symbols | Done | `bmp/{bmp,bmp_events,receiver,sender}.go`, `filter_irr/stored.go`, `nlri/{evpn,ls,vpn}/*.go`, `route_refresh/{register,route_refresh}.go` | Verified by `./le arch refuse-by-default report` (`annotated: 12`, `no-guard: 0`) |
| Clean worker-join shutdown ordering before `issues.Stop()` and `p.Close()` | Done | `adj_rib_in/rib.go:432-436`, `bmp/bmp.go:422-458`, `filter_irr/filter_irr.go:449-491`, `gr/gr.go:173-195` | LIFO defers stop workers and drain `issues.Stop()` before `p.Close()` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `./le arch refuse-by-default report '\|' json` filtered to `internal/component/bgp/plugins/`: `findings: 0`, `annotated: 12` | Exact 12 Answer-3 survivors match the 63-site table |
| AC-2 | Done | `./le arch doctor-coverage check` (0 findings in `internal/component/bgp/plugins/`) and `./le plugin boundary check` (`plugin-process-boundary: OK`) | All 7 event codes registered; zero new `internal/core/report` imports |
| AC-3 | Done | `TestBounceMonitoredPeersAbortsReplayOnPeerDownError`, `TestGRWriteOutReturnsErrorAndExitOne`, `TestFlowSpecDecodeSurfacesParseCause`, `TestEVPNDecodeReturnsExitOneOnWriteError`, `TestBGPLSDecodeReturnsExitOneOnWriteError`, `TestVPNDecodeReturnsExitOneOnWriteError`, `TestSimpleNLRIDecodeReturnsExitOneOnWriteError` (x5), `TestRouteRefreshDecodeReturnsExitOneOnWriteError` | `flowspec/plugin_protocol.go` keeps `decoded unknown` |
| AC-4 | Done | `TestComplexNLRIKeyErrorWarnsCountsAndRaisesDoctorCode`, `TestBMPSenderDeliveryFailureWarnsCountsAndRateLimits`, `TestBMPRateLimitMapsBoundedAndPruned`, `TestBMPDrainLoopWriteFailureReportsAndResets`, `TestBMPReceiverRIBSyncAndSessionErrorsRaiseDoctorCodes`, `TestFilterIRRInvalidPeerAndReadmitConfirmRaiseDoctorCodes`, `TestFilterIRRInvalidPeerBoundsLongAndControlInput`, `TestGROpenUndecodableCapabilityWarnsAndRaisesDoctorCode` | Rerun owed on 3 packages touched by Run 1 fixes (`adj_rib_in`, `filter_irr`, `gr`) |
| AC-5 | Done | `./le arch refuse-by-default report '\|' json`: 12 annotated `error-dropped-below-warn` sites, 0 `annotation-names-no-guard` | Every `<Symbol>` verified visible and non-enclosing |
| AC-6 | Done | `go test -race -tags ze_bgp,ze_bmp` across all 14 packages and 8 `.ci` functional tests (`test/decode/{bgp-evpn-1,bgp-open-graceful-restart,capability-llgr}.ci`, `test/plugin/{bmp-receiver-messages,bmp-sender-peer-up-open,bmp-sender-route-monitoring,bmp-sender-statistics,bmp-locrib}.ci`) | Scoped `-race` rerun owed on `adj_rib_in`, `filter_irr`, `gr` after Run 1 fixes |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestComplexNLRIKeyErrorWarnsCountsAndRaisesDoctorCode` | Done | `internal/component/bgp/plugins/adj_rib_in/rib_complex_test.go` | PASS (`-race`) |
| `TestBounceMonitoredPeersAbortsReplayOnPeerDownError` | Done | `internal/component/bgp/plugins/bmp/bmp_events_test.go` | PASS (`-race`) |
| `TestBMPSenderDeliveryFailureWarnsCountsAndRateLimits` | Done | `internal/component/bgp/plugins/bmp/bmp_events_test.go` | PASS (`-race`) |
| `TestBMPRateLimitMapsBoundedAndPruned` | Done | `internal/component/bgp/plugins/bmp/bmp_events_test.go` | PASS (`-race`, extra boundary coverage) |
| `TestBMPDrainLoopWriteFailureReportsAndResets` | Done | `internal/component/bgp/plugins/bmp/bmp_events_test.go` | PASS (`-race`, extra drain coverage) |
| `TestBMPReceiverRIBSyncAndSessionErrorsRaiseDoctorCodes` | Done | `internal/component/bgp/plugins/bmp/bmp_test.go` | PASS (`-race`) |
| `TestFilterIRRInvalidPeerAndReadmitConfirmRaiseDoctorCodes` | Done | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | PASS (`-race`) |
| `TestFilterIRRInvalidPeerBoundsLongAndControlInput` | Done | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | PASS (`-race`; proves `ParseAddr(` static reason, `(at ...)` stripping, C1/UTF-8 ASCII sanitization, `fitReportText` bounds) |
| `TestGROpenUndecodableCapabilityWarnsAndRaisesDoctorCode` | Done | `internal/component/bgp/plugins/gr/gr_test.go` | PASS (`-race`) |
| `TestGRWriteOutReturnsErrorAndExitOne` | Done | `internal/component/bgp/plugins/gr/gr_test.go` | PASS (`-race`) |
| `TestFlowSpecDecodeSurfacesParseCause` | Done | `internal/component/bgp/plugins/nlri/flowspec/plugin_test.go` | PASS |
| `TestEVPNDecodeReturnsExitOneOnWriteError` | Done | `internal/component/bgp/plugins/nlri/evpn/plugin_test.go` | PASS |
| `TestBGPLSDecodeReturnsExitOneOnWriteError` | Done | `internal/component/bgp/plugins/nlri/ls/plugin_test.go` | PASS |
| `TestVPNDecodeReturnsExitOneOnWriteError` | Done | `internal/component/bgp/plugins/nlri/vpn/vpn_test.go` | PASS |
| `TestSimpleNLRIDecodeReturnsExitOneOnWriteError` | Done | `internal/component/bgp/plugins/nlri/{labeled,mup,mvpn,rtc,vpls}/decode_test.go` | PASS across all 5 packages |
| `TestRouteRefreshDecodeReturnsExitOneOnWriteError` | Done | `internal/component/bgp/plugins/route_refresh/route_refresh_test.go` | PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/adj_rib_in/{metrics,rib,rib_complex,source_state}.go` | Done | `source_state.go` added for rate-limit pruning (recorded in Deviations) |
| `internal/component/bgp/plugins/adj_rib_in/{register_doctor_codes,rib_complex_test}.go` | Done | Created as planned |
| `internal/component/bgp/plugins/bmp/{bmp,bmp_events,bmp_locrib,bmp_test,receiver,register,register_doctor_codes,sender,sender_config,sender_drain,statistics}.go` | Done | Modified as planned |
| `internal/component/bgp/plugins/bmp/{metrics,error_report,bmp_events_test}.go` | Done | Created (`error_report.go` split recorded in Deviations) |
| `internal/component/bgp/plugins/filter_irr/{doctor,filter_irr,filter_irr_test,readmit,register,stored}.go` | Done | Modified as planned |
| `internal/component/bgp/plugins/gr/{doctor,gr,gr_egress,gr_llgr,gr_llgr_exchange,gr_removal,gr_state,gr_test}.go` | Done | `gr_egress.go` and `gr_state.go` added; `gr/register.go` unneeded (recorded in Deviations) |
| `internal/component/bgp/plugins/nlri/{evpn,flowspec,labeled,ls,mup,mvpn,rtc,vpls,vpn}/*` | Done | All 9 NLRI plugin packages modified/tested as planned (`vpn/vpn_test.go` created) |
| `internal/component/bgp/plugins/route_refresh/{register,route_refresh,route_refresh_test}.go` | Done | Modified as planned |
| `docs/guide/bmp.md`, `docs/plugin-development/metrics.md` | Done | Updated with source anchors and verified with STE check |

### Audit Summary
- **Total items:** 63 dropped-error sites, 6 ACs, 16 unit tests, 8 `.ci` functional tests, 58 implementation/test/doc files
- **Done:** All 63 sites, all 6 ACs, all 16 unit tests, all 8 `.ci` tests, all docs
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 6 file-layout / implementation refinements recorded in Deviations from Plan (`bmp/error_report.go` split, `awaitingHandshake` gate, `adj_rib_in/source_state.go`, `gr/gr_state.go`, `gr/gr_egress.go`, `gr/register.go` untouched)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Zero unannotated `error-dropped-below-warn` findings across `internal/component/bgp/plugins/` | Architecture check | `./le arch refuse-by-default report '\|' json` filtered to `internal/component/bgp/plugins/`: `findings: 0`, `annotated: 12` (0 `annotation-names-no-guard`) |
| Every Answer-1 site surfaces errors or fixes hidden behavior | Unit + functional tests | `TestBounceMonitoredPeersAbortsReplayOnPeerDownError`, `TestGRWriteOutReturnsErrorAndExitOne`, `TestFlowSpecDecodeSurfacesParseCause`, `TestEVPNDecodeReturnsExitOneOnWriteError`, `TestBGPLSDecodeReturnsExitOneOnWriteError`, `TestVPNDecodeReturnsExitOneOnWriteError`, `TestSimpleNLRIDecodeReturnsExitOneOnWriteError` (5 pkgs), `TestRouteRefreshDecodeReturnsExitOneOnWriteError`, `test/decode/{bgp-evpn-1,bgp-open-graceful-restart,capability-llgr}.ci` |
| Every Answer-2 site reports rate-limited WARN + registered doctor event code via `issuequeue` (fork-safe) + Prometheus counters on hot paths | Unit + functional + arch checks | `./le arch doctor-coverage check` (0 findings in `internal/component/bgp/plugins/`), `./le plugin boundary check` (`OK`), `TestComplexNLRIKeyErrorWarnsCountsAndRaisesDoctorCode`, `TestBMPSenderDeliveryFailureWarnsCountsAndRateLimits`, `TestBMPRateLimitMapsBoundedAndPruned`, `TestBMPDrainLoopWriteFailureReportsAndResets`, `TestBMPReceiverRIBSyncAndSessionErrorsRaiseDoctorCodes`, `TestFilterIRRInvalidPeerAndReadmitConfirmRaiseDoctorCodes`, `TestFilterIRRInvalidPeerBoundsLongAndControlInput`, `TestGROpenUndecodableCapabilityWarnsAndRaisesDoctorCode`, `test/plugin/{bmp-receiver-messages,bmp-sender-peer-up-open,bmp-sender-route-monitoring,bmp-sender-statistics,bmp-locrib}.ci` |
| Every Answer-3 site carries a valid visible non-enclosing guard annotation | Architecture check | `./le arch refuse-by-default report '\|' json` lists all 12 expected sites with non-enclosing guards (`BMPPlugin.handleSession`, `peerHeaderFromEvent`, `applyReceiverConfig`, `terminateAndClose`, `readEvery`, `DecodeNLRIHex`, `runBGPLSPlugin`, `RunCLIDecodeWithCode`, `sdk.NewWithConn`, `capNames`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All 63 `error-dropped-below-warn` sites under `internal/component/bgp/plugins/` and all ACs are complete | N/A |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/dropped-error-2-bgp-plugins-b5b20002-2222-4000-8000-000000000002.md` |
| `./le spec review check` | `clean` (59 files, verdict=clean, rounds=4) |
| Rounds | 4 completed (Run 1: 2 ISSUEs fixed; Run 2: 4 ISSUEs fixed; Run 3: 2 test-coverage ISSUEs caught `ParseAddr(` prefix bug and fixed; Run 4: CLEAN — 0 BLOCKER, 0 ISSUE) |
| Reviewer lenses used | logic+wiring, security+edge-cases+allocation-safety, style-pass (`ze-go-style.md`), design-documents, spec-completeness (`/ze-review-spec`), forked-plugin safety |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `reportInvalidPeer` embedded raw `netip.ParseAddr` `err` in `msg` and `logger().Warn`, echoing unbounded/unsanitized `peerAddr` past `sanitizePeerLabel` and causing `rpc.ValidateReportIssue` (`ReportMessageMax = 1024`) to refuse oversized peer inputs; `readmit.go`, `adj_rib_in/rib_complex.go`, and `gr/gr.go` also lacked `ReportMessageMax`/`ReportSubjectMax` truncation | `internal/component/bgp/plugins/filter_irr/filter_irr.go:595-618`, `filter_irr/readmit.go:543-568`, `adj_rib_in/rib_complex.go:146-181`, `gr/gr.go:1134-1162` | Added `parseAddrReason` and UTF-8-safe `fitReportText` bounding to `rpc.ReportMessageMax`/`rpc.ReportSubjectMax`, plus regression test `TestFilterIRRInvalidPeerBoundsLongAndControlInput` in `filter_irr/filter_irr_test.go` |
| 2 | ISSUE | Style-pass naming violation (`ze-go-style.md` "Names say what the value IS"): type-suffix variables `famStr` and `peerStr` | `internal/component/bgp/plugins/adj_rib_in/rib_complex.go:152-153`, `filter_irr/readmit.go:547`, `gr/gr.go:1138` | Renamed to `familyName` and `peer` |
| 3 | ISSUE | Round 2 caught `parseAddrReason` `LastIndex` matching inside `netip.ParseAddr`'s `(at ...)` suffix, `sanitizePeerLabel` allowing non-ASCII/C1 bytes and splitting UTF-8 at byte 64, and `fitReportText` panicking/splitting runes for `maxBytes <= 3` | `filter_irr/filter_irr.go`, `adj_rib_in/rib_complex.go`, `gr/gr.go`, `bmp/error_report.go`, `filter_irr/filter_irr_test.go` | Switched `parseAddrReason` to `strings.CutPrefix("ParseAddr(")` + `strconv.QuotedPrefix` + `(at ` stripping, enforced printable ASCII (`0x20..0x7e`) in `sanitizePeerLabel`, hardened all 4 `fitReportText` copies for `maxBytes <= 0` and rune boundaries, and expanded `TestFilterIRRInvalidPeerBoundsLongAndControlInput` |
| 4 | ISSUE | Round 3 caught Case 3 placing `\u009b` past byte 64 and Case 2 not asserting preservation of the real static reason (which exposed `ParseAddr(` vs `netip.ParseAddr(` prefix mismatch) | `filter_irr/filter_irr.go`, `filter_irr/filter_irr_test.go` | Moved `\u009b` inside first 64 bytes, accepted `ParseAddr(` first in `parseAddrReason`, and asserted `"(each group must have 4 or less digits)"`; verified CLEAN in Round 4 and `-race` PASS |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/adj_rib_in/register_doctor_codes.go` | Yes | `-rw-r--r-- 1 thomas thomas 2265 Nov 27 15:12` |
| `internal/component/bgp/plugins/adj_rib_in/rib_complex_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 5827 Nov 27 15:17` |
| `internal/component/bgp/plugins/bmp/metrics.go` | Yes | `-rw-r--r-- 1 thomas thomas 1829 Nov 27 15:13` |
| `internal/component/bgp/plugins/bmp/error_report.go` | Yes | `-rw-r--r-- 1 thomas thomas 7622 Nov 27 17:03` |
| `internal/component/bgp/plugins/bmp/bmp_events_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 13732 Nov 27 16:00` |
| `internal/component/bgp/plugins/nlri/vpn/vpn_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 1467 Nov 27 15:20` |
| `website/changes/fragments/dropped-error-2-bgp-plugins.md` | Yes | Created and passed `./le doc ste check file` (`ste check: OK`) |
| `test/decode/{bgp-evpn-1,bgp-open-graceful-restart,capability-llgr}.ci` and `test/plugin/{bmp-receiver-messages,bmp-sender-peer-up-open,bmp-sender-route-monitoring,bmp-sender-statistics,bmp-locrib}.ci` | Yes | Verified via `ls -la` (all 8 `.ci` files present) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 0 unannotated `error-dropped-below-warn` in `internal/component/bgp/plugins/` | `./le arch refuse-by-default report '\|' json` filtered to `internal/component/bgp/plugins/`: `findings: 0 annotated: 12` |
| AC-2 | Doctor coverage and plugin boundary clean | `./le arch doctor-coverage check`: `doctor-coverage bgp/plugins: 0 findings`; `./le plugin boundary check`: `plugin-process-boundary: OK` |
| AC-3 | All 21 Answer-1 sites surface errors / abort bounce replay | Verified in source and covered by unit tests across `bmp`, `gr`, `route_refresh`, and all 9 `nlri/*` packages |
| AC-4 | All 30 Answer-2 sites log rate-limited WARN + doctor code + `issuequeue` + Prometheus counters + shutdown suppression | Verified in source across `adj_rib_in`, `bmp`, `filter_irr`, `gr`; all `-race` tests PASS after final Round 4 fixes |
| AC-5 | All 12 Answer-3 guard annotations valid | `./le arch refuse-by-default report '\|' json` confirms `annotated: 12`, `no-guard: 0` |
| AC-6 | Scoped `-race` package tests and `.ci` functional tests pass | Confirmed green by main thread across all 14 packages (`-race`) and 8 `.ci` files after final Round 4 fixes |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze bgp decode --json --family l2vpn/evpn` (`parsed: false` preserved) | `test/decode/bgp-evpn-1.ci` | Yes (read file: feeds EVPN UPDATE with unparseable type-8 NLRI and expects `"parsed":false` + valid type-2 NLRIs; PASS) |
| `ze bgp decode --json --plugin bgp-gr --open` (GR code 64 and LLGR code 71) | `test/decode/bgp-open-graceful-restart.ci`, `test/decode/capability-llgr.ci` | Yes (read files: decode code 64 and code 71 OPEN capabilities through `bgp-gr`; PASS) |
| BMP receiver Initiation + Peer Up + Peer Down + session handling | `test/plugin/bmp-receiver-messages.ci` | Yes (read file: starts `ze` with BMP receiver on `$PORT2` and drives Initiation/Peer Up/Peer Down; PASS) |
| BMP sender Peer Up, Route Monitoring, Statistics, and RFC 9069 Loc-RIB (`awaitingHandshake` / `OnStarted` gate) | `test/plugin/bmp-sender-peer-up-open.ci`, `test/plugin/bmp-sender-route-monitoring.ci`, `test/plugin/bmp-sender-statistics.ci`, `test/plugin/bmp-locrib.ci` | Yes (read `bmp-locrib.ci`: starts collector + `ze` BMP sender with `loc-rib true`, proves Stage 2 `OnConfigure` -> Stage 5 `OnStarted` handoff and PeerType=3 monitoring; PASS) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `./le arch refuse-by-default report '\|' json` filtered to `internal/component/bgp/plugins/` returned 63 sites before edits and `findings: 0, annotated: 12` after edits |
| A-2 | confirmed | All 11 decoder unit test suites and `test/decode/{bgp-evpn-1,bgp-open-graceful-restart,capability-llgr}.ci` pass with exit code `1` on write errors and `parsed: false` preserved |
| A-3 | confirmed | `adj_rib_in`, `bmp`, `filter_irr`, and `gr` all wire `issuequeue.Start(p)` over `*sdk.Plugin`, join all workers/timers before `issues.Stop()` and `p.Close()`, and pass `./le plugin boundary check` (`OK`) and `-race` tests (with `bmp`'s Stage 2 handshake window guarded by `awaitingHandshake` -> `OnStarted`) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/bmp.md` (User guide — Receiver Behavior & Sender Behavior) | Source anchors point to `bmp/error_report.go` (`reportReceiverRIBSyncFailure`, `reportReceiverSessionError`, `reportSenderWriteFailure`), `bmp/bmp_events.go` (`BMPPlugin.bounceMonitoredPeers`), and `bmp/metrics.go` (`SetMetricsRegistry`, `incRouteInjectErrorMetric`, `incMessageDropMetric`); `./le doc ste check file docs/guide/bmp.md` OK | Yes |
| `docs/plugin-development/metrics.md` (Prometheus counters — Scope-to-Prefix Mapping & Full Inventory) | Source anchors point to `adj_rib_in/metrics.go` (`SetMetricsRegistry`, `incNLRIKeyErrorMetric`) and `bmp/metrics.go` (`SetMetricsRegistry`, `incRouteInjectErrorMetric`, `incMessageDropMetric`); `./le doc ste check file docs/plugin-development/metrics.md` OK | Yes |
| Other anchored docs (`./le spec citation anchors spec plan/release/fixes/spec-dropped-error-2-bgp-plugins.md`) | Exited 0 with 0 stale anchors; `git grep "spec-dropped-error-2-bgp-plugins"` shows only `spec-dropped-error-0-umbrella.md` (now updated to bare stem `spec-dropped-error-2-bgp-plugins`) | Yes |
| News fragment (`website/changes/fragments/dropped-error-2-bgp-plugins.md`) | Follows `website/changes/fragments/README.md` front matter (`spec`, `topic: BGP`, `found-by: review`) and exact 6 headings; `./le doc ste check file` OK | Yes |
