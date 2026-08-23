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
| AC-5 | A link flaps during a commit on a running daemon | After the commit the default route metric matches the live carrier state |
| AC-6 | The daemon has been running with no carrier events for a long period | Acted-on route metric state and live carrier state agree, verified by the reconciliation path chosen in Phase 3 |

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
