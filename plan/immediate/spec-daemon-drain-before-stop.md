# Spec: daemon-drain-before-stop

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision, 2026-10-13: "we need a drain phase for every protocol adding routes to the FIB/RIB" before the daemon stops.

Today a `ze` stop tears every routing protocol down at once: BGP sends Cease to every peer inside a 1 s budget, OSPF and IS-IS adjacencies drop when the plugin connection closes, and nothing tells the network to move traffic away first. Every neighbour loses its routes through Ze at the same instant and reconverges while traffic is black-holed. RFC 8326 (BGP graceful shutdown), RFC 6987 (OSPF MaxLinkMetric) and RFC 3787 (IS-IS overload) each describe how a router tells its neighbours to move traffic away before it leaves, and Ze performs none of them at stop.

Goal: a daemon-wide drain phase. After config changes are fenced and before any plugin is torn down, the daemon sends one `drain` RPC, in parallel, to every plugin that registered for it through the SDK. Each protocol makes itself unattractive (BGP GRACEFUL_SHUTDOWN re-advertisement and LOCAL_PREF 0, OSPF max-metric, IS-IS overload), waits its configured time, and answers. The daemon bounds the phase with a ceiling below the systemd stop timeout, refuses a budget or a configured wait above that ceiling, lets a second signal abort the drain, and skips the drain on restart/reboot, which take the graceful-restart path.

`plan/immediate/spec-ospf-max-metric-startup-shutdown-windows.md` depends on this spec: its Q-1 (where the `on-shutdown` window runs) is answered by this spec's drain phase, and its interim refusal of `on-shutdown` above 0 (its AC-11) is replaced by AC-11 here.

## Owner Questions

Nothing below is approved. Each row needs the owner's answer before `/ze-implement` starts; the recommendation is the reading this design is written under.

| # | Question | Options | Recommendation | Answer |
|---|----------|---------|----------------|--------|
| Q-1 | Daemon drain ceiling, and the systemd `TimeoutStopSec` it implies | (a) ceiling 60 s, teardown margin 30 s, `TimeoutStopSec=90`; (b) ceiling 100 s like FRR's OSPF cap, `TimeoutStopSec=130`; (c) a config leaf for the ceiling | (a): 90 s is systemd's default today, so an installed unit keeps its stop time; the teardown after the drain is about 18 s worst case (R-5). (c) rejected: the unit is generated at install time and a configurable ceiling drifts from it | open |
| Q-2 | Operator `request drain` for maintenance without stopping | (a) `request drain` + `request drain cancel` runtime verbs in this spec, reusing the drain RPC; (b) a config leaf (`system maintenance drain true`), which survives a restart like FRR `bgp graceful-shutdown` and EOS maintenance units; (c) its own spec | (c): every vendor puts maintenance drain behind its own command with an undo and a status; it needs an "undrain" RPC this spec does not otherwise need. The drain RPC carries a `reason` so either form reuses it | open |
| Q-3 | Default of each protocol's drain wait | (a) 0 everywhere (phase exists, stop as fast as today until configured); (b) non-zero default for BGP and the IGPs | (a): a drain lengthens every stop, including in tests and labs; FRR, BIRD, GoBGP, Junos default to no stop-time drain (research §1.2) | open |
| Q-4 | RFC 8326 is `Enrolment: blocked` and the RFC scope was frozen on 2026-10-08 | (a) this owner decision unfreezes RFC 8326 sender side: summary `Support` row moves from `-`, extraction walk owed; (b) implement the behaviour, leave the ledger untouched | (a): a sender implemented with `Support -` is a false public claim the other way. The receiver side (Section 4.1 inbound LOCAL_PREF on received GRACEFUL_SHUTDOWN) stays out unless the owner adds it | open |
| Q-5 | Deleting a protocol by commit (`delete ospf`, BGP removed at reload) | (a) stop only, as today; (b) drain first (FRR `max-metric router-lsa on-shutdown` defers `no router ospf`) | (a): a commit has no ceiling, no second-signal abort and runs inside the transaction's apply budget; drain on delete is its own design | open |
| Q-6 | Leaf names for the new waits | `bgp graceful-shutdown on-shutdown <seconds>`; `isis overload-on-shutdown <seconds>`; OSPF keeps `max-metric router-lsa on-shutdown` | as listed, mirroring the OSPF leaf and FRR's `max-metric router-lsa on-shutdown` | open |
| Q-7 | RSVP-TE (RFC 5817 TE graceful shutdown) and LDP | (a) no drain in this spec; LDP traffic moves with the IGP drain; RSVP-TE recorded as a gap; (b) add RFC 5817 | (a): RFC 5817 is not in `rfc/full/` and the RFC scope is frozen; LDP-IGP sync (RFC 5443) is what makes the IGP drain cover LDP, and whether Ze implements it is A-7 | open |
| Q-8 | Routes this router originates into OSPF/IS-IS as externals (static, connected, BGP redistributed) | (a) out: RFC 6987 MaxLinkMetric repels transit only; (b) also raise external metrics during drain (FRR `external-lsa` option) | (a) for this spec, recorded in Known Limitations | open |
| Q-9 | gokrazy appliance | (a) drain runs there the same way, appliance supervisor wait verified first (A-5); (b) appliance skips the drain | (a) if A-5 shows the supervisor waits at least ceiling + margin; otherwise the answer decides | open |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/plugin-system.md` - registration, engine boundary, `OnStarted` vs `OnAllPluginsReady`
  → Constraint: a new core-to-plugin call is an RPC on the plugin connection (DirectBridge for in-process), never an EventBus event; the drain is a request with a reply
  → Constraint: Stage 1 declarations are pure values; a `DrainBudget` declared at Stage 1 is validated in `startup.go` before it is stored, like `VerifyBudget`/`ApplyBudget` (`internal/component/plugin/registration.go` and `pkg/plugin/rpc/types.go`, the registration structs)
- [ ] `docs/architecture/api/process-protocol.md` - wire protocol, `bye`, shutdown
  → Constraint: `ProcessManager.Stop` sends no `bye` today; closing the connection IS the shutdown signal. The drain RPC must therefore run before `apiServer.Stop()`, while connections are open
- [ ] `docs/architecture/behavior/signals.md` - signal page
  → Constraint: the page describes ExaBGP's Python handler (SIGHUP = shutdown, SIGALRM restart); Ze reloads on SIGHUP. It disagrees with the code today and this spec rewrites it (Doc row 12)
- [ ] `docs/architecture/hub-architecture.md` - hub stop orchestration
  → Decision: the drain step lives in the hub's `run` stop path between `commitReloads.close()` and `gnmiSrv.Stop()`, so management (SSH `show`) stays up while draining and config is already fenced
- [ ] `ai/rules/plugins.md`, `ai/rules/goroutine-lifecycle.md`, `ai/rules/architecture.md`
  → Constraint: the daemon must not name BGP/OSPF/IS-IS: plugins opt in by registering an `OnDrain` handler and a `DrainBudget`; the hub fans out over whoever registered
  → Constraint: the parallel fan-out uses one goroutine per draining plugin per stop, a one-time lifecycle step (permitted shape), each selecting on the drain context
- [ ] `docs/guide/operations.md` - operator stop description (the signal table row "Graceful shutdown (NOTIFICATION Cease to all peers)")
  → Constraint: must describe the drain phase, the second signal, and restart skipping it

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8326.md` (full text `rfc/full/rfc8326.txt`) - BGP graceful shutdown
  → Constraint: RFC 8326 Section 4.2: "In the case of a shutdown of the whole router, in addition to the graceful shutdown of all EBGP sessions, there is a need to gracefully shut down the routes originated by this router (e.g., BGP aggregates redistributed from other protocols, including static routes). This can be performed by tagging these routes with the GRACEFUL_SHUTDOWN community and setting LOCAL_PREF to a low value."
  → Constraint: RFC 8326 Section 4: "The RECOMMENDED value is 0." (LOCAL_PREF); Section 4.2 gives no timer, only "waits for route re-advertisement over the EBGP session and for BGP routing convergence on both ASBRs", then "shuts down the EBGP session, optionally using [RFC8203]"
  → Constraint: summary Meta: `Enrolment: blocked`, `Support: -` (Q-4)
- [ ] `rfc/short/rfc6987.md` (full text read) - OSPF stub router
  → Constraint: RFC 6987 Section 2: "router X announces its router-LSA to the neighbors with the cost of all non-stub links (links of the types other than 3) being set to MaxLinkMetric"; Section 3 defines MaxLinkMetric 0xffff. No shutdown timer in the RFC
- [ ] `rfc/short/rfc3787.md` - IS-IS overload bit
  → Constraint: the overload bit in LSP number zero keeps transit off the node (already quoted in the `overload` leaf of `ze-isis-conf.yang`)
- [ ] `rfc/short/rfc4724.md` - graceful restart
  → Constraint: a NOTIFICATION forecloses retention (Section 5); restart/reboot stop in silence (`Reactor.StopForRestart`) and MUST NOT run the drain, whose BGP half ends in a Cease
- [ ] `rfc/short/rfc5443.md` - LDP-IGP sync
  → Constraint: LDP drains through the IGP's max cost, only where LDP-IGP sync exists (A-7)
- [ ] `rfc/short/rfc8203.md` / `rfc/short/rfc9003.md` - shutdown communication
  → Decision: the post-drain Cease keeps today's subcode 2 Administrative Shutdown

**Key insights:**
- There is no point today where all plugins are connected and the daemon waits for protocol work: every stop budget is 3 s or less, BGP's Cease budget is 1 s.
- `request shutdown` and SSH `stop` stop the BGP reactor (Cease) BEFORE the hub's ordered teardown starts, so a drain in the hub alone would run after BGP already left. Both paths must route through the hub.
- Restart/reboot already stopped the reactor silently before the hub runs; the hub needs to know the stop kind to skip the drain.
- systemd's default `KillMode=control-group` sends SIGTERM to every process in the cgroup, so external plugin subprocesses would die at the start of the drain: the unit needs `KillMode=mixed`.
- The route-source registry is `redistevents.RegisterProtocol`; MPLS label sources (LDP, RSVP-TE) and VRRP addresses are outside it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main.go` `run`, the code after `waitLoop` returns - arms `forceExitOnSignal` on its own channel (`forceCh`, SIGINT/SIGTERM), `close(reloadCh)`, `awaitReloadWorker(reloadShutdownGrace)`, `stopConfirmWindow`, `commitReloads.close()`, prints "Shutting down (Ctrl+C again to force)...", `gnmiSrv.Stop()`, `apiShutdowns` (3 s ctx), `apiServer.Stop()`, `eng.Stop(3 s)`, reboot if `rebootRequested`. Verified.
- [ ] `cmd/ze/hub/main.go` `forceExitOnSignal` - the next SIGINT/SIGTERM prints "forced exit" and `os.Exit(1)`. Verified.
- [ ] `cmd/ze/hub/main.go` `apiServer.SetShutdownFunc` closure - injects SIGTERM into `stopCh`.
- [ ] `cmd/ze/hub/main_reload.go` `reloadGate` - `close()` refuses later commit reloads with `errDaemonStopping` ("daemon is shutting down; config reload refused") and waits admitted ones. This is the config fence the drain sits behind.
- [ ] `internal/component/plugin/server/system.go` `handleDaemonShutdown` - with a real BGP reactor: `r.Stop()` (Cease) then `SignalShutdownRequested`; reactorless: `shutdownFunc` (SIGTERM to stopCh). `handleDaemonReboot`: `rebootFunc` then signal.
- [ ] `cmd/ze/hub/service_ssh.go` `SetShutdownFunc`/`SetRestartFunc`/`SetRebootFunc` closures - SSH exec `stop`: `r.Stop()` then signal; `restart`/`reboot`: `stopForRestart()` then signal.
- [ ] `cmd/ze/hub/infra_setup.go` `stopForRestart` closure - writes the GR marker, `r.StopForRestart()`; the reboot func sets `rebootRequested` then `stopForRestart`. SSH restart sets no flag the hub can read.
- [ ] `internal/component/plugin/server/server.go` `SignalShutdownRequested`, `Stop` - closes `shutdownRequested` once; `Stop` sets running=false, `stopTransaction(txShutdownGrace)`, cancels, `cleanup()` -> `ProcessManager.Stop()`.
- [ ] `internal/component/plugin/process/manager.go` `pluginStopGrace`, `ProcessManager.Stop` - grace 3 s; `Stop` cancels, stops the TLS acceptor, `proc.Stop()` closes every connection, waits each engine up to 3 s and warns on late ones. No `bye` sent.
- [ ] `internal/component/bgp/plugin/register.go` BGP engine run - when `p.Run` returns (connection closed) `bgpReactor.Stop()` then `Wait`.
- [ ] `internal/component/bgp/reactor/reactor.go` `shutdownNotifyBudget`, `Stop`, `StopForRestart` - 1 s budget; `Stop()` = seal, Cease (RFC 4271 8.2.2 / RFC 4486 subcode 2), cancel; `StopForRestart()` = same in silence (RFC 4724 Section 5).
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` `sendStaleReadvertise` - LLGR already re-advertises with a depreference (attribute mods) through Readvertise-opted egress filters: the shape the BGP drain re-advertisement extends.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` `outboundResend` - replays Adj-RIB-Out per peer selector and family.
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go` `exportFilterForBody` - gates originated routes; forwarded routes are filtered in `forwardUpdateCore`. Two rails, both must carry the drain tag (R-2).
- [ ] `internal/component/bgp/wireu/wellknown.go` - GRACEFUL_SHUTDOWN (0xFFFF0000) named, no sender anywhere.
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` container `max-metric router-lsa` - `always|on-startup|on-shutdown`; `on-shutdown` range 0..86400, stored and reported, originates nothing.
- [ ] `internal/plugins/isis/yang/ze-isis-conf.yang` leaf `overload` - boolean only, no timed form.
- [ ] `internal/plugins/fib/kernel/yang/ze-fib-conf.yang` leaf `flush-on-stop` - default false: Ze's kernel routes stay after stop. Unchanged by this spec.
- [ ] `internal/plugins/systemd/unit.go` `buildUnitFile` - `Type=simple`, `Restart=on-failure`, no `TimeoutStopSec`, no `KillMode`: systemd defaults 90 s and control-group kill.
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` `OnBye`, `OnDoctorCheck` - callback pattern: one `callbacks[callbackX]` entry decoding an `rpc.XInput`.
- [ ] `internal/core/redistevents/registry.go` `RegisterProtocol`, `RegisterProducer`, `RegisterOSInstalled` - the route-source registry.
- [ ] `internal/core/ipc/yang/ze-system-cmd.yang` container `request` - `shutdown` ze:help says "drain connections" (it drains nothing today); `quiesce` is a test barrier.

**Route sources, from the registry** (`grep redistevents.RegisterProtocol`, test fake `fakeredist` excluded):

| Protocol | Registered in | Drain in this spec |
|----------|---------------|--------------------|
| bgp | `internal/component/bgp/plugins/rib/rib.go` `bgpProtocolID`; `internal/component/bgp/redistribute/producer.go` `ProtocolID` | RFC 8326: re-advertise every route with GRACEFUL_SHUTDOWN, LOCAL_PREF 0 to iBGP, wait, then Cease |
| bmp | `internal/component/bgp/plugins/rib/rib.go` `bmpProtocolID` | none: monitored routes Ze receives, never advertised to a forwarding peer (A-8) |
| ospf | `internal/plugins/ospf/spf/install.go` `ospfProtocolID` | RFC 6987 MaxLinkMetric Router-LSA for `on-shutdown` seconds |
| isis | `internal/plugins/isis/spf/install.go` `isisProtocolID` | RFC 3787 overload bit in LSP 0 for `overload-on-shutdown` seconds |
| static | `internal/plugins/static/events/events.go` `ProtocolID` | none of its own; drained by BGP when redistributed into BGP (RFC 8326 Section 4.2 quote); into an IGP see Q-8 |
| connected | `internal/plugins/connected/events/events.go` `ProtocolID` (OS-installed) | none: kernel interface routes; carried by the BGP drain when redistributed |
| kernel | `internal/plugins/kernel/events/events.go` `ProtocolID` | none; carried by the BGP drain when redistributed |
| as112 | `internal/plugins/as112/events/events.go` `ProtocolID` | none of its own; its anycast prefixes are originated through BGP and tagged by the BGP drain |
| ipsec | `internal/component/ike/engine/redistribute.go` `ipsecProtocolID` | none: tunnel routes end with the SA; carried by the BGP drain when redistributed |
| l2tp | `internal/component/l2tp/events/events.go` `ProtocolID` | none: subscriber routes; carried by the BGP drain when redistributed |

Outside the registry but installing forwarding state: `internal/plugins/ldp/fib.go` (MPLS labels; drained by the IGP per RFC 5443 only with LDP-IGP sync, A-7, Q-7), `internal/plugins/rsvpte` (RFC 5817 gap, Q-7), `internal/plugins/vrrp` (addresses, not routes; RFC 5798 priority-0 on shutdown is VRRP's own stop behaviour, A-9).

**Behavior to preserve:**
- Restart/reboot (`request reboot`, SSH `restart`/`reboot`): GR marker written, sessions dropped in silence, no Cease, now also no drain.
- After the drain, BGP still sends Cease / Administrative Shutdown within `shutdownNotifyBudget`.
- `flush-on-stop` false keeps kernel routes after stop.
- A commit during shutdown is refused with `errDaemonStopping`.
- `forceExitOnSignal` runs on its own channel that a `request shutdown`-injected SIGTERM or a SIGHUP does not trip.
- With no plugin configured to drain (all waits 0), stop time does not grow.

**Behavior to change:**
- New drain phase between `commitReloads.close()` and `gnmiSrv.Stop()`.
- `request shutdown`, SSH `stop`, and the CLI shutdown function stop BGP through the hub (drain, then teardown), never by calling `r.Stop()` first.
- A second signal during the drain aborts the drain and continues the ordered teardown; a signal after the drain (or a third) forces exit as today.
- Unit gains `TimeoutStopSec` and `KillMode=mixed`.
- OSPF `on-shutdown`, new BGP and IS-IS waits originate their drain.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- SIGTERM/SIGINT to the daemon; `request shutdown` (dispatcher); SSH exec `stop`; CLI shutdown. Restart/reboot entries set the stop kind to restart and bypass the drain.
- Config: per-protocol wait leaves, verified at commit against the plugin's declared `DrainBudget`; Stage 1 `DrainBudget` verified against the daemon ceiling.

### Transformation Path
1. Stage 1: a plugin with an `OnDrain` handler declares `DrainBudget` (seconds) in its registration; `startup.go` refuses a budget above the ceiling or a budget without a handler.
2. Commit verify: each protocol's verify handler refuses its wait leaf above its declared budget.
3. Stop: `waitLoop` returns; hub fences config (reload worker, confirm window, `commitReloads.close()`), reads the stop kind; for `stop` it calls the plugin server's drain fan-out with a context bounded by min(ceiling, largest declared budget) and cancelled by a second signal.
4. Fan-out: one `drain` RPC per drain-registered plugin, all at once, each carrying the reason and the deadline; the server collects replies and logs late or failed plugins by name.
5. Plugin: BGP sets reactor-wide drain state, re-advertises Adj-RIB-Out through both egress rails with GRACEFUL_SHUTDOWN appended (and LOCAL_PREF 0 for iBGP), waits its configured time or the deadline, replies. OSPF re-originates its Router-LSA with MaxLinkMetric, waits, replies. IS-IS sets overload in LSP 0, re-floods, waits, replies.
6. Hub continues the existing teardown: gNMI, API shutdowns, `apiServer.Stop()` (BGP Cease), `eng.Stop`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub -> plugin server | new server method fanning the drain out | No (design) |
| Engine <-> plugin | new `drain` RPC on the plugin connection (DirectBridge in-process, socket external), `rpc.DrainInput`/`DrainOutput` value types | No (design) |
| BGP plugin -> reactor | reactor drain state read by the egress rails | No (design) |
| Daemon -> systemd | unit `TimeoutStopSec`, `KillMode=mixed` | No (design) |

### Integration Points
- `sdk.Registration` gains `DrainBudget`; `pkg/plugin/sdk` gains `OnDrain` beside `OnBye`.
- `internal/component/plugin/server/startup.go` validates the budget like `validateDoctorCheckDecls`.
- hub stop path in `cmd/ze/hub/main.go`; lifecycle handlers in `internal/component/plugin/server/system.go`; `cmd/ze/hub/service_ssh.go`; stop kind in `cmd/ze/hub/infra_setup.go`.
- BGP: `internal/component/bgp/plugin/register.go` (handler), reactor egress rails, `outboundResend`.
- OSPF/IS-IS: their `register.go` handlers and LSA/LSP origination.
- systemd plugin unit generation and its doctor check.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | drain goes hub -> plugin server -> RPC -> plugin; lifecycle handlers stop calling the reactor directly |
| No unintended coupling (components stay isolated) | Yes (design) | hub and server know only "plugins with a drain handler" |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | callback pattern of `OnBye`/`OnDoctorCheck`; budget pattern of `ApplyBudget`; BGP re-advertise reuses the LLGR readvertise rail and `outboundResend` |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | re-advertisement uses the existing attribute-mods path; no new per-route allocation path beyond what LLGR readvertise already does |
| Registration over hardcoding, outbound | Yes (design) | plugins register `OnDrain` + `DrainBudget`; no protocol name in hub or server |
| Registration over hardcoding, inbound | Yes (design) | searched: hub stop path, `system.go` lifecycle handlers, `startup.go` validators, `process/manager.go`; none learns a protocol name; the ceiling is declared once in `pkg/plugin/rpc` and the unit generator derives `TimeoutStopSec` from it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Routing `request shutdown` and SSH `stop` through the hub still yields a Cease to every peer | the BGP engine run in `bgp/plugin/register.go` calls `bgpReactor.Stop()` when `p.Run` returns on connection close | peers would see a bare FIN on `request shutdown` | functional test asserting NOTIFICATION 6/2 after `request shutdown` (AC-8) | unvalidated |
| A-2 | FRR bgpd lowers preference of a path carrying GRACEFUL_SHUTDOWN without extra config, or the interop config applies an inbound route-map that does | FRR `bgp graceful-shutdown` doc (research §1.2) covers the sender; receiver handling not read in FRR source | interop must configure the receiver side explicitly | read FRR `bgp_route.c` gshut handling before writing the scenario | unvalidated |
| A-3 | The plugin connections and the reactor are fully live between `commitReloads.close()` and `apiServer.Stop()` | research §1.5; code read of the hub stop path | drain RPC cannot be delivered | wiring test AC-1 | unvalidated |
| A-4 | `KillMode=mixed` delivers SIGTERM to the main PID only and SIGKILL to the cgroup at timeout | man systemd.kill | external plugins die at drain start | unit test reading the generated unit + functional test AC-16 | unvalidated |
| A-5 | gokrazy's supervisor sends SIGTERM and waits long enough for ceiling + margin | research §1.3: "gokrazy appliance stop behaviour: not established" | appliance SIGKILLs mid-drain | read the gokrazy supervisor source in the vendored module | unvalidated |
| A-6 | An OSPF or IS-IS neighbour keeps the adjacency during the wait, so the re-originated LSA floods | adjacencies drop only when the plugin closes | drain advertisement never leaves | interop AC-19/AC-20 | unvalidated |
| A-7 | Ze has no LDP-IGP sync (RFC 5443) | `rfc/short/rfc5443.md` exists; no implementation grep done | LDP drain answer in Q-7 changes | grep `internal/plugins/ldp` and the IGPs for sync | unvalidated |
| A-8 | BMP-ingested routes are never advertised to a forwarding BGP peer | `bmpProtocolID` registration only | the BGP drain must also cover them | read the bmp path in `rib.go` | unvalidated |
| A-9 | VRRP already sends priority 0 on stop (RFC 5798 Section 6.4.3) | not read | VRRP has its own stop-time gap; journal row | read the stop path in `internal/plugins/vrrp/engine.go` | unvalidated |
| A-10 | Drain state need not survive a plugin crash/respawn during the drain | stop is terminal | a respawned BGP re-advertises without the tag | R-7, AC-17 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Lifecycle handlers changed to stop through the hub break an existing `.ci` that expects an immediate Cease on `request shutdown` | functional red in `test/plugin` or `test/reload` | the Cease still comes, after a 0-length drain by default (Q-3); update only assertions on timing, never weaken |
| R-2 | BGP drain tags originated routes but not forwarded (reflected/route-server) routes, or the reverse | interop: FRR sees some prefixes without 65535:0 | AC-9 asserts both an originated and a forwarded prefix |
| R-3 | A second signal during drain is read by `forceExitOnSignal` and exits instead of aborting | AC-6 test exit code 1 with no Cease | one owner of the signal channel during the drain; force-exit armed only after the drain or on the third signal |
| R-4 | A plugin that never replies holds every stop to the ceiling | stop takes the full ceiling | deadline is min(ceiling, largest declared budget); late plugins named in a warning |
| R-5 | Drain + teardown exceeds `TimeoutStopSec` and systemd SIGKILLs | journal "State 'stop-sigterm' timed out" | teardown after the drain is API 3 s + tx 3 s + plugin 3 s + 0.5 s + eng 3 s plus BGP 1 s notify, about 14 s, with the 3 s reload grace before it; margin 30 s; unit test derives `TimeoutStopSec` from the constants |
| R-6 | Re-advertising a full table to every peer during the drain costs CPU and memory while budgets are short | perf of the drain on a large RIB | reuse cursor-mode `outboundResend`; measure on the Mac (perf runs there) |
| R-7 | A plugin crash and respawn during the drain re-advertises untagged routes | drain log shows a respawn | ProcessManager refuses respawn once draining begins (stop is terminal) |
| R-8 | `systemctl restart ze` now drains (it is stop + start) and lengthens restarts | operator report | documented; `restart`/`reboot` through Ze is the GR path without drain |
| R-9 | `KillMode=mixed` changes cleanup of external plugins | leftover processes after timeout | at timeout systemd SIGKILLs the whole cgroup in mixed mode too |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | stops hang to the ceiling, or peers see a Cease without drain (today's behaviour), or restart drains and the GR marker is contradicted by a Cease |
| How is it reverted? | single commit revert; installed units keep `TimeoutStopSec`/`KillMode`, which are harmless without the drain |
| Who else touches this path? | `plan/immediate/spec-ospf-max-metric-startup-shutdown-windows.md` (dependent); any spec editing the hub stop path or the `system.go` lifecycle handlers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGTERM to daemon with a drain-registered test plugin | → | hub drain step -> server fan-out -> `OnDrain` | `test/plugin/drain-before-stop.ci` |
| `request shutdown` | → | `handleDaemonShutdown` -> hub drain | `test/plugin/drain-request-shutdown.ci` |
| SSH `restart` | → | `stopForRestart` sets stop kind restart -> hub skips drain | `test/plugin/drain-skipped-on-restart.ci` |
| second SIGTERM during drain | → | drain context cancelled, teardown continues | `test/plugin/drain-second-signal-aborts.ci` |
| Stage 1 `DrainBudget` over ceiling | → | `startup.go` validation | `test/plugin/drain-budget-over-ceiling-refused.ci` |
| SIGTERM with BGP `graceful-shutdown on-shutdown` set | → | BGP drain handler -> reactor egress rails | `test/plugin/bgp-graceful-shutdown-drain.ci` |
| SIGTERM with OSPF `max-metric router-lsa on-shutdown` set | → | OSPF drain handler -> Router-LSA origination | `test/ospf/ospf-max-metric-on-shutdown-drain.ci` |
| SIGTERM with IS-IS `overload-on-shutdown` set | → | IS-IS drain handler -> LSP 0 origination | `test/isis/isis-overload-on-shutdown-drain.ci` |
| `ze systemd install` | → | `buildUnitFile` | `TestBuildUnitFileStopTimeoutDerivedFromCeiling` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SIGTERM; one plugin registered `DrainBudget` > 0 | the plugin receives `drain` while its connection is open, before any plugin connection closes; the daemon closes connections only after the plugin replies; the log carries "drain started" and "drain finished" with the plugin name and elapsed time |
| AC-2 | two drain plugins each waiting 2 s | the drain phase ends in under 3 s (parallel), not 4 s |
| AC-3 | a drain plugin that never replies, budget B | teardown starts at min(B, ceiling) after drain start; a warning names the plugin; exit code 0 |
| AC-4 | Stage 1 registration with `DrainBudget` above the ceiling, or a budget with no `OnDrain` handler | plugin start refused; the error names the plugin, the budget and the ceiling |
| AC-5 | commit with OSPF `on-shutdown`, BGP `graceful-shutdown on-shutdown` or IS-IS `overload-on-shutdown` above that plugin's declared budget | commit refused at verify; the error names the leaf path, the value and the limit |
| AC-6 | second SIGTERM or SIGINT during the drain | drain aborted at once; teardown continues (BGP Cease still sent); log "drain aborted by signal"; a further signal forces exit 1 as today |
| AC-7 | `request reboot`, SSH `restart`, SSH `reboot` | no `drain` RPC is sent; no GRACEFUL_SHUTDOWN re-advertisement; no Cease (unchanged GR path) |
| AC-8 | `request shutdown` (BGP daemon and reactorless daemon), SSH `stop` | the drain runs before any Cease; the peer receives NOTIFICATION 6/2 after the drain; the accepted answer is still written before teardown |
| AC-9 | BGP with `graceful-shutdown on-shutdown N`, one eBGP and one iBGP peer, one originated and one forwarded prefix | on SIGTERM each peer receives every previously advertised prefix again carrying community 65535:0; the iBGP peer's copy carries LOCAL_PREF 0; after N seconds (or the deadline) each peer receives Cease 6/2 |
| AC-10 | BGP with the wait at 0 or unset | no re-advertisement; Cease within today's budget |
| AC-11 | OSPF `max-metric router-lsa on-shutdown N` | on SIGTERM the Router-LSA is re-originated with every non-stub link cost 0xffff and stub links unchanged; adjacencies stay up N seconds; then teardown. Replaces AC-11 of the OSPF max-metric spec |
| AC-12 | IS-IS `overload-on-shutdown N` | on SIGTERM LSP 0 is re-originated with the overload bit set; the adjacency stays up N seconds |
| AC-13 | no plugin has a non-zero wait | stop duration within 100 ms of today's on the same test |
| AC-14 | commit attempted during the drain | refused with "daemon is shutting down; config reload refused" (existing fence) |
| AC-15 | generated systemd unit | carries `TimeoutStopSec` = ceiling + margin and `KillMode=mixed`; `ze doctor` warns, with a diagnostic code, on an installed unit whose stop timeout is below that or whose kill mode is not mixed |
| AC-16 | an external (subprocess) drain plugin under the generated unit | receives `drain` and replies; it is not killed by the stop's SIGTERM |
| AC-17 | a plugin respawn requested during the drain | refused; the plugin stays down until teardown |
| AC-18 | interop BGP against FRR | FRR shows the Ze path with community graceful-shutdown and moves best path to the alternate before the Cease arrives; red recorded with the drain reverted |
| AC-19 | interop OSPF against FRR ospfd | FRR's LSDB shows Ze's Router-LSA with metric 65535 on its non-stub links before the adjacency drops; red recorded with the drain reverted |
| AC-20 | interop IS-IS against FRR isisd | FRR shows Ze's LSP with the overload bit before the adjacency drops; red recorded with the drain reverted |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | `systemctl stop ze` on a router with BGP and OSPF drain configured | SIGTERM -> hub fence -> drain fan-out -> BGP/OSPF advertise -> wait -> teardown | `test/plugin/drain-before-stop.ci`, interop AC-18/AC-19 |
| 2 | `request shutdown` from the CLI | dispatcher -> shutdownFunc -> hub drain -> Cease | `test/plugin/drain-request-shutdown.ci` |
| 3 | operator presses Ctrl+C again during the drain | second signal -> drain abort -> teardown | `test/plugin/drain-second-signal-aborts.ci` |
| 4 | `restart` over SSH for a software upgrade | stopForRestart -> no drain, silent drop | `test/plugin/drain-skipped-on-restart.ci` |
| 5 | operator sets a wait above the limit | commit verify refusal | `test/ospf/ospf-max-metric-on-shutdown-over-budget.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDrainBudgetAboveCeilingRefused` | `internal/component/plugin/server/startup_test.go` | AC-4 | |
| `TestDrainBudgetWithoutHandlerRefused` | `internal/component/plugin/server/startup_test.go` | AC-4 | |
| `TestDrainFanOutParallel` | `internal/component/plugin/server/drain_test.go` | AC-2 | |
| `TestDrainFanOutDeadlineNamesLatePlugin` | `internal/component/plugin/server/drain_test.go` | AC-3 | |
| `TestDrainFanOutCancelAborts` | `internal/component/plugin/server/drain_test.go` | AC-6 | |
| `TestDrainSkipsPluginsWithoutBudget` | `internal/component/plugin/server/drain_test.go` | AC-13 | |
| `TestOnDrainDecodesReasonAndDeadline` | `pkg/plugin/sdk/sdk_callbacks_test.go` | SDK callback | |
| `TestHubStopKindRestartSkipsDrain` | `cmd/ze/hub/drain_test.go` | AC-7 | |
| `TestHubSecondSignalAbortsDrainThenForces` | `cmd/ze/hub/drain_test.go` | AC-6 | |
| `TestHandleDaemonShutdownRoutesThroughHub` | `internal/component/plugin/server/system_test.go` | AC-8 | |
| `TestRespawnRefusedWhileDraining` | `internal/component/plugin/process/manager_test.go` | AC-17 | |
| `TestBGPDrainTagsOriginatedAndForwarded` | `internal/component/bgp/reactor/drain_test.go` | AC-9, R-2 | |
| `TestBGPDrainIBGPLocalPrefZero` | `internal/component/bgp/reactor/drain_test.go` | AC-9 | |
| `TestBGPDrainZeroWaitNoReadvertise` | `internal/component/bgp/reactor/drain_test.go` | AC-10 | |
| `TestBGPGracefulShutdownWaitOverBudgetRefused` | BGP config package test | AC-5 | |
| `TestOSPFDrainRouterLSAMaxLinkMetric` | `internal/plugins/ospf/drain_test.go` | AC-11 (stub links keep cost) | |
| `TestOSPFOnShutdownOverBudgetRefused` | `internal/plugins/ospf/config_test.go` | AC-5 | |
| `TestISISDrainSetsOverloadLSP0` | `internal/plugins/isis/drain_test.go` | AC-12 | |
| `TestISISOverloadOnShutdownOverBudgetRefused` | `internal/plugins/isis/config_test.go` | AC-5 | |
| `TestBuildUnitFileStopTimeoutDerivedFromCeiling` | `internal/plugins/systemd/unit_test.go` | AC-15 | |
| `TestDoctorUnitStopTimeoutBelowCeiling` | `internal/plugins/systemd/doctor_test.go` | AC-15 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| registration `DrainBudget` (s) | 0..ceiling | ceiling | negative, refused as malformed | ceiling + 1 |
| OSPF `on-shutdown` (s) | 0..OSPF budget | budget | N/A (uint32) | budget + 1 |
| BGP `graceful-shutdown on-shutdown` (s) | 0..BGP budget | budget | N/A (uint32) | budget + 1 |
| IS-IS `overload-on-shutdown` (s) | 0..IS-IS budget | budget | N/A (uint32) | budget + 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `drain-before-stop` | `test/plugin/drain-before-stop.ci` | test plugin logs drain before its connection closes; daemon exits 0 | |
| `drain-request-shutdown` | `test/plugin/drain-request-shutdown.ci` | `request shutdown` drains, then ze-peer sees Cease 6/2 | |
| `drain-skipped-on-restart` | `test/plugin/drain-skipped-on-restart.ci` | restart: no drain line, no Cease | |
| `drain-second-signal-aborts` | `test/plugin/drain-second-signal-aborts.ci` | second SIGTERM aborts; Cease still sent; exit 0 | |
| `drain-budget-over-ceiling-refused` | `test/plugin/drain-budget-over-ceiling-refused.ci` | refusal text names plugin, budget, ceiling | |
| `drain-external-plugin` | `test/plugin/drain-external-plugin.ci` | subprocess plugin drains (AC-16) | |
| `bgp-graceful-shutdown-drain` | `test/plugin/bgp-graceful-shutdown-drain.ci` | ze-peer receives an UPDATE with 65535:0 (hex asserted) then NOTIFICATION 6/2 | |
| `ospf-max-metric-on-shutdown-drain` | `test/ospf/ospf-max-metric-on-shutdown-drain.ci` | Router-LSA with 0xffff before the adjacency drops | |
| `ospf-max-metric-on-shutdown-over-budget` | `test/ospf/ospf-max-metric-on-shutdown-over-budget.ci` | commit refused naming the leaf | |
| `isis-overload-on-shutdown-drain` | `test/isis/isis-overload-on-shutdown-drain.ci` | LSP 0 overload set before the adjacency drops | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-graceful-shutdown-drain-frr` | `test/interop/scenarios/` | FRR bgpd, plus a second speaker giving FRR an alternate path | FRR receives 65535:0 (and LOCAL_PREF 0 on iBGP) and selects the alternate before the Cease; red recorded with the BGP drain reverted and the image rebuilt | |
| `ospf-max-metric-on-shutdown-frr` | `test/interop/scenarios/` | FRR ospfd | FRR's LSDB shows Ze's Router-LSA metric 65535 while the adjacency is still Full; red recorded with the OSPF drain reverted | |
| `isis-overload-on-shutdown-frr` | `test/interop/scenarios/` | FRR isisd | FRR shows the overload bit on Ze's LSP before the adjacency drops; red recorded with the IS-IS drain reverted | |

Interop check code: new `check_*` files under `internal/le/interoplab/bgp/` beside `check_ospf_interas.go` and `check_isis.go`. Each red is recorded with `./le rfc discriminate-record` where the carrier holds an `RFC requirement:` tag (RFC 8326 after Q-4, RFC 6987, RFC 3787).

### Goal Validation
| Goal (from Task) | Evidence |
|------------------|----------|
| Every route-adding protocol has a drain before stop | registry table above; BGP/OSPF/IS-IS handlers with interop AC-18..20; every other registered source documented as carried by BGP or having no drain mechanism |
| Traffic moves before the routes vanish | AC-18 interop: FRR best path moves to the alternate before the Cease, with timestamps |
| Stop is bounded below the supervisor's kill | AC-3 + AC-15 unit test deriving `TimeoutStopSec` from the constants |
| An impatient operator can still stop now | AC-6 functional test |
| GR restart is untouched | AC-7 functional test |

## Files to Modify
- `pkg/plugin/rpc/types.go` - `DrainBudget` in registration; `DrainInput`/`DrainOutput`; the drain ceiling constant (single declaration)
- `pkg/plugin/sdk/sdk_callbacks.go` and the SDK registration type - `OnDrain`, `callbackDrain`, `Registration.DrainBudget`
- `internal/component/plugin/registration.go` - `DrainBudget`
- `internal/component/plugin/server/startup.go` - Stage 1 budget validation
- `internal/component/plugin/server/server.go` - drain fan-out entry
- `internal/component/plugin/server/system.go` - `handleDaemonShutdown`/`handleDaemonQuit` route through the daemon shutdown function; stop kind
- `internal/component/plugin/process/manager.go` - draining state refuses respawn
- `internal/component/plugin/ipc/rpc.go` - `SendDrain` beside `SendBye`
- `cmd/ze/hub/main.go` - drain step, signal ownership during the drain
- `cmd/ze/hub/service_ssh.go`, `cmd/ze/hub/infra_setup.go` - stop kind; SSH `stop` stops through the hub
- `internal/component/bgp/plugin/register.go` - `OnDrain`, `DrainBudget`
- `internal/component/bgp/reactor/` (`egress_inject_filter.go`, the forward rail, readvertise in `reactor_api_batch.go`) - drain tag on both rails
- the BGP config YANG module - `graceful-shutdown on-shutdown`
- `internal/plugins/ospf/register.go`, Router-LSA origination, `yang/ze-ospf-conf.yang` (`on-shutdown` description: it now drains)
- `internal/plugins/isis/register.go`, LSP origination, `yang/ze-isis-conf.yang` - `overload-on-shutdown`
- `internal/plugins/systemd/unit.go`, `internal/plugins/systemd/doctor.go` - `TimeoutStopSec`, `KillMode=mixed`, doctor check
- `internal/core/diagnostic/codes.go` - unit stop-timeout code
- `internal/core/ipc/yang/ze-system-cmd.yang` - `request shutdown` help and description state the drain
- `rfc/short/rfc8326.md` - Meta Support row (after Q-4)
- `plan/immediate/spec-ospf-max-metric-startup-shutdown-windows.md` - Q-1 answered by this spec (append-only note)

## Files to Create
- `internal/component/plugin/server/drain.go`, `drain_test.go`
- `cmd/ze/hub/drain.go`, `drain_test.go`
- `internal/component/bgp/reactor/drain.go`, `drain_test.go`
- `internal/plugins/ospf/drain.go`, `drain_test.go`; `internal/plugins/isis/drain.go`, `drain_test.go`
- the functional `.ci` files and interop scenarios named above
- a drain-capable test plugin under `internal/test/plugins/` (in-process and external modes)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | BGP `graceful-shutdown on-shutdown`, IS-IS `overload-on-shutdown`; OSPF `on-shutdown` description |
| YANG validation constraints | Yes | `uint32`, `units seconds`; the budget limit is a verify refusal, not a YANG range copy of the constant |
| YANG custom validators | No | the plugin's verify handler refuses above its declared budget |
| CLI commands/flags | No | no new verb unless Q-2 (a) |
| CLI grammar (keyword before value) | Yes | new leaves follow keyword-value (`ai/rules/cli.md`) |
| Editor autocomplete | Yes | automatic for the typed leaves |
| Functional test for new RPC/API | Yes | `test/plugin/drain-*.ci` |
| Pipe completeness | N-A | no new show output |
| Env var registration | No | no env var |
| Doctor check for runtime dependencies | Yes | systemd unit `TimeoutStopSec`/`KillMode`: `internal/plugins/systemd/doctor.go` + `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | No | the drain runs once per process lifetime; log lines carry it |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family, capability or attribute; uses the well-known community |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | new `features/daemon-drain-before-stop.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (new leaves) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`request shutdown` behaviour) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`daemon-shutdown` semantics) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (BGP, OSPF, IS-IS drain) |
| 6 | Has a user guide page? | Yes | `docs/guide/operations.md` stop row, new drain section |
| 7 | Wire format changed? | No | no new encoding |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` (`drain` RPC, `drain-budget`), `docs/plugin-development/protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8326.md`, `rfc/short/rfc6987.md`, `rfc/short/rfc3787.md`, generated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (three scenarios) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (no open-source peer drains on SIGTERM, research §1.2) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/hub-architecture.md`; rewrite `docs/architecture/behavior/signals.md`, which describes ExaBGP's Python handler and disagrees with Ze today |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md` (drain registration) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by the files this spec changes: `docs/architecture/api/ipc_protocol.md` (affected: new `drain` RPC and `drain-budget` field); `docs/architecture/cli/plugin-modes.md` (affected: unit gains `TimeoutStopSec`, `KillMode=mixed`); `docs/architecture/core-design.md` (affected: the Engine startup/shutdown order gains the drain phase); `docs/architecture/ospf/ospf-4-component-config.md` (affected: `on-shutdown` now drains); `docs/architecture/isis/isis-4-component-config.md` (affected: `overload-on-shutdown`); `docs/features/ai-first.md` (unaffected unless the doctor-check catalogue it describes lists checks by name, then the new unit check is added). Re-derive with `./le spec citation anchors spec plan/immediate/spec-daemon-drain-before-stop.md` at implementation start |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | OSPF `max-metric` examples in the OSPF guide pages; verify against YANG |

## Implementation Steps

1. **Phase: Wiring** - SDK `OnDrain` + `DrainBudget`, Stage 1 validation, server fan-out, hub drain step, test plugin
   - Tests: `test/plugin/drain-before-stop.ci`, `drain-budget-over-ceiling-refused.ci`, `TestDrainBudgetAboveCeilingRefused`
   - Verify: the test plugin receives `drain`; red before the hub step exists
2. **Phase: Stop kinds and signals** - route `request shutdown`/SSH `stop` through the hub, restart kind, second-signal abort, respawn refusal
   - Tests: the AC-6, AC-7, AC-8, AC-17 tests
3. **Phase: systemd** - unit constants and doctor check
   - Tests: `TestBuildUnitFileStopTimeoutDerivedFromCeiling`, `TestDoctorUnitStopTimeoutBelowCeiling`
4. **Phase: BGP drain** - YANG leaf, verify, reactor drain state on both egress rails, Adj-RIB-Out resend, wait, reply
   - Tests: BGP unit tests, `bgp-graceful-shutdown-drain.ci`, interop `bgp-graceful-shutdown-drain-frr` with recorded red
5. **Phase: OSPF drain** - `on-shutdown` originates MaxLinkMetric; the dependent spec's interim refusal is removed
   - Tests: OSPF unit tests, `.ci`, interop `ospf-max-metric-on-shutdown-frr` with recorded red
6. **Phase: IS-IS drain** - `overload-on-shutdown`
   - Tests: IS-IS unit tests, `.ci`, interop `isis-overload-on-shutdown-frr` with recorded red
7. **Docs** - each phase edits its pages as it lands (Documentation checklist), not at the end

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the drain runs only on stop kind `stop`; the restart path emits neither tag nor Cease |
| Correctness | BGP tags both originated and forwarded routes; iBGP LOCAL_PREF 0, eBGP community only (LOCAL_PREF is not sent on eBGP) |
| Naming | `drain-budget` JSON kebab-case; leaves mirror OSPF `on-shutdown` |
| Data flow | hub and server never name a protocol; the ceiling is declared once |
| Rule: goroutine-lifecycle | fan-out goroutines are one-time lifecycle steps selecting on the drain context |
| Rule: rfc-compliance | RFC 8326 Section 4.2, RFC 6987 Section 2, RFC 3787 quotes above the implementing functions |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| drain RPC and SDK hook | `grep -n OnDrain pkg/plugin/sdk` |
| hub drain step | `test/plugin/drain-before-stop.ci` |
| unit change | `TestBuildUnitFileStopTimeoutDerivedFromCeiling` |
| three interop scenarios with recorded reds | `rfc/discrimination/` records or scenario logs |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | a plugin cannot extend the stop past the ceiling; respawn refused while draining |
| Authorization | `request shutdown` authorization unchanged |
| Input validation | Stage 1 budget bounded; config waits bounded at verify |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- No open-source routing daemon checked drains on SIGTERM (FRR, BIRD, GoBGP, holo, OpenBGPD); commercial NOSes drain through an operator maintenance command. FRR had stop-time OSPF drain in 2005 and removed it for kill in 2023 (research §1.2). The ceiling and the second-signal abort answer why FRR backed away: a supervisor that stops waiting.
- The stop path has two Cease producers today: the lifecycle handler's direct `r.Stop()` and the plugin exit's `bgpReactor.Stop()`. The drain needs one, the second.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Daemon-wide drain RPC, plugins opt in | (b) each plugin drains on its own `bye`/connection close with a longer `pluginStopGrace` | (b) runs after connections close, so nothing is delivered; it also makes every plugin's grace the daemon's stop time, with no ceiling and no abort |
| Static `DrainBudget` declared at Stage 1, plus per-protocol wait leaves verified against it | (b) the plugin reports its configured wait to the daemon at each apply; (c) the daemon reads every protocol's config | (b) adds a second runtime channel for one number; (c) makes the core name protocol config. A static budget gives the daemon its deadline without config knowledge |
| Ceiling as one constant in `pkg/plugin/rpc`, unit `TimeoutStopSec` derived from it | config leaf | the unit is generated at install time; a configurable ceiling drifts (Q-1) |
| Lifecycle handlers stop BGP through the hub | keep `r.Stop()` and drain inside the handler | two drain entry points; SIGTERM would still need the hub one |
| Drain between `commitReloads.close()` and `gnmiSrv.Stop()` | before the config fence | a commit landing during the drain could change the config being drained |

## Known Limitations
- Drain on protocol deletion by commit: Q-5 (a) keeps it out.
- External metrics of routes redistributed into an IGP are not raised: Q-8.
- RSVP-TE RFC 5817 graceful shutdown: gap, Q-7.
- RFC 8326 receiver side (inbound LOCAL_PREF on received GRACEFUL_SHUTDOWN): not in this spec (Q-4).
- Maintenance drain without stop: Q-2 (c) gives it its own spec.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC 8326 Section 4.2 (whole-router tagging) and Section 4 (LOCAL_PREF 0) above the BGP drain; RFC 6987 Section 2 above the MaxLinkMetric origination; RFC 3787 above the overload set; RFC 4724 Section 5 at the restart skip.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-daemon-drain-before-stop.md` only, in the same `./le commit create` script
