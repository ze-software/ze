# Spec: dropped-error-3-access-security

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | 44 active errors in IKE, L2TP/PPP/PPPoE, RADIUS and TACACS+ are caught and logged only below WARN (hiding XFRM rollback failures, Child SA removal/event failures, established IKE send/build failures, NAT-T keepalive drops, PPP RA send failures, per-packet malformed wire counts, and RADIUS EAP concluding mismatches); resolving them reaches zero unannotated `error-dropped-below-warn` findings under the access-security door |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/access-security.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under the door's base paths
(`internal/component/{aaa,authz,tacacs,radius,pki,ike,l2tp}`,
`internal/core/{ipsecinventory,ikeprobe,eap,audit}`), excluding
`internal/component/ike/engine/mobike.go` (two sites moved to
`plan/post-release/features/spec-ipsec-11-mobike.md`, Known Issues #3, per the
owner ruling of 2026-11-23 moving MOBIKE to post-release).

Re-verified against HEAD on 2026-11-26: **44 active in-scope instances in 22
files** (down from 60 instances in 30 files on 2026-11-02; 17 instances across
9 files were already resolved or removed on `main`, 2 MOBIKE sites moved to
post-release, and 2 new sites were added in `l2tp/plugins/authradius/handler.go`):
- IKE dataplane: 1 instance in 1 file (`xfrm_linux.go`)
- IKE engine: 26 instances in 13 files (`certurl.go`, `child.go`, `delete.go`,
  `established.go`, `fsm.go`, `inbound.go`, `notify_error.go`,
  `notify_invalid_msgid.go`, `notify_suspicious.go`, `probe.go`, `register.go`,
  `responder.go`, `spd_policy.go`)
- IKE transport: 1 instance in 1 file (`keepalive.go`)
- L2TP / PPP / PPPoE / authradius: 10 instances in 5 files (`authradius/handler.go`,
  `authradius/register.go`, `ppp/ra_send.go`, `ppp/session_run.go`,
  `pppoe/panic_guard.go`)
- RADIUS: 1 instance in 1 file (`authenticator_eap.go`)
- TACACS+: 5 instances in 1 file (`client.go`)

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid |
| 2 | Going on is right and an operator can act | Log at `Warn` with what, why and the fix (`ai/rules/user-facing-errors.md`), emitting a registered doctor `code` from the owning package's `register_doctor_codes.go` (or existing package doctor registration) and raising transient events on the daemon's report bus (`./le arch doctor-coverage check`): forkable `ike` (`ike/engine`, `ike/dataplane`, `ike/transport`) enqueues non-blockingly onto a plugin-owned `issuequeue.Queue` (`internal/component/plugin/issuequeue`, started in `ike/engine/register.go` over `p` via `issuequeue.Start(p)` and stopped after engine shutdown; callback sinks from `ike/dataplane` and `ike/transport` enqueue onto that queue; periodic once-per-condition sites track retries with `issuequeue.RunRaises` and every `Warn` carries `issuequeue.Outcome`, never calling `internal/core/report` directly and never blocking packet-dispatch, lock-holding, or periodic goroutines on synchronous RPC), while daemon-local components (`radius`, `l2tp/ppp`, and internal-only `authradius` which enforces `!p.IsInternal()`) raise directly via `internal/core/report`; on a per-packet or periodic path, count it in the subsystem's existing telemetry family and/or WARN once per condition |
| 3 | Going on is right and nobody can act (a close at teardown, an error reply to an unauthenticated or gone peer, an expired auth wait) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above, naming a visible symbol other than the enclosing function |

### Instance Classification at HEAD (2026-11-26)

| File and site(s) | Count | Answer | Concrete resolution |
|------------------|-------|--------|---------------------|
| `internal/component/ike/dataplane/xfrm_linux.go:156` (`xfrmBackend.InstallSA`) | 1 | 2 | Log `Warn` with doctor code in `ike/dataplane/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome` naming SPI, stranded XFRM state, cause, and `ze show vpn ipsec dataplane`; enqueue non-blockingly via a callback sink registered by `ike/engine/register.go` onto the `ike` `issuequeue.Queue` (parallel to `dataplane.ESPFormReaderStoppedSink`, never blocking `childLifecycleMu`/`beginDataplaneWrite()` on synchronous RPC and never calling `internal/core/report`); join rollback failure into returned install error |
| `internal/component/ike/engine/certurl.go:335` (`certURLFetcher.start.func`) | 1 | 2 | Log `Warn` once per backoff window (already bounded by `certURLFailBackoff`) with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing onto the `ike` `issuequeue.Queue` with `issuequeue.RunRaises` (within `rpc.ReportRaisesPerWindow`), naming cause and fix |
| `internal/component/ike/engine/child.go:1574,1609` (`removeChildSAOutgoing`, `removeChildSAIncoming`) | 2 | 2 / 3 | Already-gone (`ENOENT`/`ESRCH`): Answer 3 guard annotation naming `removeChildSAExcept`; other kernel removal refusal: Answer 2 `Warn` with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly onto the `ike` `issuequeue.Queue` (coalesced per peer/condition within `rpc.ReportRaisesPerWindow`), naming SPI, direction, stranded XFRM state, and `ze show vpn ipsec dataplane` |
| `internal/component/ike/engine/delete.go:59,200` (`PeerSession.writeDelete`, `sendIKESATeardown`) | 2 | 2 | Log `Warn` with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly onto the `ike` `issuequeue.Queue` (coalesced per peer/condition within `rpc.ReportRaisesPerWindow`), naming peer, failed Delete/teardown build, impact (peer waits for liveness timeout), and `ze show vpn ipsec sa`; keep `sa.releaseRequestWindow()` |
| `internal/component/ike/engine/established.go:1008` (`sendRaw`) | 1 | 2 | Count send failure in `IPsecMetrics` (`ike/engine/metrics.go`) and log `Warn` once per SA failure condition (cleared on next successful send) with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing once per condition onto the `ike` `issuequeue.Queue` with `issuequeue.RunRaises` (within `rpc.ReportRaisesPerWindow`) |
| `internal/component/ike/engine/established.go:1219,1240,1261` (`emitChildUp`, `emitChildDown`, `emitChildRekey`) | 3 | 2 | Log `Warn` with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly onto the `ike` `issuequeue.Queue` (coalesced per peer/condition within `rpc.ReportRaisesPerWindow`), naming peer, lifecycle event, SPI, why (bus subscribers missed Child SA state change), and `ze show vpn ipsec sa` |
| `internal/component/ike/engine/fsm.go:580` (`handleInbound`) | 1 | 2 (count) | Per-packet untrusted wire parse error: count in `IPsecMetrics` (`ike/engine/metrics.go`) and record on a counter field or guard annotation naming `deliverDatagram` (no per-packet `Warn` flood; preserves RFC 7296 Section 2.4 so malformed frames never kill a live SA) |
| `internal/component/ike/engine/inbound.go:852` (`PeerSession.handleInformationalOwned`) | 1 | 2 | Log `Warn` with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly onto the `ike` `issuequeue.Queue` (coalesced per peer/condition within `rpc.ReportRaisesPerWindow`), naming peer, failed INFORMATIONAL response build, impact (peer DPD timeout), and `ze show vpn ipsec sa` |
| `internal/component/ike/engine/notify_error.go:175` (`PeerSession.respondError`) | 1 | 2 | Established-SA error notify build failure: log `Warn` with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly onto the `ike` `issuequeue.Queue` (coalesced per peer/condition within `rpc.ReportRaisesPerWindow`), while keeping `countErrorNotifySuppressed` |
| `internal/component/ike/engine/notify_error.go:316,326` (`sendInvalidIKESPI`) | 2 | 3 | Unauthenticated out-of-SA `INVALID_IKE_SPI` reply already counted via `countErrorNotifySuppressed`: annotate with `// refuse-by-default: guards answerOutOfSA -- ...` |
| `internal/component/ike/engine/notify_invalid_msgid.go:111` (`PeerSession.sendInvalidMessageID`) | 1 | 2 | Established-SA notify build failure: log `Warn` with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly onto the `ike` `issuequeue.Queue` (coalesced per peer/condition within `rpc.ReportRaisesPerWindow`), while keeping `sa.releaseRequestWindow()` and `countErrorNotifySuppressed` |
| `internal/component/ike/engine/notify_suspicious.go:239` (`PeerSession.informSuspicious`) | 1 | 2 | Established-SA notify build failure: log `Warn` with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly onto the `ike` `issuequeue.Queue` (coalesced per peer/condition within `rpc.ReportRaisesPerWindow`), while keeping `sa.releaseRequestWindow()` and `countErrorNotifySuppressed` |
| `internal/component/ike/engine/probe.go:336` (`PeerSession.sendProbeDFClear`) | 1 | 2 | Count and log `Warn` once per probe/condition with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing once per condition onto the `ike` `issuequeue.Queue` with `issuequeue.RunRaises` (within `rpc.ReportRaisesPerWindow`), naming peer, failed DF-clear probe send, and `ze show vpn ipsec sa` |
| `internal/component/ike/engine/register.go:684` (`closeSDK`) | 1 | 3 | Deferred plugin SDK close on shutdown: annotate with `// refuse-by-default: guards runEngine -- ...` |
| `internal/component/ike/engine/responder.go:175,342,512,1347,1364` (`replayCachedResponse`, `resendResponderSAInit`, `sendSAInitNotifyRaw`, `respondAuthError` send, `sendAuthFailed` send) | 5 | 3 | Unauthenticated or pre-establishment UDP reply send failure (peer retransmits or is already gone; already counted where `countErrorNotifySuppressed` applies): annotate with `// refuse-by-default: guards <Caller> -- ...` |
| `internal/component/ike/engine/responder.go:1338,1360` (`PeerSession.respondAuthError` build, `PeerSession.sendAuthFailed` build) | 2 | 2 | Internal SK build failure for a configured peer on packet dispatch: log `Warn` with doctor code in `ike/engine/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly onto the `ike` `issuequeue.Queue` (coalesced per peer/condition within `rpc.ReportRaisesPerWindow`, never blocking packet dispatch on daemon RPC), naming peer, cause, and `ze show vpn ipsec sa` |
| `internal/component/ike/engine/spd_policy.go:708` (`removeSPDPolicies`) | 1 | 2 / 3 | Already-gone (`policyGone(err)`): Answer 3 guard annotation naming `releaseSPD`; non-ENOENT shutdown removal failure: explicitly register `codeDoctorIPsecSPDUndoIncomplete` via `diagnostic.RegisterEventCode` in `ike/engine/register_doctor_codes.go` (matching `register_forward_default.go:18`), log `Warn` with `codeDoctorIPsecSPDUndoIncomplete` and `issuequeue.Outcome`, and enqueue onto the `ike` `issuequeue.Queue` (drained at shutdown before `p.Close()`) naming policy, direction, stranded kernel SPD entry, and `ze show vpn ipsec dataplane` |
| `internal/component/ike/transport/keepalive.go:61` (`Keepalive.Run`) | 1 | 2 | Periodic RFC 3948 NAT-T keepalive send failure: log `Warn` once per failure condition (reset on next successful send) with doctor code in `ike/transport/register_doctor_codes.go` (+ `RegisterEventCode`) and `issuequeue.Outcome`, enqueueing non-blockingly via a callback sink wired from `ike/engine/register.go` onto the `ike` `issuequeue.Queue` with `issuequeue.RunRaises` (parallel to `dataplane.ESPFormReaderStoppedSink`, well within `rpc.ReportRaisesPerWindow`, never blocking the keepalive ticker on daemon RPC and never calling `internal/core/report`), naming remote endpoint, NAT-binding expiry risk, and `ze show vpn ipsec sa` |
| `internal/component/l2tp/plugins/authradius/handler.go:232,240` (`radiusAuth.doRADIUS`) | 2 | 3 / 2 | Expired or already-decided auth wait (`errors.Is(err, ppp.ErrRequestExpired)`): Answer 3 guard annotation naming `EventAuthRequest`; unexpected non-expired `StageFramedMTU` error: Answer 2 `Warn` with existing `ppp.CodeDoctorRADIUSFramedMTU` and `report.RaiseErrorFixOnce` (valid on the daemon bus because `authradius` enforces internal-only mode via `!p.IsInternal()` at `register.go:136`) |
| `internal/component/l2tp/plugins/authradius/register.go:132` (`runPlugin.func`) | 1 | 3 | Deferred plugin SDK close on shutdown: annotate with `// refuse-by-default: guards runPlugin -- ...` |
| `internal/component/l2tp/ppp/ra_send.go:61` (`raSender.send`) | 1 | 2 / 3 | Abrupt teardown (`normalIPv6CarrierStop` / interface removed): Answer 3 guard annotation naming `raSenderLoop`; live send failure: Answer 2 once per failure condition (cleared on next successful send) via existing `reportIPv6Failure` / `codePPPIPv6Service` (`internal/component/l2tp/ppp/diagnostic_ipv6.go`, daemon-local `internal/core/report`) |
| `internal/component/l2tp/ppp/session_run.go:1311,1334,1353,1370,1795` (`pppSession.handleFrame`, `pppSession.handleLCPPacket`) | 5 | 2 (count) | Per-packet untrusted subscriber malformed PPP/LCP/IPCP/IPv6CP/Echo frames: count in `pppMetrics` (`internal/component/l2tp/ppp/metrics.go`, closed protocol/reason label set with no per-session label) and record counter step / guard annotation (no per-frame `Warn` flood) |
| `internal/component/l2tp/pppoe/panic_guard.go:86` (`Subsystem.dispatchDiscovery`) | 1 | 2 (count) | Per-packet untrusted L2 malformed discovery frame: increment a dedicated `ze_pppoe_discovery_parse_errors_total` counter in `internal/component/l2tp/pppoe/metrics.go` (separate from `ze_pppoe_discovery_refusals_total`, which holds only well-formed discovery refusals per `bng-5-pppoe.md`) with guard annotation naming `HandleDiscovery` (no per-frame `Warn` flood) |
| `internal/component/radius/authenticator_eap.go:118` (`radiusAuthenticator.authenticateEAP`) | 1 | 2 | Concluding `Access-Accept`/`Access-Reject` where the peer rejected the encapsulated EAP packet: raise from `Info` to `Warn` with doctor code in `radius/register_doctor_codes.go` (+ `RegisterEventCode` + daemon-local `report.RaiseErrorFixOnce`), rename `"code", resp.Code` to `"radius-code", resp.Code` so `"code"` carries the doctor code, keep `return a.result(resp, username)` per RFC 3579 Section 2.6.3 |
| `internal/component/tacacs/client.go:144,608,720,726,743` (`TacacsClient.Close`, `trySend`, `storeConn`, `closeAndEvict`) | 5 | 3 | Teardown, post-exchange non-reusable close, post-close discard, pool replacement, and error-eviction `net.Conn.Close()` calls: annotate each with `// refuse-by-default: guards <VisibleSymbol> -- ...` so a close error never alters a settled AAA answer or turns into an explicit rejection |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`
  → Constraint: every user-visible message names the problem, why it matters, and the exact fix; every forkable plugin (including `ike`) runs both internal and forked, so it MUST NOT call `internal/core/report` directly and MUST raise problems through `sdk.Plugin.RaiseIssue`.
- [ ] `ai/doors/access-security.md`
  → Constraint: a backend that cannot build degrades and never locks the operator out; explicit rejection stops the AAA chain while a connection error moves to the next backend; an inbound datagram that fails to decrypt or parse never kills a live IKE SA (RFC 7296 Section 2.4).
- [ ] `ai/digests/ipsec-ike.md` - IKE SA negotiation, rekey, XFRM install
  → Constraint: XFRM install and remove failures are per SA; a WARN names the SA/SPI and peer.
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: every `Warn` or `Error` log line names what, why, and the exact fix, carries a registered doctor code under its `"code"` attribute (reserving `"code"` exclusively for doctor codes), and raises transient problems on the daemon's report bus.
- [ ] `docs/architecture/api/process-protocol.md` - "Plugin problems"
  → Constraint: a forked plugin raises transient problems with `report-issue` (`sdk.Plugin.RaiseIssue`) under a registered `EventCode`, bounded by `rpc.ReportRaisesPerWindow` (60 raises per minute per plugin); `internal/core/report` in a forked plugin's own process is never read by the daemon.
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Every problem site names a doctor code" and "Event codes"
  → Constraint: `./le arch doctor-coverage check` and `./le commit create` refuse any newly added `Warn`/`Error` line without a covered doctor code; transient event codes register via `diagnostic.Register` in `register_doctor_codes.go` and `diagnostic.RegisterEventCode` so `recent-events` declares them; plugins meeting problems on packet-dispatch, lock-holding, or periodic goroutines raise through `internal/component/plugin/issuequeue` (`issuequeue.Start(p)`, `Queue`, `Outcome`, `RunRaises`) rather than blocking on synchronous `sdk.Plugin.RaiseIssue`.
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the guard annotation names a visible symbol other than the enclosing function.

**Key insights:**
- `ike` (`internal/component/ike/engine/register.go`) runs both internal and forked (`ze plugin ike`) and already sends `report-issue` over `sdk.Plugin.RaiseIssue` with zero calls to `internal/core/report`. Because packet-dispatch (`responder.go:1338,1360`), lock-holding (`xfrm_linux.go:156` holding `childLifecycleMu` + `beginDataplaneWrite()`), and periodic (`keepalive.go:61`, `established.go:1008`, `probe.go:336`) goroutines MUST NOT block waiting on daemon RPC answers, `ike/engine/register.go` starts one plugin-owned `issuequeue.Queue` (`internal/component/plugin/issuequeue`) over `p`, stops it after engine shutdown and before `p.Close()`, and routes all new `ike/engine`, `ike/dataplane`, and `ike/transport` raises non-blockingly through that queue (using `issuequeue.RunRaises` for periodic once-per-condition sites).
- `dispatch_report.go:82` refuses any `report-issue` whose code is not registered via `diagnostic.RegisterEventCode`. Because `codeDoctorIPsecSPDUndoIncomplete` is currently declared only on `ipsec-spd-install` (`doctor_spd_install.go:34`), `ike/engine/register_doctor_codes.go` MUST also register `codeDoctorIPsecSPDUndoIncomplete` via `diagnostic.RegisterEventCode` (matching `register_forward_default.go:18`) so `removeSPDPolicies` (`spd_policy.go:708`) can raise it over `report-issue`.
- `recordsMiss` in `internal/le/arch/refusebydefault/refusebydefault.go` recognizes direct struct-field/index assignments, `++`, or statements mentioning `err`, but does NOT recognize a bare helper call such as `countErrorNotifySuppressed("build-failed")`; counted per-packet discards that call a helper therefore pair the counter with a field step or a `// refuse-by-default: guards <Symbol> -- <why>` annotation.
- `internal/component/radius/authenticator_eap.go:120` currently passes `"code", resp.Code` on its `Info` line; raising it to `Warn` requires moving `resp.Code` to `"radius-code"` so `"code"` carries the doctor code required by `./le arch doctor-coverage check`.
- `docs/architecture/l2tp/bng-5-pppoe.md` defines `ze_pppoe_discovery_refusals_total` with a closed 6-value `reason` vocabulary for well-formed packets; malformed discovery frames in `panic_guard.go:86` therefore belong on a separate `ze_pppoe_discovery_parse_errors_total` counter, parallel to `ze_pppoe_discovery_read_errors_total`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` and `refusebydefault.go` - `droppedError`, `recordsMiss`, `mentions`
- [ ] `internal/component/plugin/issuequeue/{queue,raises}.go` - `Start`, `Queue`, `Outcome`, `RunRaises`
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` (`:156`), `espform_state.go` (`ESPFormReaderStoppedSink`), and `register_doctor_codes.go`
- [ ] `internal/component/ike/engine/{certurl,child,delete,established,fsm,inbound,notify_error,notify_invalid_msgid,notify_suspicious,probe,register,responder,spd_policy}.go`, `register_esp_form_reader.go`, `register_forward_default.go`, `register_mobike_socket.go`, `metrics.go`, and `register_doctor_codes.go`
- [ ] `internal/component/ike/transport/keepalive.go` (`:61`) and `register_doctor_codes.go`
- [ ] `internal/component/l2tp/plugins/authradius/{handler,register,doctor_codes,extract_mtu}.go`
- [ ] `internal/component/l2tp/ppp/{ra_send,session_run,request,diagnostic_ipv6,metrics,register_doctor_codes}.go`
- [ ] `internal/component/l2tp/pppoe/{panic_guard,metrics,register_doctor_codes}.go`
- [ ] `internal/component/radius/{authenticator_eap,register_doctor_codes}.go`
- [ ] `internal/component/tacacs/client.go` (`:144,608,720,726,743`)

**Behavior to preserve:**
- IKEv2, L2TP, PPP, PPPoE, RADIUS and TACACS+ wire behavior and RFC state machines.
- RFC 7296 Section 2.4 invariant: malformed or unprotected inbound packets never tear down a live IKE SA.
- RFC 3579 Section 2.6.3 invariant: RADIUS Access-Accept/Access-Reject decides operator login even when the concluding EAP packet fails peer verification.
- AAA chain order and rejection semantics: TACACS+ close/teardown errors never turn into `aaa.ErrAuthRejected` or alter a settled reply.
- Forked-plugin portability and non-blocking hot paths for `ike`: zero direct `internal/core/report` calls and zero synchronous `sdk.Plugin.RaiseIssue` waits from packet-dispatch, lock-holding, or periodic goroutines in `ike/engine`, `ike/dataplane`, or `ike/transport`.
- Closed label vocabularies on existing Prometheus metrics (`ze_pppoe_discovery_refusals_total`, `pppMetrics` with no per-session labels).

**Behavior to change:**
- Every one of the 44 active `error-dropped-below-warn` instances across the 22 files is resolved by returning/joining the error, logging a doctor-coded `Warn` (+ non-blocking report-bus event via plugin-owned `issuequeue.Queue` / `sdk.Plugin.RaiseIssue` for forkable `ike` or `internal/core/report` for daemon-local components, rate-bounded on periodic/hot paths), counting on the subsystem's metric family, or carrying a valid `// refuse-by-default: guards <Symbol> -- <why>` annotation.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error detector | `internal/le/arch/refusebydefault/dropped.go` (`droppedError`) | Lists the 44 active in-scope findings (and 2 post-release MOBIKE findings) under the access-security door | Applies unchanged: acceptance check for AC-1 |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts `// refuse-by-default: guards <Symbol> -- <why>` on the shape's line or the line above | Applies unchanged: used for Answer 3 teardown, expired-wait, and unauthenticated-reply sites |
| Doctor-coverage gate and event-code registry | `docs/architecture/doctor-and-health-checks.md` ("Every problem site names a doctor code", "Event codes"); owning-package `register_doctor_codes.go` + `diagnostic.RegisterEventCode` | Requires every new `Warn`/`Error` log line to emit `"code"` naming a declared doctor code and transient classes to raise on the daemon's report bus under a `RegisterEventCode`-registered code | Extends: adds new doctor codes and `RegisterEventCode` calls in `ike/dataplane`, `ike/engine` (including registering existing `codeDoctorIPsecSPDUndoIncomplete` as an event code), `ike/transport`, and `radius` `register_doctor_codes.go` files |
| Non-blocking plugin report queue (`issuequeue`) | `internal/component/plugin/issuequeue` (`issuequeue.Start`, `Queue`, `Outcome`, `RunRaises`); `docs/architecture/doctor-and-health-checks.md` ("Event codes") | Bounded (256-entry) per-plugin worker queue over `sdk.Plugin.RaiseIssue`/`ClearIssue` with 1s per-report timeout, 3s stop drain, `Outcome` log attributes (`report=queued` / `report-error`), and `RunRaises` retry tracking for periodic once-per-condition runs; used by BFD, OSPF, IS-IS, LDP, static, redistribute, and BGP RS so packet-dispatch, lock-holding, and periodic goroutines never wait on daemon RPC | Extends: `ike/engine/register.go` starts one plugin-owned `issuequeue.Queue` over `p` and stops it after engine shutdown (before `p.Close()`); all new `ike/engine` raises enqueue onto it, `ike/dataplane` (`xfrm_linux.go:156`) and `ike/transport` (`keepalive.go:61`) enqueue via callback sinks registered by `ike/engine/register.go`, and periodic once-per-condition sites use `issuequeue.RunRaises` |
| Existing IKE SDK report-bus raisers and dataplane sink | `internal/component/ike/engine/register_esp_form_reader.go` (`espFormIssueRaiser`, `espFormReaderStoppedSink`), `register_forward_default.go` (`raiseForwardDefaultRefusal`), `register_mobike_socket.go` (`reportMobikeSocketNotShared`), `internal/component/ike/dataplane/espform_state.go` (`ESPFormReaderStoppedSink`, `RegisterESPFormReaderStopped`) | Demonstrates the `ike` callback-sink pattern decoupling `ike/dataplane` from `pkg/plugin/sdk` and raises `report-issue` synchronously from background/apply paths | Applies (sink pattern) / does not call synchronously on hot paths: new `ike/dataplane` and `ike/transport` sinks mirror `ESPFormReaderStoppedSink` registration, but enqueue onto `ike`'s `issuequeue.Queue` instead of blocking on synchronous `RaiseIssue` |
| Existing IKE SPD undo doctor code | `internal/component/ike/engine/register_doctor_codes.go` (`codeDoctorIPsecSPDUndoIncomplete`), `doctor_spd_install.go` | Currently declared only on the `ipsec-spd-install` runtime check and not yet registered via `diagnostic.RegisterEventCode` | Extends: registers `codeDoctorIPsecSPDUndoIncomplete` via `diagnostic.RegisterEventCode` in `ike/engine/register_doctor_codes.go` so `dispatch_report.go` admits it over `report-issue`, and emits + enqueues it on non-ENOENT failures in `removeSPDPolicies` (`spd_policy.go:708`) |
| Existing PPP IPv6 service diagnostic | `internal/component/l2tp/ppp/diagnostic_ipv6.go` (`codePPPIPv6Service`, `reportIPv6Failure`, `normalIPv6CarrierStop`) | Logs `Warn` with `codePPPIPv6Service`, raises on daemon-local `internal/core/report`, and classifies normal teardown stops | Extends: reused by `raSender.send` (`ra_send.go:61`) once per live send-failure condition |
| Existing RADIUS Framed-MTU doctor code | `internal/component/l2tp/ppp/register_doctor_codes.go` (`CodeDoctorRADIUSFramedMTU`) and `authradius/register.go:109` | Registered as an event code for subscriber Framed-MTU failures in internal-only `authradius` | Extends: reused by `authradius/handler.go:240` for any unexpected non-expired `StageFramedMTU` failure |
| Subsystem Prometheus metric families | `internal/component/ike/engine/metrics.go` (`IPsecMetrics`), `internal/component/l2tp/ppp/metrics.go` (`pppMetrics`), `internal/component/l2tp/pppoe/metrics.go` (`pppoeMetrics`) | Publish push-model counters via `metrics.Registry` with closed low-cardinality labels | Extends: adds per-packet malformed/send counters in existing families without per-session labels and without widening `ze_pppoe_discovery_refusals_total` |
| Operational report bus (daemon-local) | `internal/core/report` (`RaiseErrorFix`, `RaiseErrorFixOnce`, `RaiseWarningFix`) | Stores transient errors and active warnings in the daemon process for `show errors`, `show warnings`, and `show doctor` (`recent-events`) | Extends for daemon-local components only (`radius`, `l2tp/ppp`, and internal-only `authradius`); forkable `ike` uses `issuequeue.Queue` over `sdk.Plugin.RaiseIssue` above so forked runs reach the daemon's bus |
| Retired 2026-11-02 sites already resolved at HEAD (17 instances in 9 files) | `ike/dataplane/espform_linux.go` (5, fixed in `abeb7d6535`/`99a55f480f` with Warn + doctor codes); `ike/engine/bypass.go` (1, removed in `e700fb7392`); `ike/engine/redistribute.go` (1, now `Warn` at `:286`); `ike/engine/spd_policy.go` `installSPDPolicies` (1, fixed in `6cf8d9289d`); `ike/engine/unmatched.go` (1, fixed in `51799ad534`/`827c73e9e1`); `l2tp/ppp/dhcpv6_linux.go` and `ra_linux.go` (4, removed in `59f67661db`); `l2tp/pppoe/server.go` (1, refactored to `submitFrame() bool`); `l2tp/session_timeout.go` (2, refactored to `teardownCapturedSession() bool` raising `doctor-l2tp-control-send`) | Already eliminated from `./le arch refuse-by-default report` on `main` | Does not apply: retired from this spec's edit set; verified absent in the 2026-11-26 report |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Every message a user sees names the problem, why, and the exact fix" | agrees | Every new `Warn` message added under Answer 2 names what failed, why it matters operationally, and the exact operator command to inspect or fix it |
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Forkable `ike` (`ike/engine`, `ike/dataplane`, `ike/transport`) raises every transient problem non-blockingly through a plugin-owned `issuequeue.Queue` over `sdk.Plugin.RaiseIssue` (and registered callback sinks) so forked `ze plugin ike` reaches the daemon's report bus identically to internal mode; direct `internal/core/report` calls are restricted to daemon-local `radius`, `l2tp/ppp`, and internal-only `authradius` |
| `ai/doors/access-security.md` | Boundaries | the door covering every modified file under `ike`, `l2tp`, `radius`, `tacacs` | "Explicit rejection stops the chain. A connection error moves to the next backend." | agrees | TACACS+ close errors use Answer 3 guard annotations and never become `aaa.ErrAuthRejected`; RADIUS concluding EAP mismatches log at `Warn` and still return `a.result(resp, username)` |
| `docs/architecture/aaa-tacacs.md` | Reject and unreachable are different, and the chain owns the difference | anchored to `internal/component/tacacs/client.go` | "`aaa.ErrAuthRejected` stops the chain. Any other error tries the next backend." | agrees | Annotating the five `net.Conn.Close()` teardown/eviction sites in `tacacs/client.go` preserves all error classification and AAA chain fallthrough semantics |
| `docs/architecture/api/process-protocol.md` | Plugin problems | anchored to `internal/component/ike/engine/register.go` | "A forked plugin has its own copy of `internal/core/report` in its process, which nothing reads, so it raises with `report-issue`; the SDK's `Plugin.RaiseIssue` and `Plugin.ClearIssue` send it on both transports." | agrees | All transient events from forkable `ike` (`ike/engine`, `ike/dataplane`, and `ike/transport`) enqueue onto a plugin-owned `issuequeue.Queue` (`sdk.Plugin.RaiseIssue`) under codes explicitly registered via `diagnostic.RegisterEventCode` (including `codeDoctorIPsecSPDUndoIncomplete`), coalesced with `issuequeue.RunRaises` within `rpc.ReportRaisesPerWindow` (60/min), never calling `internal/core/report` directly; daemon-local `radius`, `l2tp/ppp`, and internal-only `authradius` keep using `internal/core/report` |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operation, RPC message, or plugin authorization rule; `ike` reuses the existing `report-issue` operation via `sdk.Plugin.RaiseIssue` and deferred SDK close annotations leave plugin authorization untouched |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | `emitChildUp`, `emitChildDown`, and `emitChildRekey` make event-emit failures visible at `Warn` with a doctor code instead of hiding them at `Debug`, without introducing any queue-overflow drop |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `internal/component/l2tp/plugins/authradius/register.go` | - | silent | This page governs config verify/apply/rollback transactions; this spec changes no config transaction handling and only annotates deferred SDK close on plugin teardown in `authradius/register.go` |
| `docs/architecture/core-design.md` | 15. Operational Report Bus | anchored to `internal/component/ike/engine/probe.go` | "Warnings are state-based (condition is currently problematic, may resolve). Errors are event-based (something already happened; no clear API). Producers pick deliberately; the bus does not auto-promote." | agrees | Transient failure events raised by new `Warn` sites use error-event severity (`rpc.ReportError` via `sdk.Plugin.RaiseIssue` for forkable `ike`; `report.RaiseErrorFix` / `report.RaiseErrorFixOnce` for daemon-local components) rather than creating unowned state warnings |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (VPP kernel mismatch at commit) | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec touches no data-plane compatibility gate, VPP/kernel selector, or commit validation logic |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (mismatch refused at start/commit/reload) | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec changes no startup, commit, reload, or config-validation mismatch refusal |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (VPP firewall verifier) | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches no firewall verifier, flowtable, or YANG backend annotation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (data-plane selector naming) | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec adds or modifies no data-plane selector or YANG backend naming |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (BGP egress next-hop in export chain) | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches only access-security packages and does not affect BGP export chains or next-hop rewriting |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (next-hop authoring forms) | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec touches no BGP policy chain or `policy modify` authoring surface |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (built-in next-hop engine steps) | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec touches no BGP engine filter compilation or config loading |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (next-hop folding and raw override provenance) | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec touches no BGP UPDATE forwarding or raw override provenance |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (explicit IPv4/IPv6 next-hop on import and export) | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec touches no BGP import/export policy evaluation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (trailing next-hop:auto on forwarded routes only) | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec touches no BGP route origination or forwarding |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (implied-filter seam) | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec touches no filter-chain insertion or `\| implied` display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 (Ze always starts) | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Every new failure surfaced by this spec logs under a registered doctor code and reports via `ze show doctor` (`recent-events`) without causing `ze start` to abort |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 (VPP/kernel mismatch at daemon start) | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec introduces no startup refusal and reports runtime access-security failures under registered doctor codes |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/ike/engine/metrics.go` | "The Prometheus implementation should continue to use the existing push-model `metrics.Registry` path." | agrees | All new per-packet malformed/send counters in `ike/engine/metrics.go`, `l2tp/ppp/metrics.go`, and `l2tp/pppoe/metrics.go` use the existing push-model `metrics.Registry` pattern |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | `// Design:` page of `register_doctor_codes.go` files; anchored to `spd_policy.go`, `authradius/register.go` | "A site is covered when its code is one that a doctor check declares." | agrees | Every new `Warn` log site emits `"code"` carrying a constant registered in its owning package's `register_doctor_codes.go` (or existing doctor code) and declared by `diagnostic.RegisterEventCode` (`recent-events`, including `codeDoctorIPsecSPDUndoIncomplete`), raising transient events non-blockingly via `issuequeue.Queue` (`sdk.Plugin.RaiseIssue`) for forkable `ike` and `internal/core/report` for daemon-local components |
| `docs/architecture/doctor-and-health-checks.md` | Event codes | `// Design:` page of `register_doctor_codes.go` files; anchored to `spd_policy.go`, `authradius/register.go` | "`RaiseIssue` waits for the daemon's answer, and a plugin meets its problems on goroutines that must not wait: the BFD express loop, a Hello sender, code that holds a lock every received packet takes. So a plugin raises and clears through one report queue, `internal/component/plugin/issuequeue`, which its SDK entry starts over the SDK plugin and stops on exit, after its engines have stopped so their final clears are sent." | agrees | `ike/engine/register.go` starts one plugin-owned `issuequeue.Queue` over `p` (`issuequeue.Start(p)`) and stops it after engine shutdown and before `p.Close()`; packet-dispatch (`responder.go:1338,1360`), lock-holding (`xfrm_linux.go:156`), and periodic (`keepalive.go:61`, `established.go:1008`, `probe.go:336`, `certurl.go:335`) goroutines only enqueue onto that queue (with `issuequeue.Outcome` on `Warn` lines and `issuequeue.RunRaises` on periodic once-per-condition runs) and never block on synchronous `RaiseIssue` |
| `docs/architecture/ike/ipsec-3-data-model.md` | Decisions | anchored to `internal/component/ike/engine/responder.go` | - | silent | This page governs the typed IPsec configuration model; this spec changes no config schema or data-model types, only error visibility in `responder.go` |
| `docs/architecture/ike/ipsec-6-ikev2-crypto.md` | Certificate payload handling | `// Design:` page of `internal/component/ike/engine/certurl.go`; anchored to `responder.go` | "The CERT and CERTREQ surfaces sit in the engine package rather than in crypto, because they need the SA and the PKI store." | agrees | The hash-and-URL background fetch `Warn` remains in `ike/engine/certurl.go`, bounded by the existing backoff window, and changes no certificate validation or crypto boundary |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | `// Design:` page of `fsm.go`, `notify_error.go`, `notify_suspicious.go`, `register.go`; anchored to `child.go`, `established.go`, `inbound.go`, `probe.go`, `responder.go`, `spd_policy.go` | "Wire parsing, authentication and unsupported-input refusals are unchanged. In particular, an admitted inbound classification still passes through decryption and authentication before it acts on the message." | agrees | Counting malformed inbound packets in `fsm.go:580` and raising established-SA notify build errors to `Warn` leaves wire parsing, decryption, and authentication order unchanged |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | INFORMATIONAL requests, the window and retransmission | `// Design:` page of `xfrm_linux.go`, `child.go`, `delete.go`, `established.go`, `inbound.go`, `probe.go`, `spd_policy.go`; anchored to `fsm.go`, `metrics.go`, `register.go`, `register_doctor_codes.go`, `responder.go` | "It releases no window without a response and rewinds no message id." | agrees | Raising build/send failures in `delete.go`, `inbound.go`, `established.go`, and `probe.go` to `Warn` keeps the existing `sa.releaseRequestWindow()` only on pre-send build errors and never rewinds `NextMsgID` or frees an active in-flight window |
| `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` | NAT traversal | `// Design:` page of `internal/component/ike/transport/keepalive.go`; anchored to `child.go`, `established.go`, `fsm.go` | "When NAT is detected, the SA carries the flag and the child install sets the UDP encapsulation attribute on the XFRM SA, which is what puts ESP inside UDP." | agrees | Warning once per failure condition in `Keepalive.Run` surfaces NAT-binding expiry risk without altering NAT detection, UDP encapsulation, or Child SA rebind behavior |
| `docs/architecture/ike/ipsec-10-cli-diag.md` | ipsec-unauthenticated-input | `// Design:` page of `internal/component/ike/engine/metrics.go`; anchored to `fsm.go`, `inbound.go`, `register.go` | "`openProtected` drops such a message and keeps the handshake (RFC 7296 Section 2.4), so the tunnel only waits for the peer's genuine answer" | agrees | Malformed inbound packets in `fsm.go:580` are counted and dropped without tearing down any SA or changing `openProtected` |
| `docs/architecture/ike/ipsec-11-interop-eap.md` | Decisions | anchored to `internal/component/ike/engine/fsm.go` | - | silent | This page governs EAP-MSCHAPv2 and EAP-TLS interop negotiation; this spec only counts malformed outer IKE frames in `fsm.go:580` before EAP dispatch and changes no EAP handshake behavior |
| `docs/architecture/ike/ipsec-13-rekey-wire.md` | Decisions | `// Design:` page of `internal/component/ike/engine/notify_invalid_msgid.go`; anchored to `child.go`, `established.go`, `fsm.go`, `inbound.go`, `register.go`, `responder.go` | "Packets that arrive after establishment are routed off the shared dispatch goroutine to the owning peer session's inbound channel." | agrees | Raising `sendInvalidMessageID` and established-SA build/send failures to `Warn` preserves single-owner routing and message-ID ownership on `maintainSA` |
| `docs/architecture/ike/ipsec-14-responder.md` | Decisions | `// Design:` page of `internal/component/ike/engine/responder.go`; anchored to `child.go`, `established.go`, `fsm.go`, `inbound.go`, `metrics.go`, `register.go` | "The responder holds the Child SA offer across RFC 4739 rounds." | agrees | Resolving the seven responder dropped-error sites changes only error visibility and annotations, preserving responder state transitions and RFC 4739 multi-round auth handling |
| `docs/architecture/ike/ipsec-dataplane-inspection.md` | Decision: a failed read is an error, never an empty table | anchored to `xfrm_linux.go`, `child.go`, `metrics.go`, `spd_policy.go` | - | silent | This page governs read-only XFRM state/policy inspection and drift comparison; this spec changes no dataplane readback or inspection logic, only rollback and teardown error visibility |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | Decisions | anchored to `authradius/handler.go`, `authradius/register.go`, `ppp/session_run.go` | "Framed-MTU lowers the interface MTU, never the LCP MRU." | agrees | Distinguishing `ppp.ErrRequestExpired` (Answer 3 guard) from unexpected `StageFramedMTU` errors (Answer 2 `Warn` with `CodeDoctorRADIUSFramedMTU`) preserves Framed-MTU semantics |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Decisions | `// Design:` page of `pppoe/metrics.go`; anchored to `pppoe/panic_guard.go` | "A refusal is a protocol or configuration decision about a well-formed read; this counter is the socket itself failing to read at all" | agrees | Malformed discovery frames in `panic_guard.go:86` increment a separate `ze_pppoe_discovery_parse_errors_total` counter rather than widening the closed 6-value `ze_pppoe_discovery_refusals_total` reason vocabulary |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 (D-1 VPP API loss) | "D-1, a VPP API loss with the process alive, is resolved by the VPP lifecycle rules: a live VPP that Ze cannot adopt is killed and restarted, so its exit proof always arrives and retires the retained owners" | agrees | This spec touches no VPP session dataplane or lifecycle recovery path |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 (VPP adoption option C) | "When Ze adopts the VPP, retained owners are read back by exact identity over the new connection of the same pidfd-proven process, and cleaned there (option C)" | agrees | This spec touches no VPP adoption or retained-owner reconciliation |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 (VPP adoption inside loss timer) | "Adoption happens only inside the single VPP loss timer: a reconnect to the same pidfd-proven process whose full readback matches (option C) keeps the sessions. A declared loss is never adopted: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This narrows the row "When Ze adopts the VPP" to the single timer, and the D-1 row's "cannot adopt" to every declared loss" | agrees | This spec touches no VPP loss timer, process lease, or PPPoE VPP teardown |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 (D-1 two recovery forms) | "D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms. Form (1), inside the reconnect-and-verify limit that follows the failed alive test: the same pidfd-proven process answering with the data Ze configured is enough to adopt it, and the PPPoE sessions are kept. The retained owners are read back by exact identity over the new connection of that process and cleaned there (option C), as part of the background reconciliation; a mismatch it finds is repaired, never treated as a loss. Form (2), a forced restart: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This supersedes the row "Adoption happens only inside the single VPP loss timer" (the single timer, adoption on a full readback match, and "a declared loss is never adopted"), and narrows the D-1 row and the row "When Ze adopts the VPP" to these two forms" | agrees | This spec touches no VPP recovery or reconciliation; it only counts malformed L2 discovery frames in `pppoe/panic_guard.go` |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `internal/component/l2tp/ppp/session_run.go` | - | silent | This page governs the PPPoE client uplink lifecycle; this spec only counts malformed PPP frames in shared `session_run.go` and changes no PPPoE client state transitions or framing |
| `docs/architecture/l2tp/subscriber-session-model.md` | Decisions | anchored to `internal/component/l2tp/ppp/session_run.go` | - | silent | This page governs subscriber session ownership, packed identity, and pool lifecycle; this spec only counts malformed PPP frames in `session_run.go` and changes no session lifecycle or ownership contracts |
| `docs/architecture/linux-limitations.md` | Entries | anchored to `xfrm_linux.go`, `child.go`, `spd_policy.go` | - | silent | This page catalogues upstream Linux kernel XFRM/netlink quirks; this spec changes no kernel interaction or workaround, only rollback and shutdown error reporting |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `xfrm_linux.go`, `spd_policy.go` | - | silent | This page documents Docker interop lab suites and peer daemons; this spec changes no interop harness or scenario wiring |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | This spec adds or modifies no interop suite or kernel-capability declaration |
| `docs/guide/monitoring.md` | Contained Panics | `// Design:` page of `internal/component/l2tp/pppoe/panic_guard.go` | "Any value above zero is a Ze defect. The ERROR log line of each panic names the unit and the stack" | agrees | Adding a parse-error counter in `pppoe/panic_guard.go:86` leaves `ze_task_panics_total` strictly for recovered panics and updates `docs/guide/monitoring.md` for any new metric |
| `docs/guide/radius.md` | EAP | `// Design:` page of `internal/component/radius/authenticator_eap.go` | "The exchange ends on Access-Accept or Access-Reject. Profiles map exactly as they do for PAP." | agrees | Raising the concluding EAP mismatch line to `Warn` with a doctor code preserves the RFC 3579 Section 2.6.3 rule that the RADIUS Access-Accept or Access-Reject packet decides the login |
| `docs/research/l2tpv2-ze-integration.md` | 16. Metrics | `// Design:` page of `authradius/handler.go`, `authradius/register.go`, `ppp/metrics.go`, `ppp/ra_send.go`, `ppp/session_run.go` | "Following ze's Prometheus metrics pattern:" | agrees | New malformed-PPP-frame counters follow Ze's existing push-model `pppMetrics` registration with closed low-cardinality labels |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call in the IKE dataplane, IKE engine, IKE transport keepalive, L2TP `authradius` plugin, PPP IPv6 RA sender or frame parser, PPPoE discovery parser, RADIUS admin EAP authenticator, or TACACS+ TCP client.

### Transformation Path
1. The error branch catches the returned error.
2. Depending on the site's classification:
   - Answer 2 (operator-actionable, forkable `ike` across `ike/engine`, `ike/dataplane`, and `ike/transport`): enqueues the transient event non-blockingly onto the plugin-owned `issuequeue.Queue` (`internal/component/plugin/issuequeue`, started in `ike/engine/register.go` via `issuequeue.Start(p)` and stopped after engine shutdown; reached from `ike/dataplane` and `ike/transport` via registered callback sinks; tracked with `issuequeue.RunRaises` on periodic once-per-condition paths to stay within `rpc.ReportRaisesPerWindow` = 60/min), logs at `Warn` with what, why, exact fix, `"code", <doctorCode>`, and `issuequeue.Outcome` (never calling `internal/core/report` directly and never blocking packet-dispatch, lock-holding, or periodic goroutines on daemon RPC), plus joins rollback context where the function already returns an error.
   - Answer 2 (operator-actionable, daemon-local `radius`, `l2tp/ppp`, and internal-only `authradius`): logs at `Warn` with what, why, exact fix, and `"code", <doctorCode>` (once per failure condition on periodic/hot paths), and raises the transient event on `internal/core/report` (`report.RaiseErrorFix` / `report.RaiseErrorFixOnce`) under that registered doctor code.
   - Answer 2 (per-packet untrusted input): increments a low-cardinality counter in the subsystem's existing Prometheus metric family and records the discard for `refuse-by-default` without flooding `Warn`.
   - Answer 3 (teardown, expired wait, or unauthenticated error reply): retains the safe continuation with `// refuse-by-default: guards <Symbol> -- <why>` on the line above.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Forkable `ike` (`engine`/`dataplane`/`transport`) ↔ daemon report bus | Callback sinks / engine calls → non-blocking `issuequeue.Queue` (`internal/component/plugin/issuequeue`) → worker calls `sdk.Plugin.RaiseIssue` (`report-issue` RPC over both DirectBridge and forked TLS/RPC) | No |
| Daemon-local `ppp` / `radius` / internal-only `authradius` ↔ operator | Log at `Warn` with `"code"`, `internal/core/report` (`show errors` / `show doctor`), and subsystem Prometheus counters | No |
| AAA backend ↔ chain | Preserved error classification (TACACS+ close errors never become `aaa.ErrAuthRejected`; RADIUS concluding EAP mismatch still returns `a.result(resp, username)`) | No |

### Changed-Wire Census

No protocol wire format, RPC schema, plugin JSON event schema, or persistent storage format changes in this spec. Search evidence: inspected `pkg/plugin/rpc/report.go` (`MethodReportIssue`, `ReportIssueInput`), `pkg/plugin/sdk/report.go` (`RaiseIssue`), `internal/component/plugin/issuequeue/{queue,raises}.go` (`Start`, `Queue`, `Outcome`, `RunRaises`), and `docs/architecture/api/process-protocol.md` ("Plugin problems"); forkable `ike` reuses the existing unchanged `report-issue` RPC (`action`, `severity`, `code`, `subject`, `message`, `fix`) via `issuequeue.Queue` and existing bounds, and every other edit changes only in-process logging, doctor-code / `EventCode` registration, non-blocking queue enqueues, daemon-local report raises, Prometheus counter increments, or source guard annotations inside the 22 target files and their sibling `register_doctor_codes.go` / `metrics.go` files.

### Integration Points
- Subsystem `slogutil.Logger` instances, `internal/component/plugin/issuequeue` (`issuequeue.Start(p)`, `Queue`, `Outcome`, `RunRaises`, plus callback sinks wired in `ike/engine/register.go` for `ike/dataplane` and `ike/transport`), daemon-local `internal/core/report` (`RaiseErrorFix` / `RaiseErrorFixOnce` for `radius`, `l2tp/ppp`, internal-only `authradius`), `internal/core/diagnostic` (`Register` / `RegisterEventCode`), and existing `IPsecMetrics` / `pppMetrics` / `pppoeMetrics` families.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Forkable `ike` raises exclusively through a plugin-owned `issuequeue.Queue` (`sdk.Plugin.RaiseIssue` / `report-issue`) via non-blocking enqueues and callback sinks; daemon-local components use `internal/core/report`; all packages use existing loggers, `internal/core/diagnostic`, and existing metric families |
| No unintended coupling (components stay isolated) | Yes | No cross-component imports added beyond `ike/engine` importing infrastructure package `internal/component/plugin/issuequeue` (matching `bfd`, `ospf`, `isis`, `ldp`, `bgp/plugins/rs`); `ike/dataplane` and `ike/transport` stay decoupled from `issuequeue` and `pkg/plugin/sdk` via registered callback sinks; each package registers its own doctor codes in its own `register_doctor_codes.go` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `internal/component/plugin/issuequeue` (`Queue`, `Outcome`, `RunRaises`), `dataplane.ESPFormReaderStoppedSink` sink pattern, `codeDoctorIPsecSPDUndoIncomplete`, `codePPPIPv6Service` (`reportIPv6Failure`), `CodeDoctorRADIUSFramedMTU`, `IPsecMetrics`, `pppMetrics`, `pppoeMetrics` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-packet untrusted-input paths increment atomic/registry counters with zero string formatting |
| Registration over hardcoding, outbound | Yes | Every new doctor code and event code registers via `init()` in the owning package's `register_doctor_codes.go` (`diagnostic.Register`, `diagnostic.RegisterEventCode`) and is discovered automatically by `recent-events` |
| Registration over hardcoding, inbound | Yes | No central switch, doctor list, or metric list learns a new code or counter name; `./le arch doctor-coverage` and `recent-events` discover every registered `EventCode` automatically |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 44 active instances in 22 files (plus 2 excluded MOBIKE instances) are the exact `error-dropped-below-warn` population under `ai/doors/access-security.md` at HEAD | `./le arch refuse-by-default report` run on 2026-11-26 | Re-run the report filtered to the door's base paths before editing and resolve any newly added site | Filtered `./le arch refuse-by-default report` at Phase 1 | validated (2026-11-26) |
| A-2 | Every newly added `Warn` log line is checked by `./le arch doctor-coverage check` and `./le commit create`, which requires `"code"` to name a declared doctor code | `ai/rules/user-facing-errors.md` and `docs/architecture/doctor-and-health-checks.md` | Committing a `Warn` without a covered doctor code is refused at `./le commit create` | `./le arch doctor-coverage check` before commit | validated (2026-11-26) |
| A-3 | `Keepalive.Run` (`ike/transport/keepalive.go:61`), `sendRaw` (`ike/engine/established.go:1008`), and `raSender.send` (`l2tp/ppp/ra_send.go:61`) run repeatedly on timers/exchanges, so an unthrottled `Warn` or `RaiseIssue` would flood logs and trip `rpc.ReportRaisesPerWindow` (60/min) | Source inspection of `keepalive.go:51`, `established.go:1008`, `ra_send.go:79`, and `docs/architecture/api/process-protocol.md` ("Plugin problems") | Unthrottled logging/raises fill logs or trigger `doctor-plugin-request-refused` during transient link loss | Unit tests asserting 1 `Warn` and 1 queued/raised event across repeated consecutive failures, retry via `RunRaises` on refusal, and a fresh emit after recovery | validated (2026-11-26) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A `Warn` or `RaiseIssue` on a periodic timer or per-packet path floods logs or exceeds `rpc.ReportRaisesPerWindow` (60/min) | Unit test or `.ci` log emits repeated identical warnings or `doctor-plugin-request-refused` | Rate-bound/coalesce to once per failure condition (cleared on recovery, with `issuequeue.RunRaises` retry on refusal) for `sendRaw`, `Keepalive.Run`, `raSender.send`, `sendProbeDFClear`, and per peer/condition for IKE build/emit events; count without per-packet `Warn` for untrusted wire parse errors (`fsm.go:580`, `session_run.go`, `panic_guard.go:86`) |
| R-2 | A changed error branch in an inbound IKE path kills a live SA | An IKE unit or functional test drops an established SA on malformed input | Preserve RFC 7296 Section 2.4: inbound parse errors count and return without changing SA state |
| R-3 | A `Warn` prints a PSK, RADIUS/TACACS+ shared secret, password, or key material | Review grep of new `Warn`, `issuequeue.Queue`, and `report.RaiseErrorFix*` arguments | Log only peer/subscriber/interface/SPI/operation and error text, never credentials or key bytes |
| R-4 | A `Warn` uses `"code"` for a protocol number (e.g., RADIUS `resp.Code`) instead of the doctor code | `./le arch doctor-coverage check` refuses the site | Rename protocol code attributes (`"radius-code", resp.Code`) and reserve `"code"` strictly for the doctor code |
| R-5 | A forkable `ike` path calls `internal/core/report` directly, blocks a packet-dispatch/lock-holding/periodic goroutine on synchronous `RaiseIssue`, or sends an unregistered `EventCode` | Forked `ze plugin ike` drops reports, stalls under a slow daemon, or hits `dispatch_report.go` refusal | All `ike/engine`, `ike/dataplane`, and `ike/transport` raises enqueue non-blockingly onto the plugin-owned `issuequeue.Queue` (started in `ike/engine/register.go`, stopped after engine shutdown before `p.Close()`), and every raised code (including `codeDoctorIPsecSPDUndoIncomplete`) registers via `diagnostic.RegisterEventCode` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise on periodic/hot paths, `report-issue` rate/code refusals, hot-goroutine stalls if `RaiseIssue` ran synchronously, doctor-coverage commit refusal, or a live SA / AAA chain answer disrupted by a misclassified error |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/post-release/features/spec-ipsec-11-mobike.md` (owns the two excluded `mobike.go` sites); `plan/release/fixes/spec-dropped-error-0-umbrella.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` over `ai/doors/access-security.md` base paths (excluding `mobike.go`) | → | `droppedError` across all 22 target files | `TestRefuseByDefaultRefusesEachRuleFixture` plus filtered `./le arch refuse-by-default report` showing zero in-scope `error-dropped-below-warn` findings |
| `./le arch doctor-coverage check` over every modified Go file | → | `register_doctor_codes.go` (`diagnostic.Register` + `diagnostic.RegisterEventCode`) + `Warn` sites | `TestDoctorCoverageSourceHoldsEveryRegisteredCode` plus `./le arch doctor-coverage check` exit 0 |
| Failed NAT-T keepalive send on periodic timer | → | `Keepalive.Run` (`internal/component/ike/transport/keepalive.go`) + callback sink → `issuequeue.Queue` (`sdk.Plugin.RaiseIssue`) | `TestKeepaliveSendFailureWarnsOncePerCondition` |
| Concluding RADIUS `Access-Accept` with peer EAP mismatch | → | `radiusAuthenticator.authenticateEAP` (`internal/component/radius/authenticator_eap.go`) | `TestAuthenticateEAPConcludingMismatchWarnsWithDoctorCode` |
| Malformed PPPoE discovery frame on AF_PACKET dispatch | → | `Subsystem.dispatchDiscovery` (`internal/component/l2tp/pppoe/panic_guard.go`) | `TestDispatchDiscoveryMalformedFrameIncrementsParseErrorCounter` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report` filtered to `ai/doors/access-security.md` base paths | Every in-scope site across the 22 target files resolves cleanly, leaving 0 unannotated in-scope `error-dropped-below-warn` findings | Only `internal/component/ike/engine/mobike.go:590` and `:607` remain under the door's base paths, because MOBIKE was moved to `plan/post-release/features/spec-ipsec-11-mobike.md` (Known Issues #3) by owner ruling on 2026-11-23 |
| AC-2 | `./le arch doctor-coverage check` over all changed files | Exits 0: every new `Warn` log site emits a registered doctor `code` from its owning package's `register_doctor_codes.go` (or existing package doctor code), every `ike`-raised code (including `codeDoctorIPsecSPDUndoIncomplete`) is registered via `diagnostic.RegisterEventCode`, forkable `ike` (`ike/engine`, `ike/dataplane`, `ike/transport`) enqueues non-blockingly onto its plugin-owned `issuequeue.Queue` (zero direct `internal/core/report` calls and zero synchronous `RaiseIssue` waits on dispatch/lock/periodic goroutines), and daemon-local `radius`, `l2tp/ppp`, and internal-only `authradius` raise on `internal/core/report` | None: `./le commit create` enforces 100% doctor-code coverage on newly added `Warn`/`Error` lines, `dispatch_report.go` requires `RegisterEventCode` for `report-issue`, and forkable plugins must work non-blockingly across both transports |
| AC-3 | Periodic/hot-path Answer 2 failures (`Keepalive.Run`, `sendRaw`, `raSender.send`, `sendProbeDFClear`, `certURLFetcher`) | Unit tests observe one `Warn` line naming what, why, fix, doctor `code`, and `issuequeue.Outcome` (for `ike`) across consecutive failures, one queued/raised event (tracked with `issuequeue.RunRaises` for `ike` staying within `rpc.ReportRaisesPerWindow`, or `internal/core/report` for `ppp`), and a fresh emit only after a successful recovery (or `RunRaises` retry window after daemon refusal) | None: repeated consecutive failures must be coalesced until recovery |
| AC-4 | Per-packet untrusted wire parse failures (`fsm.go:580`, `ppp/session_run.go`, `pppoe/panic_guard.go:86`) | Unit tests observe the corresponding Prometheus counter increment while the live SA or subscriber session continues without emitting a per-packet `Warn` | None: every malformed wire frame increments its subsystem counter |
| AC-5 | Concluding RADIUS EAP mismatch (`radius/authenticator_eap.go:118`) and XFRM rollback / Child SA / SPD / IKE build-and-emit Answer 2 sites | Unit tests observe the `Warn` carrying the doctor `code` (with `"radius-code"` separate from `"code"`), what, why, and fix, plus the queued `sdk.Issue` on `issuequeue.Queue` (for `ike`, including `codeDoctorIPsecSPDUndoIncomplete` in `removeSPDPolicies`) or `report` event (for `radius`), while preserving RFC 3579 Section 2.6.3 login decisions and existing error returns | None: every Answer 2 site has direct unit-test proof of its `Warn`, raised event, and preserved protocol result |
| AC-6 | Every Answer 3 guard annotation (`tacacs/client.go`, `ike/engine/register.go`, `notify_error.go:316,326`, `responder.go` unauthenticated sends, `authradius/register.go`, expired waits in `authradius/handler.go`, ENOENT teardown in `child.go`/`spd_policy.go`/`ra_send.go`) | `./le arch refuse-by-default report` accepts each site under `annotated` with a non-empty `guard` symbol visible in the file and distinct from the enclosing function | None: unguarded or self-naming annotations are refused by `RuleAnnotationNoGuard` |
| AC-7 | Scoped package tests and functional `.ci` suites for `ipsec`, `l2tp`, `pppoe`, and `plugin` (RADIUS/TACACS+) | All scoped unit tests and affected `.ci` functional tests pass with unchanged protocol behavior | None: any red in reached tests blocks closure |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXFRMInstallSARollbackFailureWarnsAndJoinsError` | `internal/component/ike/dataplane/xfrm_state_del_failure_linux_test.go` | AC-2, AC-5 (`xfrm_linux.go:156`) | |
| `TestIKEEngineDroppedErrorsWarnWithDoctorCodes` (covering `certurl.go`, `child.go`, `delete.go`, `established.go`, `inbound.go`, `notify_error.go`, `notify_invalid_msgid.go`, `notify_suspicious.go`, `probe.go`, `responder.go`, `spd_policy.go`) | `internal/component/ike/engine/dropped_errors_test.go` (or existing per-file test files) | AC-2, AC-3, AC-5 | |
| `TestHandleInboundMalformedPacketCountsWithoutWarn` | `internal/component/ike/engine/metrics_test.go` | AC-4 (`fsm.go:580`) | |
| `TestKeepaliveSendFailureWarnsOncePerCondition` | `internal/component/ike/transport/rfc3948_keepalive_test.go` | AC-2, AC-3 (`keepalive.go:61`) | |
| `TestRASenderLiveFailureWarnsOnceAndTeardownIsSilent` | `internal/component/l2tp/ppp/ra_send_test.go` | AC-2, AC-3, AC-6 (`ra_send.go:61`) | |
| `TestPPPMalformedFramesCountWithoutWarn` | `internal/component/l2tp/ppp/metrics_test.go` | AC-4 (`session_run.go:1311,1334,1353,1370,1795`) | |
| `TestDispatchDiscoveryMalformedFrameIncrementsParseErrorCounter` | `internal/component/l2tp/pppoe/panic_guard_test.go` | AC-4 (`panic_guard.go:86`) | |
| `TestAuthenticateEAPConcludingMismatchWarnsWithDoctorCode` | `internal/component/radius/authenticator_eap_test.go` | AC-2, AC-5 (`authenticator_eap.go:118`) | |
| `TestAuthRadiusExpiredWaitAndUnexpectedMTUStage` | `internal/component/l2tp/plugins/authradius/extract_framed_mtu_test.go` | AC-5, AC-6 (`authradius/handler.go:232,240`) | |
| `TestTacacsClientCloseErrorsPreserveResultAndChain` | `internal/component/tacacs/client_test.go` | AC-6 (`tacacs/client.go`) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unannotated in-scope `error-dropped-below-warn` findings under `ai/doors/access-security.md` base paths (excluding `mobike.go`) | 0 | 0 | N/A | 1 |
| Uncovered new `Warn`/`Error` sites reported by `./le arch doctor-coverage check` | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Reached IPsec `.ci` tests | `test/ipsec/*.ci` | Unchanged IPsec handshake/dataplane behavior; doctor codes registered and queryable via `ze show diagnostic` | |
| Reached L2TP / PPPoE `.ci` tests | `test/l2tp/*.ci`, `test/pppoe/*.ci` | Unchanged subscriber session behavior; malformed frame counters and IPv6 RA diagnostics active | |
| Reached RADIUS / TACACS+ `.ci` tests | `test/plugin/*.ci`, `test/parse/*.ci` | Unchanged AAA chain behavior; RADIUS EAP doctor code registered | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A (no wire-visible behavior changes) | N-A | N-A | This spec changes only in-process error visibility, doctor codes, Prometheus counters, and guard annotations; every wire exchange, RFC 7296 Section 2.4 drop rule, RFC 3579 Section 2.6.3 decision rule, and AAA chain contract is preserved unchanged | |

## Files to Modify
- `internal/component/ike/dataplane/xfrm_linux.go` - 1: `xfrmBackend.InstallSA` (:156) rollback failure `Warn` + doctor code + non-blocking callback sink to `ike` `issuequeue.Queue` + joined error
- `internal/component/ike/dataplane/register_doctor_codes.go` - register doctor code and event code for XFRM install rollback failure
- `internal/component/ike/engine/certurl.go` - 1: `certURLFetcher.start.func` (:335) `Warn` once per backoff window with doctor code + `issuequeue.Queue` / `RunRaises`
- `internal/component/ike/engine/child.go` - 2: `removeChildSAOutgoing` (:1574), `removeChildSAIncoming` (:1609) `ENOENT`/`ESRCH` guard vs `Warn` with doctor code + `issuequeue.Queue`
- `internal/component/ike/engine/delete.go` - 2: `PeerSession.writeDelete` (:59), `sendIKESATeardown` (:200) `Warn` with doctor code + `issuequeue.Queue`
- `internal/component/ike/engine/established.go` - 4: `sendRaw` (:1008) count + once-per-condition `Warn` + `issuequeue.Queue` / `RunRaises`; `emitChildUp` (:1219), `emitChildDown` (:1240), `emitChildRekey` (:1261) `Warn` with doctor code + `issuequeue.Queue`
- `internal/component/ike/engine/fsm.go` - 1: `handleInbound` (:580) count malformed inbound packets in `IPsecMetrics`
- `internal/component/ike/engine/inbound.go` - 1: `PeerSession.handleInformationalOwned` (:852) `Warn` with doctor code + `issuequeue.Queue`
- `internal/component/ike/engine/metrics.go` - add malformed-inbound and send-failure counters in existing `IPsecMetrics`
- `internal/component/ike/engine/notify_error.go` - 3: `PeerSession.respondError` (:175) `Warn` with doctor code + `issuequeue.Queue`; `sendInvalidIKESPI` (:316, :326) guard annotations
- `internal/component/ike/engine/notify_invalid_msgid.go` - 1: `PeerSession.sendInvalidMessageID` (:111) `Warn` with doctor code + `issuequeue.Queue`
- `internal/component/ike/engine/notify_suspicious.go` - 1: `PeerSession.informSuspicious` (:239) `Warn` with doctor code + `issuequeue.Queue`
- `internal/component/ike/engine/probe.go` - 1: `PeerSession.sendProbeDFClear` (:336) count + once-per-condition `Warn` with doctor code + `issuequeue.Queue` / `RunRaises`
- `internal/component/ike/engine/register.go` - 1: `closeSDK` (:684) shutdown guard annotation, plus start plugin-owned `issuequeue.Queue` (`issuequeue.Start(p)`), stop it after engine shutdown before `p.Close()`, and wire `ike/dataplane` and `ike/transport` callback sinks to enqueue onto it
- `internal/component/ike/engine/register_doctor_codes.go` - register new `ike/engine` doctor codes and `diagnostic.RegisterEventCode` entries for all raised codes (including registering existing `codeDoctorIPsecSPDUndoIncomplete` via `diagnostic.RegisterEventCode`)
- `internal/component/ike/engine/responder.go` - 7: unauthenticated/pre-auth reply send failures (:175, :342, :512, :1347, :1364) guard annotations; build failures (:1338, :1360) `Warn` with doctor code + non-blocking `issuequeue.Queue`
- `internal/component/ike/engine/spd_policy.go` - 1: `removeSPDPolicies` (:708) `policyGone` guard vs `Warn` with `codeDoctorIPsecSPDUndoIncomplete` + `issuequeue.Queue` raise
- `internal/component/ike/transport/keepalive.go` - 1: `Keepalive.Run` (:61) once-per-condition `Warn` with doctor code + non-blocking callback sink to `ike` `issuequeue.Queue` / `RunRaises`
- `internal/component/ike/transport/register_doctor_codes.go` - register doctor code and `diagnostic.RegisterEventCode` for NAT keepalive send failure
- `internal/component/l2tp/plugins/authradius/handler.go` - 2: `radiusAuth.doRADIUS` (:232, :240) `ErrRequestExpired` guard vs unexpected `StageFramedMTU` `Warn` with existing `ppp.CodeDoctorRADIUSFramedMTU`
- `internal/component/l2tp/plugins/authradius/register.go` - 1: `runPlugin.func` (:132) shutdown guard annotation
- `internal/component/l2tp/ppp/metrics.go` - add closed-label malformed PPP frame counter in existing `pppMetrics`
- `internal/component/l2tp/ppp/ra_send.go` - 1: `raSender.send` (:61) teardown guard vs once-per-condition `Warn` via existing `reportIPv6Failure` (`codePPPIPv6Service`)
- `internal/component/l2tp/ppp/session_run.go` - 5: `pppSession.handleFrame` (:1311, :1334, :1353, :1370) and `pppSession.handleLCPPacket` (:1795) count malformed frames in `pppMetrics`
- `internal/component/l2tp/pppoe/metrics.go` - add `ze_pppoe_discovery_parse_errors_total` counter in existing `pppoeMetrics`
- `internal/component/l2tp/pppoe/panic_guard.go` - 1: `Subsystem.dispatchDiscovery` (:86) increment `ze_pppoe_discovery_parse_errors_total` with guard annotation
- `internal/component/radius/authenticator_eap.go` - 1: `radiusAuthenticator.authenticateEAP` (:118) raise `Info` to `Warn` with doctor code and `"radius-code"` attribute
- `internal/component/radius/register_doctor_codes.go` - register doctor code and `diagnostic.RegisterEventCode` for concluding RADIUS EAP mismatch
- `internal/component/tacacs/client.go` - 5: `TacacsClient.Close` (:144), `trySend` (:608), `storeConn` (:720, :726), `closeAndEvict` (:743) teardown/eviction guard annotations
- `docs/guide/monitoring.md` - document the new Prometheus counters added to `IPsecMetrics`, `pppMetrics`, and `pppoeMetrics`

Line numbers reflect HEAD on 2026-11-26; re-run `./le arch refuse-by-default report` before the first edit if lines have shifted.

## Files to Create
- None (new unit tests live in the existing per-package test files listed in the TDD Test Plan, or in sibling `*_test.go` files inside the affected packages).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf or RPC added |
| YANG validation constraints | N-A | No config leaf added |
| YANG custom validators | N-A | No config leaf added |
| CLI commands/flags | N-A | No CLI command added |
| CLI grammar (keyword before value) | N-A | No CLI command added |
| Editor autocomplete | N-A | No CLI command added |
| Functional test for new RPC/API | N-A | No RPC/API added |
| Pipe completeness | N-A | No command output changed |
| Env var registration | N-A | No env var added |
| Doctor check for runtime dependencies | N-A | No new host/runtime dependency added |
| Doctor code per problem class | Yes | Every new `Warn`/`Error` log emits a registered doctor `code` declared via `diagnostic.Register` + `diagnostic.RegisterEventCode` in `internal/component/ike/dataplane/register_doctor_codes.go`, `internal/component/ike/engine/register_doctor_codes.go` (including registering `codeDoctorIPsecSPDUndoIncomplete` via `diagnostic.RegisterEventCode`), `internal/component/ike/transport/register_doctor_codes.go`, and `internal/component/radius/register_doctor_codes.go` (plus existing `codePPPIPv6Service` and `CodeDoctorRADIUSFramedMTU`), enqueueing transient events non-blockingly via `issuequeue.Queue` (`sdk.Plugin.RaiseIssue` + callback sinks) for forkable `ike` and raising via `internal/core/report` for daemon-local components; verified by `./le arch doctor-coverage check` |
| Prometheus counters/metrics | Yes | Extends `IPsecMetrics` (`ike/engine/metrics.go`), `pppMetrics` (`l2tp/ppp/metrics.go`), and `pppoeMetrics` (`l2tp/pppoe/metrics.go` with `ze_pppoe_discovery_parse_errors_total`); documented in `docs/guide/monitoring.md` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Error visibility and diagnostic coverage only |
| 2 | Config syntax changed? | No | No YANG/config change |
| 3 | CLI command added/changed? | No | No CLI command change |
| 4 | API/RPC added/changed? | No | No API/RPC change |
| 5 | Plugin added/changed? | No | No plugin registration or lifecycle change |
| 6 | Has a user guide page? | No | Existing guides (`docs/guide/radius.md`, `docs/guide/ipsec.md`, `docs/guide/l2tp.md`) remain accurate |
| 7 | Wire format changed? | No | No protocol wire format change |
| 8 | Plugin SDK/protocol changed? | No | Reuses existing `issuequeue.Queue` / `sdk.Plugin.RaiseIssue` (`report-issue`) unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Existing RFC 7296 / 3948 / 3579 / 8907 / 1661 / 2516 / 4861 behaviors are preserved |
| 10 | Test infrastructure changed? | No | Uses existing Go unit tests and `.ci` suites |
| 11 | Affects daemon comparison? | No | No comparison impact |
| 12 | Internal architecture changed? | No | Uses existing logging, doctor-code, `issuequeue.Queue` (`sdk.Plugin.RaiseIssue`), daemon-local report-bus, and metrics patterns |
| 13 | Route metadata keys added/changed? | No | No route metadata change |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` for new malformed/send-error counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Only diagnostic codes and metrics added within existing registries |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Every declared `// Design:` and `<!-- source: -->` page is verified in `## Design documents`; none of their architectural claims change (confirm at closure with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-3-access-security.md`) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config/CLI/API example changes |

## Design presentation

### What changes

All 44 active `error-dropped-below-warn` instances across 22 files under the `access-security` door (`ike`, `l2tp`/`ppp`/`pppoe`/`authradius`, `radius`, `tacacs`) are resolved so no operator-actionable failure is silently buried at `Debug` or `Info`:
- Operator-actionable failures (XFRM install rollback, non-ENOENT Child SA / SPD removal, established IKE Delete/INFORMATIONAL/notify build and send failures, Child SA event emit failures, hash-and-URL fetch failures, NAT-T keepalive send failures, live PPP IPv6 RA send failures, and concluding RADIUS EAP mismatches) log at `Warn` with what, why, exact fix, and a registered doctor `code` from the owning package's `register_doctor_codes.go` (or existing package doctor code), and raise transient events on the daemon's report bus (coalesced to once per failure condition on periodic/hot paths): forkable `ike` (`ike/engine`, `ike/dataplane`, `ike/transport`) enqueues non-blockingly onto a plugin-owned `issuequeue.Queue` (`internal/component/plugin/issuequeue`, started over `p` in `ike/engine/register.go`, stopped after engine shutdown before `p.Close()`, reached from `ike/dataplane` and `ike/transport` via registered callback sinks, using `issuequeue.Outcome` on `Warn` lines and `issuequeue.RunRaises` on periodic once-per-condition runs, with every raised code including `codeDoctorIPsecSPDUndoIncomplete` registered via `diagnostic.RegisterEventCode` and staying within `rpc.ReportRaisesPerWindow` = 60/min), while daemon-local components (`radius`, `l2tp/ppp`, and internal-only `authradius`) raise directly on `internal/core/report`.
- Untrusted per-packet malformed wire inputs (`ike/engine/fsm.go:580`, `l2tp/ppp/session_run.go`, `l2tp/pppoe/panic_guard.go:86`) increment low-cardinality counters in the existing `IPsecMetrics`, `pppMetrics`, and `pppoeMetrics` families without flooding `Warn` or tearing down live sessions.
- Teardown closes (`tacacs/client.go`, `ike/engine/register.go`, `authradius/register.go`), already-gone `ENOENT`/`ESRCH` removals, expired auth waits (`authradius/handler.go`), and unauthenticated out-of-SA / pre-auth UDP reply send failures carry explicit `// refuse-by-default: guards <Symbol> -- <why>` annotations.
- The two `mobike.go` sites remain excluded under `plan/post-release/features/spec-ipsec-11-mobike.md` (Known Issues #3), and the 17 sites across 9 files already fixed on `main` are retired.

### Before and after

1. **NAT-T keepalive send failure (`internal/component/ike/transport/keepalive.go:61`):**
   - **Before:** logs `nat-keepalive: send failed` at `Debug` (invisible at default `WARN` level), so a failing NAT keepalive silently lets the NAT binding expire.
   - **After:** enqueues one event non-blockingly onto `ike`'s `issuequeue.Queue` (via a callback sink wired from `ike/engine/register.go`, tracked with `issuequeue.RunRaises`), logs one `Warn` per failure condition naming the remote endpoint, why (NAT mapping may expire and drop ESP/IKE), exact fix (`ze show vpn ipsec sa`), `issuequeue.Outcome`, and `"code"` registered in `internal/component/ike/transport/register_doctor_codes.go`, and re-arms on recovery without ever blocking the keepalive ticker on daemon RPC.
2. **RADIUS concluding EAP mismatch (`internal/component/radius/authenticator_eap.go:118`):**
   - **Before:** logs at `Info` with `"code", resp.Code`, hidden at default `WARN` level and colliding with the reserved `"code"` key.
   - **After:** logs at `Warn` with `"code"` carrying the doctor code registered in `internal/component/radius/register_doctor_codes.go`, moves `resp.Code` to `"radius-code"`, raises a rate-bounded event on daemon-local `internal/core/report`, and keeps the RFC 3579 Section 2.6.3 Access-Accept/Reject decision unchanged.
3. **Malformed PPPoE discovery frame (`internal/component/l2tp/pppoe/panic_guard.go:86`):**
   - **Before:** logs `pppoe: parse error` at `Debug` with no counter, leaving malformed discovery traffic invisible at default log levels.
   - **After:** increments `ze_pppoe_discovery_parse_errors_total` (separate from the closed 6-value `ze_pppoe_discovery_refusals_total` series) with a guard annotation naming `HandleDiscovery` and zero per-packet `Warn` noise.
4. **TACACS+ pooled connection close on teardown/eviction (`internal/component/tacacs/client.go`):**
   - **Before:** five `net.Conn.Close()` branches log at `Debug` and trigger `error-dropped-below-warn`.
   - **After:** each branch carries `// refuse-by-default: guards <Symbol> -- <why>`, confirming teardown/eviction close errors never alter settled AAA answers or trigger `aaa.ErrAuthRejected`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Raise every one of the 44 instances blindly to `Warn` | Violates `ai/rules/user-facing-errors.md` and `spec-dropped-error-0-umbrella.md`: per-packet malformed wire frames and periodic timers would flood logs, and teardown `Close()` errors give operators nothing actionable to fix |
| Call `internal/core/report` directly from `ike/engine`, `ike/dataplane`, or `ike/transport` | Violates `ai/doors/system.md` ("A plugin can run in another process") and `docs/architecture/api/process-protocol.md` ("Plugin problems"): a forked `ze plugin ike` process holds an unread local copy of `internal/core/report` |
| Call synchronous `sdk.Plugin.RaiseIssue` (`espFormIssueRaiser`) from packet-dispatch, lock-holding, or periodic `ike` goroutines | Violates `docs/architecture/doctor-and-health-checks.md` ("Event codes"): synchronous `RaiseIssue` waits on daemon RPC and would stall packet dispatch (`responder.go`), `childLifecycleMu` + `beginDataplaneWrite()` (`xfrm_linux.go`), or periodic timers (`keepalive.go`, `established.go`, `probe.go`); `internal/component/plugin/issuequeue` solves this with non-blocking enqueue and `RunRaises` retry tracking |
| Add `"malformed"` as a 7th `reason` label on `ze_pppoe_discovery_refusals_total` | Departs from `docs/architecture/l2tp/bng-5-pppoe.md`, which defines `ze_pppoe_discovery_refusals_total` as a closed 6-value set for well-formed discovery packets only; a separate `ze_pppoe_discovery_parse_errors_total` counter matches `ze_pppoe_discovery_read_errors_total` |
| Keep MOBIKE (`mobike.go:590,607`) inside this release-blocking fix spec | Contradicts the 2026-11-23 owner ruling moving MOBIKE and its known issues to `plan/post-release/features/spec-ipsec-11-mobike.md` (Known Issues #3) |
| Emit `Warn` lines without doctor codes or report-bus raises | Fails the blocking `./le arch doctor-coverage check` and `./le commit create` gates (`ai/rules/user-facing-errors.md`) |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- re-run `./le arch refuse-by-default report` filtered to `ai/doors/access-security.md` base paths to confirm the 44 active sites; register all new doctor codes and `diagnostic.RegisterEventCode` entries in `ike/dataplane`, `ike/engine` (including registering `codeDoctorIPsecSPDUndoIncomplete` via `diagnostic.RegisterEventCode`), `ike/transport`, and `radius` `register_doctor_codes.go` files; start a plugin-owned `issuequeue.Queue` (`issuequeue.Start(p)`) in `ike/engine/register.go` (stopped after engine shutdown before `p.Close()`) and wire non-blocking `ike/dataplane` and `ike/transport` callback sinks to enqueue onto it; write failing unit tests for the Answer 2 `Warn` + doctor-code + issue-raise sites and per-packet counter sites.
2. **Phase: IKE (`dataplane`, `engine`, `transport`)** -- resolve the 28 active IKE sites (`xfrm_linux.go`, 13 `engine/*.go` files, `keepalive.go`) with doctor-coded `Warn` (+ `issuequeue.Outcome` and non-blocking `issuequeue.Queue` enqueues, using `issuequeue.RunRaises` on periodic once-per-condition paths within `rpc.ReportRaisesPerWindow`), `IPsecMetrics` counters, and Answer 3 guard annotations; make IKE unit tests green.
3. **Phase: L2TP / PPP / PPPoE / authradius** -- resolve the 10 active subscriber sites (`authradius/handler.go`, `authradius/register.go`, `ppp/ra_send.go`, `ppp/session_run.go`, `pppoe/panic_guard.go`) using `reportIPv6Failure`/`codePPPIPv6Service`, `CodeDoctorRADIUSFramedMTU`, `pppMetrics`, `ze_pppoe_discovery_parse_errors_total`, and Answer 3 guard annotations; make L2TP/PPP/PPPoE unit tests green.
4. **Phase: RADIUS and TACACS+** -- resolve `radius/authenticator_eap.go:118` (`Warn` with doctor code, `"radius-code"`, and daemon-local `report.RaiseErrorFixOnce`, preserving RFC 3579 Section 2.6.3) and annotate the 5 `tacacs/client.go` teardown/eviction close sites; make RADIUS and TACACS+ unit tests green; update `docs/guide/monitoring.md`.
5. **Phase: Prove** -- verify AC-1 through AC-7 via `./le arch refuse-by-default report`, `./le arch doctor-coverage check`, and the main thread's scoped test and lint commands.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 44 active instances has its resolution at file:line and AC-1 shows 0 in-scope findings |
| Correctness | No returned error or log change causes an inbound malformed packet to kill a live IKE SA (RFC 7296 Section 2.4), causes a concluding EAP mismatch to override RADIUS Access-Accept/Reject (RFC 3579 Section 2.6.3), or turns a TACACS+ close error into `aaa.ErrAuthRejected` |
| Forked-plugin report boundary and non-blocking queue | Zero direct `internal/core/report` calls in `internal/component/ike/...`; every new IKE/dataplane/transport raise enqueues non-blockingly onto `ike`'s `issuequeue.Queue` (started in `ike/engine/register.go`, stopped after engine shutdown before `p.Close()`, using `issuequeue.Outcome` and `issuequeue.RunRaises` for periodic runs), stays within `rpc.ReportRaisesPerWindow` (60/min), and every raised code (including `codeDoctorIPsecSPDUndoIncomplete`) is registered via `diagnostic.RegisterEventCode` |
| Operator errors and doctor codes | Every new `Warn` line names what, why, and the exact fix, carries `"code"` naming a declared doctor code registered in its owning package's `register_doctor_codes.go` (or existing package code), never overloads `"code"` with a protocol code, and raises transient events via `issuequeue.Queue` (`ike`) or `internal/core/report` (daemon-local components) (`./le arch doctor-coverage check` passes) |
| Rate bounding | Periodic/hot-path warnings (`Keepalive.Run`, `sendRaw`, `raSender.send`, `sendProbeDFClear`, `certURLFetcher`) warn and raise once per failure condition until recovery (with `issuequeue.RunRaises` retry on daemon refusal for `ike`); per-packet malformed wire paths count without per-packet `Warn` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero in-scope `error-dropped-below-warn` findings | `./le arch refuse-by-default report` filtered to `ai/doors/access-security.md` base paths (excluding `mobike.go`) |
| 100% doctor-code coverage on all new `Warn`/`Error` sites | `./le arch doctor-coverage check` (exit 0) and `TestDoctorCoverageSourceHoldsEveryRegisteredCode` |
| Scoped package tests (run by main thread through `./le` for patched GOROOT / EAP-TLS) | `./le test unit pkg path internal/component/ike/dataplane/ internal/component/ike/engine/ internal/component/ike/transport/ internal/component/l2tp/plugins/authradius/ internal/component/l2tp/ppp/ internal/component/l2tp/pppoe/ internal/component/radius/ internal/component/tacacs/` |
| Scoped lint (run by main thread) | `./le go lint run scope "./internal/component/ike/... ./internal/component/l2tp/... ./internal/component/radius/... ./internal/component/tacacs/..."` |
| Scoped functional suites (run by main thread) | `./le test functional ipsec`, `./le test functional l2tp`, `./le test functional pppoe`, `./le test functional plugin` (and `./le repo changed scope print both` to confirm reached suites) |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Error leakage | No `Warn`, report-bus message, or returned error carries a PSK, RADIUS/TACACS+ shared secret, password, or key material |
| Fail-open / fail-closed | No TACACS+ close error becomes `aaa.ErrAuthRejected`; no RADIUS concluding EAP mismatch overrides RFC 3579 Section 2.6.3; no unauthenticated IKE packet tears down a live SA |
| Log flooding (DoS) | Untrusted wire parse errors (`fsm.go:580`, `ppp/session_run.go`, `pppoe/panic_guard.go:86`) count without emitting per-packet `Warn` lines |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint or `doctor-coverage` failure | Fix inline (register missing doctor code or move protocol code off `"code"`). If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

(The tree-wide 2026-11-02 open row in `plan/journal/dropped-error-logged-below-warn.md` is owned by `plan/release/fixes/spec-dropped-error-0-umbrella.md`; the 2026-11-01 MOBIKE row is already marked `fixed: d6ef7093a8` and the remaining two `mobike.go` sites moved to `plan/post-release/features/spec-ipsec-11-mobike.md`.)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Apply the umbrella's three answers in order (return/fix, doctor-coded `Warn` + report bus or counter, guard annotation) | Raise every line to `Warn` | A `Warn` on per-packet untrusted input floods logs; teardown `Close()` errors have no operator fix |
| Register every new `Warn` site's doctor code in the owning package's `register_doctor_codes.go` + `diagnostic.RegisterEventCode` (including registering `codeDoctorIPsecSPDUndoIncomplete` as an event code) and raise transient events non-blockingly via a plugin-owned `issuequeue.Queue` (`sdk.Plugin.RaiseIssue`, with `RunRaises` on periodic runs, for forkable `ike`) or `internal/core/report` (for daemon-local components) | Emit plain `Warn` without `"code"`, call `internal/core/report` directly from `ike`, or block hot/periodic/lock-holding `ike` goroutines on synchronous `RaiseIssue` | Required by `ai/rules/user-facing-errors.md`, `./le arch doctor-coverage check`, `ai/doors/system.md`, `docs/architecture/api/process-protocol.md` ("Plugin problems"), and `docs/architecture/doctor-and-health-checks.md` ("Event codes") so forked `ze plugin ike` reaches the daemon's report bus non-blockingly and every raised code is admitted by `dispatch_report.go` |
| Add `ze_pppoe_discovery_parse_errors_total` separately from `ze_pppoe_discovery_refusals_total` | Add `"malformed"` label to `ze_pppoe_discovery_refusals_total` | Preserves the closed 6-value well-formed-read vocabulary in `docs/architecture/l2tp/bng-5-pppoe.md` |
| Exclude `internal/component/ike/engine/mobike.go` (`:590`, `:607`) from this release fix spec | Resolve MOBIKE drops here | Owner ruling of 2026-11-23 moved MOBIKE and its known issues to `plan/post-release/features/spec-ipsec-11-mobike.md` (Known Issues #3) |

## Known Limitations
- `internal/component/ike/engine/mobike.go:590` and `:607` (`PeerSession.serviceMobike`) are deliberately out of scope here and are homed in `plan/post-release/features/spec-ipsec-11-mobike.md` (Known Issues #3) per the 2026-11-23 owner ruling.

## RFC Documentation (Scope: protocol)

This spec changes no protocol wire behavior, timer, or state transition; it preserves the RFC obligations already quoted inline at each target site.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 7296 Section 2.4 | Unprotected or malformed inbound IKE packets are dropped and counted without tearing down a live SA | Existing responder/initiator peer configuration and rate limits | `TestHandleInboundMalformedPacketCountsWithoutWarn` | Existing `internal/component/ike/engine/` handshake and `openProtected` tests |
| RFC 3948 Section 4 | Periodic NAT-T keepalives continue retrying after transient send errors; send failure warns once per condition until recovery | Existing NAT-T keepalive interval and socket binding | `TestKeepaliveSendFailureWarnsOncePerCondition` | Existing `internal/component/ike/transport/rfc3948_keepalive_test.go` |
| RFC 3579 Section 2.6.3 | RADIUS `Access-Accept` / `Access-Reject` solely decides admin EAP login even when the concluding EAP packet fails peer verification; mismatch now warns with a doctor code | Existing `eap-md5` and `eap-mschapv2` auth-method options | `TestAuthenticateEAPConcludingMismatchWarnsWithDoctorCode` | Existing `internal/component/radius/rfc3579_eap_test.go` |
| RFC 8907 Section 5.4 | TACACS+ connection teardown/eviction `Close()` errors never alter a settled authentication/authorization/accounting reply or trigger `aaa.ErrAuthRejected` | Existing single-connect pooling and server-list fallthrough | `TestTacacsClientCloseErrorsPreserveResultAndChain` | Existing `internal/component/tacacs/rfc8907_client_test.go` |
| RFC 1661 / RFC 2516 / RFC 4861 | Malformed PPP and PPPoE discovery frames are discarded and counted without tearing down valid sessions; live PPP IPv6 RA send failures warn once per condition via `codePPPIPv6Service` | Existing LCP/IPCP/IPv6CP options and PPPoE discovery refusal counters | `TestPPPMalformedFramesCountWithoutWarn`, `TestDispatchDiscoveryMalformedFrameIncrementsParseErrorCounter`, `TestRASenderLiveFailureWarnsOnceAndTeardownIsSilent` | Existing `internal/component/l2tp/ppp/` and `internal/component/l2tp/pppoe/` tests |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable positive assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted scoped tests, lint, `./le arch refuse-by-default report`, and `./le arch doctor-coverage check` pass in the main checkout (run by the main thread; GitHub CI verifies the pushed commit)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-3-access-security.md` only, in the same `./le commit create` script
