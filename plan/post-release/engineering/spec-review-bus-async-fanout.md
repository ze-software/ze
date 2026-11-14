# Spec: EventBus backpressure, slow subscribers and delivery order

| Field | Value |
|-------|-------|
| Status | deferred |
| Scope | plugin |
| Priority | P3 |
| Release reason | Owner deferral, 2026-11-14: the event bus keeps its current behavior for the first release, because no test run so far has hit any of the four problems this spec describes. No design is chosen. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner Decision

| Date | Decision | What it is not |
|------|----------|----------------|
| 2026-11-14 | The event bus stays as it is for the first release. This spec is postponed and moved from `plan/release/fixes/` to `plan/post-release/engineering/` | It is not a design decision. Thomas said that goroutines handling delivery would be the ideal, and that he could not choose between the options without a visual representation of them. The diagrams below exist so that the choice can be made when the spec is picked up again |

The reason given for the postponement: across all the testing done so far, none
of these problems occurred.

One release spec depended on this one and no longer can:
`plan/release/fixes/spec-bgp-session-write-wedge-plugin-parked-writer.md` named
this spec as the owner of the blocking engine-to-plugin send. That send stays
unchanged for the release, so the bound that spec needs is its own to design, on
the reactor's side.

## Task

The event mechanism itself is sound: one typed publish and subscribe path in the
plugin server, with JSON copies for plugins. Four problems sit around it. Each
was read at its producer on 2026-11-14 (commit 746b18426d) and still occurs.

| # | Problem | What happens today | Who is affected |
|---|---------|--------------------|-----------------|
| 1 | The two directions follow opposite rules | Engine to plugin: a full queue makes the emitter wait, with no time limit. Plugin to engine: a request the engine does not take within one second is dropped, with a log line and no counter. A third rule on the same connection closes it after a write stalled for 30 seconds, and counts the close | A plugin author whose request vanishes; an operator who cannot see the drop |
| 2 | A slow engine subscriber stalls the emitter | Engine subscribers run one after the other on the emitter's goroutine. The redistribution orchestrator is one of them, and for each route of a batch it makes one blocking call into BGP with a 10 second timeout. A batch holds up to 64 routes | The component that emitted the batch waits for every call |
| 3 | Delivery order is unstated | Plugins are queued first and engine subscribers run second, because of one deferred call. No page states the order and no test pins it | Anyone reasoning about which listener sees an event first |
| 4 | The architecture page is wrong | `docs/architecture/core-design.md` draws a standalone Bus component that no longer exists, and "bus" and "hub" each name several unrelated things with no glossary | A reader of the page |

The goal, unchanged: one documented rule for a full queue in each direction with
nothing lost silently; an emitter that does not wait on a slow subscriber's
I/O; a stated and tested delivery order; a correct page with a glossary.

Out of scope, each owned elsewhere:

- The Operational Report Bus, a different mechanism:
  `plan/release/validation/spec-finish-report-bus.md`.
- A repository-wide rename of the bus and hub symbols. This spec fixes the
  diagram and adds a glossary only.
- What the BGP reactor holds while it waits on a plugin:
  `plan/release/fixes/spec-bgp-session-write-wedge-plugin-parked-writer.md`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]: checkboxes are template markers, not progress trackers. -->
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/core.md`, `ai/doors/bgp-engine.md`, `ai/doors/system-plugins.md`
  → Constraint: `pkg/ze` `EventBus` and `pkg/plugin/rpc` are public API. An emit returns the number of plugins that took the event, and the bytes on the wire are a contract with plugins in any language.
  → Constraint: every feature works over both transports. A plugin running in its own process always takes the RPC path, so a change cannot rely on shared memory.
  → Constraint: a goroutine is a long-lived worker on a channel, never one per event.
  → Constraint: for route updates, the cache entry is stored before the event is dispatched and acknowledgements are cumulative, so a forwarding plugin must receive events in order and none may be dropped.
- [ ] `docs/architecture/api/process-protocol.md`
  → Constraint: plugin delivery is JSON over the process protocol. A change to queueing must not change the framing.
- [ ] `docs/architecture/api/ipc_protocol.md`
  → Constraint: requests and answers share one connection and one read loop, which is why a plugin's request cannot simply wait for a free slot: the engine's own answers would wait behind it.
- [ ] `docs/architecture/memory/lifetime-contracts.md`, contract D
  → Constraint: a redistribution batch is on loan for the length of the emit. A subscriber must not keep it. A debug build overwrites released entries with a sentinel, so a late read is visible.
- [ ] `plan/learned/DESIGN-HISTORY.md`, BGP engine, "Backpressure on the forward path is deliberately IMPLICIT"
  → Decision: on the forward path the waiting chain is the signal. A blocked forward blocks the route-server worker, which fills its channel, which pauses the source peer.
- [ ] `plan/learned/DESIGN-HISTORY.md`, Plugin system, the abandoned standalone Bus
  → Constraint: no standalone bus component returns. Delivery stays inside the plugin server.

**Key insights:** the four problems are about rules (what a full queue does),
scheduling (where a slow subscriber's I/O runs), determinism (delivery order)
and documentation. None needs a rewrite of the mechanism. The subscriber
contract already forbids blocking on external I/O, so problem 2 is one
subscriber breaking the rule, and the open question is whether to fix that
subscriber or to change the rule for everyone.

## Current Behavior (MANDATORY)

**Source files read:** each on 2026-11-14.
- [ ] `internal/component/plugin/process/delivery.go`: `Process.Deliver` puts an event on the plugin's 64-slot queue. A full queue makes the caller wait until the plugin's context ends. One long-lived worker per plugin drains the queue in batches, each bounded by `ze.plugin.delivery.timeout` (5 seconds by default).
- [ ] `pkg/plugin/rpc/mux.go`: the read loop offers a plugin's request to a 16-slot queue through `MuxConn.sendRequest`, waits one second, then drops it and logs "request channel full, dropping inbound request". The wait is short on purpose: the same loop carries the answers to the engine's own calls.
- [ ] `internal/component/plugin/server/dispatch.go`: `Server.deliverEvent` validates the event, registers engine dispatch as a deferred call, then queues the event for each matching plugin.
- [ ] `internal/component/plugin/server/engine_event.go`: `engineEventSubscribers.dispatch` copies the handler list and calls each handler in turn on the emitter's goroutine. The contract on `SubscribeEngineEvent` reads "They must not block on external I/O".
- [ ] `internal/plugins/redistribute/redistribute.go`: `handleBatch` is the orchestrator's engine subscriber. It records the batch with the replay coordinator, then offers every entry to every registered consumer.
- [ ] `internal/component/bgp/redistribute/consumer.go`: `BGPConsumer.InjectRoute` and `WithdrawRoute` each make one `UpdateRoute` call with a 10 second timeout.
- [ ] `internal/core/redistevents/pool.go`: `ReleaseBatch` recycles the batch when the emit returns, relying on every subscriber having returned.

**The flow today, engine to plugin and engine to engine:**

```text
producer
   |  Emit(batch)                     (producer may hold its ordering lock)
   v
plugin server: deliverEvent
   |
   |-- 1. for each subscribed plugin: put event on that plugin's queue
   |        queue has 64 slots
   |        queue full  ->  the producer WAITS here, no time limit
   |        each plugin has one worker that drains its queue to the plugin
   |
   |-- 2. then, engine subscribers, one after the other, same goroutine
   |        rib, sysrib, ... (not read for blocking I/O)
   |        redistribution orchestrator:
   |            for each consumer, for each of up to 64 routes:
   |                one blocking call into BGP, up to 10 s each
   |        the producer WAITS for all of it
   v
Emit returns  ->  the batch is recycled
```

**The flow today, plugin to engine:**

```text
plugin
   |  request
   v
engine read loop (also carries answers to the engine's own calls)
   |
   |-- request queue has 16 slots
   |     free slot           ->  taken, handled
   |     full for 1 second   ->  request DROPPED, one log line, no counter
   v
write side of the same connection
   |-- a write stalled for 30 s  ->  connection CLOSED, counted
```

**What is measured today:**

| Event | Counter | Shown by `ze doctor` |
|-------|---------|----------------------|
| A batch a plugin failed to take | `ze_plugin_delivery_failures_total` | Yes |
| A batch that took over 500ms to deliver | `ze_plugin_delivery_slow_batches_total` | Yes |
| A connection closed by the write watchdog | `ze_plugin_write_watchdog_total` | Not checked |
| A producer waiting on a full plugin queue | None | No |
| A plugin request dropped after one second | None | No |

**Behavior to preserve:**
- The typed publish and subscribe API as callers use it.
- JSON delivery to plugins over the process protocol, with unchanged framing.
- No event lost that is not lost today, and none delivered twice.
- Route updates reach a forwarding plugin in order and are never dropped.
- The Operational Report Bus stays a separate mechanism.

**Behavior to change:** none for the first release (Owner Decision above).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-plugin delivery queue with one long-lived worker | `internal/component/plugin/process/delivery.go`, `Process.Deliver`, `deliveryLoop` | Queues each event for a plugin on a 64-slot channel and drains it in batches; a full queue makes the emitter wait | Extends: this is the engine-to-plugin rule. A design documents it and bounds it; it adds no second queue |
| Per-batch delivery timeout | `internal/component/plugin/process/delivery.go`, `deliveryTimeoutFromEnv` | Bounds how long one batch may take to reach a plugin | Extends: the existing bound on a slow plugin |
| Delivery failure and slow-batch counters with a doctor record | `internal/component/plugin/process/manager.go`, `internal/component/plugin/process/doctor_records.go`, `Process.noteBatch` | Counts batches a plugin failed to take or took over 500ms to take, and reports them through `ze doctor` | Extends: a counter for dropped plugin requests joins this family |
| Write watchdog and write deadline with a hook and a counter | `pkg/plugin/rpc/conn.go`, `fireWatchdog`, `SetWriteWatchdogHook` | Closes a connection whose write stalls for 30 seconds and counts the close | Extends: the backstop of the documented rule. Its hook is the pattern a dropped-request counter copies, because `pkg/plugin/rpc` holds no metrics registry |
| Timed drop of a plugin request | `pkg/plugin/rpc/mux.go`, `MuxConn.sendRequest` | Drops a request the engine did not take within one second, with a log line | Extends: gains a hook and a counter. A blocking send in its place would stall the loop that carries the engine's answers |
| Synchronous engine fan-out | `internal/component/plugin/server/engine_event.go`, `engineEventSubscribers.dispatch` | Calls every engine subscriber on the emitter's goroutine, after plugin delivery is queued | Undecided: the design question of this spec (Design Options below) |
| Redistribution batch on loan | `internal/core/redistevents/pool.go`, `ReleaseBatch`; `docs/architecture/memory/lifetime-contracts.md`, contract D | Recycles the batch when the emit returns | Extends: work that outlives the emit copies the entries first, and contract D's page changes with it |
| Drop-oldest fan-out with a counter | `internal/component/iface/resolve.go` | Keeps only the newest event for a slow subscriber and counts the drops | Does not apply: a route event is not latest-wins, so dropping the oldest loses a route change |
| Implicit backpressure on the forward path | `plan/learned/DESIGN-HISTORY.md`, BGP engine | A blocked forward pauses the source peer through the chain of waiting workers | Keep: route updates to a forwarding plugin cannot be dropped |

## Design Options (undecided)

No option is chosen. Thomas's stated preference in principle is that goroutines
handle delivery. Both options below use long-lived workers; they differ in how
much of the system changes.

**Option A: only the slow subscriber gets a worker.**

```text
producer
   |  Emit(batch)
   v
plugin server: deliverEvent                    (unchanged)
   |-- 1. plugins: queued as today
   |-- 2. engine subscribers, same goroutine, as today
   |        redistribution orchestrator:
   |            COPY the batch entries
   |            put the copy on its own queue      ->  returns at once
   v
Emit returns, batch recycled

            orchestrator worker (one, long-lived, in order)
               |  takes a copy from the queue
               |  for each consumer, for each route:
               |      blocking call into BGP
```

**Option B: every engine subscriber gets a worker.**

```text
producer
   |  Emit(batch)
   v
plugin server: deliverEvent
   |-- 1. plugins: queued as today
   |-- 2. for each engine subscriber:
   |        COPY the payload, put it on that subscriber's queue
   v
Emit returns, batch recycled

      worker for rib          worker for sysrib        worker for redistribution
           |                        |                          |
        handler                  handler              blocking calls into BGP
```

| | Option A | Option B |
|---|---|---|
| What changes | One plugin: the redistribution orchestrator | The bus contract, for every producer and every engine subscriber |
| The emitter stops waiting on | Redistribution's calls into BGP | Every engine subscriber |
| Order of events for one subscriber | Kept: one worker, in order | Kept per subscriber; the order between subscribers is no longer fixed |
| What must be reworked | The orchestrator's held-route and replay state, which today assumes it runs inside the emit | Every producer that holds its ordering lock across the emit, the replay coordinator's reliance on immediate re-entry, and each payload's lifetime rule |
| Cost per event | One copy of the batch, for redistribution only | One copy per subscriber, for every event |
| New question it raises | What the orchestrator's queue does when it is full | What each subscriber's queue does when it is full |
| Matches the rule "a handler must not block on external I/O" | Yes: it makes the one violator comply | It removes the rule |

The session that produced this record recommended Option A as the smaller
change. That recommendation is not a decision.

Problems 1, 3 and 4 do not depend on the choice: document "the emitter waits"
as the engine-to-plugin rule, count dropped plugin requests, state the delivery
order with a test, fix the diagram and add the glossary.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A producer calls `Emit` on `ze.EventBus`, which the plugin server implements, with a typed payload: for redistribution, a batch of up to 64 route changes.
- A plugin sends a request line over its connection to the engine.

### Transformation Path
1. `Server.deliverEvent` validates the event and decodes a string payload of a typed event once.
2. It registers engine dispatch as a deferred call.
3. It marshals JSON once and queues the event for each matching plugin through `Process.Deliver`.
4. On return, the deferred call runs each engine subscriber on the emitter's goroutine.
5. The redistribution orchestrator's subscriber offers each entry to each consumer; the BGP consumer makes one blocking call per entry.
6. The emit returns and the producer releases the batch.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Emitter to plugin | JSON over the process protocol, through a 64-slot queue and one worker | [ ] |
| Emitter to engine subscriber | Direct call on the emitter's goroutine | [ ] |
| Plugin to engine | 16-slot request queue, dropped after one second | [ ] |
| Orchestrator to BGP | One `UpdateRoute` call per entry, 10 second timeout | [ ] |

### Integration Points
- `Server.deliverEvent`: the one fan-out point.
- `Process.Deliver`: the engine-to-plugin seam.
- `MuxConn.sendRequest`: the plugin-to-engine seam.
- `handleBatch` in the redistribution orchestrator: the subscriber that blocks.
- The telemetry registry, for any new counter.

### Architectural Verification
- [ ] No bypassed layers (delivery stays inside the plugin server)
- [ ] No unintended coupling (work that outlives the emit copies what it needs)
- [ ] No duplicated functionality (no second queue beside the per-plugin one)
- [ ] Registration over hardcoding (the bus names no subscriber)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The problems do not occur in practice at the first release's scale | Thomas, 2026-11-14: no test run so far has hit them | A slow plugin or a slow BGP call stalls a producer in production, or a plugin request is lost unseen | A burst test with a slow consumer; the slow-batch and failure counters in the field | unvalidated |
| A-2 | A plugin whose request is dropped notices through its own timeout | Not read | The plugin waits forever or acts on a missing answer | Read the SDK caller's timeout path | unvalidated |
| A-3 | No engine subscriber other than redistribution blocks on I/O | Only redistribution's handler was read | Option A leaves another stall in place | Read the rib, sysrib and flowexport handlers | unvalidated |
| A-4 | Replay does not depend on plugins being queued before engine subscribers run | Read in the replay coordinator and in `deliverEvent`, not traced through every producer | Stating the order as a contract changes replay | Trace each producer; an ordering test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A producer waits on a full plugin queue with nothing measuring it | Throughput drops while the slow-batch counter rises | The counters added on 2026-11-11 show the slow plugin; the wait itself stays unmeasured until this spec runs |
| R-2 | A plugin request is dropped and nobody sees it | The log line "request channel full, dropping inbound request" | Search the logs for it; a counter is owed by this spec |
| R-3 | A batch is read after the emit by code moved off the emit path | A debug build shows the poison sentinel | Copy before the hand-off |
| R-4 | Moving subscribers off the emit path reorders events a subscriber relies on | A rib or sysrib functional test changes | One worker per subscriber, in order |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A producer emits a 64-entry batch while the consumer's calls are slow | → | The emit returns without waiting for those calls | `test/plugin/bus-slow-consumer-nonblocking.ci` |
| A plugin sends more requests than the engine's request queue holds | → | Each dropped request is counted | `test/plugin/bus-inbound-overflow-metric.ci` |

## Acceptance Criteria

The criteria are independent of the option chosen.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A slow plugin and a busy engine | One page states what happens to a full queue in each direction and names the write watchdog as the backstop. Every drop and every watchdog close increments a counter. Events to a forwarding plugin are still delivered in order and never dropped |
| AC-2 | A producer emits a 64-entry redistribution batch while the BGP calls are slow | The emit returns in a time that does not depend on those calls, and every route of the batch still reaches BGP, in order |
| AC-3 | An event with both plugin and engine subscribers | The delivery order is stated on the architecture page and a test fails if it changes |
| AC-4 | A reader opens `docs/architecture/core-design.md` | No standalone Bus component is drawn or described, and a glossary separates the EventBus, the plugin delivery path, the hub and the Report Bus |
| AC-5 | A burst producer and a slow consumer under the race detector | No event lost beyond the documented rule, none delivered twice, no data race |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a redistribution burst while BGP answers slowly | producer, emit, the chosen hand-off, calls into BGP | `test/plugin/bus-slow-consumer-nonblocking.ci` |
| 2 | Runs a plugin that floods the engine with requests | read loop, request queue, counter | `test/plugin/bus-inbound-overflow-metric.ci` |
| 3 | Reads the architecture page | `docs/architecture/core-design.md` and its glossary | `./le doc check all` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEmitNonBlockingWithSlowConsumer` | `internal/plugins/redistribute/redistribute_test.go` | The emit returns while the consumer is slow | |
| `TestInboundOverflowCounted` | `pkg/plugin/rpc/mux_test.go` | A dropped request fires the hook | |
| `TestDeliveryOrderingContract` | `internal/component/plugin/server/dispatch_test.go` | Plugins are queued before engine subscribers run | |
| `TestAsyncDrainNoRace` | `internal/plugins/redistribute/redistribute_test.go` | The hand-off is safe under the race detector and keeps order | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Plugin delivery queue | 1 to N | 64 today | 0 | N/A |
| Plugin request queue | 1 to N | 16 today | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-slow-consumer-nonblocking` | `test/plugin/bus-slow-consumer-nonblocking.ci` | A slow consumer does not stall the producer | |
| `bus-inbound-overflow-metric` | `test/plugin/bus-inbound-overflow-metric.ci` | A dropped plugin request is visible | |

### Interop Tests
Not applicable: no peer-facing wire behavior changes. The existing BGP and
redistribution interop scenarios are the regression check.

## Files to Modify

The list depends on the option chosen. Common to both:

- `pkg/plugin/rpc/mux.go`: a hook on the dropped request.
- `internal/component/plugin/process/manager.go`: the counter for it.
- `internal/component/plugin/server/dispatch.go`: the delivery order stated where it is produced.
- `docs/architecture/core-design.md`: the diagram and the glossary.
- `docs/plugin-development/metrics.md`: the new counter, in the existing table.
- `docs/architecture/memory/lifetime-contracts.md`: contract D, if work outlives the emit.

Option A adds `internal/plugins/redistribute/redistribute.go`. Option B adds
`internal/component/plugin/server/engine_event.go` and every producer that
holds a lock across an emit.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema, CLI grammar, completion | N-A: no config or command changes | - |
| Env var | N-A: no new setting is decided | - |
| Prometheus counters | Yes | `internal/component/plugin/process/manager.go`, `docs/plugin-development/metrics.md` |
| Doctor check | Yes: dropped plugin requests join the existing delivery record | `internal/component/plugin/process/doctor_records.go` |
| Functional tests | Yes | `test/plugin/bus-slow-consumer-nonblocking.ci`, `test/plugin/bus-inbound-overflow-metric.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |
| 14 | Prometheus counters added or changed? | Yes | `docs/plugin-development/metrics.md` |
| 16 | Changed source referenced by doc anchors? | Yes | pages anchored on `dispatch.go`, `delivery.go`, `mux.go` |

## Files to Create
- `test/plugin/bus-slow-consumer-nonblocking.ci`
- `test/plugin/bus-inbound-overflow-metric.ci`

## Implementation Steps

Not written: the steps follow from the option chosen. When this spec is picked
up, the design runs through `/ze-spec`, starting from Design Options, and the
evidence is scoped test runs of the plugin, `pkg/plugin` and redistribute
packages and the two named functional tests. No whole-tree verification is run
by an agent.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete, every row with a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] Scoped tests of the changed packages pass under the race detector: `./le test unit pkg path ./internal/component/plugin/server/`, `./le test unit pkg path ./pkg/plugin/rpc/`, `./le test unit pkg path ./internal/plugins/redistribute/`, and the two named tests through `./le test functional plugin`
- [ ] Documentation Update Checklist answered with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the two queue capacities
- [ ] Functional tests for end-to-end behavior

## Work Not Done

| Item | Why | Destination |
|------|-----|-------------|
| All four problems | Owner deferral, 2026-11-14 | This spec, in `plan/post-release/engineering/` |
| The bound on the blocking engine-to-plugin send, as far as a BGP session is concerned | It is a release defect and this spec no longer provides it | `plan/release/fixes/spec-bgp-session-write-wedge-plugin-parked-writer.md` |
| The route-server fast path's time, before and after this spec's code | Owner decision, 2026-11-14; a time figure comes only from Thomas's Mac | "Follow-on measurement" below |

## Follow-on measurement (owner decision, 2026-11-14)

This item follows this spec's code and changes none of its design. Asked about
the time cost of the write budget on the BGP route-server fast path
(`Session.armFastPathBudget`, called from `tryDirectWriteNoFlush` in
`internal/component/bgp/reactor/forward_rs_direct.go`), which was never
measured, Thomas answered: "4 can wait the fanout code". He added, by dictation
that the interface cut off: "However we should [measure] it before and after
the sp...", the word in brackets supplied, read as: measure it before and after
the spec. Asked which work "the fanout code" names, this spec or the BGP
forward fan-out to destination peers
(`plan/release/fixes/spec-bgp-session-write-wedge-fast-path-holds-source.md`),
he answered: "I think it was both."

| Run | What is measured | Against |
|-----|------------------|---------|
| Before, when work on this spec's code starts | The fast path at the HEAD of that day. If the pair around the fast-path fix has not been taken by then, also the fast path at `73fac8af9c~1`, the commit before the write budget | - |
| After, when this spec's code has landed | The fast path at the HEAD of that day | The "before" run |

Thomas takes both on his Mac, with the same host and the same harness. A time
figure comes only from there (`ai/META-DECISIONS.md`, "Test systems and
labs"): a figure from another host is neither reported nor compared.

The expected result is no change, and the measurement is what confirms it.
The fan-out this spec is about is the plugin server handing an event to
plugins and engine subscribers (`Server.deliverEvent`). The session that
deferred this spec reported that the fast path bypasses that delivery. Read at
the producer on 2026-11-14: `tryDirectWriteNoFlush` itself emits no event. The
writer it calls reports each written UPDATE to the session's message callback
with the sent direction (`Session.writeUpdateBody`, `onMessageReceived`), and
where that callback leads was not traced, so the report stands as reported and
not verified.

The decision is recorded in
`plan/release/validation/spec-perf-rs-fast-path-regression-and-pass-labels.md`,
"The write budget's time figure".

## History

| Date | Event |
|------|-------|
| 2026-07-06 | Written from a design review's finding on the bus, as a release fix |
| 2026-07-10 | Corrected: a third rule, the write watchdog, had landed on the same connection |
| 2026-11-11 | Commit ecfbbdfed3 added the delivery failure and slow-batch counters and their doctor record |
| 2026-11-14 | Disposition at 746b18426d: all four problems still occur; the July design no longer fits, because the blocking calls moved into the redistribution orchestrator, the batch became borrow-only with no way to keep it, and a drop rule in both directions contradicts the recorded forward-path backpressure (commit ac90f6226b) |
| 2026-11-14 | Postponed by Thomas and moved to `plan/post-release/engineering/`, rewritten to describe the system as it is, with no design chosen |
| 2026-11-14 | Thomas asked for the route-server fast path's time to be measured before and after this spec's code ("Follow-on measurement") |
