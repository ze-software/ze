# Spec: Redistribution chain drops every route silently

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Depends | - |
| Phase | research complete, design drafted, design gate not yet answered |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

<!-- Scope is `plugin`, not `protocol`: no RFC obligation changes here. The
     OSPFv3 external and NSSA origination code and the IS-IS TLV 135 code are
     already conformant and were read at the producer. They are never reached,
     because the route never arrives. The change is wire-VISIBLE (FRR learns a
     prefix it does not learn today), so the Interop Tests table is filled even
     though the scope row does not demand it. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A configured `redistribute` block moves no route into an IGP, and says nothing
about it. Three interop scenarios are red on the same symptom while their
adjacency assertions are green: `ospfv3-redist-frr`, `ospfv3-nssa-redist-frr`
and `isis-redist-frr`. The prefix never reaches FRR.

The chain from an operator's `redistribute` block to a route on the wire has
four stages, and three of them return silently when a precondition is unmet.
Measurement on 2026-09-03, with the daemon run at `ze_log=debug` in the interop
image against the scenarios' own `ze.conf`, put each stage at a producing
function:

| # | Stage | What it does today | Evidence |
|---|-------|--------------------|----------|
| 1 | An UPDATE reaches the BGP Loc-RIB | It does not. No peer attaches the `bgp-rib` process, so `onMessageBatchReceived` finds no recipient and returns | `show bgp` reports `updates-received 2`; `show bgp rib status` reports `peers 0 routes-in 0` |
| 2 | The Loc-RIB publishes a best-path change | Never runs: there is no route to select | orchestrator log holds no `processing batch` line with `source=bgp` |
| 3 | The orchestrator dispatches to the consumers | Dispatches only to the consumers registered at the instant of the event, with no replay for one that registers later | a `source=static` batch went to `bgp` alone; the later `source=connected` batch went to `bgp` and `isis` |
| 4 | The IGP consumer originates the LSA or the TLV | Correct, and unreached | read at the producer by the preceding research |

The goal: `redistribute { destination ospf { import bgp } }`, written alone in
a config with a BGP peer and an OSPF instance, moves the peer's routes into
OSPF. The same for `destination isis { import static }` with a static route
present at startup. And where a stage cannot do its work, the daemon refuses or
says so, rather than returning a silent zero (`ai/rules/principles.md`).

Two findings carried into this spec from the investigation that commissioned it:

- `initRedistribute` (`internal/component/bgp/config/loader_create.go`) warns
  once and returns on any extraction error, leaving the global evaluator nil,
  which disables EVERY redistribution rule in the config. When the rule list is
  merely empty it sets nothing and says nothing at all.
- `docs/guide/redistribution.md` is titled "Route Filters" and documents route
  filters end to end. It says nothing about the `redistribute` config root. That
  silence is what authorized this investigation.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - the two halves of one delivery edge
  → Constraint: delivery is the OVERLAP of what a peer's config grants a process and what that process declared. Neither half alone delivers anything, and a disagreement is silent by construction.
  → Decision: the config's half is `PeerSettings.ProcessBindings` only. `DeliveryPeersFromSettings` (`internal/component/bgp/reactor/delivery_graph.go`) reads resolved settings and nothing else, so a peer with no `attach process` block grants nothing to anybody.
- [ ] `docs/architecture/core-design.md` - the redistribute orchestrator, the source registry, the consumer registry and the BGP source bridge
  → Constraint: one orchestrator subscribes to every registered producer at startup and fans each batch out over `configredist.ConsumerNames()` read live at event time.
  → Decision: producers are snapshotted once at startup, consumers are read per event. Neither is replayed to a party that arrives late.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - best-path change tracking
  → Constraint: `publishBestChanges` is the single point where a best-path change becomes an event, and it is the only caller of the BGP redistribution bridge.
- [ ] `docs/architecture/config/syntax.md` - reactor creation from the config tree
  → Constraint: `CreateReactorFromTree` is where redistribution rules are installed, so a refusal there refuses the daemon start and the SIGHUP reload alike.
- [ ] `docs/guide/ospf.md` - the OSPF user guide's redistribution section
  → Constraint: the page documents `redistribute { destination ospf { import connected static bgp } }` as sufficient. Today it is not, for the `bgp` source. The page states the behavior this spec must make true.
- [ ] `docs/guide/redistribution.md` - the page a reader reaches for the `redistribute` root
  → Decision: the page covers route FILTERS. The `redistribute` root has no user page, so the operator has no way to learn the plumbing the daemon silently requires.
- [ ] `docs/architecture/testing/interop.md` - the interop lab and the vacuity traps
  → Constraint: a scenario directory is named, never numbered, and a test added to already-working code needs a forced RED before it counts.

### RFC Summaries (Scope: protocol)
Not applicable. No RFC obligation changes. The OSPFv3 AS-External and Type-7
origination path and the IS-IS TLV 135 path are unchanged by this spec, and the
research that commissioned it read both at the producer and found them correct.

**Key insights:** (minimal context to resume after compaction)
- Peer-scoped event delivery has no default. `attach process` is the ONLY thing that grants it, so the built-in Loc-RIB receives no UPDATE unless the operator writes plumbing no page documents.
- The BGP redistribution source is the Loc-RIB's best-path change, so no Loc-RIB means no BGP redistribution, whatever the `redistribute` block says.
- The IGP consumers register in the plugin's `OnStarted` callback. The static plugin and the IS-IS plugin start in the SAME startup tier, so which one runs first is not decided anywhere.
- A replay mechanism already exists (`redistevents.ReplayRequestEvent`), fired on a BGP peer's down-to-up edge only, and answered by the connected, static and as112 producers.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/server/events.go` - `onMessageReceived` and `onMessageBatchReceived` resolve recipients with `PeerScopedProcs` and return with no log when the set is empty and no monitor is attached
- [ ] `internal/component/bgp/reactor/delivery_graph.go` - `DeliveryPeersFromSettings` builds the delivery graph from `PeerSettings.ProcessBindings` alone
- [ ] `internal/component/bgp/reactor/config.go` - the one site that appends a `ProcessBinding` to a peer, from the peer's `attach process` block
- [ ] `internal/component/bgp/config/peers.go` - the graceful-restart obligation check: a peer that declares GR and attaches no route-pushing process is REFUSED, with an error naming what is missing
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - `dispatchStructured` and `handleReceivedStructured`, the RIB's entry from a delivered update event
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `publishBestChanges` emits the best-change batch and calls the redistribution bridge
- [ ] `internal/component/bgp/redistribute/producer.go` - `EmitBestChange` converts a best-change batch into a `redistevents.RouteChangeBatch`; `init` registers the `bgp` producer
- [ ] `internal/component/bgp/redistribute/bgp.go` - registers the `bgp`, `ibgp` and `ebgp` config sources at `init`
- [ ] `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - `run`, `subscribe` and `handleBatch`: producers snapshotted at startup, consumers read per event
- [ ] `internal/component/bgp/plugins/redistribute_egress/replay.go` - `onPeerUp` and `handleReplayBatch`: the existing replay, fired on a BGP peer-up edge and targeted at one peer through the BGP consumer
- [ ] `internal/component/bgp/config/loader_create.go` - `initRedistribute` warns and returns on error, sets nothing on an empty rule list
- [ ] `internal/component/config/loader_redistribute.go` - `ExtractRedistributeRules` returns an error on one unknown source or family name, and nil with no error when the container or the rule list is empty
- [ ] `internal/component/config/redistribute/consumer.go` - `RegisterConsumer`, `ReregisterConsumer`, `ConsumerNames`, `LookupConsumer`
- [ ] `internal/component/config/redistribute/evaluator.go` - `SetGlobal`, `Global`, `Accept`, `HasDestination`
- [ ] `internal/component/plugin/server/delivery_reconcile.go` - `reconcileDelivery` writes one warning for a process no peer attaches, once, at startup
- [ ] `internal/plugins/ospf/register.go` - the OSPF redistribution consumer registers in `OnStarted`
- [ ] `internal/plugins/isis/register.go` - the IS-IS redistribution consumer registers in `OnStarted`
- [ ] `internal/plugins/static/register.go` - the static plugin registers its source at `init` and answers a replay request
- [ ] `internal/le/interoplab/bgp/prepare.go` - the harness appends a CLI user and an SSH server to every scenario `ze.conf`, and starts the ze peer with `start /etc/ze/bgp.conf`
- [ ] `internal/le/interoplab/bgp/checkers.go` - the three failing scenarios' assertion lists
- [ ] `internal/le/interoplab/bgp/check_extras.go` - the `isis-redist-frr` extra assertions
- [ ] `test/interop/scenarios/ospfv3-redist-frr/ze.conf` - declares the `rib` plugin, attaches it to no peer
- [ ] `test/interop/scenarios/ospfv3-nssa-redist-frr/ze.conf` - the same, in an NSSA area
- [ ] `test/interop/scenarios/isis-redist-frr/ze.conf` - no plugin block, a static route, and `destination isis { import connected static bgp }`
- [ ] `test/interop/scenarios/bgp-cluster-list-length-bird/ze.conf` - the counter-example: it declares the `rib` plugin AND attaches it to both peers, and its own comment says the Loc-RIB has nothing to answer without that
- [ ] `docs/guide/redistribution.md` - route filters, and no `redistribute` root
- [ ] `docs/guide/ospf.md` - documents the `redistribute` block as sufficient

**Behavior to preserve:**
- An operator's explicit `attach process` binding decides that peer's delivery. A narrower grant stays narrow, and no duplicate binding is created for a process the operator already named.
- Loop prevention: a source protocol's batch is never dispatched into that same protocol's consumer.
- The evaluator's accept and reject decisions, including the family filter, are unchanged. The measurement confirmed both directions work: `source=static consumer=bgp` was rejected because the config imports only `isis` into `bgp`, and `source=connected consumer=isis` was accepted.
- The existing BGP peer-up replay and its `ReplayID` correlation.
- The hot path cost of an UPDATE for a daemon that configures no redistribution.

**Behavior to change:**
- A `redistribute` rule whose source is BGP wires the Loc-RIB delivery it depends on, so the block works as written and as `docs/guide/ospf.md` documents it.
- A consumer that registers after a producer emitted receives the producer's current route set, rather than nothing.
- A redistribution config that cannot be turned into rules refuses the load, rather than warning once and disabling every rule.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BGP UPDATE arrives on an established session from a peer, carrying an IPv6 unicast NLRI. Format at entry: wire bytes, read by the reactor's per-peer read goroutine.
- A static route is declared in the config file under `static { table default { route <prefix> } }`. Format at entry: the config tree.
- The operator's intent enters as the `redistribute` container of the config tree.

### Transformation Path
1. The reactor reads the UPDATE and enqueues it on the peer's delivery channel (`internal/component/bgp/reactor/reactor_notify.go`).
2. The peer's delivery goroutine calls `OnMessageBatchReceived` (`internal/component/bgp/reactor/peer_run.go`).
3. `onMessageBatchReceived` (`internal/component/bgp/server/events.go`) resolves the recipient processes with `PeerScopedProcs` over the delivery graph, and returns when the set is empty. THIS IS WHERE THE ROUTE IS LOST TODAY.
4. The `bgp-rib` process receives a structured update event, `dispatchStructured` routes it to `handleReceivedStructured`, which inserts the NLRI and collects the affected prefixes (`internal/component/bgp/plugins/rib/rib_structured.go`).
5. `publishBestChanges` (`internal/component/bgp/plugins/rib/rib_bestchange.go`) emits the best-change batch and calls `EmitBestChange`.
6. `EmitBestChange` (`internal/component/bgp/redistribute/producer.go`) converts it to a `redistevents.RouteChangeBatch` under the `bgp` protocol id and emits it on the bus.
7. `handleBatch` (`internal/component/bgp/plugins/redistribute_egress/redistribute.go`) asks the evaluator whether each registered consumer accepts the route, and dispatches the accepted ones.
8. The OSPF consumer (`internal/plugins/ospf/redistribute/consumer.go`) injects the route, and the engine originates an AS-External-LSA, or a Type-7 LSA in an NSSA area.
9. FRR floods and installs the prefix, which is what the interop scenario asserts.

The static path replaces steps 1 to 6 with the static plugin emitting a batch under the `static` protocol id when it applies its config, and joins the same chain at step 7.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ plugin server | peer-scoped event delivery, gated by the delivery graph the peer's `attach process` blocks build | No |
| Plugin server ↔ bgp-rib | DirectBridge structured event, `rpc.StructuredEvent` | No |
| bgp-rib ↔ orchestrator | typed EventBus handle, `redistevents.RouteChangeBatch` value payload | No |
| Orchestrator ↔ IGP consumer | `configredist.RedistConsumer` interface, `RouteEntry` value | No |
| Config tree ↔ evaluator | `ExtractRedistributeRules` then `SetGlobal` | No |

### Integration Points
- `internal/component/bgp/reactor/config.go`, the single site that appends a peer's `ProcessBinding`: the auto-wired Loc-RIB binding is added here, where the operator's own bindings are built, so both halves of the decision sit in one function.
- `internal/component/config/redistribute/consumer.go`, `RegisterConsumer` and `ReregisterConsumer`: the point where a consumer becomes visible, and therefore the point that must trigger a replay.
- `internal/component/bgp/plugins/redistribute_egress/replay.go`, `onPeerUp`: the existing template for firing a replay request and correlating the answer.
- `internal/component/bgp/config/loader_create.go`, `initRedistribute`: the point that installs the evaluator, and the one that must refuse rather than warn.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every scenario that asserts Loc-RIB content and attaches no process is red today for this same reason | `bgp-srv6-frr/ze.conf` attaches no process, and `check_extras.go` asserts `show bgp rib count` at least 1 for it | The delivery graph has a path the reading missed, and stage 1 is not the whole story | Run `INTEROP_SCENARIO=bgp-srv6-frr ./le integration interop` before writing code, and read the verdict | unvalidated |
| A-2 | The auto-wired Loc-RIB binding is enough to deliver an UPDATE to the RIB | Measured the negative only: with a hand-written `attach process rib { receive [ update state refresh ]; }` and an explicit `plugin { internal rib { use bgp-rib; } }` block, `show bgp rib status` reported `peers 1` and `routes-in 0`, and the daemon started a SECOND plugin server holding only `[rib]` | The explicit-plugin path has a defect of its own and the fix is larger than one binding | Phase 1's wiring test, which asserts the RIB holds the prefix with no plugin block and no attach block in the config | unvalidated |
| A-3 | The IS-IS miss is an ordering race and not a second delivery defect | The `source=static` batch was dispatched to `bgp` alone and the later `source=connected` batch to `bgp` and `isis`, in one run, with one evaluator | A replay on consumer registration does not repair `isis-redist-frr` | Phase 3's unit test: register a consumer after a producer emitted, and assert the consumer received the producer's set | unvalidated |
| A-4 | No operator config depends on the Loc-RIB NOT receiving updates | The Loc-RIB is the daemon's own route store; the only cost of feeding it is CPU and memory per UPDATE | A large-table deployment pays a cost it did not ask for | Gate the auto-binding on a redistribution rule that names a BGP source, so a daemon with no such rule is untouched | unvalidated |
| A-5 | `ExtractRedistributeRules` can only error on a name the YANG validator already refuses | `redistribute_source_validate_test.go` shows the `redistribute-source` custom validator runs on the `import` list key at load | Making the error fatal turns a warning into a refused start for a config that loads today | Phase 4's unit test over a config carrying an unknown family name, which the source validator does not cover | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Auto-wiring the Loc-RIB changes the per-UPDATE cost for every peer of a daemon that configures redistribution | `ze-perf` UPDATE throughput drops on a redistribution config | The binding is added only when a rule names a BGP source; measure with `ze-perf` before and after and record both numbers |
| R-2 | A replay fired on consumer registration storms every producer at startup, once per consumer | Startup log shows one replay request per consumer per producer | Fire once per consumer registration, gate it on the evaluator holding a destination that consumer serves, and reuse the existing eviction and TTL from the replay coordinator |
| R-3 | The replay path was built for one BGP peer and correlates through `ReplayID`; a consumer-scoped replay is a second target kind on the same mechanism | `handleReplayBatch` needs a branch on the target kind | Give the replay coordinator one target type with two cases rather than a second coordinator, or emit the consumer replay through the ordinary non-replay path and let `handleBatch` fan out normally |
| R-4 | Making `initRedistribute` fatal refuses a config that starts today | A committed `.conf` or `.ci` fixture fails to load after the change | Grep the fixtures for a `redistribute` block before the change, and run the functional suite in Phase 4 |
| R-5 | The auto-binding collides with the explicit `plugin { internal rib { use bgp-rib; } }` block the two OSPF scenarios already carry, which starts a second plugin server | The wiring test passes without the plugin block and fails with it | Phase 1 asserts BOTH shapes: with the block and without it. If the explicit block is a second defect, it is in scope, because the scenarios carry it |
| R-6 | Two of the three scenarios have never executed their later assertions, so a green run may still hide a defect further down the chain | The scenario turns green on the first run after the fix, with no intermediate failure | Run each scenario after each phase, not only at the end, and record which assertion the run reaches |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every UPDATE on every peer gains a delivery to the Loc-RIB it did not have. A defect there costs CPU and memory per UPDATE on a full table, and a wrong refusal in `initRedistribute` stops a daemon that starts today. |
| How is it reverted? | Single commit revert. No config migration, nothing persisted, and no wire state survives a restart. |
| Who else touches this path? | `plan/spec-review-redistribute-orchestrator.md` (status `design`) covers the producer/consumer registration asymmetry in the same orchestrator and names the same live-versus-snapshot split. `plan/spec-bgp-session-ready-contract.md` touches the peer-up path the replay coordinator hooks. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A config with a BGP peer, an OSPF instance and `redistribute { destination ospf { import bgp } }`, and no `plugin` or `attach process` block | → | the peer's `ProcessBindings` gains the Loc-RIB binding at build time (`internal/component/bgp/reactor/config.go`) | `TestRedistributeBGPSourceWiresLocRIBBinding` |
| An UPDATE delivered to that peer | → | `onMessageBatchReceived` resolves the `bgp-rib` process and delivers (`internal/component/bgp/server/events.go`) | `TestPeerWithAutoWiredLocRIBReceivesUpdate` |
| The Loc-RIB best-path change | → | `EmitBestChange` (`internal/component/bgp/redistribute/producer.go`) | `TestBestChangeReachesRedistributeOrchestrator` |
| A consumer registering after a producer emitted | → | the replay fired from `RegisterConsumer` (`internal/component/config/redistribute/consumer.go`) | `TestLateConsumerReceivesProducerSet` |
| `redistribute` naming an unknown source | → | `initRedistribute` returns the error and `CreateReactorFromTree` fails (`internal/component/bgp/config/loader_create.go`) | `TestUnknownRedistributeSourceRefusesLoad` |
| The whole chain, end to end | → | FRR installs the prefix from OSPFv3 | interop scenario `ospfv3-redist-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config with one BGP peer, an OSPFv3 instance in a normal area, and `redistribute { destination ospf { import bgp { family [ ipv6/unicast ] } } }`, carrying no `plugin` block and no `attach process` block. The peer announces one IPv6 prefix | The daemon originates an OSPFv3 AS-External-LSA carrying that prefix, and a neighbor installs the prefix from OSPFv3 |
| AC-2 | The same config with the area's `area-type` set to `nssa` | The daemon originates a Type-7 NSSA-LSA carrying that prefix, and no Type-5 AS-External-LSA is flooded into the NSSA |
| AC-3 | A config with a static route, an IS-IS instance and `redistribute { destination isis { import static } }`, where the static plugin emits its route set before the IS-IS consumer registers | The static prefix is advertised in the IS-IS LSP as TLV 135 reachability, and a neighbor installs it from IS-IS |
| AC-4 | The same config as AC-1 plus an explicit `attach process rib { receive [ state ]; }` on the peer | The operator's grant is unchanged: exactly one binding for that process, carrying `state` alone, and the daemon reports the delivery disagreement it already reports for a plugin fed less than it declared |
| AC-5 | A config whose `redistribute` block names a source that no component registered | The config load fails with an error naming the offending token and the destination it sits under; the daemon does not start with redistribution silently disabled |
| AC-6 | A config with no `redistribute` block at all, one BGP peer and one OSPF instance | No Loc-RIB binding is added, the peer's delivery set is what the config grants, and the per-UPDATE work is unchanged |
| AC-7 | A daemon whose `redistribute` block names a BGP source, at startup, before any peer establishes | The startup log carries no `no peer attaches it` warning for the Loc-RIB process, because the binding exists |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `redistribute { destination ospf { import bgp } }`, starts ze, and expects the peer's routes in the neighbor's OSPFv3 table | wire UPDATE → delivery graph → bgp-rib → best change → orchestrator → OSPF consumer → AS-External-LSA → FRR | interop `ospfv3-redist-frr` |
| 2 | Does the same inside an NSSA area | the same chain, with `v6OriginateNSSALSA` in place of the external origination | interop `ospfv3-nssa-redist-frr` |
| 3 | Writes `redistribute { destination isis { import static } }` and expects the static prefix in the neighbor's IS-IS route table | config tree → static plugin emit → orchestrator → IS-IS consumer → TLV 135 → FRR | interop `isis-redist-frr` |
| 4 | Mistypes a source name and expects to be told | config tree → `ExtractRedistributeRules` → `initRedistribute` → load failure naming the token | `TestUnknownRedistributeSourceRefusesLoad` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRedistributeBGPSourceWiresLocRIBBinding` | `internal/component/bgp/reactor/config_test.go` | a peer built from a config carrying a BGP-source redistribution rule holds a Loc-RIB `ProcessBinding` granting the update, state and refresh receive tokens | |
| `TestExplicitBindingWins` | `internal/component/bgp/reactor/config_test.go` | AC-4: an operator binding for the same process is kept verbatim and not duplicated | |
| `TestNoRedistributeNoBinding` | `internal/component/bgp/reactor/config_test.go` | AC-6: a config with no redistribution rule gains no binding | |
| `TestPeerWithAutoWiredLocRIBReceivesUpdate` | `internal/component/bgp/server/events_test.go` | the delivery graph built from those settings resolves the Loc-RIB process for an update event in the received direction | |
| `TestBestChangeReachesRedistributeOrchestrator` | `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` | a best-change batch emitted by the bridge arrives at `handleBatch` and is dispatched to a non-BGP consumer the evaluator accepts | |
| `TestLateConsumerReceivesProducerSet` | `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | AC-3: a consumer registered after a producer emitted receives that producer's current set | |
| `TestReplayFiresOncePerConsumer` | `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | R-2: one registration fires one request, and a re-registration of the same consumer does not multiply it | |
| `TestUnknownRedistributeSourceRefusesLoad` | `internal/component/bgp/config/loader_create_test.go` | AC-5: the load fails and the error names the token and its destination | |
| `TestUnknownRedistributeFamilyRefusesLoad` | `internal/component/bgp/config/loader_create_test.go` | A-5: the family name path errors too, and it is not covered by the source validator | |
| `TestEmptyDestinationRefusesLoad` | `internal/component/config/loader_redistribute_test.go` | a `destination` that imports nothing is named rather than silently producing no rule | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| replay request count per consumer registration | 1 per registration | 1 | 0 (a registration that fires nothing leaves the consumer empty) | 2 or more (a duplicate request per producer) |
| `RouteChangeBatch.Entries` in a replayed batch | 0 to the producer's route count | producer's route count | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-bgp-to-ospf-no-plumbing` | `test/plugin/*.ci` | a config with a `redistribute` block and no plugin or attach block loads, and the Loc-RIB holds the peer's route | |
| `redistribute-unknown-source-refused` | `test/plugin/*.ci` | a mistyped source name refuses the load with an error naming the token | |
| `redistribute-late-consumer` | `test/plugin/*.ci` | a consumer registering after a producer emitted holds the producer's routes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospfv3-redist-frr` | `test/interop/scenarios/` | FRR + GoBGP | AC-1: a BGP prefix reaches FRR as an OSPFv3 AS-External in a normal area | red at assertion 4 |
| `ospfv3-nssa-redist-frr` | `test/interop/scenarios/` | FRR + GoBGP | AC-2: the same prefix reaches FRR as a Type-7 in an NSSA | red at assertion 4 |
| `isis-redist-frr` | `test/interop/scenarios/` | FRR | AC-3: a static prefix reaches FRR through IS-IS | red at assertion 2 |

`ospfv3-nssa-redist-frr`'s absence proofs were repaired on 2026-09-03 in commit
`666a43dff`, so its later assertions have never executed. A first green run for
that scenario proves less than a first green run for the other two, and Phase 5
records which assertion each run reaches.

The three scenarios are already red, so the forced-RED walk
(`ai/rules/interop-and-goal-validation.md`) starts from an observed red. The
walk still runs in the other direction: after each scenario turns green, revert
the phase that turned it green, rebuild the image, and confirm the scenario
returns to red at the same assertion.

## Files to Modify
- `internal/component/bgp/reactor/config.go` - add the Loc-RIB `ProcessBinding` when a redistribution rule names a BGP source and the peer names no binding for that process
- `internal/component/bgp/config/loader_create.go` - `initRedistribute` returns its error, and `CreateReactorFromTree` fails on it
- `internal/component/config/loader_redistribute.go` - name the destination that imports nothing rather than returning an empty rule list
- `internal/component/config/redistribute/consumer.go` - a consumer registration announces itself, so a replay can be fired for it
- `internal/component/bgp/plugins/redistribute_egress/replay.go` - fire a replay on consumer registration, and route the answer to that consumer
- `internal/component/bgp/plugins/redistribute_egress/register.go` - subscribe the orchestrator to the consumer-registration signal
- `docs/architecture/core-design.md` - the redistribution section: state that a BGP-source rule wires the Loc-RIB delivery, and that a consumer registration replays
- `docs/architecture/api/architecture.md` - state that one binding is derived from config rather than written by the operator, and why
- `docs/architecture/config/syntax.md` - `redistribute` extraction now refuses rather than warns
- `docs/architecture/plugin/rib-storage-design.md` - name the redistribution consumer of `publishBestChanges`
- `docs/guide/redistribution.md` - the page is titled "Route Filters" and covers no redistribution; give the `redistribute` root its section, or split the filter content into its own page and let this filename carry redistribution
- `docs/guide/ospf.md` - the redistribution paragraph is right about the intent and silent about the plumbing; state that no plumbing is needed
- `docs/guide/isis.md` - the same for the IS-IS redistribution paragraph
- `docs/guide/configuration.md` - the `redistribute` root in the config reference

## Files to Create
- `test/plugin/redistribute-bgp-to-ospf-no-plumbing.ci` - the functional test for the config that needs no plumbing
- `test/plugin/redistribute-unknown-source-refused.ci` - the refusal, with the error text
- `test/plugin/redistribute-late-consumer.ci` - the late consumer receives the producer's set

The three interop scenario directories already exist and their `ze.conf` files
are NOT modified. They are the statement of the defect: a config an operator
would reasonably write, which the daemon must make work.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new leaf. `internal/component/config/redistribute/yang/` already models the `redistribute` root, and the fix removes plumbing rather than adding it |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | The `redistribute-source` validator on the `import` list key already exists and is unchanged |
| CLI commands/flags | No | No new command. The existing `show bgp rib` and `show ospf database` surfaces are what an operator reads |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | Yes | The three `.ci` files under Files to Create |
| Pipe completeness | N-A | No command added |
| Env var registration | No | No new environment leaf |
| Doctor check for runtime dependencies | Yes | A doctor check that reports a `redistribute` rule whose source protocol has no registered producer, or whose destination has no registered consumer, with a diagnostic code in `internal/core/diagnostic/codes.go`. This is the operator's answer to "I configured it and nothing happened", and it is the surface that keeps the silent-zero class from returning |
| Prometheus counters/metrics | Yes | The orchestrator already counts `eventsReceived`, `announcements`, `withdrawals`, `filteredProtocolTotal`, `filteredRuleTotal` and `replayTotal`. Name the replay counter's new consumer-registration case in the metrics doc rather than adding a metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The feature is documented as already working; this makes the documentation true |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` - the `redistribute` root and its refusal behavior |
| 3 | CLI command added/changed? | No | None added |
| 4 | API/RPC added/changed? | No | None added |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - the `attach process` section states that one binding is now derived |
| 6 | Has a user guide page? | Yes | `docs/guide/redistribution.md` (today it covers route filters only), `docs/guide/ospf.md`, `docs/guide/isis.md` |
| 7 | Wire format changed? | No | No encoder changes. The LSAs and TLVs are unchanged; they are simply originated where they were not |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface changes; the consumer-registration signal is internal to `internal/component/config/redistribute` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No `rfc/short/` row changes. The origination code and its claims are untouched |
| 10 | Test infrastructure changed? | No | No runner change. The three interop scenarios and their checkers are unchanged |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - the redistribution row, if it claims a behavior this spec makes true for the first time. Verify the row before editing |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/architecture/api/architecture.md`, `docs/architecture/plugin/rib-storage-design.md` |
| 13 | Route metadata keys added/changed? | No | No metadata key changes |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` - the replay counter's new case |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` and `docs/features/plugins.md` if the doctor check adds a registered check name |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-fixit-redistribution-chain-drops-silently.md` at implementation time. The four design owners the changed files declare are named above: `docs/architecture/core-design.md`, `docs/architecture/api/architecture.md`, `docs/architecture/config/syntax.md`, `docs/architecture/plugin/rib-storage-design.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ospf.md` and `docs/guide/isis.md` each show a `redistribute` example that does not work today. Both are verified against the daemon after Phase 1 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the Loc-RIB is reachable from a plain `redistribute` block
   - Tests: `TestRedistributeBGPSourceWiresLocRIBBinding`, `TestExplicitBindingWins`, `TestNoRedistributeNoBinding`, `TestPeerWithAutoWiredLocRIBReceivesUpdate`
   - Files: `internal/component/bgp/reactor/config.go`
   - Verify: before the change the first test fails because the peer holds no binding. After it, the delivery graph resolves the Loc-RIB for an update in the received direction. Validate A-1 first by running `INTEROP_SCENARIO=bgp-srv6-frr ./le integration interop` and reading the verdict, and validate A-2 and R-5 by running the wiring test against BOTH config shapes: with the explicit `plugin { internal rib { use bgp-rib; } }` block the two OSPF scenarios carry, and without it
2. **Phase: The BGP source reaches the orchestrator** -- close stages 2 and 3 for a BGP-sourced route
   - Tests: `TestBestChangeReachesRedistributeOrchestrator`
   - Files: whatever Phase 1 leaves open between `publishBestChanges` and `handleBatch`
   - Verify: run `INTEROP_SCENARIO=ospfv3-redist-frr ./le integration interop` and record which assertion it reaches. AC-1 and AC-7 are demonstrated here
3. **Phase: A late consumer is not an empty consumer** -- replay on consumer registration
   - Tests: `TestLateConsumerReceivesProducerSet`, `TestReplayFiresOncePerConsumer`
   - Files: `internal/component/config/redistribute/consumer.go`, `internal/component/bgp/plugins/redistribute_egress/replay.go`, `internal/component/bgp/plugins/redistribute_egress/register.go`
   - Verify: run `INTEROP_SCENARIO=isis-redist-frr ./le integration interop`. AC-3 is demonstrated here
4. **Phase: The config refuses what it cannot do** -- close the silent whole-config disable
   - Tests: `TestUnknownRedistributeSourceRefusesLoad`, `TestUnknownRedistributeFamilyRefusesLoad`, `TestEmptyDestinationRefusesLoad`
   - Files: `internal/component/bgp/config/loader_create.go`, `internal/component/config/loader_redistribute.go`
   - Verify: grep the committed `.conf` and `.ci` fixtures for a `redistribute` block first (R-4), then run the functional suite. AC-5 is demonstrated here
5. **Phase: The operator can find out** -- the doctor check and the documentation
   - Tests: the doctor check's unit test and its functional test
   - Files: the owning package's doctor check, `internal/core/diagnostic/codes.go`, and every page in the documentation checklist
   - Verify: `ze doctor` names a redistribution rule whose source has no producer. Every page edited here is checked against the daemon, not against this spec
6. **Phase: Prove it, both ways** -- the forced RED walk and the NSSA scenario
   - Tests: the three interop scenarios
   - Files: none
   - Verify: each scenario green; then revert the phase that turned it green, rebuild the image, confirm red at the same assertion, restore, confirm green. Record the red output. AC-2 is demonstrated here, and its run is the first time that scenario's later assertions execute

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, and AC-4, AC-6 and AC-7 are the ones most likely to be left to the interop run rather than tested directly |
| Feature completeness | All four user stories run end to end, and story 4's error text names the token an operator typed |
| Correctness | The derived binding is added at the same site as an operator's binding, so precedence is decided in one function and not in two |
| Correctness | The replay on consumer registration reaches only that consumer, and does not re-deliver a producer's set to consumers that already hold it |
| Naming | The derived binding names the same process an operator would name, so `ze config graph` and the delivery reconcile report read the same |
| Data flow | The orchestrator stays protocol-agnostic: no BGP or IGP spelling is added to `internal/component/config/redistribute` |
| Rule: `ai/rules/principles.md` | No stage of the chain returns a silent zero after this spec. For each of the four stages, name what it does when its precondition is unmet |
| Rule: `ai/rules/no-layering.md` | The replay on consumer registration REPLACES the "consumers are whoever is registered right now" rule. It does not sit beside it as a second path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The three interop scenarios green | `INTEROP_SCENARIO=<name> ./le integration interop` for each, output read from a file under the session scratch |
| A forced RED recorded for each | the revert-rebuild-run output, pasted into the closure sections |
| No silent stage left | `grep -n "return" ` over the four producing functions, each checked against the "what does it do when its precondition is unmet" row |
| The doctor check reachable | `ze doctor` output on a config whose redistribution source has no producer |
| `docs/guide/redistribution.md` covers the `redistribute` root | `grep -n "destination" docs/guide/redistribution.md` returns the new section |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The refusal path in Phase 4 prints an operator-supplied token. It is a config token rather than wire input, and it goes into an error message, so it is quoted and never used to build a path or a command |
| Resource exhaustion | The replay on consumer registration is operator-triggered at startup and bounded by the consumer count. Confirm the replay coordinator's existing eviction and TTL bound the pending set |
| Fail closed | The Loc-RIB binding is derived, so a defect in the derivation must leave the peer with the operator's own bindings, never with none |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| An interop scenario stays red after its phase | Read the ze container's log at `ze_log=debug` before changing code; the four orchestrator lines partition the remaining failure space |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The delivery graph has no default, and that is the whole defect at stage 1. `attach process` is not a narrowing filter over a default set; it is the only thing that grants anything. A reader who assumes a default reads the code as correct.
- The three-stage silence is one habit repeated: at each stage the "nobody is listening" case and the "nothing to send" case produce the same nothing, so no stage can tell which one it is in.
- The counter-example was already in the tree. `test/interop/scenarios/bgp-cluster-list-length-bird/ze.conf` carries the plugin block AND the attach block, and its own comment says the Loc-RIB has nothing to answer without them. The knowledge existed as a comment on one scenario rather than as behavior or as a page.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derive the Loc-RIB binding from the redistribution rule | Refuse the config and make the operator write the plugin and attach blocks, as the graceful-restart obligation check does for a route-pushing process | GR asks the operator WHICH process pushes routes, and that is a real choice. BGP redistribution has no choice to make: the source is the daemon's own Loc-RIB. Making the operator write plumbing that has one correct value is machinery the problem does not need (`ai/rules/simplicity.md`), and `docs/guide/ospf.md` already documents the derived behavior as the truth |
| Fire a replay when a consumer registers | Have each producer re-emit on a timer, or have the orchestrator cache the last batch per producer | A cache is a second copy of the producers' state and drifts from it (`ai/rules/principles.md`). A timer picks an interval nobody can justify. The replay request already exists, three producers already answer it, and consumer registration is the exact edge that needs it |
| Refuse a config whose redistribution rules cannot be built | Keep the warning and add a metric or a doctor check | A warning that disables every rule in the file is a silent zero with a log line on top. The daemon cannot do what the operator asked, so it says so and stops (`ai/rules/go-standards.md`, fail early). The doctor check is added as well, for the case the config IS valid and no producer exists |
| Leave the three scenario `ze.conf` files unchanged | Add the plugin and attach blocks to each, which would turn them green today | Editing them would make the tests pass and leave every operator's config broken. The scenarios state what an operator writes, and that is what has to work |

## Known Limitations
- The doctor check reports a rule with no producer or no consumer. It cannot report a rule whose routes are all rejected by the evaluator's family filter, because that is a legitimate outcome.
- A consumer that registers, unregisters and re-registers gets a replay each time. That is correct and cheap at startup, and it is not rate limited.
- This spec does not merge with `plan/spec-review-redistribute-orchestrator.md`, which covers the producer side of the same asymmetry: a producer that registers after the orchestrator started is never subscribed. Phase 3 makes the consumer side right and leaves the producer side to that spec.

## RFC Documentation (Scope: protocol)

Not applicable. No MUST or MUST NOT is implemented, changed or newly proven by
this spec. The OSPFv3 external and NSSA origination code, its RFC comments and
its `rfc/short/` rows are untouched.

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### Goal Validation
| Goal | Evidence | Status |
|------|----------|--------|
| A BGP prefix reaches a neighbor as an OSPFv3 AS-External in a normal area | `INTEROP_SCENARIO=ospfv3-redist-frr ./le integration interop` green, with the forced-RED output recorded | red today, at assertion 4 |
| The same prefix reaches a neighbor as a Type-7 in an NSSA | `INTEROP_SCENARIO=ospfv3-nssa-redist-frr ./le integration interop` green, with the forced-RED output recorded. Its absence proofs were repaired on 2026-09-03 in commit `666a43dff` and its later assertions have never executed, so this run is their first | red today, at assertion 4 |
| A static prefix reaches a neighbor through IS-IS | `INTEROP_SCENARIO=isis-redist-frr ./le integration interop` green, with the forced-RED output recorded | red today, at assertion 2 |
| The operator is told when redistribution cannot work | `ze doctor` output on a config whose redistribution source has no producer, and the load failure on an unknown source name | not started |
| The per-UPDATE cost of a daemon with no redistribution is unchanged | `ze-perf` UPDATE throughput before and after, both numbers recorded | not started |

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
