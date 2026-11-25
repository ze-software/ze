# Spec: vpp-failure-lifecycle -- Ze manages every VPP it runs, kills and restarts it after a declared loss, and withdraws only when it cannot recover it

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Owner decision (2026-11-23) on how Ze manages VPP and reacts to its failure. Thomas answered choice 15 the same day: the spec stays post-release, against the review's reading that findings 1 to 4 and both journal rows are defects of the VPP backend the first release ships |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## His words (Thomas, 2026-11-23)

Quoted as he wrote them. Every rule below names the quote it rests on.

| # | Quote |
|---|-------|
| Q-1 | "we should refuse to have VPP unmanaged by Ze part of our rules, if VPP is external, the use should accept that we have the right to terminate and restart vpp. If the VPP software crashed, what happens to the routing ? ... If the routing continues and we reconnect, if we can see the state of configured hardware, we should continue from where we are, if the software could have crashed and stop forwarding we should reflect this in Ze and make sure other peers for IGP and BGP are updated to reflect the situation." |
| Q-2 | "we should know how to manage vpp if it is external, and should know how to restart it (systemd or direct binary start like when we use embedfs - I suppose)." |
| Q-3 | "we should have the same time for alive testing and declare loss. as soon as loss is declared, ensure process is terminal and start withdrwal/restart. We should accept that we may be blackholing traffic as we restart, so we should only withdraw routes if we can not restart and reconfigure VPP or recover VPP." |
| Q-4 | "recovered can be, we were able to reconnet within a time limit and can check all is still configured. or forced restart and correct re-configuration, like if on boot" |
| Q-5 | "systemd configuration: we can not be sure that an OS update will not change the settings so we should detect the settings and adapt" |
| Q-6 | "We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background." |
| Q-7 | "We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage." |

### His answers to the open choices (2026-11-23)

Put to him as choices; Q-6 and Q-7 are his own text. Each answer is written
into the rules, the design draft and the choices table below.

| # | Answer | Choices it settles |
|---|--------|--------------------|
| A-1 | Two values. The failed alive test is the loss declaration; a separate, second limit bounds the reconnect-and-verify attempt (form 1) before Ze kills and restarts | 10 (reading b), review I-1 |
| A-2 | The recovery deadline and the crash-loop limit are both adopted. Past either, Ze withdraws (VPP-backed interfaces down in Ze's model, so IGP and BGP react). The default values are proposed in this spec for his approval at design review | 9, P-1, P-2 |
| A-3 | After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the interfaces come up and the routes return by themselves; `ze doctor` reports throughout | 11 |
| A-4 | Q-6. Form (1) does not block on a full readback: VPP answering with its configured data, from the same pidfd-proven process, is the sign it is alive. Ze adopts it and reconciles in the background (the fib-vpp replay and sweep and the iface reconcile style), and never kills a healthy VPP because a dump is slow. A mismatch background reconciliation finds is repaired, not treated as a loss. This narrows "full readback matches" in form (1) | 2, 12, review I-4 |
| A-5 | Ze's own protocol timers (BFD detection, OSPF and IS-IS dead, BGP hold) keep running during recovery, as their RFCs say. The VPP loss itself triggers no withdrawal | 13, review I-6 |
| A-6 | Q-7. A foreign process on VPP's API socket (its pidfd does not resolve to the configured systemd unit) is reported and not refused: `ze doctor` names the process, the configured unit, that the configuration may be out of date, and the exact fix. Ze keeps working with it, and neither refuses nor kills it | 14 |
| A-7 | The spec stays post-release and is not moved. Removing the rows the 2026-11-23 rows narrow needs his `decision-change` from his terminal (handover 61) | 15, review I-11 |

## Owner rules

These are his rules, recorded as the `## Design decisions` rows of
`docs/research/vpp-deployment-reference.md` (the `// Design:` page of
`internal/component/vpp/vpp.go`), rows 1 to 9, and rows 10 to 15 for his
answers A-1 to A-6. This spec is the record their Source cells name. Text marked "recorder" is the recording session's
derivation, not his words: it binds nothing until he adopts it.

| # | Rule | Quote |
|---|------|-------|
| R-1 | Ze refuses to run VPP unmanaged. If VPP is external, the operator accepts that Ze may terminate and restart it | Q-1 |
| R-2 | Ze must know how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime (`plan/release/delivery/spec-vpp-embedded-runtime.md`) | Q-2 |
| R-3 | After a failure: if routing (forwarding) continued, and Ze reconnects and can see the configured state, Ze continues from where it is. If the software could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated. R-4 to R-7 narrow it: its continue branch holds only inside the reconnect-and-verify limit (form 1 of R-7), and Q-3 attaches the peer update to a failed recovery only | Q-1, Q-3 |
| R-4 | Two limits (A-1, reading (b) of choice 10). The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1) | Q-3, Q-4, A-1 |
| R-5 | Once the reconnect-and-verify limit expires without form (1), Ze makes sure the process is terminal, then restarts and reconfigures VPP. A-1 places the kill after the second limit, not at the declaration Q-3 names. Recorder: the mechanism (kill through the retained pidfd, SIGKILL, wait for the exit proof) is derived, not his words (review N-6) | Q-3, A-1 |
| R-6 | Blackholing during the restart is accepted. The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead, BGP hold) keep running as their RFCs say (A-5). Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only if Ze cannot restart and reconfigure VPP, or recover it, inside the recovery deadline and the crash-loop limit (R-10) | Q-3, A-2, A-5 |
| R-7 | "Recovered" has exactly two forms. (1) Inside the reconnect-and-verify limit, VPP answers over the API with the data Ze configured, from the same pidfd-proven process: Ze adopts it, with no kill and no withdrawal, and reconciles in the background. A mismatch the background reconciliation finds is repaired, not treated as a loss, and a slow dump never kills a healthy VPP (A-4). (2) A forced restart followed by a correct reconfiguration, exactly as at boot. Recorder: the readback list is derived (step 3.1a widens it to every VPP owner) | Q-4, Q-6, A-4 |
| R-10 | A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws (R-6) and `ze doctor` reports it. The default values are proposed at design review for his approval | A-2 |
| R-11 | After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves; `ze doctor` reports the state throughout | A-3 |
| R-12 | A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed: `ze doctor` names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with it | Q-7, A-6 |
| R-9 | With a systemd unit, Ze cannot be sure an OS update will not change the unit's settings, so it detects them and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts, never from an earlier read, and adapts its restart to them (step 2a) | Q-5 |

R-8 ("Ze is the only restarter: a systemd unit it drives does not restart VPP
itself, Restart=no") is removed. It was the main session's proposal P-3,
recorded as a rule by mistake, and Q-5 reverses it (review B-1). Its text
still stands in page row 6's second sentence and forbids clause; row 9
supersedes them, and correcting row 6 needs Thomas's `decision-change`.

Consequences of R-4 to R-7 and R-10 to R-12, recorded with them (recorder):

- The adopt path survives only inside the reconnect-and-verify limit (form
  1), on the same pidfd-proven process answering with its configured data.
  Option C's readback (PPPoE spec) runs in the background reconciliation and
  no longer gates the adoption. After the limit there is no adopt.
- Ze needs the retained process lease and pidfd: it is the identity proof of
  form (1), the kill handle and the source of the exit proof (PPPoE F-1 stays;
  F-2 closes with it; F-3 is fixed for form 1 by PPPoE change 2).
- No graceful-restart claim on re-open: the F bit stays 0.
- PPPoE sessions are torn down per OD-2 on form (2), because their VPP state
  is gone, and subscribers redial.
- Retrying after a withdrawal (R-11) means a systemd unit at its start limit
  is reset (`reset-failed`) before the next start, or the retry cannot run.
  Confirm at design.
- A foreign process (R-12) is never killed. How a loss proceeds when the peer
  on the socket is foreign, where R-5 would make the process terminal, is
  for the design to propose to him.

Conditions the main session proposed to Thomas. P-1 and P-2 were not his
words; he adopted both on 2026-11-23 (A-2, rule R-10), with the default
values to be proposed at design review. P-3 is replaced by his rule R-9.

| # | Proposal | Status |
|---|----------|--------|
| P-1 | A recovery deadline. Recovered means VPP restarted, interfaces and LCP pairs applied again, and the FIB replay complete. Past the deadline Ze withdraws (R-6) | Adopted (A-2, R-10). Default value proposed at design review |
| P-2 | A crash-loop limit: N restarts inside a window, then Ze withdraws and reports through `ze doctor` | Adopted (A-2, R-10). N and the window proposed at design review. "Ze stops restarting" is replaced by R-11: Ze keeps retrying with a growing backoff |
| P-3 | Ze is the only restarter: the systemd method requires a unit that does not restart VPP itself (Restart=no, as VyOS) | Replaced by owner rule R-9: no unit policy is required; Ze reads the unit's effective settings when it acts and adapts |
| P-4 | The alive-test time of R-4, which is also the loss declaration | Value proposed at design review (today a 1 s probe tick and a 5 s probe timeout) |
| P-5 | The reconnect-and-verify limit of R-4 (A-1) | Value proposed at design review |

## Task

Today Ze runs VPP in one of two modes. In managed mode it starts the program,
and on the first failed API probe it kills it and starts it again. In external
mode (`vpp external true`) it only connects, never signals the process, and
reconnects after an API loss. In neither mode does the rest of Ze react to the
loss: no VPP-backed interface goes down in Ze's model, so OSPF, IS-IS, LDP,
RSVP-TE, VRRP, static routes, BGP and BFD learn that the data plane died only
from their own timers or from the kernel removing the linux-cp TAPs.

Goal: one lifecycle for every VPP Ze runs. Ze always holds a restart method
(R-1, R-2). A failed alive test declares the loss (R-4). Inside the second
limit, the reconnect-and-verify limit, a reconnect to the same process that
answers with the data Ze configured lets Ze adopt it with no kill and no
withdrawal, and reconcile in the background (form 1, R-7). When that limit
expires without form 1, or at once when the process exit is proven, Ze makes
the process terminal, restarts VPP by the method and reconfigures it (R-5),
accepting the blackhole meanwhile (form 2). Ze's own protocol timers keep
running throughout (R-6). Only past the recovery deadline or the crash-loop
limit (R-10) does it take every VPP-backed interface down in its model, so
IGP and BGP peers are updated (R-6); it then keeps retrying with a growing
backoff, and the interfaces and routes come back by themselves when VPP
recovers (R-11). With a systemd unit, Ze reads the unit's effective settings
each time it acts and adapts to them (R-9), and a process on the socket that
is not in the unit is reported, never refused or killed (R-12).

The PPPoE VPP data plane (`plan/post-release/features/spec-pppoe-vpp-dataplane.md`,
D-1 and OD-2) depends on this spec's decision procedure and events.

## Findings (read at the producers, 2026-11-23)

| # | Finding | Where |
|---|---------|-------|
| 1 | Nothing reacts to VPP loss. `vpp/disconnected` has one emitter and no subscriber outside tests. `iface` (`subscribeReconcileOnReady`) and `fib-vpp` (`onVPPReady`) subscribe to connected and reconnected only. The only process-exit observer is PPPoE (`RegisterProcessExitObserver`) | `VPPManager.retireReadiness` (`internal/component/vpp/startup_readiness.go`); `l2tp/pppoe/dataplane/vpp/reconcile.go` |
| 2 | No interface goes down in Ze's model. The VPP monitor emits `interface/down` only from a VPP `SwInterfaceEvent`. OSPF and IS-IS drop adjacencies only through `Transport.HandleLinkDown` from that bus; LDP, RSVP-TE, VRRP, static (`routeManager.onLinkEvent`) and connected routes read the same bus. So adjacencies and sessions die by timers or by kernel TAP removal | `monitor.handleEvent` (`internal/plugins/iface/vpp/monitor.go`) |
| 3 | BGP has no link-down reaction: the reactor holds no EventBus interface subscription and watches kernel addresses; `Reactor.handleAddressRemoved` stops listeners and closes no session. BFD imports no interface events | `internal/component/bgp/reactor/reactor_iface.go` |
| 4 | Managed mode kills VPP on the first failed probe (1 s tick, 5 s timeout). Any probe error (a stream error, a send or receive error, the timeout) reaches `connectionLost` at once, so the loss declaration is not R-4's alive-test time; external mode then never kills (R-5) | `waitVPPConnectionWithStop` (`internal/component/vpp/vpp.go`), `startHealth`, `probeVPPConnection`, `connectionProbeTimeout` (`conn_health.go`) |
| 5 | External mode cannot kill or restart. `runOnce` closes the connector; `runWithOps` backs off from 1 s, doubling to 30 s, and reconnects. No `PidfdSendSignal` exists anywhere, though each connection holds the peer pidfd. Corrected at design (2026-11-23): the claim "No systemd unit control exists" was wrong. `internal/plugins/systemd` already drives `systemctl` for `ze.service`: `nativeService.Stop` and `installSelected` (`native.go`) stop and start it, `effectiveUnit` (`native_unit.go`) reads LoadState, FragmentPath and DropInPaths, and `systemctlShowStop` (`doctor.go`) reads TimeoutStopUSec, KillMode and Restart and parses them with `parseSystemdTimespan`. What does not exist is control of any unit other than `ze.service`, and any read of a peer's cgroup or unit | `captureProcessWitness` (`process_peer_linux.go`, SO_PEERPIDFD); `internal/plugins/systemd/native.go`, `native_unit.go`, `doctor.go` |
| 6 | Graceful restart is already safe on reconnect: `restartFlagsFor` sets R and F only inside the `ze signal restart` marker window, and `fib-vpp` registers no `PreservesForwardingState` (only `fib/kernel` does). Exposure: a GR-capable peer keeps Ze's routes for its restart time after a silent TCP death | `internal/component/bgp/reactor/peer_gr_flags.go` |
| 7 | Reconnect already resyncs: `iface` runs `reconcileOnVPPReady`; `fib-vpp` `restart` replays and then sweeps stale routes after `labelSweepDelay`. Unverified: a reloaded iface backend re-creating loopbacks against a same-process VPP | `internal/component/iface/config_apply.go`, `internal/plugins/fib/vpp/` |
| 8 | `runWithOps`'s `backoff` only grows: it starts at 1 s, doubles to 30 s and never resets after a good run, so a VPP that crashes once a day restarts after 30 s (review N-1; relevant to P-2) | `VPPManager.runWithOps` (`vpp.go`) |
| 9 | `vpp/disconnected` is emitted by `retireReadiness`, deferred in `runOnce`, so it also fires on shutdown and on a replacement run, not only on a loss. A declared-loss subscriber must tell them apart (review N-2) | `VPPManager.runOnce` (`vpp.go`) |
| 10 | OSPF's transport opens a socket per interface name on link-up and hands the engine the handle's ifindex through `OnInterfaceUp` and `OnInterfaceDown`. A form (2) restart recreates the linux-cp TAPs with new ifindexes, so the engine must see a down and an up for each (review I-7) | `Transport.linkDown`, `OnInterfaceUp` (`internal/plugins/ospf/transport/transport.go`) |
| 11 | A full FIB readback is `IPRouteV2Dump` per table. At a million routes it can outlast a timer of seconds (review I-4) | `routeSource.dumpRoutes` (`internal/plugins/fib/vpp/routesource.go`) |
| 12 | Read at design (2026-11-23) while validating A-10; read, not run. Under the VPP interface backend the OSPFv2, OSPFv3 and IS-IS transports open their kernel sockets by the wrong index. Each resolves its interface with `iface.Resolve`, and the VPP backend answers with VPP's interface name and VPP's sw_if_index (`detailsToInfo` sets `OsName` to the VPP name and `Index` to `SwIfIndex`). OSPF then joins its multicast groups by that number as a kernel ifindex (`joinGroup`, `multicastDevice`), and IS-IS binds its packet socket to it (`OpenCircuit`). The kernel device of a VPP interface is its linux-cp TAP, created under the Ze interface name (`SetupLCPPair(name, name)`); its kernel ifindex is known only to the LCP pair read (`LCPPairReader.GetLCPPair`, `LCPPair.LinuxIndex`), which only PPPoE calls. No lab and no `.ci` runs an IGP over a VPP interface. The same transports ignore an `up` for an interface they hold open (`HandleLinkUp`, `openEnabledInterface`), and their socket readers end only on a closed descriptor (`readLoop` retries every other error), so nothing in them notices a TAP that the kernel deleted: after a VPP restart each keeps its socket on a device that is gone, because no `down` is published and the 30 s rescan reopens only an interface that is closed | `resolveOSPFInterface` (`internal/plugins/ospf/transport/backend_linux.go`); `resolveInterface` (`internal/plugins/isis/transport/backend_linux.go`); `detailsToInfo` (`internal/plugins/iface/vpp/query.go`); `internal/component/iface/lcp_pair.go` |

VPP facts. Settled at design (2026-11-23): each was ASSUMED when recorded and
is now read in VPP's source at Ze's pin, v25.10 (`cbba0451bb`,
`~/Code/github.com/FDio/vpp`). The "Was" column says what changed.

| # | Fact, as read | Source in VPP | Was |
|---|---------------|---------------|-----|
| V-1 | A crash ends the process: every fatal signal path ends in abort. linux-cp TAPs are created with the tun flag only, never the persist flag, so they vanish with the process, and the kernel deletes their addresses and routes | `unix_signal_handler`, `os_exit`; `src/plugins/linux-cp/lcp_interface.c` | Confirmed |
| V-2 | The binary API runs on the main thread and handles the messages of one socket in order. A handler not marked thread safe runs under the worker barrier, and that is the default. Workers forward while the main thread is stuck outside the barrier, and all stop while it is stuck inside | `src/vlibmemory/memclnt_api.c` | Confirmed |
| V-3 | ControlPing is marked thread safe, so an answer proves the main thread's API process ran and says nothing about the workers | `memclnt_api.c`, `vl_api_set_msg_thread_safe` for `VL_API_CONTROL_PING` | Sharpened: the recorded text said a missed ping cannot tell the two apart; an answered ping cannot either |
| V-4 | Nothing bounds a hung main thread. `BARRIER_SYNC_TIMEOUT` (1 s in a release build) bounds only the main thread waiting for the workers to reach the barrier. VPP has no watchdog | `src/vlib/threads.h` | New |
| V-5 | `ip_route_v2_dump` is not thread safe: a full route dump holds the barrier and stops forwarding for its whole run, and a ControlPing behind it waits. `sw_interface_dump` and `ip_address_dump` are thread safe. No linux-cp message is. Route add and delete are thread safe and take the barrier only when a pool grows | `src/vnet/ip/ip_api.c`, the thread-safe list | New. It conflicts with the harmless background dump A-4 assumed: see C-5 |
| V-6 | VPP never closes a live, well-behaved socket client, runs no keepalive toward it and buffers its replies without bound. A disconnect removes only the client's event registrations: routes, interfaces and LCP pairs stay, and `WantInterfaceEvents` must be renewed | The socket client transport and the `want_*` reapers, as the research read them; the file name was not recorded, so the implementer re-reads it before relying on this row (A-9) | New |
| V-7 | Lost while the main thread is blocked: linux-cp netlink message sync (batches of 2048 messages, 50 ms apart), neighbor updates, the stats collector | `src/plugins/linux-cp/lcp_nl.c` | Confirmed |
| V-8 | `/sys/heartbeat` is written by a main-thread process every stats update interval, 10 s by default, which Ze does not set. Interface counters are written in place by the workers. Per-node counters need `per-node-counters on`, which Ze does not set, and freeze with the main thread | `src/vlib/stats/init.c` | Corrected: the recorded text said per-worker input-node call counts advance every loop. They are not published in Ze's configuration. Form (1) uses no forwarding sample (A-4), so nothing in this design reads them |
| V-9 | The API socket opens only after every init and config function has run. VPP sends no readiness notification to systemd, and its packaged unit is Type=simple with Restart=always and no RestartSec | `vl_api_clnt_process`; `src/pkg/debian/vpp.service` | New |
| V-10 | SIGTERM only sets a flag the main loop reads, so a hung main thread does not exit on it: the kill is SIGKILL. Ze writes `full-coredump` (`startupconf.go`), so a crash writes its hugepage mappings before the process exits | `unix_signal_handler` | New. It confirms choice 5 and gives the cost in C-2 |
| V-11 | VPP publishes no figure for how long a handler, a dump or a start takes, and the Ze tree holds no measured VPP start time | VPP source and docs; Ze's logs and tests | New. The values proposed below rest on Ze's own constants and the protocol timers |

systemd facts. Settled at design (2026-11-23), read in systemd's source
(`~/Code/github.com/systemd/systemd`, main 5049191). Both recorded claims
were ASSUMED.

| # | Fact, as read | Source in systemd | Was |
|---|---------------|-------------------|-----|
| S-1 | A start request sent while the unit is activating merges with the running job: no second start, nothing charged to the start limit | `job.c`, `job_install`, `jobs_may_late_merge` | Confirmed for activating |
| S-2 | A start request sent while the unit waits in auto-restart is not harmless on version 257 and later: it skips the RestartSec wait, restarts the units bound to it and is charged to the start limit. On 252 to 256 it waits for the timer and merges | `service.c`, `service_start` | Corrected: the recorded text said it merges and is harmless. Ze therefore sends no start request while the unit is activating or in auto-restart |
| S-3 | `GetUnitByPIDFD` exists since version 253. Debian 12 and RHEL 9 ship 252, Ubuntu 22.04 ships 249 | NEWS 253 | Corrected: the recorded text assumed it on every version Ze supports. This design does not use it: see "Design" step D-3 |
| S-4 | Defaults: RestartSec 100 ms, StartLimitIntervalSec 10 s, StartLimitBurst 5. Automatic restarts count toward the start limit. At the limit the unit is failed with Result `start-limit-hit`, and a start fails until the window passes or the unit is reset. A reset clears the limit and zeroes NRestarts | `constants.h`, `service.c` | New |
| S-5 | SIGKILL of the main process gives Result `signal`. Restart= values on-failure, on-abnormal, on-abort and always restart it; no, on-success and on-watchdog do not. InvocationID changes on every start. Restart, RestartUSec and the start-limit properties change only on a daemon reload | `service.c` | New |
| S-6 | A unit with Requires=, BindsTo= or PartOf= on the VPP unit is restarted with it by systemd's automatic restart, and BindsTo= stops it when VPP's main process is killed. A plain start of an inactive VPP unit touches nothing else | The `JOB_RESTART_DEPENDENCIES` job type, as the research read it | New |

## References (clones on disk)

| Project | Revision | What it does on VPP failure |
|---------|----------|-----------------------------|
| osvbng (`~/Code/github.com/veesix-networks/osvbng`) | 753e934 | VPP is the systemd `vpp.service` with a drop-in (ExecStart runs VPP with its own config; ExecStartPost waits for the API socket), no `Restart=` in the drop-in; the Docker entrypoint backgrounds VPP. `frr.service` has Requires= and PartOf=vpp.service; `osvbng.service` Requires=vpp.service with Restart=always (`scripts/qemu/packer/scripts/configure-services.sh`). `VPPTarget.Restart` (`internal/watchdog/targets/vpp.go`) runs a systemd restart, or kill -9 and exec; the default action is warn (`buildRunnerConfig`, `cmd/osvbngd/main.go`), restart is opt-in. API loss (3 failed GetVersion): OnDown only pauses processing; nothing tells FRR, which learns from the kernel TAPs or the systemd PartOf restart. OnRecover uses an interface-count heuristic and replays sessions, with no identity or forwarding proof |
| VyOS (`~/Code/github.com/vyos/vyos-1x`) | db29bdc89 | VPP unit Restart=no, Type=notify, OnFailure=vpp-failure-handler.service (`data/templates/vpp/override.conf.j2`). The handler reloads the VPP section (`src/systemd/vpp-failure-handler.service`): systemd restart of VPP, LCP pairs, lcp resync (`src/conf_mode/vpp.py` apply). It never touches FRR (zebra sees the TAPs vanish and return). No long-lived API connection, so an API-only loss is invisible until the next commit |
| accel-ppp-ng (`~/Code/github.com/accel-ppp/accel-ppp-ng`) | 7e4ec88 | Disconnects and reconnects lazily; no identity check |
| osvbng-vpp (`~/Code/github.com/veesix-networks/osvbng-vpp`) | - | VPP plugins osvbng carries; not read for this spec |

None of the three proves process identity or forwarding before it resumes.

## Design draft (research, 2026-11-23; not approved)

On Ze's own mechanisms. Each step names the open choice it waits on.

1. **Restart method replaces the `external` boolean** (`yang/ze-vpp-conf.yang`,
   `VPPSettings.External`). Two methods: `exec`, today's managed spawn
   (`startManagedProcess`, the program from the host path or the embedded
   runtime), and `systemd` with a unit name. Verify refuses an enabled VPP with
   no method, and a systemd unit that does not exist. "External" then means
   only that Ze did not write `startup.conf` or bind the NICs. The users of
   `external true` and of `VPPSettings.External` convert; re-derive them by
   grep at design (the review's list, 2026-11-23: under
   `internal/le/test/deployment` `vpppppoe/product_plan.go`,
   `vpppppoe/product_state_test.go`, `vppevidencescenarios.go` and
   `vppifacescenarios.go`; the fixtures `misc_fixture_vpp.go`,
   `misc_fixture_vpp_shutdown.go`, `plugin_fixture_16_vpp.go` and
   `register_vpp_mpls_ldp_pop.go`; `config_test.go`, `vpp_test.go` and
   `newcleanup_outcome_test.go` (`External: true`); the readers
   `managed_startup.go` and `startup_cli.go`; `docs/guide/vpp.md`; the two
   `published-configuration` files under `internal/le/site/testdata/`). The
   IS-IS hits are route externality, not this leaf. This includes the PPPoE
   product lab and the `./le test vpp` stub (choice 7). Gaps against
   "manage" (review I-10), each owed an answer at design:
   - stopping an external VPP when Ze shuts down;
   - Ze starting beside a unit VPP already running: adopt it with no retained
     pidfd and no memory of what was installed, or restart it;
   - `exec` with an operator-written `startup.conf`;
   - the sidecar-container deployment, which the `external` leaf's YANG
     description names and which has no method;
   - PPPoE F-5's unread question (does the lab VPP run in another container)
     decides whether a pidfd kill works there.
2. **Kill by pidfd, never by PID**, in both methods: signal the retained pidfd
   of the connection's process witness. Death is the existing proof
   (`observeExit`, then `notifyProcessExit`). Signal choice: choice 5.
   2a. **systemd: detect and adapt (R-9).** No systemd control exists in the
   tree (finding 5). Steps:
   1. Read the unit's and service's properties over systemd's D-Bus API, or
      through `systemctl show` (which one: at design): Restart, RestartUSec,
      StartLimitBurst, StartLimitIntervalUSec, ActiveState, SubState, Result,
      InvocationID, NRestarts, ControlGroup.
   2. Read them at verify (the unit exists, refused otherwise) and again,
      fresh, at every declared loss before the kill. Never cache them: an OS
      update can change the unit between two reads.
   3. After the exit proof, decide from the unit state observed then:
      activating or auto-restart means systemd is restarting it, so Ze waits
      and proves that restart; failed or inactive means Ze starts the unit.
      Whether a redundant start request is harmless is read in systemd's
      source first (systemd facts above).
   4. A unit that hit its start limit: Ze keeps retrying after a withdrawal
      (R-11), so it resets the unit (`reset-failed`) before its next start.
      Recorder: that reset, and counting systemd's restarts toward the
      crash-loop limit (R-10), are derived; confirm at design.
   5. RestartUSec must fit inside P-1; a unit whose restart delay exceeds it
      is reported at verify or by `ze doctor`.
   6. Prove systemd's restart: the new API peer's pidfd must resolve to the
      configured unit with an InvocationID different from the one read before
      the kill, by unit-by-pidfd or by a cgroup check made while the pidfd is
      held; `ProcessLease.SameProcess(old, new)` false and the old exit proof,
      as the systemd analogue of `bindManagedProcess`. Never MainPID, which is
      a PID.
   7. A peer on the API socket that is not in the unit (R-12): Ze reports it
      and keeps working with it; it neither refuses nor kills it. The
      `ze doctor` report names the process, the configured unit, that the
      configuration may be out of date, and the exact fix (the config leaf
      that names the unit). Whether a declared loss over a foreign peer ends
      in a restart of the configured unit, or in reconfiguring the foreign
      process as at boot, is for the design to propose (no kill either way).
   8. The path missing today: VPP crashes and systemd restarts it before the
      alive-test time expires. The exit proof arrives at once on the retained, dead pidfd, so
      the loss is declared at once (step 3.1); Ze sends no kill and no start,
      and reconfigures the unit's new invocation as at boot.
   9. Unit coupling: when Ze's own unit has Requires=, BindsTo= or PartOf= on
      the VPP unit (osvbng's `frr.service` shape), killing VPP stops Ze.
      Detect it at verify or in `ze doctor`.
3. **Loss and recovery (R-4 to R-7, R-9 to R-12; narrows the first draft's
   adopt verdict to form (1) inside the reconnect-and-verify limit, and moves
   its down verdict to a failed recovery):**
   1. Two limits (R-4, A-1). The alive-test time (P-4) covers the alive test
      and the loss declaration; the reconnect-and-verify limit (P-5) follows
      the declaration and bounds form (1). Three states, which the code does
      not tell apart today (finding 4):

      | State | What Ze sees | Today | Draft |
      |-------|--------------|-------|-------|
      | Probe late, transport intact, inside the alive-test time | A ControlPing unanswered, the socket open | The 5 s probe timeout reaches `connectionLost` | Not a loss: nothing changes until the alive-test time expires |
      | Alive test failed | No answer for the whole alive-test time, or a stream or socket error while the retained pidfd has not exited | Any probe error reaches `connectionLost` at once | Loss declared; form (1) runs inside the reconnect-and-verify limit (step 3.1a) |
      | Reconnect-and-verify limit expired | No successful form (1) inside the limit | No such state | Kill and restart (step 3.2) |

      Recorder: whether a broken transport fails the alive test at once, or
      the alive-test time still runs on a new connection, is for the design.
      A proven process exit (`cmd.Wait` of the managed child, or the retained
      pidfd) declares the loss at once and skips form (1): a dead process
      cannot answer.
      Unknown outcomes (review I-3) concern every VPP owner (fib-vpp, iface,
      firewall, traffic, static, IKE, PPPoE), not only PPPoE E-1. Draft: a
      late probe with the transport intact fences new mutations and owes no
      readback, because replies on the same connection still arrive in order
      and settle each outcome; a broken transport fences every owner, and each
      reads back its unknown outcomes over form (1)'s new connection. Confirm
      at design against GoVPP's stream semantics.
   1a. Form (1), inside the reconnect-and-verify limit (R-7, A-4): Ze fences
      mutations and reconnects. It adopts the VPP when the new peer is the
      same process (`SameProcess` true on the retained leases) and answers
      over the API with the data Ze configured: "the presence of data" (Q-6)
      is the sign it is alive. Form (1) does not wait on a full readback
      (A-4 replaces the review's I-4 open choice and narrows "full readback
      matches"). After the adoption Ze lifts the fence and reconciles in the
      background:
      - Each VPP owner registers a reconciler with the `vpp` component, as
        PPPoE registers `RegisterProcessExitObserver`; the component names no
        owner, because a central list of owners is an enumeration. Across a
        process split the reconciler is a typed request, never a Go call.
      - The owners known today: `iface` (interfaces, LCP pairs, loopbacks),
        `fib-vpp` (routes and MPLS table 0 entries, `mplsentry.go`),
        `firewall/vpp`, `traffic/vpp`, `internal/plugins/static/backend_vpp_linux.go`,
        `internal/component/ike/dataplane/vpp.go`, and PPPoE (option C).
      - Background reconciliation is the existing style: `fib-vpp`'s replay
        and sweep and `iface`'s `reconcileOnVPPReady` (finding 7). A
        mismatch it finds is repaired, not treated as a loss. A million-route
        FIB dump (finding 11) runs in the background and never kills a
        healthy VPP because it is slow.
      - The baseline is the state acknowledged at the loss instant; unknown
        outcomes follow step 3.1.
      - Which data proves "configured data present" (an owner's quick read
        such as the interface and LCP pair dump, never the full FIB) is for
        the design to propose. Recorder.
      - Whether form (1) ends in `vpp/reconnected` (review I-5) is choice 8.
        I-5's first reason, that a resync hides a mismatch, no longer holds:
        A-4 makes the repair the intended path. Its second reason stands:
        PPPoE would see Ready with no Lost before it.
      A different process or an unprovable comparison fails form (1), and so
      does no configured data inside the limit; a mismatch alone does not.
   2. When the reconnect-and-verify limit expires without form (1), or the
      exit is proven, Ze fences mutations, reads the systemd unit fresh (step
      2a), makes the process terminal through the retained pidfd at once
      (SIGKILL, choice 5), and waits for the exit proof. When the exit is
      already proven, no kill is sent. The declared-loss event (choice 8;
      finding 9: it must not be confused with shutdown or replacement) is
      emitted at the declaration (step 3.1); whether the kill has an event of
      its own is choice 8 too. A foreign process is never killed (step 2a.7).
   3. Ze restarts VPP by its method, or, when the systemd unit restarts it by
      its own settings, waits for that restart and proves it (step 2a). It
      verifies the new process identity (`SameProcess(old, new)` false, old
      exit proven), applies the interfaces and LCP pairs again, and replays
      the FIB, as at boot. Then `vpp/connected` or `vpp/reconnected`, as
      today's resync expects. The new TAPs have new ifindexes (finding 10):
      the design names the interface events recovery publishes for them, and
      a test proves the OSPF and IS-IS adjacencies re-form over them.
   4. Meanwhile VPP-backed interfaces stay up in Ze's model and nothing is
      withdrawn: the blackhole is accepted (R-6).
   5. If recovery fails, Ze withdraws: `iface` publishes `interface/down` for
      every VPP-backed interface, and `ze doctor` reports it. "Fails" is his
      rule R-10 (A-2): the recovery deadline passes or the crash-loop limit is
      reached. Recorder: a start that fails counts as one restart toward the
      crash-loop limit; confirm at design.
   6. After the withdrawal Ze keeps retrying recovery with a growing backoff
      (R-11): never an operator command. `runWithOps`'s backoff (finding 8)
      is the base: it grows, and the design says when it resets after a
      recovery. When a retry recovers VPP (either form), `iface` publishes
      `interface/up` for the VPP-backed interfaces and the routes return
      through the existing reactions. `ze doctor` reports the withdrawn
      state, the retry count and the next attempt throughout.
4. **Peers updated on withdrawal only.** The VPP loss itself triggers no
   withdrawal (R-6). Ze's own BFD detection, OSPF and IS-IS dead and BGP hold
   timers keep running during recovery, as their RFCs say (A-5): nothing
   holds or extends them, so an adjacency or session over a VPP interface
   may go down by its own timer while Ze recovers. On withdrawal:
   - OSPF, IS-IS, LDP, RSVP-TE, VRRP, static and connected routes react
     through the existing `interface/down`, and routes leave sysrib.
   - BGP (review I-8): the reactor holds no EventBus interface subscription
     (finding 3), so the mechanism is an event BGP subscribes to; `vpp` or
     `iface` never names BGP. BGP sessions over VPP interfaces close locally
     (the peers detect it by hold timer or BFD). BGP sessions over a path
     that does not ride VPP keep running; the routes they learned through VPP
     leave with the interfaces (choice 3). Routes Ze originates and
     advertises over those sessions stay advertised after the interfaces go
     down; the design states which leave, and by what event.
   - BFD sessions on VPP interfaces go Down locally, which needs a hook.
   - On re-open no graceful-restart claim is made: the F bit stays 0, and the
     VPP restart path never writes the `ze signal restart` marker. OSPF and
     IS-IS restart signalling (grace-LSA, RR) stays off (not read).
5. **PPPoE.** The D-1 dead end disappears: a declared loss always ends in a
   terminal process, so the exit proof always arrives. This needs the PPPoE
   provider to take a process lease (F-1 in the PPPoE spec; F-2 and F-3 close
   with it). Form (1) keeps the sessions, inside the reconnect-and-verify
   limit only; the PPPoE option C readback runs in the background
   reconciliation and no longer gates the adoption (A-4). OD-2's drain runs
   on every forced restart (form 2): VPP's session state is gone, so sessions
   are torn down and subscribers redial. Lab (review I-2): a SIGSTOP shorter
   than the alive-test time does not break the socket, so it exercises the
   late-probe state (step 3.1), not form (1), and a red cut of the
   `SameProcess` adoption would stay green. Form (1) needs a real API-only
   drop with the process alive; a socket proxy cannot give one, because
   SO_PEERPIDFD would name the proxy. The mechanism is a design question.
   Cases: a freeze shorter than the alive-test time (nothing changes); an
   API-only drop (form 1, sessions survive, background reconciliation
   repairs); a freeze longer than both limits (a kill, a restart, the OD-2
   drain and a redial).

## Choices the rules do not settle (owner)

| # | Choice | Facts |
|---|--------|-------|
| 1 | The alive-test time (P-4) and the reconnect-and-verify limit (P-5) | Two values (A-1). Both proposed at design review for his approval. Today a 1 s probe tick and a 5 s probe timeout |
| 2 | What proves form (1) | Answered (A-4): the same pidfd-proven process answering with the data Ze configured; no full readback and no forwarding sample. Which data is read first is for the design (step 3.1a) |
| 3 | BGP sessions that do not ride VPP | Answered: no withdrawal while recovering; on withdrawal their VPP routes leave with the interfaces. Routes Ze originates over them: step 4 |
| 4 | systemd unit settings | Answered by R-9: Ze requires no unit policy and adapts to the settings it reads when it acts (step 2a). Still open: whether Ze starts an inactive unit at boot |
| 5 | SIGKILL or SIGTERM with grace | Answered: make the process terminal at once once the loss is declared (R-5) |
| 6 | PPPoE inside the reconnect-and-verify limit; E-1 on form (1) | Inside the limit PPPoE fences admission and holds its sessions; form (1) keeps them and reconciles them in the background (A-4), form (2) drains them per OD-2. E-1 (unknown-outcome creations on form 1) stays open: whether "repaired" (A-4) settles it is for the PPPoE design. Step 3.1 widens unknown outcomes to every VPP owner |
| 7 | Which method the functional-test VPP stub declares | Open: proposed at design review |
| 8 | Event names: a published plugin contract | Open: proposed at design review (`docs/architecture/api/process-protocol.md`). It covers the declared-loss event (finding 9), whether form (1) reuses `vpp/reconnected` (step 3.1a), the kill, the withdrawal and the recovery after a withdrawal, and the signal that fences mutations inside the limits, which has no event today |
| 9 | Adopting P-1 and P-2, and their values | Answered (A-2): both adopted (R-10). The deadline, N and the window are proposed at design review. A unit at its start limit is reset before the next retry (R-11, step 2a.4; recorder, confirm at design) |
| 10 | The two readings of R-4 | Answered (A-1): reading (b). A failed alive test is the declaration, and the reconnect-and-verify limit is a second value. Page row 5's forbids clause "a second window after the declaration" was written for reading (a); page row 10 supersedes it |
| 11 | After the withdrawal | Answered (A-3): Ze keeps retrying with a growing backoff; interfaces and routes return by themselves; `ze doctor` reports throughout (R-11) |
| 12 | The bound on form (1)'s readback | Answered (A-4): neither reading. Form (1) does not wait on a full readback; reconciliation runs in the background and repairs (step 3.1a) |
| 13 | Ze's own protocol timers during recovery | Answered (A-5): they run as their RFCs say; nothing holds them |
| 14 | A foreign process on the API socket | Answered (A-6): reported, never refused or killed; Ze keeps working with it (R-12, step 2a.7) |
| 15 | The bucket of this spec | Answered (A-7): post-release, not moved. The PPPoE spec stays post-release and depends on it |

## Design (2026-11-23, for approval)

This section is the design. It keeps the draft above as the record of what
was researched, and replaces it wherever the two differ. Each step names the
rule it serves, the fact it rests on and the existing mechanism it extends.
What only Thomas can decide is a row of "Choices for Thomas" (C-1 to C-19).

### D-1 The restart method replaces `external` (R-1, R-2)

| Leaf | Type and default | Meaning |
|------|------------------|---------|
| `vpp restart method` | enumeration `exec`, `systemd`; default `exec` | `exec`: Ze starts the VPP program itself and owns the process (today's managed mode, `startManagedProcess`). `systemd`: Ze drives the named unit. The default keeps every config without the leaf on the method it has today |
| `vpp restart unit` | string, default `vpp.service`; read only under `systemd` | The unit Ze starts, resets and checks the API peer against |
| `vpp restart startup-conf` | absolute path, absent by default; `exec` only | An operator-written startup.conf. Ze starts VPP with this file and renders none. Verify refuses it beside any `cpu`, `memory`, `dpdk`, `stats segment-size`, `lcp` or `plugins` leaf, because Ze cannot apply those to a file it did not write (exact or refuse), and refuses a missing or unreadable file |
| `vpp external` | removed | No mode remains where Ze only connects. Every config, fixture, lab and page that sets it converts in the same work (Changed-Wire Census) |

Under `systemd` Ze writes no startup.conf and binds no NIC: the unit's own
configuration owns both, as `external true` did. That is all "external" still
means. Verify at commit refuses `systemd` when the unit does not exist, and
when Ze could neither signal nor start it (D-9).

### D-2 The four limits (R-4, R-10; P-1, P-2, P-4, P-5)

All six values (the five below and `long-read-time`, the last row) are operator leaves under `vpp recovery`, in seconds, with
these defaults. The reason for leaves is V-11: no stall or start time is
published or measured, so a site with a full table or a slow NIC init must be
able to tune them. Verify refuses `alive-time` plus `verify-time` at or above
`deadline`.

| Item | Leaf, default, range | Reasoning | Cost if lower | Cost if higher |
|------|----------------------|-----------|---------------|----------------|
| P-4 alive test and loss declaration | `alive-time` 10 (2 to 300) | Ze probes every second, as today (`connectionProbeInterval`). The loss is declared when no probe has been answered for this long on an intact transport. A healthy main thread stalls only in a handler that is not thread safe (V-2), and a ping proves only that the main thread ran (V-3), so this test detects a hung control channel, never a forwarding failure: BFD (150 to 900 ms) and the IGP and BGP timers tell the peers about forwarding (A-5). Today's effective value is one 5 s timeout | An operator's `show ip fib` over a full table, or any long handler, fails the test on a healthy VPP | A hung main thread keeps a frozen FIB longer before repair starts |
| P-5 reconnect and verify | `verify-time` 20 (1 to 300) | Runs from the declaration. With P-4 it puts the kill at 30 s of silence, the IS-IS default hold, and before the OSPF default dead interval of 40 s: by then a peer without BFD is dropping Ze by its own timer, so the kill no longer costs an adjacency that would have lived. A wrong kill costs a full restart and every PPPoE subscriber redialling | Kills a VPP whose main thread is busy but healthy. Ze's own long reads are excluded (D-7), an operator's CLI is not | Nothing bounds a hung main thread (V-4): VPP blackholes that much longer before the kill. A crash whose core dump outlasts P-4 plus P-5 is cut short by the kill (V-10) |
| P-1 recovery deadline | `deadline` 120 (30 to 3600) | Runs from the declaration to the end of recovery (D-4, state Reconfiguring). Budget: up to 20 s of verify, the 45 s the tree already allows VPP to publish readiness (`startupReadyTimeout`), the interface and LCP apply, and margin. It is above the BGP default hold of 90 s, so every protocol session over VPP has already ended by its own timer before a withdrawal; the withdrawal then removes what no timer removes: connected and static routes in the system RIB, what is redistributed from them, and sessions sourced from a VPP address | A slow NIC or hugepage init passes it on a recovery that was about to succeed: Ze withdraws, then brings everything back seconds later | Routes Ze advertises over paths that do not ride VPP keep drawing traffic to a dead data plane for that long |
| P-2 crash-loop count | `crash-loop count` 3 (2 to 20) | The third restart needed inside the window withdraws at once, before it is attempted. Two restarts are absorbed with the blackhole his rule accepts. Each restart is an outage, a PPPoE redial and a flap toward every peer, and peers that damp flaps suppress a prefix after a handful of them | 2: one repeat crash withdraws | 5 or more: five outages and five flaps before Ze steps out, enough for a damping peer to suppress Ze's prefixes for longer than the withdrawal would have lasted |
| P-2 crash-loop window | `crash-loop window` 600 (60 to 86400) | A sliding window. A restart older than the window stops counting, so the count, the backoff and the return from a crash-loop withdrawal all come from this one list (D-6). systemd's own window (10 s, S-4) guards a tight loop; this one guards the network | A VPP that crashes every few minutes never reaches the count | A VPP that crashed twice this morning withdraws on its next crash tonight |
| Silence allowed during one of Ze's own declared long reads | `long-read-time` 60 (10 to 3600, not below `alive-time`) | Replaces `alive-time` while a declared read is open, and is a bound, never a suspension (D-7). It is above the IS-IS hold and the OSPF dead interval: a dump stops forwarding for its run (V-5), so by 60 s the peers have left and a kill costs nothing more. The value is C-16 | A healthy VPP is killed in the middle of a full-table dump when Ze restarts beside it | A main thread hung inside a dump stays undeclared that long (V-4) |

### D-3 Process identity, the unit, and the kill (R-5, R-9, R-12)

| Step | Design | Extends |
|------|--------|---------|
| Retained lease | The VPP manager takes a `ProcessLease` at readiness (`RetainProcessForGeneration`) and holds it across the loss. Today no production code holds one, and a lease can only be taken while connected, so it must exist before the loss | `process_peer.go` |
| Same process | Form (1) compares the lease of the new connection with the retained one through `SameProcess`. It answers an error, never false, when a lease is closed or its process exited: an error fails form (1) | `process_lease_equality.go` |
| New process | A restarted VPP is proven new by the retained lease's exit proof (`ProcessLease.Exit`), never by `SameProcess` answering false | `process_exit.go` |
| Unit of a process | Ze reads the process's cgroup under the held pidfd: the PID from the pidfd, its cgroup from the kernel, then the pidfd is checked still alive, which proves the PID was not reused in between. The cgroup is compared with the unit's ControlGroup property. This works on every systemd version, so `GetUnitByPIDFD` (S-3) is not used. Never MainPID | New, in a core leaf beside the witness |
| Unit reads and requests | Through `systemctl` (`show`, `start`, `reset-failed`), the tool `internal/plugins/systemd` already drives. The unit reader, with `parseSystemdTimespan`, moves to a core leaf (`internal/core/systemdunit`) that both the `systemd` plugin and the `vpp` component use: a component cannot import an edge plugin. Properties are read fresh before every action and never kept (R-9). A property a version does not have is absent, never zero | `internal/plugins/systemd/doctor.go`, `native.go` |
| Kill | SIGKILL through the retained pidfd (V-10: SIGTERM does not end a hung VPP), then the exit proof. Under `exec` this is today's kill of the child, which Go already sends through the child's pidfd. A foreign process is never signalled (D-5) | `waitVPPConnectionWithStop`; new signal on `processDescriptor` |

### D-4 The lifecycle (R-3 to R-7, R-10, R-11)

One state machine in the VPP manager replaces `runWithOps`'s loop and the
first-probe kill. The recovery deadline (P-1) runs from Lost to Ready.

| State | Entered when | What Ze does | Leaves to |
|-------|--------------|--------------|-----------|
| Ready | Readiness published | Probes every second | Lost |
| (late probe) | A probe is unanswered, the transport intact, inside P-4 | Nothing changes and nothing is published. While an owner's declared long read is open, the limit is `long-read-time` in place of P-4, never no limit (D-7) | Ready, or Lost at P-4 |
| Lost | No probe answered for P-4; or a transport error while the retained pidfd is alive, at once, because a dead socket has no probe to wait for; or a proven exit, at once | Declares the loss: publishes `vpp/lost` then `vpp/disconnected`, retires the connection generation (the existing fence: every new stream is refused), reports through `ze doctor`. Nothing is withdrawn (R-6) | Verifying; Terminating at once when the exit is proven |
| Verifying (form 1) | Lost with the process alive | Opens a new connection, one attempt a second, for at most P-5. Adopts when the peer is the same process, answers a ping, and every registered presence check passes (D-7) | Ready through `vpp/adopted`; Terminating at P-5 |
| Terminating | P-5 expired without form (1), or the exit is proven | Reads the unit fresh (D-6), kills through the retained pidfd unless the exit is already proven, waits for the exit proof. A foreign process: D-5 | Restarting. With no exit proof at the deadline: Withdrawn, and Ze sends the kill again at each backoff step and starts nothing until the exit is proven, because a second VPP beside a live one is the worse failure |
| Restarting | Exit proven | Counts one restart (D-6). Starts VPP by the method, or waits for the unit's own restart and proves it. Waits for the API and proves the new identity | Reconfiguring; Backoff on a failed start |
| Reconfiguring (form 2) | The new process is ready | Publishes `vpp/reconnected`: every owner applies as at boot (D-8). Recovery ends when every presence check passes on the new process | Ready. A VPP that answers but passes no presence check by the deadline: Withdrawn, and the attempt counts as a failed start, so at the next backoff step Ze kills that process and restarts it |
| Backoff | A start failed, or the next restart must wait | Waits the delay of D-6 | Restarting |
| Withdrawn | P-1 passed outside Ready, or the crash-loop count is reached | Publishes `vpp/withdrawn`. Keeps running Backoff and Restarting without end (R-11) | Ready through `vpp/recovered`, when VPP is ready and fewer restarts than the count lie inside the window |

What ends the recovery deadline is C-6: P-1 as adopted says "the FIB replay
complete", and the replay has no end marker today.

### D-5 A foreign process on the socket (R-12)

Foreign means: the method is `systemd` and the API peer's cgroup is not the
configured unit's. Under `exec` the peer must be Ze's own child, as
`bindManagedProcess` requires today: that is unchanged. A unit process Ze
cannot signal (D-9) follows the same table from "At P-5".

| Moment | What Ze does | Why |
|--------|--------------|-----|
| At connect, at boot or later | Configures it as at boot and works with it. `ze doctor` reports it (D-10) | R-12 |
| Alive test fails | Declares the loss exactly as for a unit process: `vpp/lost` with `foreign` true, the fence, the report | The loss is a fact about the API, whoever owns the process |
| Inside P-5 | Form (1), unchanged: the same process answering with its data is adopted | Form (1) needs only the lease, which holds for any process |
| At P-5 | No signal, and no start of the configured unit while the foreign process is alive: a second VPP beside it would fight it for the NICs, the hugepages and the socket, which is the outage R-12 forbids. Ze keeps trying form (1), once a second then at the backoff of D-6. The attempts have no end of their own: what bounds them is the deadline, at which Ze withdraws, and the process's exit. This is a reconnect attempt with no bound in the sense his row "Two limits" forbids, and the whole procedure is his to decide: C-17 | R-12: never killed |
| Toward the deadline | The deadline runs as for any loss. Past it Ze withdraws (C-7): it cannot program a data plane it cannot reach, and a frozen FIB under changing routes loops or blackholes | R-10. A withdrawal is neither a refusal nor a kill |
| The foreign process exits | The exit proof arrives on the retained pidfd. Ze then runs the configured method as in Restarting: it reads the unit fresh, connects to a unit process that is already there, or starts the unit. One restart is counted | R-5 from the exit proof on |
| A unit process appears on the socket | The new peer is in the unit and is not the retained process: Ze reconfigures it as at boot (`vpp/reconnected`), and the foreign report clears | Form (2) without a kill |
| It answers again after a withdrawal | Adopted: `vpp/adopted`, then `vpp/recovered` | R-11 |

### D-6 systemd adaptation, the crash-loop count and the backoff (R-9, R-10, R-11)

After the exit proof Ze reads the unit and acts on what it reads then:

| Unit state read | Ze does |
|-----------------|---------|
| ActiveState activating, or SubState auto-restart | Sends nothing (S-1, S-2) and waits for the unit's own restart, then proves it: the new API peer is in the unit's cgroup and the InvocationID differs from the one read before the kill |
| active, with a new InvocationID | systemd already restarted it: Ze connects and reconfigures. This is also the path where VPP crashes and the unit restarts it before P-4: the exit proof declares the loss, Ze sends no kill and no start |
| inactive, or failed with any Result but the start limit | Starts the unit |
| failed, Result `start-limit-hit` | Resets the unit, then starts it. Confirmed from S-4: without the reset the retry of R-11 cannot run. The reset happens only when Ze's own backoff says to try, so Ze's backoff governs once systemd has given up |

RestartUSec longer than the deadline, and a Requires=, BindsTo= or PartOf=
from Ze's own unit to the VPP unit (S-6), are reported by `ze doctor` and
never refused (R-9).

| Rule | Design |
|------|--------|
| What counts as a restart | Every new VPP process needed after a declared loss, whoever starts it: Ze's start, the unit's automatic restart, an `exec` spawn. A start attempt that fails counts one. When the unit's NRestarts advanced by more than Ze saw, the difference counts too, so a tight systemd loop Ze never connected to is counted |
| Crash-loop limit | When the restarts inside the window reach the count, Ze withdraws at once and keeps restarting |
| Backoff (finding 8) | Derived from the same list: no delay before the first restart inside a window, then 1 s doubling per restart in the window, to 30 s (`maxRestartBackoff`) while not withdrawn and to 300 s once withdrawn. Nothing resets it: a restart leaves the list when it is older than the window, so a VPP that crashes once a day restarts at once |
| Return after a withdrawal | `vpp/recovered` when VPP is ready, reconfigured, and fewer restarts than the count lie inside the window. After a deadline withdrawal that is the first successful recovery; after a crash-loop withdrawal it waits until the loop has aged out, so the interfaces do not flap with VPP |

### D-7 What form (1) reads, and long reads (R-7, A-4, V-5)

| Topic | Design |
|-------|--------|
| Presence checks | An owner registers a named, bounded check with the `vpp` component, as PPPoE registers `RegisterProcessExitObserver`; the component names no owner. `iface` registers the only one this spec needs: every interface it created is still in `sw_interface_dump`, which is thread safe (V-5). The same checks say when recovery is complete in form (2) |
| Why a registered Go function is sound here | Every VPP owner reaches VPP through `vpp.GetActiveConnector`, a pointer in the daemon's process, so a VPP owner runs in that process by construction. A forked plugin has no connector and follows the `vpp` events on the EventBus instead |
| Background reconciliation | Each owner subscribes to `vpp/adopted` on the EventBus, as `iface` and `fib-vpp` subscribe to `connected` and `reconnected` today. No reconciler registry is added |
| The FIB after form (1) | `fib-vpp` keeps its replay and sweep, and takes the mark from memory instead of from VPP. Today a withdrawal that arrives while fenced fails on the retired generation and `delVPPRoute` keeps the route in the writer's `installed` set, and `restart` then opens a writer with an empty set: without a mark that prefix would forward in VPP for ever. On `vpp/adopted` the new writer is therefore handed all four of the retired writer's records as its stale set, where a writer opened on a new process dumps (`markStale`, `markStaleLabelsLocked`): `installed` (IP routes), `mplsInstalled` (labeled routes, `processMPLSChange`), `labelOwners` (MPLS table 0 entries) and `labelPushes` (the push FECs of `mplsentry.go`, which are IP routes with labels). Today's dump covers the labeled pushes too, so a hand-over of two records would leave a labeled route or a push withdrawn while fenced in VPP for ever. SRv6 steering is not handed over: it is owned through durable records, and the adopting writer restores from them (`restoreSRv6`) as a writer does on every reconnect today, which also puts its MPLS and IP fallbacks back into `mplsInstalled` and `installed`. The system RIB and the label sources replay; every route or label they re-announce leaves the stale set; after `labelSweepDelay` the writer deletes what is left, which is exactly what was withdrawn while fenced. No `ip_route_v2_dump` and no `MplsRouteDump` is sent, so the adoption stops no forwarding for a dump (V-5). The same process was written to by this Ze alone under its FIB source, so the retired record is complete for IP routes; an add whose reply was lost at the loss instant is in neither record and is re-sent by the replay or, if withdrawn meanwhile, is the one leftover this cannot see (C-5). IP route adds and deletes are thread safe. No MPLS message is marked thread safe in VPP's `mpls_api.c` at the pin (as the main session read it), so each label add or delete of the replay takes the barrier for that one message: a brief stall per label, never a walk of the table |
| Interfaces after form (1) | `iface` reconciles without reloading its backend. The VPP backend opens a new channel and renews `WantInterfaceEvents`, which VPP dropped with the old connection (V-6). This settles finding 7's open question by not reloading |
| Declared long reads | An owner that must run a read VPP serves under the barrier declares it to the connector for its duration. A declared read never suspends the alive test: it replaces `alive-time` by a longer, bounded silence, `recovery long-read-time` (default 60 s, 10 to 3600, refused below `alive-time`). Any reply on the connection, a ping's or one of the dump's own, restarts the clock, so the read's progress counts as liveness; silence of every kind for `long-read-time` declares the loss with cause `alive-test`, exactly as outside a read. Progress alone is not enough, because VPP walks the table before it sends the first reply and buffers the replies of a socket client (V-5, V-6, as the research read them; A-9), so the walk is silent and needs the bound. When the read ends, or its owner fails to close it within `long-read-time` of the last reply, `alive-time` applies again. `fib-vpp` declares its mark dump, which still runs when Ze starts beside a VPP that kept its FIB. This keeps "a slow dump never kills a healthy VPP" true for Ze's own dumps up to the bound, and keeps every wait bounded: a main thread that hangs inside a dump is declared lost at `long-read-time`. The value is C-16 |
| Unknown outcomes (review I-3) | The fence is the retired generation. Form (1) always uses a new connection, and each owner settles its unknown outcomes in its `vpp/adopted` reconcile by applying its desired state again |

### D-8 Events and who reacts (choice 8; finding 3, 9, 10)

The `vpp` namespace, a published contract. The three existing events keep
their name, payload (empty) and meaning.

| Event | Published when | Payload |
|-------|----------------|---------|
| `connected` | First readiness of the daemon's life (unchanged) | empty |
| `disconnected` | Readiness retired: shutdown, replacement, or a declared loss (unchanged) | empty |
| `reconnected` | Readiness on a new VPP process (unchanged in name; now never the same process) | empty |
| `lost` (new) | The loss is declared, before `disconnected`. Never on shutdown or replacement, which settles finding 9 | `cause`: `alive-test`, `transport` or `process-exit`; `foreign`: true or false |
| `adopted` (new) | Form (1) succeeded on the same process. Form (1) never publishes `reconnected` (review I-5): a subscriber always sees `lost` before `adopted` | empty |
| `withdrawn` (new) | Recovery passed the deadline or the crash-loop count | `reason`: `recovery-deadline` or `crash-loop`; `restarts`: the count inside the window |
| `recovered` (new) | The first Ready after `withdrawn` | empty |

The kill has no event: the exit proof already reaches in-process owners
(`RegisterProcessExitObserver`), and `reconnected` tells everyone the state
is gone. The fence has no event of its own: `lost` raises it, `adopted` or
`reconnected` lifts it.

Interface events, published by the VPP interface backend
(`internal/plugins/iface/vpp`), which holds the names and the addresses:

| Moment | Published on `interface` | Effect |
|--------|--------------------------|--------|
| `vpp/withdrawn` | For every interface it holds: `addr-removed` for each address, then `down`. The shape `monitor.handleEvent` already publishes for a deleted interface | The withdrawal of R-6 |
| `vpp/reconnected`, not withdrawn | No `down` and no `addr-removed`. The reconcile publishes `created`, `up` and `addr-added` for the interfaces of the new process, as today. Their `index` is VPP's sw_if_index (`monitor.handleEvent`), which a restarted VPP given the same config can hand out again, so the index of the event does not say that the kernel device changed | A `down` here would be a withdrawal caused by the loss, which his row forbids, and it is one: static withdraws a route whose next hops are all down (`setLinkLocked`), RSVP-TE sends PathErr and removes the LSP (`engine.handleLinkDown`) and hears no `up` to restore it, and OSPF and IS-IS lose the adjacency and its routes. Instead the three IGP transports rebind on that `up` (the rebind table below), and static's VPP backend applies again on `reconnected`. C-8 |
| `vpp/recovered` | `up` for every interface that is up in VPP and `addr-added` for every address, read from VPP (the existing address replay) | The return of R-11, whichever form recovered |

Who reacts, each in its own package, none named by `vpp` or `iface`:

| Owner | On `lost` | On `adopted` | On `reconnected` | On the withdrawal's interface events |
|-------|-----------|--------------|------------------|--------------------------------------|
| `iface` and its VPP backend | Nothing | Reconcile without a reload; new channel and event registration | Reload and reconcile (today) | Publishes them |
| `fib-vpp` | Nothing; writes fail on the retired generation as today | Replay then sweep, with the retired writer's own record as the mark and no dump (D-7) | Mark by dump, replay, sweep (today) | Routes leave through the system RIB |
| `firewall/vpp`, `traffic/vpp`, `static` on VPP, the IKE VPP data plane | Nothing | Apply the desired state again | Apply the desired state again. None subscribes today (only `iface` and `fib-vpp` do), so a restarted VPP loses their state until the next commit: "as at boot" needs it | None |
| PPPoE | Fences admission, holds sessions | Keeps sessions; option C in the background | Drains per OD-2 (PPPoE spec) | None |
| Static | Nothing | Nothing | Its VPP backend applies again (the row above). The existing `moved` case (`setLinkLocked`) also reprograms a route when the `up` carries another sw_if_index. No withdrawal either way | Existing `down` reaction |
| OSPFv2, OSPFv3, IS-IS | Nothing | Nothing | New, the rebind (table below): on an `up` or `appeared` for an interface it holds open, the transport resolves the kernel device again and, when its ifindex is not the open socket's, opens a socket on the new device and closes the old one without the interface-down callback, so the adjacency and its routes are kept when neither side's timer has expired. The resolver delivers every `up` to a subscriber that already holds the interface, and keeps the newest event when a subscriber is behind (`onLinkEvent`, `sendLatest`); the newest event of a restart is that `up`, so a full channel cannot lose the rebind | Existing `down` reaction |
| VRRP | Nothing | Nothing | Existing wake on `up`: readiness is recomputed | Existing reaction |
| LDP | Nothing | Nothing | New, the rebind (table below). Read at design: `waitForInterface` subscribes only until the device first resolves and then cancels, and `discoverOnInterface` holds one multicast socket for the life of discovery, so today LDP hears no `up` once it runs and stays joined on the deleted TAP for ever | Nothing today: discovery does not subscribe once it runs. With the subscription kept for the rebind, a `down` still changes nothing in LDP: its Hello hold time and session keepalive end what the link no longer carries |
| RSVP-TE | Nothing | Nothing | Nothing is published that it reads: its LSP state stays | Existing `down` reaction (PathErr, teardown) |
| Connected routes | Nothing | Nothing | Nothing | Existing `addr-removed` reaction: the connected routes leave the system RIB, and what is redistributed from them leaves BGP and the IGPs |
| BGP | Nothing | Nothing | Nothing | New: C-9 |
| BFD | Nothing | Nothing | New, the loop reopen (table below, C-20). No BFD session outlives a restart: its detection time is under 3 s and runs untouched (A-5). What the design owes is the return, because a loop bound to a device keeps a socket on the deleted TAP (`loopFor`: "cannot be rebound") | New: C-9 |

The rebind, per protocol. Read at the producers at design (A-10, finding 12);
nothing here was run. On the default timers it keeps an adjacency only when
the outage is shorter than the dead or hold time of both sides, which run
untouched (A-5): a crash followed by a fast restart, not a hang, which is
killed at 30 s (D-2), the IS-IS default hold.

| Part | What the code does today | What the rebind needs | Keeps the adjacency |
|------|--------------------------|-----------------------|---------------------|
| The kernel device, all three | Finding 12: each transport takes VPP's index for a kernel ifindex | The resolver's `Binding` gains the kernel control device, its name and its kernel ifindex. Each interface backend fills it: the kernel backend with the device itself, the VPP backend from its LCP pair read (`LCPPairReader.GetLCPPair`, which must not invent a name or an index). The transports open on the control device and name no backend. An interface with no LCP pair has no control device: the open is refused, and the message names the interface and the `vpp lcp` leaf that creates the pair | It is what lets an adjacency form over a VPP interface at all (AC-36) |
| The trigger, all three | The event workers call `HandleLinkUp` for `up` and `appeared`, which returns at once for an open interface. The resolver drops its cached binding on every `created`, `up` and `down` | On `up` or `appeared` for an open interface, and at each 30 s rescan, the transport resolves again and compares the control device's ifindex with the open handle's. A resolve or an open that fails, because the pair or the address is not there yet, keeps the old handle and is retried on the next `appeared`, `up` or address event for the name and at the rescan (A-12) | - |
| OSPFv2 | The engine keys its interfaces by name (`markInterfaceDownLocked`, `startInterfaceUpLocked`) and turns the ifindex of a received packet into a name through the transport's open handles (`runningByIfIndexLocked`, `InterfaceNameByIfIndex`). It holds no ifindex of its own | The transport alone: swap the handle under its lock, join AllSPFRouters on the new socket, and join AllDRouters again when the old handle had joined it, which the transport must now remember per interface (`JoinAllDRouters` passes straight to the handle). No engine callback runs | Yes |
| OSPFv3 | The same engine. The Interface ID in Hellos and in the Link, Network and Router LSAs is the live index of the interface unless one was preserved for its name (`grInterfaceID`, `restoreInterfaceIDs`, built for RFC 5187). The RFC 4552 installer is keyed by the ifindex (`ipsecInstaller.onInterfaceUp`) | The transport as for OSPFv2, and two engine steps on a new moved callback: preserve the Interface ID in use for the name, in the existing preserved map, before the swap, so no Hello and no LSA changes; and install the interface's IPsec again for the new ifindex. The link-local of a new TAP can be tentative at first: the open is retried as it is today | Yes |
| IS-IS | The engine keys its circuits by ifindex (`e.circuits`, `launchCircuitGoroutine`), routes a received frame by the ifindex of its socket address, derives the local circuit ID from the ifindex (`localCircuitID`) and ties each circuit to one raw transport circuit by identity (`CircuitID`, compared in `onCircuitDown`). The transport has no up callback: the engine reads `CircuitInfo` | A new moved callback from the transport, carrying the old ifindex, the new one and the new raw identity. Under its circuit lifecycle lock the engine re-keys the binding to the new ifindex and identity and gives the running circuit its new ifindex. The local circuit ID and the SNPA already sent are kept and never derived again: a change of either is another neighbor to the peer | Yes, when the new TAP has the MAC of the old one (A-13) |
| Retiring the old socket, all | The three transport pages say a reader that ends "for any other reason" takes the link-down path (`receiveEnded`), and closing a socket ends its reader. `receiveEnded` returns when the state's stop channel is closed, and `linkDown(name, only)` does nothing for a state that is no longer the registered one | The order is fixed: under the transport lock the new state replaces the old one as the registered state for the name; then the old state's stop channel is closed; only then is the old socket closed. The old reader then ends into both guards and no down callback runs. A rebind that closed first would drop the adjacency it exists to keep | It is what makes the other rows true |
| A TAP in another network namespace, all | `LCPPair.Namespace`: empty means the namespace of the VPP process, "which callers MUST verify they can reach" | The control device is filled only when the pair's namespace is empty and a device of that name and kernel ifindex exists in Ze's own namespace. Otherwise the interface has no control device, and the refusal names the namespace: Ze opens no socket in another namespace | No adjacency: refused, never bound to a look-alike device |
| LDP | `resolveDiscoveryInterface` takes the binding's `OsName` to `net.InterfaceByName`, finding 12's shape by name; `discoverOnInterface` opens one socket for the life of discovery | Discovery opens on the control device and keeps its `iface.Subscribe` while it runs. On `up` or `appeared`, and at its retry timer, it compares the control device's kernel ifindex with the one its socket was opened on, opens a new multicast socket on a change and then closes the old one. Adjacencies and the TCP session are not touched by the swap | A Hello adjacency is kept only when the outage is under its 15 s hold time. Otherwise it expires by that timer and discovery forms it again on the new socket, with no Ze restart |
| BFD | A loop per VRF and mode. In the default VRF a single-hop loop is bound to a device only when every pinned session names the same interface (`resolveLoopDevices`); otherwise it is unbound and each packet is pinned by an ifindex control message (`pktinfoPin`). `loopFor` cannot rebind a live socket and tells the operator to restart Ze | The BFD component, which C-9 already subscribes to the `interface` namespace, also reads `up` and `created`. For a loop bound to that device it opens a new transport on the device and swaps it into the loop, then closes the old one: a new socket, never a rebind of a live one. Sessions stay; they are Down by then and come Up by themselves. For an unbound loop the socket is unaffected, and the pin must use the kernel ifindex of the device at each send: where the pin's index is resolved was not traced (A-14) | No: every session over VPP goes Down by its own detection time during the outage, and returns by itself after it (C-20) |

BGP and BFD (finding 3). Both bind to an address. BGP may not import `iface`
to turn a Ze interface name into the address it is bound to; BFD could
(`canonicalRequest` resolves the device), but one event then serves both. So
both react to their local address leaving Ze's model. The reactor and the BFD component each subscribe
on the EventBus to `interface/addr-removed` and `interface/addr-added`. A BGP
session whose local address is removed stops locally and its peer does not
open again until the address is added; routes learned over it leave, and the
peers of the other sessions are sent the withdrawals. A BFD session whose
local address is removed goes Down with diagnostic Path Down and keeps its periodic
transmission, as RFC5880-6.8.6-15 requires of a session that is not Up ("the
local system MUST send periodic BFD Control packets"): each send fails at the
socket while the address is absent and succeeds again once it is added, so
the session returns by itself. Its clients are told as for any Down. AdminDown
(RFC5880-6.8.16-5) is not used: it says BFD was administratively disabled,
not that the path failed, and a client takes no control protocol action on it
(RFC5882-4.2-1), so the routes would stay. Peer source, read at FRR f1ebf4328a and BIRD: FRR's `bgp_ifp_down`
(`bgpd/bgp_zebra.c`, "Take down directly connected peers.") stops a directly
connected peer whose local interface went down, and BIRD's `bgp_neigh_notify`
(`proto/bgp/bgp.c`) stops a direct session on a lost neighbor or a down link
and waits for it; FRR's `bfd_session_disable` (`bfdd/bfd.c`, "Set session
down so it doesn't report UP and disabled.") takes the session down with
Path Down. BIRD (2500b450) goes further: `bfd_neigh_notify`
(`proto/bfd/bfd.c`) frees the session request when the neighbor is no longer
reachable (`rfree(n->req)` under `nb->scope <= 0`) and requests it again when
it is. Ze keeps the session and takes it Down, as FRR does, so its clients
are told. A BGP peer stopped this way is in a new peer state, shown as
`address-wait`: deliberately down, and back by itself on `addr-added`. It is
not `PeerStateIdleHold`, which only an operator ends. Nothing here is a wire change: no NOTIFICATION can cross a dead
data plane, and the peers detect the loss by their own timers. On re-open no
graceful-restart claim is made: the F bit stays 0 and the restart path never
writes the `ze signal restart` marker (finding 6, unchanged).

Routes Ze originates in BGP configuration and advertises over a session that
does not ride VPP stay advertised after a withdrawal: C-10.

### D-9 Privileges (R-1, R-2)

Ze's shipped unit (`internal/plugins/systemd/unit.go`, `buildUnitFile`) runs
as user `ze` with `privilege.DaemonCapabilities`: CAP_NET_ADMIN, CAP_NET_RAW,
CAP_NET_BIND_SERVICE and CAP_BPF. It holds no CAP_KILL, so it cannot signal
a VPP that runs as root, and a start or reset of a system unit by a non-root
caller needs a polkit authorization. Under that unit, R-1 and R-2 are not
reachable with the `systemd` method today. The proposal is C-1. Whatever he
chooses, Ze probes both rights before it needs them: signal 0 through the
retained pidfd answers whether the kill is permitted, and the start right is
asked of polkit. A commit that selects `systemd` while a probe fails is
refused with the fix (R-1). At boot and at run time a failed probe is
reported by `ze doctor` and Ze keeps working with the VPP it has, as for a
foreign process, so an OS update that changes a right causes no outage (R-9).

### D-10 What the operator is told

One doctor code per problem class, each with its fix. Report-bus source
`vpp-recovery`.

| Check and code | Phase | Reports |
|----------------|-------|---------|
| `vpp-recovery`, `doctor-vpp-recovery` | Runtime | A loss in recovery (cause, since when, the state of D-4, the next step); a withdrawal (reason, restarts inside the window, the next attempt); the return |
| `vpp-restart-method`, `doctor-vpp-restart-method` | PostConfig | The unit does not exist; Ze cannot signal VPP; Ze cannot start the unit; RestartUSec exceeds the deadline; Ze's unit is tied to the VPP unit |
| `vpp-foreign-process`, `doctor-vpp-foreign-process` | Runtime | The process on the API socket is not in the configured unit: its PID, program and cgroup, the configured unit, that the configuration may be out of date, and the fix |

### D-11 The gaps against "manage" (review I-10) and the remaining open items

| Item | Design | Choice |
|------|--------|--------|
| Starting an inactive unit at boot (choice 4) | Ze starts it. A Ze that waits for someone else to start VPP is the unmanaged mode R-1 refuses | C-3 |
| Ze starting beside a running unit VPP | Ze adopts it: it connects, takes the lease, and applies as at boot. `fib-vpp` marks and sweeps what a previous run left, under a declared long read | C-4 |
| Stopping VPP when Ze shuts down | `exec`: the child is killed, as today. `systemd`: the unit is left running, so a Ze restart costs no data-plane outage | C-4 |
| `exec` with an operator startup.conf | The `startup-conf` leaf of D-1 | C-11 |
| The sidecar container | Removed from the pages. Ze can neither start nor signal a process in another container, so R-1 leaves it no method; `docs/guide/vpp.md` names an image that nothing produces | C-12 |
| PPPoE F-5 | Answered by the research: every VPP lab runs VPP and Ze in one container, one PID namespace and one user, so a pidfd signal works there | none |
| The stub's method (choice 7) | `exec`. The fixture stops starting the stub: Ze starts it, from the program path the registered `ze.vpp.binary` environment variable names. The functional tests then run the real kill and restart path | C-13 |
| Lab mechanism for an API-only drop | The stub closes its accepted API connections on a signal and keeps its state: a transport loss with the process alive, in a `.ci`. With a real VPP, a SIGSTOP longer than P-4 and shorter than P-4 plus P-5, then SIGCONT: the loss is declared on silence, the new connection completes after the SIGCONT, and the same process is adopted. A cut of the `SameProcess` adoption turns both red, because Ze then kills at P-5 | none |

## Pages this work makes wrong

| Page | What |
|------|------|
| `docs/research/vpp-deployment-reference.md`, "Socket Transport" | "Ze neither kills that process": a note now says the decision replaces it and this spec owes it |
| `docs/research/vpp-deployment-reference.md`, "Design decisions" row 6 | Its second sentence ("Ze is the only restarter ... Restart=no") and its forbids clause "or leaving the restart to systemd or another supervisor" are superseded by row 9; its timing ("as soon as a loss is declared") is narrowed by row 10. Removing it needs Thomas's `decision-change` |
| `docs/research/vpp-deployment-reference.md`, "Design decisions" rows 3, 4, 5, 7 and 8 | Rows 3 and 4 (rule (c)) are narrowed by rows 5 and 8; row 5 (one timer, "a declared loss is never adopted", "a second window after the declaration") is superseded by row 10 (A-1); row 7's "full readback ... matches" and "any mismatch means form (1) failed" by row 11 (A-4); rows 7 and 8 carried P-1 and P-2 as proposals, now adopted by row 12. Rows 10 to 15 restate everything rows 3 to 8 hold that still stands, so the six can be removed whole. Removing them needs Thomas's `decision-change` (handover 61) |
| `docs/architecture/l2tp/bng-5-pppoe.md`, "Design decisions" rows 1 to 3 | Rows 1 and 2 are narrowed by row 3; row 3 (the single timer, adoption on a full readback match, "a declared loss is never adopted") is superseded by row 4 (A-1, A-4), which restates what the three hold that still stands. Removing them needs Thomas's `decision-change` (handover 61) |
| `docs/guide/vpp.md`, "Running against an externally supervised VPP" | The whole mode; a note now says the same |
| `internal/component/vpp/yang/ze-vpp-conf.yang`, the `external` leaf description | Replaced by the restart method |
| `ai/digests/vpp-dataplane.md` | External mode and the loss path; corrected against today's code on 2026-11-23, rewritten when this lands |
| `docs/architecture/l2tp/bng-5-pppoe.md`, "VPP session ownership" | Owned by the PPPoE spec |

## Required Reading

### Architecture Docs
- [ ] `docs/research/vpp-deployment-reference.md` - VPP process lifecycle, socket transport, process identity
  → Decision: rows 1 to 15 of "Design decisions" (R-1 to R-7, R-9 to R-12; row 9 supersedes part of row 6, rows 10 to 15 carry his answers A-1 to A-6)
  → Constraint: process identity is the pidfs identity of the API peer, never a PID
- [ ] `docs/guide/vpp.md` - what the operator writes for each mode
  → Constraint: the section "Running against an externally supervised VPP" already says "This mode is going away" and names this spec; it is replaced by the two methods, and its "Python stub" and sidecar image are wrong today (the stub is Go, nothing builds the image)
- [ ] `ai/digests/vpp-dataplane.md` - the supervision loop and its consumers
  → Constraint: only `iface` and `fib-vpp` subscribe to the `vpp` events; every other VPP owner holds the connector and reapplies only at a commit
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - PPPoE owners retained across loss
  → Decision: rows 1 to 4 of "Design decisions" (D-1, option C, adoption inside the timer; row 4: adoption on the presence of data inside the reconnect-and-verify limit, background reconciliation)
- [ ] `docs/architecture/api/process-protocol.md` - events are a published contract
  → Constraint: a plugin subscribes to another namespace by naming it in `SubscribeEventsInput`; the page documents each namespace in a table of event, direction and payload (`system-dns`) and has no `vpp` section, which this spec adds
- [ ] `docs/architecture/bgp/interface-event-reactions.md` - how BGP reacts to interface and address events
  → Decision: the first interface flip is handled at once, bursts coalesce per peer ("Design decisions")
  → Constraint: the reactor watches the kernel address table on purpose and does not tear a session down when its address is removed; the session reaction of D-8 departs from this (C-9)
- [ ] `ai/doors/forwarding.md`, `ai/doors/system.md` - the boundaries
  → Constraint: interface events reach consumers on the EventBus, BGP and `iface` do not import each other, and a component cannot import an edge plugin

**Key insights:**
- The process witness, the lease and the exit proof exist; the kill, the
  restart method, the systemd reader, the readback verifiers and the reaction
  are missing.

## Current Behavior (MANDATORY)

**Source files read:** (by the 2026-11-23 research and review; re-read at design)
- [ ] `internal/component/vpp/vpp.go` - `runWithOps` restart loop and its backoff, `runOnce`, `waitVPPConnectionWithStop` kills the managed child on API loss
- [ ] `internal/component/vpp/conn_health.go` - `connectVPPOnce`, one health worker per connection, any probe error reports loss
- [ ] `internal/component/vpp/startup_readiness.go` - `retireReadiness` emits `vpp/disconnected`
- [ ] `internal/component/vpp/runtime_stats.go` - `activateRuntimeStatsGeneration` emits connected or reconnected
- [ ] `internal/component/vpp/process_peer.go`, `process_peer_linux.go`, `process_exit.go` - witness, lease, exit proof, `RegisterProcessExitObserver`
- [ ] `internal/plugins/iface/vpp/monitor.go` - `interface/down` only from VPP events
- [ ] `internal/plugins/ospf/transport/transport.go` - per-name sockets, ifindex handed to the engine on up and down
- [ ] `internal/plugins/fib/vpp/routesource.go` - the FIB dump
- [ ] `internal/component/bgp/reactor/reactor_iface.go`, `peer_gr_flags.go` - no link-down reaction; GR flags

**Behavior to preserve:**
- Process identity by pidfs, the exit proof, the readiness events and their consumers' resync.
- The GR flag rule: R and F only inside the `ze signal restart` window.

**Behavior to change:**
- The `external` mode, the first-probe kill, and the absent reaction to loss (R-1 to R-3).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VPP supervisor | `internal/component/vpp/vpp.go`, `VPPManager.runWithOps`, `runOnce` | Writes config, binds NICs, runs VPP, connects, restarts with a backoff that never resets | Extends: gains the decision procedure and the restart method |
| Operator-supervised mode | `internal/component/vpp/config.go`, `VPPSettings.External` | Ze only connects | Replaces: R-1 refuses an unmanaged VPP; the restart method takes its place |
| Process witness and lease | `process_peer.go` `Connector.RetainProcessForGeneration`, `process_lease_equality.go` `ProcessLease.SameProcess`, `process_peer_linux.go` `captureProcessWitness` | Binds each connection to the peer's pidfs identity | Extends (rewritten against D-3): the manager takes the first production lease, at readiness, and holds it across the loss. `SameProcess` proves form (1)'s reconnect reaches the same process; it answers an error, never false, so it cannot prove a new one. The retained pidfd is the kill handle, the signal-0 probe, and the handle under which the process's cgroup is read to name its unit; `GetUnitByPIDFD` is not used |
| Exit proof | `process_exit.go`, `observeExit`, `notifyProcessExit` | Publishes a proven process exit | Extends: a proven exit declares the loss at once, the restart waits on it, and it is the only proof that a restarted VPP is a new process (`ProcessLease.Exit`) |
| Process-exit observers | `process_exit.go`, `RegisterProcessExitObserver` | PPPoE registers to learn of an exit | Extends (rewritten against D-7): the same name-keyed registration carries the presence checks and the declared long reads, both for connector users in the daemon's process. It is not a reconciler registry: reconciliation is an EventBus subscription to `vpp/adopted` |
| Connection health | `conn_health.go`, `startHealth`, `probeVPPConnection` | Any probe error reports loss | Extends: tells the three states apart; the alive-test time (P-4) declares the loss, and the reconnect-and-verify limit (P-5) bounds form (1) |
| Stats connection | `stats_conn.go`, `runtime_stats.go` | Counters for telemetry and runtime readers | Does not apply: form (1) is proven by identity and the presence of configured data (A-4), not by a forwarding sample |
| Readiness events | `internal/core/vpp/events/events.go` | connected, disconnected (also on shutdown and replacement), reconnected | Extends: a declared-loss event distinct from shutdown, form (1)'s own event or none, a failed-recovery event (choice 8) |
| FIB resync | `internal/plugins/fib/vpp/`, `restart`, replay then sweep | Converges the FIB after reconnect | Extends (rewritten against D-7): form (2) marks by dump, replays and sweeps, as at boot. Form (1) replays and sweeps too, with the retired writer's `installed` set and label owners as the mark, so no dump is sent |
| Interface reconcile | `internal/component/iface/config_apply.go`, `reconcileOnVPPReady` | Reconciles interfaces when VPP becomes ready | Extends (rewritten against D-7): `reconcileOnVPPReady` reloads the backend and stays the form (2) path. Form (1) runs the body without the reload (`reconcileOnVPPEvent` with `reloadBackend` false), on `vpp/adopted` |
| Interface events | `internal/plugins/iface/vpp/monitor.go`, the `interface/down` bus | Driven by VPP interface events only | Extends: a failed recovery publishes `addr-removed` then `down` for every VPP-backed interface; a restart inside the limits publishes `created`, `up` and `addr-added` only, never `down` (D-8) |
| IGP transport open and close | `internal/plugins/ospf/transport/transport.go`, `HandleLinkUp`, `linkDown`, `OnInterfaceDown`, `OnInterfaceUp`, `RescanInterfaces`; the same in `ospf/v3/transport` and, with `openEnabledInterface` and `OnCircuitDown`, in `isis/transport` | Opens a socket per name on `up` or `appeared` and ignores both for an open interface; closes it on `down`; a 30 s rescan reopens a closed one | Extends (rewritten at design, A-10): an `up` or `appeared` for an open interface, and the rescan, compare the kernel device and rebind on a change, with no down callback (D-8, the rebind table) |
| Kernel device of a data-plane interface | `internal/component/iface/lcp_pair.go`, `LCPPairReader.GetLCPPair`, `LCPPair`; `internal/component/iface/resolve.go`, `Resolve`, `ResolveForBackend`, `bindingFromInfo` | The LCP pair read answers the TAP's name and kernel ifindex, for PPPoE only. `Resolve` answers the backend's own index, and `ResolveForBackend` exists for a consumer that must tell a kernel ifindex from a data-plane one | Extends: the binding carries the control device, filled from the pair read, so a kernel-socket consumer gets a kernel device from `Resolve` on either backend (finding 12) |
| OSPFv3 Interface ID preservation | `internal/plugins/ospf/gr_preserve.go`, `restoreInterfaceIDs`, `grInterfaceID` | Keeps the protocol Interface ID of an interface across a graceful restart, apart from the kernel index | Extends: the rebind records the ID in use in the same map, so a new TAP changes no Hello and no LSA |
| BGP interface reaction | `docs/architecture/bgp/interface-event-reactions.md`, `Reactor.handleAddressRemoved` | Stops listeners, closes no session | Extends: a failed recovery reaches BGP through an event it subscribes to |
| systemd control | `internal/plugins/systemd`: `nativeService.Stop`, `installSelected` (`native.go`), `effectiveUnit` (`native_unit.go`), `systemctlShowStop`, `parseSystemdTimespan` (`doctor.go`) | Drives `systemctl` for `ze.service` only: start, stop, LoadState, FragmentPath, DropInPaths, TimeoutStopUSec, KillMode, Restart | Extends (corrected: the row said "None"): the property reader and the timespan parser move to a core leaf both the plugin and the `vpp` component use, and gain `start`, `reset-failed` and the properties of D-6 for a named unit (D-3) |
| systemd D-Bus libraries | `go.mod`: `github.com/godbus/dbus/v5`, `github.com/coreos/go-systemd/v22`, both indirect | Linked through dependencies, unused by Ze's own code | Does not apply: the only call that needs D-Bus is `GetUnitByPIDFD`, absent before systemd 253 (S-3). The cgroup read under the held pidfd answers the same question on every version (D-3) |
| Connection generation fence | `conn_health.go`, `Connector.connectionLost`; `runtime_stats.go`, `errVppGenerationChanged` | A loss retires the generation, so every new stream and stats read on it is refused | Extends: it is the fence of the Lost state; `vpp/lost` is its EventBus form |
| Backend reload on reconnect | `internal/component/iface/config_apply.go`, `reconcileOnVPPEvent` with `reloadBackend`, `LoadBackend` | On `vpp/reconnected` closes the old VPP backend and its monitor, loads a new one, reconciles | Extends: stays the form (2) path, and publishes no `down` for the interfaces of the old process (D-8, C-8). Form (1) reconciles without the reload (D-7) |
| Interface resolver and the raw bus | `internal/component/iface/resolve.go`, `Subscribe`, `bindEvents`; the `interface` namespace on the EventBus | Two delivery paths. The resolver turns bus events into per-name link events for OSPF, IS-IS, LDP, VRRP and static by a Go call in the daemon's process; RSVP-TE and connected routes read the bus | Does not change: the withdrawal is published on the bus, so both paths carry it. BGP and BFD subscribe on the bus (D-8) |
| Deleted-interface publication | `internal/plugins/iface/vpp/monitor.go`, `monitor.handleEvent` | For a deleted VPP interface publishes `addr-removed` for each cached address, then `down` | Extends: the withdrawal publishes the same shape for every interface, without forgetting the names |
| Address replay | `monitor.go`, `monitor.replayAddresses` | Publishes `addr-added` for every cached address on request | Extends: `vpp/recovered` runs it, with `up` for each interface up in VPP |
| BFD link derivation and diagnostics | `internal/component/bfd/api/session_identity.go`, `deriveLink`; `bfd/packet/diag.go`, `DiagPathDown`, `DiagForwardingPlaneReset` | `deriveLink` maps a local address to a kernel link for the session key. Both diagnostics are defined and nothing produces them | Extends `DiagPathDown`: it is the diagnostic of a session whose local address left Ze's model, as FRR does. `deriveLink` does not apply: it names kernel devices and the withdrawal names Ze interfaces, so BFD keys on the address. `DiagForwardingPlaneReset` does not apply: the address event carries no cause, and no packet could carry the diagnostic across a dead data plane |
| LDP discovery interface and socket | `internal/plugins/ldp/register.go`, `waitForInterface`, `resolveDiscoveryInterface`, `discoverOnInterface` | `waitForInterface` subscribes to the resolver (`iface.Subscribe`) and retries every 5 s until the name resolves, then cancels the subscription and returns. `resolveDiscoveryInterface` takes the `OsName` of the `iface.Resolve` binding to `net.InterfaceByName`. `discoverOnInterface` opens one multicast socket on that device, pins its multicast egress to it and keeps it until discovery is stopped; a listen failure ends discovery on that interface. So LDP hears no interface event once discovery runs | Extends, the rebind of D-8's rebind table: discovery resolves the binding's control device (finding 12), keeps its subscription while it runs, and on `up`, `appeared` or its retry timer opens a new multicast socket when the device's kernel ifindex changed, then closes the old one. The adjacency table and the TCP sessions are not touched by the swap (AC-38) |
| BFD loops and their device | `internal/component/bfd/bfd.go`, `runtimeState.loopFor`, `resolveLoopDevices`; `internal/component/bfd/transport/udp_linux.go`, `pktinfoPin` | One loop per VRF and hop mode, created on first use and kept. `resolveLoopDevices` chooses a loop's device from the pinned sessions of one apply: the VRF name in a non-default VRF; in the default VRF, for single-hop, the interface when every pinned session names the same one, and none otherwise. `loopFor` hands back a live loop as it is: asked for another device, it logs that the socket cannot be rebound and tells the operator to restart Ze. `pktinfoPin` builds the control message that pins one packet to an ifindex and a source address, which is how a session on an unbound loop leaves by its interface | Extends, the loop reopen of D-8's rebind table (C-20): the BFD component reads `up` and `created`, and for a loop bound to that device opens a new transport on it, swaps it into the loop and closes the old one. No live socket is rebound and no session is removed. `resolveLoopDevices` and `pktinfoPin` do not change; whether the index handed to the pin is resolved at each send was not traced (A-14) (AC-39) |
| BGP address watch | `internal/component/bgp/reactor/reactor_address_watch.go`; `reactor_iface.go`, `handleAddressRemoved` | Follows the kernel address table and stops and starts listeners | Does not change. It cannot carry the withdrawal: the kernel removes a TAP's addresses when the VPP process dies, which is the loss, not the withdrawal, and keeps them while a foreign VPP hangs. The session reaction is a second, EventBus subscription (D-8, C-9) |
| Static route device move | `internal/plugins/static/link.go`, `routeManager.onLinkEvent`, the `moved` case | Reprograms a route when its device comes up with another index | Does not change. Under the VPP backend the index it compares is VPP's, which a restart can hand out again, so it is not what restores static routes after form (2): the VPP static backend applying again on `reconnected` is (D-8) |
| Embedded runtime start | `plan/release/delivery/spec-vpp-embedded-runtime.md` | Starts VPP from the bundle Ze carries | Extends: it is the `exec` method; that spec keeps where the program comes from |
| PPPoE loss handling | `plan/post-release/features/spec-pppoe-vpp-dataplane.md`, OD-2 and D-1 | Designed, not built | Does not apply here: PPPoE drains on this spec's declared-loss event |

## Design documents

Derived with `./le spec design documents` on 2026-11-23; the command
prints the current pages and decision rows. A `silent` verdict on a page that only anchors a changed file
is a judgment from the page's subject and from what changes in that file; the
phase that edits the file reads the page first.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Reach another unit through the boundary that survives a process split" | agrees | Every reaction is an EventBus subscription. The two Go registrations (presence checks, long reads) are for connector users, which share the daemon's process by construction (D-7) |
| `ai/doors/forwarding.md` | Boundaries | the door covering `internal/component/vpp`, `iface`, the FIB and VPP backends | "Interface events reach consumers on the EventBus. BGP and `iface` do not import each other" | agrees | BGP learns of the withdrawal from the `interface` namespace; neither `vpp` nor `iface` names it |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/vpp/events`, `internal/core/systemdunit` | "Never import upward." | agrees | `systemdunit` is a library with no lifecycle; the plugin and the component import it, it imports neither |
| `ai/doors/system-plugins.md` | Where it sits | the door covering `internal/plugins/systemd`, `static` | "If a feature starts to import it, it is a component and moves" | agrees | The `vpp` component does not import the `systemd` plugin: the shared reader moves to core |
| `ai/doors/access-security.md` | Boundaries | the door covering `internal/component/ike/dataplane` | "Readers of IKE state use the two core leaves" | agrees | The IKE data plane subscribes to `vpp` events itself; nothing reads IKE state |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering `internal/component/bgp/reactor` | "Never name a plugin package from the reactor." | agrees | The reactor subscribes to an event namespace in `internal/core` and names no package |
| `ai/doors/routing-protocols.md` | Boundaries | the door covering `internal/component/bfd` | "The BFD client lookup is in-process only." | agrees | Unchanged: BFD tells its clients of the Down as it does today |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/test`, `internal/le/test/deployment` | "A compiled Go fixture returns an error when an assertion fails" | agrees | The converted fixtures and the new lab keep that contract |
| `docs/research/vpp-deployment-reference.md` | Socket Transport | the `// Design:` page of the `vpp` component's files; a page this spec modifies | "Ze refuses to run VPP unmanaged" | agrees | The page already says the decisions replace external mode; this design builds it and the section is rewritten when it lands |
| `docs/architecture/fib/fib-depth-4-srv6.md` | Ownership and restart | anchored to `vpp.go`, `fib/vpp/register.go`; a page this spec modifies | "is not provided by this change; a Ze process restart reconnects and reconciles" | agrees | The page records the gap: no event when an external VPP restarts. After this work every VPP restart publishes `reconnected`, and the paragraph is corrected |
| `docs/guide/vpp.md` | Running against an externally supervised VPP | the `// Design:` page of `managed_startup.go`, `startup_cli.go` | "This mode is going away." | agrees | The section is replaced by the two methods |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | the `// Design:` page of `register_doctor_codes.go`; a page this spec modifies | "an IP route or labeled push a previous run left in VPP that could not be removed" | agrees | The page lists each runtime check with its owner and code; the three checks of D-10 are added as rows of the same table |
| `docs/architecture/api/process-protocol.md` | Subscription Namespace | the `// Design:` page of `events.go`; a page this spec modifies | "A non-empty" | agrees | A plugin subscribes to `vpp` by naming the namespace, as the section describes. The page gains the `vpp` section it lacks, in the shape of its `system-dns` section |
| `docs/architecture/fib/fib-vpp-route-ownership.md` | The sweep | the `// Design:` page of `routesource.go`; a page this spec modifies | "When it opens, before it installs anything or asks sysrib and the label" | agrees | The mark, the replay and the sweep all stay, on every writer open. Against a new process or after a Ze restart the mark is the dump the section describes, declared as a long read. On an adoption the mark is the retired writer's own record of the same routes, so the list the section calls "a previous run's" is known without asking VPP (D-7, C-5). The page gains both |
| `docs/architecture/traffic/fw-7-traffic-vpp.md` | Tolerant reconcile after a VPP restart | the `// Design:` page of `traffic/vpp/backend_linux.go`; a page this spec modifies | "No reconnect subscription is needed." | departs | The page relies on "the first Apply" after a restart, and an Apply runs only at a commit, so a restarted VPP has no policers until someone commits. "As at boot" (R-5) needs them at once: the backend subscribes to `adopted` and `reconnected` and applies again. The tolerant deletes the section describes stay. Its section "A Ze restart with external VPP" keeps its content under a heading that no longer says external |
| `docs/architecture/traffic/followup-vpp-traffic.md` | Reconcile | anchored to `traffic/vpp/backend_linux.go`; a page this spec modifies | "After a Ze restart, policer discovery returns names but no indices." | agrees | Unchanged; the link to the renamed heading is updated |
| `docs/architecture/bgp/interface-event-reactions.md` | The decisions | the `// Design:` page of `reactor_iface.go`; a page this spec modifies | "Watch the kernel address table, not the interface bus topics." | departs | The reactor also subscribes to the `interface` address events, and a session whose local address leaves Ze's model stops (C-9) |
| `docs/architecture/bfd.md` | Stage 2 complete (production transport hardening) | anchored to `bfd/session_identity.go` and `bfd/bfd.go`; a page this spec modifies | "target derived from pinned sessions." | agrees | The "Device choice" row stands: `resolveLoopDevices` still chooses a loop's device, and the reopen gives a bound loop a new socket on the same device name when the device is recreated, never a rebind of a live one (C-20). The "Egress link" row stands: an unbound loop still pins each packet by ifindex, and A-14 is whether that index is read at each send. The Down the address event causes goes through `session.Machine` like any other transition. The page gains the reaction with its diagnostic, and the reopen of a bound loop |
| `docs/architecture/testing/interop.md` | Real VPP SRv6 service-route proof | the `// Design:` page of the deployment scenarios; a page this spec modifies | "A normal Ze restart against the still-running external" | agrees | That proof stays: under `systemd` the VPP outlives Ze (C-4). Its config converts from `external true`, and the page gains the `vpplifecycle` suite |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | a page this spec modifies | "D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms." | agrees | This design supplies the two forms and the events PPPoE reacts to. The foreign-process exception is C-15 |
| `docs/functional-tests.md` | Choosing functional tests | the `// Design:` page of `cmd_vpp_stub.go`, `misc_fixture_vpp_shutdown.go` | "Runs stub-backed VPP scenarios and checks the stub request log." | agrees | Still true; the page gains that Ze starts the stub (C-13) |
| `docs/features/interfaces.md` | Bus Topics | the `// Design:` page of `iface/register.go`, `config_apply.go`; a page this spec modifies | "BGP does not read `interface/addr-added` or `interface/addr-removed` either," | departs | After this work BGP reads both, for one reaction: stopping and restarting the sessions sourced from the address (C-9). Its listeners still follow the kernel table, for the reason the section gives. The topic table's triggers also change: under the VPP backend `interface/down` and `interface/addr-removed` are published at a withdrawal as well |
| `docs/architecture/ospf/ospf-3-ip-transport.md` | Decisions | the `// Design:` page of `ospf/transport/transport.go` | "device by ifindex and the source by the interface IPv4 address" | agrees | The socket stays bound to one device by ifindex. The moved rule opens a new socket on the new device and closes the old one; it adds no second socket per interface, beyond the receive and transmit pair the section requires, which move together. The section, read whole, says three things the design meets. "The ifindex comes from the iface resolver, whose cache a link event clears when a device is re-created": the rebind's second resolve is that read, and under the VPP backend the resolver must answer the kernel device for it to be true (finding 12). Names resolve through the resolver, "never `net.InterfaceByName`": the control device is part of the binding, so that stays. And "A reader that ends for any other reason, such as a socket error, takes the same path (`receiveEnded`)": closing the old socket ends its reader, so the rebind retires the old state before it closes the socket (D-8, "Retiring the old socket"), and that reader then ends into the two guards the code already has and runs no link-down. The page gains the rebind and that order; a page this spec modifies |
| `docs/architecture/ospf/ospfv3-3-ipv6-transport.md` | Decisions | the `// Design:` page of `ospf/v3/transport/transport.go`; a page this spec modifies | "ifindex and hop limit come from the control message" | agrees | Receive is unchanged. The section, read whole, mirrors the OSPFv2 orchestrator ("the link lifecycle are the same"), so the rebind is the same in both, and it ends with "A reader that ends for any other reason takes the same path (`receiveEnded`)": the same retirement order keeps the old reader's end from running the link-down. The transport binds the send source to the link-local address the checksum covers: a rebind reads the new device's link-local again, and the adjacency is kept only when it is the address sent before (A-13). The page gains the rebind and the order |
| `docs/architecture/isis/isis-3-l2-transport.md` | Decision: one socket per circuit, with explicit multicast membership | the `// Design:` page of `isis/transport/transport.go` | "Each circuit binds its own `AF_PACKET` / `SOCK_RAW` socket to its ifindex" | agrees | One socket per circuit stays. Its section "Decision: a down callback identifies one raw circuit" ties a circuit's identity to its physical ifindex, so the rebind must hand the engine the new identity without the down callback: that is the moved callback of D-8's rebind table, read at design (A-10) |
| `docs/architecture/ospf/ospf-12-auth.md` | Decisions | anchored to `ospf/transport/transport.go` | - | silent | Packet signing is unchanged; the signer hook is kept across a rebind |
| `docs/architecture/ospf/ospf-ext-9-graceful-restart.md` | Decisions | the `// Design:` page of `ospf/gr_preserve.go`; a page this spec modifies | "Runtime Hellos, the neighbor table and native LSAs use the same ID even when the" | agrees | The line goes on "live kernel ifindex differs", and the section adds that transport receive and egress still use the kernel index. The rebind relies on exactly that split: it records the Interface ID in use in the same preserved map before the socket moves, so the protocol identity stays while the kernel index changes. The page gains that a rebind, and not only a graceful restart, preserves an ID. No grace-LSA is sent and no restart is signalled |
| `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md` | Decisions | the `// Design:` page of `ospf/ipsec_install.go`; a page this spec modifies | "Interface scoping uses the POLICY SELECTOR ifindex, not the XFRM interface" | agrees | Because a policy's selector holds the ifindex, a new TAP needs the interface's policies installed for the new index and the ones of the old index removed, which the moved callback does in that order, so no Hello leaves unprotected. The section says "On a link-down the engine removes the whole interface first": no link-down runs at a rebind, and the page gains the moved case beside it |
| `docs/architecture/ospf/ospf-5-interface-ism.md` | Traps | anchored to `ospf/instance.go` | "Link-down keeps the configured interface record and marks its runtime Down." | agrees | A rebind runs no link-down, so the runtime is not marked Down and the interface state machine sees no event. At a withdrawal the link-down runs exactly as the page says, and the record is kept for the return |
| `docs/architecture/ospf/ospf-ext-7-virtual-links.md` | Traps | anchored to `ospf/instance.go` | "interface, and must not accept one on an unknown or non-enrolled ifindex.** It" | agrees | The enrolled test turns an ifindex into a name through the transport's open handles. After a rebind the new TAP's ifindex is the enrolled one and the old one is unknown, so the rule holds on both sides of it |
| `docs/architecture/ospf/ospf-11-stub-nssa.md` | Decisions | anchored to `ospf/instance.go` | - | silent | Stub and NSSA handling is unchanged; the file changes only where the transport callbacks are registered |
| `docs/architecture/ospf/ospf-7-lsdb-flooding.md` | Decisions | anchored to `ospf/instance.go` | - | silent | Flooding is unchanged: a rebind originates and flushes no LSA, because the Interface ID is preserved |
| `docs/architecture/isis/isis-4-component-config.md` | Decision: a commit is written into the running circuit where it can be | anchored to `isis/circuits.go`; a page this spec modifies | "Each ifindex entry pairs its adjacency circuit with the raw transport's opaque" | agrees | The pairing and its mutex stay. The moved callback is a third operation under that mutex, beside publication and the down callback: it moves the entry to the new ifindex and gives it the new raw identity, so a delayed close of the old socket still retires nothing, as the section requires. The page gains it |
| `docs/architecture/ospf/ospf-4-component-config.md` | Constraints on callers | the `// Design:` page of `ospf/instance.go`; anchored to `ospf/transport/transport.go`; a page this spec modifies | "The transport exposes link-up and link-down callbacks because engine running" | departs | The line goes on "state tracks raw socket lifecycle across carrier flaps and config reloads". The rebind replaces the raw socket of a running interface and runs neither callback for OSPFv2, and a third, moved callback for OSPFv3: the engine's running state no longer follows the socket one for one. It is the cost of C-8 (a); under (b) the page stands as written |
| `docs/architecture/wire/ospf.md` | IPv4 transport | anchored to `ospf/transport/transport.go`; a page this spec modifies | "`mac/match` selectors behave like the IS-IS transport." | agrees | The transport still resolves through the shared resolver, and selectors behave the same. What it binds changes under the VPP backend only: the control device of the binding in place of VPP's own index (finding 12). No byte on the wire changes. The page gains the control device and the rebind |
| `docs/architecture/wire/ospfv3.md` | IPv6 transport | anchored to `ospf/v3/transport/transport.go`; a page this spec modifies | "by the periodic rescan and on `interface/addr-added`." | agrees | The retry the section describes for a link-local that is not ready is the retry the rebind relies on for a new TAP (A-12). The page gains the control device and the rebind. No byte on the wire changes: the Interface ID is preserved across the rebind |
| `docs/architecture/isis/isis-5-adjacency.md` | Transport additions this layer required | the `// Design:` page of `isis/circuits.go`; anchored to `isis/transport/transport.go`; a page this spec modifies | "circuit-info and ifindex-to-name accessors. Both additions are additive." | agrees | The moved callback is one more additive transport addition of the same kind, and the page gains it. The adjacency state machine sees no event at a rebind: the circuit keeps its table, its local circuit ID and its SNPA |
| `docs/architecture/firewall/fw-6-firewall-vpp.md` | What VPP programs, everything else rejected at commit | the `// Design:` page of `firewall/vpp/backend_linux.go` | - | silent | The page is about lowering firewall tables to VPP ACLs. This design changes when `Apply` runs (again on `adopted` and `reconnected`), not what it lowers |
| `docs/architecture/static-routes.md` | Startup ordering and emit rules | the `// Design:` page of `static/backend_vpp_linux.go`; a page this spec modifies | "A forward route whose next hops are all down is withdrawn." | agrees | That rule is unchanged, and it is why a restart publishes no `down`: a `down` for every VPP interface would withdraw every static route over them (C-8). At a withdrawal the rule runs as written. The page gains that the VPP static backend reopens its channel and applies again on `adopted` and `reconnected` |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | SPD dispositions | the `// Design:` page of `ike/dataplane/vpp.go`; a page this spec modifies | "(`followAddress`, on `addr-added`, `addr-removed` and `created`) rebuilds them," | agrees | The address worker keeps that reaction. It now also hears the `addr-removed` of a withdrawal and the `addr-added` of a return for VPP-backed addresses, and rebuilds as the section says. The page gains that, and that the VPP data plane reopens its channel and installs again on `adopted` and `reconnected` |
| `docs/architecture/ike/ipsec-dataplane-inspection.md` | Decision: a failed read is an error, never an empty table | anchored to `ike/dataplane/vpp.go` | - | silent | Inspection reads are unchanged |
| `docs/features/ai-first.md` | Shared Registrations | the `// Design:` page of `systemd/doctor.go` | - | silent | The page is about agent-readable diagnostics; the doctor check keeps its output and only takes its parser from the core leaf |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `systemd/doctor.go`, `unit.go` | - | silent | Ze's own signals and stop timeout are unchanged |
| `docs/architecture/cli/plugin-modes.md` | Offline flag completion | the `// Design:` page of `systemd/unit.go` | - | silent | The unit file changes only by the rights of C-1, which the page does not describe |
| `docs/architecture/system-architecture.md` | System RIB process boundary | anchored to `systemd/unit.go` | - | silent | The capability list it may cite changes only if C-1 (a) is chosen; phase 4 re-reads the anchor then |
| `docs/architecture/config/apply-ordering.md` | The decisions | anchored to `iface/register.go`, `monitor.go`, `reactor_iface.go`; a page this spec modifies | "itself once the synchronous binary API reply reports success." | agrees | Inside a commit the VPP backend still emits `addr-added` and `addr-removed` on the API reply, and commit ordering and `listener-ready` are unchanged. The page gains the two other moments the backend emits them, with no API call behind them: a withdrawal and a return (D-8) |
| `docs/architecture/config/transaction-protocol.md` | Side-Effect Events (produced during apply) | anchored to `iface/register.go`; a page this spec modifies | "IP address assigned. BGP reads the kernel address table itself" | departs | After this work BGP and BFD are consumers of `(interface, addr-added)` and `(interface, addr-removed)`, for the session reaction of C-9; BGP's listeners still read the kernel table. The table's consumer cells are corrected. No transaction step changes. The same departure as `interfaces.md`, "Bus Topics" |
| `docs/architecture/core-design.md` | 14. Interface Management | anchored to `iface/register.go`, `config_apply.go`, `fib/vpp/register.go`, `traffic/vpp/backend_linux.go`; a page this spec modifies | "BGP subscribes to `interface/` events and reacts: starting listeners when addresses appear," | agrees | The sentence goes on "draining sessions when addresses disappear", which is the reaction of C-9. This page and `interface-event-reactions.md` contradict each other today: that page says BGP watches the kernel table and tears no session down, and the code follows that page. C-9 puts both in front of Thomas; whichever he chooses, the other page is corrected |
| `docs/architecture/iface/logical-name-resolution.md` | Where the resolver lives, and what it returns | the `// Design:` page of `iface/resolve.go`; anchored to `iface/register.go`, `config_apply.go`, `bfd/session_identity.go`; a page this spec modifies | "a Linux ifindex from a userspace data-plane index. It checks the selected backend" | agrees | The section already separates the two indexes for a consumer that asks. The design puts the kernel side into the binding itself, as the control device, so a consumer that opens a kernel socket cannot take a data-plane index for a kernel one (finding 12). `Binding` stays a pure value; the page gains its two new fields. The section "Cache invalidation rides the monitor events" is what the rebind's second resolve relies on. The BFD reaction keys on the address and resolves nothing |
| `docs/architecture/iface/management.md` | The resolver fan-out discards the oldest, never the newest | anchored to `iface/register.go`; a page this spec modifies | "so the state an interface ENDED in always arrives." | agrees | The rebind of D-8 rests on exactly this guarantee: it needs only the newest event of a restart, the `up` published once the new TAPs exist, and never a `down` before it. The section also says the rescans repair "a lost `up` and not a lost `down`", which is why a design that needed each protocol to see `down` first could not be guaranteed. A withdrawal's `down` is the last event of its burst, so it arrives too. The page gains the moved rule beside static's |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | RFC obligations carried by this code | anchored to `iface/register.go` | - | silent | The PPPoE client path is unchanged |
| `docs/architecture/plugin/plugin-relationships.md` | 1. System Plugins | anchored to `iface/register.go`, `bfd/session_identity.go`; a page this spec modifies | "`addr/added` (start listener), `addr/removed` (stop listener)" | agrees | The page lists what each plugin publishes and subscribes to. BGP's row gains the session stop and start, BFD gains its `interface` subscription, and the `vpp` topics and their subscribers (sections 1 and 6) are added. No dependency between plugins is added |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `iface/register.go`, `config_apply.go` | - | silent | The process-boundary rule it states is the one the system door row answers |
| `docs/architecture/traffic/cos-plugin.md` | Decisions | anchored to `iface/register.go`, `config_apply.go` | - | silent | CoS apply is unchanged |
| `docs/architecture/traffic/cos-dynamic.md` | Decisions | anchored to `iface/vpp/ifacevpp.go` | - | silent | Dynamic CoS is unchanged |
| `docs/architecture/appliance/kernel-profiles.md` | Go owns the guarantee | anchored to `config_apply.go` | - | silent | No kernel profile changes |
| `docs/architecture/iface/offload.md` | Absence means "keep the OS default" | anchored to `config_apply.go` | - | silent | Offload settings are unchanged |
| `docs/architecture/mpls/mpls-kernel.md` | Label routes a previous run left in VPP | anchored to `fib/vpp/register.go`; the `// Design:` page of `register_vpp_mpls_ldp_pop.go`; a page this spec modifies | "A new writer removes them" | agrees | The stale-mark-then-sweep of the label table stays on every writer open. On an adoption the mark is the retired writer's label owners, so `MplsRouteDump`, which VPP serves under the barrier, is not sent (D-7). The page gains that case |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 1: the backend loads on demand | anchored to `firewall/vpp/backend_linux.go` | - | silent | Table ownership and the shutdown flush are unchanged |
| `docs/architecture/behavior/peer-lifecycle.md` | Peer states (distinct from FSM states) | anchored to `reactor_iface.go`; a page this spec modifies | "Only an operator recreating the peer brings it back." | agrees | `PeerStateIdleHold` is unchanged and is not reused. The table gains one state, shown as `address-wait`: deliberately down because the local address left Ze's model, and back by itself when the address returns |
| `docs/architecture/route-selection.md` | Overview | anchored to `reactor_iface.go` | - | silent | Selection is unchanged |
| `rfc/short/rfc5882.md` | Meta | the `// Design:` page of `bfd/session_identity.go` | - | silent | The one-session-per-system rule is unchanged: the reaction acts on sessions that exist and creates none |
| `docs/architecture/ldp/mpls-ldp.md` | Trap: an LDP-only config loads no iface backend | the `// Design:` page of `ldp/register.go`; a page this spec modifies | "LDP reads its interfaces through `iface.Resolve`" | agrees | It still does, and takes the binding's control device from it in place of the backend's own name (finding 12). The section "Decision: reconcile discovery, do not restart the engine" says stopping discovery tears a session down through the hold timer: the rebind does not stop discovery, it swaps the socket under a running one, so a session is lost only by its timer. The page gains the kept subscription and the rebind |
| `rfc/short/rfc5880.md` | Compliance Checklist | the `// Design:` page of `bfd/bfd.go` | "the local system MUST send periodic BFD Control packets" | agrees | Read at design at the requirement rows. Transmission (RFC5880-6.8.6-15): a session that is not Up sends periodic Control packets, so the session the address event takes Down keeps its transmit timer; a send fails at the socket while the address is absent, and nothing in the design stops the timer. Session Down: the diagnostic table gives 5, Path Down, to an external signal, which the address event is; the session goes Down through the same machine as a detection-time expiry and comes Up through it. Diagnostics and the AdminDown route (RFC5880-6.8.16-5, with 6.8.16-2 and 6.8.16-3 for its transmission): AdminDown is the administrator's disable and is not used here, because the path failed and nobody disabled BFD, and clients take no action on AdminDown (RFC5882-4.2-1). RFC5880-6.8.15-1 (Forwarding Plane Reset) is not claimed: the address event does not say the forwarding plane was reset (Existing Mechanisms, `DiagForwardingPlaneReset`). The loop reopen touches no requirement row: a session that lost its socket is Down by its detection time and returns through the same machine |
| `docs/architecture/ospf/bfd-client.md` | Decisions | anchored to `bfd/bfd.go` | "Only a path failure tears the adjacency; AdminDown does not." | agrees | The Down a VPP outage causes is a detection-time expiry, and the Down of C-9 carries Path Down: both are path failures, so OSPF tears the adjacency down as the section says, and neither is reported as AdminDown. This is why a BFD-protected OSPF adjacency does not survive a restart (C-20). The page is unchanged |
| `docs/research/bfd-implementation-guide.md` | Single-Hop (RFC 5881) | anchored to `bfd/bfd.go` | "**Bound to outgoing interface**: on Linux, the sending socket is bound via `SO_BINDTODEVICE`" | agrees | The binding stays, and no live socket is rebound: when the device is recreated the loop gets a new socket bound to the new device, which is the only way the rule can hold after a VPP restart (C-20) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | D-1 removes the unmanaged mode |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | The two methods of D-1. Under the shipped unit the systemd method needs rights Ze lacks: C-1 |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | Form (1), as his later rows narrow it to the reconnect-and-verify limit |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | The withdrawal of D-8, at the moment his later rows fix: past a limit |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | departs | The design has two limits. His row "Two limits" supersedes this one; removing it waits for his `decision-change` |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | departs | The kill follows the second limit, and a unit's own restart is used. His rows "systemd configuration" and "Two limits" supersede the timing and the only-restarter sentence |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | departs | Form (2) is as the row says. Form (1) no longer needs the full readback the row asks for: his row "Form (1) adopts on the presence of data" supersedes that part |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | Nothing is withdrawn inside the limits (AC-10) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | D-6: read before every action, no setting required or refused |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | departs | For a unit or child process the design is the row: D-2 and D-4, with a transport error or a proven exit also declaring at once, because neither is a missed probe. It departs for a process Ze may not kill, foreign (R-12) or unsignalable (D-9): when the second limit expires there is no kill, and Ze keeps trying to reconnect until the process answers or exits. The row forbids "a reconnect attempt with no bound"; his row on a foreign process forbids the kill. C-17 |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | Replay and sweep both run after an adoption, and `iface` reconciles. Only the source of the sweep's mark differs: the retired writer's record in place of a route dump, because the dump stops forwarding for its run (V-5). C-5 shows him that choice |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | D-2 proposes 120 s, and 3 restarts in 600 s |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | D-6 |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | No `down` and no `addr-removed` is published inside the limits, the restart included: the protocols move their sockets to the new TAPs on the `up` published for them, by comparing the kernel device (D-8). The timers run untouched (AC-12). The alternative that publishes `down` then `up`, and what each subscriber would then withdraw, is C-8 (b) |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | D-5 and D-10. Whether the deadline still withdraws over a foreign process is C-7 |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | Untouched: this design discovers no device |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | Untouched |
| `docs/architecture/bgp/interface-event-reactions.md` | Design decisions | decision of 2026-11-22 | "Interface and neighbor change re-advertisement runs the first flip immediately, then coalesces bursts behind a per-peer cooldown with at most one queued trailing re-pass; BGP route flap dampening (RFC 2439) is a separate concept and, if ever implemented, must be an option off by default ("flap dampening should be an option and off by default as since the RFC it was found to not help. 1A is fine.")" | agrees | The re-advertisement path is unchanged; the session stop is a different reaction and adds no settle delay and no dampening |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | The `vpplifecycle` suite declares only what its scenarios need |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 | "D-1, a VPP API loss with the process alive, is resolved by the VPP lifecycle rules: a live VPP that Ze cannot adopt is killed and restarted, so its exit proof always arrives and retires the retained owners" | departs | For a foreign process only: R-12 forbids the kill, so the exit proof does not always arrive. C-15 |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 | "When Ze adopts the VPP, retained owners are read back by exact identity over the new connection of the same pidfd-proven process, and cleaned there (option C)" | agrees | Form (1) always uses a new connection to the same proven process (D-7) |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 | "Adoption happens only inside the single VPP loss timer: a reconnect to the same pidfd-proven process whose full readback matches (option C) keeps the sessions. A declared loss is never adopted: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This narrows the row "When Ze adopts the VPP" to the single timer, and the D-1 row's "cannot adopt" to every declared loss" | departs | His later row on the two recovery forms supersedes it; removing it waits for his `decision-change` |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 | "D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms. Form (1), inside the reconnect-and-verify limit that follows the failed alive test: the same pidfd-proven process answering with the data Ze configured is enough to adopt it, and the PPPoE sessions are kept. The retained owners are read back by exact identity over the new connection of that process and cleaned there (option C), as part of the background reconciliation; a mismatch it finds is repaired, never treated as a loss. Form (2), a forced restart: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This supersedes the row "Adoption happens only inside the single VPP loss timer" (the single timer, adoption on a full readback match, and "a declared loss is never adopted"), and narrows the D-1 row and the row "When Ze adopts the VPP" to these two forms" | agrees | D-4 and D-7; `adopted` is the event PPPoE keeps its sessions on. The foreign exception to its form (2) is C-15 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The API loss reported by the connection health worker (`Connector.connectionLost`)
- A proven process exit (`cmd.Wait` or the retained pidfd)
- The `vpp` config root, for the restart method

### Transformation Path
0. Inside the alive-test time: a late probe changes nothing
1. Loss: the alive test fails or the exit is proven; the VPP component declares it, emits the declared-loss event and fences mutations
2. Form (1), inside the reconnect-and-verify limit: Ze reconnects; the same process answering with its configured data is adopted with no kill and no withdrawal, the fence lifts, and each owner reconciles in the background, repairing what it finds
3. Terminal, when the limit expires without form (1) or the exit is proven: read the systemd unit fresh, kill through the retained pidfd unless the exit is already proven (never a foreign process), wait for the exit proof
4. Recovery (form 2): restart by the method or prove the unit's own restart, check the new identity, apply interfaces and LCP pairs, replay the FIB
5. Withdrawal, only past the recovery deadline or the crash-loop limit: `interface/down` for each VPP-backed interface, the BGP event, the routing reactions, and a `ze doctor` report; then retries with a growing backoff, and `interface/up` when one recovers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VPP component to its consumers | EventBus `vpp` namespace, a published contract | No |
| VPP owners to the VPP component | Presence checks and declared long reads registered by Go call in the daemon's process, where every connector user already runs (D-7); reconciliation over the EventBus (`vpp/adopted`) | No |
| The VPP interface backend to BGP and BFD | EventBus `interface/addr-removed` and `interface/addr-added` | No |
| Ze to systemd | `systemctl show`, `start`, `reset-failed`; the peer's cgroup read under the held pidfd | No |
| iface to routing protocols | EventBus `interface/down` | No |
| Ze to VPP | Retained pidfd signal; systemd unit reads and start requests | No |

### Integration Points
- `VPPManager.runOnce` and `waitVPPConnectionWithStop` - the decision procedure replaces the first-probe kill
- `iface` VPP monitor - publishes `interface/down` when recovery fails
- `ProcessLease.SameProcess` - proves form (1)'s reconnect reaches the same process. Design correction (2026-11-23): it does not prove a restarted VPP is a new one; `ProcessLease.Exit` on the retained lease does (D-3)
- `iface` `reconcileOnVPPReady` and `fib-vpp` replay and sweep - form (1)'s background reconciliation
- Design correction (2026-11-23): on form (1) `iface` reconciles without the backend reload, and `fib-vpp` replays and sweeps with its mark taken from the retired writer's record, so no dump is sent (D-7, C-5); the reload and the mark by dump stay on form (2)
- `iface.Resolve` and the three IGP transports - the binding carries the kernel control device, and an `up` for an open interface rebinds on it (D-8, finding 12)
- `events.RegisterNamespace` in `internal/component/vpp/register.go` - registers the four new event types of D-8
- `Reactor` and the BFD component - each subscribes to the `interface` address events on the EventBus (D-8)

### Changed-Wire Census

**Search evidence:** searched on 2026-11-23 over `internal`, `cmd`, `pkg`,
`test`, `docs`, `ai` and `scripts`, Go and non-Go. Terms: `vppevents`,
`vpp/events`, the three event names beside `"vpp"`; `external true`,
`external false`, `External` under `internal/component/vpp`;
`EventAddrRemoved`, `EventAddrAdded`, `EventDown` of
`internal/core/iface/events`. Positive controls: the event search finds the
known writer `runtime_stats.go` and the known reader
`internal/component/iface/register.go`; the leaf search finds the known
reader `vpp.go` and the known writer `misc_fixture_vpp.go`. No shell, Python,
`.ci` or `.et` file reads a `vpp` event; the one non-Go hit is the digest
`ai/digests/vpp-dataplane.md`. The `.ci` files under `test/vpp/` name the
leaf in comments only; their config comes from the Go fixtures.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| EventBus `vpp` namespace | writer | `internal/component/vpp/runtime_stats.go`, `activateRuntimeStatsGeneration` | Emits `connected` or `reconnected` | Change: `reconnected` only on a new process; emits `adopted` on form (1) and `recovered` after a withdrawal |
| EventBus `vpp` namespace | writer | `internal/component/vpp/startup_readiness.go`, `retireReadiness` | Emits `disconnected` | Change: the declaration emits `lost` first; `disconnected` keeps its three causes |
| EventBus `vpp` namespace | writer | `internal/component/vpp/register.go`, `events.RegisterNamespace` | Registers the namespace with three types | Change: registers `lost`, `adopted`, `withdrawn`, `recovered` |
| EventBus `vpp` namespace | reader | `internal/component/iface/register.go`, `subscribeReconcileOnReady` | Subscribes to `connected` and `reconnected` | Change: adds `adopted` (reconcile without reload). The two existing subscriptions are preserved |
| EventBus `vpp` namespace | reader | `internal/plugins/fib/vpp/register.go`, `onVPPReady` | Subscribes to `connected` and `reconnected` | Change: adds `adopted` (replay and sweep, the mark from the retired writer and no dump). Preserved otherwise |
| EventBus `vpp` namespace | reader | `internal/plugins/iface/vpp/monitor.go`, `StartMonitor` | Names `connected` in a comment only; subscribes to nothing in the namespace today | Change: subscribes to `adopted`, `reconnected`, `withdrawn` and `recovered` to publish the interface events of D-8 |
| EventBus `vpp` namespace | reader | `internal/plugins/firewall/vpp/backend_linux.go`, `internal/plugins/traffic/vpp/backend_linux.go`, `internal/plugins/static/backend_vpp_linux.go`, `internal/component/ike/dataplane/vpp.go` | Each calls `vpp.GetActiveConnector`; none subscribes | Change: each subscribes to `adopted` and `reconnected` and applies its desired state again |
| EventBus `vpp` namespace | reader | `internal/component/l2tp/pppoe/dataplane/vpp/api.go` and its package | Registers `RegisterProcessExitObserver`; no event subscription | Preserve here; its `lost`, `adopted` and `reconnected` reactions are the PPPoE spec's |
| EventBus `vpp` namespace | reader | Tests: `startup_readiness_ownership_integration_linux_test.go` (asserts connected, disconnected, reconnected, disconnected) and the other `internal/component/vpp` tests; `internal/component/iface/config_test.go`; `internal/core/events/events_test.go` | They assert or emit the three existing names | Preserve: the three names and their empty payload do not change. The ownership test gains the `lost` it now sees before a loss's `disconnected` |
| EventBus `vpp` namespace | reader | A third-party plugin, by `SubscribeEventsInput` with namespace `vpp` | `docs/architecture/api/process-protocol.md` has no section for the namespace, though `events.go` names that page | Change: the page gains the section, with the table of D-8 |
| Config leaf `vpp external` | reader | `internal/component/vpp/yang/ze-vpp-conf.yang` (the leaf and two descriptions); `config.go` (the field, the known keys, the parse) | The schema and the parser | Change: the leaf is removed; `restart` and `recovery` are added |
| Config leaf `vpp external` | reader | `internal/component/vpp/vpp.go` (`runWithOps`, `runOnce`), `managed_startup.go` (`checkManagedStartupCLI`), `startup_cli.go` (`activateManagedRuntime`) | Branch on `VPPSettings.External` | Change: branch on the method |
| Config leaf `vpp external` | writer | `internal/test/fixture/misc_fixture_vpp.go`, `misc_fixture_vpp_shutdown.go`, `plugin_fixture_16_vpp.go`, `register_vpp_mpls_ldp_pop.go` | Write `external true` into the daemon config of the `.ci` suite | Change: `restart method exec`, with the stub as the program (C-13) |
| Config leaf `vpp external` | writer | `internal/le/test/deployment/vppevidencescenarios.go` (`vppExternalConfig`), `vppifacescenarios.go` (`vppBase`, `vppBaseLCP`), `vpppppoe/product_plan.go` (`productConfigWith`), `vpp_srv6_managed_integration_test.go` (`external false`) | Write the leaf into lab configs | Change: `restart method exec` with `startup-conf` where the lab writes its own file; the labs stop starting VPP themselves |
| Config leaf `vpp external` | reader | `internal/le/test/deployment/vpppppoe/product_state_test.go`; the unit tests of `internal/component/vpp` that set `External` | Assert the text or the field | Change: converted with the leaf |
| Config leaf `vpp external` | reader | `internal/le/site/testdata/published-configuration.md`, `published-yang-config-tree.json` | Generated goldens of the schema | Change: regenerated |
| Config leaf `vpp external` | reader | Pages: `docs/guide/vpp.md`, `ai/digests/vpp-dataplane.md`, `docs/research/vpp-deployment-reference.md`, `docs/architecture/fib/fib-depth-4-srv6.md`, `docs/features/srv6.md`, `docs/architecture/traffic/fw-7-traffic-vpp.md`, `docs/architecture/traffic/followup-vpp-traffic.md`, `docs/architecture/testing/interop.md`, `docs/architecture/l2tp/bng-5-pppoe.md` | Show or describe the leaf | Change: each is corrected in the phase that removes the leaf |
| Config leaf `vpp external` | writer | An operator's saved config | The YANG schema is the contract | Change: Ze is pre-release, so no converter is written; the commit error names the two `restart` leaves that replace it |
| EventBus `interface` namespace | writer | `internal/plugins/iface/vpp/monitor.go`, `monitor.emit` | Publishes `created`, `up`, `down`, `addr-added`, `addr-removed` | Preserve the payloads. Change: publishes them at the moments of D-8 as well |
| EventBus `interface` namespace | writer | `internal/plugins/iface/netlink/monitor_linux.go` | Publishes the same events from the netlink socket | Preserve: unchanged |
| EventBus `interface` namespace | reader | `internal/component/iface/resolve.go` (`bindEvents`), `register.go`, `iface.go`, `link_queue.go`, `unnumbered_donor.go` | The resolver and the component's own consumers | Preserve: the event format and the fan-out are unchanged, and they carry the new publications to OSPF, IS-IS, LDP, VRRP and static. Change, outside the wire: the `Binding` the resolver answers gains the kernel control device (finding 12) |
| EventBus `interface` namespace | reader | `internal/plugins/ospf/transport/transport.go`, `internal/plugins/ospf/v3/transport/transport.go`, `internal/plugins/isis/transport/transport.go`, each through `iface.Subscribe` (`subscribeIface`) | Open on `up` and `appeared`, close on `down`; an `up` for an open interface is ignored | Change: an `up` or `appeared` for an open interface rebinds when the kernel device changed (D-8). The `down` reaction is preserved |
| EventBus `interface` namespace | reader | `internal/plugins/rsvpte/register.go`; `internal/plugins/connected/eventbus.go`, `connected.go`; `internal/plugins/ospf/transport/transport.go` (`enqueueAddrEvent`); `internal/component/ike/engine/dynamic_wan.go` | Read `down` or the address events from the bus | Preserve: unchanged. Each now also hears a withdrawal |
| EventBus `interface` namespace | reader | `internal/component/bgp/reactor/reactor_iface.go` and its package; `internal/component/bfd/session_identity.go` and its package | Neither subscribes today (finding 3) | Change: each subscribes to `addr-removed` and `addr-added` (D-8, C-9) |
| EventBus `interface` namespace | reader | A third-party plugin subscribed to the namespace | The published event format | Preserve: no name or payload changes |

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The withdrawal reaches every protocol through the `interface` events they already read; routes leave through the system RIB, never by a FIB write |
| No unintended coupling (components stay isolated) | Yes | `vpp` and `iface` name no protocol. BGP and BFD subscribe to the `interface` namespace from their own packages |
| No duplicated functionality (extends existing, does not recreate) | Yes | Every row of Existing Mechanisms is extended in place; the systemd reader moves to a shared core leaf, it is not copied |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No encoding path changes. The events are control-plane transitions, a handful per loss |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Presence checks and long reads register with the `vpp` component by name; reactions are EventBus subscriptions; doctor checks register with `diagnostic.Register` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | The only list that learns the new event names is the namespace's own registration in `internal/component/vpp/register.go`. The method is a YANG enumeration, which completion and validation derive from the schema |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A healthy VPP main thread answers a ping inside 10 s outside Ze's own declared reads | V-2, V-4, V-11: no figure is published | P-4 kills busy VPPs; the default rises | The real-VPP lab under a full-table load, with the probe latency recorded | unvalidated |
| A-2 | VPP is ready inside the 45 s Ze already allows at boot | `startupReadyTimeout` (`startup_lifecycle.go`); V-11 | The 120 s deadline is too short for some hardware | The lab's recorded start-to-ready time; the leaf covers a slower site | unvalidated |
| A-3 | A full replay of route adds onto a live VPP stops no forwarding | V-5: route add is thread safe and takes the barrier only on pool growth; a re-add grows nothing | Form (1) would stall forwarding in the background | A lab run that forwards traffic through a form (1) replay and counts drops | unvalidated |
| A-10 | The OSPFv2, OSPFv3 and IS-IS transports and engines can move an interface to a new kernel device without their interface-down event, so the adjacency survives the rebind | Read at design (2026-11-23), not run. OSPFv2 transport: `HandleLinkUp`, `linkDown`, `rxLoop`, `receiveEnded`, `RescanInterfaces`, `addrWorker`, `JoinAllDRouters`. OSPFv3 transport: `HandleLinkUp`, `linkDown`, `rxLoop`. IS-IS transport: `subscribeIface`, `openEnabledInterface`, `linkDown`, `CircuitID`. Backends: `readLoop`, `resolveOSPFInterface`, `resolveInterface`. OSPF engine: `onInterfaceUp`, `onInterfaceDown`, `runningByIfIndexLocked`, `grInterfaceID`. IS-IS engine: `launchCircuitGoroutine`, `buildCircuit`, `onCircuitDown`, `localCircuitID`. Resolver: `onLinkEvent`, `sendLatest`, `bindEvents`, `resolve`. Verdict: it does not hold as first written, and holds with the changes of D-8's rebind table. No transport moves an open interface today, and the index of the event is VPP's, not the kernel's (finding 12), so the rule cannot be "another index in the event". With the control device in the binding, OSPFv2 needs its transport only; OSPFv3 also needs its Interface ID preserved and its IPsec installed again; IS-IS needs a moved callback and an engine re-key. Nothing read forbids any of the three | A protocol that cannot keep its adjacency loses it on every restart under C-8 (a), and the choice goes back to Thomas for that protocol with (b)'s costs | The rebind unit tests of each transport and engine, and AC-32 in the lab against FRR | unvalidated |
| A-12 | The LCP pair and the address of an interface are in the kernel by the time the reconcile has published the last of `created`, `up` and `addr-added` for it | The order of the reconcile in `config_apply.go` was read at design by position only, not traced: the pair is set up after the interface is created (`SetupLCPPair`). In the loopback path the pair is set up directly after the create, and admin-up and the addresses are applied later, so `up` and `addr-added` follow the pair there. The second `SetupLCPPair` call site was not traced | A rebind finds no control device at the event and waits for the 30 s rescan, which can outlast a dead interval | Reading the order in phase 3, the second `SetupLCPPair` call site included. If the pair follows the last event, the VPP backend publishes `up` again once the pair exists | unvalidated |
| A-13 | A recreated linux-cp TAP carries the MAC of the one it replaced, so the IS-IS SNPA and the OSPFv3 link-local source do not change | linux-cp gives the TAP the MAC of its VPP interface, as the design agent understands it; not read in VPP at the pin | A LAN IS-IS neighbor sees another SNPA and forms a new adjacency, and OSPFv3 sends from another source address: the adjacency is not kept for that protocol | Read at the pin at design (2026-11-23): in `lcp_itf_pair_create` (`src/plugins/linux-cp/lcp_interface.c`) the TAP's host MAC is copied from the Ethernet address of the VPP interface it shadows (`mac_address_copy` into `host_mac_addr`). So the TAP's MAC is the VPP interface's, and it is the same after a restart wherever VPP gives that interface the same MAC: a hardware NIC does. The MAC VPP gives a loopback or a sub-interface on a new process was not read. Phase 3 reads it; AC-32 proves it in the lab | unvalidated |
| A-14 | BFD's per-packet pin (`pktinfoPin`) is given the kernel ifindex of the session's device as it is at each send, not one kept from the session's creation | Not traced at design: only `pktinfoPin` and `resolveLoopDevices` were read, not the caller that supplies the index | Sessions on an unbound loop keep sending to the deleted TAP's index after a restart and never return | Reading the caller of `sendControl` in phase 6. If the index is kept, the session resolves it again on the `up` the BFD component now reads | unvalidated |
| A-11 | The label replay after an adoption, one barrier per label message, stalls forwarding by less than a route dump would | No MPLS message is thread safe in VPP at the pin (the main session's read); A-3 covers the IP replay only | An adoption on a box with many labels stalls forwarding | The `vpp-freeze-adopt` lab with a label table, counting drops through the replay | unvalidated |
| A-4 | The cgroup of a process, read while its pidfd is held and then checked alive, names its unit on cgroup v2 and on the hybrid layout | The research's fallback for S-3 | The foreign check misreports on some hosts | A unit test over both layouts, and the systemd lab | unvalidated |
| A-5 | Signal 0 through a pidfd answers the same permission as SIGKILL would | The kernel applies one permission rule to every signal | The doctor check passes and the kill is refused | A test that probes and then kills a process of another user, with and without CAP_KILL | unvalidated |
| A-6 | `firewall/vpp`, `traffic/vpp`, `static` on VPP and the IKE VPP data plane each have an apply entry point that can run again on an event | They hold a connector and apply at commit; only `iface` and `fib-vpp` import the `vpp` events | Their form (2) reaction needs more than a subscription | Reading each backend's apply path in phase 5 | unvalidated |
| A-7 | VPP reports an admin-up event after `iface` reapplies on a new process, so `interface/up` follows `created` | `monitor.handleEvent`; today's reconnect relies on it | No `up` follows the `created` of a restart, the rebind of D-8 has no trigger but the 30 s rescan, and the protocols stay on a dead socket until then | AC-11 and AC-32 in the lab | unvalidated |
| A-8 | The orphan a crashed Ze leaves under `exec` is handled by the existing pre-start cleanup | `cleanup_prestart_linux_test.go` exists; the path was not read at design | A Ze restart under `exec` meets a VPP it did not start | Reading the cleanup path in phase 2 and a test that restarts Ze over a live child | unvalidated |
| A-9 | The facts of V-1 to V-11 and S-1 to S-6 hold as the research read them | The research facts, starred ones re-checked by the main session | A limit or a reaction rests on a wrong fact | The implementer re-reads each producer its phase rests on | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator's long CLI command on the main thread fails the alive test and VPP is killed at P-5 | `doctor-vpp-recovery` reports a loss with cause `alive-test` while VPP was only busy | The two leaves; the report says which limit expired and how to raise it |
| R-2 | The kill at P-5 cuts a core dump still being written (V-10) | A truncated core after a crash | The report names it; `verify-time` can cover the dump time. A later kernel reports a dumping process through the pidfd, which would let Ze wait |
| R-3 | A rebind loses an adjacency that should have survived a short restart, through a protocol detail the reading missed: an Interface ID or a circuit ID that changes, a multicast group not joined again, a source address that differs | The peer logs an adjacency change on a restart shorter than its timers | AC-32 in the lab against FRR, one protocol at a time. A protocol that cannot keep its adjacency goes back to Thomas under C-8 |
| R-4 | A crash-loop withdrawal and its return oscillate | `vpp/withdrawn` and `vpp/recovered` alternating | The return waits until the loop has aged out of the window (D-6) |
| R-5 | A forked protocol plugin does not hear the withdrawal, because the interface resolver delivers by a Go call in the daemon's process | A forked OSPF or IS-IS keeps its adjacency state after `vpp/withdrawn` | The withdrawal is on the raw bus, which crosses the split. The resolver's reach is an existing gap this spec does not widen; it is reported to the main session for a journal row |
| R-6 | Two commits race the state machine: a config change that replaces the manager during a recovery | A `disconnected` with no `lost` during a recovery | A replacement ends the recovery in hand and starts as at boot; the test plan carries the case |
| R-7 | Ze's backoff and systemd's start limit fight: Ze resets the unit faster than the operator expects | `reset-failed` in the journal every attempt | Ze resets only on its own backoff, at most once every 300 s when withdrawn |
| R-8 | The spec is large for one implementation: the lifecycle, six owners' reactions, BGP, BFD, systemd and three labs | A phase that cannot finish inside one agent | The phases are cut so each lands alone; the main session may re-cut them into child specs |
| R-9 | A declared long read hides a hang: the main thread stops inside Ze's own dump and no probe is counted | A loss with cause `alive-test` whose report says a long read was open | The read extends the alive test to `long-read-time` and never suspends it (D-7, AC-30); the report names the read and the leaf. A full-table dump slower than the bound is killed, which the report also says, with the leaf to raise |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A healthy VPP killed (every session and subscriber over it drops), or a dead one kept (traffic blackholed with routes still advertised). A wrong withdrawal drops every route through the box |
| How is it reverted? | By reverting the commits; the removed `external` leaf means the configs converted in the same work revert with them. Nothing is persisted |
| Who else touches this path? | `plan/post-release/features/spec-pppoe-vpp-dataplane.md` (depends on the events), `plan/release/delivery/spec-vpp-embedded-runtime.md` (owns where the `exec` program comes from), and every VPP backend |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-11-23 | Nothing reacts to VPP loss: no `vpp/disconnected` subscriber, VPP-backed interfaces never go down even when VPP cannot be recovered |
| `transient-failure-treated-as-fatal.md` | 2026-11-23 | One failed API probe declares the loss, not the single timer R-4 asks for; external mode then never kills |

## Acceptance Criteria

Written at design (2026-11-23). The paragraph is the recorder's outline, kept
as recorded; the table below it is the criteria. The values are the defaults
of D-2. Each rule gets a row: an enabled VPP with no method refused at verify; a
probe missed inside the alive-test time changes nothing; a failed alive test
declares the loss; a same-process reconnect inside the reconnect-and-verify
limit that answers with its configured data is adopted with no kill and no
withdrawal, and a mismatch background reconciliation finds is repaired with
no kill; a slow FIB dump never kills a healthy VPP; a proven exit declares the
loss at once; the limit expiring without form (1) ends in a terminal process,
the exit proof, a restart by the method (or a proven unit restart) and a
reconfiguration, with nothing withdrawn while it recovers and no F bit on
re-open; Ze's own protocol timers keep running during recovery; the systemd
unit's settings are read fresh at each declared loss; a foreign process on
the socket is reported by `ze doctor` and neither refused nor killed; passing
the recovery deadline or the crash-loop limit takes VPP-backed interfaces
down in Ze's model so IGP and BGP peers are updated, and `ze doctor` reports
it; after a withdrawal Ze keeps retrying, and a recovery brings the interfaces
and routes back with no operator action.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A commit enables VPP with `restart method systemd` and a unit that does not exist | The commit is refused; the message names the unit, says Ze could not restart VPP without it, and gives the two fixes (name an existing unit, or select `exec`) | A config with no `restart` leaf commits and runs under `exec`, because that is the method it has today |
| AC-2 | A config carries `vpp external` | The commit is refused; the message names `vpp restart method` and `vpp restart unit` as the replacement | None: R-1 leaves no mode where Ze only connects |
| AC-3 | `alive-time` plus `verify-time` is at or above `deadline`, or a value is outside its range | The commit is refused naming the three leaves and the rule | Every combination below the deadline, because the operator tunes them to the site (V-11) |
| AC-4 | VPP answers no probe for less than `alive-time`, the socket open | VPP stays Ready: the same process, no `vpp` event, every session and adjacency over it still established, `ze doctor` clean | Any number of late probes inside the time: R-4 says a probe missed inside it is not a loss |
| AC-5 | VPP answers no probe for `alive-time`, the socket open | Ze publishes `vpp/lost` with cause `alive-test`, then `vpp/disconnected`; `ze doctor` reports the loss, its cause and the next step; every VPP-backed interface is still up in Ze's model | The interfaces, addresses and routes stay: R-6, the loss itself withdraws nothing |
| AC-6 | The API socket fails while the VPP process is alive | `vpp/lost` with cause `transport` is published at once and form (1) starts | Nothing is killed: the process is alive and inside `verify-time` |
| AC-7 | The VPP process exits | `vpp/lost` with cause `process-exit` is published within the exit observer's one-second poll, no signal is sent, and the restart begins | The unit's own automatic restart is used when the unit provides one (R-9) |
| AC-8 | After a declared loss the same process answers inside `verify-time` | `vpp/adopted` is published; the process identity is unchanged; routes the system RIB changed while fenced are in VPP afterwards; a route deleted from VPP during the loss is back; PPPoE sessions and protocol adjacencies are still established | A mismatch is repaired in the background and is never a loss (A-4). No interface event is published, because nothing left Ze's model |
| AC-9 | `fib-vpp` runs its mark dump for longer than `alive-time` against a VPP that kept a full table | VPP stays Ready and the dump completes, provided VPP is never silent for `long-read-time` | A probe that is late for a reason other than a declared read still counts toward `alive-time` |
| AC-30 | VPP's main thread hangs while a declared long read is open, and sends nothing more | `vpp/lost` with cause `alive-test` is published once the connection has been silent for `long-read-time` (60 s by default), and form (1) then the kill follow as for any loss | Silence shorter than `long-read-time` during the read: that is the allowance the read declares. Nothing is open-ended: R-4 forbids a wait with no bound |
| AC-10 | `verify-time` expires without form (1) on a unit or child process | The process receives SIGKILL through the retained pidfd, its exit is proven, VPP is started by the method, and `vpp/reconnected` is published; interfaces, LCP pairs, routes, ACLs, policers, VPP static routes and IPsec state are in the new VPP; no `vpp/withdrawn` and no `addr-removed` is published | The blackhole between the loss and the recovery: R-6 accepts it. No `down` is published either: static routes over VPP interfaces are programmed again by the VPP static backend on `reconnected` and never leave the system RIB, and RSVP-TE keeps its LSP state (C-8) |
| AC-11 | The same recovery, where the outage outlasted the dead or hold time of an OSPFv2, an OSPFv3 and an IS-IS adjacency over VPP interfaces, with a static route over one of them | With no `down` published and no operator action, each transport holds a socket on the new TAP, each adjacency forms again there and reaches Full or Up, and the peer holds Ze's routes again; the static route never left the system RIB | The adjacency that went down by its own timer during the outage: A-5. One whose timers had not expired is kept, not formed again: AC-32 |
| AC-12 | A BFD session and a BGP session run over a VPP interface while VPP is frozen for longer than their timers and shorter than the deadline | The BFD session goes Down at its detection time and the BGP session ends at its hold time, exactly as with no recovery in hand | Nothing holds or extends a timer: A-5 |
| AC-13 | Under `systemd`, the unit is activating or in auto-restart after the exit proof | Ze sends no start request, waits, and proves the restart: the new peer is in the unit's cgroup under another InvocationID; then it reconfigures | A unit with Restart=no is started by Ze instead (AC-14): no unit policy is required (R-9) |
| AC-14 | Under `systemd`, the unit is inactive or failed after the exit proof; in a second case its Result is `start-limit-hit` | Ze starts the unit; in the second case it resets the unit first, and the start succeeds | The reset runs only when Ze's backoff allows an attempt |
| AC-15 | The unit's Restart= setting is changed and reloaded between two losses | The second loss acts on the new setting: Ze starts a unit that no longer restarts itself, or waits for one that now does | No setting is refused: R-9 |
| AC-16 | Under `systemd`, the process on the API socket is not in the configured unit | Ze configures it and forwards through it; `ze doctor` reports `doctor-vpp-foreign-process` with the PID, program and cgroup, the configured unit, that the configuration may be out of date, and the fix | The process itself: R-12, it is never refused |
| AC-17 | That foreign process stops answering for longer than `alive-time` plus `verify-time` | The loss is declared with `foreign` true; the process is still running afterwards and received no signal; the configured unit was not started; when it answers again it is adopted | The foreign process keeps running: R-12, it is never killed |
| AC-18 | That foreign process exits | Ze starts the configured unit, reconfigures the new VPP as at boot, and the foreign report clears | None: with the foreign process gone, R-5 applies in full |
| AC-19 | Recovery passes `deadline` | `vpp/withdrawn` with reason `recovery-deadline` is published; for every VPP-backed interface Ze publishes `addr-removed` for each address and `down`; the connected and static routes over them leave the system RIB; OSPF and IS-IS adjacencies over them go down in Ze; BGP sessions sourced from their addresses stop and the routes learned over them are withdrawn from the other peers; BFD sessions on those addresses are Down with diagnostic Path Down; `ze doctor` reports the withdrawal, its reason and the next attempt | Interfaces VPP does not back, and the sessions and routes over them, are untouched. BGP routes configured in Ze and advertised over such sessions stay (C-10) |
| AC-20 | A third restart is needed inside `crash-loop window` | `vpp/withdrawn` with reason `crash-loop` is published before the third restart is attempted, with the reactions of AC-19 | The first two restarts run with the interfaces up: R-6 |
| AC-21 | After a withdrawal, VPP stays down, then starts | Ze attempts a restart at growing intervals up to 300 s with no operator command; once VPP is ready and reconfigured, `vpp/recovered` is published, the interfaces and addresses return to Ze's model, adjacencies and sessions re-form, the routes return, and the doctor report clears | After a crash-loop withdrawal the return waits until fewer restarts than the count lie inside the window (R-4 of Risks) |
| AC-22 | VPP crashes once, more than `crash-loop window` after its last restart | A new VPP process is started as soon as the exit is proven and reaches Ready, and the restart list then holds one entry | A second crash inside the window waits one second, then two: the backoff still grows inside a loop |
| AC-23 | A BGP session re-opens after a VPP restart | The OPEN Ze sends carries the graceful-restart capability with the F bit 0 for every family | The F bit Ze sets inside a `ze signal restart` window over the kernel FIB is unchanged (finding 6) |
| AC-24 | A commit selects `systemd` while Ze may not signal VPP's process or start its unit | The commit is refused; the message names the missing right and the exact fix | At boot and at run time the same condition is reported by `doctor-vpp-restart-method` and Ze keeps working with the VPP it has, so an OS update causes no outage (R-9) |
| AC-25 | Ze stops | Under `exec` the VPP process is gone when Ze has exited; under `systemd` the unit is still active with the same InvocationID | Under `systemd` VPP keeps forwarding through a Ze restart (C-4) |
| AC-26 | Ze starts while the configured unit already runs VPP | Ze connects, configures it as at boot and reports Ready; the unit's InvocationID is unchanged | Routes a previous Ze left in VPP are swept by the existing mark and sweep |
| AC-27 | Ze starts under `systemd` and the unit is inactive | Ze starts the unit and reaches Ready | None: R-1 |
| AC-28 | `restart startup-conf` names an operator's file under `exec` | VPP runs with that file and Ze reaches Ready on the socket `api-socket` names; a `cpu`, `memory`, `dpdk` or `plugins` leaf beside it refuses the commit naming the leaf | The `api-socket` and `stats socket-path` leaves stay: Ze needs them to connect |
| AC-29 | A plugin subscribes to the `vpp` namespace | It receives `lost`, `adopted`, `withdrawn` and `recovered` with the payloads `docs/architecture/api/process-protocol.md` documents, in a forked process as in the daemon | The three existing events, unchanged |
| AC-31 | A route is withdrawn from the system RIB while VPP is fenced, and the same process is then adopted | Once the sweep delay has passed after `vpp/adopted`, VPP's FIB holds every route the system RIB holds and does not hold the withdrawn prefix; the same holds for a label entry, a labeled route and a labeled push (a FEC with out-labels) each withdrawn while fenced; a route, a label and a labeled push that were not withdrawn are still there; the request log shows no route dump and no label dump | An add whose reply was lost at the loss instant and that was withdrawn before the adoption can stay until the next mark by dump (C-5) |
| AC-32 | VPP exits and is restarted, and the outage is shorter than the dead or hold time on both sides of an OSPFv2, an OSPFv3 and an IS-IS adjacency over VPP interfaces | Each adjacency is still Full or Up, on a socket bound to the new TAP, and the peer logged no adjacency change; the OSPFv3 Interface ID and the IS-IS local circuit ID Ze sends are the ones it sent before; a static route over the interface stayed in the system RIB throughout; an RSVP-TE LSP over it kept its state and sent no PathErr | An adjacency whose timer expired on either side during the outage is lost by that timer and forms again (A-5, AC-11). On the default timers a hang is such a case: the kill comes at 30 s |
| AC-36 | OSPFv2, OSPFv3 and IS-IS are enabled on a VPP-backed interface that has an LCP pair, before any loss | Each opens its socket on the pair's TAP, by the TAP's kernel ifindex and not by VPP's index, and its adjacency with a peer on that link reaches Full or Up. On a VPP interface with no LCP pair the open is refused, and the log names the interface and the leaf that creates the pair | The kernel backend, where the control device is the device itself and nothing changes |
| AC-37 | `interface/addr-removed` is published for the configured local address of an established BGP session, on any interface backend | The peer's state reads `address-wait` wherever peer state is shown; its session is closed; while the address is absent Ze dials nothing for that peer and refuses a connection from it; the routes learned from it are withdrawn from the other peers. When `interface/addr-added` is published for that address the peer leaves `address-wait` and its session establishes again with no operator command | Peers bound to another local address keep their sessions. A peer in `PeerStateIdleHold` stays there on both events: only an operator ends that state. A peer with no configured local address, whose session ran from the removed address, is closed the same way and reconnects at once from the address routing gives it: it has no address to wait for |
| AC-38 | LDP runs on a VPP-backed interface with an LCP pair; VPP is then killed and restarted, in one case inside the 15 s Hello hold time and in another past it | Before the loss LDP sends and receives Hellos on the pair's TAP and its session with the peer is operational. After the restart, with no `down` published and no Ze restart, discovery holds a socket on the new TAP. In the first case the Hello adjacency and the session never left; in the second both form again and the label bindings return | The adjacency lost by its own hold time in the second case: A-5 |
| AC-39 | A BFD session is pinned to a VPP-backed interface, alone on its loop so the loop is bound to that device, and a second pair of sessions on two interfaces shares an unbound loop; VPP is killed and restarted | Every session goes Down at its detection time during the outage. After the restart each returns to Up over the new TAP with no Ze restart and no operator command, and its clients are told of both transitions | No session stays Up across the restart: the detection time is shorter than any VPP start, and A-5 lets it run (C-20) |
| AC-33 | A foreign process stays silent past `deadline` | `vpp/withdrawn` is published with the reactions of AC-19, the process is still running and unsignalled, the unit was not started, and `ze doctor` reports both the withdrawal and the foreign process with the fix | The process itself: R-12. Whether Ze withdraws at all here is C-7 |
| AC-34 | That foreign process answers again after the withdrawal | It is adopted: `vpp/adopted` then `vpp/recovered`, the interfaces and addresses return to Ze's model, and the foreign report stays until the configuration names its unit | The foreign report: the configuration is still out of date |
| AC-35 | The unit's RestartUSec exceeds `deadline`; in a second case Ze's own unit has Requires=, BindsTo= or PartOf= on the VPP unit | `ze doctor` reports `doctor-vpp-restart-method`, naming the setting, its value, what it will cause (a withdrawal on every restart; Ze stopped or restarted with VPP) and the exact fix | VPP runs and the commit is accepted: R-9, no unit setting is required or refused |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `vpp restart method systemd` with a missing unit, at commit | → | The `vpp` component's commit verify | `test/vpp/vpp-restart-method-refused.ci` |
| The VPP process stops answering probes | → | The lifecycle state machine in `internal/component/vpp`: Lost, Verifying, Terminating, Restarting, Reconfiguring | `test/vpp/vpp-loss-restart.ci` |
| The API connection drops with the process alive | → | Form (1): `SameProcess`, the presence checks, `vpp/adopted` | `test/vpp/vpp-loss-adopt.ci` |
| `vpp/withdrawn` on the EventBus | → | The VPP interface backend publishes `addr-removed` and `down`; connected routes leave the system RIB | `test/vpp/vpp-loss-withdraw.ci` |
| `interface/addr-removed` for a BGP session's local address | → | The reactor's address subscription stops the session | `test/vpp/vpp-loss-withdraw-bgp.ci` |
| `interface/addr-removed` for a BFD session's local address | → | The BFD component takes the session Down with Path Down | `test/vpp/vpp-loss-withdraw-bfd.ci` |
| `interface/addr-added` for the local address of a peer in `address-wait` | → | The reactor's address subscription starts the peer again | `test/vpp/vpp-loss-return.ci` |
| `interface/up` for an interface an IGP transport holds open, after a VPP restart | → | The resolver's control device, the transport's rebind, the moved callback | `vpp-crash-rebind-frr` (lab: the stub creates no TAP) |
| VPP starts again after a withdrawal | → | Backoff, Restarting, `vpp/recovered`, the interface replay | `test/vpp/vpp-loss-return.ci` |
| `ze doctor` during a recovery | → | The `vpp-recovery` check | `test/vpp/vpp-loss-doctor.ci` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs VPP as a systemd unit and commits the config | YANG `vpp restart` -> commit verify -> unit read -> start or adopt -> Ready | `vpp-systemd-boot` (lab) |
| 2 | Watches VPP hang and come back by itself | probe -> Lost -> Verifying -> `adopted` -> owners reconcile | `test/vpp/vpp-loss-adopt.ci`, `vpp-freeze-adopt` (lab) |
| 3 | Watches VPP hang for good | Lost -> Terminating -> Restarting -> `reconnected` -> owners apply as at boot, no `down` published -> the IGP transports rebind to the new TAPs -> an adjacency whose timers had not expired is kept, and one that was lost by its timer forms again | `test/vpp/vpp-loss-restart.ci`, `vpp-freeze-restart-frr` and `vpp-crash-rebind-frr` (lab) |
| 4 | Has a VPP that cannot be restarted | deadline -> `withdrawn` -> interface events -> IGP, BGP and BFD react -> `ze doctor` | `test/vpp/vpp-loss-withdraw.ci`, `vpp-withdraw-frr` (lab) |
| 5 | Fixes VPP after a withdrawal | Backoff -> Restarting -> `recovered` -> interfaces, adjacencies and routes return | `test/vpp/vpp-loss-return.ci`, `vpp-withdraw-frr` (lab) |
| 6 | Replaced the VPP unit with another program | cgroup check -> `doctor-vpp-foreign-process` -> Ze keeps working | `vpp-systemd-foreign` (lab) |
| 7 | Reads `ze doctor` during any of the above | report bus -> `vpp-recovery` check | `test/vpp/vpp-loss-doctor.ci` |

## 🧪 TDD Test Plan

One question per test. A unit test decides the rule; a `.ci` with the stub
decides that the daemon reaches it; a lab decides what only a real VPP, a
real systemd or a real peer can show.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRestartMethod` | `internal/component/vpp/config_test.go` | The `restart` leaves parse to the method, the unit and the startup file; no leaf means `exec` | |
| `TestExternalLeafIsRefused` | `internal/component/vpp/config_test.go` | A config with `external` is refused with the replacement named (AC-2) | |
| `TestRecoveryLimitsValidate` | `internal/component/vpp/config_test.go` | `alive-time` plus `verify-time` at or above `deadline` is refused (AC-3) | |
| `TestStartupConfRefusesRenderedLeaves` | `internal/component/vpp/config_test.go` | `startup-conf` beside a rendered leaf is refused naming the leaf (AC-28) | |
| `TestLateProbeInsideAliveTimeChangesNothing` | `internal/component/vpp/lifecycle_test.go` | Probes late for less than the alive time leave the state Ready and publish nothing (AC-4) | |
| `TestSilenceForAliveTimeDeclaresLoss` | `internal/component/vpp/lifecycle_test.go` | Silence for the alive time publishes `lost` with cause `alive-test` before `disconnected` (AC-5) | |
| `TestTransportErrorDeclaresLossAtOnce` | `internal/component/vpp/lifecycle_test.go` | A transport error with a live process declares with cause `transport` and no wait (AC-6) | |
| `TestProvenExitSkipsFormOne` | `internal/component/vpp/lifecycle_test.go` | An exit proof declares with cause `process-exit`, sends no signal and goes to Restarting (AC-7) | |
| `TestDeclaredLongReadExtendsAliveTest` | `internal/component/vpp/lifecycle_test.go` | During a declared long read, silence longer than the alive time and shorter than the long-read time declares nothing, and the alive time applies again when the read ends (AC-9) | |
| `TestReplyDuringLongReadRestartsTheClock` | `internal/component/vpp/lifecycle_test.go` | A reply of any kind during a declared read restarts the silence clock | |
| `TestHangDuringLongReadDeclaresLossAtTheBound` | `internal/component/vpp/lifecycle_test.go` | Silence for the long-read time inside a declared read publishes `lost` with cause `alive-test` (AC-30) | |
| `TestUnclosedLongReadExpires` | `internal/component/vpp/lifecycle_test.go` | A read its owner never closes stops extending the alive test after the long-read time | |
| `TestFormOneAdoptsSameProcess` | `internal/component/vpp/lifecycle_test.go` | The same lease, a ping and passing presence checks publish `adopted` and never `reconnected` (AC-8) | |
| `TestFormOneRefusesAnotherProcess` | `internal/component/vpp/lifecycle_test.go` | A `SameProcess` error or another identity fails form (1) | |
| `TestVerifyTimeExpiryKillsThroughPidfd` | `internal/component/vpp/lifecycle_test.go` | At the verify time the retained descriptor is signalled with SIGKILL and the restart waits for the exit proof (AC-10) | |
| `TestForeignProcessIsNeverSignalled` | `internal/component/vpp/lifecycle_test.go` | With a foreign peer no signal and no start are sent at the verify time, and form (1) keeps being tried (AC-17) | |
| `TestForeignExitStartsTheUnit` | `internal/component/vpp/lifecycle_test.go` | The foreign process's exit proof leads to the method's start (AC-18) | |
| `TestDeadlineWithdraws` | `internal/component/vpp/lifecycle_test.go` | Passing the deadline outside Ready publishes `withdrawn` with reason `recovery-deadline` once (AC-19) | |
| `TestCrashLoopWithdrawsBeforeTheThirdRestart` | `internal/component/vpp/lifecycle_test.go` | The count reached inside the window publishes `withdrawn` with reason `crash-loop` before the start (AC-20) | |
| `TestBackoffFollowsTheWindow` | `internal/component/vpp/lifecycle_test.go` | The delay is zero for a first restart, doubles inside the window, caps at 30 s and at 300 s when withdrawn, and a restart older than the window stops counting (AC-22) | |
| `TestRecoveredWaitsForTheLoopToAgeOut` | `internal/component/vpp/lifecycle_test.go` | After a crash-loop withdrawal `recovered` is published only when fewer restarts than the count lie in the window (AC-21) | |
| `TestReplacementDuringRecoveryStartsAsAtBoot` | `internal/component/vpp/lifecycle_test.go` | A manager replaced mid-recovery publishes `disconnected` without `lost` and its successor starts clean (R-6 of Risks) | |
| `TestUnitStateDecidesTheAction` | `internal/component/vpp/unit_restart_test.go` | Each row of the D-6 table: wait, connect, start, reset then start (AC-13, AC-14) | |
| `TestUnitIsReadBeforeEveryAction` | `internal/component/vpp/unit_restart_test.go` | Two losses read the unit twice and act on the second read (AC-15) | |
| `TestRestartCountIncludesUnitRestarts` | `internal/component/vpp/unit_restart_test.go` | An NRestarts advance beyond what Ze saw is counted | |
| `TestParseUnitProperties` | `internal/core/systemdunit/unit_test.go` | The `show` output parses to the properties; an absent property is absent, never zero | |
| `TestProcessUnitFromCgroup` | `internal/core/systemdunit/cgroup_test.go` | The cgroup of a held pidfd names the unit on cgroup v2 and on the hybrid layout, and a process outside it is foreign (A-4) | |
| `TestSignalProbeReportsPermission` | `internal/component/vpp/process_peer_linux_test.go` | Signal 0 through the pidfd reports a refusal as a refusal (A-5) | |
| `TestWithdrawnPublishesAddrRemovedThenDown` | `internal/plugins/iface/vpp/monitor_test.go` | `vpp/withdrawn` publishes, for every interface, `addr-removed` for each address and then `down`, and keeps the names | |
| `TestReconnectedPublishesNoDown` | `internal/plugins/iface/vpp/monitor_test.go` | `vpp/reconnected` outside a withdrawal publishes no `down` and no `addr-removed`, and the `up` that follows carries the new index | |
| `TestBindingCarriesControlDevice` | `internal/component/iface/resolve_test.go` | `Resolve` answers the control device its backend reports, and the device itself when the backend reports none apart (AC-36) | |
| `TestVPPLinkReportsLCPControlDevice` | `internal/plugins/iface/vpp/query_test.go` | The VPP backend reports the TAP name and kernel ifindex of the LCP pair as the control device, and an error, never a zero index, for an interface with no pair (AC-36) | |
| `TestOpenBindsControlDevice` | `internal/plugins/ospf/transport/rfc2328_backend_linux_test.go` | The OSPFv2 open joins and binds by the control device's ifindex when the binding's own index differs (AC-36) | |
| `TestOpenBindsControlDeviceV3` | `internal/plugins/ospf/v3/transport/backend_linux_test.go` | The same for OSPFv3 | |
| `TestOpenCircuitBindsControlDevice` | `internal/plugins/isis/transport/backend_linux_test.go` | The same for an IS-IS circuit | |
| `TestUpUnderNewIndexRebindsOpenInterface` | `internal/plugins/ospf/transport/transport_test.go` | An `up` for an open interface whose control device has another kernel ifindex opens a new handle, closes the old one and never calls `OnInterfaceDown` (AC-32) | |
| `TestUpUnderNewIndexRebindsOpenInterfaceV3` | `internal/plugins/ospf/v3/transport/transport_test.go` | The same for OSPFv3, and the moved callback runs once | |
| `TestUpUnderNewIndexRebindsOpenCircuit` | `internal/plugins/isis/transport/transport_test.go` | The same for an IS-IS circuit: the moved callback carries the old ifindex, the new one and the new raw identity, and `OnCircuitDown` is never called | |
| `TestUpUnderSameIndexChangesNothing` | `internal/plugins/ospf/transport/transport_test.go` | An `up` for an open interface whose control device kept its ifindex does not rebind | |
| `TestRescanRebindsMovedInterface` | `internal/plugins/ospf/transport/transport_test.go` | With no event, the rescan rebinds an open interface whose control device changed | |
| `TestFailedRebindKeepsOldHandle` | `internal/plugins/ospf/transport/transport_test.go` | A rebind whose open fails keeps the old handle, calls no down callback, and succeeds on the next event | |
| `TestRebindRejoinsAllDRouters` | `internal/plugins/ospf/transport/transport_test.go` | A rebind joins AllDRouters on the new handle when the old one had joined it, and not otherwise | |
| `TestRebindKeepsInterfaceID` | `internal/plugins/ospf/gr_preserve_test.go` | After the moved callback the OSPFv3 Interface ID of the interface is the one in use before it, and the IPsec installer ran for the new ifindex (AC-32) | |
| `TestCircuitMovedRekeysBinding` | `internal/plugins/isis/circuits_test.go` | After the moved callback the circuit is found under the new ifindex with the new raw identity, its adjacency table, local circuit ID and SNPA are unchanged, and a frame received on the new ifindex reaches it (AC-32) | |
| `TestRebindRetiresOldStateBeforeClose` | `internal/plugins/ospf/transport/transport_test.go` | During a rebind the old handle is closed only after the new state is registered and the old stop channel closed, and the old reader's end calls no down callback | |
| `TestRebindRetiresOldStateBeforeCloseV3` | `internal/plugins/ospf/v3/transport/transport_test.go` | The same for OSPFv3 | |
| `TestRebindRetiresOldCircuitBeforeClose` | `internal/plugins/isis/transport/transport_test.go` | The same for an IS-IS circuit | |
| `TestControlDeviceRefusesOtherNamespace` | `internal/plugins/iface/vpp/query_test.go` | A pair with a named namespace, or whose device is absent from Ze's namespace under that index, reports no control device and an error naming the namespace | |
| `TestDiscoveryRebindsOnMovedDevice` | `internal/plugins/ldp/register_test.go` | An `up` for a discovery interface whose control device has another ifindex opens a new socket, closes the old one, and leaves the adjacency table untouched (AC-38) | |
| `TestBoundLoopReopensOnRecreatedDevice` | `internal/component/bfd/bfd_test.go` | An `up` for the device a loop is bound to swaps a new transport into the loop and keeps its sessions (AC-39) | |
| `TestAdoptedSweepsLabeledRouteAndPush` | `internal/plugins/fib/vpp/register_test.go` | A labeled route and a push FEC in the retired writer's records that the replay does not re-announce are deleted after the sweep delay, and those it re-announces stay (AC-31) | |
| `TestAdoptedSweepsRouteWithdrawnWhileFenced` | `internal/plugins/fib/vpp/register_test.go` | A route in the retired writer's installed set that the replay does not re-announce is deleted after the sweep delay (AC-31) | |
| `TestAdoptedSweepsLabelWithdrawnWhileFenced` | `internal/plugins/fib/vpp/mplsstale_test.go` | The same for a label owner the replay does not re-claim, with no `MplsRouteDump` sent | |
| `TestRecoveredReplaysUpAndAddresses` | `internal/plugins/iface/vpp/monitor_test.go` | `vpp/recovered` publishes `up` and `addr-added` from VPP's state | |
| `TestAdoptedRenewsInterfaceEvents` | `internal/plugins/iface/vpp/monitor_test.go` | `vpp/adopted` opens a new channel and sends `WantInterfaceEvents` again | |
| `TestAdoptedReconcilesWithoutReload` | `internal/component/iface/config_test.go` | `vpp/adopted` reconciles and does not call `LoadBackend` | |
| `TestAdoptedReplaysWithoutDump` | `internal/plugins/fib/vpp/register_test.go` | `vpp/adopted` replays, takes its mark from the retired writer, and sends neither `ip_route_v2_dump` nor `MplsRouteDump` | |
| `TestMarkDumpIsADeclaredLongRead` | `internal/plugins/fib/vpp/routesource_test.go` | `markStale` opens and closes a long read around its dump | |
| `TestLocalAddressRemovedStopsSession` | `internal/component/bgp/reactor/reactor_iface_test.go` | An `addr-removed` for a session's local address stops that session and no other, and the peer waits for `addr-added` | |
| `TestAddressWaitHoldsUntilAddressReturns` | `internal/component/bgp/reactor/reactor_iface_test.go` | A peer whose configured local address is removed reads `address-wait`, dials nothing and refuses an inbound connection while there, and starts again on `addr-added` for that address (AC-37) | |
| `TestAddressWaitLeavesIdleHoldAlone` | `internal/component/bgp/reactor/reactor_iface_test.go` | A peer in `PeerStateIdleHold` is moved by neither address event (AC-37) | |
| `TestUnboundPeerReconnectsWithoutWaiting` | `internal/component/bgp/reactor/reactor_iface_test.go` | A peer with no configured local address whose session ran from the removed address is closed and does not enter `address-wait` (AC-37) | |
| `TestLocalAddressRemovedTakesSessionDown` | `internal/component/bfd/session_identity_test.go` | An `addr-removed` for a session's local address takes it Down with Path Down, tells its clients, and keeps the periodic transmission scheduled (RFC5880-6.8.6-15): a send that fails at the socket while the address is absent does not stop the timer, and the session is not put in AdminDown | |
| `TestRestartOpenCarriesFBitZero` | `internal/component/bgp/reactor/peer_gr_flags_test.go` | The OPEN after a VPP restart has the F bit 0 (AC-23) | |
| `TestRecoveryDoctorReportsEachState` | `internal/component/vpp/doctor_recovery_test.go` | The check reports Lost, Withdrawn and the return, each with its fix | |
| `TestForeignDoctorNamesProcessUnitAndFix` | `internal/component/vpp/doctor_recovery_test.go` | The foreign report names the PID, the program, the cgroup, the unit and the fix (AC-16) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `recovery alive-time` | 2 to 300 | 300 | 1 | 301 |
| `recovery verify-time` | 1 to 300 | 300 | 0 | 301 |
| `recovery deadline` | 30 to 3600 | 3600 | 29 | 3601 |
| `recovery crash-loop count` | 2 to 20 | 20 | 1 | 21 |
| `recovery crash-loop window` | 60 to 86400 | 86400 | 59 | 86401 |
| `recovery long-read-time` | 10 to 3600, and not below `alive-time` | 3600 | 9 | 3601 |

### Functional Tests
The `.ci` tests run Ze over the stub under `exec` with short limits set in
the config. The stub takes signals: one stops it answering, one closes its
API connections and keeps its state, and it exits when told.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-restart-method-refused` | `test/vpp/vpp-restart-method-refused.ci` | A commit naming a missing unit is refused with the fix (AC-1) | |
| `vpp-late-probe` | `test/vpp/vpp-late-probe.ci` | The stub is silent for less than the alive time: the same stub process serves afterwards and no loss is reported (AC-4) | |
| `vpp-loss-adopt` | `test/vpp/vpp-loss-adopt.ci` | The stub drops its connections: Ze adopts the same process, a route added meanwhile reaches the stub, and the stub was never restarted (AC-6, AC-8) | |
| `vpp-loss-restart` | `test/vpp/vpp-loss-restart.ci` | The stub goes silent for good: Ze kills it, starts a new one, and the interfaces and routes are in the new stub (AC-5, AC-10) | |
| `vpp-loss-exit` | `test/vpp/vpp-loss-exit.ci` | The stub exits: Ze restarts it at once and reconfigures it (AC-7, AC-22) | |
| `vpp-loss-withdraw` | `test/vpp/vpp-loss-withdraw.ci` | The stub cannot start again: after the deadline the connected route over a VPP interface is gone from `show rib` and `ze doctor` reports the withdrawal (AC-19) | |
| `vpp-loss-withdraw-bgp` | `test/vpp/vpp-loss-withdraw-bgp.ci` | A BGP session sourced from a VPP interface address stops at the withdrawal, its peer reads `address-wait`, and a second peer receives the withdrawal of the routes learned over it (AC-19, AC-37) | |
| `vpp-loss-withdraw-bfd` | `test/vpp/vpp-loss-withdraw-bfd.ci` | A BFD session on a VPP interface address is Down with Path Down after the withdrawal (AC-19) | |
| `vpp-crash-loop` | `test/vpp/vpp-crash-loop.ci` | The stub exits three times inside the window: the withdrawal is reported with reason `crash-loop` (AC-20) | |
| `vpp-loss-return` | `test/vpp/vpp-loss-return.ci` | After a withdrawal the stub is allowed to start: the route returns, and the BGP peer leaves `address-wait` and its session establishes, with no command (AC-21, AC-37) | |
| `vpp-loss-doctor` | `test/vpp/vpp-loss-doctor.ci` | `ze doctor` during a recovery names the cause, the state and the next step | |
| `vpp-shutdown` (existing) | `test/vpp/vpp-shutdown.ci` | Ze stops and the stub it started is gone (AC-25, `exec`) | |
| `vpp-event-subscription` | `test/plugin/vpp-event-subscription.ci` | A forked plugin subscribed to `vpp` receives `lost` and `adopted` with their payloads (AC-29) | |

Labs with a real VPP, under `internal/le/test/deployment/`, in a new
`vpplifecycle` suite. Each scenario is named, never numbered.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-freeze-short` | `internal/le/test/deployment/vpplifecycle/` | SIGSTOP shorter than the alive time: nothing changes (AC-4) | |
| `vpp-freeze-adopt` | same | SIGSTOP longer than the alive time and shorter than both limits, then SIGCONT: the same VPP is adopted, a ping through it never stops for longer than the freeze, and no route dump was sent (AC-8, A-3) | |
| `vpp-full-table-dump` | same | Ze restarts beside a VPP holding a full table: the mark dump outlasts the alive time and VPP is not killed (AC-9, A-1) | |
| `vpp-exec-startup-conf` | same | `startup-conf` under `exec` reaches Ready (AC-28) | |
| `vpp-systemd-boot` | same, in a guest with systemd | The unit inactive at boot is started; a running unit is adopted with its InvocationID unchanged; Ze's stop leaves it active (AC-25, AC-26, AC-27) | |
| `vpp-systemd-restart` | same | Restart=no, Restart=always and a unit at its start limit each end in a reconfigured VPP; the setting changed between two losses is followed (AC-13, AC-14, AC-15) | |
| `vpp-systemd-foreign` | same | A VPP started outside the unit is reported, kept, not signalled when frozen, and replaced by the unit when it exits (AC-16, AC-17, AC-18) | |
| `vpp-systemd-rights` | same | Ze as user `ze` without the rights: the commit is refused; with them the kill and the start work (AC-24, A-5) | |

### Interop Tests
The protocol reactions owe a peer's view. Each scenario runs Ze over a real
VPP against FRR, in the `vpplifecycle` suite, and is proven red with the
reaction reverted and the image rebuilt. Before a check reads an FRR output,
the implementer reads that output's producer in FRR at the lab's version.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vpp-igp-adjacency-frr` | `internal/le/test/deployment/vpplifecycle/` | FRR | Before any loss, FRR's OSPFv2, OSPFv3 and IS-IS adjacencies with Ze over VPP interfaces are Full and Up and FRR holds Ze's routes. It is red today if finding 12 holds, and it is the first scenario built. It also runs LDP and a BFD session on those interfaces and requires FRR's LDP neighbor operational and its BFD peer up (AC-36, and the "before" of AC-38 and AC-39) | |
| `vpp-restart-ldp-bfd-frr` | same | FRR | VPP is killed and restarted, once inside 15 s and once past it: FRR's BFD peer goes down at its detection time and comes back up after the restart; FRR's LDP neighbor stays operational in the first run and returns with its label bindings in the second; Ze was not restarted in either (AC-38, AC-39) | |
| `vpp-freeze-restart-frr` | same | FRR | VPP hangs for good and is killed at 30 s, past FRR's timers: FRR loses the adjacencies by its own timers only, Ze publishes no `down`, and after the restart the OSPFv2, OSPFv3 and IS-IS adjacencies form again over the new TAPs and FRR holds Ze's routes; FRR's BGP session re-opens and Ze's OPEN has the F bit 0 (AC-10, AC-11, AC-23) | |
| `vpp-crash-rebind-frr` | same | FRR | VPP is killed from outside and restarted inside FRR's timers: FRR logs no OSPFv2, OSPFv3 or IS-IS adjacency change, keeps Ze's routes throughout, and receives Ze's Hellos with the Interface ID and circuit ID it saw before (AC-32) | |
| `vpp-freeze-timers-frr` | same | FRR | With VPP frozen below the deadline, FRR sees BFD go down at the detection time and BGP at the hold time (AC-12) | |
| `vpp-withdraw-frr` | same | FRR | With VPP unable to start, a FRR peer reached outside VPP receives the withdrawal of the routes Ze redistributed from VPP interfaces and of those learned over VPP sessions; when VPP is allowed to start they return (AC-19, AC-21) | |

### Goal evidence planned

One row per goal of the Task, naming the evidence the closure's Goal
Validation table will cite. A passing unit test alone answers none of them.

| Goal | Evidence |
|------|----------|
| Ze always holds a restart method (R-1, R-2) | `vpp-restart-method-refused` (the refusal), `vpp-systemd-boot` (a unit started and adopted), `vpp-loss-restart` (the `exec` kill and restart through the daemon) |
| A probe missed inside the alive time is not a loss; a failed alive test is (R-4) | `vpp-late-probe` and `vpp-freeze-short` against `vpp-loss-restart` and `vpp-freeze-adopt`, with the declaration time read from the event log |
| The same process is adopted with no kill and no withdrawal (form 1) | `vpp-freeze-adopt`: the VPP PID is unchanged, a ping through VPP never stops for longer than the freeze, and the request log holds no route dump |
| A lost VPP is killed, restarted and reconfigured with nothing withdrawn (form 2) | `vpp-freeze-restart-frr`: Ze's event log holds no `interface/down` and no `addr-removed`, FRR's log shows no loss other than its own timers', and its adjacencies and routes are back. `vpp-crash-rebind-frr`: on a restart shorter than the timers FRR's log shows no adjacency change at all. Both rest on `vpp-igp-adjacency-frr`, which proves an IGP runs over a VPP interface in the first place |
| Ze's protocol timers run during recovery (A-5) | `vpp-freeze-timers-frr`: FRR records the BFD and BGP loss at the configured times |
| Past a limit the peers are updated, and everything returns by itself (R-10, R-11) | `vpp-withdraw-frr`: a FRR peer outside VPP receives the withdrawals, then the routes again, with no command typed on Ze |
| The unit's settings are detected and followed (R-9) | `vpp-systemd-restart`, over three Restart= settings and a change between two losses |
| A foreign process is reported and never refused or killed (R-12) | `vpp-systemd-foreign`: the process's PID survives a declared loss, and `ze doctor` prints the report of D-10 |
| The operator is told what is wrong and how to fix it | `vpp-loss-doctor` and the doctor unit tests, asserting the fix line of each report |

## Files to Modify

- `internal/component/vpp/yang/ze-vpp-conf.yang` - `external` removed; `restart` and `recovery` containers; the container description, which still says a later commit does not reach VPP
- `internal/component/vpp/config.go` - the settings and their validation
- `internal/component/vpp/vpp.go` - the lifecycle state machine replaces the loop of `runWithOps` and the first-probe kill
- `internal/component/vpp/conn_health.go` - the alive test over `alive-time`, declared long reads bounded by `long-read-time`, the three loss causes
- `internal/component/vpp/startup_readiness.go` - `lost` before `disconnected` on a loss
- `internal/component/vpp/runtime_stats.go` - `adopted`, `reconnected` and `recovered` at readiness
- `internal/component/vpp/process_peer.go` - the manager's retained lease; the signal on the descriptor
- `internal/component/vpp/process_peer_linux.go` - the pidfd signal and the signal-0 probe
- `internal/component/vpp/process_exit.go` - presence checks registered beside the exit observers
- `internal/component/vpp/managed_startup.go` - the method in place of `External`; the operator's startup file
- `internal/component/vpp/startup_cli.go` - the method in place of `External`
- `internal/component/vpp/register.go` - the new event types; the live commit verify of the unit and the rights; the program path from the environment
- `internal/component/vpp/register_doctor_codes.go` - the three new codes
- `internal/core/vpp/events/events.go` - the four new event names
- `internal/plugins/systemd/doctor.go` - uses the moved unit reader and timespan parser
- `internal/plugins/systemd/unit.go` - the rights of C-1, as he decides
- `internal/component/iface/register.go` - the `adopted` subscription
- `internal/component/iface/config_apply.go` - the reconcile without reload
- `internal/plugins/iface/vpp/monitor.go` - the interface events of D-8
- `internal/plugins/iface/vpp/ifacevpp.go` - the channel and event registration renewed on `adopted`; the presence check
- `internal/plugins/fib/vpp/register.go` - on `adopted`, replay and sweep with the mark from the retired writer
- `internal/plugins/fib/vpp/routesource.go` - the mark dump as a declared long read; the mark from a retired writer's record on an adoption
- `internal/plugins/fib/vpp/fibvpp.go` - the retired writer hands its `installed` set and label owners to the adopting writer
- `internal/plugins/fib/vpp/mplsstale.go` - the label mark from the retired writer on an adoption
- `internal/plugins/ospf/transport/transport.go`, `internal/plugins/ospf/v3/transport/transport.go`, `internal/plugins/isis/transport/transport.go` - the rebind: an `up` or `appeared` for an open interface, and the rescan, move it to a changed kernel device with no down callback; OSPF remembers AllDRouters membership; OSPFv3 and IS-IS gain the moved callback (C-8, D-8)
- `internal/plugins/ospf/transport/backend_linux.go`, `internal/plugins/ospf/v3/transport/backend_linux.go`, `internal/plugins/isis/transport/backend_linux.go` - open on the binding's control device (finding 12)
- `internal/component/iface/resolve.go` - `bindingFromInfo` fills the control device; the `Binding` and `InterfaceInfo` value types gain it, in the file that declares them
- `internal/plugins/iface/vpp/query.go` - the VPP backend reports the LCP pair's TAP as the control device
- `internal/plugins/ospf/instance.go`, `internal/plugins/ospf/gr_preserve.go`, `internal/plugins/ospf/ipsec_install.go` - the OSPFv3 moved callback: the Interface ID preserved, the IPsec installed again
- `internal/plugins/isis/circuits.go` - the moved callback: the circuit binding re-keyed to the new ifindex and raw identity
- `internal/plugins/ldp/register.go` - discovery opens on the control device, keeps its subscription and rebinds its socket (`waitForInterface`, `resolveDiscoveryInterface`, `discoverOnInterface`)
- `internal/component/bfd/bfd.go` - a bound loop gets a new transport when its device is recreated (`loopFor`, `loopDevices`); the loop's transport swap in `internal/component/bfd/engine/`; in phase 6, with the address reaction
- `internal/plugins/fib/vpp/mplsentry.go` - `labelPushes` is one of the four records handed to the adopting writer
- `docs/features/interfaces.md`, `docs/architecture/core-design.md`, `docs/architecture/plugin/plugin-relationships.md`, `docs/architecture/iface/management.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/mpls/mpls-kernel.md` - the sections their Design documents rows name
- `docs/architecture/ospf/ospf-3-ip-transport.md`, `docs/architecture/ospf/ospfv3-3-ipv6-transport.md`, `docs/architecture/isis/isis-3-l2-transport.md`, `docs/architecture/ospf/ospf-4-component-config.md`, `docs/architecture/wire/ospf.md`, `docs/architecture/wire/ospfv3.md`, `docs/architecture/isis/isis-5-adjacency.md`, `docs/architecture/iface/logical-name-resolution.md`, `docs/architecture/ospf/ospf-ext-9-graceful-restart.md`, `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md`, `docs/architecture/isis/isis-4-component-config.md`, `docs/architecture/ldp/mpls-ldp.md` - the control device and the rebind, in phase 3
- `docs/architecture/config/transaction-protocol.md`, `docs/architecture/config/apply-ordering.md`, `docs/architecture/static-routes.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the sections their Design documents rows name
- `internal/plugins/firewall/vpp/backend_linux.go` - apply again on `adopted` and `reconnected`
- `internal/plugins/traffic/vpp/backend_linux.go` - the same
- `internal/plugins/static/backend_vpp_linux.go` - the same; its channel is opened once and dies with the connection
- `internal/component/ike/dataplane/vpp.go` - the same; its channel is opened once too
- `internal/component/bgp/reactor/reactor_iface.go` - the local-address reaction
- `internal/component/bfd/session_identity.go` - the local-address reaction, in the package that holds the session identity
- `internal/test/cli/cmd_vpp_stub.go` - started by Ze with a startup file; the control signals
- `internal/test/fixture/misc_fixture_vpp.go`, `misc_fixture_vpp_shutdown.go`, `plugin_fixture_16_vpp.go`, `register_vpp_mpls_ldp_pop.go` - the converted config; the fixture no longer starts the stub
- `internal/le/test/deployment/vppevidencescenarios.go`, `vppifacescenarios.go`, `vpp_srv6_managed_integration_test.go`, `vpppppoe/product_plan.go`, `vpppppoe/product_state_test.go` - the converted config
- `internal/le/site/testdata/published-configuration.md`, `published-yang-config-tree.json` - regenerated
- `docs/research/vpp-deployment-reference.md`, `docs/guide/vpp.md`, `ai/digests/vpp-dataplane.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/bgp/interface-event-reactions.md`, `docs/architecture/bfd.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/fib/fib-vpp-route-ownership.md`, `docs/architecture/fib/fib-depth-4-srv6.md`, `docs/features/srv6.md`, `docs/architecture/traffic/fw-7-traffic-vpp.md`, `docs/architecture/traffic/followup-vpp-traffic.md`, `docs/architecture/testing/interop.md`, `docs/architecture/l2tp/bng-5-pppoe.md`, `docs/guide/configuration.md`, `docs/guide/graceful-restart.md` - each in the phase that makes it wrong

## Files to Create
- `internal/component/vpp/lifecycle.go` - the states, the limits, the restart list
- `internal/component/vpp/unit_restart.go` - the unit decisions of D-6 and the foreign check
- `internal/component/vpp/doctor_recovery.go` - the three checks
- `internal/core/systemdunit/unit.go` - the unit reader, the timespan parser, start and reset through `systemctl`
- `internal/core/systemdunit/cgroup.go` - the unit of a held pidfd
- `internal/le/test/deployment/vpplifecycle/` - the lab suite
- `test/vpp/vpp-restart-method-refused.ci`, `vpp-late-probe.ci`, `vpp-loss-adopt.ci`, `vpp-loss-restart.ci`, `vpp-loss-exit.ci`, `vpp-loss-withdraw.ci`, `vpp-loss-withdraw-bgp.ci`, `vpp-loss-withdraw-bfd.ci`, `vpp-crash-loop.ci`, `vpp-loss-return.ci`, `vpp-loss-doctor.ci`
- `test/plugin/vpp-event-subscription.ci`
- `features/vpp-failure-lifecycle.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/vpp/yang/ze-vpp-conf.yang`: `restart`, `recovery`; `external` removed |
| YANG validation constraints | Yes | The enumeration, the ranges of the boundary table, the absolute-path pattern `startup-cli-file` already uses, a `must` tying `unit` to `systemd` and `startup-conf` to `exec` |
| YANG custom validators | Yes | The sum rule of AC-3 goes in `VPPSettings.Validate`, which `verifyVPPConfig` runs offline and at commit. The unit and rights checks need the host, so they run in the live commit verify only, never in `ze config validate` |
| CLI commands/flags | No | No command is added: the state is read through `ze doctor` and `show warnings` |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | Yes | Automatic for the enumeration |
| Functional test for new RPC/API | Yes | `test/plugin/vpp-event-subscription.ci` for the event contract |
| Pipe completeness | N-A | No command output |
| Env var registration | Yes | `ze.vpp.binary`, registered with `env.MustRegister` beside its reader in `internal/component/vpp/register.go`, in place of the constant `defaultVPPBinary`. It is env-only because it selects the program for tests and for the embedded runtime, not operator policy; `plan/release/delivery/spec-vpp-embedded-runtime.md` owns where the program comes from and this spec adds no second source |
| Doctor check for runtime dependencies | Yes | The systemd unit, `systemctl`, the kill right and the start right: `vpp-restart-method` in `internal/component/vpp/doctor_recovery.go`, registered in `register_doctor_codes.go`, with a unit test and `vpp-systemd-rights` |
| Doctor code per problem class | Yes | `doctor-vpp-recovery` (Runtime: loss, withdrawal), `doctor-vpp-restart-method` (PostConfig: missing unit, missing right, coupling, RestartUSec), `doctor-vpp-foreign-process` (Runtime). Every commit refusal of AC-1, AC-2, AC-3, AC-24 and AC-28 is an operator error with its fix on the last line |
| Prometheus counters/metrics | Yes | In the `vpp` component's metrics: losses by cause, restarts by method and by who started them, adoptions, withdrawals by reason, and a state gauge. The names follow the component's existing metrics and are listed in its telemetry page |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/vpp-failure-lifecycle.md`, its level read from `./le feature report` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/guide/vpp.md` |
| 3 | CLI command added/changed? | No | No command |
| 4 | API/RPC added/changed? | No | No RPC; the events are row 8 |
| 5 | Plugin added/changed? | No | No plugin is added |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md`: the two methods, the limits, what a loss looks like, the foreign report |
| 7 | Wire format changed? | No | No protocol wire changes |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`: the missing `vpp` namespace section, with the seven events |
| 9 | RFC behavior implemented, changed, or newly proven? | No | The BFD diagnostic and the BGP session stop are local behavior no requirement row of Ze's summaries changes. `rfc/short/rfc5880.md` was read at design at its requirement rows (its Design documents row): the BFD reaction keeps RFC5880-6.8.6-15 and uses no AdminDown. Phase 6 re-checks `rfc/short/rfc4271.md` before the BGP reaction lands |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (the stub under `exec` and its signals), `docs/architecture/testing/interop.md` (the `vpplifecycle` suite) |
| 11 | Affects daemon comparison? | No | No compared capability changes |
| 12 | Internal architecture changed? | Yes | `docs/research/vpp-deployment-reference.md` (lifecycle, socket transport), `ai/digests/vpp-dataplane.md`. In phase 3, with the transports: `docs/architecture/ospf/ospf-3-ip-transport.md`, `docs/architecture/ospf/ospfv3-3-ipv6-transport.md` and `docs/architecture/isis/isis-3-l2-transport.md` (the rebind and the control device), `docs/architecture/ospf/ospf-4-component-config.md` (the callbacks no longer follow the socket one for one), `docs/architecture/isis/isis-5-adjacency.md` (the moved callback), `docs/architecture/wire/ospf.md` and `docs/architecture/wire/ospfv3.md` (what the transport binds), `docs/architecture/iface/logical-name-resolution.md` (the binding's control device), `docs/architecture/iface/management.md` (the rebind beside static's rule), `ai/digests/ospf.md` and `ai/digests/isis.md`, `docs/architecture/ldp/mpls-ldp.md` (discovery opens on the control device, keeps its subscription and rebinds its socket). In phase 6: `docs/architecture/config/transaction-protocol.md` (the consumers of the address events), `docs/architecture/bfd.md` (the local-address reaction with its diagnostic, and the reopen of a bound loop when its device is recreated). `docs/research/bfd-implementation-guide.md` is not changed: its Design documents row agrees. In phase 3: `docs/architecture/config/apply-ordering.md` (the two new emission moments) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | The telemetry section of `docs/guide/vpp.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Four event types: `docs/architecture/api/process-protocol.md`; three doctor checks: `docs/architecture/doctor-and-health-checks.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors`: see the note under this table |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every page of the census row "Pages" shows `external` and is corrected |

Row 16, derived on 2026-11-23: `./le spec citation anchors` lists 36 pages
that mention the changed files. Every design page among them is a row of
`## Design documents` with its verdict, and a `silent` row there is the
statement that the page is unaffected, with the reason. The command was run
again once those rows existed and its whole output read: six guide pages
remain, each named here.

| Page | Mentioned by | Disposition |
|------|--------------|-------------|
| `docs/guide/operations.md` | `systemd/unit.go` | Affected only if C-1 (a) is chosen: it then gains the CAP_KILL grant and the polkit rule beside whatever it says of the unit. Phase 4 |
| `docs/guide/ze-install.md` | `systemd/doctor.go`, `systemd/unit.go` | The same: `ze install systemd` writes the rights of C-1. The doctor check's output is unchanged. Phase 4 |
| `docs/guide/plugins.md` | `systemd/unit.go`, `iface/register.go` | Unaffected: no plugin is added or renamed, and the `iface` change is one more event subscription |
| `docs/guide/pppoe.md` | `iface/register.go`, `config_apply.go`, `traffic/vpp/backend_linux.go` | Updated where it shows `vpp { external true; }` for a PPPoE deployment, in phase 1; its PPPoE behavior over a loss is the PPPoE spec's |
| `docs/guide/redistribution.md` | `iface/register.go` | Unaffected: redistribution reads the system RIB, which a withdrawal changes through the existing address events |
| `docs/guide/traffic-control.md` | `traffic/vpp/backend_linux.go` | Gains one sentence in phase 5: the VPP backend applies again after a VPP restart or adoption |

## Design presentation

### What changes

Ze manages every VPP it uses. The `external` mode is gone: the operator
chooses how Ze restarts VPP, by starting the program itself (`exec`, the
default) or by driving a systemd unit. When VPP stops answering, Ze waits 10
seconds before it calls it a loss, then tries for 20 seconds to get the same
process back. If that works, nothing was killed and nothing was withdrawn. If
it does not, Ze kills VPP, restarts it and configures it again as at boot,
and still withdraws nothing: traffic is dropped meanwhile, and Ze's own
protocol timers decide what the peers see. Only when VPP is not back 120
seconds after the loss, or needs a third restart inside 10 minutes, does Ze
take the VPP-backed interfaces and their addresses down in its own model, so
OSPF, IS-IS, BGP and BFD react. It then keeps trying, and everything returns
by itself when VPP does. Each of these times is a leaf under `vpp recovery`,
with a sixth, `long-read-time` (60 seconds), for how long one of Ze's own
route dumps may keep VPP silent before that too is a loss. `ze doctor` says
where this stands at every step. A
VPP that is not the configured unit's process is reported and never killed.

One defect found while reading for this design is fixed with it, because the
rest depends on it (finding 12): over a VPP interface, OSPF and IS-IS open
their kernel sockets by VPP's interface index, not by the kernel index of the
interface's TAP. As read, and not yet run, no IGP adjacency over VPP can be
relied on today. The interface resolver will answer the kernel device, and
the first lab scenario built proves an adjacency over VPP before any loss is
tested.

### Before and after

1. The config. Before: `vpp { enabled true; external true; api-socket
   /run/vpp/api.sock; }`, and Ze never touches the process. After: `vpp {
   enabled true; restart { method systemd; unit vpp.service; } }`. A config
   that still says `external` is refused: "vpp external is no longer
   accepted: Ze manages every VPP it uses. Set 'vpp restart method systemd'
   and 'vpp restart unit <name>' for a VPP that systemd runs, or remove the
   leaf to let Ze start VPP itself."
2. VPP busy for 6 seconds. Before: the 5 s probe timeout expires, the log
   says "vpp: process exited unexpectedly", and in managed mode VPP is killed
   and every session over it drops. After: nothing happens and nothing is
   logged above debug.
3. VPP's API drops for 15 seconds and the process is fine. Before: managed
   mode kills it; external mode reconnects and replays, with a full route
   dump that stops forwarding while it runs. After: `ze doctor` shows "VPP
   stopped answering 12 s ago (no reply for 10 s). Ze is reconnecting to the
   same process and will restart VPP in 18 s if it does not answer. Nothing
   is withdrawn." Then the report clears; no session dropped and no dump ran.
4. VPP hangs for good. Before, external mode: Ze reconnects for ever, every
   interface stays up in Ze, and the peers find out by their own timers or
   not at all. After: at 30 s Ze kills VPP and restarts it; `ze doctor`
   shows "VPP was restarted (no reply for 30 s). Interfaces and routes are
   being applied again. Deadline: 87 s." No interface goes down in Ze. By
   then the peers' own timers have dropped the OSPF and IS-IS adjacencies
   (30 s is the IS-IS default hold), and they form again over the new
   interfaces once VPP is back. When VPP crashes instead and is back inside
   those timers, the adjacencies are kept: OSPF and IS-IS move their sockets
   to the new interfaces and the peers see no change.
5. VPP cannot start. Before: Ze retries every 30 s for ever and advertises
   routes through a dead data plane. After, at 120 s: "VPP is not back 120 s
   after it was lost, so Ze took the 4 VPP interfaces down: OSPF, IS-IS, BGP
   and BFD sessions over them are closed and their routes withdrawn. Ze
   retries in 32 s (attempt 6). Last start failed: vpp.service result
   start-limit-hit. Fix: run 'journalctl -u vpp.service' for VPP's error; Ze
   brings everything back by itself once VPP starts." When VPP starts, the
   interfaces and routes return with no command.
6. VPP crashes three times in ten minutes. Before: three outages, three
   flaps, and a fourth on the way. After: the third crash withdraws at once
   with "VPP needed 3 restarts in 10 minutes", and the interfaces return once
   the loop has aged out.
7. Someone runs another VPP on the socket. Before: Ze uses it silently.
   After: Ze uses it and `ze doctor` shows "The process on /run/vpp/api.sock
   (PID 4312, /usr/local/bin/vpp, cgroup /user.slice/...) is not part of
   vpp.service. Ze works with it but will not restart it, and the
   configuration may be out of date. Fix: set 'vpp restart unit' to the unit
   that runs this VPP, or stop this process so Ze starts vpp.service."
8. Ze stops. Before and after under `exec`: VPP stops with it. Under
   `systemd`: the unit keeps running and forwarding, and the next Ze adopts
   it.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| One limit: kill at the declaration | His answer A-1 chose two |
| Keep `external` as a third, unmanaged method | R-1 refuses it |
| Read the unit over D-Bus with `GetUnitByPIDFD` | It does not exist before systemd 253 (S-3), so the cgroup read would be needed anyway; one route that works everywhere replaces two, and it extends the `systemctl` use Ze already has |
| Kill through systemd (`KillUnit`) | The decision rows say the kill goes through the retained pidfd; a unit-wide kill names no process Ze proved |
| A reconciler registry in the `vpp` component | The owners already subscribe to `vpp` events for exactly this; a registry beside the bus would be a second mechanism |
| Mark and sweep the FIB after form (1), as on a reconnect today | The mark is a full route dump, which stops forwarding for its run (V-5): C-5 |
| Hold form (1) until every owner has read its state back | A-4 forbids it |
| A proof of forwarding from the stats segment | A-4 chose identity and the presence of data; the counters Ze could read do not prove it either (V-8) |
| BGP and BFD react to `interface/down` by interface name | BGP may not import `iface` to turn a Ze interface name into the address it is bound to, and on the kernel backend a link-down reaction is a larger change than this work needs. The address is what both bind to, and one event then serves both |
| BGP reacts through the kernel address watch it already has | The kernel removes a TAP's addresses when the process dies, which is the loss, not the withdrawal (R-6), and keeps them under a hung foreign VPP |
| An event for the kill | The exit proof and `reconnected` already say it |
| A separate stable time before the return, and a separate backoff reset | The sliding window gives both from one list |
| Constants instead of leaves for the six values | V-11: nothing is measured, so a site must be able to tune them (C-2) |

### Documents departed from

`docs/architecture/bgp/interface-event-reactions.md`, section "The
decisions". The page says: "Watch the kernel address table, not the
interface bus topics", and of a removed address: "A session already running
on that address is not torn down by this path." Before: BGP hears addresses
only from the kernel, and a session whose local address goes away lives
until its hold timer. After: the listeners still follow the kernel table,
unchanged, and the reactor also subscribes to `interface/addr-removed` and
`interface/addr-added` on the EventBus: a session whose local address leaves
Ze's model stops at once and waits for the address to return. The kernel
table cannot carry this, because Ze's model and the kernel differ exactly
during a VPP recovery. On the kernel backend the visible change is that
removing an address from an interface ends the BGP sessions sourced from it
at once instead of at the hold time. After this work the page states both
subscriptions and what each is for. This is C-9.

`docs/research/vpp-deployment-reference.md`, "Design decisions". Three rows
are departed from only because his own later rows supersede them: "One
timer" (the design has two limits), "As soon as a loss is declared" (the kill
follows the second limit, and a unit may restart VPP itself) and "Recovered
has exactly two forms" (form (1) needs no full readback). Removing them waits
for his `decision-change`. One row is departed from on substance: "Two
limits" forbids "a reconnect attempt with no bound", and for a process Ze may
not kill (foreign, or one it lacks the right to signal) the design keeps
reconnecting after the second limit until the process answers or exits.
Before: Ze reconnects to any external VPP for ever. After: the same for a
foreign process only, reported throughout, with the withdrawal at the
deadline; every other process is killed at the limit (C-17). The rows on
form (1) and on "no withdrawal" are kept by the recommended options: the
adoption replays and sweeps with a mark from memory, and a restart publishes
no `down`. Their alternatives, C-5 (b) and C-8 (b), would each depart, and
each row says what it would cost.

`docs/architecture/ospf/ospf-4-component-config.md`, "Constraints on
callers". The page says: "The transport exposes link-up and link-down
callbacks because engine running state tracks raw socket lifecycle across
carrier flaps and config reloads." Before: every socket an OSPF interface
opens or closes reaches the engine as an up or a down, and a down ends the
adjacencies. After, under C-8 (a): when a VPP restart replaces the kernel
device, the transport replaces the socket of a running interface with no
down, OSPFv2 hears nothing, and OSPFv3 hears a third, moved callback. The
adjacency survives a restart shorter than its timers, and the page says that
the engine's state follows the interface, not each socket. Under C-8 (b) the
page stands and every restart ends every adjacency over VPP.

`docs/architecture/config/transaction-protocol.md`, "Side-Effect Events
(produced during apply)". The page says of `addr-added`: "BGP reads the
kernel address table itself". After this work BGP and BFD are also consumers
of both address events, for the session reaction only: the same departure as
the next page, settled by C-9.

`docs/features/interfaces.md`, "Bus Topics". The page says: "BGP does not
read `interface/addr-added` or `interface/addr-removed` either". After this
work it reads both, for the session reaction only (C-9), and the topic table
says that under the VPP backend `down` and `addr-removed` are also published
at a withdrawal. `docs/architecture/core-design.md`, section 14, already says
the opposite of that page ("draining sessions when addresses disappear"): the
two pages disagree today, and C-9 settles which one is corrected.

`docs/architecture/traffic/fw-7-traffic-vpp.md`, "Tolerant reconcile after a
VPP restart". The page says: "No reconnect subscription is needed." Before:
after a VPP restart the policers and classify sessions are absent until the
next commit runs an Apply. After: the backend subscribes to `adopted` and
`reconnected` and applies at once, and the page says so. The same holds for
the firewall, static and IKE backends, whose pages say nothing of it.

`docs/architecture/l2tp/bng-5-pppoe.md`, "Design decisions". The row "Adoption
happens only inside the single VPP loss timer" is superseded by his later
row. The row on D-1 says the exit proof "always arrives": for a foreign
process, which R-12 forbids Ze to kill, it arrives only when that process
exits by itself (C-15).

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | The rights Ze needs under `systemd`. Fact: `ze.service` runs as user `ze` with four capabilities and no CAP_KILL (`buildUnitFile`, `privilege.DaemonCapabilities`), and a non-root start of a system unit needs polkit. Under the shipped unit Ze can neither kill nor start a root VPP, so R-1 and R-2 are unreachable there today | (a) recommended: `ze install systemd` grants CAP_KILL and writes a polkit rule that lets user `ze` start and reset only the configured unit; the commit is refused without the rights, and a missing right found later is reported, not fatal. Cost: Ze may signal any process on the host. (b) the `systemd` method requires Ze to run as root. (c) a root helper that kills and starts only the unit: no broad right, one more program to ship | |
| C-2 | The values, and leaves or constants. Five here; the sixth, `long-read-time`, is C-16 | (a) recommended: leaves with defaults 10 s (`alive-time`), 20 s (`verify-time`), 120 s (`deadline`), 3 restarts in 600 s (`crash-loop`), and `long-read-time` 60 s as C-16 decides (D-2 gives each cost). (b) constants with the same values. Fact: V-11, nothing is measured; today's values are a 1 s tick and one 5 s timeout | |
| C-3 | An inactive unit at boot | (a) recommended: Ze starts it. (b) Ze waits for it and reports | |
| C-4 | Ze stopping and starting beside a unit VPP | (a) recommended: the unit keeps running when Ze stops, and a starting Ze adopts a running unit. Cost: the `fib-vpp` mark dump stops forwarding for its run when the VPP kept a full table (V-5); this is today's behavior on any Ze restart. (b) Ze stops the unit when it stops and restarts it when it starts: clean state, an outage on every Ze restart | |
| C-5 | A conflict with A-4. He said reconciliation "may be then happening in the background", in the `fib-vpp` replay-and-sweep style. Fact: the sweep's mark is `ip_route_v2_dump`, which is not thread safe: it holds the worker barrier and stops forwarding for its whole run, and a ping behind it waits (V-5). At a full table a form (1) adoption would itself stall forwarding and could fail the alive test | (a) recommended: replay and sweep as he said, with the mark taken from the retired writer's own four records of what it installed (`installed`, `mplsInstalled`, `labelOwners`, `labelPushes`: IP routes, labeled routes, label entries and labeled pushes) instead of from a dump; SRv6 steering restores from its durable records as on any reconnect. A route withdrawn while fenced is then swept, and no dump stops forwarding. Costs, stated: one leftover is invisible, an add whose reply was lost at the loss instant and whose route was withdrawn before the adoption, which stays in VPP until the next mark by dump; and each label of the replay takes the barrier for its one message, because no MPLS message is thread safe in VPP. (b) the mark by dump as today: sees every leftover, and stops forwarding for a time proportional to the table on every adoption (`ip_route_v2_dump` and `MplsRouteDump`). (c) replay with no mark and no sweep: a route withdrawn while fenced forwards in VPP until the next Ze or VPP restart | |
| C-6 | What ends the recovery deadline. P-1 as adopted says "the FIB replay complete". Fact: the replay has no end marker today, which is why `fib-vpp` sweeps on a 30 s timer (`labelSweepDelay`) | (a) recommended: readiness, the interfaces and LCP pairs applied, and the replay requested. (b) add an end-of-replay marker to the system RIB replay and wait for it: a full-table replay then counts against the 120 s | |
| C-7 | A foreign process past the deadline. Q-7 says "not cause a routing outage"; R-10 says Ze withdraws past the deadline. With a foreign VPP that forwards but whose API stays dead, the two pull apart | (a) recommended: the deadline applies; Ze withdraws, because it cannot program a data plane it cannot reach. (b) never withdraw while the foreign process is alive; Ze keeps advertising over a FIB it can no longer update | |
| C-8 | The cost of form (2) to the IGPs. Facts, read at design (A-10): the restart recreates the TAPs with new kernel ifindexes; the OSPF and IS-IS transports ignore an `up` for an interface they hold open and never notice the old TAP going, so today each moves only on `down` then `up` (findings 10 and 12). A `down` is a withdrawal, which his row "The VPP loss itself triggers no withdrawal" forbids. Owed under either option, because it is a defect and not a choice: the transports take VPP's index for a kernel ifindex, so the binding must carry the kernel device before any IGP adjacency over VPP can be relied on (finding 12, AC-36) | (a) recommended: no `down`. The OSPFv2, OSPFv3 and IS-IS transports rebind on the `up`: the socket moves to the new TAP and the adjacency is kept (D-8's rebind table). Cost, larger than first written: a rebind in three transports; OSPFv2 needs nothing in its engine; OSPFv3 needs its Interface ID preserved and its IPsec installed again; IS-IS needs a moved callback and its circuits re-keyed by ifindex; and the pages of `ospf-4-component-config.md` and the three transports change. What it buys is narrow on the default timers: an adjacency is kept only when the outage is shorter than the dead or hold time on both sides, so after a crash with a fast restart and not after a hang, which is killed at 30 s. In every other case the adjacency is lost by its own timer, as A-5 says, and forms again with no `down` from Ze. Whether IS-IS and OSPFv3 keep an adjacency at all turned on the TAP's MAC (A-13): read in VPP at the pin, linux-cp copies it from the VPP interface, so on a hardware NIC it does not change and both keep it; a loopback's MAC on a new process is still to be read. LDP gets the same rebind (15 s hold time), and BFD cannot keep a session and gets a reopen instead (C-20). (b) publish `down` then `up`, as review I-7 recorded. It departs from his row, and it costs, on every successful restart: static withdraws each route whose next hops are all down and reinstalls it; RSVP-TE sends PathErr and removes its LSPs, and hears no `up`, so they stay down until re-signalled; OSPF and IS-IS drop every adjacency over VPP and the routes learned over it, then re-form; and a subscriber that is behind can lose the `down` and keep a dead socket, because the resolver keeps only the newest event | |
| C-9 | BGP and BFD reacting to their local address leaving Ze's model, on every backend. This departs from `interface-event-reactions.md` (above). FRR and BIRD stop only directly connected sessions, on link down; Ze would stop any session whose local address is removed, which also covers a session sourced from a VPP loopback | (a) recommended: as designed in D-8. (b) the reaction applies only to addresses a VPP withdrawal removed, by a `cause` field added to the event: the kernel backend keeps today's behavior, the event format changes | |
| C-10 | BGP routes configured in Ze and advertised over a session that does not ride VPP (a route reflector reached out of band) | (a) recommended for this spec: they stay advertised; what leaves is what the system RIB loses and what was learned over VPP sessions. `ze doctor` says so in the withdrawal report. (b) a new BGP reaction that withdraws configured routes whose next hop is a withdrawn address: a feature of its own | |
| C-11 | `exec` with an operator's startup.conf | (a) recommended: the `restart startup-conf` leaf, refusing the leaves Ze could not apply. The labs that write their own file need it to leave `external`. (b) no leaf: every lab and operator expresses its VPP in Ze's leaves | |
| C-12 | The sidecar container | (a) recommended: removed from the pages; no method reaches a VPP in another container, and nothing builds the image the guide names. (b) keep it as a future method of its own | |
| C-13 | The stub's method (choice 7) | (a) recommended: `exec`, Ze starts the stub from the path `ze.vpp.binary` names, so the `.ci` suite runs the real kill and restart. (b) a systemd unit: no systemd exists in the functional runner | |
| C-14 | The event names (choice 8) | (a) recommended: `lost`, `adopted`, `withdrawn`, `recovered`, beside the three unchanged; no event for the kill or the fence. (b) other names, or a payload on the existing three | |
| C-15 | PPPoE over a hung foreign VPP. The PPPoE page forbids "holding PPPoE service fenced until an external VPP happens to exit", and R-12 forbids killing a foreign process: with a foreign VPP that never answers again and never exits, both cannot hold | (a) recommended: R-12 wins. PPPoE stays fenced for that process, `ze doctor` says so with the fix (stop the foreign process, or name its unit), and the PPPoE spec records the exception. (b) a foreign process may be killed once the deadline has passed, which narrows R-12 | |
| C-16 | How long Ze's own dump may keep VPP silent before it is a loss. Fact: the route dump holds the barrier and VPP sends nothing until the walk is done (V-5), nothing bounds a hung main thread (V-4), and no dump time is published or measured (V-11). Forwarding is stopped for the whole dump, so a dump longer than the peers' timers is already an outage | (a) recommended: a sixth leaf, `recovery long-read-time`, default 60 s: above the IS-IS hold and the OSPF dead interval, so by then the peers have left and a kill costs nothing more. Lower kills a healthy VPP mid-dump on a large table; higher leaves a hung one undeclared that long. (b) a multiple of `alive-time` (6 times), with no leaf of its own. (c) no allowance: Ze's own dump is subject to `alive-time`, and a table whose dump takes longer than 10 s gets its VPP killed when Ze restarts beside it | |
| C-17 | The whole procedure for a process Ze may not kill (D-5), which his rules leave "for the design to propose to him". It departs from his row "Two limits", which forbids a reconnect attempt with no bound | (a) recommended: no kill; no start of the configured unit while that process lives; reconnect attempts without end, adopting whenever it answers; the loss declared, the deadline and the withdrawal as for any process; the unit started once the process has exited. (b) at the deadline Ze stops reconnecting and only waits for the exit: bounded attempts, and a foreign VPP that answers again later is not adopted until an operator acts. (c) at the deadline Ze starts the configured unit beside the foreign process: bounded, and two VPPs then contend for the NICs and the socket | |
| C-18 | The hold-down after a crash-loop withdrawal. R-11 says the interfaces come up "when VPP recovers"; the design holds `vpp/recovered` until fewer restarts than the count lie inside the window, so a VPP that is ready and stable can stay withdrawn for up to the window (600 s) | (a) recommended: hold, so the interfaces do not flap with a VPP that crashes every few minutes. (b) return at once when VPP is ready, as R-11 reads literally: a crash-looping VPP then takes the interfaces down and up on every cycle. (c) a shorter hold of its own (for example 60 s ready) | |
| C-19 | What counts as a restart, and the backoff caps: neither is in his words | (a) recommended: every new VPP process after a declared loss counts, whoever started it; a failed start counts; an advance of the unit's NRestarts beyond what Ze saw counts. No delay before the first restart inside a window, then 1 s doubling to 30 s (today's cap), and to 300 s once withdrawn. (b) count only the restarts Ze itself requests: a tight systemd restart loop is then invisible to the limit. (c) other caps | |
| C-20 | BFD across a form (2) restart. Facts: every BFD session over VPP is lost during the restart by its own detection time (under 3 s, A-5), so its clients (BGP, OSPF, static) see a Down and react, although Ze publishes no `down`. That is the timer his row lets run, not a withdrawal by Ze, but it means "nothing is withdrawn" does not hold for a route protected by BFD. And today a loop bound to a device cannot be rebound (`loopFor` tells the operator to restart Ze), so after any VPP restart those sessions never return | (a) recommended: the loss stands, as A-5 says, and the return is built: the BFD component opens a new transport for a bound loop when its device is recreated, and the sessions come Up by themselves. (b) Ze also holds BFD's clients from reacting to a Down while a recovery is inside its limits: it keeps BFD-protected routes through a restart and suspends what BFD is for, against A-5. (c) nothing is built: BFD over VPP stays down after a VPP restart until Ze restarts, which R-11 does not allow | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the method and the events exist end to end, as stubs
   - Tests: `vpp-restart-method-refused`, `vpp-loss-restart`, `vpp-loss-adopt`, `vpp-loss-withdraw`, `vpp-event-subscription`; `TestParseRestartMethod`, `TestExternalLeafIsRefused`, `TestRecoveryLimitsValidate`
   - Files: the YANG module, `config.go`, `events.go`, `register.go`, `cmd_vpp_stub.go`, the four fixtures, every census writer of `external`, the site goldens
   - Verify: the config parses, `external` is refused, every repository config is converted, the stub runs under `exec`, and the loss tests fail because the state machine is today's loop
2. **Phase: The lifecycle under `exec`** -- D-4, D-6's list, D-7's long reads and presence checks
   - Tests: the `lifecycle_test.go` rows; `vpp-late-probe`, `vpp-loss-exit`, `vpp-crash-loop`, `vpp-loss-return`
   - Files: `lifecycle.go`, `vpp.go`, `conn_health.go`, `startup_readiness.go`, `runtime_stats.go`, `process_peer.go`, `process_peer_linux.go`, `process_exit.go`
   - Verify: tests fail, implement, tests pass; the wiring tests progress to the interface events
3. **Phase: The interface backend, `iface`, `fib-vpp`, the IGP transports and LDP discovery** -- D-8's interface events, the control device, the rebind, the reconcile without reload, the replay and sweep without dump
   - First: `vpp-igp-adjacency-frr` is built and run before any other change of this phase. It decides finding 12: red, the control device is built first; green, finding 12 is corrected here and the rebind compares whatever index the transports already bind
   - Tests: the `monitor_test.go` rows, `TestAdoptedReconcilesWithoutReload`, `TestAdoptedReplaysWithoutDump`, `TestAdoptedSweepsRouteWithdrawnWhileFenced`, `TestAdoptedSweepsLabelWithdrawnWhileFenced`, `TestMarkDumpIsADeclaredLongRead`; the control-device rows and the rebind rows of the three transports and two engines; `TestDiscoveryRebindsOnMovedDevice`; `vpp-loss-withdraw`
   - Files: `monitor.go`, `ifacevpp.go`, `query.go`, `iface/register.go`, `iface/resolve.go`, `config_apply.go`, `fib/vpp/register.go`, `routesource.go`, `fibvpp.go`, `mplsstale.go`, `mplsentry.go`; the three `transport.go` and their `backend_linux.go`; `ospf/instance.go`, `gr_preserve.go`, `ipsec_install.go`; `isis/circuits.go`; `internal/plugins/ldp/register.go` (the LDP rebind) and `docs/architecture/ldp/mpls-ldp.md`; the transport, resolver and wire pages the Documentation checklist names for this phase
   - Before the transports change: the three transport pages are read whole, A-12 is settled by reading the reconcile order in `config_apply.go`, and A-13 by reading `lcp_interface.c` at the pin. A statement against a rebind, or a MAC that changes, sends C-8 back to Thomas for that protocol
4. **Phase: The systemd method** -- D-3, D-5, D-6, D-9
   - Tests: `unit_restart_test.go`, `internal/core/systemdunit` tests, `TestSignalProbeReportsPermission`, the foreign rows; the four `vpp-systemd-*` labs
   - Files: `internal/core/systemdunit/`, `unit_restart.go`, `internal/plugins/systemd/doctor.go`, `unit.go`
5. **Phase: The other VPP owners** -- firewall, traffic, static and IKE apply again on `adopted` and `reconnected`
   - Tests: one unit test per backend that the event runs its apply; `vpp-loss-restart` asserts their state in the new stub
   - Files: the four backend files. A-6 is validated here first
6. **Phase: BGP and BFD** -- the local-address reactions and the BFD loop reopen
   - Tests: `TestLocalAddressRemovedStopsSession`, `TestLocalAddressRemovedTakesSessionDown`, `TestRestartOpenCarriesFBitZero`, `TestBoundLoopReopensOnRecreatedDevice`; `vpp-loss-withdraw-bgp`, `vpp-loss-withdraw-bfd`
   - Files: `reactor_iface.go`; `internal/component/bfd/session_identity.go` (the local-address reaction); `internal/component/bfd/bfd.go` and the loop's transport swap in `internal/component/bfd/engine/` (the loop reopen); `docs/architecture/bgp/interface-event-reactions.md`, `docs/architecture/bfd.md`, `docs/architecture/config/transaction-protocol.md`
   - Before the loop reopen: A-14 is settled by reading the caller of `sendControl`
7. **Phase: Doctor, metrics and pages** -- D-10 and the Documentation checklist rows not already made in their phase
   - Tests: `doctor_recovery_test.go`; `vpp-loss-doctor`
   - Files: `doctor_recovery.go`, `register_doctor_codes.go`, the pages
8. **Phase: Labs and interop** -- the `vpplifecycle` suite
   - Tests: every lab and interop row, each proven red with its reaction reverted
   - Files: `internal/le/test/deployment/vpplifecycle/`

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No path signals a process by PID; no path signals a foreign process; no start request is sent to a unit that is activating or in auto-restart; the unit is read before every action |
| Naming | Event payload keys and config leaves are kebab-case; the doctor codes start with `doctor-vpp-` |
| Data flow | `vpp` and `iface` import no protocol; BGP reaches the withdrawal only through the `interface` namespace |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: `ai/rules/goroutine-lifecycle.md` | The state machine is one long-lived worker; no goroutine per probe, per loss or per event |
| Rule: `ai/rules/no-layering.md` | `External`, the first-probe kill and the never-reset backoff are deleted, not kept beside the new path |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `external` is gone from the tree | A search for `external true` and `.External` over `internal`, `test`, `docs` and `ai` finds no VPP hit |
| The four event types are registered and documented | A search for the four names in `internal/core/vpp/events/events.go` and `docs/architecture/api/process-protocol.md` |
| Each `.ci` and lab row runs | `./le test vpp` and the `vpplifecycle` deployment suite, run by the main thread |
| The three doctor codes are registered | `./le doc check all` and the doctor inventory test |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| The kill | It can only reach the process whose pidfd Ze holds from the API socket's peer credentials; no PID, name or path selects a target |
| The unit name | It is passed to `systemctl` as one argument, never through a shell, and its YANG pattern admits only unit-name characters |
| `startup-conf` and `ze.vpp.binary` | The first is an operator's config leaf with the path pattern `startup-cli-file` uses; the second is an environment variable only whoever starts Ze can set. Neither is reachable from a remote interface without commit rights |
| The rights of C-1 | CAP_KILL and the polkit rule are no wider than the chosen option states, and the doctor check says when they are missing |
| Report content | The foreign report prints a PID, a program path and a cgroup: nothing secret, and only to an operator allowed to run `ze doctor` |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A fact of V-1 to V-11 or S-1 to S-6 reads differently at the producer | Stop the phase, correct the fact here, and take the changed value or reaction back to Thomas |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The unit of a process is read from its cgroup under the held pidfd | `GetUnitByPIDFD` over D-Bus | One route on every systemd version, and it extends the `systemctl` use Ze has |
| One sliding list of restarts drives the count, the backoff and the return | A stable time and a backoff reset of their own | One mechanism, and finding 8 disappears with it |
| The withdrawal removes addresses, then takes interfaces down | `down` alone | Connected routes follow the address events only, and BGP and BFD bind to addresses |
| Form (1) replays and sweeps with its mark from the retired writer, and never dumps the FIB | The mark by dump, as on a reconnect; a replay with no sweep | V-5: the dump stops forwarding for its run. Without a sweep a route withdrawn while fenced would forward for ever (C-5) |
| A restart inside the limits publishes no `down`; the IGP transports rebind on the `up` | `down` then `up` for every interface (review I-7) | A `down` is a withdrawal his row forbids, and it costs static routes, RSVP-TE LSPs and every IGP adjacency on each restart. The rebind keeps what the timers did not already take (C-8) |
| The kernel device of an interface is part of the resolver's binding | Each transport asks the VPP backend for its LCP pair | A transport then names no backend, and a kernel-socket consumer cannot take a data-plane index for a kernel one (finding 12) |
| A BGP peer whose local address left waits in a state of its own, `address-wait` | Reuse `PeerStateIdleHold`; let the peer retry and fail | Idle-hold is ended only by an operator, and a retry loop on an address that is absent is noise and never says why the peer is down |

## Known Limitations

- Design correction (2026-11-23): the two limitations recorded here (VPP's
  behavior under API loss and systemd's start handling "ASSUMED until read")
  no longer hold. Both are read: V-1 to V-11 and S-1 to S-6.
- A VPP in another container has no method (C-12).
- A hung main thread can leave the workers forwarding or stopped; Ze cannot
  tell which from the API (V-3) and does not try (A-4). The peers' own
  timers and BFD decide.
- PPPoE's reactions to `lost`, `adopted` and `reconnected` are built by
  `plan/post-release/features/spec-pppoe-vpp-dataplane.md`, which depends on
  this spec.
- Where the `exec` program comes from on the appliance is
  `plan/release/delivery/spec-vpp-embedded-runtime.md`.

## Independent design review (strongest tier, 2026-11-23)

Verdict: not approvable as recorded. Three blocking findings, eleven
important ones and seven notes. Each is applied above, or recorded as an open
choice where only Thomas can answer it. Status, 2026-11-23: Thomas answered
the owner choices the findings raised (A-1 to A-7); the Applied column says
where each answer went. The design itself is still unwritten and unjudged.

| # | Finding | Applied |
|---|---------|---------|
| B-1 | "Ze is the only restarter (Restart=no)" was proposal P-3 and Q-5 reverses it | R-8 removed; R-9 and P-3 rewritten; Task, steps 2, 2a and 3.3, choice 4; page row 9 added, row 6's removal listed for `decision-change`; the PPPoE spec and handover 61 corrected |
| B-2 | The detect-and-adapt mechanism was missing | Step 2a |
| B-3 | P-1, P-2 and "Ze stops restarting" are not in his words | Resolved: he adopted P-1 and P-2 (A-2, R-10, page row 12) and replaced "Ze stops restarting" with retrying (A-3, R-11, page row 13). Page rows 7 and 8 stay as committed until his `decision-change` removes them |
| I-1 | Two readings of R-4 | Resolved: reading (b) (A-1, R-4, page row 10, which supersedes row 5) |
| I-2 | Three states; a SIGSTOP does not exercise form (1) | Step 3.1 table, step 5, the functional test row, the PPPoE spec's lab row |
| I-3 | Unknown outcomes concern every VPP owner | Step 3.1, choice 6 |
| I-4 | The full readback and its bound | Resolved: no blocking readback; the presence of configured data adopts, and reconciliation runs in the background and repairs (A-4, R-7, step 3.1a, page row 11) |
| I-5 | Form (1) must not end in `vpp/reconnected` | Narrowed by A-4: its resync reason no longer holds, its PPPoE reason stands (step 3.1a, choice 8) |
| I-6 | R-6 reworded; Ze's own timers | Resolved: Ze's own timers run as their RFCs say (A-5, R-6, step 4, page row 14) |
| I-7 | New TAP ifindexes and OSPF rebinding | Finding 10, step 3.3, functional test row |
| I-8 | BGP's mechanism; originated routes over non-VPP sessions | Step 4 |
| I-9 | A proven exit declares the loss at once | Task, step 3.1, data flow |
| I-10 | Gaps against "manage" | Step 1 |
| I-11 | The bucket | Resolved: post-release, not moved (A-7, header, choice 15) |
| N-1 | `backoff` never resets | Finding 8 |
| N-2 | `vpp/disconnected` on shutdown and replacement | Finding 9, step 3.2, choice 8 |
| N-3 | The users of `external true` | Step 1 |
| N-4 | "eight" choices against the table | Checklist |
| N-5 | R-3's widening rests on the later quote | R-3 cites Q-3 |
| N-6 | pidfd kill and SIGKILL are derived mechanisms in an owner row | R-5 |
| N-7 | Required Reading cited only R-1 to R-3 | Required Reading |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Every choice in "Choices the rules do not settle" answered and written into the design (A-1 to A-7 written in on 2026-11-23; open: choices 4, 7 and 8, and the values of P-1, P-2, P-4 and P-5, all proposed at design review)
- [ ] VPP's source read for the ASSUMED facts
- [ ] systemd's source read for the ASSUMED facts
- [ ] Design documents table derived with `./le spec design documents`
- [ ] No code snippets

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol reactions (OSPF, IS-IS, BGP peers see the loss)
