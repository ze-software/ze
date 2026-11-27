# Spec: dropped-error-4-system-plugins

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | 50 errors in the system plugins and the ExaBGP bridge (TFTP, image and DHCP servers, NTP, DNS doctor probe closes, container runtime/network, flow export, DDoS detection, VRRP telemetry, ExaBGP bridge and runner) are caught and logged only below WARN at HEAD, hiding operator-actionable failures such as NTP persistence/RTC/event-emit failures, DDoS baseline load/save and direction-lookup failures, DHCP reply write failures, TFTP transfer dial/file-read/resolve-fallback failures, flow-export destroy-listener/teardown failures, image-server zefs cleanup failures, and VRRP state-event emit failures |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/system-plugins.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/plugins/` and
`internal/exabgp/`, outside the routing-protocol plugins (`ospf`, `isis`,
`ldp`, `rsvpte`, child 6) and the forwarding plugins (`iface`, `fib`,
`firewall`, `traffic`, `policyroute`, child 5). Verified at HEAD on 2026-11-26:
**50 instances in 26 files** (TFTP 10 across 5 files, ExaBGP bridge and runner
9 across 5 files, container runtime and network 7 across 3 files, the three DNS
doctor probe-socket closes 6 across 3 files, image server 4 across 2 files, NTP
4 in 1 file, DHCP server 3 in 1 file, DDoS detect 3 across 2 files, flow export
3 across 3 files, VRRP 1 in 1 file). Three earlier 2026-11-02 sites are already
resolved at HEAD and retired into Existing Mechanisms (`bridge.go:512`,
`bridgerun/script.go` `script.reap`, `ntp/register.go` `runNTPPlugin`).

Each instance is resolved by exactly one answer, judged in the umbrella's order:

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid. Source audit at HEAD confirms zero remaining Answer-1 sites in this child's 50 findings (the three DNS `probeBindable` functions already return bind failures to `ze doctor`, NTP `doSync` already WARNs on unanswered/invalid servers and clock-set failures, and `script.reap` and `ntp/register.go` shutdown save already propagate errors) |
| 2 | Going on is right and an operator can act (15 sites, plus `tftpserver` `sendWindow` outer `readBlock` error = 15 registered event codes across 7 plugins) | Every site MUST BOTH: (a) log at `WARN` with what, why, exact fix (`ai/rules/user-facing-errors.md`), `"code", <code>` registered via `diagnostic.Register` and `diagnostic.RegisterEventCode` in the owning package's `register_doctor_codes.go` (`./le arch doctor-coverage check`), and `issuequeue.Outcome(qErr)`, AND (b) unconditionally enqueue a report-bus raise (`RaiseWarning` for persistent degraded conditions or `RaiseError` for discrete events, bounded as specified below) on the plugin's `internal/component/plugin/issuequeue` started over `*sdk.Plugin` in its `register.go` and stopped on exit after its workers stop and before `p.Close()`. No `RegisterEventCode` site is ever log-only. On per-packet/per-transfer paths (`dhcpserver`, `tftpserver`), count and rate-limit both the `WARN` and the raise |
| 3 | Going on is right and nobody can act (35 sites: teardown/probe socket closes, broken stdin pipes to exited ExaBGP scripts reaped by the runner, best-effort RFC 1350 TFTP ERROR packets, container teardown/unconfigured-record cleanup, DHCP RFC 2131 broadcast fallback documented at Info, appliance built-in `modprobe` probe, plus the `errors.Is(rerr, iface.ErrNoBackendLoaded)` branch of `tftpserver` `listenInterface`) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above naming a visible dependent/guard symbol (never the enclosing function) |

### Verified Per-Site Classification (HEAD 2026-11-26: 50 Findings in 26 Files)

| File | Line | Function | Failure | Answer | Resolution / Guard Symbol |
|------|------|----------|---------|--------|---------------------------|
| `internal/exabgp/bridge/bridge_ack.go` | 56 | `AckMode.AnswerLocal` | write `done` to script stdin | 3 | `guards Bridge.Run` (broken pipe means script exited; runner reaps exit) |
| `internal/exabgp/bridge/bridge_ack.go` | 113 | `AckMode.WriteAck` | write `done` to script stdin | 3 | `guards Bridge.Run` (same) |
| `internal/exabgp/bridge/bridge_ack.go` | 144 | `AckMode.WriteError` | write error reason + marker to script stdin | 3 | `guards Bridge.Run` (same) |
| `internal/exabgp/bridge/bridge_ack.go` | 156 | `AckMode.WriteErrorMarker` | write `error` marker to script stdin | 3 | `guards Bridge.Run` (same) |
| `internal/exabgp/bridge/bridge_batch.go` | 514 | `writeReply` | write local action reply to script stdin | 3 | `guards Bridge.Run` (same) |
| `internal/exabgp/bridge/bridge_ping.go` | 84 | `writePong` | write `pong` reply to script stdin | 3 | `guards Bridge.Run` (same) |
| `internal/exabgp/bridge/bridge_status.go` | 202 | `statusInfo.write` | write `status` reply to script stdin | 3 | `guards Bridge.Run` (same; `StatusReply` writes to `bytes.Buffer`) |
| `internal/plugins/exabgp/bridgerun/script.go` | 364 | `script.writeLoop` | write queued line to child stdin | 3 | `guards script.runOnce` (broken pipe means script exited; `runOnce` reads stdout to EOF, calls `script.reap`, and reports exit to `Fleet.notifyScriptExit`) |
| `internal/plugins/exabgp/bridgerun/script.go` | 373 | `script.writeLoop` | close child stdin at queue teardown | 3 | `guards script.closeQueue` (teardown close after queue closed) |
| `internal/plugins/as112/doctor.go` | 183 | `probeBindable` | `pc.Close()` after successful UDP bind probe | 3 | `guards as112ListenDiagnostic` (bind probe already succeeded; releasing throwaway socket) |
| `internal/plugins/as112/doctor.go` | 190 | `probeBindable` | `ln.Close()` after successful TCP bind probe | 3 | `guards as112ListenDiagnostic` (same) |
| `internal/plugins/dnsresolver/doctor.go` | 148 | `probeBindable` | `pc.Close()` after successful UDP bind probe | 3 | `guards listenerDiagnostics` (same) |
| `internal/plugins/dnsresolver/doctor.go` | 155 | `probeBindable` | `ln.Close()` after successful TCP bind probe | 3 | `guards listenerDiagnostics` (same) |
| `internal/plugins/geodns/doctor.go` | 191 | `probeBindable` | `pc.Close()` after successful UDP bind probe | 3 | `guards geodnsListenDiagnostic` (same) |
| `internal/plugins/geodns/doctor.go` | 198 | `probeBindable` | `ln.Close()` after successful TCP bind probe | 3 | `guards geodnsListenDiagnostic` (same) |
| `internal/plugins/container/container.go` | 90 | `runContainerEngine.func` | deferred `p.Close()` at plugin exit | 3 | `guards supervisor.close` (teardown close of plugin connection on exit) |
| `internal/plugins/container/network/attach.go` | 130 | `manager.ended` | `client.OwnedLinkRemove` at container end | 3 | `guards manager.created` (kernel removes veth pair with netns; `created` removes leftover records before next start; documented at Info in `container/network.md`) |
| `internal/plugins/container/supervise.go` | 802 | `supervisor.releaseTunablesWhenIdle` | `s.tunables.release` | 3 | `guards supervisor.createWithTunables` (`s.claimed` remains true; sysctl restores kernel value on daemon stop; documented in `container/runtime.md`) |
| `internal/plugins/container/supervise.go` | 918 | `supervisor.cleanup` | `s.recordProblem` after `created.destroy` WARN | 3 | `guards supervisor.recordProblem` (`cleanup` already logged WARN under `codeCleanupIncomplete`; `recordProblem` intentionally refuses containers removed from `s.desired`) |
| `internal/plugins/container/supervise.go` | 995 | `supervisor.stopTogether` | `created.signal` on init that already exited | 3 | `guards awaitEnd` (`awaitEnd` immediately observes container end or kills cgroup after timeout) |
| `internal/plugins/container/supervise.go` | 1011 | `supervisor.stopTogether` | `s.recordProblem` after stop-killed WARN | 3 | `guards supervisor.recordProblem` (`stopTogether` already logged WARN under `codeStopKilled`; `recordProblem` intentionally refuses unconfigured containers) |
| `internal/plugins/container/supervise.go` | 1030 | `supervisor.stopTogether` | `s.recordProblem` before returning `opererr.New` | 3 | `guards supervisor.recordProblem` (`stopTogether` already returns `opererr.New` in `failures`; `recordProblem` intentionally refuses unconfigured containers) |
| `internal/plugins/ddos/detect/detector.go` | 265 | `detector.saveBaselineContext` | `saveBaselines` failed (non-`StateUnavailable`) | 2 | `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-ddos-detect-baseline-save-failed` + `issuequeue.Outcome` |
| `internal/plugins/ddos/detect/detector.go` | 522 | `detector.classifyDirection` | `iface.AddressIsLocal` failed, defaulting to remote | 2 | `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-ddos-detect-direction-fallback` + `issuequeue.Outcome` |
| `internal/plugins/ddos/detect/persist.go` | 93 | `loadBaselines` | `client.StateGet` failed | 2 | Suppress `rpc.StateUnavailable` (matching `saveBaselines`); on real store errors, `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-ddos-detect-baseline-load-failed` + `issuequeue.Outcome` |
| `internal/plugins/dhcpserver/register.go` | 236 | `closeLogged` | `c.Close()` at teardown/rollback | 3 | `guards runDHCPServerPlugin` (teardown/rollback close) |
| `internal/plugins/dhcpserver/register.go` | 489 | `serveRequests` | `conn.WriteToUDP(resp, dst)` failed | 2 | Rate-limited via existing `warnLimited` (`unservedWarnInterval` = 1m) + counter increment: `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-dhcp-reply-write-failed` + `issuequeue.Outcome` |
| `internal/plugins/dhcpserver/register.go` | 583 | `replyDestination` | `neighborRead` failed; falls back to broadcast | 3 | `guards broadcastReply` (RFC 2131 Section 4.1 broadcast fallback when neighbor table is unreadable; `dhcp-server.md` specifies Info + broadcast) |
| `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` | 69 | `loadConntrackModules` | `modprobe` failed on appliance | 3 | `guards setupConntrackTracking` (modules are built-in on gokrazy; `enableConntrackAccounting`, `registerConntrackHook`, and `conntrack.NewReader` WARN if conntrack is actually unavailable) |
| `internal/plugins/flowexport/conntrack_worker.go` | 120 | `conntrackWorker.Start` | `conntrack.NewDestroyListener` failed; dump-only mode | 2 | `issuequeue.RaiseWarning` (cleared on stop/next start) + `WARN` once at startup with what/why/fix + `doctor-flowexport-conntrack-destroy-unavailable` + `issuequeue.Outcome` |
| `internal/plugins/flowexport/sampling_worker.go` | 141 | `samplingWorker.Stop` | `sampling.RemoveSampling` failed at teardown | 2 | Track interfaces where `SetupSampling` succeeded in `Start`; skip uninstalled/non-Linux interfaces, and on `RemoveSampling` failure (`ENOENT` already ignored), `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-flowexport-sampling-teardown-failed` + `issuequeue.Outcome` |
| `internal/plugins/imageserver/config_tx.go` | 177 | `prepareZefs` | `os.RemoveAll(dir)` after `buildZefsDB` failure | 2 | `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-imageserver-zefs-cleanup-failed` + `issuequeue.Outcome` |
| `internal/plugins/imageserver/config_tx.go` | 193 | `removeZefsDir` | `os.RemoveAll(dir)` on reload/stop | 2 | `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-imageserver-zefs-cleanup-failed` + `issuequeue.Outcome` |
| `internal/plugins/imageserver/config_tx.go` | 201 | `imageServer.closeServers` | `srv.Close()` on reload/stop | 3 | `guards imageServer.stop` (teardown/reload close of HTTP servers) |
| `internal/plugins/imageserver/register.go` | 314 | `closeLogged` | `c.Close()` at teardown/rollback | 3 | `guards runImageServerPlugin` (teardown/rollback close) |
| `internal/plugins/ntp/ntp.go` | 317 | `syncWorker.doSync` | `eventBus.Emit` (`clock-synced`) failed | 2 | `issuequeue.RaiseError` + `WARN` (once per worker via `CompareAndSwap`) with what/why/fix + `doctor-ntp-clock-synced-emit-failed` + `issuequeue.Outcome` |
| `internal/plugins/ntp/ntp.go` | 323 | `syncWorker.doSync` | `setRTC(clockTime)` failed (`ENOENT` already returns nil on Linux) | 2 | Skip non-Linux unsupported stub; on real RTC write failures, `issuequeue.RaiseWarning` (with `issuequeue.RunRaises` and `ClearWarning` on next successful `setRTC`) + `WARN` with what/why/fix + `doctor-ntp-rtc-write-failed` + `issuequeue.Outcome` |
| `internal/plugins/ntp/ntp.go` | 331 | `syncWorker.doSync` | `saveTime` failed (`nil` client / `StateUnavailable` already return nil) | 2 | `issuequeue.RaiseWarning` (with `issuequeue.RunRaises` and `ClearWarning` on next successful `saveTime`) + `WARN` with what/why/fix + `doctor-ntp-time-persist-failed` + `issuequeue.Outcome` |
| `internal/plugins/ntp/ntp.go` | 453 | `syncWorker.handleDHCPEvent` | `json.Unmarshal` of DHCP lease event failed | 2 | Rate-limited (1m) `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-ntp-dhcp-event-malformed` + `issuequeue.Outcome` |
| `internal/plugins/tftpserver/config_tx.go` | 200 | `listenInterface` | `bindDeviceFor(ifName)` failed | 2 / 3 | Inspect `errors.Is(rerr, iface.ErrNoBackendLoaded)` (never call `iface.GetBackend()`): when `ErrNoBackendLoaded` (install/provision path), keep `Debug` with Answer-3 `// refuse-by-default: guards bindTFTP -- ...` because `tftpListen(device)` immediately validates and `bindTFTP` reports any bind failure; for any other `rerr != nil`, `issuequeue.RaiseWarning` (cleared on next clean bind) + `WARN` with what/why/fix + `doctor-tftp-interface-resolve-fallback` + `issuequeue.Outcome` |
| `internal/plugins/tftpserver/handler.go` | 274 | `serveRequests` | send WRQ `"write not supported"` ERROR packet | 3 | `guards handleRRQ` (best-effort RFC 1350 ERROR packet to rejected client; never retransmitted) |
| `internal/plugins/tftpserver/handler.go` | 287 | `handleRRQ.func` | send RRQ validation/access ERROR packet | 3 | `guards serveRequests` (same) |
| `internal/plugins/tftpserver/handler.go` | 397 | `runTransfer` | `net.DialUDP("udp4", nil, t.clientAddr)` failed | 2 | Rate-limited (1m, never echoing unbounded filenames): `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-tftp-transfer-dial-failed` + `issuequeue.Outcome` |
| `internal/plugins/tftpserver/handler.go` | 452 | `sendOACKAndWait` | send file-not-found ERROR packet after `os.Stat` error | 3 | `guards runTransfer` (best-effort RFC 1350 ERROR packet; `sendOACKAndWait` returns false and ends transfer) |
| `internal/plugins/tftpserver/panic_guard.go` | 66 | `signalPanic` | send panic ERROR packet | 3 | `guards tftpGuard` (`taskguard` already logged ERROR, incremented `ze_task_panics_total`, and wrote crash record; RFC 1350 ERROR is best-effort) |
| `internal/plugins/tftpserver/register.go` | 92 | `closeLogged` | `c.Close()` at teardown/reload | 3 | `guards runTFTPServerPlugin` (teardown/reload close) |
| `internal/plugins/tftpserver/window_send.go` | 33 | `serveFile` | send file-not-found ERROR packet after `os.Open` error | 3 | `guards runTransfer` (best-effort RFC 1350 ERROR packet; transfer ends) |
| `internal/plugins/tftpserver/window_send.go` | 39 | `serveFile.func` | deferred `f.Close()` on read-only file | 3 | `guards runTransfer` (closing read-only file after transfer completes) |
| `internal/plugins/tftpserver/window_send.go` | 128 | `windowSender.sendWindow` | send `"read error"` ERROR packet after `readBlock` failure | 3 | `guards serveFile` for `wErr` (best-effort RFC 1350 ERROR packet); ALSO log and raise the outer server-side `readBlock` I/O `err` via rate-limited (1m) `issuequeue.RaiseError` + `WARN` with `doctor-tftp-file-read-failed` + `issuequeue.Outcome` |
| `internal/plugins/vrrp/telemetry.go` | 140 | `publishStateChange` | `stateChangeEvent.Emit` failed | 2 | Rate-limited (1m per group): `issuequeue.RaiseError` + `WARN` with what/why/fix + `doctor-vrrp-state-event-emit-failed` + `issuequeue.Outcome` |

### Report-Bus Event Codes, Severity, Bounds, and `issuequeue` Lifecycle

Every Answer-2 plugin starts one `internal/component/plugin/issuequeue.Queue` via `issuequeue.Start(p)` in its engine entry point (`register.go`) and calls `q.Stop()` on exit AFTER its workers/servers stop and BEFORE `p.Close()`, so all queued raises and final clears drain over the SDK transport on both internal (`DirectBridge`) and forked (TLS/RPC) placements. Every queued call supplies a non-nil `failed` callback that logs daemon refusals at `WARN` under the same doctor `code`.

| Event Code | Owning Package (`register_doctor_codes.go` + `register.go`) | Report-Bus Severity | Subject | Rate / Once / Clear Bound |
|------------|-------------------------------------------------------------|---------------------|---------|---------------------------|
| `doctor-ddos-detect-baseline-save-failed` | `internal/plugins/ddos/detect` (`runEngine`) | `RaiseError` | `"baseline"` | At most once per periodic save interval (`baselineSaveInterval`, ~5m) or stop save |
| `doctor-ddos-detect-baseline-load-failed` | `internal/plugins/ddos/detect` (`runEngine`) | `RaiseError` | `"baseline"` | Once per startup or reload restore (`rpc.StateUnavailable` suppressed) |
| `doctor-ddos-detect-direction-fallback` | `internal/plugins/ddos/detect` (`runEngine`) | `RaiseError` | victim prefix | Once per attack characterization (fires only on confirmed attack) |
| `doctor-dhcp-reply-write-failed` | `internal/plugins/dhcpserver` (`runDHCPServerPlugin`) | `RaiseError` | listen interface | Rate-limited via `warnLimited` (`unservedWarnInterval` = 1m) per listener |
| `doctor-flowexport-conntrack-destroy-unavailable` | `internal/plugins/flowexport` (`runEngine`) | `RaiseWarning` | `"conntrack-destroy"` | Raised once per `conntrackWorker.Start` when `NewDestroyListener` fails; cleared via `ClearWarning` when a subsequent `Start` opens the listener or `Stop` tears the worker down |
| `doctor-flowexport-sampling-teardown-failed` | `internal/plugins/flowexport` (`runEngine`) | `RaiseError` | interface name | Once per installed interface at `samplingWorker.Stop` (skipped if `SetupSampling` never succeeded) |
| `doctor-imageserver-zefs-cleanup-failed` | `internal/plugins/imageserver` (`runImageServerPlugin`) | `RaiseError` | temp directory path | Once per failed `os.RemoveAll` in `prepareZefs` or `removeZefsDir` |
| `doctor-ntp-clock-synced-emit-failed` | `internal/plugins/ntp` (`runNTPPlugin`) | `RaiseError` | `"clock-synced"` | At most once per `syncWorker` lifetime (`CompareAndSwap`) |
| `doctor-ntp-rtc-write-failed` | `internal/plugins/ntp` (`runNTPPlugin`) | `RaiseWarning` | `"/dev/rtc0"` | Once per failing run via `issuequeue.RunRaises` (retry window `IntervalSec`, min 1m); cleared via `ClearWarning` on next successful `setRTC` (non-Linux stub and `ENOENT` skipped) |
| `doctor-ntp-time-persist-failed` | `internal/plugins/ntp` (`runNTPPlugin`) | `RaiseWarning` | `"last-time"` | Once per failing run via `issuequeue.RunRaises` (retry window `IntervalSec`, min 1m); cleared via `ClearWarning` on next successful `saveTime` (`nil` client / `StateUnavailable` skipped) |
| `doctor-ntp-dhcp-event-malformed` | `internal/plugins/ntp` (`runNTPPlugin`) | `RaiseError` | `"dhcp-lease-event"` | Rate-limited to at most once per minute |
| `doctor-tftp-interface-resolve-fallback` | `internal/plugins/tftpserver` (`runTFTPServerPlugin`) | `RaiseWarning` | logical interface name | Raised once per config apply when `bindDeviceFor` fails with a non-`ErrNoBackendLoaded` error; cleared via `ClearWarning` on a subsequent apply where the interface resolves or is removed |
| `doctor-tftp-transfer-dial-failed` | `internal/plugins/tftpserver` (`runTFTPServerPlugin`) | `RaiseError` | `"transfer-dial"` | Rate-limited to at most once per minute per server |
| `doctor-tftp-file-read-failed` | `internal/plugins/tftpserver` (`runTFTPServerPlugin`) | `RaiseError` | validated file path | Rate-limited to at most once per minute per server |
| `doctor-vrrp-state-event-emit-failed` | `internal/plugins/vrrp` (`runVRRPEngine`) | `RaiseError` | `"<interface>/<group>/<vrid>"` | Rate-limited to at most once per minute per VRRP group |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - shared contracts across every subsystem
  → Constraint: every operator-visible message names what went wrong, why it matters, and the exact fix; plugins may run forked so logs go through the plugin logger, report-bus issues go through `internal/component/plugin/issuequeue` over `*sdk.Plugin`, and plugins never add unguarded `dangerousCalls` (`iface.GetBackend`)
- [ ] `ai/doors/system-plugins.md` - boundaries for system plugins and ExaBGP bridge
  → Constraint: every plugin here can run forked; a WARN goes to the plugin's logger and report-bus raises go through `issuequeue` over `*sdk.Plugin`, never directly to `internal/core/report`
  → Constraint: the ExaBGP bridge's contract is ExaBGP's text and JSON; an error made visible to logs changes nothing the script reads
  → Constraint: DHCP, TFTP, and PXE clients are boot firmware Ze cannot change; no wire change here
- [ ] `docs/architecture/exabgp-bridge.md` - the endpoint and standalone runners, ack modes, script lifecycle
  → Constraint: an ack or reply write failure on script stdin means the script's pipe is gone; `Bridge.Run` / `script.runOnce` reaps the script exit
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Every problem site names a doctor code" and "Event codes"
  → Constraint: every new `Warn` or `Error` log line under `internal/plugins/` MUST carry a `code` attribute backed by `diagnostic.Register` and `diagnostic.RegisterEventCode` in the owning package's `register_doctor_codes.go` (`./le arch doctor-coverage check`), AND every site carrying a `RegisterEventCode` code MUST raise it on the report bus via `issuequeue` over `*sdk.Plugin` (never log-only)
- [ ] `ai/rules/user-facing-errors.md` - operator-facing error and log message standard
  → Constraint: what, why, fix, and `code` attribute on every `Warn`/`Error`; never log at `Warn` when nothing is wrong
- [ ] `docs/contributing/ze-go-style.md` - "A zero value is never an answer" and "Every error is handled"
  → Constraint: return, count, or WARN with doctor code + raise; the guard annotation names a visible non-enclosing dependent symbol

**Key insights:**
- All six DNS doctor findings (`as112`, `dnsresolver`, `geodns` `probeBindable`) are post-bind `pc.Close()` / `ln.Close()` calls; bind failures are already returned and reported by `ze doctor`.
- All four NTP findings in `ntp.go` are post-sync side effects (`eventBus.Emit`, `setRTC`, `saveTime`) and `handleDHCPEvent` unmarshaling; NTP query and clock-set failures already log at `WARN`.
- Three 2026-11-02 findings (`bridge.go:512`, `bridgerun/script.go` `script.reap`, `ntp/register.go` `runNTPPlugin`) are already fixed at HEAD, while eight new findings appeared in `container` (7) and `ddos/detect/persist.go` (1), giving the verified HEAD population of 50 findings in 26 files.
- `dhcpserver` `replyDestination` (`neighborRead` error) and `container/network` `manager.ended` (`OwnedLinkRemove` error) both have explicit architecture-doc contracts requiring `Info` logging plus fallback/next-start cleanup, so both take Answer 3 rather than changing log level.
- `iface.GetBackend` is on `./le plugin boundary check`'s `dangerousCalls` list (`internal/le/plugin/boundary/pluginboundary.go:108`), so `tftpserver` distinguishes no-backend fallback by checking `errors.Is(rerr, iface.ErrNoBackendLoaded)` on `bindDeviceFor`'s returned error instead of calling `iface.GetBackend()`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - `droppedError`, `caughtError`, `belowWarn`, `drops`
- [ ] `internal/le/arch/refusebydefault/annotation.go` - `// refuse-by-default: guards <Symbol> -- <why>`, `visibleNames`, `namesEnclosing`
- [ ] `internal/component/plugin/issuequeue/queue.go`, `raises.go` - `Start`, `RaiseWarning`, `RaiseError`, `ClearWarning`, `Stop`, `Flush`, `Outcome`, `RunRaises`
- [ ] `internal/le/plugin/boundary/pluginboundary.go` - `dangerousCalls` list confirming `iface.GetBackend` is forbidden in unguarded plugins
- [ ] `internal/component/iface/dispatch.go`, `resolve.go` - `iface.ErrNoBackendLoaded` sentinel returned/wrapped by `iface.Resolve` when no backend is loaded
- [ ] `internal/exabgp/bridge/bridge.go` - confirmed line 512 finding is gone; `Bridge.Run` owns child process lifecycle
- [ ] `internal/exabgp/bridge/bridge_ack.go`, `bridge_batch.go`, `bridge_ping.go`, `bridge_status.go` - 7 stdin write errors logged at `slog.Debug`
- [ ] `internal/plugins/exabgp/bridgerun/script.go` - `script.reap` already returns `exitErr` to `script.runOnce`; `script.writeLoop` logs stdin write and teardown close errors at Debug
- [ ] `internal/plugins/as112/doctor.go`, `internal/plugins/dnsresolver/doctor.go`, `internal/plugins/geodns/doctor.go` - `probeBindable` already returns bind failures; only `pc.Close()` and `ln.Close()` log at Debug
- [ ] `internal/plugins/container/container.go`, `internal/plugins/container/network/attach.go`, `internal/plugins/container/supervise.go` - 7 teardown, fallback-cleanup, and unconfigured-`recordProblem` branches log at Debug/Info
- [ ] `internal/plugins/ddos/detect/detector.go`, `persist.go`, `register.go`, `register_doctor_codes.go` - `saveBaselineContext`, `classifyDirection`, and `loadBaselines` log errors at Debug; `runEngine` owns `*sdk.Plugin`
- [ ] `internal/plugins/dhcpserver/register.go`, `register_doctor_codes.go`, `neighbor_permanent_test.go` - `closeLogged` and `serveRequests` reply write log at Debug; `replyDestination` logs `neighborRead` failure at Info and falls back to `broadcastReply()`; `runDHCPServerPlugin` owns `*sdk.Plugin`
- [ ] `internal/plugins/flowexport/conntrack_setup_appliance_linux.go`, `conntrack_worker.go`, `sampling_worker.go`, `sampling/tc_linux.go`, `sampling/tc_other.go`, `register.go`, `register_doctor_codes.go` - appliance `modprobe` and `RemoveSampling` teardown log at Debug; `NewDestroyListener` failure logs at Info; `runEngine` owns `*sdk.Plugin`
- [ ] `internal/plugins/imageserver/config_tx.go`, `register.go` - `prepareZefs`, `removeZefsDir`, `closeServers`, and `closeLogged` log at Debug; `runImageServerPlugin` owns `*sdk.Plugin`
- [ ] `internal/plugins/ntp/ntp.go`, `register.go`, `clock_linux.go`, `persist.go`, `register_doctor_codes.go` - `register.go` shutdown save already returns `*opererr.Error` and `runNTPPlugin` owns `*sdk.Plugin`; `ntp.go` logs `clock-synced` emit, `setRTC`, `saveTime`, and `handleDHCPEvent` unmarshal at Debug
- [ ] `internal/plugins/tftpserver/config_tx.go`, `handler.go`, `panic_guard.go`, `register.go`, `window_send.go` - `listenInterface`, `runTransfer` `net.DialUDP`, best-effort ERROR replies, `closeLogged`, and `f.Close` log at Debug; `windowSender.sendWindow` drops the outer `readBlock` error without warning; `runTFTPServerPlugin` owns `*sdk.Plugin`
- [ ] `internal/plugins/vrrp/telemetry.go`, `doctor.go`, `register.go` - `publishStateChange` logs `stateChangeEvent.Emit` failure at Debug; `runVRRPEngine` owns `*sdk.Plugin`

**Behavior to preserve:**
- TFTP, DHCP, NTP, DNS, VRRP, flow-export, container, and image-server wire and protocol behavior; ExaBGP script stdin/stdout framing and ack semantics; existing `ze doctor` bind-capability checks; `bindDeviceFor` verbatim fallback when `errors.Is(rerr, iface.ErrNoBackendLoaded)`; `dhcpserver` RFC 2131 broadcast fallback on unreadable neighbor table.

**Behavior to change:**
- Resolve all 50 HEAD `error-dropped-below-warn` instances across 26 files: wire `internal/component/plugin/issuequeue` over `*sdk.Plugin` in all 7 Answer-2 plugins' `register.go` files; elevate the 15 operator-actionable sites (plus `tftpserver` `sendWindow` outer `readBlock` error) to `WARN` with what/why/fix, registered doctor `code`, `issuequeue.Outcome`, and mandatory report-bus raise (`RaiseWarning`/`RaiseError` with explicit rate/once/clear bounds); annotate the 35 guard-protected teardown/best-effort sites (plus the `ErrNoBackendLoaded` branch of `tftpserver` `listenInterface`) with `// refuse-by-default: guards <Symbol> -- <why>`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists the 50 unannotated instances under this child's paths at HEAD | Applies unchanged: defines the population and AC-1 check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` `parseGuard`, `namesEnclosing` | Accepts `// refuse-by-default: guards <Symbol> -- <why>` naming a visible non-enclosing symbol | Applies unchanged: resolves the 35 Answer-3 sites and the `ErrNoBackendLoaded` branch of `tftpserver` `listenInterface` |
| Doctor code & event-code registries | `internal/core/diagnostic` `Register`, `RegisterEventCode`; `internal/plugins/{ddos/detect,dhcpserver,flowexport,ntp,exabgp/bridgerun}/register_doctor_codes.go`; `./le arch doctor-coverage check` | Backs every `Warn`/`Error` log line under `internal/plugins/` with a documented doctor code declared by a check or `recent-events` | Extends: adds event-code entries in existing `register_doctor_codes.go` files (`ddos/detect`, `dhcpserver`, `flowexport`, `ntp`) and creates `register_doctor_codes.go` in `imageserver`, `tftpserver`, and `vrrp` for all 15 Answer-2 event codes |
| SDK plugin report-bus queue | `internal/component/plugin/issuequeue/queue.go` (`Start`, `RaiseWarning`, `RaiseError`, `ClearWarning`, `Stop`, `Flush`, `Outcome`) and `raises.go` (`RunRaises`) | Carries a plugin's raises and clears to the daemon report bus via `sdk.Plugin.RaiseIssue`/`ClearIssue` on both internal and forked transports without blocking hot/locked goroutines (used by BFD, OSPF, IS-IS, LDP) | Extends: wired in all 7 Answer-2 plugins' `register.go` (`ddos/detect`, `dhcpserver`, `flowexport`, `imageserver`, `ntp`, `tftpserver`, `vrrp`) over `*sdk.Plugin` and stopped after workers exit, so every `RegisterEventCode` site raises on the report bus |
| Interface no-backend sentinel | `internal/component/iface/dispatch.go` `ErrNoBackendLoaded`, returned by `iface.Resolve` | Signals that no `iface` backend is loaded without calling `iface.GetBackend()` (which is on `./le plugin boundary check`'s `dangerousCalls` list) | Applies unchanged: `tftpserver` inspects `errors.Is(rerr, iface.ErrNoBackendLoaded)` on `bindDeviceFor`'s returned error to keep the install/provision fallback quiet while warning and raising on real resolution errors |
| DNS doctor bind-capability checks | `probeBindable` in `internal/plugins/{as112,dnsresolver,geodns}/doctor.go` | Already returns bind failure (`false` or `error`) to `ze doctor` (`doctor-as112-port-unavailable`, `doctor-dns-resolver-port-unavailable`, `doctor-geodns-port-unavailable`) | Does not change bind logic: only the post-bind `Close()` calls are dropped-error sites, resolved by Answer 3 |
| NTP sync and shutdown error reporting | `syncWorker.doSync` in `internal/plugins/ntp/ntp.go` (lines 238-308); `saveFinalTime` / `finishNTPWorkerContext` in `internal/plugins/ntp/register.go` | `doSync` already logs server query/validation/clock-set failures at `WARN`; `register.go` already returns `*opererr.Error` on shutdown save failure (retiring the 2026-11-02 `runNTPPlugin` line 285 finding) | Extends: wires `issuequeue` in `ntp/register.go` and raises/warns the four remaining Debug drops in `ntp.go` (`clock-synced` emit, `setRTC`, `saveTime`, `handleDHCPEvent`) |
| ExaBGP bridge & script lifecycle | `Bridge.Run` (`internal/exabgp/bridge/bridge.go`); `script.runOnce` / `script.reap` (`internal/plugins/exabgp/bridgerun/script.go`) | Reaps child script exit and notifies `Fleet.notifyScriptExit` (retiring the 2026-11-02 `bridge.go:512` and `script.reap:452` findings) | Applies unchanged: serves as the named guard for the 9 Answer-3 stdin write/close annotations |
| Rate-limited warning helper | `warnLimited` (`internal/plugins/dhcpserver/register.go:445`) | Limits per-packet DHCP warnings to at most once per `unservedWarnInterval` (1 minute) | Extends: reuses `warnLimited` for `serveRequests` reply write failure so misbehaving clients cannot flood logs or the report bus |
| Plugin metrics | `docs/plugin-development/metrics.md`, `internal/core/metrics` | Per-plugin Prometheus counters | Extends: counts per-packet/per-transfer Answer-2 failures (`dhcpserver` reply write failure) alongside rate-limited `WARN` and raise |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "A plain Go call into another component's package is the classic defect." | agrees | All 7 Answer-2 plugins send report-bus raises through `internal/component/plugin/issuequeue` over `*sdk.Plugin` (which works identically internal and forked), `tftpserver` inspects `errors.Is(rerr, iface.ErrNoBackendLoaded)` with zero calls to `iface.GetBackend()`, and every operator-visible `WARN` states what, why, and fix |
| `ai/doors/system-plugins.md` | Who depends on it from outside | the door covering all 30 modified files and 3 new `register_doctor_codes.go` files | "Any wire change. The client is boot firmware, which Ze cannot change" | agrees | Zero DHCP, TFTP, HTTP, NTP, DNS, VRRP, flow-export, or ExaBGP wire bytes change; only log levels, doctor codes, `issuequeue` raises, counters, and guard annotations change |
| `docs/architecture/core-design.md` | 15. Operational Report Bus | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Warnings are state-based (condition is currently problematic, may resolve). Errors are event-based (something already happened; no clear API). Producers pick deliberately; the bus does not auto-promote." | agrees | Every Answer-2 event code declares its exact severity (`RaiseWarning` with clear-on-recovery for lasting degraded states: conntrack dump-only, NTP RTC/time persistence, TFTP resolve fallback; `RaiseError` for discrete failures) and rate/once bound through `issuequeue` over `*sdk.Plugin` |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec changes no config validation, `ze:backend` annotation, or kernel/VPP backend selection rule |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec does not alter start, commit, reload, or validate verdicts |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches neither firewall YANG nor the VPP firewall verifier |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces no data-plane selector, backend registration name, or `ze:backend` value |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches no BGP export filter or next-hop rewriting path |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec touches no policy chain or BGP next-hop configuration |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec touches no BGP policy chain or `FilterInsertion` extension |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec touches no BGP export chain or raw UPDATE override path |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec touches no BGP policy chain or next-hop validation |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec touches no route origination or export next-hop behavior; ExaBGP bridge annotations only document broken-pipe stdin writes |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec touches no filter-chain member or `\| implied` seam |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | No Answer-2 or Answer-3 change in this spec turns a runtime plugin error into a `ze start` refusal; every newly surfaced problem logs at `WARN` and raises on the report bus under its registered doctor code while keeping Ze running |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/exabgp/bridge/bridge_ack.go` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec does not alter daemon startup, dataplane mismatch handling, commit, reload, or `ze config validate` |
| `docs/architecture/exabgp-bridge.md` | Script exit, respawn, and route ownership | anchored to `bridge_ack.go`, `bridge_batch.go`, `bridge_ping.go`, `bridge_status.go`, `bridgerun/script.go` | "The translator mirrors the script it runs: when the script exits, the translator exits with the script's outcome." | agrees | Stdin write/close failures in `bridge_*` and `bridgerun/script.go` occur when the script closed stdin or exited, which `Bridge.Run` and `script.runOnce` reap and mirror; all 9 sites take Answer 3 naming that guard |
| `docs/architecture/exabgp-bridge.md` | Design decisions | anchored to `bridge_ack.go`, `bridge_batch.go`, `bridge_ping.go`, `bridge_status.go`, `bridgerun/script.go` | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | This spec adds no config root and no internal bridge runner; it only annotates existing stdin write/close error branches in the wrapper and standalone bridge |
| `docs/architecture/exabgp-bridge.md` | Design decisions | anchored to `bridge_ack.go`, `bridge_batch.go`, `bridge_ping.go`, `bridge_status.go`, `bridgerun/script.go` | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <peer> <local> <router-id> <extra-addpath> <n>:<b64-body>`, zero key names, `-` for absent metadata tokens, `0:` for an empty body) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | This spec leaves the `base64` wire grammar, `MuxConn` batching, and hot-path UPDATE renderer completely untouched |
| `docs/architecture/config/environment.md` | ExaBGP Compatibility Wrapper | anchored to `internal/exabgp/bridge/bridge_ack.go` | "Wrapper emits `done`/`error` lines on script stdin after each dispatched command" | agrees | `AckMode` continues emitting the exact same `done`/`error` lines; Answer-3 annotations document only broken-pipe write errors |
| `docs/architecture/dns/as112.md` | Address family is a parameter, not a constant | the `// Design:` page of and anchored to `internal/plugins/as112/doctor.go` | "" | silent | `as112.md` specifies zone responses, listener anchors, and family-aware probing, and is silent on closing the throwaway probe sockets in `probeBindable`; bindability reporting is unchanged |
| `docs/architecture/container/runtime.md` | Runtime checks | the `// Design:` page of `container/container.go`, `container/supervise.go` | "A record is refused for a container the configuration does not hold, and a container that leaves the configuration takes its records with it." | agrees | The three `recordProblem` calls in `supervise.go` already log WARN or return an operator error before recording and intentionally ignore unconfigured-container refusals; `releaseTunablesWhenIdle`, `stopTogether` signal, and `p.Close` teardown all keep their documented behavior under Answer 3 |
| `docs/architecture/container/image-store.md` | The fetch | anchored to `internal/plugins/container/container.go` | "" | silent | Anchored to `container.go` only for `commandDecls` and `handleCommand`; silent on the deferred `p.Close()` connection close in `runContainerEngine` |
| `docs/architecture/container/network.md` | The participant | the `// Design:` page of and anchored to `internal/plugins/container/network/attach.go` | "A refused removal is logged at info: the kernel removes a pair with its namespace and the next start removes the record first." | agrees | `manager.ended` keeps its `Info` log and takes Answer 3 naming `manager.created`, matching this page's documented behavior |
| `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` | Decisions | the `// Design:` page of `ddos/detect/detector.go` and anchored to `detector.go`, `register.go` | "" | silent | The umbrella page covers EventBus broadcast, two-stage trigger order, leak probe, and the rate-feed doctor check, and is silent on baseline-save/`AddressIsLocal` error log levels and `issuequeue` wiring |
| `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md` | The two locks, and why the second one exists | anchored to `internal/plugins/ddos/detect/detector.go` | "Lock order is `emitMu` then `d.mu`. Never take `emitMu` while holding `d.mu`." | agrees | Logging `saveBaselineContext` and `classifyDirection` failures at `WARN` and enqueuing non-blocking `issuequeue` raises touches neither `emitMu` nor `d.mu` ordering |
| `docs/architecture/ddos/ddos-detect-enhancements.md` | Baseline persistence | the `// Design:` page of `ddos/detect/persist.go` and anchored to `detector.go`, `persist.go`, `register.go` | "The format is versioned JSON under `meta/ddos/detect-baseline` in the daemon's selected `database/` tree, through the plugin SDK state RPCs (`p.StateGet` / `p.StatePut`, granted via `statestore.RegisterPluginKeys` in `register.go`)." | agrees | Format, key, `byeSaveBudget`, and storeless (`rpc.StateUnavailable`) no-op semantics stay unchanged while real store load/save errors raise via `issuequeue` and log at `WARN` with doctor codes |
| `docs/architecture/traffic/traffic-analysis-layers.md` | The shared confirm-and-clear machine | anchored to `internal/plugins/ddos/detect/detector.go` | "" | silent | Anchored only for `stats.Hysteresis` wiring in `newDetector` and `applyTick`; silent on baseline persistence and direction-lookup error logging |
| `docs/architecture/traffic/traffic-usage-monitor.md` | Consequences | anchored to `internal/plugins/ddos/detect/register.go` | "" | silent | Anchored to `ddos/detect/register.go` only for `trafficstat.EnsureGlobal` / `SubscribeRates`; silent on `issuequeue` wiring in `runEngine` |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | the `// Design:` page of `imageserver/config_tx.go`, `tftpserver/config_tx.go` and anchored to `ddos/detect/detector.go`, `ddos/detect/register.go`, `ntp/register.go` | "" | silent | Defines stream-based verify/apply/rollback budgets and `ErrRunningConfigLost` handling, and is silent on temp-dir cleanup, interface-resolution fallback logging, and `issuequeue` startup/stop ordering; no transaction behavior changes |
| `docs/architecture/provisioning/dhcp-server.md` | Decisions | the `// Design:` page of and anchored to `internal/plugins/dhcpserver/register.go` | "A table that cannot be read is broadcast too, because the write could replace an entry the server could not see." | agrees | `replyDestination` keeps its RFC 2131 broadcast fallback and `Info` log under Answer 3 (`guards broadcastReply`), matching this page's documented contract; `serveRequests` reply write failure raises via `issuequeue` and logs a rate-limited `WARN` with doctor code |
| `docs/architecture/iface/logical-name-resolution.md` | The plugin-facing registries take the resolved device too | anchored to `dhcpserver/register.go`, `vrrp/register.go` | "A failed resolution refuses the bind and never falls back to the logical name, and a reload names the `os-name` or `mac match` selector in its fix." | agrees | `dhcpserver` `listenInterface` and `vrrp` device resolution are untouched; `tftpserver` `listenInterface` inspects `errors.Is(rerr, iface.ErrNoBackendLoaded)` (never calling `iface.GetBackend()`), keeping the no-backend provisioning fallback under Answer 3 while raising and warning on any real resolution failure |
| `docs/architecture/plugin/plugin-system.md` | The process boundary | anchored to `dhcpserver/register.go`, `imageserver/register.go`, `tftpserver/register.go` | "Plugin code reaches the engine through the SDK's RPC layer, which hides the transport difference." | agrees | All 7 Answer-2 plugins raise report-bus events through `internal/component/plugin/issuequeue` over `*sdk.Plugin` (working identically internal and forked), and `tftpserver` inspects `errors.Is(rerr, iface.ErrNoBackendLoaded)` without calling `iface.GetBackend()`, keeping `./le plugin boundary check` green |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | anchored to `dhcpserver/register.go`, `imageserver/register.go`, `tftpserver/register.go` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | This spec changes no plugin declaration, query-mode path, or offline config parse |
| `docs/architecture/dns/lan-resolver.md` | Doctor | the `// Design:` page of and anchored to `internal/plugins/dnsresolver/doctor.go` | "Both probes then bind with `IP_FREEBIND` too (`dnsserver.ListenConfig(true)`), as the resolver does, so an absent address binds while a port another socket holds still fails with `EADDRINUSE` and a refused permission still fails." | agrees | `probeBindable` keeps its exact `IP_FREEBIND` bind-probe return contract; only the post-probe `pc.Close()` and `ln.Close()` calls carry Answer-3 annotations |
| `ai/digests/flow-ddos.md` | Invariants & gotchas | the `// Design:` page of `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` | "On the appliance ze now sets it up itself: `flowexport/conntrack_setup_appliance_linux.go` (`//go:build linux && ze_appliance`, called from `conntrackWorker.Start`) loads nf_conntrack/nf_conntrack_netlink, enables `nf_conntrack_acct`, and installs a minimal `ct`-referencing nftables rule via `google/nftables` (no nft binary on gokrazy) to register the tracking hooks netns-wide; off the appliance it is a no-op (`conntrack_setup_other.go`)." | agrees | Appliance `modprobe` stays best-effort under Answer 3 (`guards setupConntrackTracking`) because modules are built-in on gokrazy and the downstream accounting/hook/reader steps already WARN on real absence |
| `docs/architecture/flowexport/flow-export-0-umbrella.md` | In-process component with the SDK protocol | anchored to `internal/plugins/flowexport/register.go` | "It still speaks the SDK config protocol over an in-memory connection, so its config surface is identical to an out-of-process plugin." | agrees | Wiring `issuequeue.Start(p)` over `flowexport`'s `*sdk.Plugin` uses the SDK `report-issue` protocol over that connection rather than a process-local bus |
| `docs/architecture/flowexport/flow-export-1-counter-export.md` | IPFIX counter records use the Total IEs, not the Delta IEs | anchored to `internal/plugins/flowexport/register.go` | "" | silent | Anchored to `flowexport/register.go` only for `interfaceCountersFrom`; silent on `issuequeue` wiring in `runEngine` |
| `docs/architecture/flowexport/flow-export-2-flow-records.md` | Export limits and counter sources | the `// Design:` page of `conntrack_worker.go`, `sampling_worker.go` and anchored to `conntrack_worker.go`, `sampling_worker.go`, `register.go` | "If the listener cannot start, the worker logs the error and uses periodic dumps." | agrees | Raising `NewDestroyListener` failure via `issuequeue.RaiseWarning` (cleared on recovery/stop) and logging at `WARN` with doctor code makes dump-only degradation visible at the default log level and in `show warnings`/`show doctor` while preserving periodic-dump operation |
| `docs/architecture/dns/geodns.md` | Observability | the `// Design:` page of and anchored to `internal/plugins/geodns/doctor.go` | "It warns only when an enabled geodns has a privileged listener port below 1024 that fails a bind probe." | agrees | `probeBindable` keeps its exact bind-probe return contract; only post-probe `Close()` calls carry Answer-3 annotations |
| `docs/architecture/provisioning/image-server.md` | Decisions | the `// Design:` page of `imageserver/register.go` and anchored to `imageserver/config_tx.go`, `imageserver/register.go` | "It is written to a temporary directory that is removed on stop and on reconfigure." | agrees | Raising `os.RemoveAll` failures in `prepareZefs` and `removeZefsDir` via `issuequeue.RaiseError` and logging at `WARN` with doctor code makes leaked temporary zefs directories visible to operators while preserving server close behavior |
| `docs/features/interfaces.md` | Capability Table | the `// Design:` page of `internal/plugins/ntp/ntp.go` | "NTP servers from DHCP (option 42)" | agrees | Raising `handleDHCPEvent` JSON unmarshal failure and post-sync `clock-synced`/RTC/persistence drops via `issuequeue` and logging at `WARN` with doctor codes makes broken option-42 delivery and persistence failures visible without altering DHCP or NTP wire behavior |
| `docs/features/interfaces.md` | Design decisions | the `// Design:` page of `internal/plugins/ntp/ntp.go` | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | This spec does not touch `ze init` or interface discovery |
| `docs/features/interfaces.md` | Design decisions | the `// Design:` page of `internal/plugins/ntp/ntp.go` | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | This spec does not touch `ze init --confirm` |
| `docs/architecture/provisioning/tftp-server.md` | Panic containment | the `// Design:` page of and anchored to `tftpserver/handler.go`, `panic_guard.go`, `register.go`, `window_send.go`, `config_tx.go` | "RFC 1350 section 2 says most errors end the connection and are signaled by an error packet that is never acknowledged or retransmitted, so a lost ERROR costs the client its timeout." | agrees | All best-effort RFC 1350 ERROR-packet send failures take Answer 3 with guard annotations, while server-side `net.DialUDP`, `readBlock`, and non-`ErrNoBackendLoaded` `listenInterface` failures raise via `issuequeue` and log at `WARN` with doctor codes |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `internal/plugins/tftpserver/window_send.go` | "" | silent | Anchored to `window_send.go` only for `windowSender.awaitACK` duplicate-ACK interop; silent on error-response and file-close logging |
| `docs/architecture/testing/interop.md` | Design decisions | anchored to `internal/plugins/tftpserver/window_send.go` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | This spec touches no Docker interop suite or kernel capability declaration |
| `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` | Layers | the `// Design:` page of and anchored to `internal/plugins/vrrp/telemetry.go`, `internal/plugins/vrrp/register.go` | "registration, YANG augments, extract-only config walk with a cross-leaf verifier, instance manager, show and clear, engine-owned state telemetry, events, `doctor-vrrp-config-invalid`" | agrees | Wiring `issuequeue` in `vrrp/register.go`, raising `publishStateChange` emit failures via `issuequeue.RaiseError` + `WARN`, and registering `doctor-vrrp-state-event-emit-failed` in `internal/plugins/vrrp/register_doctor_codes.go` keeps all VRRP telemetry and doctor codes owned inside `internal/plugins/vrrp` |
| `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` | The address owner needs a filter, not a sysctl | anchored to `internal/plugins/vrrp/register.go` | "" | silent | Anchored to `vrrp/register.go` only for `vipMaskBits`; silent on `issuequeue` wiring in `runVRRPEngine` |
| `docs/architecture/doctor-and-health-checks.md` | Event codes | the `// Design:` page of `register_doctor_codes.go` files in `ddos/detect`, `dhcpserver`, `flowexport`, `ntp`, `imageserver`, `tftpserver`, `vrrp` | "Registering a code whose sites only log, and never raise, would declare a row this check never shows: every site carrying a registered event code raises it." | agrees | Every new `Warn` site added by this spec carries a `code` attribute backed by `diagnostic.Register` + `diagnostic.RegisterEventCode` in its owning package's `register_doctor_codes.go`, AND unconditionally raises that code on the daemon report bus via `internal/component/plugin/issuequeue` over `*sdk.Plugin` (with explicit severity, bounds, and `issuequeue.Outcome` on the `WARN` line), satisfying both `./le arch doctor-coverage check` and `recent-events` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call inside a system plugin or the ExaBGP bridge: a socket write or dial, a store read/write, a hardware RTC write, an EventBus emit, a DHCP lease event unmarshal, a conntrack destroy listener open, a tc sampling teardown, a temp-directory removal, a file read, a logical interface resolve, or a teardown/probe socket close.

### Transformation Path
1. The branch catches the error.
2. For all 15 Answer-2 sites (plus `tftpserver` `windowSender.sendWindow` outer `readBlock` error), the plugin:
   - Enqueues its report-bus raise (`RaiseWarning` or `RaiseError` per the Event-Code Severity & Rate-Bound Table, using `issuequeue.RunRaises` / `ClearWarning` for persistent conditions or bounded rate-limiting on per-packet/per-transfer/event paths) onto the plugin's `internal/component/plugin/issuequeue.Queue` started over `*sdk.Plugin` in `register.go`, with a non-nil `failed` callback logging daemon refusals at `WARN` under the same doctor `code`.
   - Logs at `WARN` with what, why, exact fix, `"code", <code>`, and `issuequeue.Outcome(qErr)`, plus a counter increment on `dhcpserver` reply write failure.
3. For the 35 Answer-3 guard-protected teardown, probe-close, broken-pipe, best-effort RFC 1350 / RFC 2131 fallback sites, and the `errors.Is(rerr, iface.ErrNoBackendLoaded)` branch of `tftpserver` `listenInterface`, the branch keeps its existing quiet log and carries `// refuse-by-default: guards <Symbol> -- <why>` naming the visible guard.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ daemon report bus | `internal/component/plugin/issuequeue` → `sdk.Plugin.RaiseIssue` / `ClearIssue` (`report-issue` RPC, identical internal and forked) | Yes |
| Plugin ↔ operator | Plugin log at `WARN` with registered doctor `code` + `issuequeue.Outcome`, `ze show doctor` (`recent-events`), `ze show warnings` / `ze show errors`, `ze show diagnostic <code>`, metrics | Yes |
| Bridge ↔ ExaBGP script | stdin and stdout of the script (bytes unchanged) | Yes |

### Changed-Wire Census

**Search evidence:** Inspected all 50 `error-dropped-below-warn` sites across all 26 files from `./le arch refuse-by-default report '|' json` at HEAD on 2026-11-26 (`internal/exabgp/bridge/*`, `internal/plugins/{as112,container,ddos/detect,dhcpserver,dnsresolver,exabgp/bridgerun,flowexport,geodns,imageserver,ntp,tftpserver,vrrp}`), plus the 7 Answer-2 plugins' `register.go` entry points. No wire format changes: this spec modifies only Go log levels, doctor-code attributes, existing SDK `report-issue` calls through `internal/component/plugin/issuequeue`, internal counters, and `// refuse-by-default:` source comments; zero DHCP, TFTP, HTTP, NTP, DNS, VRRP, flow-export, plugin-RPC schemas, or ExaBGP script wire bytes are altered.

### Integration Points
- Existing plugin loggers (`slogutil.Logger`), `diagnostic.Register` + `diagnostic.RegisterEventCode` in each owning package's `register_doctor_codes.go`, `internal/component/plugin/issuequeue` started over `*sdk.Plugin` in all 7 Answer-2 plugins' `register.go` (`ddos/detect`, `dhcpserver`, `flowexport`, `imageserver`, `ntp`, `tftpserver`, `vrrp`), `iface.ErrNoBackendLoaded` via `errors.Is` in `tftpserver`, `dhcpserver` `warnLimited`, and the Prometheus metrics registry.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Uses each plugin's existing logger, `register_doctor_codes.go`, `warnLimited`, and `internal/component/plugin/issuequeue` over `*sdk.Plugin` (never `internal/core/report` directly) |
| No unintended coupling (components stay isolated) | Yes | `internal/component/plugin/issuequeue` is shared plugin infrastructure already used by BFD/OSPF/IS-IS/LDP, and `internal/core/diagnostic` is core leaf tier (`./le arch tier check`); `tftpserver` uses `errors.Is(rerr, iface.ErrNoBackendLoaded)` with zero `iface.GetBackend()` calls (`./le plugin boundary check`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `internal/component/plugin/issuequeue`, `warnLimited` in `dhcpserver`, `iface.ErrNoBackendLoaded`, and the `diagnostic.Register` / `RegisterEventCode` pattern from `bridgerun/register_doctor_codes.go` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-packet and per-transfer paths (`dhcpserver` `serveRequests`, `tftpserver` `runTransfer` / `sendWindow`) rate-limit warnings and raises, enqueue non-blocking on `issuequeue`, and touch no hot-path buffer allocation |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | New doctor codes and event codes register from each owning plugin's own `register_doctor_codes.go` `init()`; removing a plugin removes its doctor codes |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | `./le arch doctor-coverage` and `recent-events` derive covered codes dynamically from `diagnostic.EventCode` registrations; no central table is edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 50 findings in 26 files are the exact current population under this child's paths | `./le arch refuse-by-default report '|' json` run on 2026-11-26 | Re-run report before first edit and reconcile any delta | Filtered report run before Phase 1 and at AC-1 | confirmed 2026-11-26 |
| A-2 | A failed stdin write in `internal/exabgp/bridge` or `bridgerun/script.go` means the script closed stdin or exited and `Bridge.Run` / `script.runOnce` reaps it | Source read of `bridge.go:551`, `bridge_ack.go`, `bridgerun/script.go:202-377`, and `docs/architecture/exabgp-bridge.md` | A live script losing acks would need Answer 2 | Confirmed against source (`script.runOnce` calls `script.reap` and `Fleet.notifyScriptExit`; `queueWriter` already counts queue drops in `s.dropped` and WARNs) | confirmed 2026-11-26 |
| A-3 | Every new `Warn` site under `internal/plugins/` must carry a doctor code declared by `diagnostic.Register` + `diagnostic.RegisterEventCode` AND raise that code via `issuequeue` over `*sdk.Plugin` | `ai/rules/user-facing-errors.md`, `docs/architecture/doctor-and-health-checks.md` ("Every problem site names a doctor code", "Event codes"), `internal/component/plugin/issuequeue/queue.go` | Commit gate refuses the commit or `recent-events` declares an unraised code | `./le arch doctor-coverage check` + `issuequeue.Publisher` unit tests | confirmed 2026-11-26 |
| A-4 | `iface.Resolve(ifName)` returns/wraps `iface.ErrNoBackendLoaded` when no `iface` backend is loaded, avoiding `iface.GetBackend()` | `internal/component/iface/dispatch.go:23`, `resolve.go:196-210`, `no_backend_error_test.go:30-35`, and `internal/le/plugin/boundary/pluginboundary.go:108` | `./le plugin boundary check` would refuse `tftpserver` | `./le plugin boundary check` + `TestTFTPServerSideErrorsAreWarnedWithDoctorCode` | confirmed 2026-11-26 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A `WARN` or report-bus raise per DHCP packet or TFTP transfer floods logs or the report bus under a misbehaving client | `test/dhcp` or TFTP test logs grow per packet or `issuequeue` hits `ErrFull` | Rate-limit with `warnLimited` (`dhcpserver`) and a 1-minute window (`tftpserver`, `vrrp`, `ntp` DHCP event) before logging and before enqueuing on `issuequeue` |
| R-2 | Warning/raising on `setRTC` off Linux, `RemoveSampling` for never-installed interfaces, `loadBaselines` with `rpc.StateUnavailable`, or `tftpserver` `listenInterface` with `iface.ErrNoBackendLoaded` creates false warnings on healthy hosts or in unit tests | Unit tests on darwin or without `iface` backend emit unexpected `WARN` lines or `issuequeue` raises | Skip unsupported non-Linux `setRTC` stub, track installed sampling interfaces in `samplingWorker`, suppress `rpc.StateUnavailable` in `loadBaselines`, and inspect `errors.Is(rerr, iface.ErrNoBackendLoaded)` in `tftpserver` `listenInterface` |
| R-3 | `issuequeue.Stop()` called before workers/servers stop drops final clears or returns `ErrStopped` | Race detector or shutdown unit test sees `ErrStopped` | In all 7 `register.go` files, stop workers/servers first, then call `q.Stop()` before `p.Close()` |
| R-4 | A `WARN` log or report-bus message echoes an unbounded client-controlled TFTP filename or DHCP payload or SSH password hash | Security review or unit test with oversized input | Log/raise only validated paths, client addresses, or structured error causes, never raw untrusted client filenames, payloads, or credentials |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise or spurious `show warnings`/`show errors` rows on expected fallback paths, a `./le arch doctor-coverage check` refusal if a new `Warn` omits a registered doctor code, or a `./le plugin boundary check` refusal if a `dangerousCalls` function is added |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Specs on TFTP, NTP, container runtime/network, and ExaBGP bridge (`spec-dropped-error-0-umbrella.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's 26 files (`internal/plugins/` outside routing/forwarding, `internal/exabgp/`) | `TestRefuseByDefaultReportListsTree` + filtered `./le arch refuse-by-default report '|' json` asserting zero unannotated `error-dropped-below-warn` findings and valid non-enclosing guards on all annotated sites |
| `./le arch doctor-coverage check` & `./le plugin boundary check` | → | Every new `Warn` site, its `register_doctor_codes.go` registration, and `tftpserver` `errors.Is(rerr, iface.ErrNoBackendLoaded)` | `TestDoctorCoverageSourceHoldsEveryRegisteredCode`, `./le arch doctor-coverage check`, and `./le plugin boundary check` clean |
| NTP post-sync persistence / RTC / clock-synced emit failure and malformed DHCP lease event | → | `runNTPPlugin` (`issuequeue.Start(p)`), `syncWorker.doSync` (`saveTime`, `setRTC`, `eventBus.Emit`), and `syncWorker.handleDHCPEvent` (`internal/plugins/ntp/{register,ntp}.go`) | `TestNTPPostSyncAndDHCPEventErrorsAreWarnedAndRaised` |
| DDoS baseline load/save store failure and `iface.AddressIsLocal` failure | → | `runEngine` (`issuequeue.Start(p)`), `detector.saveBaselineContext`, `detector.classifyDirection`, `loadBaselines` (`internal/plugins/ddos/detect/{register,detector,persist}.go`) | `TestDDoSDetectPersistenceAndDirectionErrorsAreWarnedAndRaised` |
| DHCP reply UDP write failure on a serving listener | → | `runDHCPServerPlugin` (`issuequeue.Start(p)`), `serveRequests` (`internal/plugins/dhcpserver/register.go`) | `TestDHCPServeRequestsReplyWriteFailureIsWarnedRaisedAndRateLimited` |
| Flow-export conntrack destroy listener unavailable and tc sampling teardown failure | → | `runEngine` (`issuequeue.Start(p)`), `conntrackWorker.Start`, `samplingWorker.Stop` (`internal/plugins/flowexport/{register,conntrack_worker,sampling_worker}.go`) | `TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise` |
| TFTP transfer dial failure, server-side `readBlock` error, and non-`ErrNoBackendLoaded` `listenInterface` fallback | → | `runTFTPServerPlugin` (`issuequeue.Start(p)`), `runTransfer`, `windowSender.sendWindow`, `listenInterface` (`internal/plugins/tftpserver/{register,handler,window_send,config_tx}.go`) | `TestTFTPServerSideErrorsAreWarnedAndRaised` |
| Image-server zefs temp-directory cleanup failure | → | `runImageServerPlugin` (`issuequeue.Start(p)`), `prepareZefs`, `removeZefsDir` (`internal/plugins/imageserver/{register,config_tx}.go`) | `TestImageServerZefsCleanupFailureIsWarnedAndRaised` |
| VRRP state-change EventBus emit failure | → | `runVRRPEngine` (`issuequeue.Start(p)`), `publishStateChange` (`internal/plugins/vrrp/{register,telemetry}.go`) | `TestVRRPPublishStateChangeEmitFailureIsWarnedAndRaised` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to `internal/plugins/` (excluding `ospf`, `isis`, `ldp`, `rsvpte`, `iface`, `fib`, `firewall`, `traffic`, `policyroute`) and `internal/exabgp/` | Every previously flagged site across the 26 files resolves cleanly so the filtered `findings` count for `error-dropped-below-warn` is `0` and every Answer-3 entry appears in `annotated` with a non-empty, visible, non-enclosing `guard` name | The 35 Answer-3 sites (plus the `errors.Is(rerr, iface.ErrNoBackendLoaded)` branch in `tftpserver/config_tx.go`) remain in `annotated` because each is a verified teardown close, probe-socket close, exited-script stdin write, best-effort RFC 1350 ERROR reply, RFC 2131 Info-documented broadcast fallback, no-backend install fallback, or unconfigured-container record refusal where going on is correct and no operator action exists |
| AC-2 | `./le arch doctor-coverage check`, `./le plugin boundary check`, and unit tests driving each Answer-2 plugin's `issuequeue` over a stub `issuequeue.Publisher` | Every new `Warn` log line carries `"code", <code>` backed by `diagnostic.Register` + `diagnostic.RegisterEventCode` in the owning package's `register_doctor_codes.go` AND enqueues its report-bus raise (`RaiseWarning` or `RaiseError` per the Event-Code Severity & Rate-Bound Table) on the plugin's `issuequeue.Queue` (started over `*sdk.Plugin` in `register.go` and stopped after workers exit, before `p.Close()`); `./le arch doctor-coverage check` and `./le plugin boundary check` both exit 0 | None: every new `Warn` site must carry a covered event code and raise it on the report bus, and no plugin may introduce an unguarded `dangerousCalls` call such as `iface.GetBackend()` |
| AC-3 | Trigger each of the 15 Answer-2 failures (plus `tftpserver` `windowSender.sendWindow` outer `readBlock` error) in unit tests with a capturing logger and a stub `issuequeue.Publisher` (`q.Flush()`) | Each site emits a `WARN` log line naming what failed, why it matters, the exact fix (`ai/rules/user-facing-errors.md`), its registered doctor `code`, and `issuequeue.Outcome`, AND delivers the matching `sdk.Issue` (`Code`, `Severity`, `Subject`, `Message`, `Fix`) to `Publisher.RaiseIssue` (and `ClearIssue` on recovery for `RaiseWarning` codes); rate-limited sites (`dhcpserver` `serveRequests`, `tftpserver` `runTransfer`/`sendWindow`, `ntp` `handleDHCPEvent`, `vrrp` `publishStateChange`) emit and raise at most once per window under repeated errors | Expected no-op conditions (`rpc.StateUnavailable` / `nil` state client in `loadBaselines`/`saveTime`, absent `/dev/rtc0` `ENOENT` in `setRTC`, non-Linux stubs, `samplingWorker.Stop` on interfaces never installed, `tftpserver` `listenInterface` when `errors.Is(rerr, iface.ErrNoBackendLoaded)`) neither warn nor raise so healthy hosts produce zero false alerts |
| AC-4 | Run the scoped unit tests for every modified package (`./le test unit pkg path ...`) and the functional `.ci` suites the change reaches (`./le test dhcp`, `./le test exabgp`, and affected plugin `.ci` suites) in the main checkout | All unit and functional tests pass with unchanged wire and protocol behavior, proving zero regressions across ExaBGP scripts, DHCP, TFTP, image server, NTP, DNS doctor probes, container runtime/network, flow export, DDoS detection, and VRRP | None: every reached package test and functional suite must pass cleanly |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs NTP on a router whose state store write, RTC write, `clock-synced` emit, or DHCP option-42 event fails | `ntp` `runNTPPlugin` (`issuequeue`) → `syncWorker.doSync` / `handleDHCPEvent` → `WARN` log with doctor `code` + `issuequeue.Outcome`, and `sdk.Plugin.RaiseIssue` into `ze show doctor` (`recent-events`) / `ze show warnings` / `ze show errors` | `TestNTPPostSyncAndDHCPEventErrorsAreWarnedAndRaised` |
| 2 | Runs DDoS detection when baseline store load/save fails or `iface.AddressIsLocal` fails during characterization | `ddos-detect` `runEngine` (`issuequeue`) → `loadBaselines` / `saveBaselineContext` / `classifyDirection` → `WARN` log with doctor `code` + `issuequeue.Outcome` and report-bus `RaiseError` | `TestDDoSDetectPersistenceAndDirectionErrorsAreWarnedAndRaised` |
| 3 | Operates DHCP, TFTP, image server, flow export, or VRRP when a reply write, transfer dial, file read, non-`ErrNoBackendLoaded` interface resolve, zefs cleanup, destroy listener, tc teardown, or state-event emit fails | Owning plugin `register.go` (`issuequeue`) → error branch → rate-limited/once `WARN` log with doctor `code` + `issuequeue.Outcome` and report-bus `RaiseWarning`/`RaiseError` (with `ClearWarning` on recovery for warning codes) | `TestDHCPServeRequestsReplyWriteFailureIsWarnedRaisedAndRateLimited`, `TestTFTPServerSideErrorsAreWarnedAndRaised`, `TestImageServerZefsCleanupFailureIsWarnedAndRaised`, `TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise`, `TestVRRPPublishStateChangeEmitFailureIsWarnedAndRaised` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNTPPostSyncAndDHCPEventErrorsAreWarnedAndRaised` | `internal/plugins/ntp/ntp_test.go` | AC-2, AC-3 for `ntp.go` (`clock-synced` emit `RaiseError`, `setRTC` and `saveTime` `RaiseWarning` + `ClearWarning` on recovery/stop, `handleDHCPEvent` rate-limited `RaiseError`), asserting `WARN` fields, `issuequeue.Outcome`, and stub `Publisher` calls | PASS (re-run owed after Run 1 fix) |
| `TestDDoSDetectPersistenceAndDirectionErrorsAreWarnedAndRaised` | `internal/plugins/ddos/detect/detector_test.go` | AC-2, AC-3 for `detector.go` and `persist.go` (`RaiseError` + `WARN` + `issuequeue.Outcome`; silent on `rpc.StateUnavailable`) | PASS |
| `TestDHCPServeRequestsReplyWriteFailureIsWarnedRaisedAndRateLimited` | `internal/plugins/dhcpserver/register_test.go` | AC-2, AC-3 for `dhcpserver/register.go:489` rate-limiting (`warnLimited`), counter, `WARN`, `issuequeue.Outcome`, and stub `Publisher` `RaiseError` | PASS |
| `TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise` | `internal/plugins/flowexport/conntrack_worker_test.go`, `internal/plugins/flowexport/sampling_worker_test.go` | AC-2, AC-3 for `conntrack_worker.go:120` (`RaiseWarning` + `ClearWarning` on recovery/stop/overlap) and `sampling_worker.go:141` (`RaiseError` only for installed interfaces) | PASS (re-run owed after Run 1 fix) |
| `TestTFTPServerSideErrorsAreWarnedAndRaised` | `internal/plugins/tftpserver/handler_test.go`, `internal/plugins/tftpserver/config_tx_test.go` | AC-2, AC-3 for `handler.go:397` (`RaiseError`), `window_send.go:128` outer `readBlock` (`RaiseError`), and `config_tx.go:200` (`RaiseWarning` on non-`ErrNoBackendLoaded` resolve error + clear on clean apply/rollback; silent on `iface.ErrNoBackendLoaded` with zero `iface.GetBackend()` calls) | PASS (re-run owed after Run 1 fix) |
| `TestImageServerZefsCleanupFailureIsWarnedAndRaised` | `internal/plugins/imageserver/config_tx_test.go` | AC-2, AC-3 for `imageserver/config_tx.go:177,193` (`RaiseError` + `WARN` + `issuequeue.Outcome`) | PASS |
| `TestVRRPPublishStateChangeEmitFailureIsWarnedAndRaised` | `internal/plugins/vrrp/telemetry_test.go` | AC-2, AC-3 for `vrrp/telemetry.go:140` (rate-limited `RaiseError` + `WARN` + `issuequeue.Outcome`) | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated `error-dropped-below-warn` findings under this child's paths | 0 | 0 | N/A | 1 |
| uncovered new `Warn` log sites in `./le arch doctor-coverage check` | 0 | 0 | N/A | 1 |
| per-window `WARN` and `issuequeue.RaiseError` emissions on repeated DHCP reply write / TFTP dial / TFTP read / NTP DHCP event / VRRP emit errors inside one 1-minute window | 1 | 1 | 0 | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Reached `.ci` suites under `test/dhcp`, `test/exabgp`, `test/plugin` | `test/` | Unchanged DHCP/ExaBGP/plugin end-to-end behavior under default WARN logging and active `issuequeue` | PASS (`dhcp` 1/1 runnable 1.6s; `vrrp` 7/7 runnable 6.4s) |

## Files to Modify
- `internal/exabgp/bridge/bridge_ack.go` - 4 Answer-3 annotations: `AckMode.AnswerLocal` 56; `AckMode.WriteAck` 113; `AckMode.WriteError` 144; `AckMode.WriteErrorMarker` 156
- `internal/exabgp/bridge/bridge_batch.go` - 1 Answer-3 annotation: `writeReply` 514
- `internal/exabgp/bridge/bridge_ping.go` - 1 Answer-3 annotation: `writePong` 84
- `internal/exabgp/bridge/bridge_status.go` - 1 Answer-3 annotation: `statusInfo.write` 202
- `internal/plugins/as112/doctor.go` - 2 Answer-3 annotations: `probeBindable` 183, 190
- `internal/plugins/container/container.go` - 1 Answer-3 annotation: `runContainerEngine.func` 90
- `internal/plugins/container/network/attach.go` - 1 Answer-3 annotation: `manager.ended` 130
- `internal/plugins/container/supervise.go` - 5 Answer-3 annotations: `supervisor.releaseTunablesWhenIdle` 802; `supervisor.cleanup` 918; `supervisor.stopTogether` 995, 1011, 1030
- `internal/plugins/ddos/detect/detector.go` - 2 Answer-2 sites (`issuequeue.RaiseError` + `WARN` + doctor code + `issuequeue.Outcome`): `detector.saveBaselineContext` 265; `detector.classifyDirection` 522
- `internal/plugins/ddos/detect/persist.go` - 1 Answer-2 site (`issuequeue.RaiseError` + `WARN` + doctor code + `issuequeue.Outcome`, suppressing `rpc.StateUnavailable`): `loadBaselines` 93
- `internal/plugins/ddos/detect/register.go` - start `issuequeue.Start(p)` over `*sdk.Plugin` in `runEngine`, pass queue to detector, call `q.Stop()` after `stopCurrent` and before `p.Close()`
- `internal/plugins/ddos/detect/register_doctor_codes.go` - register `doctor-ddos-detect-baseline-save-failed`, `doctor-ddos-detect-baseline-load-failed`, `doctor-ddos-detect-direction-fallback` (`diagnostic.Register` + `diagnostic.RegisterEventCode`)
- `internal/plugins/dhcpserver/register.go` - start `issuequeue.Start(p)` over `*sdk.Plugin` in `runDHCPServerPlugin` (stop after `srv.stop()`, before `closeLogged(p, ...)`); 2 Answer-3 annotations (`closeLogged` 236, `replyDestination` 583); 1 Answer-2 rate-limited `issuequeue.RaiseError` + `WARN` + doctor code + counter (`serveRequests` 489)
- `internal/plugins/dhcpserver/register_doctor_codes.go` - register `doctor-dhcp-reply-write-failed` (`diagnostic.Register` + `diagnostic.RegisterEventCode`)
- `internal/plugins/dnsresolver/doctor.go` - 2 Answer-3 annotations: `probeBindable` 148, 155
- `internal/plugins/exabgp/bridgerun/script.go` - 2 Answer-3 annotations: `script.writeLoop` 364, 373
- `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` - 1 Answer-3 annotation: `loadConntrackModules` 69
- `internal/plugins/flowexport/conntrack_worker.go` - 1 Answer-2 site (`issuequeue.RaiseWarning` + `ClearWarning` on recovery/stop + `WARN` + doctor code + `issuequeue.Outcome`): `conntrackWorker.Start` 120
- `internal/plugins/flowexport/sampling_worker.go` - 1 Answer-2 site (track installed interfaces; `issuequeue.RaiseError` + `WARN` + doctor code + `issuequeue.Outcome`): `samplingWorker.Stop` 141
- `internal/plugins/flowexport/register.go` - start `issuequeue.Start(p)` over `*sdk.Plugin` in `runEngine`, pass queue to `conntrackWorker` and `samplingWorker`, call `q.Stop()` after `exp.stop()` and before `p.Close()`
- `internal/plugins/flowexport/register_doctor_codes.go` - register `doctor-flowexport-conntrack-destroy-unavailable`, `doctor-flowexport-sampling-teardown-failed` (`diagnostic.Register` + `diagnostic.RegisterEventCode`)
- `internal/plugins/geodns/doctor.go` - 2 Answer-3 annotations: `probeBindable` 191, 198
- `internal/plugins/imageserver/config_tx.go` - 2 Answer-2 sites (`issuequeue.RaiseError` + `WARN` + doctor code + `issuequeue.Outcome`: `prepareZefs` 177, `removeZefsDir` 193); 1 Answer-3 annotation (`imageServer.closeServers` 201)
- `internal/plugins/imageserver/register.go` - start `issuequeue.Start(p)` over `*sdk.Plugin` in `runImageServerPlugin` (pass queue to `imageServer`, stop after `srv.stop()`, before `closeLogged(p, ...)`); 1 Answer-3 annotation (`closeLogged` 314)
- `internal/plugins/ntp/ntp.go` - 4 Answer-2 sites (`issuequeue` raises + `WARN` + doctor codes + `issuequeue.Outcome`): `syncWorker.doSync` 317 (`RaiseError`), 323 (`RaiseWarning` + `RunRaises` + `ClearWarning`), 331 (`RaiseWarning` + `RunRaises` + `ClearWarning`); `syncWorker.handleDHCPEvent` 453 (rate-limited `RaiseError`)
- `internal/plugins/ntp/register.go` - start `issuequeue.Start(p)` over `*sdk.Plugin` in `runNTPPlugin`, pass queue to `syncWorker`, call `q.Stop()` after worker stop/join and before `p.Close()`
- `internal/plugins/ntp/register_doctor_codes.go` - register `doctor-ntp-clock-synced-emit-failed`, `doctor-ntp-rtc-write-failed`, `doctor-ntp-time-persist-failed`, `doctor-ntp-dhcp-event-malformed` (`diagnostic.Register` + `diagnostic.RegisterEventCode`)
- `internal/plugins/tftpserver/config_tx.go` - `listenInterface` 200: inspect `isNoBackendLoaded(rerr)` (`errors.Is(rerr, iface.ErrNoBackendLoaded)`, no `iface.GetBackend()` call) — Answer-3 `guards bindTFTP` on `ErrNoBackendLoaded`, Answer-2 `issuequeue.RaiseWarning` (cleared on clean apply) + `WARN` + doctor code on any other error
- `internal/plugins/tftpserver/handler.go` - 3 Answer-3 annotations (`serveRequests` 274, `handleRRQ.func` 287, `sendOACKAndWait` 452); 1 Answer-2 rate-limited `issuequeue.RaiseError` + `WARN` + doctor code (`runTransfer` 397)
- `internal/plugins/tftpserver/panic_guard.go` - 1 Answer-3 annotation: `signalPanic` 66
- `internal/plugins/tftpserver/register.go` - start `issuequeue.Start(p)` over `*sdk.Plugin` in `runTFTPServerPlugin` (pass queue to `tftpServer`, stop after `srv.stop()`, before `closeLogged(p, ...)`); add `isNoBackendLoaded(err)` helper using `errors.Is(err, iface.ErrNoBackendLoaded)`; 1 Answer-3 annotation (`closeLogged` 92)
- `internal/plugins/tftpserver/window_send.go` - 3 Answer-3 annotations (`serveFile` 33, `serveFile.func` 39, `windowSender.sendWindow` 128 `wErr`) + rate-limited `issuequeue.RaiseError` + `WARN` + doctor code for outer `readBlock` `err`
- `internal/plugins/vrrp/register.go` - start `issuequeue.Start(p)` over `*sdk.Plugin` in `runVRRPEngine`, wire queue into telemetry (`setIssueQueue`), call `q.Stop()` after `eng.stopAll()` and before `p.Close()`
- `internal/plugins/vrrp/telemetry.go` - 1 Answer-2 site (rate-limited `issuequeue.RaiseError` + `WARN` + doctor code + `issuequeue.Outcome`): `publishStateChange` 140

Lines are those of HEAD on 2026-11-26; re-run `./le arch refuse-by-default report` before Phase 1.

## Files to Create
- `internal/plugins/imageserver/register_doctor_codes.go` - register `doctor-imageserver-zefs-cleanup-failed` (`diagnostic.Register` + `diagnostic.RegisterEventCode`)
- `internal/plugins/tftpserver/register_doctor_codes.go` - register `doctor-tftp-interface-resolve-fallback`, `doctor-tftp-transfer-dial-failed`, `doctor-tftp-file-read-failed` (`diagnostic.Register` + `diagnostic.RegisterEventCode`)
- `internal/plugins/vrrp/register_doctor_codes.go` - register `doctor-vrrp-state-event-emit-failed` (`diagnostic.Register` + `diagnostic.RegisterEventCode`)
- Unit test additions in the packages listed in TDD Test Plan.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC added |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | No command added |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No command added |
| Functional test for new RPC/API | N-A | No RPC added |
| Pipe completeness | N-A | No command output changed |
| Env var registration | N-A | No env var added |
| Doctor check for runtime dependencies | No | No new runtime dependency; the three DNS `probeBindable` checks already report bind failures |
| Doctor code per problem class | Yes | Every new `Warn` log added by this spec registers its doctor `code` via `diagnostic.Register` + `diagnostic.RegisterEventCode` in the owning package's `register_doctor_codes.go` (`ddos/detect`, `dhcpserver`, `flowexport`, `imageserver`, `ntp`, `tftpserver`, `vrrp`), emits `"code", <code>` and `issuequeue.Outcome(qErr)`, AND unconditionally enqueues its report-bus raise (`RaiseWarning`/`RaiseError`) on `internal/component/plugin/issuequeue` started over `*sdk.Plugin` in each plugin's `register.go`, clearing `./le arch doctor-coverage check` and `recent-events` |
| Prometheus counters/metrics | Yes | Existing plugin metric registries; add/increment reply-write-error counter in `dhcpserver` beside the rate-limited `WARN` and raise, documented in `docs/plugin-development/metrics.md` |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Bug/observability fix only |
| 2 | Config syntax changed? | No | No YANG config change |
| 3 | CLI command added/changed? | No | No CLI command change |
| 4 | API/RPC added/changed? | No | No API/RPC change |
| 5 | Plugin added/changed? | No | No plugin registration change |
| 6 | Has a user guide page? | No | Operator workflows unchanged |
| 7 | Wire format changed? | No | Zero wire changes |
| 8 | Plugin SDK/protocol changed? | No | Uses existing `sdk.Plugin.RaiseIssue` / `ClearIssue` and `internal/component/plugin/issuequeue` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol/RFC change |
| 10 | Test infrastructure changed? | No | Unit tests only |
| 11 | Affects daemon comparison? | No | Internal logging/reporting/annotations only |
| 12 | Internal architecture changed? | No | Reuses existing `issuequeue`, `diagnostic.RegisterEventCode`, and plugin loggers |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | If a new counter is added in `dhcpserver` | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Doctor codes self-register via `diagnostic.Register` / `RegisterEventCode` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors`: `ai/digests/flow-ddos.md`, `docs/architecture/config/environment.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/container/image-store.md`, `docs/architecture/container/network.md`, `docs/architecture/container/runtime.md`, `docs/architecture/core-design.md`, `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md`, `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md`, `docs/architecture/ddos/ddos-detect-enhancements.md`, `docs/architecture/dns/as112.md`, `docs/architecture/dns/geodns.md`, `docs/architecture/dns/lan-resolver.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/flowexport/flow-export-0-umbrella.md`, `docs/architecture/flowexport/flow-export-1-counter-export.md`, `docs/architecture/flowexport/flow-export-2-flow-records.md`, `docs/architecture/iface/logical-name-resolution.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/provisioning/dhcp-server.md`, `docs/architecture/provisioning/image-server.md`, `docs/architecture/provisioning/tftp-server.md`, `docs/architecture/testing/interop.md`, `docs/architecture/traffic/traffic-analysis-layers.md`, `docs/architecture/traffic/traffic-usage-monitor.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`, `docs/features/interfaces.md`. All remain accurate because wire, protocol, and documented fallback behaviors are preserved; re-confirm with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-4-system-plugins.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No syntax change |

## Design presentation

### What changes

This spec resolves all 50 `error-dropped-below-warn` findings currently reported under the system-plugins door (`internal/plugins/` outside routing/forwarding, plus `internal/exabgp/`) across 26 files. Three sites from the original 2026-11-02 snapshot (`bridge.go`, `bridgerun/script.go` `script.reap`, `ntp/register.go` `runNTPPlugin`) are already fixed at HEAD and retired into Existing Mechanisms, while eight newer sites (`container` and `ddos/detect/persist.go`) are brought into scope. Across the 7 plugins with Answer-2 sites (`ddos/detect`, `dhcpserver`, `flowexport`, `imageserver`, `ntp`, `tftpserver`, `vrrp`), each plugin's `register.go` starts an `internal/component/plugin/issuequeue.Queue` over `*sdk.Plugin` and stops it after its workers exit and before `p.Close()`. Fifteen operator-actionable failure branches (plus TFTP's outer `readBlock` server-side read failure) move from `Debug`/`Info` to `WARN` carrying what failed, why it matters, the exact fix, a registered doctor `code` in `register_doctor_codes.go`, and `issuequeue.Outcome(qErr)`, AND unconditionally enqueue their report-bus raise (`RaiseWarning` with clear-on-recovery for persistent degraded states; `RaiseError` for discrete events, rate-limited on per-packet/per-transfer/flapping paths). In `tftpserver/config_tx.go`, `listenInterface` inspects `errors.Is(rerr, iface.ErrNoBackendLoaded)` rather than calling `iface.GetBackend()` so `./le plugin boundary check` stays clean and forked runs behave consistently. The remaining 35 sites—probe-socket closes, teardown closes, broken stdin pipes to exited ExaBGP scripts, best-effort RFC 1350 TFTP ERROR packets, container teardown/unconfigured-record checks, DHCP's documented Info broadcast fallback, and appliance built-in `modprobe`—keep their existing quiet logging and receive explicit `// refuse-by-default: guards <Symbol> -- <why>` annotations naming the visible guard symbol.

### Before and after

1. **NTP post-sync persistence, RTC, clock-synced emit, and DHCP option-42 parsing (`internal/plugins/ntp/{register,ntp}.go`)**
   - Before: At the default `WARN` log level, a failed managed-store `saveTime`, a failed `/dev/rtc0` write on an existing RTC device, a failed `clock-synced` EventBus emit, or a malformed DHCP lease event logged only at `Debug` and never appeared in `ze show doctor` or `ze show warnings`/`ze show errors`.
   - After: `runNTPPlugin` owns an `issuequeue.Queue` over `*sdk.Plugin`. Each failure logs at `WARN` with what, why, exact fix, doctor `code`, and `issuequeue.Outcome`, and raises on the daemon report bus (`RaiseWarning` with `RunRaises` + `ClearWarning` on recovery for `doctor-ntp-rtc-write-failed` and `doctor-ntp-time-persist-failed`; `RaiseError` for `doctor-ntp-clock-synced-emit-failed` and rate-limited `doctor-ntp-dhcp-event-malformed`), while expected no-ops (`ENOENT` on RTC-less hosts, non-Linux stub, `nil` state client / `rpc.StateUnavailable`) stay silent.
2. **DDoS baseline persistence and victim direction lookup (`internal/plugins/ddos/detect/{register,detector,persist}.go`)**
   - Before: Failed baseline store saves, failed baseline store reads, and `iface.AddressIsLocal` failures during characterization logged only at `Debug`, hiding lost baselines or a fallback from INPUT to FORWARD hook.
   - After: `runEngine` owns an `issuequeue.Queue` over `*sdk.Plugin`. Real store save/load failures (excluding `rpc.StateUnavailable`) and `AddressIsLocal` failures log at `WARN` with what, why, fix, `issuequeue.Outcome`, and registered doctor codes (`doctor-ddos-detect-baseline-save-failed`, `doctor-ddos-detect-baseline-load-failed`, `doctor-ddos-detect-direction-fallback`), and raise `RaiseError` events on the report bus.
3. **DHCP reply write, TFTP server-side errors, image-server zefs cleanup, flow-export destroy/teardown, and VRRP state-event emit**
   - Before: Failed DHCP `WriteToUDP` replies, TFTP `net.DialUDP` / `readBlock` / non-`ErrNoBackendLoaded` `listenInterface` errors, failed temp-dir `os.RemoveAll` in `imageserver`, unavailable conntrack destroy listener, failed tc sampling teardown on installed interfaces, and failed VRRP `state-change` emits were hidden at `Debug` or `Info` with no report-bus visibility.
   - After: Each plugin wires `issuequeue.Start(p)` in `register.go`, logs at `WARN` (rate-limited for DHCP/TFTP/VRRP hot/flapping paths) with what, why, fix, registered doctor `code`, and `issuequeue.Outcome`, and raises the matching `RaiseWarning`/`RaiseError` on the daemon report bus (with `ClearWarning` on recovery for `doctor-flowexport-conntrack-destroy-unavailable` and `doctor-tftp-interface-resolve-fallback`), clearing `./le arch refuse-by-default report`, `./le arch doctor-coverage check`, and `./le plugin boundary check`.
4. **Teardown, probe-socket close, ExaBGP stdin pipe, RFC 1350 ERROR reply, `ErrNoBackendLoaded`, and documented Info fallback sites**
   - Before: Unannotated `Debug`/`Info` drops triggered `error-dropped-below-warn` in `./le arch refuse-by-default report`.
   - After: Each carries `// refuse-by-default: guards <Symbol> -- <why>` naming the visible guard (`Bridge.Run`, `script.runOnce`, `as112ListenDiagnostic`, `listenerDiagnostics`, `geodnsListenDiagnostic`, `manager.created`, `broadcastReply`, `setupConntrackTracking`, `bindTFTP`, `tftpGuard`, etc.) with zero change to runtime or wire behavior.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Register `diagnostic.EventCode` codes and only log at `WARN` without raising via `issuequeue` | Violates `docs/architecture/doctor-and-health-checks.md` ("Registering a code whose sites only log, and never raise, would declare a row this check never shows: every site carrying a registered event code raises it") and `ai/rules/user-facing-errors.md` |
| Call `iface.GetBackend() != nil` in `tftpserver/config_tx.go` to distinguish no-backend fallback | `iface.GetBackend` is on `dangerousCalls` (`internal/le/plugin/boundary/pluginboundary.go:108`), which fails `./le plugin boundary check` and reads `nil` in a forked `tftpserver` process; checking `errors.Is(rerr, iface.ErrNoBackendLoaded)` inspects the error already returned by `bindDeviceFor` without any boundary violation |
| Raise all 50 sites uniformly to `WARN` | Violates `ai/rules/user-facing-errors.md` ("A line that reports nothing wrong MUST be logged at `Info` or `Debug`, never at `Warn`"), departs from `container/network.md` and `dhcp-server.md`, and would spam operators on normal teardowns, exited ExaBGP scripts, and TFTP clients that left |
| Change DNS `probeBindable` to report `Close()` failures as doctor port-unavailable findings | `probeBindable` already returns bind failures; turning a successful bind's `Close()` error into a port-unavailable warning would misdiagnose the host |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- run `./le arch refuse-by-default report '|' json` filtered to this child's paths to confirm the 50-finding population; create the three new `register_doctor_codes.go` files (`imageserver`, `tftpserver`, `vrrp`) and add the new event codes to the four existing `register_doctor_codes.go` files (`ddos/detect`, `dhcpserver`, `flowexport`, `ntp`); wire `issuequeue.Start(p)` / `q.Stop()` in all 7 Answer-2 plugins' `register.go` files; write failing unit tests from the TDD Test Plan asserting both `WARN` logs (`code`, `issuequeue.Outcome`, what/why/fix) and stub `issuequeue.Publisher` raises/clears.
2. **Phase: Answer-2 `WARN` + `issuequeue` raise sites** -- implement the 15 Answer-2 upgrades (plus `tftpserver` `sendWindow` outer `readBlock`), using `errors.Is(rerr, iface.ErrNoBackendLoaded)` in `tftpserver` `listenInterface` (zero `iface.GetBackend()` calls), rate-limiting DHCP/TFTP/NTP-DHCP/VRRP sites, and suppressing expected no-op cases (`rpc.StateUnavailable`, `ENOENT`/non-Linux `setRTC`, uninstalled sampling interfaces, `iface.ErrNoBackendLoaded`), until all new unit tests pass and `./le arch doctor-coverage check` and `./le plugin boundary check` are clean.
3. **Phase: Answer-3 guard annotations** -- add the 35 `// refuse-by-default: guards <Symbol> -- <why>` annotations (plus the `ErrNoBackendLoaded` branch in `tftpserver/config_tx.go`), verifying each guard symbol is visible and non-enclosing.
4. **Phase: Prove** -- verify AC-1 through AC-4: zero unannotated `error-dropped-below-warn` findings under this child's paths, `./le arch doctor-coverage check` clean, `./le plugin boundary check` clean, `./le arch tier check` clean, and all scoped package tests and reached `.ci` suites green in the main checkout.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 50 HEAD instances resolved at exact file:line (15 Answer 2 + 35 Answer 3) |
| Correctness | Zero change to ExaBGP script stdin/stdout, DHCP, TFTP, NTP, DNS, VRRP, flow-export, or container wire/protocol behavior |
| Operator errors, doctor codes, and report bus | Every new `Warn` line names what, why, exact fix, `"code", <code>` registered via `diagnostic.Register` + `RegisterEventCode` in `register_doctor_codes.go`, and `issuequeue.Outcome(qErr)`, AND enqueues its `RaiseWarning`/`RaiseError` on the plugin's `issuequeue` over `*sdk.Plugin` (stopped after workers exit, before `p.Close()`); `./le arch doctor-coverage check` passes |
| Process boundary and rate limiting | Zero `iface.GetBackend()` calls in `tftpserver` (`./le plugin boundary check` clean); per-packet/per-transfer/flapping sites are rate-limited; storeless/RTC-less/`ErrNoBackendLoaded`/non-Linux paths produce zero spurious warnings or raises |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero unannotated `error-dropped-below-warn` findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered to this child's paths |
| Doctor-coverage commit gate clean | `./le arch doctor-coverage check` |
| Tier and process-boundary gates clean | `./le arch tier check`, `./le plugin boundary check` |
| Scoped package unit tests | `./le test unit pkg path ./internal/exabgp/bridge/ ./internal/plugins/as112/ ./internal/plugins/container/ ./internal/plugins/container/network/ ./internal/plugins/ddos/detect/ ./internal/plugins/dhcpserver/ ./internal/plugins/dnsresolver/ ./internal/plugins/exabgp/bridgerun/ ./internal/plugins/flowexport/ ./internal/plugins/geodns/ ./internal/plugins/imageserver/ ./internal/plugins/ntp/ ./internal/plugins/tftpserver/ ./internal/plugins/vrrp/` (main thread) |
| Scoped functional `.ci` suites | `./le repo changed scope print both` + `./le test dhcp`, `./le test exabgp`, and any other `.ci` suite reached by the changed packages (main thread) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | No `WARN` line or report-bus issue echoes an unbounded client-controlled TFTP filename, DHCP option payload, or raw untrusted packet bytes |
| Credential exposure | `imageserver` zefs cleanup `WARN` and report-bus issue carry the temporary directory path and error, never the SSH password hash |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint, doctor-coverage, or plugin-boundary failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Wire `internal/component/plugin/issuequeue` over `*sdk.Plugin` in all 7 Answer-2 plugins so every `RegisterEventCode` site both logs at `WARN` and raises on the daemon report bus | Bare `Warn` logs with `RegisterEventCode` (log-only), or direct `internal/core/report` calls | `docs/architecture/doctor-and-health-checks.md` forbids registering event codes that never raise, and direct `internal/core/report` fails across forked plugin boundaries; `issuequeue` works identically internal and forked without blocking hot/locked goroutines |
| Inspect `errors.Is(rerr, iface.ErrNoBackendLoaded)` in `tftpserver` `listenInterface` instead of calling `iface.GetBackend()` | Calling `iface.GetBackend() != nil` | `iface.GetBackend` is on `./le plugin boundary check`'s `dangerousCalls` list and reads `nil` in a forked plugin; `ErrNoBackendLoaded` is already returned by `iface.Resolve` via `bindDeviceFor` |
| Audit every site at HEAD before assigning answers: DNS `probeBindable` closes are Answer 3, NTP post-sync/DHCP drops are Answer 2, and 3 already-fixed sites retire to Existing Mechanisms | Keeping the stale 2026-11-02 site list and wiring tests | Source inspection showed `probeBindable` already returns bind errors and `doSync` already warns on unreachable NTP servers; stale wiring tests would test non-existent bugs |
| Keep `dhcpserver` `replyDestination` (`neighborRead`) and `container/network` `manager.ended` (`OwnedLinkRemove`) at `Info` under Answer 3 | Elevating both to `WARN` | Both `docs/architecture/provisioning/dhcp-server.md` and `docs/architecture/container/network.md` explicitly specify `Info` logging with deterministic fallback/cleanup; keeping `Info` with named guards avoids departing from documented architecture and avoids noisy warnings on unprivileged/non-Linux hosts |
| Follow the umbrella's three answers in order (with explicit severity and rate/once bounds on report-bus raises) | Raise every line to `WARN` | A `WARN` where nothing is wrong violates `user-facing-errors.md` and floods operator logs and the report bus |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow (with Changed-Wire Census), and Design presentation sections completed
- [ ] AC-N rows carry positive observable assertions and Permitted Survivors and Why
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-4-system-plugins.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- Resolved all 50 `error-dropped-below-warn` instances across 26 files under `internal/plugins/` (outside routing/forwarding) and `internal/exabgp/`: 15 Answer-2 sites (plus `tftpserver` `windowSender.sendWindow` outer `readBlock` error) upgraded to `WARN` + `issuequeue` raises under 15 registered doctor event codes, and 35 Answer-3 guard-protected sites (plus the `iface.ErrNoBackendLoaded` branch of `tftpserver` `listenInterface`) annotated with `// refuse-by-default: guards <Symbol> -- <why>`.
- Wired `internal/component/plugin/issuequeue.Start(p)` and `defer issues.Stop()` (running after worker/server teardown and before `p.Close()`) across all 7 Answer-2 plugins (`ddos/detect`, `dhcpserver`, `flowexport`, `imageserver`, `ntp`, `tftpserver`, `vrrp`).
- Registered all 15 event codes via `diagnostic.Register` and `diagnostic.RegisterEventCode` across 4 existing `register_doctor_codes.go` files (`ddos/detect`, `dhcpserver`, `flowexport`, `ntp`) and 3 new `register_doctor_codes.go` files (`imageserver`, `tftpserver`, `vrrp`).
- Added the `ze_dhcp_reply_write_errors_total` CounterVec (`interface` label) in `internal/plugins/dhcpserver/register.go` (`configureMetrics`, `replyWriteErrorsCounter`, `serveRequests`).
- Added unit tests proving `WARN` log fields, `issuequeue.Outcome`, stub `issuequeue.Publisher` raises/clears, rate limits, expected no-op suppressions, and zero credential leakage across all 7 Answer-2 plugins.
- Wrote operator news fragment `website/changes/fragments/dropped-error-4-system-plugins.md` (STE check OK) and updated external citers (`plan/release/fixes/spec-dropped-error-0-umbrella.md` row 82 and `features/lan-dns-resolver.md` row 19) to bare stem.

### Bugs Found/Fixed
- `internal/plugins/tftpserver/config_tx.go` (`bindTFTP`, `tftpServer.install`): on strict reload failure, `bindTFTP` returned `nil, nil, err`, dropping `warnedIfaces` so rollback via `s.restore` never cleared `doctor-tftp-interface-resolve-fallback` warnings raised for interfaces in the rejected candidate config. Fixed by returning `warnedIfaces` and merging them into `s.resolveWarned` before `s.restore`; covered by `TestTFTPServerSideErrorsAreWarnedAndRaised/interface-resolve-fallback`.
- `internal/plugins/ntp/ntp.go` (`syncWorker.run`): exiting a `syncWorker` (on disable, reload replacement, or plugin shutdown) did not clear standing `doctor-ntp-rtc-write-failed` or `doctor-ntp-time-persist-failed` warnings, leaving stale warnings on disable and preventing replacement workers (whose `failures == 0`) from clearing them. Fixed by deferring `clearRTCFailure` and `clearPersistFailure` after `defer close(w.done)` in `syncWorker.run`; covered by `TestNTPPostSyncAndDHCPEventErrorsAreWarnedAndRaised`.
- `internal/plugins/flowexport/conntrack_worker.go` (`conntrackWorker.Start`, `conntrackWorker.Stop`): during config reload, `flowexport/register.go` starts the replacement `conntrackWorker` before stopping `prev`, so when `NewDestroyListener` remained unavailable `prev`'s `Stop()` enqueued `ClearWarning` after the replacement worker's `RaiseWarning`, erasing the live standing warning. Fixed by reference-counting active dump-only workers with `activeDestroyWarnings atomic.Int32`; covered by `TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise/conntrack-destroy-unavailable`.

### Documentation Updates
- `docs/plugin-development/metrics.md` line 279: carries the STE-clean `ze_dhcp_reply_write_errors_total` row with anchor `<!-- source: internal/plugins/dhcpserver/register.go -- configureMetrics, replyWriteErrorsCounter, serveRequests -->` (already landed on HEAD via sibling commit `5b59287672`; verified present on HEAD).
- `website/changes/fragments/dropped-error-4-system-plugins.md`: created operator-facing news fragment (`./le doc ste check file` -> `OK`).
- `features/lan-dns-resolver.md` line 19 and `plan/release/fixes/spec-dropped-error-0-umbrella.md` line 82: updated citations of this spec to bare stem `spec-dropped-error-4-system-plugins` (closed 2026-11-27).
- `./le doc check all`: verified during closure.

### Deviations from Plan
- `internal/plugins/dhcpserver/config_tx.go` was modified in addition to the files originally listed in `Files to Modify` so `dhcpServer` threads its `issues *issuequeue.Queue` into `bindListenersWithIssues` and `restore`'s `dhcpServe`.
- `internal/plugins/dhcpserver/listen_resolve_test.go` was modified to match the updated `dhcpServe` test-hook signature carrying `*issuequeue.Queue`.
- `internal/plugins/flowexport/sampling_worker_test.go` was created as a separate unit test file (matching the TDD Test Plan entry) rather than appending to `conntrack_worker_test.go`.
- `docs/plugin-development/metrics.md` shows an empty diff against `HEAD` because its `ze_dhcp_reply_write_errors_total` row was already carried onto `HEAD` by commit `5b59287672` (verified present at line 279).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Initial implementation of the three standing-warning (`RaiseWarning`) plugins missed warning-clear lifecycle edge cases on strict reload rollback (`tftpserver`), worker stop/reload (`ntp`), and overlapping reload (`flowexport`) | Standing report-bus warnings persist on the daemon bus across rejected reloads, worker replacements, and overlapping exporter swaps unless explicitly cleared or ref-counted | Run 1 independent `/ze-review` during `/ze-close` | Fixed all three at source (`tftpserver/config_tx.go`, `ntp/ntp.go`, `flowexport/conntrack_worker.go`) and added regression cases in their respective tests |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Resolve all 50 `error-dropped-below-warn` instances across 26 files under system-plugins door | Done | 26 source files across `internal/exabgp/bridge`, `internal/plugins/{as112,container,ddos/detect,dhcpserver,dnsresolver,exabgp/bridgerun,flowexport,geodns,imageserver,ntp,tftpserver,vrrp}` | Filtered `./le arch refuse-by-default report` reports 0 unannotated findings and 0 `annotation-names-no-guard` findings |
| Wire `issuequeue.Start(p)` / `defer issues.Stop()` in all 7 Answer-2 plugins' `register.go` (after workers stop, before `p.Close()`) | Done | `ddos/detect/register.go:146`, `dhcpserver/register.go:135`, `flowexport/register.go:188`, `imageserver/register.go:74`, `ntp/register.go:213`, `tftpserver/register.go:69`, `vrrp/register.go:192` | Verified LIFO/explicit teardown order in all 7 functions |
| Register all 15 Answer-2 event codes via `diagnostic.Register` + `RegisterEventCode` in owning `register_doctor_codes.go` | Done | `register_doctor_codes.go` in `ddos/detect`, `dhcpserver`, `flowexport`, `imageserver`, `ntp`, `tftpserver`, `vrrp` | `./le arch doctor-coverage check` OK |
| Every Answer-2 `WARN` carries what/why/executable YANG fix, `"code"`, `issuequeue.Outcome`, bounded `Subject`/`Message`, non-nil `failed` callback | Done | `ddos/detect/{detector,persist}.go`, `dhcpserver/register.go`, `flowexport/{conntrack,sampling}_worker.go`, `imageserver/config_tx.go`, `ntp/ntp.go`, `tftpserver/{config_tx,handler,window_send}.go`, `vrrp/telemetry.go` | Verified every `fix` string (`ze show doctor`, `ze show interface`, `ze show system ntp`, `ze show vrrp`) against `*.yang` `ze:command`, every subject via `fitSubject` (`rpc.ReportSubjectMax`), every message via `report.FitMessage` (`rpc.ReportMessageMax`), and zero `fmt.Sprintf` |
| Use `errors.Is(rerr, iface.ErrNoBackendLoaded)` in `tftpserver` without calling `iface.GetBackend()` | Done | `internal/plugins/tftpserver/register.go:129` (`isNoBackendLoaded`), `config_tx.go:258` | `./le plugin boundary check` OK; zero `iface.GetBackend()` calls |
| Annotate all 35 Answer-3 sites (plus `ErrNoBackendLoaded` branch) with non-enclosing guards | Done | `bridge_{ack,batch,ping,status}.go`, `bridgerun/script.go`, `{as112,dnsresolver,geodns}/doctor.go`, `container/{container,supervise}.go`, `container/network/attach.go`, `dhcpserver/register.go`, `flowexport/conntrack_setup_appliance_linux.go`, `imageserver/{config_tx,register}.go`, `tftpserver/{config_tx,handler,panic_guard,register,window_send}.go` | Zero `annotation-names-no-guard` findings |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Filtered `./le arch refuse-by-default report '\|' json` under system-plugins door paths -> 0 `error-dropped-below-warn`, 0 `annotation-names-no-guard` | All 35 Answer-3 sites + `ErrNoBackendLoaded` branch annotated with visible non-enclosing guards |
| AC-2 | Done | `./le arch doctor-coverage check` OK; `./le plugin boundary check` OK; `./le arch tier check` OK; stub `issuequeue.Publisher` unit tests across all 7 Answer-2 plugins | Standing-warning clear edge cases fixed in Run 1 (`tftpserver`, `ntp`, `flowexport`); re-run owed on those 3 packages |
| AC-3 | Done | 7 named unit tests in `ddos/detect`, `dhcpserver`, `flowexport`, `imageserver`, `ntp`, `tftpserver`, `vrrp` asserting `WARN`, `code`, `fix`, `report=queued`, severity, subject, rate limits, and silent no-ops | Re-verified PASS under `-race` after Run 1 fixes (`tftpserver` 49.90s, `ntp` 15.25s, `flowexport` 5.33s) |
| AC-4 | Done | Main-thread `-race` unit + Linux userns suites across all 14 packages PASS (incl. `-tags ze_appliance` for `flowexport`); scoped lint 0 issues; functional `.ci` `dhcp` (1/1) and `vrrp` (7/7) PASS | Re-verified PASS after Run 1 fixes |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestNTPPostSyncAndDHCPEventErrorsAreWarnedAndRaised` | Done | `internal/plugins/ntp/ntp_test.go:1143` | Extended with worker-stop warning-clear regression check; PASS (`-race`, 15.25s) |
| `TestDDoSDetectPersistenceAndDirectionErrorsAreWarnedAndRaised` | Done | `internal/plugins/ddos/detect/detector_test.go:495` | PASS (`-race`, 8.91s) |
| `TestDHCPServeRequestsReplyWriteFailureIsWarnedRaisedAndRateLimited` | Done | `internal/plugins/dhcpserver/register_test.go:388` | PASS (`-race` userns, 7.81s + 1.09s) |
| `TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise` | Done | `internal/plugins/flowexport/conntrack_worker_test.go:143`, `sampling_worker_test.go:44` | Extended with overlapping-reload warning-clear regression check; PASS (`-race`, 5.33s) |
| `TestTFTPServerSideErrorsAreWarnedAndRaised` | Done | `internal/plugins/tftpserver/handler_test.go:1326`, `config_tx_test.go:413` | Extended with strict-reload rollback warning-clear regression check; PASS (`-race` userns, 49.90s) |
| `TestImageServerZefsCleanupFailureIsWarnedAndRaised` | Done | `internal/plugins/imageserver/config_tx_test.go:313` | PASS (`-race`, 4.81s) |
| `TestVRRPPublishStateChangeEmitFailureIsWarnedAndRaised` | Done | `internal/plugins/vrrp/telemetry_test.go:288` | PASS (`-race` userns, 72.64s) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 33 files in `Files to Modify` | Done | Verified in `git diff HEAD` |
| All 3 `register_doctor_codes.go` files in `Files to Create` (`imageserver`, `tftpserver`, `vrrp`) | Done | Verified with `ls -l` |
| `internal/plugins/flowexport/sampling_worker_test.go` | Done | Created per TDD Test Plan; verified with `ls -l` |
| `internal/plugins/dhcpserver/config_tx.go`, `internal/plugins/dhcpserver/listen_resolve_test.go` | Changed | Recorded in Deviations from Plan |
| `docs/plugin-development/metrics.md` | Changed | Already landed on HEAD in `5b59287672`; verified line 279 |

### Audit Summary
- **Total items:** 22
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Zero unannotated `error-dropped-below-warn` findings across all 26 system-plugin/ExaBGP files | Static gate (`./le arch refuse-by-default report`) | Main-thread filtered JSON report under `internal/plugins/` (excluding routing/forwarding) and `internal/exabgp/`: `0` `error-dropped-below-warn` findings, `0` `annotation-names-no-guard` findings |
| Every Answer-2 failure warns with registered doctor code and raises on report bus via `issuequeue` over `*sdk.Plugin` | Unit tests + `./le arch doctor-coverage check` + `./le plugin boundary check` | All 7 TDD unit tests (`TestNTPPostSyncAndDHCPEventErrorsAreWarnedAndRaised`, `TestDDoSDetectPersistenceAndDirectionErrorsAreWarnedAndRaised`, `TestDHCPServeRequestsReplyWriteFailureIsWarnedRaisedAndRateLimited`, `TestFlowExportDestroyListenerAndSamplingTeardownWarnAndRaise`, `TestTFTPServerSideErrorsAreWarnedAndRaised`, `TestImageServerZefsCleanupFailureIsWarnedAndRaised`, `TestVRRPPublishStateChangeEmitFailureIsWarnedAndRaised`) + `./le arch doctor-coverage check` OK + `./le plugin boundary check` OK |
| Zero wire, protocol, or functional regressions across system plugins and ExaBGP bridge | Scoped `-race` unit/userns tests + `.ci` functional tests | All 14 package suites PASS with `-race`; `./le --name b3ci test functional dhcp` 1/1 runnable PASS (`1.6s`); `./le --name b3ci test functional vrrp` 7/7 runnable PASS (`6.4s`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None — all 50 instances under this child's door are resolved | Tree-wide completion of remaining doors is tracked by the umbrella and sibling child specs | `plan/release/fixes/spec-dropped-error-0-umbrella.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/dropped-error-4-system-plugins-b3b20002-2222-4000-8000-000000000002.md` |
| `./le spec review check` | `review_gate: OK (48 code files, clean, rounds 2)` |
| Rounds | 2 (Run 1 found and fixed 3 standing-warning lifecycle ISSUEs; independent Run 2 `ze-read` review returned **CLEAN**, 0 BLOCKER, 0 ISSUE) |
| Reviewer lenses used | logic+wiring, standing-warning lifecycle, user-facing errors & security, design-documents lens, ze-go-style pass |

### Run 1 (independent `/ze-review` + `/ze-review-spec` over all 48 files)
- Automated pre-checks: `./le test weakened check` reported 0 findings on this spec's 48 files (1 unrelated finding in `internal/component/bgp/format/raw_b64_test.go`); `./le spec design check` -> `ready to present`; `./le repo check` reported 0 findings on new symbols added by this spec.
- Design-documents lens: checked all rows in `## Design documents` against the diff — 0 contradictions, 0 unrecorded departures.
- Style pass (`docs/contributing/ze-go-style.md`): verified all 7 checks across changed Go files (zero peer-reachable `panic()`, every loop/rate-limiter map/ring bounded, zero `fmt.Sprintf` in added code, paired `Start`/`Stop` lifecycle order verified across all 7 `register.go` files).
- Security & user-facing errors check: verified every `fix` string (`ze show doctor`, `ze show interface`, `ze show system ntp`, `ze show vrrp`) maps to an executable YANG `ze:command`, every `Subject` is bounded by `rpc.ReportSubjectMax`, every `Message` is bounded by `report.FitMessage` (`rpc.ReportMessageMax`), every `issuequeue` call passes a non-nil `failed` callback, and zero untrusted filenames, DHCP payloads, or SSH password hashes leak.
- Found 3 standing-warning (`RaiseWarning`) lifecycle ISSUEs; fixed all 3 at source with regression tests (below).

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Rejected strict TFTP reload dropped `warnedIfaces` in `bindTFTP`, leaving standing `doctor-tftp-interface-resolve-fallback` warning un-cleared after rollback | `internal/plugins/tftpserver/config_tx.go:188,341` | Returned `warnedIfaces` on strict error in `bindTFTP` and merged into `s.resolveWarned` before `s.restore` in `tftpServer.install`; added rollback regression check in `internal/plugins/tftpserver/config_tx_test.go` |
| 2 | ISSUE | Stopping NTP `syncWorker` on disable, reload replacement, or shutdown left standing `doctor-ntp-rtc-write-failed` and `doctor-ntp-time-persist-failed` warnings active (and replacement workers had `failures == 0`, so recovery skipped `ClearWarning`) | `internal/plugins/ntp/ntp.go:178` | Deferred `w.clearRTCFailure(logger)` and `w.clearPersistFailure(logger)` after `defer close(w.done)` in `syncWorker.run`; added worker-stop clear regression check in `internal/plugins/ntp/ntp_test.go` |
| 3 | ISSUE | Overlapping flow-export reload in `register.go:227-232` starts replacement `conntrackWorker` before stopping `prev`, so `prev.Stop()` erased the replacement worker's newly raised `doctor-flowexport-conntrack-destroy-unavailable` warning | `internal/plugins/flowexport/conntrack_worker.go:186,375` | Added `activeDestroyWarnings atomic.Int32` refcount so `Stop()` clears only when the last dump-only worker exits and `Start()` clears on success only when count is 0; added overlapping-reload regression check in `internal/plugins/flowexport/conntrack_worker_test.go` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/imageserver/register_doctor_codes.go` | Yes | `-rw-r--r-- 1 thomas thomas 1799 Nov 27 10:51 internal/plugins/imageserver/register_doctor_codes.go` |
| `internal/plugins/tftpserver/register_doctor_codes.go` | Yes | `-rw-r--r-- 1 thomas thomas 3321 Nov 27 10:51 internal/plugins/tftpserver/register_doctor_codes.go` |
| `internal/plugins/vrrp/register_doctor_codes.go` | Yes | `-rw-r--r-- 1 thomas thomas 1743 Nov 27 10:51 internal/plugins/vrrp/register_doctor_codes.go` |
| `internal/plugins/flowexport/sampling_worker_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 3305 Nov 27 11:06 internal/plugins/flowexport/sampling_worker_test.go` |
| `website/changes/fragments/dropped-error-4-system-plugins.md` | Yes | `-rw-r--r-- 1 thomas thomas 3237 Nov 27 12:55 website/changes/fragments/dropped-error-4-system-plugins.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 0 unannotated `error-dropped-below-warn` and 0 `annotation-names-no-guard` findings under system-plugins door | Filtered `./le arch refuse-by-default report '\|' json` in main checkout: 0 findings, 0 invalid guards |
| AC-2 | Every new `Warn` carries registered doctor code + raises via `issuequeue` over `*sdk.Plugin`; zero `iface.GetBackend()` calls | `./le arch doctor-coverage check` OK, `./le plugin boundary check` OK, `./le arch tier check` OK; `git diff HEAD` grep for `iface.GetBackend` returned only a doc comment line in `tftpserver/register.go` |
| AC-3 | All 15 Answer-2 sites + `sendWindow` `readBlock` warn with what/why/fix/code/`report=queued` and raise/clear cleanly; expected no-ops stay silent | 7 unit tests verified against source and re-run under `-race` after Run 1 standing-warning fixes (`tftpserver` 49.90s, `ntp` 15.25s, `flowexport` 5.33s PASS; `doctor-coverage` OK; scoped lint 0 issues) |
| AC-4 | All 14 modified packages and reached functional `.ci` suites pass with zero regressions | Main-thread `-race` unit/userns suites PASS for all 14 packages (re-verified post-Run-1 on `tftpserver`, `ntp`, `flowexport`) + `dhcp` (1/1) and `vrrp` (7/7) `.ci` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| DHCP server reply path (`runDHCPServerPlugin` -> `serveRequests`) | `test/dhcp/` (`./le --name b3ci test functional dhcp`) | Yes — 1/1 runnable PASS (`1.6s`) |
| VRRP state-change telemetry (`runVRRPEngine` -> `publishStateChange`) | `test/vrrp/` (`./le --name b3ci test functional vrrp`) | Yes — 7/7 runnable PASS (`6.4s`) |
| Plugin SDK `issuequeue` (`Start` -> `RaiseIssue`/`ClearIssue` -> `Stop`) | Driven by all 7 Answer-2 plugins' `register.go` entry points + stub `issuequeue.Publisher` tests | Yes — verified in `ddos/detect/register.go`, `dhcpserver/register.go`, `flowexport/register.go`, `imageserver/register.go`, `ntp/register.go`, `tftpserver/register.go`, `vrrp/register.go` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Filtered `./le arch refuse-by-default report '\|' json` confirmed exact 50-finding population in 26 files and 0 remaining findings after implementation |
| A-2 | confirmed | Source inspection of `bridge_ack.go`, `bridge_batch.go`, `bridge_ping.go`, `bridge_status.go`, and `bridgerun/script.go` confirmed `Bridge.Run` / `script.runOnce` reap child exit |
| A-3 | confirmed | `./le arch doctor-coverage check` OK and all 7 Answer-2 plugins deliver raises through `issuequeue.Start(p)` over `*sdk.Plugin` |
| A-4 | confirmed | `tftpserver/register.go` `isNoBackendLoaded` uses `errors.Is(err, iface.ErrNoBackendLoaded)`; `./le plugin boundary check` OK |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Metrics (`docs/plugin-development/metrics.md` line 279) | `ze_dhcp_reply_write_errors_total` row present on HEAD with anchor to `internal/plugins/dhcpserver/register.go -- configureMetrics, replyWriteErrorsCounter, serveRequests`; `./le doc ste check file` OK | Yes |
| Operator news fragment (`website/changes/fragments/dropped-error-4-system-plugins.md`) | Created with valid front matter (`spec`, `topic: Diagnostics`, `found-by: review`) and 6 headings; `./le doc ste check file` OK | Yes |
| External spec citations (`plan/release/fixes/spec-dropped-error-0-umbrella.md:82`, `features/lan-dns-resolver.md:19`) | Updated to bare stem `spec-dropped-error-4-system-plugins`; `git grep 'plan/release/fixes/spec-dropped-error-4-system-plugins\.md'` returns only self-references in this spec | Yes |
| Existing architecture doc anchors (`./le spec citation anchors`) | Wire, protocol, and documented fallback behaviors (`dhcp-server.md`, `container/network.md`, `tftp-server.md`, `flow-export-2-flow-records.md`, `exabgp-bridge.md`) preserved unchanged | Yes |
