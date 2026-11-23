# Spec: vpp-failure-lifecycle -- Ze manages every VPP it runs, kills and restarts it after a declared loss, and withdraws only when it cannot recover it

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | Owner decision (2026-11-23) on how Ze manages VPP and reacts to its failure. Homed post-release by the brief that recorded it; the main session puts the bucket to Thomas, because the routing reaction (findings 1 to 3) is a defect of the VPP backend the first release ships |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner rules (Thomas, 2026-11-23)

These are his rules, recorded as the `## Design decisions` rows of
`docs/research/vpp-deployment-reference.md` (the `// Design:` page of
`internal/component/vpp/vpp.go`). This spec is the record their Source cells
name.

| # | Rule |
|---|------|
| R-1 | Ze refuses to run VPP unmanaged. If VPP is external, the operator accepts that Ze may terminate and restart it |
| R-2 | Ze must know how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime (`plan/release/delivery/spec-vpp-embedded-runtime.md`) |
| R-3 | After a failure: if the process is alive, routing (forwarding) continued, and Ze reconnects and can see the configured state, Ze continues from where it is. If the software could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated. R-4 to R-7 narrow it: its continue branch holds only inside the single timer (form 1 of R-7), and peers are updated only when neither recovery form succeeds |
| R-4 | Refinement, same day, narrowing R-3's continue branch to the timer. One timer: the alive test and the loss declaration use the same time. A probe missed inside it is not a loss |
| R-5 | As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, wait for the exit proof), then restarts and reconfigures VPP |
| R-6 | Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only if Ze cannot restart and reconfigure (recover) VPP |
| R-7 | Clarification, same day, on top of R-4 to R-6. "Recovered" has exactly two forms. (1) Ze reconnected within the time limit (the single alive and loss timer) and checked that everything it configured is still configured: Ze continues, with no kill and no withdrawal. The reconnect must reach the same pidfd-proven process, and a full readback of what Ze installed (interfaces, LCP pairs, owned routes, PPPoE sessions) must match; any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit |
| R-8 | Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no) |

Consequences of R-4 to R-7, recorded with them:

- The adopt path survives only inside the timer (form 1), with option C's
  identity proof and a full readback (PPPoE spec). After the timer there is
  no adopt.
- Ze needs the retained process lease and pidfd: it is the identity proof of
  form (1), the kill handle and the source of the exit proof (PPPoE F-1 stays;
  F-2 closes with it; F-3 is fixed for form 1 by PPPoE change 2).
- No graceful-restart claim on re-open: the F bit stays 0.
- PPPoE sessions are torn down per OD-2, because their VPP state is gone, and
  subscribers redial.

Conditions the main session proposed to Thomas. He adopted P-1 to P-3 as
rules (R-7, R-8); three values stay open:

| # | Proposal | Value |
|---|----------|-------|
| P-1 | A recovery deadline. Recovered means VPP restarted, interfaces and LCP pairs applied again, and the FIB replay complete. Past the deadline Ze withdraws (R-6) | Open (the deadline value) |
| P-2 | A crash-loop limit: N restarts inside a window, then Ze stops restarting, withdraws, and reports through `ze doctor` | Open (N and the window) |
| P-3 | Ze is the only restarter: the systemd method requires a unit that does not restart VPP itself (Restart=no, as VyOS) | Decided (R-8) |
| P-4 | The single loss timer of R-4 | Open (today a 1 s probe tick and a 5 s probe timeout) |

## Task

Today Ze runs VPP in one of two modes. In managed mode it starts the program,
and on the first failed API probe it kills it and starts it again. In external
mode (`vpp external true`) it only connects, never signals the process, and
reconnects after an API loss. In neither mode does the rest of Ze react to the
loss: no VPP-backed interface goes down in Ze's model, so OSPF, IS-IS, LDP,
RSVP-TE, VRRP, static routes, BGP and BFD learn that the data plane died only
from their own timers or from the kernel removing the linux-cp TAPs.

Goal: one lifecycle for every VPP Ze runs. Ze always holds a restart method
(R-1, R-2). Inside its single timer (R-4), a reconnect to the same
pidfd-proven process whose full readback matches lets Ze continue with no kill
and no withdrawal (form 1, R-7). Ze declares a loss when the timer expires or
form 1 fails. It then kills the process through the retained pidfd, waits for the exit proof,
restarts VPP by the method and reconfigures it (R-5), accepting the blackhole
meanwhile (form 2). Only when neither form succeeds within the recovery
deadline and the crash-loop limit does it take every VPP-backed interface down
in its model, so IGP and BGP peers are updated (R-6). Ze is the only restarter
(R-8).

The PPPoE VPP data plane (`plan/post-release/features/spec-pppoe-vpp-dataplane.md`,
D-1 and OD-2) depends on this spec's decision procedure and events.

## Findings (read at the producers, 2026-11-23)

| # | Finding | Where |
|---|---------|-------|
| 1 | Nothing reacts to VPP loss. `vpp/disconnected` has one emitter and no subscriber outside tests. `iface` (`subscribeReconcileOnReady`) and `fib-vpp` (`onVPPReady`) subscribe to connected and reconnected only. The only process-exit observer is PPPoE (`RegisterProcessExitObserver`) | `VPPManager.retireReadiness` (`internal/component/vpp/startup_readiness.go`); `l2tp/pppoe/dataplane/vpp/reconcile.go` |
| 2 | No interface goes down in Ze's model. The VPP monitor emits `interface/down` only from a VPP `SwInterfaceEvent`. OSPF and IS-IS drop adjacencies only through `Transport.HandleLinkDown` from that bus; LDP, RSVP-TE, VRRP, static (`routeManager.onLinkEvent`) and connected routes read the same bus. So adjacencies and sessions die by timers or by kernel TAP removal | `monitor.handleEvent` (`internal/plugins/iface/vpp/monitor.go`) |
| 3 | BGP has no link-down reaction: `Reactor.handleAddressRemoved` stops listeners and closes no session. BFD imports no interface events | `internal/component/bgp/reactor/reactor_iface.go` |
| 4 | Managed mode kills VPP on the first failed probe (1 s tick, 5 s timeout). Any stream error fails a probe at once, so the loss declaration is not R-4's single timer; external mode then never kills (R-5) | `waitVPPConnectionWithStop` (`internal/component/vpp/vpp.go`), `startHealth`, `connectionProbeTimeout` (`conn_health.go`) |
| 5 | External mode cannot kill or restart. `runOnce` closes the connector; `runWithOps` backs off from 1 s, doubling to 30 s, and reconnects. No `PidfdSendSignal` exists anywhere, though each connection holds the peer pidfd. No systemd unit control exists under `internal/component` or `internal/core` | `captureProcessWitness` (`process_peer_linux.go`, SO_PEERPIDFD) |
| 6 | Graceful restart is already safe on reconnect: `restartFlagsFor` sets R and F only inside the `ze signal restart` marker window, and `fib-vpp` registers no `PreservesForwardingState` (only `fib/kernel` does). Exposure: a GR-capable peer keeps Ze's routes for its restart time after a silent TCP death | `internal/component/bgp/reactor/peer_gr_flags.go` |
| 7 | Reconnect already resyncs: `iface` runs `reconcileOnVPPReady`; `fib-vpp` `restart` replays and then sweeps stale routes after `labelSweepDelay`. Unverified: a reloaded iface backend re-creating loopbacks against a same-process VPP | `internal/component/iface/config_apply.go`, `internal/plugins/fib/vpp/` |

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
   only that Ze did not write `startup.conf` or bind the NICs. The ten users
   of `external true` in the repository convert (four under
   `internal/le/test/deployment`, four fixtures, the IS-IS test,
   `config_test.go`), including the PPPoE product lab and the `./le test vpp`
   stub (choice 7).
2. **Kill by pidfd, never by PID**, in both methods: signal the retained pidfd
   of the connection's process witness. Death is the existing proof
   (`observeExit`, then `notifyProcessExit`). After a systemd restart: take a
   fresh witness, require `ProcessLease.SameProcess(old, new)` false and the
   old exit proof, and bind the new peer to the unit (by cgroup, or systemd
   unit-by-pidfd, ASSUMED) as the systemd analogue of `bindManagedProcess`.
   Signal choice: choice 5. Unit policy: choice 4.
3. **Loss and recovery (R-4 to R-8; narrows the first draft's adopt verdict
   to form (1) inside the timer, and moves its down verdict to a failed
   recovery):**
   1. One timer (P-4) covers both the alive test and the loss declaration. A
      probe missed inside it is not a loss and changes nothing. Today a single
      failed ControlPing declares the loss (`startHealth`), and any stream
      error fails a probe at once, not only the 5 s timeout.
   1a. Form (1), inside the timer (R-7): when the API connection is lost Ze
      fences mutations and reconnects. It continues only when the new peer is
      the same process (`SameProcess` true on the retained leases) and a full
      readback of interfaces, LCP pairs, owned routes and PPPoE sessions
      matches what it installed; then `vpp/reconnected`, with no kill and no
      withdrawal. A different process, an unprovable comparison or any
      mismatch fails form (1) and declares the loss.
   2. On its expiry Ze declares the loss and emits `vpp/disconnected`
      (event names: choice 8). It fences mutations, kills the process through
      the retained pidfd at once (SIGKILL, choice 5), and waits for the exit
      proof.
   3. Ze restarts VPP by its method, verifies the new process identity
      (`SameProcess(old, new)` false, old exit proven), applies the interfaces
      and LCP pairs again, and replays the FIB. That is "recovered" (P-1);
      then `vpp/connected` or `vpp/reconnected`, as today's resync expects.
   4. Meanwhile VPP-backed interfaces stay up in Ze's model and nothing is
      withdrawn: the blackhole is accepted.
   5. If recovery fails (the start fails, the deadline P-1 passes, or the
      crash-loop limit P-2 is reached), Ze withdraws: `iface` publishes
      `interface/down` for every VPP-backed interface, Ze stops restarting,
      and `ze doctor` reports it.
4. **Peers updated on withdrawal only.** OSPF, IS-IS, LDP, RSVP-TE, VRRP,
   static and connected routes react through the existing `interface/down`:
   routes leave sysrib, and BGP stops advertising what it redistributed. BGP
   sessions over VPP interfaces close locally (the peers detect it by hold
   timer or BFD). BGP sessions over a path that does not ride VPP keep running;
   their routes through VPP leave with the interfaces (choice 3 answered: no
   withdrawal while recovering). BFD sessions on VPP interfaces go Down locally,
   which needs a hook. On re-open no graceful-restart claim is made: the F bit
   stays 0, and the VPP restart path never writes the `ze signal restart`
   marker. OSPF and IS-IS restart signalling (grace-LSA, RR) stays off (not
   read).
5. **PPPoE.** The D-1 dead end disappears: a declared loss always ends in a
   kill, so the exit proof always arrives. This needs the PPPoE provider to
   take a process lease (F-1 in the PPPoE spec; F-2 and F-3 close with it).
   Form (1) keeps the sessions through the PPPoE option C readback, inside the
   timer only. OD-2's drain runs on every declared loss: after a forced
   restart VPP's session state is gone, so sessions are torn down and
   subscribers redial. The lab's product-restart SIGSTOP case asserts: a
   freeze shorter than the timer ends in form (1), sessions surviving; a longer
   one ends in a kill, a restart, the OD-2 drain and a redial.

## Choices the rules do not settle (owner)

| # | Choice | Facts |
|---|--------|-------|
| 1 | The single loss timer (P-4) | Open value. Today a 1 s probe tick and a 5 s probe timeout |
| 2 | What proves form (1) | Answered by R-7: same pidfd-proven process and a full readback match; no forwarding sample is required |
| 3 | BGP sessions that do not ride VPP | Answered: no withdrawal while recovering; on withdrawal their VPP routes leave with the interfaces |
| 4 | systemd unit policy | Answered by R-8: Restart=no, Ze the only restarter. Still open: whether Ze starts an inactive unit at boot |
| 5 | SIGKILL or SIGTERM with grace | Answered: kill at once once the loss is declared (R-5) |
| 6 | PPPoE inside the timer; E-1 on form (1) | Inside the timer PPPoE fences admission and holds its sessions; form (1) keeps them, form (2) drains them per OD-2. E-1 (unknown-outcome creations on form 1) stays open |
| 7 | Which method the functional-test VPP stub declares | Open |
| 8 | Event names: a published plugin contract | Open (`docs/architecture/api/process-protocol.md`) |
| 9 | The recovery deadline (P-1) and the crash-loop limit (P-2) | Open values |

## Pages this work makes wrong

| Page | What |
|------|------|
| `docs/research/vpp-deployment-reference.md`, "Socket Transport" | "Ze neither kills that process": a note now says the decision replaces it and this spec owes it |
| `docs/guide/vpp.md`, "Running against an externally supervised VPP" | The whole mode; a note now says the same |
| `internal/component/vpp/yang/ze-vpp-conf.yang`, the `external` leaf description | Replaced by the restart method |
| `ai/digests/vpp-dataplane.md` | External mode and the loss path; corrected against today's code on 2026-11-23, rewritten when this lands |
| `docs/architecture/l2tp/bng-5-pppoe.md`, "VPP session ownership" | Owned by the PPPoE spec |

## Required Reading

### Architecture Docs
- [ ] `docs/research/vpp-deployment-reference.md` - VPP process lifecycle, socket transport, process identity
  → Decision: R-1, R-2, R-3 ("Design decisions")
  → Constraint: process identity is the pidfs identity of the API peer, never a PID
- [ ] `docs/guide/vpp.md` - what the operator writes for each mode
- [ ] `ai/digests/vpp-dataplane.md` - the supervision loop and its consumers
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - PPPoE owners retained across loss
- [ ] `docs/architecture/api/process-protocol.md` - events are a published contract
- [ ] `docs/architecture/bgp/interface-event-reactions.md` - how BGP reacts to interface and address events
  → Decision: the first interface flip is handled at once, bursts coalesce per peer ("Design decisions")

**Key insights:**
- The process witness, the lease and the exit proof exist; only the kill, the
  restart method and the reaction are missing.

## Current Behavior (MANDATORY)

**Source files read:** (by the 2026-11-23 research; re-read at design)
- [ ] `internal/component/vpp/vpp.go` - `runWithOps` restart loop, `runOnce`, `waitVPPConnectionWithStop` kills the managed child on API loss
- [ ] `internal/component/vpp/conn_health.go` - `connectVPPOnce`, one health worker per connection, first failed probe reports loss
- [ ] `internal/component/vpp/startup_readiness.go` - `retireReadiness` emits `vpp/disconnected`
- [ ] `internal/component/vpp/runtime_stats.go` - `activateRuntimeStatsGeneration` emits connected or reconnected
- [ ] `internal/component/vpp/process_peer.go`, `process_peer_linux.go`, `process_exit.go` - witness, lease, exit proof
- [ ] `internal/plugins/iface/vpp/monitor.go` - `interface/down` only from VPP events
- [ ] `internal/component/bgp/reactor/reactor_iface.go`, `peer_gr_flags.go` - no link-down reaction; GR flags

**Behavior to preserve:**
- Process identity by pidfs, the exit proof, the readiness events and their consumers' resync.
- The GR flag rule: R and F only inside the `ze signal restart` window.

**Behavior to change:**
- The `external` mode, the first-probe kill, and the absent reaction to loss (R-1 to R-3).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VPP supervisor | `internal/component/vpp/vpp.go`, `VPPManager.runWithOps`, `runOnce` | Writes config, binds NICs, runs VPP, connects, restarts with backoff | Extends: gains the decision procedure and the restart method |
| Operator-supervised mode | `internal/component/vpp/config.go`, `VPPSettings.External` | Ze only connects | Replaces: R-1 refuses an unmanaged VPP; the restart method takes its place |
| Process witness and lease | `process_peer.go` `ProcessLease.SameProcess`, `process_peer_linux.go` `captureProcessWitness` | Binds each connection to the peer's pidfs identity | Extends: `SameProcess` proves form (1)'s reconnect reaches the same process and a restarted VPP is a new one; the retained pidfd is the kill handle |
| Exit proof | `process_exit.go`, `observeExit`, `notifyProcessExit` | Publishes a proven process exit | Extends: the down path waits on it before restarting |
| Connection health | `conn_health.go`, `startHealth` | First failed ControlPing reports loss | Extends: one timer (P-4) declares the loss; a probe missed inside it is not one |
| Stats connection | `stats_conn.go`, `runtime_stats.go` | Counters for telemetry and runtime readers | Does not apply: form (1) is proven by identity and readback, not by a forwarding sample |
| Readiness events | `internal/core/vpp/events/events.go` | connected, disconnected, reconnected | Extends: disconnected on a declared loss; a new event for a failed recovery (choice 8) |
| Interface events | `internal/plugins/iface/vpp/monitor.go`, the `interface/down` bus | Driven by VPP interface events only | Extends: a failed recovery publishes `interface/down` for every VPP-backed interface |
| BGP interface reaction | `docs/architecture/bgp/interface-event-reactions.md`, `Reactor.handleAddressRemoved` | Stops listeners, closes no session | Extends: a failed recovery closes BGP sessions over VPP interfaces |
| Embedded runtime start | `plan/release/delivery/spec-vpp-embedded-runtime.md` | Starts VPP from the bundle Ze carries | Extends: it is the `exec` method; that spec keeps where the program comes from |
| PPPoE loss handling | `plan/post-release/features/spec-pppoe-vpp-dataplane.md`, OD-2 and D-1 | Designed, not built | Does not apply here: PPPoE drains on this spec's declared-loss event |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The API loss reported by the connection health worker (`Connector.connectionLost`)
- The `vpp` config root, for the restart method

### Transformation Path
0. Inside the single timer: the VPP component fences mutations and reconnects; a same-process reconnect whose full readback matches continues (form 1), with no kill and no withdrawal
1. Loss: the timer expires or form 1 fails; the VPP component declares it, emits `vpp/disconnected` and fences mutations
2. Terminal: kill through the retained pidfd, wait for the exit proof
3. Recovery: restart by the method, check the new identity, apply interfaces and LCP pairs, replay the FIB
4. Withdrawal, only when recovery fails: `interface/down` for each VPP-backed interface, then the routing reactions, and a `ze doctor` report

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VPP component to its consumers | EventBus `vpp` namespace, a published contract | No |
| iface to routing protocols | EventBus `interface/down` | No |
| Ze to VPP | Retained pidfd signal; systemd unit control | No |

### Integration Points
- `VPPManager.runOnce` and `waitVPPConnectionWithStop` - the decision procedure replaces the first-probe kill
- `iface` VPP monitor - publishes `interface/down` when recovery fails
- `ProcessLease.SameProcess` - proves form (1)'s reconnect reaches the same process, and the restarted VPP is a new one

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-11-23 | Nothing reacts to VPP loss: no `vpp/disconnected` subscriber, VPP-backed interfaces never go down even when VPP cannot be recovered |
| `transient-failure-treated-as-fatal.md` | 2026-11-23 | One failed API probe declares the loss, not the single timer R-4 asks for; external mode then never kills |

## Acceptance Criteria

To be written at design, after the owner values. Each rule gets a row: an
enabled VPP with no method refused at verify; a probe missed inside the timer
changes nothing; a same-process reconnect inside the timer whose full readback
matches continues with no kill and no withdrawal, and one that mismatches
declares the loss; a declared loss ends in a pidfd kill, the exit proof, a
restart by the method and a reconfiguration, with nothing withdrawn while it
recovers and no F bit on re-open; a failed recovery (deadline or crash-loop
limit) takes VPP-backed interfaces down in Ze's model so IGP and BGP peers are
updated, and `ze doctor` reports it.

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
| [design: a real-VPP lab freezes VPP (SIGSTOP) shorter and longer than the timer, kills it, and blocks its restart; asserts nothing changes inside the timer, a kill and recovery with nothing withdrawn after it, and the IGP and BGP reaction when recovery fails] | `internal/le/test/deployment/` | [design] | |

## Files to Modify

To be derived at design. Known: `internal/component/vpp/vpp.go`,
`conn_health.go`, `config.go`, `yang/ze-vpp-conf.yang`,
`internal/core/vpp/events/events.go`, `internal/plugins/iface/vpp/`, the BGP
reactor's interface reaction, BFD, the ten repository users of
`external true`, and the pages in "Pages this work makes wrong".

## Implementation Steps

To be written at design. Phase 1 is wiring: the restart method in YANG and
verify, and the loss and failed-recovery events with their subscribers.

## Known Limitations

- VPP's own behavior under API loss is ASSUMED until VPP's source is read.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] The eight owner choices answered and written into the design
- [ ] VPP's source read for the ASSUMED facts
- [ ] Design documents table derived with `./le spec design documents`
- [ ] No code snippets

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol reactions (OSPF, IS-IS, BGP peers see the loss)
