# Spec: pppoe-subscribers-produce-no-accounting-or-telemetry

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | The supported subscriber access contract must produce PPPoE accounting and telemetry as well as L2TP. |
| Scope | protocol |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Per-session Prometheus metric naming | Rename `ze_l2tp_session_*` to `ze_subscriber_session_*` with an `access_type` (`l2tp` / `pppoe`) label (pre-release; no compatibility alias). |
| Per-subscriber Prometheus label cardinality | Keep per-subscriber Prometheus metrics enabled by default (matching existing L2TP behavior). |

## Task

**A PPPoE subscriber generates no RADIUS accounting record and no per-session
Prometheus metric. An L2TP subscriber generates both.**

The underlying counter collection and failure-safe encoding already exist on
HEAD. In commit `38267f5e0c` (2026-11-22), L2TP RADIUS accounting moved off
interface-name lookups (`iface.GetStats`) onto the shared PPP driver's
original-unit counter capture (`internal/component/l2tp/ppp/network_counters.go`):
PPP reads the creation-bound baseline before emitting `EventSessionIPAssigned`,
captures the terminal sample before closing the handle on teardown, and
`internal/component/l2tp/plugins/authradius/acct.go` subtracts that baseline
from the network-qualified capture's live and terminal readings, omitting all
six traffic attributes (and logging + raising `doctor-radius-accounting-counters`)
rather than sending zeros whenever a counter sample is unavailable. On the VPP
data plane, the PPPoE session handle (`internal/component/l2tp/pppoe/dataplane/vpp/session.go`)
already supplies creation-bound `Counters()` from the VPP stats segment, which
the same PPP `networkCounterLifetime` reads directly.

Two wires still stop at L2TP:

1. **RADIUS accounting subscribes only to `l2tpevents`.**
   `(*radiusAcct).subscribeEventBus` (`internal/component/l2tp/plugins/authradius/acct.go`)
   subscribes to `l2tpevents.SessionIPAssigned`, `l2tpevents.SessionIPReleased`,
   and `l2tpevents.SessionDown`, and captures counters only through
   `l2tp.LookupService().CaptureSession`. The PPPoE subsystem emits `subevents`
   (`SessionUp`, `SessionIPAssigned`, `SessionIPReleased`, `SessionDown`), which
   L2TP's `subscriberBridge` also re-emits for every L2TP session, yet
   `subevents` does not yet carry `BaselineCounters`, `CallingStationID`, or
   `TerminateCause`, PPPoE does not yet expose a network-qualified counter
   capture, `acctHealthSubject` hardcodes the `l2tp:` prefix, and `authradius`
   has no Prometheus counter for omitted accounting counters.
2. **Per-session Prometheus telemetry polls only L2TP reactors and publishes `ze_l2tp_session_*`.**
   `(*Subsystem).startStatsPollerLocked` (`internal/component/l2tp/subsystem.go`)
   constructs `l2tpStatsPoller` over `s.reactors` and emits the six
   `ze_l2tp_session_*` series (`state`, `uptime_seconds`, `rx_bytes_total`,
   `tx_bytes_total`, `rx_packets_total`, `tx_packets_total`). That path never
   sees PPPoE sessions — and on a PPPoE-only BNG where `l2tp` is unconfigured,
   `l2tp.Subsystem` is not even registered (`cmd/ze/hub/register_l2tp.go`), so
   no per-session poller runs at all.

Because `docs/features/rfc-status.md` publishes RFC 2866 and RFC 2869 as
supported for subscriber access, leaving PPPoE silent makes those claims
over-claim. Per `ai/rules/rfc-compliance.md`, an over-claiming row is repaired
by implementing and proving the missing half rather than lowering the row.

**The unifying abstraction already exists.** `subscriber.Session` and
`subscriber.DefaultRegistry` (`internal/component/l2tp/subscriber/session.go`,
`registry.go`) already hold both L2TP and PPPoE sessions with `AccessType`,
`PppInterface`, `Username`, `IPv4Addr`, and identity fields, and `subevents`
already serves the pool and route-observer consumers for both access types. Per
Thomas's 2026-11-24 owner decisions, the six per-session Prometheus series are
renamed from `ze_l2tp_session_*` to `ze_subscriber_session_*` with an
`access_type` (`l2tp` / `pppoe`) label (no compatibility alias), and
per-subscriber Prometheus metrics remain enabled by default.

**The goal:** a PPPoE subscriber produces the same RADIUS accounting records
(Start, Interim-Update, Stop with creation-bound counters, Gigawords, and
teardown cause) and the same per-session Prometheus metrics
(`ze_subscriber_session_*` with `access_type="pppoe"`) as an L2TP subscriber
(`access_type="l2tp"`), from one shared subscriber code path on L2TP-only,
PPPoE-only, and concurrent BNG deployments, with unavailable counters omitted,
logged, doctor-reported, and counted on Prometheus rather than reported as zero.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - the design document `subscriber/session.go`, `subscriber/events/events.go`, `subscriber_bridge.go`, and `ppp/network_counters.go` declare
  → Decision: `subscriber.Session` and `subscriber.DefaultRegistry` are the access-type-neutral record and registry; a lifecycle consumer subscribes to the `subscriber` topic alone, never to both topics.
  → Constraint: accounting counters come from the session's original PPP unit (`BaselineCounters`, network-qualified capture `Counters`, and `TerminalCounters` awaited off publication callbacks), never by interface name.
  → Constraint: the page currently states `"PPPoE accounting is not connected to subscriber events."`; this spec removes that gap note when the work lands.
- [ ] `docs/architecture/l2tp/l2tp-10-metrics.md` - the design document `internal/component/l2tp/metrics.go` and `authradius/metrics.go` declare
  → Decision: per Thomas's 2026-11-24 owner decisions, this spec departs from the page's `ze_l2tp_session_*` naming and 5-label set by renaming the six per-session series to `ze_subscriber_session_*` with an `access_type` (`l2tp` / `pppoe`) label (no compatibility alias) while keeping per-subscriber metrics enabled by default.
  → Constraint: late metrics registry binding through `registry.InjectPluginMetrics` and clean `Stop`/`unwindLocked` detachment must continue to hold for L2TP-only, PPPoE-only, and concurrent daemons.
- [ ] `docs/research/l2tpv2-ze-integration.md` - the design document `acct.go` and `internal/component/l2tp/subsystem.go` declare
  → Constraint: RADIUS accounting stays inside the `l2tp-auth-radius` plugin over the EventBus with no reactor coupling.
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` - what an Accounting-Request carries and what each teardown path reports
  → Constraint: its "What every Accounting-Request carries" table currently omits the six traffic-counter rows (octets, packets, Gigawords) that `buildAcctPacket` sends, and its teardown-cause table omits PPPoE PADT and interface removal/disable; this spec updates both tables.
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - the design document the PPPoE subsystem declares
  → Constraint: kernel PPPoE counters come from the created unit's original generation-bound reader and VPP PPPoE counters come from the session handle's VPP stats segment snapshot; neither reads by interface name.
- [ ] `docs/architecture/doctor-and-health-checks.md` - runtime doctor checks and problem-site doctor codes
  → Constraint: every `Warn`/`Error` log line and `report.RaiseWarningFix` names a code declared on a doctor check (`codeRADIUSAccountingCounters` on `l2tp-radius-accounting`).
- [ ] `docs/architecture/testing/qemu-integration.md` - guest `netnsSelections` discovery for PPPoE functional tests
  → Constraint: `netnsSelections` in `internal/le/test/qemu/netns_linux.go` is an explicit list, so `pppoe-accounting` must be added to `netnsSelections["pppoe"]`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2866.md` - RADIUS Accounting: Start, Stop, and Interim-Update
  → Constraint: RFC 2866 Section 2 and Section 5.13 mark `Acct-Input-Octets`, `Acct-Output-Octets`, `Acct-Input-Packets`, and `Acct-Output-Packets` as `0-1` (optional), confirming that omitting unreadable counters is RFC-compliant.
  → Constraint: RFC 2866 Section 5.10 defines `Acct-Terminate-Cause` (Stop only); PPPoE teardown paths must report honest causes rather than falling back to zero or misattributing peer PADT as NAS Request.
- [ ] `rfc/short/rfc2869.md` - RADIUS Extensions: Interim-Update and Gigaword counters
  → Constraint: `splitGigawords` emits `Acct-Input-Gigawords` and `Acct-Output-Gigawords` only when non-zero (Sections 5.1 and 5.2), and only in Interim-Update and Stop records; this behavior is preserved for both access types.

**Key insights:**
- Original-unit counter capture, zero-on-error omission, and the `doctor-radius-accounting-counters` finding are already landed on HEAD for L2TP (`38267f5e0c`) and in the shared `ppp` layer (`ppp/network_counters.go`).
- Moving `authradius` to `subevents` requires forwarding three existing fields (`BaselineCounters`, `CallingStationID`, `Cause`) through `subscriberBridge` and `pppoe.Subsystem`, plus a transport-neutral counter-capture registration seam in `l2tp/subscriber`.
- On a PPPoE-only daemon, `l2tp.Subsystem` is not registered (`cmd/ze/hub/register_l2tp.go`), so the shared `ze_subscriber_session_*` stats poller must be acquired by both `l2tp.Subsystem` and `pppoe.Subsystem` rather than living inside L2TP startup alone.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `subscribeEventBus` subscribes only to `l2tpevents.SessionIPAssigned`, `l2tpevents.SessionIPReleased`, and `l2tpevents.SessionDown`; `radiusAcct.sessions` keys on `sessionKey{birth, tunnelID, sessionID}`; `bindCapture` calls `l2tp.LookupService().CaptureSession` only; `buildAcctPacket` already omits all six counter attributes when `recordCounters` answers false (never sends zero on failure) and calls `recordCountersOmitted`.
- [ ] `internal/component/l2tp/plugins/authradius/acct_health.go` - `acctHealthSubject` formats `"l2tp:<tid>-<sid> <acctSessID>"`; `recordCountersOmitted` logs a warning with `codeRADIUSAccountingCounters` and raises a runtime doctor warning on `l2tp-radius-accounting`, with no Prometheus counter incremented.
- [ ] `internal/component/l2tp/plugins/authradius/metrics.go` - registers `ze_radius_up`, `ze_radius_auth_sent_total`, `ze_radius_acct_sent_total`, and `ze_radius_interim_sent_total`; registers no counter for omitted accounting counters.
- [ ] `internal/component/l2tp/subscriber/events/events.go` - `SessionIPAssignedPayload` carries `Session`, `Address`, `FramedRoutes`, and `NetworkLifetime` (lacks `BaselineCounters` and `CallingStationID`); `SessionDownPayload` carries `Session`, `Reason`, and `NetworkLifetime` (lacks `Cause`).
- [ ] `internal/component/l2tp/subscriber/session.go` - `Session` holds `L2TPBirth`, `ID`, `AccessType`, `State`, `MAC`, `Username`, `AccessInterface`, `PPPoESID`, `AccessIfIndex`, `TunnelID`, `SessionID`, `PppInterface`, `IPv4Addr`, `ActivatedAt`, `AcctSessionID`, and `PPPKey()`.
- [ ] `internal/component/l2tp/subscriber_bridge.go` - subscribes to `l2tpevents.SessionUp`, `SessionDown`, `SessionIPAssigned`, `SessionIPReleased` and re-emits `subevents` synchronously within the tunnel's order ticket; drops `BaselineCounters`, `CallingStationID`, and `Cause` when re-emitting.
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `onSessionUp`, `onSessionIPAssigned`, `onSessionIPReleased`, and `onSessionDown` update `subscriber.DefaultRegistry` (setting `PppInterface = snap.Identity.InterfaceName` before emitting `SessionIPAssigned`) and emit `subevents`; drops `e.BaselineCounters` on `onSessionIPAssigned` and `e.Cause` on `onSessionDown`; does not capture `e.Control.CaptureNetworkCounters(e.NetworkLifetime)`; does not acquire a per-session Prometheus stats poller.
- [ ] `internal/component/l2tp/pppoe/server.go` and `session.go` - `handlePADT` closes the transport and cancels the PPP attempt without recording a peer-initiated `TerminateCauseUserRequest` override; `Session` does not retain active network-lifetime counter receipts or teardown cause.
- [ ] `internal/component/l2tp/ppp/network_counters.go` and `session_run.go` - already capture `BaselineCounters` before `EventSessionIPAssigned`, capture `TerminalCounters` in `prepareSessionDown`, `prepareLCPDown`, and `closeFilesLocked` before closing the handle (including under `Driver.StopSession`), and expose `SessionControl.CaptureNetworkCounters(lifetime)`.
- [ ] `internal/component/l2tp/metrics.go` and `subsystem.go` - `bindL2TPMetrics` registers `ze_l2tp_session_{state,uptime_seconds,rx_bytes_total,tx_bytes_total,rx_packets_total,tx_packets_total}` with 5 labels (`sid`, `ifname`, `username`, `ip`, `caller_id`); `l2tpStatsPoller` walks only `s.reactors`, updates the four `ze_l2tp_{tunnels,sessions}_*` aggregate gauges and the six per-session series using `readStats(s.PppInterface)`, and keys `prev`/`prevKeys` by `PppInterface`; `Subsystem.Start` starts the poller only when L2TP reactors exist.
- [ ] `cmd/ze/hub/register_l2tp.go` - skips registering `l2tp.Subsystem` when `l2tp` is disabled and has no listeners, while always registering `pppoe.Subsystem`.
- [ ] `internal/le/test/qemu/netns_linux.go` - `netnsSelections["pppoe"]` lists PPPoE functional tests explicitly.

**Behavior to preserve:**
- An L2TP subscriber's RADIUS Accounting-Request attribute set (Start, Interim-Update, Stop, CoA `currentSessionIDs()` correlation, attribute exclusions, endpoint pinning across reload, 10s terminal/record bounds, and IPv4 release omission) stays byte-for-byte identical.
- Creation-bound original-unit counter reading and baseline subtraction stay for accounting; a renamed or reused `pppN` interface never bills another subscriber, and display-counter clears never affect accounting.
- Gigawords stay conditional on exceeding 2^32 (RFC 2869 Sections 5.1 and 5.2), on Interim-Update and Stop only.
- Unavailable accounting counters stay omitted (never reported as zero), logged, and reported via `doctor-radius-accounting-counters` on `l2tp-radius-accounting`.
- `l2tp-auth-radius` stays a plugin over the EventBus with no import of `pppoe` and no reactor coupling.
- L2TP-only metrics (`ze_l2tp_tunnels_active`, `ze_l2tp_sessions_{active,starting,finishing}`, `ze_l2tp_session_lcp_opened_total`, `ze_l2tp_relay_total`, CQM series) and `show l2tp session traffic` stay unchanged.
- Late metrics registry arrival via `registry.InjectPluginMetrics` and race-free `Stop`/`unwindLocked` teardown stay intact.

**Behavior to change:**
- `subevents.SessionIPAssignedPayload` gains `BaselineCounters` and `CallingStationID`; `subevents.SessionDownPayload` gains `Cause`; both `l2tp.subscriberBridge` and `pppoe.Subsystem` populate them.
- `l2tp/subscriber` exposes a transport-neutral `SessionCounterCapture` registration seam; both `l2tp.Subsystem` and `pppoe.Subsystem` register their network-qualified original-unit capturers at start and unregister at stop.
- `authradius` moves its single subscription from `l2tpevents` to `subevents`, keys sessions by `{accessType, birth, subscriberID}`, captures counters through the `subscriber` seam, prefixes health subjects by access type (`l2tp:` / `pppoe:`), and increments `ze_radius_acct_counters_omitted_total{access_type}` whenever counters are omitted.
- PPPoE teardown paths carry explicit `l2tpevents.TerminateCause` values (peer PADT -> `UserRequest`, LCP Terminate -> `UserRequest`, LCP echo timeout -> `LostCarrier`, no-NCP idle bound -> `IdleTimeout`, reload interface removal/disable -> `AdminReset`, subsystem stop -> `AdminReboot`, `Driver.StopSession` -> `NASRequest`, unattributed/failure -> `NASError`).
- The six per-session Prometheus series are renamed from `ze_l2tp_session_*` to `ze_subscriber_session_*` with label set `access_type`, `sid`, `ifname`, `username`, `ip`, `caller_id` (no compatibility alias; enabled by default), polled from `subscriber.DefaultRegistry` by a shared refcounted subscriber stats poller acquired by both `l2tp.Subsystem` and `pppoe.Subsystem`, skipping sessions with empty `PppInterface`, keying per-session state by canonical `Session.ID`, preferring creation-bound transport counter readers over `readStats`, and counting failed poller counter reads on `ze_subscriber_session_stats_read_errors_total{access_type}`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Shared subscriber record and registry | `internal/component/l2tp/subscriber/session.go` (`Session`), `registry.go` (`Registry`), `service.go` (`DefaultRegistry`) | Tracks both L2TP and PPPoE subscriber sessions with `AccessType`, `ID`, `L2TPBirth`, `PppInterface`, `Username`, `IPv4Addr`, `MAC`, and `PPPKey()` | Extends: adds the transport-neutral `SessionCounterCapture` registry (`counters.go`) and drives the shared `ze_subscriber_session_*` stats poller from `DefaultRegistry` |
| Subscriber event namespace and L2TP bridge | `internal/component/l2tp/subscriber/events/events.go` (`SessionIPAssigned`, `SessionIPReleased`, `SessionDown`), `internal/component/l2tp/subscriber_bridge.go` (`subscriberBridge`) | Both PPPoE and L2TP (via the bridge) emit subscriber lifecycle events; pool and route observer already subscribe to `subevents` alone | Extends: adds `BaselineCounters`, `CallingStationID`, and `Cause` to the payloads and forwards them from `subscriberBridge` and `pppoe.Subsystem` so `authradius` moves to a single `subevents` subscription |
| Original-unit PPP counter capture | `internal/component/l2tp/ppp/network_counters.go` (`prepareNetworkCounters`, `captureNetworkTerminal`, `NetworkCounterReceipt`, `SessionControl.CaptureNetworkCounters`), `ppp/session_run.go` (`prepareSessionDown`, `closeFilesLocked`) | Shared PPP driver reads creation-bound baseline before `EventSessionIPAssigned` and captures terminal counters before closing the owner (including under `Driver.StopSession`) for both L2TP and PPPoE | Extends: PPPoE subsystem captures and retains `NetworkCounterReceipt` on assignment and serves live + terminal reads through `pppoe/capture.go`, matching L2TP's `CapturedSession` |
| L2TP session capture | `internal/component/l2tp/session_capture.go` (`Subsystem.CaptureSession`, `CapturedSession.Counters`, `TerminalCounters`) | Network-qualified live and terminal counter capture for L2TP sessions | Extends: registered under `subscriber.AccessL2TP` in the shared `subscriber` counter-capture seam so `authradius` consumes it without transport-specific branches |
| RADIUS accounting plugin and zero-on-failure omission | `internal/component/l2tp/plugins/authradius/acct.go` (`radiusAcct`, `buildAcctPacket`, `liveDelta`, `terminalDelta`), `acct_health.go` (`recordCountersOmitted`, `codeRADIUSAccountingCounters`) | Subscribes to `l2tpevents` only; already omits all six traffic counter attributes on failure/unavailability (never sends zero), logs a warning, and raises `doctor-radius-accounting-counters` on `l2tp-radius-accounting` | Extends: switches subscription to `subevents`, keys records by `{accessType, birth, subscriberID}`, formats health subjects by access type, and increments `ze_radius_acct_counters_omitted_total{access_type}` |
| L2TP stats poller and late metrics binding | `internal/component/l2tp/metrics.go` (`bindL2TPMetrics`, `registerLateL2TPMetrics`, `l2tpStatsPoller`), `subsystem.go` (`startStatsPollerLocked`, `onLateMetricsRegistry`) | Polls `s.reactors` for L2TP aggregate gauges and `ze_l2tp_session_*` series; defers binding via `registry.InjectPluginMetrics` when telemetry starts late | Extends and replaces per-session half: keeps L2TP aggregate gauges on `l2tpStatsPoller`, replaces `ze_l2tp_session_*` with `ze_subscriber_session_*` (`access_type` label) polled from `subscriber.DefaultRegistry` through a refcounted poller acquired by both `l2tp.Subsystem` and `pppoe.Subsystem` |
| VPP PPPoE session handle counters | `internal/component/l2tp/pppoe/dataplane/vpp/session.go` (`session.Counters`, `session.CountersWithin`) | Reads creation-bound counters from the VPP stats segment via the session's `sw_if_index`, returning an error rather than zero on failure | Extends (reused automatically): PPP's `networkCounterLifetime` and `pppoe/capture.go` call `SessionHandle.Counters()`, so VPP PPPoE sessions feed accounting and the poller with no VPP-specific branch |
| Interface statistics with baseline | `internal/component/iface/dispatch.go` (`GetStats`), `internal/component/iface/counters.go` (`baselineStore.applyBaseline`) | Reads Linux netdev statistics by interface name with baseline subtraction | Does not apply to accounting (accounting never reads by name); remains the stats poller's fallback seam when a test or session holds no registered transport counter capture |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "It never needs an edit to a switch, a factory, a field list, a help string or a completion table elsewhere" | agrees | Transport counter capturers register in `l2tp/subscriber` from `l2tp.Subsystem` and `pppoe.Subsystem`; `authradius` looks up by `AccessType` through the registry with no `pppoe` import or per-transport switch, and communicates over the EventBus. |
| `ai/doors/access-security.md` | Boundaries | the door covering `internal/component/l2tp/plugins/authradius/acct.go`, `acct_health.go`, `metrics.go`, `internal/component/l2tp/metrics.go`, `subsystem.go`, `subscriber_bridge.go`, `subscriber/events/events.go`, `subscriber/counters.go`, `pppoe/subsystem.go`, `pppoe/server.go`, `pppoe/session.go`, `pppoe/capture.go` | "Subscriber auth, pool and shaper reach a session through `l2tp/subscriber` handler registries" | agrees | Extends the same `l2tp/subscriber` boundary to accounting events, counter capture registration, and per-session telemetry so both L2TP and PPPoE share one path. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/le/test/qemu/netns_linux.go`, `test/pppoe/pppoe-accounting.ci`, `test/interop-pppoe/scenarios/radius-accounting-pppoe/` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | `test/pppoe/pppoe-accounting.ci` carries `option=needs-linux` and is added to `netnsSelections["pppoe"]` for guest namespace execution. |
| `docs/research/l2tpv2-ze-integration.md` | 6. Plugin Ecosystem | the `// Design:` page of `internal/component/l2tp/plugins/authradius/acct.go` and `internal/component/l2tp/subsystem.go`; a page this spec modifies | "Policy decisions are plugins. The L2TP subsystem provides the protocol engine; plugins control what happens at decision points." | agrees | RADIUS accounting remains in `l2tp-auth-radius` over the EventBus with no reactor coupling; this spec updates the page to reflect subscriber-topic accounting and the shared subscriber stats poller. |
| `docs/architecture/l2tp/subscriber-session-model.md` | Decisions | the `// Design:` page of `subscriber/events/events.go`, `subscriber/counters.go`, `subscriber_bridge.go`, `acct_health.go`; anchored to `acct.go`, `pppoe/subsystem.go`; a page this spec modifies | "A lifecycle consumer subscribes to the subscriber topic alone, never to both topics." | agrees | `authradius` moves from `l2tpevents` to `subevents` alone, matching the pool and route observer; the page's temporary gap note ("PPPoE accounting is not connected to subscriber events") is removed in the same change. |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | What every Accounting-Request carries | anchored to `internal/component/l2tp/plugins/authradius/acct.go`; a page this spec modifies | "Ze emits the session attributes below on every subscriber Accounting-Request." | agrees | PPPoE subscribers emit the same attribute set as L2TP; this spec updates the page's attribute table to document the six traffic counter attributes already sent and adds PPPoE teardown rows to "What each teardown path reports". |
| `docs/architecture/l2tp/l2tp-10-metrics.md` | Decisions | the `// Design:` page of `internal/component/l2tp/metrics.go` and `authradius/metrics.go`; anchored to `internal/component/l2tp/metrics.go` and `subsystem.go`; a page this spec modifies | "The metric families are `ze_l2tp_*` and `ze_radius_*`." | departs | Per Thomas's 2026-11-24 owner decision, the six per-session series are renamed from `ze_l2tp_session_*` to `ze_subscriber_session_*` (pre-release; no compatibility alias) and paired with `ze_subscriber_session_stats_read_errors_total`; L2TP-only tunnel/session aggregate gauges and CQM metrics remain `ze_l2tp_*`. |
| `docs/architecture/l2tp/l2tp-10-metrics.md` | Decisions | the `// Design:` page of `internal/component/l2tp/metrics.go`; anchored to `internal/component/l2tp/metrics.go` and `subsystem.go`; a page this spec modifies | "Per-session labels are `sid`, `ifname`, `username`, `ip` and `caller_id`." | departs | Per Thomas's 2026-11-24 owner decision, per-session series gain an `access_type` (`l2tp` / `pppoe`) label (`access_type`, `sid`, `ifname`, `username`, `ip`, `caller_id`) and remain enabled by default; `l2tp-10-metrics.md` is updated in the same change. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Decisions | the `// Design:` page of `pppoe/subsystem.go`, `pppoe/server.go`, `pppoe/session.go`, `pppoe/capture.go`; a page this spec modifies | "Counters come from the created unit's original generation-bound reader; no interface-name callback or active-backend lookup replaces it." | agrees | PPPoE accounting reads creation-bound counters via PPP's `BaselineCounters`, `SessionControl.Counters()`, and `NetworkCounterReceipt.TerminalCounters(ctx)` through `pppoe/capture.go`, never by interface name. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | a page this spec modifies | "D-1, a VPP API loss with the process alive, is resolved by the VPP lifecycle rules: a live VPP that Ze cannot adopt is killed and restarted, so its exit proof always arrives and retires the retained owners" | agrees | This spec does not modify VPP API loss detection, adoption, or process kill/restart; VPP PPPoE sessions continue to follow VPP lifecycle rules while alive and emit terminal teardown when killed. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | a page this spec modifies | "When Ze adopts the VPP, retained owners are read back by exact identity over the new connection of the same pidfd-proven process, and cleaned there (option C)" | agrees | This spec adds only readonly counter capture over the existing session handle and does not alter retained-owner readback or cleanup authority. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | a page this spec modifies | "Adoption happens only inside the single VPP loss timer: a reconnect to the same pidfd-proven process whose full readback matches (option C) keeps the sessions. A declared loss is never adopted: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This narrows the row "When Ze adopts the VPP" to the single timer, and the D-1 row's "cannot adopt" to every declared loss" | agrees | Superseded by the two-form recovery row below; this spec leaves VPP adoption, process-lease identity, and OD-2 session teardown untouched, only ensuring those teardowns publish their honest `TerminateCause` to RADIUS accounting. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | a page this spec modifies | "D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms. Form (1), inside the reconnect-and-verify limit that follows the failed alive test: the same pidfd-proven process answering with the data Ze configured is enough to adopt it, and the PPPoE sessions are kept. The retained owners are read back by exact identity over the new connection of that process and cleaned there (option C), as part of the background reconciliation; a mismatch it finds is repaired, never treated as a loss. Form (2), a forced restart: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This supersedes the row "Adoption happens only inside the single VPP loss timer" (the single timer, adoption on a full readback match, and "a declared loss is never adopted"), and narrows the D-1 row and the row "When Ze adopts the VPP" to these two forms" | agrees | This spec leaves both VPP recovery forms, pidfd verification, background reconciliation, and OD-2 teardown unchanged; kept sessions continue accounting via their VPP session handles, and forced-restart teardowns emit Accounting-Stop. |
| `docs/architecture/l2tp/followup-l2tp-call.md` | Decisions | anchored to `internal/component/l2tp/pppoe/session.go` and `internal/component/l2tp/pppoe/server.go` | "" | silent | This spec does not touch L2TP outgoing calls, Service-Name relays, or realm relays; `pppoe/session.go` and `pppoe/server.go` change only to retain `NetworkCounterReceipt` and record peer-PADT `TerminateCauseUserRequest` for locally terminated PPPoE sessions. |
| `docs/architecture/iface/logical-name-resolution.md` | The plugin-facing registries take the resolved device too | anchored to `internal/component/l2tp/pppoe/server.go` | "Creation-bound counters do not resolve the child's current name." | agrees | PPPoE accounting reads creation-bound counters via `ppp.SessionControl.Counters()` and `NetworkCounterReceipt.TerminalCounters(ctx)` without resolving any interface name. |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | anchored to `internal/component/l2tp/subsystem.go` | "A site is covered when its code is one that a doctor check declares." | agrees | PPPoE accounting counter omissions reuse `codeRADIUSAccountingCounters` declared on `l2tp-radius-accounting`; stats poller read failures log at `Debug` and increment `ze_subscriber_session_stats_read_errors_total` without adding uncovered warning sites. |
| `docs/architecture/l2tp/l2tp-9-observer.md` | Decisions | anchored to `internal/component/l2tp/subsystem.go` | "" | silent | This spec does not touch the L2TP CQM observer, event rings, or echo RTT buckets; `l2tp/subsystem.go` changes only counter-capturer registration and stats-poller lifecycle. |
| `docs/architecture/testing/qemu-integration.md` | The two entry points | the `// Design:` page of `internal/le/test/qemu/netns_linux.go` | "`netns` uses the explicit named populations in `netnsSelections`; selecting a suite does not run every test in its directory." | agrees | `pppoe-accounting` is added explicitly to `netnsSelections["pppoe"]` in `internal/le/test/qemu/netns_linux.go` so the PPPoE netns suite executes it. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A subscriber session completing NCP address assignment (`ppp.EventSessionIPAssigned`), releasing a family (`ppp.EventSessionIPReleased`), reaching up (`ppp.EventSessionUp`), or ending (`ppp.EventLCPDown` in network phase, `ppp.EventSessionDown`, peer PADT, reload removal/disable, or subsystem Stop):
  - For PPPoE: `handlePADR` -> `ppp.Driver` -> `pppoe.Subsystem.handlePPPEvent` -> `subscriber.DefaultRegistry` + `subevents.*`.
  - For L2TP: `l2tpReactor` -> `l2tpevents.*` -> `l2tp.subscriberBridge` -> `subscriber.DefaultRegistry` + `subevents.*` (synchronously within the tunnel's order ticket).
- Format at entry: `subevents.SessionIPAssignedPayload` (carrying `subscriber.Session`, `Address`, `FramedRoutes`, `NetworkLifetime`, `BaselineCounters`, `CallingStationID`), `subevents.SessionIPReleasedPayload` (`Session`, `Family`, `NetworkLifetime`), and `subevents.SessionDownPayload` (`Session`, `Reason`, `Cause`, `NetworkLifetime`).

### Transformation Path
1. **Subsystem startup and registration:** `l2tp.Subsystem.Start` registers its `subscriber.AccessL2TP` counter capturer (wrapping `CaptureSession`) and acquires the shared subscriber stats poller (while keeping its own `l2tpStatsPoller` for the four `ze_l2tp_{tunnels,sessions}_*` aggregate gauges). `pppoe.Subsystem.startLocked` registers its `subscriber.AccessPPPoE` counter capturer (backed by `pppoe/capture.go`) and acquires the same shared subscriber stats poller so PPPoE-only daemons also run it. Both use `registry.InjectPluginMetrics` when the metrics registry arrives after subsystem start.
2. **Accounting start:** When PPP completes an NCP, PPP reads `BaselineCounters` from the original `SessionHandle` before emitting `EventSessionIPAssigned`. `pppoe.Subsystem.onSessionIPAssigned` captures `e.Control.CaptureNetworkCounters(e.NetworkLifetime)` on the PPPoE admission, updates `subscriber.DefaultRegistry` (with `PppInterface`, `Username`, `IPv4Addr`/`IPv6InterfaceID`, `MAC`), and emits `subevents.SessionIPAssigned` carrying `BaselineCounters` and `CallingStationID` (subscriber MAC). For L2TP, `subscriberBridge.onSessionIPAssigned` forwards `BaselineCounters` and `CallingStationID` onto `subevents.SessionIPAssigned`. `radiusAcct.onSessionIPAssigned` validates `SubscriberID`, keys the record by `{accessType, birth, subscriberID}`, records `counterBase` (marking `countersInvalid` and calling `recordCountersOmitted` if the baseline is unavailable), and launches `runAccounting`.
3. **Counter binding and Interim-Update:** Inside `runAccounting` (never on the EventBus callback), `bindCapture` calls `subscriber.CaptureSessionCounters(selector)` using the session's `AccessType`, `SubscriberID`, `L2TPBirth`, `PPPKey()`, and `NetworkLifetime`. It sends Accounting-Start, then on each interim tick calls `liveDelta()` through the bound capture minus `counterBase`. On failure, unavailability, or reading below baseline, `recordCountersOmitted` logs with `codeRADIUSAccountingCounters`, raises the `l2tp-radius-accounting` finding under `<access_type>:<id> <acctSessID>`, increments `ze_radius_acct_counters_omitted_total{access_type}`, and omits all six traffic counter attributes from the packet.
4. **Family release:** `subevents.SessionIPReleased` for IPv4 clears `sess.peerAddr` so subsequent Interims omit `Framed-IP-Address` until IPCP reopens; IPv6 release leaves IPv4 accounting intact.
5. **Accounting stop:** On `subevents.SessionDown`, `radiusAcct.onSessionDown` claims the session out of `a.sessions`, records `Cause`, duration, and `Event-Timestamp`, and hands off to `runAccounting`. `runAccounting` awaits `TerminalCounters(ctx)` (bounded by `acctTerminalBound` = 10s), subtracts `counterBase`, encodes `Acct-Terminate-Cause` (falling back to `TerminateCauseNASError` only if `Unspecified`), and sends Accounting-Stop. On retirement (plugin stop or RADIUS config removal), `endLive` reads `liveDelta()` and sends `TerminateCauseAdminReboot` Stop.
6. **Per-session Prometheus polling:** On each `ze.l2tp.metrics.poll-interval` tick (default 30s), the shared subscriber stats poller snapshots `subscriber.DefaultRegistry.All()`. For each session with non-empty `PppInterface`, it computes labels `(access_type, sid, ifname, username, ip, caller_id)` (using `PPPKey()` session ID, `PppInterface`, `Username`, IPv4 string, and MAC/Calling-Station-Id or empty), sets `ze_subscriber_session_state` (`authenticating=1`, `configuring=2`, `active=3`, `terminating=4`, `unknown=0`) and `ze_subscriber_session_uptime_seconds`, and reads counters via the registered creation-bound transport counter reader (falling back to `readStats(sess.PppInterface)` when no capturer is registered, as in standalone unit tests). On a read error it increments `ze_subscriber_session_stats_read_errors_total{access_type}` and skips counter delta updates rather than emitting zeros. When a canonical `Session.ID` disappears from `DefaultRegistry`, its six series are deleted on the next poll tick.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| L2TP / PPPoE subsystem → `l2tp/subscriber` registry | `subscriber.DefaultRegistry.Add`/`Remove`/`All` and `subscriber.RegisterCounterCapturer` | No |
| L2TP / PPPoE subsystem → `l2tp-auth-radius` plugin | `subevents` namespace (`SessionIPAssigned`, `SessionIPReleased`, `SessionDown`) over the EventBus | No |
| `l2tp-auth-radius` / stats poller → transport original-unit counter owner | `subscriber.CaptureSessionCounters` -> `l2tp.CapturedSession` (L2TP) or `pppoe.capturedSession` (PPPoE kernel or VPP handle) | No |
| Ze → RADIUS accounting server | UDP port 1813 Accounting-Request (RFC 2866 / RFC 2869), built by `buildAcctPacket` | No |
| Ze → Prometheus scraper | `/metrics` exposition of `ze_subscriber_session_*` (labelled with `access_type`), `ze_radius_acct_counters_omitted_total`, and `ze_subscriber_session_stats_read_errors_total` | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `cmd/`, `test/`, and `docs/` for `ze_l2tp_session_`, `newL2TPStatsPoller`, `l2tpStatsPoller`, `ze.l2tp.metrics.poll-interval`, `subevents.SessionIPAssigned`, `subevents.SessionDown`, and `subevents.SessionIPReleased` (positive control: `internal/component/l2tp/metrics.go:137-142` for `ze_l2tp_session_*`, `internal/component/l2tp/subscriber_bridge.go:95-232` and `internal/component/l2tp/pppoe/subsystem.go:521-716` for `subevents.*`, `internal/component/l2tp/plugins/pool/register.go:148` and `internal/component/l2tp/route_observer.go:79-85` for `subevents` readers). Two internal/observability wires change (the `subscriber` EventBus payloads gain optional backward-compatible fields, and the six per-session Prometheus metric names/labels change per the 2026-11-24 owner decision); the RADIUS Accounting-Request UDP wire format itself is unchanged (PPPoE becomes a second producer of the same RFC 2866/2869 attribute set).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `subscriber` EventBus JSON (`session-ip-assigned`, `session-down`, `session-ip-released`) | writer | `internal/component/l2tp/subscriber_bridge.go` (`onSessionIPAssigned`, `onSessionDown`); `internal/component/l2tp/pppoe/subsystem.go` (`onSessionIPAssigned`, `withdrawSubscriberIdentity`); `internal/test/plugins/fakel2tp/pool.go` | Both production emitters construct `subevents.SessionIPAssignedPayload` and `subevents.SessionDownPayload`; `fakel2tp` emits test `SessionDownPayload` | Change production writers to populate new `omitzero`/`omitempty` fields (`BaselineCounters`, `CallingStationID`, `Cause`); zero values preserve existing test emitters |
| `subscriber` EventBus JSON (`session-ip-assigned`, `session-down`, `session-ip-released`) | reader | `internal/component/l2tp/plugins/authradius/acct.go` (`subscribeEventBus`); `internal/component/l2tp/plugins/pool/register.go` (`onSessionDown`); `internal/component/l2tp/plugins/shaper/shaper.go` (`onSubscriberSessionDown`); `internal/component/l2tp/route_observer.go` (`StartSubscriberRouteObserver`); `internal/component/l2tp/plugins/authradius/coa_health.go`; `internal/plugins/cos/handler.go`; external plugins over SDK EventBus | All existing readers decode `subevents` payloads ignoring unknown fields; `authradius/acct.go` moves from `l2tpevents` to `subevents` | Change `authradius/acct.go` to read new fields; preserve all other readers unchanged (additive JSON fields) |
| Prometheus `/metrics` exposition (`ze_subscriber_session_*`) | writer | `internal/component/l2tp/metrics.go` (`bindL2TPMetrics`, `subscriberStatsPoller.poll`, `deleteSessionSeries`); `internal/component/l2tp/plugins/authradius/metrics.go` (`bindRADIUSMetrics`) | `metrics.go:137-142` registers the six `ze_l2tp_session_*` series; `ze_l2tp_session_lcp_opened_total` (`metrics.go:150`) is separate and L2TP-only | Rename the six per-session series to `ze_subscriber_session_*` with `access_type` label (no compatibility alias, per 2026-11-24 owner decision); add `ze_radius_acct_counters_omitted_total` and `ze_subscriber_session_stats_read_errors_total`; preserve `ze_l2tp_session_lcp_opened_total` and `ze_l2tp_{tunnels,sessions}_*` |
| Prometheus `/metrics` exposition (`ze_subscriber_session_*`) | reader | External Prometheus scrapers and dashboards (outside repo); `internal/component/l2tp/metrics_test.go`; `docs/guide/l2tp.md`; `docs/guide/pppoe.md`; `docs/architecture/l2tp/l2tp-10-metrics.md`; `test/pppoe/pppoe-accounting.ci` | Grep confirmed zero `.ci` or Go code consumers of `ze_l2tp_session_{state,uptime_seconds,rx_*,tx_*}` outside `metrics.go`, `metrics_test.go`, `observability_initiator_test.go` comment, and `docs/guide/l2tp.md` | Update `metrics_test.go`, docs, and new `pppoe-accounting.ci` to the new names and `access_type` label; external scrapers adopt the pre-release rename per owner decision |
| RADIUS Accounting-Request UDP (RFC 2866 / RFC 2869) | writer | `internal/component/l2tp/plugins/authradius/acct.go` (`buildAcctPacket`, `sendAcctPacket`) | Encodes Start, Interim-Update, and Stop packets over UDP 1813 | Preserve wire format byte-for-byte for L2TP; extend producer path to emit identical attribute set for PPPoE sessions |
| RADIUS Accounting-Request UDP (RFC 2866 / RFC 2869) | reader | External RADIUS accounting servers (e.g., FreeRADIUS, outside repo); `test/l2tp/radius-acct-wire.ci`; `test/interop-pppoe/scenarios/radius-accounting-pppoe/` | Existing `radius-acct-wire.ci` proves L2TP wire accounting; new interop scenario proves PPPoE against FreeRADIUS | Preserve existing L2TP reader proof; add FreeRADIUS PPPoE interop proof |

### Integration Points
- `internal/component/l2tp/subscriber/events/events.go` - carries `BaselineCounters`, `CallingStationID`, and `Cause` on the shared subscriber event payloads.
- `internal/component/l2tp/subscriber/counters.go` - transport-neutral counter-capturer registry (`RegisterCounterCapturer`, `UnregisterCounterCapturer`, `CaptureSessionCounters`).
- `internal/component/l2tp/subscriber_bridge.go` - forwards `BaselineCounters`, `CallingStationID`, and `Cause` from `l2tpevents` to `subevents`.
- `internal/component/l2tp/pppoe/subsystem.go`, `server.go`, `session.go`, `capture.go` - captures PPP `NetworkCounterReceipt`, supplies PPPoE teardown causes, registers the PPPoE counter capturer, and acquires the shared subscriber stats poller.
- `internal/component/l2tp/plugins/authradius/acct.go`, `acct_health.go`, `metrics.go` - subscribes to `subevents`, uses `subscriber.CaptureSessionCounters`, formats health subjects per access type, and counts omitted counters on `ze_radius_acct_counters_omitted_total`.
- `internal/component/l2tp/metrics.go` and `subsystem.go` - owns `ze_subscriber_session_*` registration, shared refcounted subscriber stats poller lifecycle, `ze_subscriber_session_stats_read_errors_total`, and L2TP counter-capturer registration.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A PPPoE session's `pppN` interface name and PPP `BaselineCounters` are available before `SessionIPAssigned` is emitted | `pppoe/subsystem.go:532-553, 624-661` sets `sess.PppInterface = snap.Identity.InterfaceName` and calls `DefaultRegistry.Add` before emitting `subevents.SessionIPAssigned`; `ppp/network_counters.go:91-133` reads `BaselineCounters` before emitting `ppp.EventSessionIPAssigned` | Accounting or the poller sees an empty interface or missing baseline | Verified directly in `pppoe/subsystem.go:532-553, 624-661` and `ppp/network_counters.go:91-133` on HEAD | confirmed |
| A-2 | The `subscriber` event family is emitted for L2TP as well as PPPoE, so one subscription replaces the `l2tpevents` one | `l2tp/subscriber_bridge.go:36-44, 66-240` synchronously re-emits `SessionUp`, `SessionDown`, `SessionIPAssigned`, and `SessionIPReleased` onto `subevents` whenever `bus != nil`; `pool` and `route_observer` already subscribe to `subevents` alone | Two subscriptions would be needed and L2TP events could double-fire | Verified directly in `subscriber_bridge.go:36-240`, `l2tp/subsystem.go:232-235`, `pool/register.go:148`, `route_observer.go:79-85` on HEAD (adding `BaselineCounters`, `CallingStationID`, and `Cause` to `subevents` makes forwarding lossless) | confirmed |
| A-3 | Nothing outside `l2tp/subsystem.go` and `l2tp/metrics_test.go` depends on `newL2TPStatsPoller` taking a reactor list | Grep across `internal/`, `cmd/`, `test/`, and `docs/` for `newL2TPStatsPoller` and `l2tpStatsPoller` | Changing poller ownership could break another consumer | Full-tree grep confirmed only `l2tp/metrics.go`, `l2tp/subsystem.go:99,491`, and `l2tp/metrics_test.go:126,139` reference it; L2TP aggregate gauges stay fed by `s.reactors` while per-session series move onto `subscriber.DefaultRegistry` | confirmed |
| A-4 | A RADIUS server tolerates an Accounting-Request that omits `Acct-Input-Octets` / `Acct-Output-Octets` / packet counters rather than sending zero | `rfc/full/rfc2866.txt` §5.13 Table of Attributes marks all four as `0-1`, and §2 states Stop carries "optionally statistics such as elapsed time, input and output octets, or input and output packets"; L2TP already omits them on HEAD (`38267f5e0c`) | Omitting would break billing servers | Verified directly in `rfc/full/rfc2866.txt:1331-1337` and existing tests `TestAccountingCountersUnavailableSnapshots` / `TestAccountingStopOmitsCountersOfAnUnavailableMeasurement` in `authradius/acct_counter_*_test.go` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Moving `authradius` onto `subevents` changes L2TP accounting records (ordering, duplicate Start, missing Stop, lost CoA `Acct-Session-Id` correlation, or dropped `Calling-Station-Id`) | Existing L2TP `authradius` unit tests or `TestL2TPAccountingUnchanged` fail | Assert the complete encoded L2TP attribute list byte-for-byte before and after, verify `subscriberBridge` forwards `BaselineCounters`, `CallingStationID`, and `Cause` synchronously within the tunnel order ticket, and keep `currentSessionIDs()` keyed by `L2TPBirth` |
| R-2 | A PPPoE-only daemon never starts the stats poller because `l2tp.Subsystem` is not registered when `l2tp` is unconfigured (`cmd/ze/hub/register_l2tp.go:65-67`) | `ze_subscriber_session_*` absent from `/metrics` on a PPPoE-only BNG | Acquire/release the shared subscriber stats poller from BOTH `l2tp.Subsystem` and `pppoe.Subsystem` with reference counting and `registry.InjectPluginMetrics` late binding, and test PPPoE-only startup and teardown explicitly |
| R-3 | Per-session Prometheus labels (`access_type`, `sid`, `ifname`, `username`, `ip`, `caller_id`) scale cardinality linearly with subscriber count when PPPoE joins | Scrape size grows with concurrent subscribers | Recorded owner decision (2026-11-24): keep per-subscriber Prometheus metrics enabled by default (matching existing L2TP behavior); delete stale series on the next poll tick after a session leaves `DefaultRegistry`, and key poller state by canonical `Session.ID` so reused `pppN` names never leak orphan series |
| R-4 | A PPPoE Stop record carries `TerminateCauseUnspecified` (0 -> NAS Error) or `NASRequest` for a peer-sent PADT because PPPoE did not preserve the teardown cause | Interop or unit test Stop record carries cause 9 or 10 on PADT | Record `TerminateCauseUserRequest` on the PPPoE admission in `handlePADT` before cancelling PPP, `TerminateCauseAdminReset` on interface removal/disable, `TerminateCauseAdminReboot` on subsystem Stop, `TerminateCauseUserRequest` on network-phase `EventLCPDown`, and forward `e.Cause` from `ppp.EventSessionDown`; assert one cause per teardown path in unit tests |
| R-5 | Omitting unreadable traffic counters could go unnoticed by an operator who only watches RADIUS records | Billing reports gaps without operator alert | Already mitigated for L2TP via `Warn` log + `doctor-radius-accounting-counters` runtime finding (`acct_health.go`); this spec extends that finding with `<access_type>:` subjects for PPPoE and adds `ze_radius_acct_counters_omitted_total{access_type}` so Prometheus alerts fire immediately |
| R-6 | On `Driver.StopSession` or peer PADT, the unit fd closes before the consumer reads final counters, degrading AC-4 into AC-7 | `TestPPPoEStopSessionStopCarriesFinalCounters` sees omitted counters | PPP already runs `captureNetworkTerminal()` before closing the owner (`ppp/session_run.go:624,694`, `38267f5e0c`); PPPoE captures `e.Control.CaptureNetworkCounters(e.NetworkLifetime)` on IP assignment and `pppoe/capture.go` serves `TerminalCounters(ctx)` from that retained receipt so Stop reads the pre-close sample even after `pppN` is gone |
| R-7 | Lock-order deadlock if `bindCapture` or stats polling holds a subsystem mutex while a transport callback or `Subsystem.Stop` runs | `-race` test hangs during concurrent Stop and session assignment | `bindCapture` runs inside `runAccounting` goroutine (never on the EventBus callback), matching L2TP's documented lock-order contract; `pppoe/capture.go` snapshots the admission's `Control` and `NetworkCounterReceipt` under short locks and releases every table/subsystem lock before invoking `Counters()` or `TerminalCounters(ctx)`; the stats poller copies `DefaultRegistry.All()` before any counter read |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | L2TP subscriber billing could lose attributes or mis-key sessions, or Prometheus dashboards scraping per-session metrics could miss sessions or see stale series. The pre-release metric rename (`ze_l2tp_session_*` -> `ze_subscriber_session_*` with `access_type` label, no compatibility alias) is intentional and approved by Thomas on 2026-11-24. |
| How is it reverted? | Single commit revert. No config migration and no RADIUS wire-format change. |
| Who else touches this path? | `plan/release/fixes/spec-subscriber-utilisation-has-no-operator-view.md` depends on this spec; `plan/release/fixes/spec-lifecycle-invariants.md` (D-11 / AC-20) adds RADIUS `Session-Timeout` and `Idle-Timeout` timers for PPPoE; `plan/post-release/features/spec-pppoe-vpp-dataplane.md` (AC-17) exercises VPP session handle counters through the same PPP counter path. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A PPPoE session completing IPCP (`pppoe.Subsystem.onSessionIPAssigned`) | → | `subevents.SessionIPAssigned` → `radiusAcct.onSessionIPAssigned` → `sendAcctStart` (`acct.go`) | `TestPPPoESessionSendsAccountingStart` |
| A PPPoE session torn down by peer PADT or `Driver.StopSession` | → | `pppoe.Server.handlePADT` / `onSessionDown` → `subevents.SessionDown` → `sendAcctStop` (`acct.go`) | `TestPPPoESessionStopCarriesCountersAndCause` |
| A PPPoE session live on a PPPoE-only daemon when the poller runs | → | `pppoe.Subsystem` acquires shared subscriber poller → `subscriber.DefaultRegistry` → `ze_subscriber_session_*` (`metrics.go`) | `TestPollerPublishesPPPoESessionSeriesOnPPPoEOnlyDaemon` |
| A counter read failure or unavailable baseline/terminal sample on PPPoE | → | `buildAcctPacket` → `recordCountersOmitted` (`acct.go`, `acct_health.go`, `authradius/metrics.go`) | `TestPPPoEAccountingOmitsCountersAndCountsFailure` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A PPPoE subscriber completes authentication and address assignment (`pppoe.Subsystem.onSessionIPAssigned`) | An Accounting-Request with `Acct-Status-Type = Start` is sent over UDP 1813 carrying the same identity attributes as L2TP (`User-Name`, `Calling-Station-Id` from subscriber MAC when present, `Event-Timestamp`, `Acct-Session-Id`, `Service-Type`, `Framed-Protocol`, `NAS-Port-Type`, `NAS-Port`, `NAS-IP-Address`/`NAS-Identifier`, `NAS-Port-Id`, `Framed-IP-Address` for IPv4, `Acct-Delay-Time` unless excluded) | Optional attributes omitted when empty (`User-Name` on explicit no-auth, `Framed-IP-Address` on IPv6-only, `Calling-Station-Id` when MAC is empty) or explicitly listed in `attributes exclude`, per RFC 2866 §5 and `bng-1-radius-attributes.md` |
| AC-2 | That PPPoE subscriber passes traffic and the interim timer fires | An `Interim-Update` carries `Acct-Session-Time`, `Acct-Input-Octets`, `Acct-Output-Octets`, `Acct-Input-Packets`, and `Acct-Output-Packets` computed from the session's original PPP unit (`pppoe/capture.go` over `SessionControl.Counters()`, covering both kernel generation-bound reader and VPP session handle counters), never by interface name, minus the baseline PPP captured before `EventSessionIPAssigned` | None: every live PPPoE session with a valid baseline and readable original handle reports its real delta |
| AC-3 | That PPPoE subscriber's byte counters exceed 2^32 octets | `Acct-Input-Gigawords` and/or `Acct-Output-Gigawords` are emitted with `bytes >> 32` and the octet attributes carry `bytes & 0xFFFFFFFF` on Interim-Update and Stop, and are omitted when the gigaword quotient is zero, matching L2TP (RFC 2869 Sections 5.1 and 5.2) | Gigawords omitted when total bytes are below 2^32, as RFC 2869 Sections 5.1 and 5.2 permit and L2TP already does |
| AC-4 | That PPPoE subscriber is torn down (peer PADT, peer LCP Terminate, LCP echo timeout, no-NCP idle bound, operator reload interface removal/disable, `Driver.StopSession`, or subsystem Stop) | A Stop record carries the pre-close terminal counters minus baseline (captured before handle close via `NetworkCounterReceipt.TerminalCounters`), `Acct-Session-Time`, `Event-Timestamp` at end of service, and the honest `Acct-Terminate-Cause` (PADT -> `UserRequest` 1, LCP Terminate -> `UserRequest` 1, LCP echo timeout -> `LostCarrier` 2, no-NCP idle bound -> `IdleTimeout` 4, reload removal/disable -> `AdminReset` 6, subsystem Stop -> `AdminReboot` 7, `Driver.StopSession` -> `NASRequest` 10, unattributed/error -> `NASError` 9) | RADIUS `Session-Timeout` and `Idle-Timeout` timer enforcement for PPPoE is owned by `plan/release/fixes/spec-lifecycle-invariants.md` (D-11 / AC-20); once those timers emit `subevents.SessionDown` with `SessionTimeout` (5) / `IdleTimeout` (4), `authradius` encodes them directly with no further change |
| AC-5 | A PPPoE session is live when the subscriber stats poller runs (on a PPPoE-only daemon or a concurrent L2TP+PPPoE daemon, with per-subscriber metrics enabled by default) | Prometheus exposes `ze_subscriber_session_state`, `ze_subscriber_session_uptime_seconds`, `ze_subscriber_session_rx_bytes_total`, `ze_subscriber_session_tx_bytes_total`, `ze_subscriber_session_rx_packets_total`, and `ze_subscriber_session_tx_packets_total` with label `access_type="pppoe"` alongside `sid`, `ifname`, `username`, `ip`, and `caller_id`, and deletes those series on the next poll tick after teardown; no `ze_l2tp_session_{state,uptime_seconds,rx_*,tx_*}` series are registered | `ze_l2tp_session_lcp_opened_total`, `ze_l2tp_tunnels_active`, `ze_l2tp_sessions_{active,starting,finishing}`, and CQM metrics remain `ze_l2tp_*` because they are L2TP-specific, not shared per-session telemetry |
| AC-6 | An L2TP subscriber runs before and after this change | Its RADIUS Accounting-Request attribute sets (Start, Interim, Stop, IPv4 release omission, CoA `currentSessionIDs()` lookup) are byte-for-byte unchanged, and its per-session Prometheus metrics report the same values under `ze_subscriber_session_*` with `access_type="l2tp"` | Metric series rename from `ze_l2tp_session_*` to `ze_subscriber_session_*` with `access_type="l2tp"` and no compatibility alias is explicitly approved by Thomas's 2026-11-24 owner decision |
| AC-7 | Reading a session's original PPP unit fails, PPP's baseline or terminal measurement is unavailable, or a reading falls below baseline (for PPPoE or L2TP) | All six traffic counter attributes are omitted from the RADIUS record (never sent as zero), a warning is logged with `codeRADIUSAccountingCounters` and session identity, a `doctor-radius-accounting-counters` finding is raised under `<access_type>:... <acctSessID>`, and `ze_radius_acct_counters_omitted_total{access_type}` increments; a poller counter-read failure skips updating counter deltas and increments `ze_subscriber_session_stats_read_errors_total{access_type}` | The accounting record itself is still sent with non-counter attributes (RFC 2866 §2: accounting failures never tear down a session), and a later Interim/Stop carrying valid counters clears the session's doctor finding |
| AC-8 | A subscriber session in `subscriber.DefaultRegistry` still has an empty `PppInterface` when the stats poller runs | The poller skips publishing any `ze_subscriber_session_*` series for that session until `PppInterface` is populated, and publishes real series once it is non-empty | None: sessions with empty `PppInterface` never emit placeholder zero series |
| AC-9 | A PPPoE session and an L2TP session are live concurrently (including with identical numeric `(ifindex, sid)` and `(tunnelID, sessionID)` pairs) | Both produce independent RADIUS accounting streams keyed by `{accessType, birth, subscriberID}` and distinct `ze_subscriber_session_*` series distinguished by `access_type` (`pppoe` vs `l2tp`) and canonical `Session.ID`, with zero cross-attribution of counters or teardown events | None: overlapping numeric wire IDs across access types remain completely isolated |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Bills a PPPoE subscriber from RADIUS accounting | PADR -> PPP -> `pppoe.Subsystem` -> `subevents` -> `authradius` Start, Interim-Update, Stop with real creation-bound octets and terminate cause | `radius-accounting-pppoe` interop scenario + `test/pppoe/pppoe-accounting.ci` |
| 2 | Graphs L2TP and PPPoE subscriber throughput from Prometheus using one unified metric family | `subscriber.DefaultRegistry` -> shared subscriber stats poller -> `ze_subscriber_session_*` with `access_type="pppoe"\|"l2tp"` | `TestPollerPublishesPPPoESessionSeriesOnPPPoEOnlyDaemon`, `TestPollerSeparatesTwoAccessTypes`, and `test/pppoe/pppoe-accounting.ci` scrape assertion |
| 3 | Detects an unreadable subscriber counter rather than silently billing zero | Capture/read error -> attributes omitted from RADIUS record, warning logged, `doctor-radius-accounting-counters` raised, `ze_radius_acct_counters_omitted_total{access_type}` incremented | `TestPPPoEAccountingOmitsCountersAndCountsFailure` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPPPoESessionSendsAccountingStart` | `internal/component/l2tp/plugins/authradius/acct_pppoe_test.go` | AC-1: `subevents.SessionIPAssigned` for PPPoE emits Accounting-Start with MAC `Calling-Station-Id`, IPv4 `Framed-IP-Address`, `NAS-Port`, `NAS-Port-Id`, and `Acct-Session-Id` | |
| `TestPPPoEInterimCarriesRealCounters` | `internal/component/l2tp/plugins/authradius/acct_pppoe_test.go` | AC-2: PPPoE Interim-Update reads creation-bound capture minus baseline (never by interface name) | |
| `TestPPPoECountersAboveGigawordBoundary` | `internal/component/l2tp/plugins/authradius/acct_pppoe_test.go` | AC-3: PPPoE byte counts >= 2^32 emit `Acct-Input-Gigawords` / `Acct-Output-Gigawords` and remainder octets; below 2^32 omit Gigawords | |
| `TestPPPoESessionStopCarriesCountersAndCause` | `internal/component/l2tp/plugins/authradius/acct_pppoe_test.go` | AC-4: Stop awaits terminal counters and encodes `UserRequest` (PADT, LCP Terminate), `LostCarrier` (echo timeout), `IdleTimeout` (no-NCP bound), `AdminReset` (interface removal), `AdminReboot` (stop), `NASRequest` (`StopSession`), and `NASError` (unspecified/error) | |
| `TestPPPoEStopSessionStopCarriesFinalCounters` | `internal/component/l2tp/pppoe/capture_test.go` | AC-4, R-6: a PPPoE session stopped through `Driver.StopSession` or peer PADT delivers a terminal sample captured before handle close through `pppoe/capture.go` | |
| `TestL2TPAccountingUnchanged` | `internal/component/l2tp/plugins/authradius/acct_pppoe_test.go` | AC-6, R-1: L2TP accounting through `subscriberBridge` -> `subevents` produces byte-for-byte identical Start, Interim, IPv4-release, Stop, and CoA `currentSessionIDs()` results | |
| `TestPPPoEAccountingOmitsCountersAndCountsFailure` | `internal/component/l2tp/plugins/authradius/acct_pppoe_test.go` | AC-7: unavailable baseline, live read error, terminal timeout, or counter below baseline omits all six traffic attributes, raises `doctor-radius-accounting-counters` with `pppoe:` subject, and increments `ze_radius_acct_counters_omitted_total{access_type="pppoe"}` | |
| `TestConcurrentL2TPAndPPPoEAccountingNumericIDCollision` | `internal/component/l2tp/plugins/authradius/acct_pppoe_test.go` | AC-9: concurrent L2TP `(tid=5, sid=1)` and PPPoE `(ifindex=5, sid=1)` keep isolated accounting streams, counters, and teardowns | |
| `TestPollerPublishesPPPoESessionSeriesOnPPPoEOnlyDaemon` | `internal/component/l2tp/metrics_test.go` | AC-5, R-2: acquiring the shared subscriber poller without `l2tp.Subsystem` publishes `ze_subscriber_session_*` with `access_type="pppoe"` and deletes series after teardown (including late metrics registry arrival and Stop safety) | |
| `TestPollerSkipsSessionWithoutInterface` | `internal/component/l2tp/metrics_test.go` | AC-8: a registry session with empty `PppInterface` emits no `ze_subscriber_session_*` series | |
| `TestPollerSeparatesTwoAccessTypes` | `internal/component/l2tp/metrics_test.go` | AC-5, AC-6, AC-9: concurrent L2TP and PPPoE sessions publish separate `ze_subscriber_session_*` series with `access_type="l2tp"` and `access_type="pppoe"`, zero cross-attribution, and no legacy `ze_l2tp_session_{state,uptime_seconds,rx_*,tx_*}` series | |
| `TestPollerCountsStatsReadErrorsWithoutZeroing` | `internal/component/l2tp/metrics_test.go` | AC-7: a failed stats read increments `ze_subscriber_session_stats_read_errors_total{access_type}` and does not emit zero counters or corrupt prior snapshots | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `Acct-Input-Octets` / `Acct-Output-Octets` | 0 to 2^32-1 per attribute | 4294967295 | N/A (unsigned) | 2^32 carries 1 into Gigawords and 0 into Octets |
| `Acct-Input-Gigawords` / `Acct-Output-Gigawords` | 0 to 2^32-1 | 4294967295 | N/A (unsigned) | N/A (`uint64 >> 32`) |
| Baseline-corrected counter delta | 0 upward (`curr >= base`) | 0 on idle session | `curr < base` marks session counters invalid and omits attributes (no unsigned underflow) | N/A |
| `ze.l2tp.metrics.poll-interval` | positive Go duration (`parsePollInterval`) | positive duration (default 30s) | `<= 0` or unparseable falls back to 30s default | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-accounting` | `test/pppoe/pppoe-accounting.ci` | An operator connects a PPPoE subscriber (with `option=needs-linux`, listed in `netnsSelections["pppoe"]`), passes traffic, verifies RADIUS Start/Interim/Stop with non-zero octets and terminate cause, and scrapes `/metrics` for `ze_subscriber_session_*` with `access_type="pppoe"` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `radius-accounting-pppoe` | `test/interop-pppoe/scenarios/radius-accounting-pppoe/` | FreeRADIUS + pppd/rp-pppoe client | A real RADIUS server accepts Ze's PPPoE Accounting-Start, Interim-Update, and Stop records with non-zero usage and honest `Acct-Terminate-Cause`, and goes red if the PPPoE accounting subscription is reverted | |

## Files to Modify
- `internal/component/l2tp/subscriber/events/events.go` - add `BaselineCounters` (`l2tpevents.TrafficSample`) and `CallingStationID` (`string`) to `SessionIPAssignedPayload`, and `Cause` (`l2tpevents.TerminateCause`) to `SessionDownPayload`
- `internal/component/l2tp/subscriber_bridge.go` - forward `BaselineCounters`, `CallingStationID`, and `Cause` onto `subevents`
- `internal/component/l2tp/pppoe/session.go` - retain active `NetworkLifetime`, `ppp.NetworkCounterReceipt`, and teardown `TerminateCause` override on PPPoE `Session`
- `internal/component/l2tp/pppoe/server.go` - record `TerminateCauseUserRequest` on peer PADT before cancelling PPP
- `internal/component/l2tp/pppoe/subsystem.go` - capture `ppp.NetworkCounterReceipt` on IP assignment, forward `BaselineCounters`, `CallingStationID` (subscriber MAC), and `Cause` on `subevents`, register/unregister the PPPoE counter capturer, and acquire/release the shared subscriber stats poller
- `internal/component/l2tp/plugins/authradius/acct.go` - replace `l2tpevents` subscription with `subevents`, key sessions by `{accessType, birth, subscriberID}`, capture counters through `subscriber.CaptureSessionCounters`, and preserve byte-for-byte L2TP records
- `internal/component/l2tp/plugins/authradius/acct_health.go` - prefix health subjects by access type (`l2tp:` / `pppoe:`) and increment `ze_radius_acct_counters_omitted_total` from `recordCountersOmitted`
- `internal/component/l2tp/plugins/authradius/metrics.go` - register `ze_radius_acct_counters_omitted_total` labelled by `access_type`
- `internal/component/l2tp/metrics.go` - rename per-session series to `ze_subscriber_session_*` with `access_type` label (no compatibility alias), register `ze_subscriber_session_stats_read_errors_total`, poll `subscriber.DefaultRegistry` keyed by canonical `Session.ID` (preferring creation-bound counter readers over `readStats`), and expose refcounted shared poller acquire/release lifecycle
- `internal/component/l2tp/subsystem.go` - register/unregister L2TP counter capturer in `subscriber` and wire the L2TP aggregate gauge poller and shared subscriber stats poller lifecycle (including late metrics registry binding and clean Stop/unwind)
- `internal/le/test/qemu/netns_linux.go` - add `"pppoe-accounting"` to `netnsSelections["pppoe"]`
- `docs/research/l2tpv2-ze-integration.md` - update accounting event subscription and subscriber stats poller sections
- `docs/architecture/l2tp/l2tp-10-metrics.md` - document the `ze_subscriber_session_*` rename, `access_type` label, default-enabled cardinality decision, shared registry poller, `ze_radius_acct_counters_omitted_total`, and `ze_subscriber_session_stats_read_errors_total`
- `docs/architecture/l2tp/subscriber-session-model.md` - remove the `"PPPoE accounting is not connected to subscriber events."` gap note and document the shared counter-capture seam and extended `subevents` accounting fields
- `docs/architecture/l2tp/bng-1-radius-attributes.md` - add the six traffic-counter rows to "What every Accounting-Request carries" and add PPPoE teardown paths to "What each teardown path reports"
- `docs/architecture/l2tp/bng-5-pppoe.md` - document PPPoE accounting event forwarding, counter-capture registration, and shared subscriber stats poller lifecycle
- `docs/guide/l2tp.md` and `docs/guide/pppoe.md` - update per-session metric names to `ze_subscriber_session_*` with `access_type`, document `ze_radius_acct_counters_omitted_total` and `ze_subscriber_session_stats_read_errors_total`, and document PPPoE RADIUS accounting
- `rfc/short/rfc2866.md` and `rfc/short/rfc2869.md` - record PPPoE accounting proof and source anchors on the requirement rows newly proven for both access types

## Files to Create
- `internal/component/l2tp/subscriber/counters.go` - transport-neutral `SessionCounterCapture`, `SessionCounterSelector`, and `RegisterCounterCapturer` / `UnregisterCounterCapturer` / `CaptureSessionCounters` registry
- `internal/component/l2tp/pppoe/capture.go` - PPPoE network-qualified original-unit counter capture over the admission's `ppp.SessionControl` and `ppp.NetworkCounterReceipt`
- `internal/component/l2tp/plugins/authradius/acct_pppoe_test.go` - PPPoE and concurrent L2TP+PPPoE RADIUS accounting unit tests
- `internal/component/l2tp/pppoe/capture_test.go` - PPPoE capture and `Driver.StopSession` pre-close terminal counter unit tests
- `test/pppoe/pppoe-accounting.ci` - functional test verifying PPPoE RADIUS accounting and `ze_subscriber_session_*` Prometheus metrics
- `test/interop-pppoe/scenarios/radius-accounting-pppoe/` - FreeRADIUS + pppd/rp-pppoe interop scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No operator leaf or RPC is added; accounting is already configured under `l2tp auth radius` and per-subscriber metrics stay enabled by default per Thomas's 2026-11-24 decision |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | Operator-facing counter views are owned by `plan/release/fixes/spec-subscriber-utilisation-has-no-operator-view.md` |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/pppoe/pppoe-accounting.ci` (and `netnsSelections["pppoe"]` in `internal/le/test/qemu/netns_linux.go`) |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | Reuses existing `ze.l2tp.metrics.poll-interval` (`internal/component/l2tp/config.go`) |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, kernel module, binary, or sysctl; netlink/PPP unit handles and RADIUS UDP socket already have doctor checks |
| Doctor code per problem class | Yes | Reuses `codeRADIUSAccountingCounters` (`doctor-radius-accounting-counters`) declared on runtime check `l2tp-radius-accounting` (`internal/component/l2tp/plugins/authradius/acct_health.go`, `doctor_codes.go`) with `<access_type>:` subjects; poller read errors log at `Debug` and step `ze_subscriber_session_stats_read_errors_total` |
| Prometheus counters/metrics | Yes | Renames six per-session series to `ze_subscriber_session_{state,uptime_seconds,rx_bytes_total,tx_bytes_total,rx_packets_total,tx_packets_total}` with labels `access_type`, `sid`, `ifname`, `username`, `ip`, `caller_id` (`internal/component/l2tp/metrics.go`); adds `ze_subscriber_session_stats_read_errors_total{access_type}` (`metrics.go`) and `ze_radius_acct_counters_omitted_total{access_type}` (`authradius/metrics.go`) |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` entry rendering `docs/features.md` (PPPoE subscriber accounting and telemetry) |
| 2 | Config syntax changed? | No | No YANG config leaf added or changed |
| 3 | CLI command added/changed? | No | Operator CLI views belong to `plan/release/fixes/spec-subscriber-utilisation-has-no-operator-view.md` |
| 4 | API/RPC added/changed? | No | No RPC change |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if it names `l2tp-auth-radius` event subscriptions |
| 6 | Has a user guide page? | Yes | `docs/guide/pppoe.md` and `docs/guide/l2tp.md` |
| 7 | Wire format changed? | No | RADIUS Accounting-Request attribute wire format is unchanged; PPPoE becomes a second producer |
| 8 | Plugin SDK/protocol changed? | No | EventBus protocol unchanged; additive optional JSON fields on existing `subscriber` events |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2866.md`, `rfc/short/rfc2869.md` (with source anchors and discrimination records), and regenerated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/labs/pppoe-interop.md` for `pppoe-accounting.ci` and `radius-accounting-pppoe` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (accel-ppp per-session PPPoE accounting/telemetry parity) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/l2tp-10-metrics.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/research/l2tpv2-ze-integration.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/architecture/l2tp/l2tp-10-metrics.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` (`l2tp-auth-radius` subscribes to `subscriber` events) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED via `./le spec citation anchors`: modified pages (`docs/research/l2tpv2-ze-integration.md`, `docs/architecture/l2tp/l2tp-10-metrics.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md`) are updated; advisory mentions in `docs/architecture/doctor-and-health-checks.md` (`kernelDataPlane` only), `docs/architecture/l2tp/l2tp-9-observer.md` (CQM observer lifecycle only), `docs/architecture/l2tp/followup-l2tp-call.md` (Service-Name and realm relays only), `docs/architecture/iface/logical-name-resolution.md` (access preparation and logical-name resolution only), `docs/guide/configuration.md` (L2TP config block only), `docs/guide/monitoring.md` (contained panics task table only), and `docs/architecture/testing/qemu-integration.md` (`netnsSelections` mechanism unchanged) are unaffected |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/l2tp/bng-1-radius-attributes.md` Accounting-Request and teardown tables, plus `docs/guide/l2tp.md` per-session metrics table |

## Design presentation

### What changes

PPPoE subscribers now produce the same RADIUS accounting records (Start, Interim-Update, Stop with creation-bound octets, packets, RFC 2869 Gigawords, and honest `Acct-Terminate-Cause`) and the same per-session Prometheus metrics as L2TP subscribers, through one shared `subscriber` event and registry path rather than parallel L2TP-only wiring. Per Thomas's 2026-11-24 decisions, the six per-session Prometheus series are renamed from `ze_l2tp_session_*` to `ze_subscriber_session_*` with an `access_type` (`l2tp` / `pppoe`) label (no compatibility alias), remain enabled by default, and run on L2TP-only, PPPoE-only, and concurrent BNG daemons alike. Any counter read failure or unavailable baseline/terminal sample omits the six traffic counter attributes (never reporting zero), logs and raises `doctor-radius-accounting-counters` with `<access_type>:` identity, and steps `ze_radius_acct_counters_omitted_total{access_type}` (or `ze_subscriber_session_stats_read_errors_total{access_type}` on the stats poller).

### Before and after

1. **PPPoE subscriber RADIUS accounting**
   - **Before:** A PPPoE subscriber authenticates and passes traffic, and FreeRADIUS receives zero Accounting-Request packets because `l2tp-auth-radius` subscribes only to `l2tpevents`.
   - **After:** The PPPoE subscriber emits Accounting-Start on IP assignment (carrying subscriber MAC as `Calling-Station-Id`, assigned IPv4 as `Framed-IP-Address`, `NAS-Port`, `NAS-Port-Id`, and `Acct-Session-Id`), cumulative Interim-Updates from the original PPP unit (or VPP session handle) minus baseline with RFC 2869 Gigawords above 2^32, and Accounting-Stop on teardown with pre-close terminal counters and honest `Acct-Terminate-Cause` (`User-Request` for PADT or LCP Terminate, `Lost-Carrier` for echo timeout, `Idle-Timeout` for no-NCP bound, `Admin-Reset` for interface removal/disable, `Admin-Reboot` for stop, `NAS-Request` for `StopSession`).

2. **Per-session Prometheus metrics (PPPoE-only, L2TP-only, and concurrent BNG)**
   - **Before:** `/metrics` exposes `ze_l2tp_session_{state,uptime_seconds,rx_bytes_total,tx_bytes_total,rx_packets_total,tx_packets_total}{sid,ifname,username,ip,caller_id}` for L2TP sessions only; PPPoE sessions never appear, and a PPPoE-only daemon never starts a stats poller.
   - **After:** `/metrics` exposes `ze_subscriber_session_{state,uptime_seconds,rx_bytes_total,tx_bytes_total,rx_packets_total,tx_packets_total}{access_type="l2tp"|"pppoe",sid,ifname,username,ip,caller_id}` for both access types on L2TP-only, PPPoE-only, and concurrent daemons, enabled by default, with no `ze_l2tp_session_{state,uptime_seconds,rx_*,tx_*}` aliases. L2TP-only aggregate gauges (`ze_l2tp_tunnels_active`, `ze_l2tp_sessions_*`), `ze_l2tp_session_lcp_opened_total`, and CQM metrics remain unchanged.

3. **Unreadable counters**
   - **Before:** L2TP accounting omits unreadable counters and raises `doctor-radius-accounting-counters` under a hardcoded `l2tp:` subject with no Prometheus counter; PPPoE produces no accounting at all; poller read errors are silently skipped with no counter.
   - **After:** Both L2TP and PPPoE omit all six traffic attributes on unavailable/failed/underflow reads (never sending zero), raise `doctor-radius-accounting-counters` under `l2tp:...` or `pppoe:...`, and increment `ze_radius_acct_counters_omitted_total{access_type}`. Poller read failures skip counter delta updates and increment `ze_subscriber_session_stats_read_errors_total{access_type}`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Add a second `subevents` subscription beside the existing `l2tpevents` subscription in `authradius` | Violates `docs/architecture/l2tp/subscriber-session-model.md` ("A lifecycle consumer subscribes to the subscriber topic alone, never to both topics"), duplicates accounting lifecycle code, and risks double-delivering L2TP events via `subscriberBridge`. |
| Keep `ze_l2tp_session_*` series names or publish compatibility aliases alongside `ze_subscriber_session_*` | Explicitly rejected by Thomas's 2026-11-24 owner decision (Ze is pre-release; an `l2tp` prefix over PPPoE sessions is misleading, and duplicate aliases double cardinality). |
| Gate per-subscriber Prometheus metrics behind an opt-in config flag | Explicitly rejected by Thomas's 2026-11-24 owner decision (keep per-subscriber metrics enabled by default, matching existing L2TP behavior). |
| Have `authradius` import `internal/component/l2tp/pppoe` and switch on `AccessType` to capture counters | Violates `ai/doors/system.md` registration-over-hardcoding and couples `authradius` to every future access transport; registering transport capturers in `l2tp/subscriber` keeps `authradius` transport-agnostic. |
| Start the subscriber stats poller only from `l2tp.Subsystem.Start` | Fails on PPPoE-only BNG deployments where `cmd/ze/hub/register_l2tp.go` does not register `l2tp.Subsystem`; refcounted acquisition by both `l2tp.Subsystem` and `pppoe.Subsystem` covers all three deployment modes cleanly. |
| Read accounting counters from `iface.GetStats` by `pppN` interface name | Already rejected on HEAD (`38267f5e0c`): interface names do not survive renames or unit-number reuse and cannot read VPP PPPoE session handles. |

### Documents departed from

- `docs/architecture/l2tp/l2tp-10-metrics.md`, section `Decisions`:
  - Departs from `"The metric families are `ze_l2tp_*` and `ze_radius_*`."` — after this work, the page documents the `ze_subscriber_session_*` family (`state`, `uptime_seconds`, `rx_bytes_total`, `tx_bytes_total`, `rx_packets_total`, `tx_packets_total`, plus `ze_subscriber_session_stats_read_errors_total`) alongside the L2TP-only `ze_l2tp_*` family and `ze_radius_*` (including `ze_radius_acct_counters_omitted_total`), with no `ze_l2tp_session_*` compatibility aliases (per Thomas's 2026-11-24 owner decision).
  - Departs from `"Per-session labels are `sid`, `ifname`, `username`, `ip` and `caller_id`."` — after this work, the page documents the six-label set `access_type`, `sid`, `ifname`, `username`, `ip`, and `caller_id`, with `access_type` carrying `l2tp` or `pppoe`, enabled by default (per Thomas's 2026-11-24 owner decisions).

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- extend `subevents` payloads (`BaselineCounters`, `CallingStationID`, `Cause`), add `subscriber/counters.go` registration seam, forward fields from `subscriberBridge` and `pppoe.Subsystem`, and move `authradius` subscription to `subevents` with failing wiring test first
   - Tests: `TestPPPoESessionSendsAccountingStart`
   - Files: `internal/component/l2tp/subscriber/events/events.go`, `internal/component/l2tp/subscriber/counters.go`, `internal/component/l2tp/subscriber_bridge.go`, `internal/component/l2tp/pppoe/subsystem.go`, `internal/component/l2tp/plugins/authradius/acct.go`
   - Verify: `TestPPPoESessionSendsAccountingStart` fails before switching subscription and passes once wired; run `./le test unit pkg path ./internal/component/l2tp/plugins/authradius/` (main thread)
2. **Phase: PPPoE original-unit counter capture, teardown causes, and L2TP parity** -- implement `pppoe/capture.go`, retain `NetworkCounterReceipt` and teardown cause overrides on `pppoe/session.go` and `server.go`, register both L2TP and PPPoE capturers, and verify Start/Interim/Gigawords/Stop and byte-for-byte L2TP parity
   - Tests: `TestPPPoEInterimCarriesRealCounters`, `TestPPPoECountersAboveGigawordBoundary`, `TestPPPoESessionStopCarriesCountersAndCause`, `TestPPPoEStopSessionStopCarriesFinalCounters`, `TestL2TPAccountingUnchanged`, `TestConcurrentL2TPAndPPPoEAccountingNumericIDCollision`
   - Files: `internal/component/l2tp/pppoe/capture.go`, `internal/component/l2tp/pppoe/session.go`, `internal/component/l2tp/pppoe/server.go`, `internal/component/l2tp/pppoe/subsystem.go`, `internal/component/l2tp/subsystem.go`, `internal/component/l2tp/plugins/authradius/acct.go`
   - Verify: L2TP encoded attribute set is asserted byte-for-byte unchanged and PPPoE `Driver.StopSession` delivers pre-close terminal counters; run `./le test unit pkg path ./internal/component/l2tp/pppoe/` and `./le test unit pkg path ./internal/component/l2tp/plugins/authradius/` (main thread)
3. **Phase: Omitted-counter observability** -- format `acctHealthSubject` by access type and add `ze_radius_acct_counters_omitted_total{access_type}`
   - Tests: `TestPPPoEAccountingOmitsCountersAndCountsFailure`
   - Files: `internal/component/l2tp/plugins/authradius/acct_health.go`, `internal/component/l2tp/plugins/authradius/metrics.go`
   - Verify: unavailable baseline, live error, terminal timeout, and underflow omit all six attributes, raise `doctor-radius-accounting-counters` with `pppoe:`/`l2tp:` subject, and step `ze_radius_acct_counters_omitted_total`
4. **Phase: Shared subscriber stats poller and metric rename** -- rename `ze_l2tp_session_*` to `ze_subscriber_session_*` with `access_type` label (no alias), add `ze_subscriber_session_stats_read_errors_total{access_type}`, move per-session polling onto `subscriber.DefaultRegistry` keyed by `Session.ID`, and wire refcounted acquire/release from both `l2tp.Subsystem` and `pppoe.Subsystem`
   - Tests: `TestPollerPublishesPPPoESessionSeriesOnPPPoEOnlyDaemon`, `TestPollerSkipsSessionWithoutInterface`, `TestPollerSeparatesTwoAccessTypes`, `TestPollerCountsStatsReadErrorsWithoutZeroing` (plus existing late-metrics and concurrency tests in `metrics_test.go`)
   - Files: `internal/component/l2tp/metrics.go`, `internal/component/l2tp/subsystem.go`, `internal/component/l2tp/pppoe/subsystem.go`
   - Verify: run `./le test unit pkg path ./internal/component/l2tp/` and `./le test unit pkg path ./internal/component/l2tp/pppoe/` (main thread)
5. **Phase: End-to-end proof, RFC records, and docs** -- add `test/pppoe/pppoe-accounting.ci` + `netnsSelections["pppoe"]`, add `test/interop-pppoe/scenarios/radius-accounting-pppoe/`, prove red-then-green against FreeRADIUS, record RFC discrimination entries, and update all listed docs
   - Tests: `test/pppoe/pppoe-accounting.ci`, `radius-accounting-pppoe`
   - Files: `test/pppoe/pppoe-accounting.ci`, `internal/le/test/qemu/netns_linux.go`, `test/interop-pppoe/scenarios/radius-accounting-pppoe/`, `rfc/short/rfc2866.md`, `rfc/short/rfc2869.md`, and docs listed in Files to Modify
   - Verify: run `./le test qemu ci test test/pppoe/pppoe-accounting.ci`, `./le test deployment docker-pppoe-accel`, `./le rfc record` / `./le rfc check`, and `./le doc check all` (main thread)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-9 has an implementation at file and symbol and a passing test |
| Feature completeness | A PPPoE subscriber reaches both RADIUS accounting and `ze_subscriber_session_*` Prometheus metrics on both PPPoE-only and concurrent L2TP+PPPoE daemons |
| Correctness | L2TP accounting records are byte-for-byte unchanged; PPPoE Stop carries pre-close terminal counters and honest `Acct-Terminate-Cause`; baseline subtraction applies per network lifetime |
| Naming | The six per-session series are named `ze_subscriber_session_*` with `access_type` (`l2tp` / `pppoe`) and no compatibility alias; L2TP-only gauges remain `ze_l2tp_*` |
| Data flow | One `subevents` subscription in `authradius` and one shared `subscriber` counter-capturer registry; no `pppoe` import in `authradius` |
| Operator errors and doctor codes | `recordCountersOmitted` logs and raises `codeRADIUSAccountingCounters` declared on `l2tp-radius-accounting` with `Fix`; poller read errors log at `Debug` and increment `ze_subscriber_session_stats_read_errors_total` |
| Rule: `ai/rules/principles.md` | An unreadable counter is never reported as zero on RADIUS or Prometheus |
| Rule: `ai/rules/rfc-compliance.md` | `rfc/short/rfc2866.md` and `rfc/short/rfc2869.md` claim and prove behavior for both L2TP and PPPoE with recorded red/green discrimination |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| PPPoE accounting reaches FreeRADIUS | `radius-accounting-pppoe` interop scenario recorded red (reverted subscription) and green |
| PPPoE functional path (accounting + `/metrics`) | `./le test qemu ci test test/pppoe/pppoe-accounting.ci` passes |
| L2TP accounting unchanged | `TestL2TPAccountingUnchanged` and existing `internal/component/l2tp/plugins/authradius/` suite pass |
| Shared poller covers L2TP and PPPoE (including PPPoE-only) | `TestPollerPublishesPPPoESessionSeriesOnPPPoEOnlyDaemon` and `TestPollerSeparatesTwoAccessTypes` pass; grep confirms zero `ze_l2tp_session_{state,uptime_seconds,rx_*,tx_*}` registrations remain |
| No zero-on-failure on RADIUS or Prometheus | `TestPPPoEAccountingOmitsCountersAndCountsFailure` and `TestPollerCountsStatsReadErrorsWithoutZeroing` pass |
| RFC summaries and docs current | `./le rfc check` and `./le doc check all` pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Counters come from kernel/VPP handles, never from peer frames; peer-supplied identities (`Username`, `MAC`, `CallingStationID`) are already validated at admission and encoded via `AppendTextAttr` (omitting zero-length text per RFC 2866 §5) |
| Fail-closed guard | Unreadable, missing, or underflowing counters omit all six RADIUS traffic attributes rather than emitting a billable zero, and poller read errors never advance counters with zero |
| Error leakage | Log lines and doctor findings name the access type, session identity, and error, never subscriber traffic content or shared secrets |
| Resource exhaustion | Stats poller copies `subscriber.DefaultRegistry.All()` under a short RLock and performs all counter reads without holding any registry, table, or subsystem mutex; series for gone sessions are deleted on the next poll tick |
| Authorization | No new RADIUS attribute is emitted beyond what L2TP already sends, and `attributes exclude` continues to filter the finished list once per builder |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood -> RESEARCH |
| Lint failure | Fix inline. If architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The subscriber layer (`subscriber.DefaultRegistry` and `subevents`) was built to unify L2TP and PPPoE, and the shared PPP driver (`ppp/network_counters.go`) already captures creation-bound baseline and pre-close terminal counters for both. Connecting accounting and per-session telemetry to that shared layer eliminates the two remaining L2TP-only wires.
- Checking `cmd/ze/hub/register_l2tp.go` revealed that a PPPoE-only BNG never registers `l2tp.Subsystem`, so moving the per-session poller onto `subscriber.DefaultRegistry` inside `l2tp.Subsystem.Start` alone would still leave PPPoE-only deployments with zero per-session metrics; refcounted acquisition by both `l2tp.Subsystem` and `pppoe.Subsystem` closes that gap.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `lifetime-ends-before-its-last-reader.md` | 2026-10-28 | `Driver.StopSession` closes pppN before session-down reaches the counter reader (AC-4, R-6) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Consume `subevents` alone in `authradius` and register transport counter capturers in `l2tp/subscriber` | Add a second `subevents` subscription beside `l2tpevents`, or import `pppoe` into `authradius` | Honors `subscriber-session-model.md` ("subscribe to the subscriber topic alone"), prevents duplicate L2TP deliveries, and keeps `authradius` decoupled from transport packages via registration. |
| Rename `ze_l2tp_session_*` to `ze_subscriber_session_*` with `access_type` (`l2tp` / `pppoe`) label and no compatibility alias, enabled by default | Keep `ze_l2tp_session_*`, add a parallel `ze_pppoe_session_*` family, keep compatibility aliases, or gate behind an opt-in flag | Settled by Thomas's 2026-11-24 owner decisions: Ze is pre-release, one unified family avoids duplicate dashboards, no alias avoids doubled cardinality, and default-on matches existing L2TP behavior. |
| Share one refcounted subscriber stats poller acquired by both `l2tp.Subsystem` and `pppoe.Subsystem` | Start the poller only from `l2tp.Subsystem.Start` | On a PPPoE-only BNG, `l2tp.Subsystem` is not registered (`cmd/ze/hub/register_l2tp.go`), so both subsystems must be able to start and cleanly release the shared poller (including late metrics registry arrival). |
| Omit all six traffic counter attributes on failure/unavailability and step `ze_radius_acct_counters_omitted_total{access_type}` | Send zero or a synthetic sentinel | RFC 2866 §2 and §5.13 make traffic counters optional (`0-1`); L2TP already omits them and raises `doctor-radius-accounting-counters`; adding the Prometheus counter gives immediate alertability across both access types. |

## Known Limitations
- Operator-facing counter views (`show subscriber`, per-session detail counters, web UI, gNMI) are NOT in this spec; they are owned by `plan/release/fixes/spec-subscriber-utilisation-has-no-operator-view.md`, which depends on this spec.
- RADIUS `Session-Timeout` and `Idle-Timeout` timer enforcement for PPPoE subscribers is owned by `plan/release/fixes/spec-lifecycle-invariants.md` (D-11 / AC-20); this spec ensures `subevents.SessionDownPayload.Cause` carries `TerminateCauseSessionTimeout` and `TerminateCauseIdleTimeout` through to RADIUS Stop once those timers fire.
- The shaper keeps no dropped-byte counts (`internal/component/traffic/model.go` holds rates and burst only), so shaper drop counters are not part of this spec.
- Per-subscriber Prometheus label cardinality remains enabled by default per Thomas's 2026-11-24 owner decision.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 2866 §2, §4.1, §5, §5.5, §5.10, §5.13 | Both L2TP and PPPoE sessions send Accounting-Start, Interim-Update, and Stop with unique matching `Acct-Session-Id`, NAS identity, actual assigned IPv4 `Framed-IP-Address`, honest Stop-only `Acct-Terminate-Cause`, omitted zero-length text, and omitted unreadable traffic counters | `l2tp auth radius attributes exclude` suppresses operator-chosen optional attributes while refusing mandatory ones (`Acct-Status-Type`, `Acct-Session-Id`, NAS identity) | `TestPPPoESessionSendsAccountingStart`, `TestPPPoEInterimCarriesRealCounters`, `TestPPPoESessionStopCarriesCountersAndCause`, `TestL2TPAccountingUnchanged`, `TestPPPoEAccountingOmitsCountersAndCountsFailure`, `radius-accounting-pppoe` | Existing `exclude_test.go` and `rfc2866_attribute_table_test.go` tests preserved for both access types |
| RFC 2869 §2.1, §5.1, §5.2, §5.3, §5.17 | Interim-Update carries cumulative counters (`acct-interval` overrides `Acct-Interim-Interval`); Gigawords emitted on Interim/Stop only when non-zero; `Event-Timestamp` and `NAS-Port-Id` encoded identically for L2TP and PPPoE | Operator-configured `acct-interval` and `nas-port-id-format` templates | `TestPPPoECountersAboveGigawordBoundary`, `TestPPPoEInterimCarriesRealCounters` | Existing `acct_interval_test.go`, `rfc2869_interim_interval_precedence_test.go`, `nasportid_test.go` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/l2tp/`, `./le test unit pkg path ./internal/component/l2tp/pppoe/`, `./le test unit pkg path ./internal/component/l2tp/plugins/authradius/`, `./le test unit pkg path ./internal/component/l2tp/subscriber/`, `./le test qemu ci test test/pppoe/pppoe-accounting.ci`, `./le test deployment docker-pppoe-accel`, `./le rfc check`, `./le doc check all`, and the scoped checks `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including outbound and inbound registration over hardcoding
- [ ] Critical Review passes (all checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + journal row resolution in `plan/journal/lifetime-ends-before-its-last-reader.md`
- [ ] **Commit B:** `remove plan/release/fixes/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
