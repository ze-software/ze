# Spec: fixit-link-event-drop-loses-carrier-state

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 1/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Carrier events can be lost, and nothing ever notices.

The `EventUp` and `EventDown` subscribers in `internal/component/iface/register.go`
push into `linkEventCh`, a 16-deep channel, with a `default:` branch that drops
the event when the buffer is full. The worker draining that channel takes
`dhcpMu`, and the config-apply callback holds `dhcpMu` across `reconcileDHCP`
while it stops and starts DHCP clients. A commit is therefore exactly the moment
the queue backs up and starts dropping.

A dropped event is unrecoverable. The route handlers are idempotent by
`routeMetricState`, which is what makes them safe against duplicate events and
what makes them helpless against a missing one: after an applied DOWN, a dropped
UP leaves the DHCP default route at `base + 1024` forever. Nothing reads live
carrier state after an apply, so nothing repairs it. The only self-healing path
is a DHCP lease event resetting the state to unknown.

The drop is invisible: no log line, no counter, no doctor check. The single
operator-visible symptom is a default route sitting at a deprioritized metric
with the link up.

The same defect has a second half. Event handlers run synchronously on the
emitter's goroutine, which is the netlink monitor's own read loop, and the
`EventRouterDiscovered` / `EventRouterLost` subscribers take `dhcpMu` inline
rather than handing off to a worker. During a commit the monitor's read loop
therefore blocks, its 64-deep subscription channels stop draining, and the
kernel-side queue is what overflows next. The resolver's `onLinkEvent` does a
backend `GetInterface` call inline on the same goroutine.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/netlink-monitor.md` - what the monitor promises its subscribers
  → Constraint: the page documents the subscribe API and names no drop policy, so any lossy behavior added here is undocumented and must become documented or must stop being lossy
- [ ] `docs/architecture/iface/management.md` - how interface state reaches the rest of the daemon
  → Decision: link state is delivered as events, never polled, so a lost event has no second source unless one is added
- [ ] `docs/architecture/core-design.md` - event bus contract
  → Constraint: an event handler MUST NOT block on I/O; the handler runs on the emitter's goroutine, so a lock held across I/O is the same violation as the I/O itself
- [ ] `ai/rules/goroutine-lifecycle.md` - rules for any new goroutine or worker
  → Constraint: a worker started here needs a defined stop path; `linkEventCh` is already closed on shutdown and any new channel follows the same lifecycle

**Key insights:**
- The repository already has the coalescing idiom this needs: `nonBlockingNotify` over cap-1 channels, where the next worker pass absorbs the signal.
- The route handlers are idempotent by design, so a last-state-wins queue is safe: replaying the final state costs nothing and repairs everything.
- The subscriber closure, the channel and the worker all live inside `runEngine`, so no test outside that function can drive them today. Extracting them is a precondition for unit coverage, not an optional cleanup.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/register.go` - `EventUp` / `EventDown` subscribers do a non-blocking send into `linkEventCh` (cap 16) and drop on a full buffer; the link worker takes `dhcpMu` per event; `EventRouterDiscovered` / `EventRouterLost` take `dhcpMu` inline; `reconcileDHCP` runs under `dhcpMu` and does client stop and start; `handleLinkUp` / `handleLinkDown` and their IPv6 twins move the default route by `routeMetricState`; `handleDHCPLeaseEvent` is the only path that resets that state
- [ ] `internal/plugins/iface/netlink/monitor_linux.go` - `run` is the netlink read loop; `handleLinkUpdate` emits created, up and down with no state comparison; `emit` marshals and calls the event bus on the read-loop goroutine; `isLinkUp` decides up from operstate or the up flag
- [ ] `internal/component/plugin/server/engine_event.go` - `EmitEngineEvent` dispatches subscribers synchronously on the caller's goroutine
- [ ] `internal/component/iface/resolve.go` - `onLinkEvent` calls the backend inline on the monitor goroutine and drops its own events with a debug log
- [ ] `internal/component/iface/dispatch.go` - `GetInterface` and `ListInterfaces` are the live-carrier reads available for reconciliation
- [ ] `internal/plugins/iface/netlink/show_linux.go` - `linkToInfo` produces the up or down state a sweep would compare against
- [ ] `internal/component/iface/route_metric_test.go` - the existing metric state machine tests call the handlers directly, bypassing the channel

**Behavior to preserve:**
- Route handlers stay idempotent by `routeMetricState`: repeated events must keep moving the route at most once.
- The event bus contract stays intact: no handler gains blocking I/O.
- The learned metric must keep surviving a link bounce, as the existing tests pin.
- DHCP lease handling keeps resetting the metric state to unknown.

**Behavior to change:**
- A full queue no longer loses an interface's final carrier state.
- The router-discovered and router-lost handlers no longer hold `dhcpMu` on the monitor's read loop.
- A dropped or coalesced event becomes observable rather than silent.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A kernel RTM_NEWLINK or RTM_DELLINK message on the netlink link subscription.
- Format at entry: a netlink link update carrying operstate and flags.

### Transformation Path
1. The netlink library goroutine delivers the update into `linkCh` (cap 64) in `internal/plugins/iface/netlink/monitor_linux.go`
2. `run` reads it and calls `handleLinkUpdate`, which decides up or down with `isLinkUp` and calls `emit`
3. `emit` marshals JSON and publishes on the event bus, still on the read-loop goroutine
4. `EmitEngineEvent` in `internal/component/plugin/server/engine_event.go` invokes every subscriber synchronously
5. The `EventUp` / `EventDown` subscribers in `internal/component/iface/register.go` attempt a non-blocking send into `linkEventCh`; the router subscribers instead take `dhcpMu` inline
6. The link worker drains `linkEventCh`, takes `dhcpMu`, and calls `handleLinkUp` or `handleLinkDown`
7. Those call the backend to remove and add the default route at the chosen metric

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel ↔ netlink library | subscription socket, 64-deep channel | Yes: unchanged, and the read loop no longer blocks in front of it |
| Monitor ↔ event bus | synchronous emit on the read-loop goroutine | Yes: `Subscribe` in `pkg/ze/eventbus.go` states the handler contract, `TestSubscribersHandOffRatherThanApply` and `TestLinkEventHandlerMakesNoBackendCall` hold it |
| Subscriber ↔ worker | `linkEventQueue`, one entry per subject, coalescing | Yes: `TestLinkQueueKeepsFinalStateUnderPressure` |
| Worker ↔ config apply | `dhcpMu`, held across DHCP client stop and start | Yes: unchanged, and only the worker takes it now |
| Component ↔ backend | route add and remove over netlink | Yes: `applyLinkEvent` is the only route caller, on the worker's goroutine |

### Integration Points
- `nonBlockingNotify` and the cap-1 reconcile channels - the existing coalescing idiom to follow
- `iface.ListInterfaces` - the single netlink dump a reconciliation sweep would use
- `handleDHCPLeaseEvent` - the existing state-reset path a sweep must not fight

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every carrier and router event reaches a route call through `linkEventQueue` and `applyLinkEvent`; no subscriber calls a handler |
| No unintended coupling (components stay isolated) | Yes | the queue, the resync and the drop counter are internal to the iface component; no consumer gained an import |
| No duplicated functionality (extends existing, does not recreate) | Yes | the queue reuses `nonBlockingNotify` over a cap-1 wake channel, and the resync reuses the rate tracker's existing 1 Hz dump rather than adding a netlink call |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `pushResync` stores the caller's carrier map by reference, and says the caller must not write to it afterwards |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Coalescing to last-state-wins is safe because the route handlers are idempotent by `routeMetricState` | the handlers and the existing metric tests in `internal/component/iface/route_metric_test.go` | Dropping intermediate transitions could lose a state machine step, and the fix would need every event | A unit test that enqueues down then up while the worker is blocked, and asserts the route ends at the base metric | confirmed: `TestCoalescedUpRestoresBaseMetric` ends at the base metric and also asserts the superseded DOWN reached no route call at all |
| A-2 | A blocking send would stall the monitor's read loop and time out a commit through the address-settlement path | the settlement rule in the iface operation path depends on address events arriving within its window | Blocking would be the simplest fix and this spec is over-built | A test that blocks the worker and asserts the address settlement still completes under the coalescing design | confirmed by a stronger route than the one planned: `Subscribe` in `pkg/ze/eventbus.go` states that a handler MUST NOT block on I/O, so a blocking send is refused by contract rather than by its consequences. `TestSubscribersHandOffRatherThanApply` fails inside 5s against a subscriber that does the work itself |
| A-3 | The overflow is reachable in practice during a commit, not only in theory | `reconcileDHCP` does client stop and start under the lock; the queue is 16 deep | The severity is lower and the fix can be limited to observability | A test or QEMU scenario that flaps enough interfaces during an apply to overflow the queue, with the drop counter proving it | confirmed in a unit test, NOT yet on a running daemon. `TestLinkQueueKeepsFinalStateUnderPressure` blocks the worker the way a commit does and pushes 65 subjects, and restoring the 16-deep bound plus the drop makes it red at `require.Len(final, 65)`. The daemon-level half is what the missing functional test owes |
| A-4 | The router-discovered and router-lost handlers can hop to a worker without changing IPv6 route behavior | they perform the same class of work as the link handlers, under the same lock | The hop reorders IPv6 route moves against DHCP events | The IPv6 metric tests must stay green with the handlers moved behind a queue | confirmed: the IPv6 metric tests in `internal/component/iface/route_metric_test.go` are green with the handlers behind the queue, and `TestLinkQueueAppliesKeysInArrivalOrder` pins that coalescing changes the VALUE at a position and never the position, so a router event that arrived before a carrier event is still applied before it |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Extracting the subscriber, channel and worker out of `runEngine` touches a large closure and may change startup ordering | Startup logs change order, or the monitor starts before the worker | Extract with no behavior change first, prove it with the existing tests, then change the queue |
| R-2 | A coalescing map keyed by interface name grows without bound if names churn | Memory growth on a box creating and destroying many interfaces | Delete the key when the worker consumes it; the map holds at most one entry per pending interface |
| R-3 | A post-apply sweep, if added, fights the DHCP lease path and moves a route the lease handler just placed | A route metric flaps immediately after a lease renewal | Sweep only interfaces whose acted-on state disagrees with live carrier, and take the same lock the lease handler takes |
| R-4 | Moving the router handlers off the emitter goroutine changes the ordering between an IPv6 router event and a config apply | An IPv6 default route is placed against a router that the apply has just removed | The worker takes the same lock, so the apply and the handler stay mutually exclusive; only the arrival order changes |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The default route metric on every DHCP interface, and IPv6 router-derived routes. A regression sends traffic out a dead link or parks it on a live one |
| How is it reverted? | Single commit revert; no persisted state, no config surface |
| Who else touches this path? | `plan/journal/state-event-emitted-without-comparing-state.md` records the duplicate-emit fix that made the consumers idempotent, which is the decision this spec builds on. The resolver subscribes to the same events |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A carrier down then up arrives while the worker is blocked | → | the link event queue in `internal/component/iface/register.go` | `TestLinkQueueKeepsFinalStateUnderPressure` |
| The worker drains a coalesced entry | → | `handleLinkUp` in `internal/component/iface/register.go` | `TestCoalescedUpRestoresBaseMetric` |
| An IPv6 router event arrives during a config apply | → | the router event worker | `TestRouterEventDoesNotBlockTheMonitorLoop` |
| A queue entry is coalesced or dropped | → | the new counter | `TestLinkQueueCoalesceCounted` |
| A real link flaps on a running daemon during a commit | → | the whole chain to the kernel route table | `test/plugin/iface-link-flap-during-commit.ci` |
| A real link flaps against the kernel with the drain driven by the test | → | the monitor, the subscribers and `applyLinkEvent` | `internal/component/iface/link_flap_integration_linux_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A down and then an up for one interface arrive while the worker is blocked on `dhcpMu` | The worker acts on the final state; the default route ends at the base metric |
| AC-2 | More carrier transitions arrive than the queue can hold | No interface's final state is lost; per-interface last-state-wins |
| AC-3 | An event is coalesced or discarded | A counter records it and a log line names the interface, so the condition is observable |
| AC-4 | An IPv6 router-discovered or router-lost event arrives during a config apply | The monitor's read loop does not block for the duration of the apply |
| AC-5 | A link flaps during a commit on a running daemon | After the commit the default route metric matches the live carrier state, AND the event path alone carried it there: `ze_iface_carrier_resyncs_total` for the interface never moves. See "AC-5 was rewritten, and why" below |
| AC-6 | The daemon has been running with no carrier events for a long period | Acted-on route metric state and live carrier state agree, verified by the reconciliation path chosen in Phase 3 |

### AC-5 was rewritten, and why

**AC-5's original wording could not fail, and the phase that made it unfailable
was a LATER phase of this same spec.** As written it said only that the route
metric matches live carrier after the commit. Phase 3 then added the carrier
resync: `resyncCarrierState` (`internal/component/iface/link_queue.go`), fed
every second by `SubscribeCollectNotify` (`internal/component/iface/register.go`,
1 Hz ticker in `rate.go`), compares acted-on metric state against live carrier
and repairs a contradiction inside about a second. So the route reaches the
right metric under the DROPPING producer too, roughly a second later.

Measured on the arm64 QEMU VM: against a queue restored to the pre-fix 16-deep
drop-on-full channel, six rounds of 101 carrier transitions during real config
commits ended with the route at the right metric in EVERY round. The only
difference from HEAD was that `ze_iface_carrier_resyncs_total` had risen by 4.
A functional test asserting AC-5's own words would have passed against the
defect it exists to catch.

The fact that separates the two producers is whether the self-heal HAD TO FIRE,
which is what the counter reports, so the criterion now names it and
`test/plugin/iface-link-flap-during-commit.ci` asserts it.

**The general trap, for the next spec that adds a self-heal: every acceptance
criterion written BEFORE the self-heal must be re-read for whether the self-heal
now satisfies it.** Where it does, that criterion's test must also assert the
self-heal did not have to fire. A repair mechanism and the criterion it repairs
are indistinguishable from the outside, and the repair is the newer of the two,
so it is the criterion that has silently changed meaning.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Disables and re-enables an interface inside one commit | config apply → DHCP reconcile → link events → route metric | `test/plugin/iface-link-flap-during-commit.ci` on a running daemon; `internal/component/iface/link_flap_integration_linux_test.go` at the kernel boundary |
| 2 | Pulls and replugs a cable while a commit runs | kernel → monitor → queue → worker → route | `TestLinkQueueKeepsFinalStateUnderPressure` |
| 3 | Looks for evidence that events were lost | counter and log output | `TestLinkQueueCoalesceCounted` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLinkQueueKeepsFinalStateUnderPressure` | `internal/component/iface/link_queue_test.go` | AC-1 and AC-2: last state wins when the queue is saturated | green; red at `require.Len(final, 65)` when `push` is bounded at 16 and drops beyond, which is the pre-fix producer |
| `TestCoalescedUpRestoresBaseMetric` | `internal/component/iface/route_metric_test.go` | the coalesced state reaches the metric state machine and moves the route once | green; the `assert.Empty(fb.routeAdds)` half is what a non-coalescing queue fails, because it applies the superseded DOWN |
| `TestLinkQueueCoalesceCounted` | `internal/component/iface/link_queue_test.go` | AC-3: coalescing increments the counter and logs the interface | green |
| `TestRouterEventDoesNotBlockTheMonitorLoop` | `internal/component/iface/link_queue_test.go` | AC-4: the router handlers hand off instead of locking inline | green, and NOT sufficient on its own: it builds the queue itself and registers no subscriber, so it stays green against a subscriber that applies inline. `TestSubscribersHandOffRatherThanApply` is the one that fails there |
| `TestSubscribersHandOffRatherThanApply` | `internal/component/iface/link_queue_test.go` | AC-4 at the wiring layer: `subscribeLinkEvents` pushes rather than working, proven over a bus that dispatches synchronously | green; red at its 5s deadline when the router subscriber applies inline |
| `TestLinkEventHandlerMakesNoBackendCall` | `internal/component/iface/resolve_test.go` | AC-4: `onLinkEvent` reaches no backend on the emitter's goroutine, and still reaches a deferred mac/match binding | green; red at "made 1 backend calls" with the pre-fix `GetInterface` restored |
| `TestResolverDropIsCounted` | `internal/component/iface/resolve_test.go` | AC-3 on the second discard path: the resolver fan-out drop is counted per logical name | green; red at "counter is -1, want 3" with the counter call removed |
| `TestSubscribePermMACAppeared` | `internal/component/iface/resolve_test.go` | behaviour guard: an appearing device still reaches a deferred mac/match binding after the backend call is gone | green against both producers by design; it is the guard that refused a first fix which reached the binding a tick late |
| `TestLearnedMetricSurvivesTheLinkBounce` | `internal/component/iface/route_metric_test.go` | existing test must stay green: the queue change preserves idempotence | green |
| `TestExtractedWorkerStartsAndStops` | `internal/component/iface/link_queue_test.go` | R-1: the extracted worker keeps its lifecycle, including shutdown | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pending interfaces in the coalescing map | 0 to the interface count | one entry per interface | N/A | N/A: the map cannot exceed one entry per interface |
| deprioritized metric offset | 1024 as configured today | 1024 | N/A | N/A: unchanged by this spec |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-link-flap-during-commit` | `test/plugin/iface-link-flap-during-commit.ci` | a link flaps 101 times while a config commit holds `dhcpMu`, six rounds, and the kernel's IPv6 default route ends at the metric live carrier calls for each time | green. Three assertions per round: the route reaches `254 + 1024`, `ze_iface_link_events_coalesced_total{zeflapv0}` rose (the burst outran the worker, measured 85-100), and `ze_iface_carrier_resyncs_total{zeflapv0}` never moves (no transition was lost). Red-and-green measured 2026-08-22 in the arm64 QEMU VM: against a queue restored to the pre-fix producer the resync counter rose by 4 and the run failed; against HEAD it is 0 and the run passes in 39.9s. Run it with `make ze-qemu-needs-linux-test`, or one test with `make ze-qemu-debug RUN='bin/ze-test-linux-arm64 bgp plugin 308'` |
| `TestIntegrationLinkFlapDuringCommitKeepsTheRouteMetric` | `internal/component/iface/link_flap_integration_linux_test.go` | the same chain from a real kernel carrier to a real kernel route, with the drain driven by the test | green. `//go:build integration && linux`; it does not run ze, which is why the `.ci` above exists |

**AC-5's stated outcome is NOT on its own a discriminating assertion, and the `.ci` above does not rest on it.** AC-6's carrier resync repairs a contradicted route metric within one rate-tracker tick (`resyncCarrierState`, `internal/component/iface/link_queue.go`; the tick is `SubscribeCollectNotify` in `register.go`), so the route reaches the right metric under the dropping producer too, about a second later. The fact that separates the two is whether the self-heal had to fire, which is what `ze_iface_carrier_resyncs_total` reports.

Two bounds were measured while building it, and both constrain the test rather than the product. A SIGHUP over an UNCHANGED config never reaches the iface component's apply, so it takes `dhcpMu` for no time at all and stalls no worker; each round therefore flips a DHCP hostname first. And above roughly 400 transitions the kernel's own netlink socket drops notifications before ze sees them (`LinkSubscribe`, `internal/plugins/iface/netlink/monitor_linux.go`, sends into a 64-deep channel from a goroutine that blocks on it), which would make the resync assertion judge the socket rather than the queue; the driver reads `/proc/net/netlink` and fails the run if it happens.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No wire-visible protocol behavior changes; this is local route metric handling | |

## Files to Modify
- `internal/component/iface/register.go` - extract the subscriber, queue and worker out of `runEngine`; coalesce per interface; move the router handlers behind the worker; count and log a coalesced entry
- `internal/component/iface/resolve.go` - stop calling the backend inline on the monitor goroutine, and count the fan-out drop
- `internal/component/iface/rate.go` - register the new counters alongside the existing iface metric family
- `docs/architecture/iface/management.md` - document the delivery guarantee, for the queue and for the resolver fan-out
- `docs/architecture/iface/logical-name-resolution.md` - the design doc `resolve.go` declares; it describes how an appearing device reaches a mac/match binding, which is what changed to remove the backend call
- `docs/features/interfaces.md` - the design doc `rate.go` declares; it carries the iface counter list
- `docs/plugin-development/metrics.md` - the metric inventory the three iface link-event counters belong in

`docs/architecture/iface/netlink-monitor.md` was named here at design time and is
NOT the right page: it documents the `monitor system netlink` CLI command, whose
sources are `internal/component/iface/cmd/monitor_netlink*.go`. It says nothing
about how the iface monitor plugin delivers events to in-process subscribers.
`docs/architecture/iface/management.md` is that page and carries the guarantee.

## Files to Create
- `internal/component/iface/link_queue.go` - the extracted queue and worker, so a test can drive them
- `internal/component/iface/link_queue_test.go` - unit coverage for the queue
- `internal/component/iface/link_flap_integration_linux_test.go` - kernel-level proof, `needs-linux`
- `test/plugin/iface-link-flap-during-commit.ci` - AC-5 on a running daemon, `needs-linux:caps=net-admin`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface changes; the queue is internal |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | Yes | `internal/component/iface/link_flap_integration_linux_test.go` |
| Pipe completeness | N-A | no new command output |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | N-A | This spec adds no runtime dependency: no binary, no kernel feature, no external service. The row said `Yes` at design time for a different check, one comparing acted-on route metric state against live carrier, and that check is not implementable and is not wanted. Not implementable: acted-on state lives in the daemon's `activeDHCP` and `activeRouters` maps, and `doctorCheckContext` (`internal/component/doctor/registry.go:32`) carries `Tree`, `ConfigDir`, `Plugins` (a `[]zeplugin.PluginConfig`, config rather than a live dispatcher), `Store` and `Platform`, so no registered check can read them. Not wanted: a check re-deriving the same comparison from sysfs carrier and the kernel route table would be a second source of truth racing the first, and it would report every window in which the daemon has not yet converged. The in-daemon `resyncCarrierState` (`internal/component/iface/link_queue.go`) already repairs a contradiction within a second and counts it in `ze_iface_carrier_resyncs_total`, which is what AC-6 is verified by and what `test/plugin/iface-link-flap-during-commit.ci` asserts stays at zero |
| Prometheus counters/metrics | Yes | `ze_iface_link_events_coalesced_total{name}` and `ze_iface_carrier_resyncs_total{name}` register in `bindMetricsRegistry` (`internal/component/iface/rate.go`) beside `ze_iface_owned_devices`, and `ze_iface_resolver_events_dropped_total{name}` joins them for the resolver fan-out drop. The label is `name` on all three: the interface for the first two, the LOGICAL interface for the third. All three are listed in `docs/plugin-development/metrics.md` |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a reliability fix, not a feature |
| 2 | Config syntax changed? | No | no config surface |
| 3 | CLI command added/changed? | No | no command surface |
| 4 | API/RPC added/changed? | No | no API surface |
| 5 | Plugin added/changed? | No | the netlink backend plugin is unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` carries the link handler anchors and describes the metric behavior |
| 7 | Wire format changed? | No | no wire format |
| 8 | Plugin SDK/protocol changed? | No | the event bus contract is unchanged, it is now honored |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC requirement involved |
| 10 | Test infrastructure changed? | No | the new `.ci` needs no runner change and no helper: it uses `option=needs-linux:caps=net-admin`, a tmpfs driver plugin and `$PORT2` telemetry, all of which `docs/architecture/testing/ci-format.md` already documents. `docs/functional-tests.md` lists suites, not tests, and the `plugin` suite is unchanged |
| 11 | Affects daemon comparison? | No | no comparison claim |
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/netlink-monitor.md` gains the delivery guarantee; `docs/architecture/core-design.md` anchors the monitor and registration |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | Yes | the new counter belongs in the iface telemetry documentation |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no new event type; the existing up and down events are unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/configuration.md` anchors the link handlers in `register.go`; `docs/architecture/core-design.md` anchors `register.go` and `monitor_linux.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify the route metric description in `docs/guide/configuration.md` still matches after the change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the queue testable without changing behavior
   - Tests: `TestExtractedWorkerStartsAndStops`, plus every existing iface test staying green
   - Files: `internal/component/iface/link_queue.go`, `internal/component/iface/register.go`
   - Verify: the subscriber, channel and worker are reachable from a test; behavior is unchanged
2. **Phase: prove the loss** -- write the failing test first
   - Tests: `TestLinkQueueKeepsFinalStateUnderPressure`
   - Files: `internal/component/iface/link_queue_test.go`
   - Verify: the test fails today, demonstrating the dropped final state, and validates A-1 and A-3
3. **Phase: coalesce** -- last state wins per interface
   - Tests: `TestCoalescedUpRestoresBaseMetric`, `TestLearnedMetricSurvivesTheLinkBounce`
   - Files: `internal/component/iface/link_queue.go`, `internal/component/iface/register.go`
   - Verify: a saturated queue loses no interface's final state
4. **Phase: unblock the monitor loop** -- router handlers hand off
   - Tests: `TestRouterEventDoesNotBlockTheMonitorLoop`
   - Files: `internal/component/iface/register.go`, `internal/component/iface/resolve.go`
   - Verify: no handler holds `dhcpMu` or calls the backend on the emitter goroutine
5. **Phase: make it observable** -- counter, log line, doctor check
   - Tests: `TestLinkQueueCoalesceCounted`
   - Files: `internal/component/iface/rate.go`, the doctor check and its diagnostic code
   - Verify: a coalesced entry is counted and a disagreement between acted-on and live state is reportable
6. **Phase: functional proof**
   - Tests: `internal/component/iface/link_flap_integration_linux_test.go`
   - Files: the new `.ci`
   - Verify: reverting Phase 3 makes it fail, so the test is not vacuous

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The final state always reaches the worker; idempotence is preserved, not relied on to hide a loss |
| Naming | The counter name follows the existing iface metric naming |
| Data flow | No handler blocks the emitter goroutine; every hand-off is bounded |
| Rule: `ai/rules/goroutine-lifecycle.md` | The extracted worker has a defined start and stop |
| Rule: `ai/rules/evidence.md` | A-2 and A-3 are settled by tests, not by reasoning about the queue depth |
| Registration over hardcoding | The three counters register through `bindMetricsRegistry`, reached by the component's `ConfigureMetrics` hook. There is no doctor check; see the Integration Checklist for why |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Queue testable outside `runEngine` | `internal/component/iface/link_queue_test.go` exists and drives it |
| No silent drop | grep the subscribers: no bare `default:` without a counter increment |
| No inline lock on the emitter goroutine | grep the subscribers for `dhcpMu` taken in a handler body |
| Functional proof | `make ze-qemu-needs-linux-test` runs `test/plugin/iface-link-flap-during-commit.ci` green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | The coalescing map must be bounded by the interface count and must delete consumed keys |
| Denial of service | A rapid carrier flap must not let an unprivileged event source grow memory or starve the worker |
| Fail closed | An unknown carrier state must not silently leave a route at a metric nobody chose |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-1 refuted (coalescing loses a needed transition) | Back to DESIGN: keep every event and make the queue blocking behind a dedicated goroutine instead |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Coalesce per interface, last state wins | Blocking send; a bigger buffer; a post-apply reconciliation sweep | Blocking stalls the monitor read loop and risks timing out a commit through address settlement. A bigger buffer moves the threshold and keeps the loss. Coalescing is lossless for the only thing the consumer uses, the final state, and the repository already uses this idiom for its reconcile wakeups |
| Treat the inline-lock handlers as part of the same defect | Fix only the drop | The drop is a symptom of the reader stalling; leaving the stall in place means the kernel-side queue overflows next, one layer further away from anything that can report it |
| Add counters, and no doctor check | Fix the loss silently; or add a doctor check as well | A failure with no signal was undetectable for the lifetime of this defect, so the fix must leave the next occurrence visible. Counters do that from inside the daemon, where the state lives. A doctor check cannot reach that state and would have to re-derive it from the kernel, racing the daemon's own convergence |

## Known Limitations
- Coalescing repairs state lost inside this process. It cannot repair state that diverged before the daemon started; a reconciliation sweep is the answer to that and is scoped as AC-6's implementation choice in Phase 5.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- The link event queue, its worker and the subscribers were extracted out of `runEngine` into `internal/component/iface/link_queue.go` and made coalescing per subject, last state wins (`linkEventQueue.push`, `drain`, `start`, `stop`). Committed in `30ce55a41`.
- The router-discovered and router-lost subscribers push into that queue instead of taking `dhcpMu` inline, so no handler holds the config-apply lock on the netlink monitor's read loop (`subscribeLinkEvents`, `applyLinkEvent`). Committed in `30ce55a41`.
- The carrier resync reads live carrier state from the rate tracker's existing 1 Hz dump and repairs a route metric that contradicts it (`resyncCarrierState`, `carrierContradicted`, `carrierFromInterfaces`, fed by `SubscribeCollectNotify` in `register.go`). Committed in `30ce55a41`.
- Three counters register through `bindMetricsRegistry` (`internal/component/iface/rate.go`): `ze_iface_link_events_coalesced_total`, `ze_iface_carrier_resyncs_total`, `ze_iface_resolver_events_dropped_total`.
- The resolver's `onLinkEvent` no longer calls the backend on the emitter's goroutine. `logicalsForLocked` replaced its `devMatchMAC` parameter with an `appearing` flag and reaches a deferred mac/match binding by waking every mac/match name that does not currently know its device. `permMACOf` and `hasPermMACMatches` are gone with their only caller.
- The resolver fan-out now discards the OLDEST buffered event on a full subscriber channel rather than the newest (`sendLatest`, `internal/component/iface/resolve.go`), so no subscriber can be left believing the wrong final state. This landed at closure; see Findings fixed, finding 1.
- `test/plugin/iface-link-flap-during-commit.ci` proves AC-5 on a running daemon. Committed in `182b7a2f3`.

### Bugs Found/Fixed
- **The resolver fan-out could strand a consumer for the life of the process.** Found at closure review. `Sender.onLinkEvent` (`internal/plugins/iface/ra/sender_linux.go`) sets `state.linkDown` on a down, and its timer branch returns without rearming while that flag is set, so the next `up` is the only thing that can restart router advertisements. The fan-out's bare non-blocking send discarded the NEWEST event, so a burst that filled the 8-deep channel discarded exactly that `up`. Covered by `TestResolverKeepsTheFinalStateWhenTheChannelFills` (`internal/component/iface/resolve_test.go`), red against the restored bare send.
- **`test/plugin/iface-link-flap-during-commit.ci` cited the wrong file** for `TestCarrierResyncRepairsAContradictedRouteMetric`. It is in `link_queue_test.go`, not `route_metric_test.go`. Corrected in the header.

### Documentation Updates
- `docs/architecture/iface/management.md`: the section on the resolver fan-out states the guarantee (oldest discarded, never the newest), names the RA sender as the consumer that accumulates, and records that `RescanInterfaces` re-opens and never closes. Anchors added for `resolve.go`, `sender_linux.go`, the IS-IS transport and the LDP register.
- `docs/plugin-development/metrics.md`: the three iface link-event counters joined the inventory table, with a paragraph on how to read them together. Anchor updated to name `sendLatest`.
- `docs/architecture/iface/logical-name-resolution.md`: the "Cache invalidation rides the monitor events" section now states that invalidation runs inside `onLinkEvent` rather than over a subscriber channel, and describes the appearing-device mac/match wake. The "a drop is logged" sentence in "Event reality differs from the older prose" became the real guarantee. Two anchors added.

### Deviations from Plan
- **Phase 5's doctor check was not implemented, and is recorded N-A with its producer named.** `doctorCheckContext` (`internal/component/doctor/registry.go`) carries `Tree`, `ConfigDir`, `Plugins`, `Store` and `Platform`, and no handle to a running daemon, so no registered check can read `activeDHCP` or `activeRouters`. The Integration Checklist row carries the reasoning.
- **`docs/architecture/iface/netlink-monitor.md` was named in Files to Modify at design time and is the wrong page.** It documents the `monitor system netlink` CLI command. `management.md` carries the delivery guarantee and was edited instead.
- **`docs/features/interfaces.md` was named in Files to Modify as the iface counter list and carries no such list.** Its `### Counters` section sits under `## IPv6 Router Advertisements` and is RA-scoped. The metric inventory is `docs/plugin-development/metrics.md`, which was edited.
- **AC-5's wording was corrected during closure.** See "AC-5 was rewritten, and why".

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The resolver fan-out kept its drop, justified by an enumeration of the consumers that recompute on a timer | The enumeration named five consumers and the tree held six, and one of the five it did name accumulates rather than recomputes | Closure review grepped `iface.Subscribe` across `internal/` and found `internal/plugins/iface/ra/sender_linux.go` | Fixed at the producer: `sendLatest` discards the oldest, so the guarantee holds for every subscriber and the next one inherits it without an audit |
| assumption | AC-5's stated observable was taken as a discriminating assertion | A later phase of the same spec added the carrier resync, which reaches the same end state under the dropping producer within about a second | Measured on the arm64 QEMU VM: the pre-fix producer ended every round at the right metric, differing only in `ze_iface_carrier_resyncs_total` | AC-5 rewritten to name the counter, and the `.ci` asserts it |
| escalation | A safety property was asserted in prose over a hand-enumerated consumer set | `ai/rules/evidence.md` already refuses a string that enumerates data a registry holds, and a subscriber registry is one | The enumeration was wrong on the day it was written | Row in `plan/journal/gate-excludes-part-of-its-population.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A full queue no longer loses an interface's final carrier state | Done | `linkEventQueue.push` and `drain`, `internal/component/iface/link_queue.go` | coalesces per `linkEventKey`; `order` fixes position and the map holds the latest value |
| The router handlers no longer hold `dhcpMu` on the monitor's read loop | Done | `subscribeLinkEvents`, `internal/component/iface/link_queue.go` | every subscriber parses and pushes; `applyLinkEvent` is the only route caller and runs on the worker |
| A dropped or coalesced event becomes observable | Done | `countLinkEventCoalesced`, `countCarrierResync`, `countResolverEventDropped`, `internal/component/iface/rate.go` | three CounterVecs registered in `bindMetricsRegistry` |
| The resolver does no I/O on the emitter's goroutine | Done | `resolver.onLinkEvent`, `internal/component/iface/resolve.go` | the `GetInterface` call and `hasPermMACMatches` are gone; `logicalsForLocked` reaches the same names |
| No consumer is left believing the wrong final state | Done | `sendLatest`, `internal/component/iface/resolve.go` | added at closure; see Findings fixed |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCoalescedUpRestoresBaseMetric` (`route_metric_test.go`) | the superseded DOWN reaches no route call at all |
| AC-2 | Done | `TestLinkQueueKeepsFinalStateUnderPressure` (`link_queue_test.go`) | red at its `require.Len` on the final map against the 16-deep drop-on-full bound |
| AC-3 | Done, on both discard paths | `TestLinkQueueCoalesceCounted` (`link_queue_test.go`), `TestResolverDropIsCounted` (`resolve_test.go`) | queue coalesce and resolver discard each counted and named |
| AC-4 | Done | `TestSubscribersHandOffRatherThanApply` (`link_queue_test.go`), `TestLinkEventHandlerMakesNoBackendCall` (`resolve_test.go`) | `TestRouterEventDoesNotBlockTheMonitorLoop` alone is NOT sufficient: it builds the queue itself and registers no subscriber |
| AC-5 | Done | `test/plugin/iface-link-flap-during-commit.ci` | on a running daemon; three assertions per round, six rounds. Red-and-green measured on the arm64 QEMU VM |
| AC-6 | Done | `TestCarrierResyncRepairsAContradictedRouteMetric` (`link_queue_test.go`) | the spec's TDD table omitted this test; recorded here |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLinkQueueKeepsFinalStateUnderPressure` | Done | `link_queue_test.go` | green |
| `TestCoalescedUpRestoresBaseMetric` | Done | `route_metric_test.go` | green |
| `TestLinkQueueCoalesceCounted` | Done | `link_queue_test.go` | green |
| `TestRouterEventDoesNotBlockTheMonitorLoop` | Done | `link_queue_test.go` | green, and recorded as not sufficient on its own |
| `TestSubscribersHandOffRatherThanApply` | Done | `link_queue_test.go` | green |
| `TestLinkEventHandlerMakesNoBackendCall` | Done | `resolve_test.go` | green |
| `TestResolverDropIsCounted` | Done | `resolve_test.go` | green |
| `TestSubscribePermMACAppeared` | Done | `resolve_test.go` | green |
| `TestLearnedMetricSurvivesTheLinkBounce` | Done | `route_metric_test.go` | green |
| `TestExtractedWorkerStartsAndStops` | Done | `link_queue_test.go` | green |
| `TestResolverKeepsTheFinalStateWhenTheChannelFills` | Changed | `resolve_test.go` | added at closure, not in the plan; see Findings fixed |
| `iface-link-flap-during-commit` | Done | `test/plugin/iface-link-flap-during-commit.ci` | green |
| `TestIntegrationLinkFlapDuringCommitKeepsTheRouteMetric` | Done | `link_flap_integration_linux_test.go` | `integration && linux` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/iface/link_queue.go` | Done | created, `30ce55a41` |
| `internal/component/iface/link_queue_test.go` | Done | created, extended this phase |
| `internal/component/iface/link_flap_integration_linux_test.go` | Done | created, `30ce55a41` |
| `test/plugin/iface-link-flap-during-commit.ci` | Done | created, `182b7a2f3` |
| `internal/component/iface/register.go` | Done | subscribers, worker and resync feed extracted |
| `internal/component/iface/resolve.go` | Done | backend call removed, discard direction fixed |
| `internal/component/iface/rate.go` | Done | three counters registered |
| `docs/architecture/iface/management.md` | Done | delivery guarantee for queue and fan-out |
| `docs/architecture/iface/logical-name-resolution.md` | Done | edited at closure, not by the earlier phase |
| `docs/features/interfaces.md` | Changed | carries no iface counter list; see Deviations |
| `docs/plugin-development/metrics.md` | Done | the three counters joined the inventory |
| `docs/architecture/iface/netlink-monitor.md` | Changed | wrong page; see Deviations |

### Audit Summary
- **Total items:** 36
- **Done:** 32
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A carrier event lost during a commit no longer leaves the default route at the wrong metric forever | functional | `test/plugin/iface-link-flap-during-commit.ci`: six rounds of 101 transitions during real config commits on a running daemon. Green at HEAD in 39.9s; red against the queue restored to the pre-fix producer, with `ze_iface_carrier_resyncs_total` up by 4 |
| The netlink monitor's read loop is not blocked by a handler | unit, at the wiring layer | `TestSubscribersHandOffRatherThanApply` drives `subscribeLinkEvents` over a synchronously dispatching bus while holding the apply lock; it fails inside 5s against an inline handler. `TestLinkEventHandlerMakesNoBackendCall` counts backend calls on the emitter goroutine and wants 0 |
| A discarded or superseded event is observable | functional | the `.ci` scrapes `ze_iface_link_events_coalesced_total{zeflapv0}` and requires it to RISE each round (measured 85-100), so a drain that later keeps up cannot make the run vacuous in silence |
| No subscriber is left believing the wrong final state | unit, with mutation | `TestResolverKeepsTheFinalStateWhenTheChannelFills`: red with `sendLatest` reduced to a bare non-blocking send, "the last event delivered is up, want down". `TestResolverDropIsCounted` stayed GREEN against the same mutant, which is why the new test exists |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| no shard exists | done | the spec metadata reads `Deferral shard: -`, and `ls plan/deferrals/` matches no `*link-event*`. Nothing to remove in commit A |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-link-event-drop-loses-carrier-state-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring and removed-behavior; logic, guard audit and data flow; documentation drift and evidence; ze-style pass over every changed Go file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The diff asserted a safety property it did not prove (`/ze-review` step 14, guard audit 3). Prose and comments said the fan-out's discard costs latency rather than correctness because every consumer recomputes from a full listing on a timer. Two halves were false. The enumeration named five subscribers and `grep -rn iface.Subscribe internal` returns six: the router-advertisement sender was missing, and `Sender.onLinkEvent` accumulates into `state.linkDown` with a timer branch that returns without rearming, so a discarded `up` stopped router advertisements for the life of the process. And `RescanInterfaces` in the IS-IS, OSPF and OSPFv3 transports only re-opens an enabled-but-closed circuit; it reads no carrier and never closes one, so it repairs a lost `up` and not a lost `down` | `internal/component/iface/resolve.go`, `internal/component/iface/rate.go`, `docs/architecture/iface/management.md`, `docs/plugin-development/metrics.md`, `docs/architecture/iface/logical-name-resolution.md` | `sendLatest` discards the OLDEST buffered event rather than the newest, so the state an interface ENDED in always reaches every subscriber. `TestResolverKeepsTheFinalStateWhenTheChannelFills` covers it. Every prose and comment site restated to the guarantee that now holds, and the rescan's real reach recorded |
| 2 | ISSUE | The exported `Subscribe` doc comment stated a weaker contract than the code now honours: on a full channel events are dropped and the consumer should re-Resolve on the next event it does receive | `Subscribe`, `internal/component/iface/resolve.go` | Rewritten to state what a consumer MAY and MUST NOT assume, and to name the counter. `cancel`'s obligation moved onto its own MUST sentence |
| 3 | ISSUE | `docs/architecture/iface/logical-name-resolution.md` is the design doc `resolve.go` declares and was not updated. Its "a drop is logged" sentence predated the counter, and the appearing-device mac/match wake that replaced the backend MAC lookup was described nowhere | `docs/architecture/iface/logical-name-resolution.md` | Both sections rewritten, with anchors |
| 4 | NOTE | `test/plugin/iface-link-flap-during-commit.ci` cited `route_metric_test.go` for a test that lives in `link_queue_test.go` | `test/plugin/iface-link-flap-during-commit.ci` | Citation corrected. A record defect: it earned no extra round |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/iface/link_queue.go` | Yes | `ls -l` reports 14K |
| `internal/component/iface/link_queue_test.go` | Yes | `ls -l` reports 20K |
| `internal/component/iface/link_flap_integration_linux_test.go` | Yes | `ls -l` reports 7.9K |
| `test/plugin/iface-link-flap-during-commit.ci` | Yes | `ls -l` reports 21K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the worker acts on the final state | `grep -rn '^func TestCoalescedUpRestoresBaseMetric'` resolves in `route_metric_test.go`; whole package green |
| AC-2 | no interface's final state is lost | `grep -rn '^func TestLinkQueueKeepsFinalStateUnderPressure'` resolves in `link_queue_test.go`; whole package green |
| AC-3 | a discarded or coalesced event is counted and named | `grep -rn '^func TestLinkQueueCoalesceCounted'` and `'^func TestResolverDropIsCounted'` both resolve |
| AC-4 | no handler blocks the emitter goroutine | `grep -rn '^func TestSubscribersHandOffRatherThanApply'` and `'^func TestLinkEventHandlerMakesNoBackendCall'` both resolve |
| AC-5 | the event path alone carries the route to the right metric | `test/plugin/iface-link-flap-during-commit.ci` expects `OK: 6 rounds of 101 transitions during commits`, and the driver runtime-fails when `ze_iface_carrier_resyncs_total` moves |
| AC-6 | acted-on state and live carrier agree | `grep -rn '^func TestCarrierResyncRepairsAContradictedRouteMetric'` resolves in `link_queue_test.go` |
| all | package green, cache defeated | `GOFLAGS=-count=1 make ze-unit-pkg-test PKG=./internal/component/iface RACE=1` reports `ok github.com/ze-software/ze/internal/component/iface 1.744s` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A real link flaps on a running daemon during a commit | `test/plugin/iface-link-flap-during-commit.ci` | Yes: read the file. It creates a veth pair and a dummy from the config, injects an NTF_ROUTER neighbour, flips a DHCP hostname before each SIGHUP so the reload reaches `reconcileDHCP`, drives 101 transitions through one `ip -batch`, and asserts the kernel route metric, a RISING coalesced counter and a FLAT resync counter each round |
| A carrier down then up while the worker is blocked | unit, `link_queue_test.go` | Yes: `TestLinkQueueKeepsFinalStateUnderPressure` blocks the apply callback the way a commit does |
| A queue entry is coalesced | unit, `link_queue_test.go` | Yes: `TestLinkQueueCoalesceCounted` reads the capturing registry |
| The resolver fan-out overruns a subscriber | unit, `resolve_test.go` | Yes: `TestResolverKeepsTheFinalStateWhenTheChannelFills` and `TestResolverDropIsCounted` |
| A real link flaps against the kernel, drain driven by the test | `link_flap_integration_linux_test.go` | Yes: `//go:build integration && linux`; it does not run ze, which is why the `.ci` exists |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestCoalescedUpRestoresBaseMetric` ends at the base metric and asserts the superseded DOWN reached no route call |
| A-2 | confirmed, by a stronger route than planned | `Subscribe` in `pkg/ze/eventbus.go` states a handler MUST NOT block on I/O, so a blocking send is refused by contract. `TestSubscribersHandOffRatherThanApply` fails inside 5s against an inline handler |
| A-3 | confirmed on a running daemon | `test/plugin/iface-link-flap-during-commit.ci` measured 85-100 coalesced events per round on the arm64 QEMU VM, and reddened against the pre-fix producer |
| A-4 | confirmed | the IPv6 metric tests in `route_metric_test.go` are green with the handlers behind the queue, and `TestLinkQueueAppliesKeysInArrivalOrder` pins that coalescing changes the VALUE at a position, never the position |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Doc 6, user guide: `docs/guide/configuration.md` | Its anchors name `resolve.go -- Resolve, Addresses, osDeviceFor`, `resolve.go -- matchByMAC, deviceMatchMAC`, `register.go -- handleLinkDown, handleLinkUp, handleLinkDownIPv6, handleLinkUpIPv6` and `register.go -- writtenRoutePriorities, suppressRAForConfig, suppressAcceptRaDefrtr, handleRouterDiscovered`. None of those symbols changed behaviour: the handlers are unchanged and now run on the worker rather than the emitter, which the page does not describe | No update needed |
| Doc 12 and 16, internal architecture: `docs/architecture/iface/management.md` | Rewrote the fan-out section against `sendLatest`, `Sender.onLinkEvent`, `RescanInterfaces` and `ldpInterfaceRetry`, each read at the producer | Yes |
| Doc 12, internal architecture: `docs/architecture/iface/logical-name-resolution.md` | The design doc `resolve.go` declares. Cache-invalidation and fan-out sections rewritten; two anchors added | Yes |
| Doc 14, Prometheus counters: `docs/plugin-development/metrics.md` | Three rows added to the inventory table, and a paragraph on reading the three together. Names and labels checked against `bindMetricsRegistry` | Yes |
| Doc 14, counters: `docs/features/interfaces.md` | `grep -n '^## '` puts the `### Counters` section under `## IPv6 Router Advertisements`. The page carries no general iface counter list, so the design-time claim in Files to Modify was wrong | No update needed; recorded in Deviations |
| Doctor check for a runtime dependency | This spec adds no binary, kernel feature, socket, port or external service. The Integration Checklist row names `doctorCheckContext` (`internal/component/doctor/registry.go`) and why the originally planned check cannot be registered | N-A |
| RFC status | No RFC-level protocol behaviour changed. The RA sender's RFC 4861 Section 6.2.4 burst behaviour is UNCHANGED in code; what changed is that a discarded event can no longer prevent it firing | N-A |

## Core Insight

**A safety property asserted over a hand-enumerated consumer set is a claim with
a decay rate.** The resolver's discard was justified in four places by naming
the consumers that recompute on a timer. The enumeration was already wrong when
it was written: it named five subscribers where `grep -rn iface.Subscribe
internal` returns six, and it mischaracterised one of the five it did name. The
fix was not to correct the enumeration. It was to move the guarantee into the
producer, where `sendLatest` holds it for every subscriber that exists and every
one added later, so nothing has to be re-derived when a seventh appears.

The same shape appears twice more in this spec. AC-5's observable stopped being
able to fail because a later phase added a repair for exactly the condition it
asserted. And `RescanInterfaces` was cited as recomputing from a full listing
when it re-opens a closed circuit and reads no carrier at all. In all three the
error is the same: a property was stated about a SET (of consumers, of
producers, of states) that nobody re-derived from the tree.
