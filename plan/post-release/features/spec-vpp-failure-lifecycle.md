# Spec: vpp-failure-lifecycle -- Ze manages every VPP it runs, kills and restarts it after a declared loss, and withdraws only when it cannot recover it

| Field | Value |
|-------|-------|
| Status | skeleton |
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
| 5 | External mode cannot kill or restart. `runOnce` closes the connector; `runWithOps` backs off from 1 s, doubling to 30 s, and reconnects. No `PidfdSendSignal` exists anywhere, though each connection holds the peer pidfd. No systemd unit control exists: the one `systemctl` call in shipped code reads getty state (`console_linux.go` `gettyActive`) | `captureProcessWitness` (`process_peer_linux.go`, SO_PEERPIDFD) |
| 6 | Graceful restart is already safe on reconnect: `restartFlagsFor` sets R and F only inside the `ze signal restart` marker window, and `fib-vpp` registers no `PreservesForwardingState` (only `fib/kernel` does). Exposure: a GR-capable peer keeps Ze's routes for its restart time after a silent TCP death | `internal/component/bgp/reactor/peer_gr_flags.go` |
| 7 | Reconnect already resyncs: `iface` runs `reconcileOnVPPReady`; `fib-vpp` `restart` replays and then sweeps stale routes after `labelSweepDelay`. Unverified: a reloaded iface backend re-creating loopbacks against a same-process VPP | `internal/component/iface/config_apply.go`, `internal/plugins/fib/vpp/` |
| 8 | `runWithOps`'s `backoff` only grows: it starts at 1 s, doubles to 30 s and never resets after a good run, so a VPP that crashes once a day restarts after 30 s (review N-1; relevant to P-2) | `VPPManager.runWithOps` (`vpp.go`) |
| 9 | `vpp/disconnected` is emitted by `retireReadiness`, deferred in `runOnce`, so it also fires on shutdown and on a replacement run, not only on a loss. A declared-loss subscriber must tell them apart (review N-2) | `VPPManager.runOnce` (`vpp.go`) |
| 10 | OSPF's transport opens a socket per interface name on link-up and hands the engine the handle's ifindex through `OnInterfaceUp` and `OnInterfaceDown`. A form (2) restart recreates the linux-cp TAPs with new ifindexes, so the engine must see a down and an up for each (review I-7) | `Transport.linkDown`, `OnInterfaceUp` (`internal/plugins/ospf/transport/transport.go`) |
| 11 | A full FIB readback is `IPRouteV2Dump` per table. At a million routes it can outlast a timer of seconds (review I-4) | `routeSource.dumpRoutes` (`internal/plugins/fib/vpp/routesource.go`) |

VPP facts. VPP's source is not on disk, so each of these is ASSUMED until read:

- A crash stops all forwarding. linux-cp TAPs are non-persistent tun file
  descriptors, so the kernel deletes them with their addresses and routes.
- With the API lost and the process alive, the API runs on the main thread and
  the workers keep forwarding unless the main thread holds the worker barrier
  (stuck in a handler that is not multi-thread safe). A missed ControlPing
  cannot tell these apart.
- Lost while the API is down: linux-cp netlink sync, neighbor updates, the
  stats collector.
- Proof of forwarding: the stats segment is shared memory, readable with the
  API dead. `/sys/heartbeat` and `/sys/vector_rate_per_worker` exist
  (`vendor/go.fd.io/govpp/core/stats.go`); per-worker input-node call counts
  advance every loop (ASSUMED). Ze's `runOnce` closes stats right after the
  loss today. Stats and API must be the same pidfs identity
  (`Connector.checkStatsProcessLocked`). Counters prove that workers ran at each
  sample, not across the whole gap; a BFD or IGP session surviving over a VPP
  interface proves punt and inject.

systemd facts, ASSUMED until read in systemd's source: a start request sent
while the unit is already activating or in auto-restart merges with the
pending job and is harmless; the manager answers unit-by-pidfd
(`GetUnitByPIDFD`) on the systemd versions Ze supports.

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
- [ ] `ai/digests/vpp-dataplane.md` - the supervision loop and its consumers
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - PPPoE owners retained across loss
  → Decision: rows 1 to 4 of "Design decisions" (D-1, option C, adoption inside the timer; row 4: adoption on the presence of data inside the reconnect-and-verify limit, background reconciliation)
- [ ] `docs/architecture/api/process-protocol.md` - events are a published contract
- [ ] `docs/architecture/bgp/interface-event-reactions.md` - how BGP reacts to interface and address events
  → Decision: the first interface flip is handled at once, bursts coalesce per peer ("Design decisions")

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
| Process witness and lease | `process_peer.go` `Connector.RetainProcessForGeneration`, `process_lease_equality.go` `ProcessLease.SameProcess`, `process_peer_linux.go` `captureProcessWitness` | Binds each connection to the peer's pidfs identity | Extends: `SameProcess` proves form (1)'s reconnect reaches the same process and a restarted VPP is a new one; the retained pidfd is the kill handle and the key of the unit-by-pidfd check |
| Exit proof | `process_exit.go`, `observeExit`, `notifyProcessExit` | Publishes a proven process exit | Extends: a proven exit declares the loss at once, and the down path waits on it before restarting |
| Process-exit observers | `process_exit.go`, `RegisterProcessExitObserver` | PPPoE registers to learn of an exit | Pattern for the background reconcilers each VPP owner registers (step 3.1a), carried across a process split as a typed request |
| Connection health | `conn_health.go`, `startHealth`, `probeVPPConnection` | Any probe error reports loss | Extends: tells the three states apart; the alive-test time (P-4) declares the loss, and the reconnect-and-verify limit (P-5) bounds form (1) |
| Stats connection | `stats_conn.go`, `runtime_stats.go` | Counters for telemetry and runtime readers | Does not apply: form (1) is proven by identity and the presence of configured data (A-4), not by a forwarding sample |
| Readiness events | `internal/core/vpp/events/events.go` | connected, disconnected (also on shutdown and replacement), reconnected | Extends: a declared-loss event distinct from shutdown, form (1)'s own event or none, a failed-recovery event (choice 8) |
| FIB resync | `internal/plugins/fib/vpp/`, `restart`, replay then sweep | Converges the FIB after reconnect | Extends: form (2) replays as at boot, and form (1) runs it as background reconciliation (A-4, step 3.1a) |
| Interface reconcile | `internal/component/iface/config_apply.go`, `reconcileOnVPPReady` | Reconciles interfaces when VPP becomes ready | Extends: form (1)'s background reconciliation of interfaces and LCP pairs (A-4) |
| Interface events | `internal/plugins/iface/vpp/monitor.go`, the `interface/down` bus | Driven by VPP interface events only | Extends: a failed recovery publishes `interface/down` for every VPP-backed interface; form (2) names the events for the new TAPs |
| OSPF transport rebinding | `internal/plugins/ospf/transport/transport.go`, `OnInterfaceDown`, `OnInterfaceUp` | Opens a socket per name on link-up, hands the ifindex to the engine | Does not change: recovery must publish the events it rebinds on (step 3.3) |
| BGP interface reaction | `docs/architecture/bgp/interface-event-reactions.md`, `Reactor.handleAddressRemoved` | Stops listeners, closes no session | Extends: a failed recovery reaches BGP through an event it subscribes to |
| systemd control | None (finding 5) | - | New: the unit reader of step 2a |
| Embedded runtime start | `plan/release/delivery/spec-vpp-embedded-runtime.md` | Starts VPP from the bundle Ze carries | Extends: it is the `exec` method; that spec keeps where the program comes from |
| PPPoE loss handling | `plan/post-release/features/spec-pppoe-vpp-dataplane.md`, OD-2 and D-1 | Designed, not built | Does not apply here: PPPoE drains on this spec's declared-loss event |

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
| VPP owners to the VPP component | Background reconciler registration, a typed request across a process split | No |
| iface to routing protocols | EventBus `interface/down` | No |
| Ze to VPP | Retained pidfd signal; systemd unit reads and start requests | No |

### Integration Points
- `VPPManager.runOnce` and `waitVPPConnectionWithStop` - the decision procedure replaces the first-probe kill
- `iface` VPP monitor - publishes `interface/down` when recovery fails
- `ProcessLease.SameProcess` - proves form (1)'s reconnect reaches the same process, and the restarted VPP is a new one
- `iface` `reconcileOnVPPReady` and `fib-vpp` replay and sweep - form (1)'s background reconciliation

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-11-23 | Nothing reacts to VPP loss: no `vpp/disconnected` subscriber, VPP-backed interfaces never go down even when VPP cannot be recovered |
| `transient-failure-treated-as-fatal.md` | 2026-11-23 | One failed API probe declares the loss, not the single timer R-4 asks for; external mode then never kills |

## Acceptance Criteria

To be written at design; the owner choices of the rules are answered (A-1 to
A-7). Each rule gets a row: an enabled VPP with no method refused at verify; a
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
| AC-1 | [design] | [design] | [design] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [design] | → | [design] | [design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [design] | [design] | [design] | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [design: a real-VPP lab freezes VPP (SIGSTOP) shorter than the alive-test time and longer than both limits, drops the API alone with the process alive (mechanism at design, step 5), kills it, and blocks its restart; asserts nothing changes on a short freeze, form (1) with background reconciliation on the API-only drop, a kill and recovery with nothing withdrawn after a long freeze, OSPF and IS-IS adjacencies re-formed over the new TAPs, the IGP and BGP reaction once recovery passes its limits, and the interfaces and routes back when a later retry recovers VPP] | `internal/le/test/deployment/` | [design] | |

## Files to Modify

To be derived at design. Known: `internal/component/vpp/vpp.go`,
`conn_health.go`, `config.go`, `yang/ze-vpp-conf.yang`,
`internal/core/vpp/events/events.go`, `internal/plugins/iface/vpp/`, each VPP
owner's background reconciler (step 3.1a), the BGP reactor's interface reaction,
BFD, the repository users of `external true` (step 1), and the pages in
"Pages this work makes wrong".

## Implementation Steps

To be written at design. Phase 1 is wiring: the restart method in YANG and
verify, and the loss and failed-recovery events with their subscribers.

## Known Limitations

- VPP's own behavior under API loss is ASSUMED until VPP's source is read.
- systemd's handling of a redundant start request and its unit-by-pidfd
  lookup are ASSUMED until systemd's source is read.

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
