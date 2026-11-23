# Spec: vpp-failure-lifecycle -- Ze manages every VPP it runs, decides after a failure whether to adopt or restart it, and updates its routing peers

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
| R-3 | After a failure: if the process is alive, routing (forwarding) continued, and Ze reconnects and can see the configured state, Ze continues from where it is. If the software could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated |

## Task

Today Ze runs VPP in one of two modes. In managed mode it starts the program,
and on the first failed API probe it kills it and starts it again. In external
mode (`vpp external true`) it only connects, never signals the process, and
reconnects after an API loss. In neither mode does the rest of Ze react to the
loss: no VPP-backed interface goes down in Ze's model, so OSPF, IS-IS, LDP,
RSVP-TE, VRRP, static routes, BGP and BFD learn that the data plane died only
from their own timers or from the kernel removing the linux-cp TAPs.

Goal: one lifecycle for every VPP Ze runs. Ze always holds a restart method
(R-1, R-2). After an API loss it decides between two verdicts (R-3): adopt a
VPP it proves alive, forwarding and still holding Ze's state; or declare the
data plane down, take every VPP-backed interface down in its model, update its
IGP and BGP peers, kill the process by its pidfd, restart it by the method, and
rebuild.

The PPPoE VPP data plane (`plan/post-release/features/spec-pppoe-vpp-dataplane.md`,
D-1 and OD-2) depends on this spec's decision procedure and events.

## Findings (read at the producers, 2026-11-23)

| # | Finding | Where |
|---|---------|-------|
| 1 | Nothing reacts to VPP loss. `vpp/disconnected` has one emitter and no subscriber outside tests. `iface` (`subscribeReconcileOnReady`) and `fib-vpp` (`onVPPReady`) subscribe to connected and reconnected only. The only process-exit observer is PPPoE (`RegisterProcessExitObserver`) | `VPPManager.retireReadiness` (`internal/component/vpp/startup_readiness.go`); `l2tp/pppoe/dataplane/vpp/reconcile.go` |
| 2 | No interface goes down in Ze's model. The VPP monitor emits `interface/down` only from a VPP `SwInterfaceEvent`. OSPF and IS-IS drop adjacencies only through `Transport.HandleLinkDown` from that bus; LDP, RSVP-TE, VRRP, static (`routeManager.onLinkEvent`) and connected routes read the same bus. So adjacencies and sessions die by timers or by kernel TAP removal | `monitor.handleEvent` (`internal/plugins/iface/vpp/monitor.go`) |
| 3 | BGP has no link-down reaction: `Reactor.handleAddressRemoved` stops listeners and closes no session. BFD imports no interface events | `internal/component/bgp/reactor/reactor_iface.go` |
| 4 | Managed mode kills VPP on the first failed probe (1 s tick, 5 s timeout): this contradicts R-3's adopt branch | `waitVPPConnectionWithStop` (`internal/component/vpp/vpp.go`), `startHealth`, `connectionProbeTimeout` (`conn_health.go`) |
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
3. **Decision procedure:**
   1. API loss: fence new mutations, keep the stats connection, sample the
      worker counters and the heartbeat at each probe interval, and emit a new
      "API lost, undecided" event (choice 8). `vpp/disconnected` keeps one
      meaning: the data plane is down.
   2. Exit proven, or workers stopped: the down path at once.
   3. Otherwise reconnect within the bound (choice 1). Adopt needs
      `SameProcess` true, the stats peer of the same identity, worker counters
      advanced at every sample (choice 2), and a readback equal to what Ze
      installed: interfaces, LCP pairs, owned routes, PPPoE sessions. Then
      emit `vpp/reconnected`; `iface` and `fib-vpp` resync as today.
   4. Any miss, or the bound expiring: emit `vpp/disconnected`; `iface`
      publishes `interface/down` for every VPP-backed interface; kill, wait
      for the exit proof, restart by the method, verify the new identity, emit
      `vpp/connected`, rebuild.
4. **Peers updated on the down path.** OSPF, IS-IS, LDP, RSVP-TE, VRRP, static
   and connected routes react through the existing `interface/down`: routes
   leave sysrib, and BGP stops advertising what it redistributed. BGP sessions
   over VPP interfaces close locally (the peers detect it by hold timer or
   BFD). BGP sessions over a path that does not ride VPP: choice 3; where they
   close, a NOTIFICATION, with an RFC 8538 Hard Reset where negotiated
   (`bgp/message/notification.go`, not read through). They reopen with R=0 and
   F=0: the VPP restart path never writes the restart marker. OSPF and IS-IS
   restart signalling (grace-LSA, RR) stays off (not read). BFD sessions on VPP
   interfaces go Down locally, which needs a hook. On adopt nothing is
   announced.
5. **PPPoE.** The D-1 dead end disappears: a live VPP Ze cannot adopt is
   killed, so its exit proof always arrives. This needs the PPPoE provider to
   take a process lease (F-1, F-2 in the PPPoE spec). OD-2's drain runs on the
   down verdict only; in the undecided window PPPoE fences admission and holds
   its sessions (choice 6). Adopt keeps the sessions through the PPPoE
   option C readback. The lab's product-restart SIGSTOP case then asserts
   either "adopted, sessions survive" or "killed after the bound".

## Choices the rules do not settle (owner)

| # | Choice | Facts |
|---|--------|-------|
| 1 | The bound before an alive, unreachable VPP is killed | Today: 6 s to declare the loss, then 10 connect attempts 1 s apart |
| 2 | Adopt on an idle box where only the loop counters move: is that enough, or is a surviving BFD or IGP session required | Counters prove workers ran at each sample; a surviving session proves punt and inject |
| 3 | BGP sessions that do not ride VPP: tear them down, or withdraw only the affected routes | - |
| 4 | systemd unit policy: require Restart=no (Ze the sole restarter, as VyOS) or tolerate a systemd restart (as osvbng) and detect it by identity; and whether Ze starts an inactive unit at boot | - |
| 5 | SIGKILL at once (today) or SIGTERM with a grace period | - |
| 6 | PPPoE sessions held in the undecided window; E-1 (unknown-outcome creations) on adopt | PPPoE spec, "Implementation designs" |
| 7 | Which method the functional-test VPP stub declares | - |
| 8 | Event names: a published plugin contract | `docs/architecture/api/process-protocol.md` |

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
| Process witness and lease | `process_peer.go` `ProcessLease.SameProcess`, `process_peer_linux.go` `captureProcessWitness` | Binds each connection to the peer's pidfs identity | Extends: the retained pidfd becomes the kill handle and the adopt proof |
| Exit proof | `process_exit.go`, `observeExit`, `notifyProcessExit` | Publishes a proven process exit | Extends: the down path waits on it before restarting |
| Connection health | `conn_health.go`, `startHealth` | First failed ControlPing reports loss | Extends: loss opens the undecided window instead of an immediate kill |
| Stats connection | `stats_conn.go`, `runtime_stats.go` | Counters for telemetry and runtime readers | Extends: kept open through the window to sample worker counters and the heartbeat |
| Readiness events | `internal/core/vpp/events/events.go` | connected, disconnected, reconnected | Extends: adds the undecided event; disconnected means data plane down |
| Interface events | `internal/plugins/iface/vpp/monitor.go`, the `interface/down` bus | Driven by VPP interface events only | Extends: the down verdict publishes `interface/down` for every VPP-backed interface |
| BGP interface reaction | `docs/architecture/bgp/interface-event-reactions.md`, `Reactor.handleAddressRemoved` | Stops listeners, closes no session | Extends: the down verdict closes BGP sessions over VPP interfaces |
| Embedded runtime start | `plan/release/delivery/spec-vpp-embedded-runtime.md` | Starts VPP from the bundle Ze carries | Extends: it is the `exec` method; that spec keeps where the program comes from |
| PPPoE loss handling | `plan/post-release/features/spec-pppoe-vpp-dataplane.md`, OD-2 and D-1 | Designed, not built | Does not apply here: PPPoE consumes this spec's verdict events |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The API loss reported by the connection health worker (`Connector.connectionLost`)
- The `vpp` config root, for the restart method

### Transformation Path
1. Loss: the VPP component fences mutations and opens the undecided window, sampling stats
2. Verdict: adopt (reconnect, identity and readback proven) or down
3. Down: `vpp/disconnected`, then `interface/down` for each VPP-backed interface, then the routing reactions, then kill by pidfd, restart by the method, `vpp/connected`, rebuild

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VPP component to its consumers | EventBus `vpp` namespace, a published contract | No |
| iface to routing protocols | EventBus `interface/down` | No |
| Ze to VPP | Retained pidfd signal; systemd unit control | No |

### Integration Points
- `VPPManager.runOnce` and `waitVPPConnectionWithStop` - the decision procedure replaces the first-probe kill
- `iface` VPP monitor - publishes `interface/down` on the down verdict
- `ProcessLease.SameProcess` - the adopt proof

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-11-23 | Nothing reacts to VPP loss: no `vpp/disconnected` subscriber, VPP-backed interfaces never go down, peers rely on timers |
| `transient-failure-treated-as-fatal.md` | 2026-11-23 | Managed mode kills VPP on the first failed API probe, against R-3's adopt branch |

## Acceptance Criteria

To be written at design, after the owner choices. Each rule and each verdict
gets a row: an enabled VPP with no method refused at verify; an alive,
forwarding, readable VPP adopted with nothing announced; a crashed or stalled
VPP taken down in Ze's model with IGP and BGP peers updated, killed by pidfd,
restarted by its method and rebuilt.

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
| [design: a real-VPP lab freezes VPP (SIGSTOP) and kills it, and asserts the adopt and down verdicts and the IGP and BGP reaction] | `internal/le/test/deployment/` | [design] | |

## Files to Modify

To be derived at design. Known: `internal/component/vpp/vpp.go`,
`conn_health.go`, `config.go`, `yang/ze-vpp-conf.yang`,
`internal/core/vpp/events/events.go`, `internal/plugins/iface/vpp/`, the BGP
reactor's interface reaction, BFD, the ten repository users of
`external true`, and the pages in "Pages this work makes wrong".

## Implementation Steps

To be written at design. Phase 1 is wiring: the restart method in YANG and
verify, and the verdict events with their subscribers.

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
