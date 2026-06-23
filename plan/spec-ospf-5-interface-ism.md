# Spec: ospf-5-interface-ism

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospf-2-wire.md, spec-ospf-4-component-config.md |
| Phase | 10/10 |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - umbrella scope; this child is row "ospf-5". Read "Shared Contracts": "Frame addressing + transport", "Packet receive dispatcher", "Area + interface config model", the canonical Metrics table (this spec OWNS exactly `ze_ospf_interface_up` and `ze_ospf_dr_elections_total`), and the `iface/` Architecture row
4. `docs/research/ospf-implementation-guide.md` - sec 5a (Interface State Machine, lines ~247-270), sec 5b (DR/BDR election, lines ~272-290), sec 7 (Network Types and Interface Model, lines ~470-514), sec 13 traps 11 (Hello E-bit mismatch in stub areas) and 12 (DR stickiness on join, lines ~1488-1495)
5. `plan/spec-ospf-2-wire.md` - Hello packet + common-header codec (Network Mask, HelloInterval, RouterDeadInterval, Options, Router Priority, Designated Router, Backup Designated Router, neighbour list) consumed here
6. `plan/spec-ospf-4-component-config.md` - component, config-resolved interface structs, packet receive dispatcher, events namespace, instance/area scaffolding
7. `plan/learned/931-isis-5-adjacency.md` - the IS-IS sibling (circuit + adjacency FSM); OSPF mirrors the per-interface-runtime + FSM split, the snapshot-API pattern, and the dispatcher-registration pattern, but ISM and NSM are SEPARATE machines (ospf-6 owns the NSM)

## Task

Build the OSPF Interface State Machine (ISM) and the per-interface Hello protocol
for Ze. An OSPF interface is the per-interface runtime object created for each
interface on which OSPF is enabled (config resolved by ospf-4). It drives one
Interface State Machine through the eight RFC 2328 §9.1 states (Down, Loopback,
Waiting, Point-to-Point, DROther, Backup, DR), runs a timer-driven Hello sender
to the AllSPFRouters multicast group, validates and consumes received Hellos to
maintain a per-interface neighbour list at the ISM level, and on broadcast
interfaces runs the RFC 2328 §9.4 two-step DR/BDR election (with the sticky rule)
after the Wait timer or a BackupSeen event.

This spec OWNS the per-interface neighbour list AT THE ISM LEVEL ONLY: it tracks
which neighbours have been heard, whether bidirectional communication exists
(our Router ID echoed in the neighbour's Hello), each neighbour's advertised
Router Priority, and each neighbour's declared DR/BDR, because the §9.4 election
reads exactly those fields. The FULL Neighbor State Machine (Down/Attempt/Init/
2-Way/ExStart/Exchange/Loading/Full, DD exchange, LS Request drain) is ospf-6;
this spec carries each neighbour only as far as the §9.4 election needs (heard,
2-Way-eligible, priority, declared DR/BDR) and hands the 2-Way determination and
the DR/BDR identity to ospf-6, which decides whether to form a full adjacency.

The Hello packet built here carries the interface Network Mask, HelloInterval,
the Options byte (the E-bit MUST be clear on stub/NSSA interfaces and set on
normal areas; the N-bit set on NSSA -- trap 11), Router Priority, RouterDead-
Interval, the elected Designated Router and Backup Designated Router, and the
list of Router IDs from which a valid Hello has recently been heard. The Hello
codec (encode/decode) is provided by ospf-2; this spec is the constructor of the
Hello field values and the consumer of the received Hello for ISM/election
purposes.

The deliverable is: two Ze nodes on a broadcast LAN bring their interfaces up,
exchange Hellos, run the §9.4 election so exactly one DR and one BDR are elected
(stable under reboot of a lower-priority router and not displaced when a higher-
priority router joins later), and reach the 2-Way/DROther/Backup/DR steady state;
two Ze nodes on a point-to-point link bring their interfaces straight to Point-to-
Point with no election. This spec provides the interface-table snapshot API that
`plan/spec-ospf-13-cli-diag-interop.md` renders as `show ip ospf interface`, and
emits ISM state-change and DR-change notifications on the events namespace defined
by `plan/spec-ospf-4-component-config.md` (consumed by ospf-6 NSM AdjOK? and by
ospf-7 Network-LSA origination when this router becomes DR).

This spec covers the ISM, Hello, and DR/BDR election but NOT the Neighbor State
Machine or DD exchange (`plan/spec-ospf-6-neighbor-nsm.md`), NOT Network-LSA or
Router-LSA origination (`plan/spec-ospf-7-lsdb-flooding.md`, which reads the
elected DR and the 2-Way neighbour set from here), and NOT Hello authentication
(`plan/spec-ospf-12-auth.md`, which inserts an AuType verify/sign hook on the
receive/send path).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/research/ospf-implementation-guide.md` sec 5a (Interface State Machine) - the eight states, the events, the broadcast vs point-to-point split
  -> Decision: model the ISM as a pure event-driven FSM (state x event table), one per interface, with the Wait timer and the Hello timer driven by the interface runtime; on point-to-point go straight to Point-to-Point on InterfaceUp, on broadcast go to Waiting and arm the Wait timer for RouterDeadInterval
  -> Constraint: BackupSeen fires when a Hello declaring a non-zero DR or BDR arrives, short-circuiting the Wait timer; NeighborChange (a neighbour entered or left 2-Way) re-runs the election while in DROther/Backup/DR
- [ ] `docs/research/ospf-implementation-guide.md` sec 5b (DR/BDR election) - the two-phase algorithm and the sticky rule
  -> Constraint: elect BDR first then DR; a router already advertising itself as DR stays DR (sticky); priority 0 is ineligible; Router ID breaks ties; if either role changed, fire NeighborChange to every 2-Way neighbour (AdjOK? in ospf-6)
- [ ] `docs/research/ospf-implementation-guide.md` sec 7 (Network Types and Interface Model) - broadcast (DR, AllSPFRouters) vs point-to-point (no DR, AllSPFRouters); loopback (stub host link, no Hellos); per-interface configurables (HelloInterval default 10, RouterDeadInterval default 4x, priority default 1, passive)
  -> Constraint: only broadcast and point-to-point are in scope (umbrella); NBMA/P2MP/virtual-link are future; passive interfaces originate as stub links but send no Hellos and run no election
- [ ] `docs/research/ospf-implementation-guide.md` sec 13 trap 11 (Hello E-bit mismatch in stub areas) and trap 12 (DR stickiness on join) - silent failure modes
  -> Constraint: on stub/NSSA interfaces clear the Options E-bit in the originated Hello and reject a received Hello whose E-bit disagrees with the interface's area-external capability (adjacency never forms with no obvious error otherwise); the sticky election prevents flap when a higher-priority router reboots/joins
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, no-alloc hot path
  -> Constraint: parse only the Hello fields the ISM/election need (Network Mask, HelloInterval, RouterDeadInterval, Options, Router Priority, DR, BDR, neighbour list) via the ospf-2 codec; Hello encode is buffer-first `WriteTo(buf, off) int`; the election runs over value-typed neighbour snapshots, not per-packet allocation
- [ ] `ai/rules/plugin-self-containment.md` - self-contained component
  -> Constraint: interface, ISM, and election code live under `internal/plugins/ospf/iface/`; the `show ip ospf interface` snapshot is produced here and rendered by ospf-13

### RFC Summaries (MUST for protocol work; existing, read before implementation)
- [ ] `rfc/short/rfc2328.md` §7.1-§7.3, §9.1-§9.5, §10.5 (created by ospf-2; this spec adds the ISM/election sections via `/ze-rfc`)
  -> Constraint: §9.1 the eight interface states; §9.2 the interface events; §9.3 the state x event transition table; §9.4 the two-step DR/BDR election with the sticky rule and priority-0 ineligibility; §9.5 sending Hellos (destination, fields); §10.5 receiving Hellos (validation: Network Mask on broadcast, HelloInterval, RouterDeadInterval, Options E-bit, Area ID via the dispatcher)
  -> Constraint: §9.5.1 a router whose priority is 0 still sends Hellos and is heard, but is never elected DR/BDR; the DR/BDR fields in its Hello reflect what it has elected, not a self-claim

**Key insights:** (minimal context to resume after compaction)
- Eight ISM states (Down/Loopback/Waiting/Point-to-Point/DROther/Backup/DR). Point-to-point interfaces skip election and go straight to Point-to-Point on InterfaceUp. Broadcast interfaces go to Waiting, arm the Wait timer for RouterDeadInterval, then run the §9.4 election on WaitTimer or BackupSeen.
- DR/BDR election is two-step: elect BDR (the candidate that declares itself BDR, else highest priority, Router ID tie-break) then elect DR (the candidate that declares itself DR stays -- the STICKY rule -- else the BDR is promoted; re-run BDR if it vacated). Priority 0 is ineligible. This runs over the ISM-level neighbour set (priority >= 1, NSM state >= 2-Way, our Router ID echoed).
- The per-interface neighbour list maintained HERE is ISM-scoped: heard, 2-Way-eligible, advertised priority, declared DR/BDR. The full NSM (Down..Full, DD, LS Request) is ospf-6, which consumes the 2-Way determination and the elected DR identity from here.
- Hello validation on receive (§10.5): Area ID match (done by the ospf-4 dispatcher before dispatch), Network Mask match on broadcast, HelloInterval match, RouterDeadInterval match, Options E-bit match for stub/NSSA. A mismatch drops the Hello (and may count a drop) and never forms a neighbour.
- Hello addressing: sent to AllSPFRouters (224.0.0.5) on both broadcast and point-to-point; the transport (ospf-3) owns the actual destination and the AllDRouters (224.0.0.6) join when DR/BDR. This spec only signals "I am now DR/BDR" so ospf-3 can (re)join the multicast group.
- ISM and election interact with the NSM only through events: NeighborChange (a neighbour entered/left 2-Way) drives a re-election here; a DR/BDR change here fires NeighborChange to ospf-6 (AdjOK?) and lets ospf-7 originate/withdraw the Network-LSA.
- Owns exactly two metrics: `ze_ospf_interface_up{area,interface}` (gauge) and `ze_ospf_dr_elections_total{interface}` (counter). No others.

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey; this child reads ospf-2 and ospf-4 outputs)
- [ ] Ze has no OSPF interface machinery; nothing runs an OSPF ISM or DR election today
  -> Constraint: this is entirely new; nothing to preserve in the OSPF namespace
- [ ] `internal/plugins/ospf/transport/` (ospf-3) exposes a per-interface RX/TX path and the AllSPFRouters/AllDRouters multicast join/leave
  -> Constraint: the interface runtime consumes that path; it does not open raw sockets or join multicast groups itself -- it signals DR/BDR state so ospf-3 joins/leaves 224.0.0.6
- [ ] `internal/plugins/ospf/instance.go` (ospf-4) owns the packet receive dispatcher keyed by the common-header Type field (Shared Contracts "Packet receive dispatcher")
  -> Constraint: this spec registers a Hello (Type 1) handler with the ospf-4 dispatcher; the transport holds no protocol switch and the interface does not classify packet types itself; the dispatcher has already validated version == 2, Area ID, checksum, and auth before handing the Hello over
- [ ] `internal/plugins/ospf/packet/` (ospf-2) decodes the common header and the Hello body and encodes Hellos buffer-first
  -> Constraint: the ISM/election call the codec; they do not parse bytes inline
- [ ] `internal/plugins/ospf/` events namespace (ospf-4) defines the event types this spec emits (ISM state change, DR change)
  -> Constraint: emit ISM/DR changes through that namespace, not a new ad hoc channel

**Behavior to preserve:**
- ospf-2 Hello/common-header codec and ospf-3 transport RX/TX + multicast contracts are consumed unchanged
- ospf-4 component lifecycle (OnConfigure/OnConfigApply/OnStarted) and events namespace unchanged; this spec plugs interfaces into the already-wired component and registers a Hello handler with the already-wired dispatcher
- Other protocols (BGP, LDP, IS-IS) untouched

**Behavior to change:**
- New `internal/plugins/ospf/iface/` package (interface runtime + ISM + Hello + DR/BDR election + ISM-level neighbour list)
- The component starts an OSPF interface per enabled interface and tears it down on interface-down events from the iface EventBus
- New OSPF ISM state-change and DR-change events are emitted (consumed by ospf-6 NSM AdjOK? and ospf-7 Network-LSA origination)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Inbound Hello packets are delivered by the ospf-4 packet receive dispatcher (Shared Contracts "Packet receive dispatcher") to the Hello (Type 1) handler this spec registers; the ospf-3 transport hands `(ifindex, src, payload)` to ospf-4, which validates version/Area ID/checksum/auth and switches on the common-header Type, routing Type 1 here
- The interface's periodic Hello timer fires (every HelloInterval)
- The interface's Wait timer expires (RouterDeadInterval after a broadcast interface came up)
- A neighbour's inactivity timer expires (RouterDeadInterval with no Hello -> neighbour lost -> NeighborChange)
- An interface-up / interface-down notification arrives from the iface EventBus

### Transformation Path
1. **Hello in:** ospf-3 transport -> ospf-4 dispatcher (validate version/Area ID/checksum/auth, switch on Type) -> this spec's registered Hello handler -> Hello codec decode (ospf-2) -> typed Hello (Network Mask, HelloInterval, RouterDeadInterval, Options, Router Priority, DR, BDR, neighbour Router IDs)
2. **Validate (§10.5):** Network Mask match (broadcast only), HelloInterval match, RouterDeadInterval match, Options E-bit match (stub/NSSA); on mismatch drop the Hello, count `ze_ospf_packets_dropped_total{reason}` (owner ospf-3), do not touch the neighbour list
3. **Neighbour update (ISM level):** locate or create the per-interface neighbour record keyed by source Router ID; store advertised priority, declared DR, declared BDR; mark 2-Way if our own Router ID appears in the received neighbour list, else 1-Way; (re)arm the neighbour inactivity timer for the neighbour's advertised RouterDeadInterval; hand the (Router ID, 2-Way, priority, DR, BDR) to ospf-6 NSM via the events namespace
4. **ISM events from Hello:** if the Hello declares a non-zero DR or BDR while we are Waiting, raise BackupSeen; if a neighbour entered or left 2-Way, raise NeighborChange
5. **Election (§9.4, broadcast only):** on WaitTimer / BackupSeen / NeighborChange run the two-step election over the eligible neighbour set (priority >= 1, 2-Way, plus self): elect BDR, then elect DR with the sticky rule; if (DR, BDR) changed from the prior result, increment `ze_ospf_dr_elections_total{interface}`, fire NeighborChange to every 2-Way neighbour (ospf-6 AdjOK?), emit a DR-change event (ospf-7 Network-LSA), and signal ospf-3 to (re)join/leave AllDRouters
6. **ISM transition:** move to DR / Backup / DROther (broadcast) or Point-to-Point (p2p) and set `ze_ospf_interface_up{area,interface}` = 1 when operational, 0 on Down/Loopback
7. **Hello out:** Hello timer -> build the FULL Hello via the ospf-2 codec carrying the interface Network Mask, HelloInterval, the Options byte (E-bit per area type -- clear on stub/NSSA, N-bit on NSSA), Router Priority, RouterDeadInterval, the current elected DR and BDR, and the list of Router IDs from which a valid Hello was heard within RouterDeadInterval; hand to ospf-12 to sign (AuType), then to the ospf-3 transport which sends to AllSPFRouters (224.0.0.5) with TTL=1. Passive interfaces skip this step
8. **Teardown:** Wait/Hello timers fire on the interface goroutine; on InterfaceDown (link down or OSPF disabled) tear down every neighbour on the interface, emit NSM KillNbr to ospf-6, go to Down, set `ze_ospf_interface_up` = 0

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Transport <-> interface | per-interface RX (frames in via the ospf-4 dispatcher) and TX (Hello out) from ospf-3; DR/BDR signal so ospf-3 joins/leaves AllDRouters | [ ] |
| Dispatcher <-> ISM | ospf-4 dispatcher routes Hello (Type 1) to this spec's registered Hello handler | [ ] |
| Codec <-> ISM | typed Hello structs from the ospf-2 packet codec; buffer-first Hello encode | [ ] |
| iface EventBus <-> interface | link up/down subscription drives interface enable/teardown | [ ] |
| ISM <-> NSM (ospf-6) | per-Hello (Router ID, 2-Way, priority, DR, BDR) and NeighborChange/DR-change events on the ospf-4 events namespace; ospf-6 owns the full NSM | [ ] |
| ISM <-> LSDB (ospf-7) | DR-change event lets ospf-7 originate/withdraw the Network-LSA; the 2-Way neighbour set + DR identity feed the Router-LSA transit-link encoding | [ ] |
| Interface <-> CLI | interface-table snapshot API consumed by ospf-13 `show ip ospf interface` | [ ] |

### Integration Points
- New `internal/plugins/ospf/iface/` (per-interface RX/TX/timers; ISM FSM; DR/BDR election; ISM-level neighbour list and snapshot API)
- ospf-4 `instance.go` starts/stops interfaces, owns the events namespace, and owns the packet receive dispatcher this spec registers a Hello handler with
- ospf-3 transport supplies the RX/TX path, the iface up/down subscription, and the AllSPFRouters/AllDRouters multicast membership (this spec only signals DR/BDR so ospf-3 (re)joins/leaves 224.0.0.6)
- ospf-2 codec supplies Hello encode/decode
- ospf-6 NSM consumes the per-Hello neighbour state, the 2-Way determination, the elected DR/BDR, and the NeighborChange/DR-change events to drive the full Down..Full machine and the AdjOK? check (downstream, not built here)
- ospf-7 LSDB consumes the DR-change event (Network-LSA origination as DR) and the 2-Way/DR data (Router-LSA transit-link encoding) (downstream, not built here)

### Architectural Verification
- [ ] No bypassed layers (transport -> ospf-4 dispatcher -> registered Hello handler -> codec -> ISM/election -> neighbour table -> events; no inline byte parsing in the ISM and no packet-type switch outside the ospf-4 dispatcher)
- [ ] No unintended coupling (iface does not import the LSDB or SPF; the full NSM, DD exchange, and LS Request drain stay in ospf-6; Network-LSA origination stays in ospf-7)
- [ ] No duplicated functionality (transport RX/TX + multicast and the Hello codec reused, not reimplemented; the dispatcher is the ospf-4 one)
- [ ] Zero-copy preserved where applicable (parse only needed Hello fields; buffer-first Hello encode; election over value snapshots)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ospf-3 exposes a per-interface RX/TX path the interface runtime can read/write without owning the socket, plus a DR/BDR signal to join/leave AllDRouters | umbrella "Frame addressing + transport" (transport behind an interface) | the interface must open its own socket and manage multicast, breaking layering | `internal/plugins/ospf/transport.Transport` implements `iface.Sender`; `TestOSPFDRElectionTwoNodes`; `TestOSPFStopLeavesAllDRouters` | confirmed |
| A-2 | the ospf-4 dispatcher validates version/Area ID/checksum/auth BEFORE handing the Hello to this handler | umbrella "Packet receive dispatcher" | this spec must re-validate the common header, duplicating ospf-4 | `dispatcher.dispatch` verifies header/checksum/area before `handleHello`; `TestOSPFPacketDispatchAreaFilter` | confirmed |
| A-3 | the ospf-2 codec decodes the Hello neighbour list and the DR/BDR/priority/Options fields needed by the §9.4 election | umbrella "LSA inventory" + ospf-2 Hello body | the election has no inputs and cannot run | `TestOSPFHelloOriginationFields`, `TestOSPFHelloTwoWayDetection`, `TestOSPFDRElectionTwoNodes` | confirmed |
| A-4 | a broadcast interface reaches a stable DR/BDR via Hellos alone, before any DD/LSDB exchange (ospf-6/7) | research guide sec 5a/5b; RFC 2328 §9.4 reads only Hello-advertised state | DR election must wait on ospf-6 adjacency, delaying this spec | `TestOSPFDRElectionTwoNodes`, `TestOSPFDRElectionSticky`, `TestOSPFISMBackupSeenRequiresTwoWay` | confirmed |
| A-5 | one ISM-level neighbour record per (interface, source Router ID) is the correct keying on a LAN, and one neighbour per point-to-point interface | research guide sec 5a/7; RFC 2328 §10 | neighbour table mis-keys duplicates on a LAN | neighbour table keyed by `types.RouterID`; `TestOSPFHelloTwoWayDetection`; `TestOSPFNeighborInactivityReElection` | confirmed |
| A-6 | the iface EventBus delivers interface-down promptly enough to tear down the ISM before a stale Network-LSA is originated | umbrella foundation table | the interface lingers Up until a neighbour inactivity timer fires | `TestOSPFCarrierFlapRestoresRunningInterface`; OSPF transport `OnInterfaceDown` callback calls `markInterfaceDownLocked` | confirmed |
| A-7 | passive interfaces (no Hellos, no election) still need an interface record so ospf-7 can advertise the subnet as a stub link | research guide sec 7 (passive originates a stub link) | passive subnets are not advertised | `TestOSPFPassiveInterfaceNoHello`; `TestOSPFPassiveAndLoopbackRecordsDoNotOpenTransport`; `test/ospf/ospf-interface-runtime.ci` | confirmed |
| A-8 | the Options E-bit (and N-bit) the interface advertises and validates is derived from the interface's area type resolved by ospf-4 | umbrella "Area + interface config model"; trap 11 | E-bit mismatch causes silent non-formation on stub areas | `TestOSPFHelloEbitSetNormalClearStub`; `TestOSPFReconcileAreaTypeRefreshesRuntime` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Election non-determinism: two routers disagree on the DR (split-brain) because the eligible set or the sticky rule is applied inconsistently | both routers claim DR; Network-LSAs conflict | implement §9.4 exactly as the two-step BDR-then-DR with sticky; `TestOSPFDRElectionSticky`; compare to FRR in ospf-13 interop |
| R-2 | DR flap on join: a higher-priority router joining a working LAN displaces the DR (trap 12) | DR changes every time a router reboots | sticky rule: a router already advertising itself as DR stays DR; `TestOSPFDRElectionHigherPriorityJoinsNoDisplace` |
| R-3 | Wait timer never fires / fires twice, so election runs early or never on a fresh broadcast interface | interface stuck in Waiting, or election with an empty set | deterministic Wait-timer test with a fake clock; BackupSeen short-circuit tested separately |
| R-4 | E-bit / RouterDeadInterval / HelloInterval / Network-Mask mismatch silently drops Hellos with no diagnostic (trap 11) | neighbour never appears, no log | explicit per-field validation with a structured drop reason + `ze_ospf_packets_dropped_total{reason}`; `TestOSPFHelloMismatch*` |
| R-5 | Neighbour inactivity timer / NeighborChange race causes election churn or a stale neighbour | DR re-elects repeatedly, or a dead neighbour stays in the set | single-writer neighbour table on the interface goroutine; deterministic timer test with a fake clock |
| R-6 | Priority-0 router is wrongly considered eligible (or wrongly excluded from being heard) | a priority-0 router becomes DR, or its Hellos are ignored | §9.4 excludes priority 0 from candidacy but still hears it and lists it; `TestOSPFDRElectionPriorityZeroIneligible` |
| R-7 | Point-to-point interface runs an election (it must not) | a Network-LSA is originated on a p2p link | network-type gate: p2p goes straight to Point-to-Point, no election, no DR/BDR; `TestOSPFPointToPointNoElection` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Hello delivered by the ospf-4 packet dispatcher to the registered Hello handler | -> | ISM updates the neighbour list and (broadcast) runs the election | `TestOSPFInterfaceUp` |
| broadcast interface InterfaceUp, Wait timer expires | -> | §9.4 election elects exactly one DR and one BDR | `TestOSPFDRElectionTwoNodes` |
| point-to-point interface InterfaceUp | -> | ISM goes straight to Point-to-Point, no election | `TestOSPFPointToPointNoElection` |
| two engines on an in-memory broadcast segment | -> | both agree on the same DR/BDR; one is DR, one is Backup | `TestOSPFDRElectionTwoNodes` |
| neighbour inactivity timer expires with no Hello | -> | neighbour removed, NeighborChange raised, election re-runs | `TestOSPFNeighborInactivityReElection` |
| interface-down from iface EventBus | -> | ISM to Down, neighbours torn down, `ze_ospf_interface_up` = 0 | `TestOSPFInterfaceDownTeardown` |
| `show ip ospf interface` snapshot requested | -> | interface-table snapshot API returns state, DR, BDR, priority, timers | `test/ospf/ospf-interface.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Broadcast interface brought up (InterfaceUp), non-zero priority | ISM enters Waiting and arms the Wait timer for RouterDeadInterval; no DR/BDR elected yet |
| AC-2 | Wait timer expires on a broadcast interface | The §9.4 two-step election runs and the interface transitions to DR, Backup, or DROther; `ze_ospf_dr_elections_total{interface}` increments |
| AC-3 | A Hello arrives declaring a non-zero DR or BDR while the interface is Waiting | BackupSeen fires, the Wait timer is short-circuited, and the election runs immediately |
| AC-4 | Point-to-point interface brought up | ISM goes straight to Point-to-Point; no DR/BDR election; no Network-LSA; Hellos sent to AllSPFRouters |
| AC-5 | Two broadcast nodes, priorities 1 and 1, Router IDs differ | Exactly one DR and one BDR are elected; both nodes agree; the higher Router ID is DR (no other claimant) |
| AC-6 | A working LAN with an elected DR; a new router with HIGHER priority joins | The existing DR is NOT displaced (sticky rule, trap 12); the new router becomes DROther |
| AC-7 | A router on the segment advertises Router Priority 0 | It is heard and listed but is never elected DR or BDR (ineligible) |
| AC-8 | A Hello echoes our own Router ID in its neighbour list | The neighbour is marked 2-Way at the ISM level and handed to ospf-6; absence of our Router ID keeps it 1-Way |
| AC-9 | A neighbour's inactivity timer expires (RouterDeadInterval with no Hello) | The neighbour is removed, NeighborChange is raised, and (broadcast) the election re-runs |
| AC-10 | A received Hello whose Network Mask (broadcast), HelloInterval, RouterDeadInterval, or Options E-bit disagrees with the interface | The Hello is dropped (with a structured reason and `ze_ospf_packets_dropped_total{reason}`); no neighbour is formed |
| AC-11 | Periodic Hello timer fires on a non-passive interface | A Hello is built and sent to AllSPFRouters carrying Network Mask, HelloInterval, RouterDeadInterval, Options (E-bit per area type, N-bit on NSSA), Router Priority, the elected DR and BDR, and the list of heard Router IDs |
| AC-12 | Interface is configured passive | No Hellos are sent and no election runs, but an interface record exists so ospf-7 advertises the subnet as a stub link |
| AC-13 | DR or BDR changes as a result of an election | A DR-change event is emitted (consumed by ospf-7 Network-LSA), NeighborChange fires to every 2-Way neighbour (ospf-6 AdjOK?), and ospf-3 is signalled to (re)join/leave AllDRouters |
| AC-14 | Interface is a loopback | ISM enters Loopback; the interface sends no Hellos and runs no election (advertised as a stub host link by ospf-7) |
| AC-15 | Interface-down event from the iface EventBus | The ISM transitions to Down, every neighbour on the interface is torn down (KillNbr to ospf-6), and `ze_ospf_interface_up{area,interface}` is set to 0 |
| AC-16 | `show ip ospf interface` snapshot requested | The snapshot returns per-interface area, state, network type, cost, priority, DR, BDR, HelloInterval, RouterDeadInterval, and neighbour count |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures OSPF on a broadcast LAN of two nodes and expects a DR/BDR | config (ospf-4) -> interface start -> Hello exchange (AllSPFRouters) -> ospf-4 dispatcher -> registered Hello handler -> ISM Waiting -> Wait timer -> §9.4 election -> DR/Backup | `TestOSPFDRElectionTwoNodes`, `test/ospf/ospf-interface.ci` |
| 2 | Adds a third, higher-priority router to the working LAN | new router's Hello -> ISM neighbour update -> NeighborChange -> election with the sticky rule -> existing DR retained, new router DROther | `TestOSPFDRElectionHigherPriorityJoinsNoDisplace` |
| 3 | Configures a point-to-point link between two nodes | config (ospf-4 network-type point-to-point) -> InterfaceUp -> ISM Point-to-Point -> Hellos to AllSPFRouters -> 2-Way, no DR | `TestOSPFPointToPointNoElection`, `test/ospf/ospf-interface.ci` |
| 4 | Misconfigures HelloInterval / RouterDeadInterval / area type (E-bit) on one node | mismatching Hello -> §10.5 validation fails -> drop + structured reason + dropped-metric -> no neighbour | `TestOSPFHelloMismatchHelloInterval`, `TestOSPFHelloEbitMismatchDropped` |
| 5 | Brings the LAN link down on the DR | iface EventBus interface-down -> ISM Down -> neighbours torn down -> NeighborChange on the survivors -> BDR promoted on re-election | `TestOSPFInterfaceDownTeardown`, `test/ospf/ospf-interface.ci` |
| 6 | Runs `show ip ospf interface` | CLI (ospf-13) -> snapshot API -> interface record (state, DR, BDR, priority, timers, neighbour count) | `test/ospf/ospf-interface.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFISMBroadcastUpToWaiting` | `internal/plugins/ospf/iface/ism_test.go` | InterfaceUp on a broadcast, non-zero-priority interface -> Waiting + Wait timer armed | |
| `TestOSPFISMP2PUpToPointToPoint` | `internal/plugins/ospf/iface/ism_test.go` | InterfaceUp on a point-to-point interface -> Point-to-Point, no election | |
| `TestOSPFISMWaitTimerElects` | `internal/plugins/ospf/iface/ism_test.go` | WaitTimer -> election -> DR/Backup/DROther (fake clock) | |
| `TestOSPFISMBackupSeenShortCircuit` | `internal/plugins/ospf/iface/ism_test.go` | a Hello declaring a DR/BDR while Waiting raises BackupSeen and elects before the Wait timer | |
| `TestOSPFISMNeighborChangeReElects` | `internal/plugins/ospf/iface/ism_test.go` | a neighbour entering/leaving 2-Way in DR/Backup/DROther re-runs the election | |
| `TestOSPFISMLoopback` | `internal/plugins/ospf/iface/ism_test.go` | LoopInd -> Loopback (no Hellos, no election); UnloopInd returns toward Down/Up | |
| `TestOSPFISMInterfaceDown` | `internal/plugins/ospf/iface/ism_test.go` | InterfaceDown -> Down, neighbours torn down, `ze_ospf_interface_up` = 0 | |
| `TestOSPFISMTransitionTable` | `internal/plugins/ospf/iface/ism_test.go` | the full state x event table (8 states x events) drives the documented next state | |
| `TestOSPFDRElectionTwoNodes` | `internal/plugins/ospf/iface/election_test.go` | two eligible routers -> one DR, one BDR; both agree; Router ID tie-break | |
| `TestOSPFDRElectionSticky` | `internal/plugins/ospf/iface/election_test.go` | a router already advertising itself as DR is not displaced | |
| `TestOSPFDRElectionHigherPriorityJoinsNoDisplace` | `internal/plugins/ospf/iface/election_test.go` | a higher-priority joiner does NOT take DR from the sitting DR (trap 12) | |
| `TestOSPFDRElectionPriorityZeroIneligible` | `internal/plugins/ospf/iface/election_test.go` | a priority-0 router is heard/listed but never elected DR or BDR | |
| `TestOSPFDRElectionBDRPromotedOnDRLoss` | `internal/plugins/ospf/iface/election_test.go` | DR lost -> BDR promoted to DR, BDR re-elected from the remaining set | |
| `TestOSPFDRElectionCountsMetric` | `internal/plugins/ospf/iface/election_test.go` | a (DR,BDR) change increments `ze_ospf_dr_elections_total{interface}`; an unchanged result does not | |
| `TestOSPFHelloDecodeElectionFields` | `internal/plugins/ospf/iface/hello_test.go` | priority, declared DR, declared BDR, neighbour list decoded from the received Hello drive the neighbour record | |
| `TestOSPFHelloTwoWayDetection` | `internal/plugins/ospf/iface/hello_test.go` | our Router ID echoed in the neighbour list -> 2-Way; absent -> 1-Way | |
| `TestOSPFHelloOriginationFields` | `internal/plugins/ospf/iface/hello_test.go` | originated Hello carries Network Mask, HelloInterval, RouterDeadInterval, Options, priority, elected DR/BDR, heard Router IDs | |
| `TestOSPFHelloEbitSetNormalClearStub` | `internal/plugins/ospf/iface/hello_test.go` | E-bit set on a normal-area interface, clear on stub/NSSA; N-bit set on NSSA (trap 11) | |
| `TestOSPFHelloMismatchNetworkMask` | `internal/plugins/ospf/iface/hello_test.go` | Network-Mask mismatch on broadcast -> drop, no neighbour | |
| `TestOSPFHelloMismatchHelloInterval` | `internal/plugins/ospf/iface/hello_test.go` | HelloInterval mismatch -> drop, no neighbour | |
| `TestOSPFHelloMismatchDeadInterval` | `internal/plugins/ospf/iface/hello_test.go` | RouterDeadInterval mismatch -> drop, no neighbour | |
| `TestOSPFHelloEbitMismatchDropped` | `internal/plugins/ospf/iface/hello_test.go` | Options E-bit disagreement (stub area) -> drop, no neighbour (trap 11) | |
| `TestOSPFHelloPeriodicSend` | `internal/plugins/ospf/iface/hello_test.go` | Hello timer emits a Hello at HelloInterval to AllSPFRouters (fake clock) | |
| `TestOSPFPassiveInterfaceNoHello` | `internal/plugins/ospf/iface/hello_test.go` | a passive interface sends no Hello and runs no election but keeps a record | |
| `TestOSPFNeighborInactivityReElection` | `internal/plugins/ospf/iface/neighbor_test.go` | RouterDeadInterval with no Hello -> neighbour removed, NeighborChange, election re-runs | |
| `TestOSPFNeighborTableLANKeying` | `internal/plugins/ospf/iface/neighbor_test.go` | three LAN neighbours keyed by Router ID; one neighbour per point-to-point interface | |
| `TestOSPFInterfaceSnapshot` | `internal/plugins/ospf/iface/snapshot_test.go` | snapshot returns area, state, network type, cost, priority, DR, BDR, timers, neighbour count | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Router Priority | 0..255 | 255 | N/A (0 = ineligible, valid) | 256 |
| HelloInterval (seconds) | 1..65535 | 65535 | 0 | 65536 |
| RouterDeadInterval (seconds) | 1..65535 | 65535 | 0 | 65536 |
| ISM-level neighbours per broadcast interface | 1..N (cap) | N (cap) | 0 (no neighbours, valid) | N+1 (rejected, resource cap) |

### Functional Tests
<!-- New RPCs/APIs MUST have functional tests -- unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-interface` | `test/ospf/ospf-interface.ci` | two nodes on a broadcast LAN elect a DR/BDR, `show ip ospf interface` shows the state, link-down tears it down and re-elects; a point-to-point pair reaches Point-to-Point with no DR | |

### Interop Tests (MANDATORY for protocol features)
<!-- See ai/rules/interop-and-goal-validation.md. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (deferred) | - | - | FRR `ospfd` interop for DR election and broadcast/P2P adjacency lives in ospf-13 as `ospf-broadcast-frr` / `ospf-p2p-frr`; not duplicated here | |

### Future (if deferring any tests)
- FRR `ospfd` interop (broadcast/DR election, point-to-point, E-bit/area-type) is owned and run by ospf-13 (CLI/diag/interop), not this child; this spec proves the ISM and the §9.4 election between two Ze engines on an in-memory/QEMU broadcast segment.
- Raw-IP / multicast on a real segment is Linux-only and runs as a QEMU integration test (`ai/rules/qemu-testing.md`); the in-memory broadcast harness proves the protocol logic on any host.
- NBMA / point-to-multipoint network types and virtual links are out of scope (umbrella).

## Files to Modify
<!-- MUST include feature code, not only test files -->
- `internal/plugins/ospf/instance.go` (ospf-4) - start an OSPF interface per enabled interface; register a Hello (Type 1) handler with the ospf-4 packet receive dispatcher; subscribe interfaces to iface up/down; expose the interface snapshot for ospf-13
- `internal/plugins/ospf/events.go` (ospf-4) - add ISM state-change and DR-change event payloads if not already present (consumed by ospf-6 NSM AdjOK? and ospf-7 Network-LSA origination)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | none new here; network-type / cost / hello-interval / dead-interval / priority / passive live in `ze-ospf-conf.yang` (ospf-4) |
| YANG validation constraints | No | priority 0..255, hello-interval / dead-interval ranges enforced in ospf-4 |
| CLI commands/flags | No | `show ip ospf interface` is registered/rendered in ospf-13; this spec only supplies the snapshot API |
| CLI grammar (action before identifier) | No | `ai/rules/cli-grammar.md` (applied in ospf-13) |
| Editor autocomplete | No | n/a (no new config leaves here) |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-interface.ci` |
| Pipe completeness | No | snapshot rendering and pipes handled in ospf-13 |
| Doctor check for runtime dependencies | No | `CAP_NET_RAW` / raw-socket check owned by ospf-3 |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_ospf_interface_up{area,interface}` (gauge) and `ze_ospf_dr_elections_total{interface}` (counter) per the umbrella canonical Metrics table. Per-owner registration here, NOT in ospf-13 (ospf-13 only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | ISM/election surfaced via `show ip ospf interface` in ospf-13 |
| 2 | Config syntax changed? | No | no new leaves here (ospf-4 owns network-type/priority/timers/passive) |
| 3 | CLI command added/changed? | No | `show ip ospf interface` documented in ospf-13 |
| 4 | API/RPC added/changed? | No | snapshot RPC registered in ospf-13 |
| 5 | Plugin added/changed? | No | component change is internal to OSPF |
| 6 | Has a user guide page? | No | `docs/guide/ospf.md` covered by ospf-13 |
| 7 | Wire format changed? | No | Hello/common-header wire format documented by ospf-2 |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` (§9.1-§9.5 ISM + election, §10.5 receiving Hellos) -- created/extended via `/ze-rfc` at implementation time |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- new `test/ospf/ospf-interface.ci` |
| 11 | Affects daemon comparison? | No | comparison row owned by ospf-13 |
| 12 | Internal architecture changed? | No | new `iface/` subpackage noted in the umbrella architecture layout |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `ze_ospf_interface_up` / `ze_ospf_dr_elections_total` owned and registered HERE (umbrella canonical table), documented in `docs/plugin-development/metrics.md`; ospf-13 only scrapes/surfaces |
| 15 | Registered plugin/event/command/capability changed? | No | ISM/DR events live in the OSPF events namespace (ospf-4) |
| 16 | Changed source file referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/plugins/ospf/iface/iface.go` - OSPF interface runtime: per-interface RX (Hello handler) / TX (Hello sender) over the ospf-3 path, timer wiring (Hello timer, Wait timer, neighbour inactivity timers), network-type gate
- `internal/plugins/ospf/iface/ism.go` - the Interface State Machine: 8 states (Down/Loopback/Waiting/Point-to-Point/DROther/Backup/DR), the state x event transition table (InterfaceUp/WaitTimer/BackupSeen/NeighborChange/LoopInd/UnloopInd/InterfaceDown), `ze_ospf_interface_up` gauge
- `internal/plugins/ospf/iface/ism_test.go` - ISM transition unit tests with mocked events and a fake clock
- `internal/plugins/ospf/iface/election.go` - RFC 2328 §9.4 two-step DR/BDR election (BDR then DR, sticky rule, priority-0 ineligibility, Router ID tie-break), `ze_ospf_dr_elections_total` counter
- `internal/plugins/ospf/iface/election_test.go` - election unit tests (two-node, sticky, higher-priority join, priority-0, BDR promotion, metric)
- `internal/plugins/ospf/iface/hello.go` - Hello build (Network Mask, HelloInterval, RouterDeadInterval, Options E/N-bit per area type, priority, elected DR/BDR, heard Router IDs) and Hello receive validation (§10.5)
- `internal/plugins/ospf/iface/hello_test.go` - Hello origination, E/N-bit, two-way detection, and §10.5 mismatch-drop tests
- `internal/plugins/ospf/iface/neighbor.go` - ISM-level per-interface neighbour list (record: Router ID, state hint, advertised priority, declared DR/BDR, inactivity timer), keying, NeighborChange raising
- `internal/plugins/ospf/iface/neighbor_test.go` - neighbour keying, inactivity-driven re-election, two-way state tests
- `internal/plugins/ospf/iface/snapshot.go` - interface-table snapshot API for ospf-13 `show ip ospf interface`
- `internal/plugins/ospf/iface/snapshot_test.go` - snapshot field tests
- `test/ospf/ospf-interface.ci` - functional test: two nodes elect a DR/BDR, `show ip ospf interface` lists the state, link-down re-elects; a point-to-point pair reaches Point-to-Point

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + spec-ospf-0 umbrella |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- create the interface skeleton, register the Hello (Type 1) handler with the ospf-4 packet receive dispatcher, connect the interface to the ospf-3 RX/TX path and the iface EventBus, write the failing wiring test
   - Tests: `TestOSPFInterfaceUp` (fails: ISM is a stub)
   - Files: `iface/iface.go` (skeleton), `iface/ism.go` (states + stub transitions), wiring + Hello handler registration into `instance.go`
   - Verify: an OSPF interface starts on an enabled interface; the ospf-4 dispatcher routes Hello packets to the registered handler; the ISM stub keeps it Down so the wiring test fails for the right reason
2. **Phase: ISM core** -- implement the 8-state state x event transition table (InterfaceUp/WaitTimer/BackupSeen/NeighborChange/LoopInd/UnloopInd/InterfaceDown), the network-type gate (broadcast -> Waiting + Wait timer; point-to-point -> Point-to-Point; loopback -> Loopback), the Wait timer arm/fire, the `ze_ospf_interface_up` gauge
   - Tests: `TestOSPFISMBroadcastUpToWaiting`, `TestOSPFISMP2PUpToPointToPoint`, `TestOSPFISMWaitTimerElects`, `TestOSPFISMBackupSeenShortCircuit`, `TestOSPFISMNeighborChangeReElects`, `TestOSPFISMLoopback`, `TestOSPFISMInterfaceDown`, `TestOSPFISMTransitionTable`
   - Files: `iface/ism.go`, `iface/iface.go`
   - Verify: broadcast goes Waiting -> election; point-to-point goes straight to Point-to-Point with no election; the Wait timer and BackupSeen both trigger the election; the gauge tracks operational state
3. **Phase: Hello receive + neighbour list** -- decode the Hello via the ospf-2 codec, run §10.5 validation (Network Mask / HelloInterval / RouterDeadInterval / Options E-bit), update the ISM-level neighbour record (priority, declared DR/BDR, 2-Way via our Router ID echoed), arm the neighbour inactivity timer, hand (Router ID, 2-Way, priority, DR, BDR) to ospf-6, raise BackupSeen/NeighborChange
   - Tests: `TestOSPFHelloDecodeElectionFields`, `TestOSPFHelloTwoWayDetection`, `TestOSPFHelloMismatchNetworkMask`, `TestOSPFHelloMismatchHelloInterval`, `TestOSPFHelloMismatchDeadInterval`, `TestOSPFHelloEbitMismatchDropped`, `TestOSPFNeighborInactivityReElection`, `TestOSPFNeighborTableLANKeying`
   - Files: `iface/hello.go` (receive path), `iface/neighbor.go`
   - Verify: valid Hellos populate the neighbour list and detect 2-Way; mismatching Hellos are dropped with a structured reason and the dropped-metric; inactivity removes the neighbour and re-runs the election
4. **Phase: DR/BDR election (§9.4)** -- the two-step BDR-then-DR election over the eligible set (priority >= 1, 2-Way, plus self), the sticky rule, priority-0 ineligibility, Router ID tie-break, BDR promotion on DR loss, the ISM transition to DR/Backup/DROther, the `ze_ospf_dr_elections_total` counter, the DR-change event, and the AllDRouters (re)join signal to ospf-3
   - Tests: `TestOSPFDRElectionTwoNodes`, `TestOSPFDRElectionSticky`, `TestOSPFDRElectionHigherPriorityJoinsNoDisplace`, `TestOSPFDRElectionPriorityZeroIneligible`, `TestOSPFDRElectionBDRPromotedOnDRLoss`, `TestOSPFDRElectionCountsMetric`
   - Files: `iface/election.go`, `iface/ism.go`
   - Verify: exactly one DR and one BDR; the sitting DR is sticky against a higher-priority joiner (trap 12); priority 0 is heard but ineligible; a (DR,BDR) change increments the counter, fires NeighborChange to ospf-6, emits the DR-change event, and signals ospf-3 to (re)join AllDRouters
5. **Phase: Hello sender** -- periodic Hello build to AllSPFRouters originating Network Mask, HelloInterval, RouterDeadInterval, Options (E-bit per area type, N-bit on NSSA -- trap 11), Router Priority, the elected DR/BDR, and the heard Router IDs; passive interfaces send no Hello and run no election
   - Tests: `TestOSPFHelloOriginationFields`, `TestOSPFHelloEbitSetNormalClearStub`, `TestOSPFHelloPeriodicSend`, `TestOSPFPassiveInterfaceNoHello`
   - Files: `iface/hello.go` (send path)
   - Verify: Hellos are sent at HelloInterval to AllSPFRouters with the correct fields; E-bit set on normal areas and clear on stub/NSSA; passive interfaces are silent but keep a record
6. **Phase: Snapshot + interface-down teardown** -- the `show ip ospf interface` snapshot API; subscribe to the iface EventBus, tear down neighbours and set `ze_ospf_interface_up` = 0 on interface-down
   - Tests: `TestOSPFInterfaceSnapshot`, `TestOSPFInterfaceDownTeardown`
   - Files: `iface/snapshot.go`, `iface/iface.go`, `instance.go`
   - Verify: the snapshot returns the expected fields; link-down tears down all neighbours on the interface and zeroes the gauge
7. **Functional test** -- `test/ospf/ospf-interface.ci`: two nodes elect a DR/BDR, the snapshot lists the state, link-down re-elects; a point-to-point pair reaches Point-to-Point
8. **RFC refs** -- add `// RFC 2328 Section 9.1`, `// RFC 2328 Section 9.3`, `// RFC 2328 Section 9.4`, `// RFC 2328 Section 9.5`, and `// RFC 2328 Section 10.5` comments above the enforcing code
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-ospf-5-interface-ism.md`; two commits (code+spec+learned, then `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; every End-to-End User Story has a working path |
| Correctness | The 8 states and the state x event table match RFC 2328 §9.1/§9.3; the election matches §9.4 (BDR then DR, sticky, priority-0 ineligible, Router ID tie-break); Hello validation matches §10.5 |
| Naming | Package `iface`; events on the ospf-4 namespace; snapshot fields match the `show ip ospf interface` columns |
| Data flow | transport -> ospf-4 dispatcher -> registered Hello handler -> codec -> ISM/election -> neighbour table -> events; no inline byte parsing; no packet-type switch outside the ospf-4 dispatcher; no LSDB/SPF/NSM import |
| CLI grammar | n/a here (rendering in ospf-13) |
| Doctor checks | n/a here (transport owns `CAP_NET_RAW` in ospf-3) |
| YANG validation | priority 0..255 / hello-interval / dead-interval ranges enforced in ospf-4 and respected here |
| Prometheus counters | `ze_ospf_interface_up` set on every ISM transition; `ze_ospf_dr_elections_total` incremented only on a (DR,BDR) change; no other OSPF series touched |
| Rule: plugin-self-containment | interface/ISM/election/snapshot code lives under `internal/plugins/ospf/iface/` |
| Rule: no premature NSM/LSDB coupling | the full NSM (Down..Full, DD, LS Request) stays in ospf-6; Network-LSA origination stays in ospf-7; this spec only emits the events they consume |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| iface package | `ls internal/plugins/ospf/iface/iface.go internal/plugins/ospf/iface/ism.go internal/plugins/ospf/iface/election.go internal/plugins/ospf/iface/hello.go internal/plugins/ospf/iface/neighbor.go internal/plugins/ospf/iface/snapshot.go` |
| Functional test | `ls test/ospf/ospf-interface.ci` |
| ISM transitions | `go test ./internal/plugins/ospf/iface/ -run TestOSPFISM` |
| DR election | `go test ./internal/plugins/ospf/iface/ -run TestOSPFDRElection` |
| Two-node DR/BDR | `go test ./internal/plugins/ospf/... -run TestOSPFDRElectionTwoNodes` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every Hello field length validated before slicing (delegated to the ospf-2 codec; the ISM never indexes raw bytes); §10.5 mismatch drops before any neighbour state changes |
| Spoofing | Area ID / version / checksum / auth checked by the ospf-4 dispatcher before dispatch; Network-Mask / HelloInterval / RouterDeadInterval / E-bit checked here before forming a neighbour; reject mismatches |
| Authentication | AuType verify/sign is an ospf-12 hook on the receive/send path; this spec must leave a clean insertion point and not bypass it |
| Resource exhaustion | Cap the ISM-level neighbour count per interface; neighbour inactivity and Wait timers bounded; reject malformed Hellos without allocating per-packet; the election is O(neighbours) not O(neighbours^2) |
| Privilege | none new (raw-socket / multicast privilege owned by ospf-3) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2328 §9 / research guide sec 5a/5b |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| Interop mismatch (later, ospf-13) | Capture with tcpdump, compare to FRR `ospfd`, fix codec/ISM/election |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| ISM and NSM are separate machines; this spec owns only the ISM-level neighbour view (heard, 2-Way, priority, declared DR/BDR) | Fold a single FSM per neighbour as IS-IS does | RFC 2328 has two distinct machines; the §9.4 election reads Hello-advertised state only, so the ISM can elect a DR before any DD exchange (ospf-6); keeps this spec shippable before the NSM |
| Election runs over value-typed neighbour snapshots under the interface's single-writer goroutine | Lock the live neighbour table during election | Avoids RWMutex re-entrancy traps (seen in IS-IS isis-5) and races between the RX fan and timer goroutines; the election is a pure function of a snapshot |
| Point-to-point interfaces gate out of the election entirely | One ISM that conditionally skips election | Clear separation: p2p has no DR/BDR/Network-LSA, so the network-type gate at InterfaceUp keeps the election code broadcast-only and prevents an accidental Network-LSA (R-7) |
| DR/BDR change is signalled to ospf-3 (AllDRouters join/leave) rather than ospf-5 owning multicast | iface owns multicast membership | Transport (ospf-3) owns the socket and multicast per umbrella "Frame addressing + transport"; the ISM only declares its DR/BDR role and lets ospf-3 (re)join 224.0.0.6 |

## Known Limitations
- No Neighbor State Machine, DD exchange, or LS Request drain here (ospf-6); this spec carries each neighbour only as far as the §9.4 election needs and hands 2-Way + DR identity to ospf-6.
- No Network-LSA or Router-LSA origination here (ospf-7); the DR-change event and the 2-Way/DR data are emitted for ospf-7 to consume.
- No Hello authentication here (ospf-12 inserts the AuType verify/sign hook on the receive/send path).
- NBMA, point-to-multipoint, and virtual-link network types are out of scope (umbrella); only broadcast and point-to-point ISM behaviour is implemented.
- Raw-IP / multicast on a real segment is exercised only under a Linux QEMU integration test; the in-memory broadcast harness proves the ISM and election logic on any host.

## RFC Documentation

Add `// RFC 2328 Section 9.1: "<quoted requirement>"` above the ISM state definitions,
`// RFC 2328 Section 9.3: "<quoted requirement>"` above the state x event transition table,
`// RFC 2328 Section 9.4: "<quoted requirement>"` above the two-step DR/BDR election (including the sticky rule),
`// RFC 2328 Section 9.5: "<quoted requirement>"` above the Hello send path, and
`// RFC 2328 Section 10.5: "<quoted requirement>"` above the Hello receive validation.
MUST document: the eight states, the state transitions, the election (two-step, sticky, priority-0 ineligibility, Router ID tie-break), the Wait-timer constraint, and the receive-validation rules (Network Mask, HelloInterval, RouterDeadInterval, Options E-bit).

## Implementation Summary

### What Was Implemented
- Added `internal/plugins/ospf/iface/` with ISM state values, per-interface runtime, Hello RX/TX, neighbour tracking, DR/BDR election, metrics, event sink, and snapshot API.
- Wired OSPF engine interface reconciliation to active, passive, and loopback records; Hello packets enter through the ospf-4 dispatcher and drops are counted through ospf-3 transport metrics.
- Added ISM, DR, and neighbour events to the OSPF event namespace and registered them from `register.go`.
- Added loopback network type support in OSPF config/YANG because AC-14 requires loopback interface records.
- Added OSPF interface functional fixtures and documentation for config, metrics, architecture, plugin inventory, and functional tests.

### Bugs Found/Fixed
- Fixed review finding: Hello DR/BDR fields were treated as Router IDs instead of interface addresses; `TestOSPFDRElectionTwoNodes` now checks the encoded fields.
- Fixed review finding: runtime reconcile did not refresh on router ID or area type changes; `TestOSPFReconcileAreaTypeRefreshesRuntime` covers this.
- Fixed review finding: ISM/DR/neighbour events were missing from the OSPF namespace; `TestOSPFInterfaceEvents` and `TestOSPFEventNamespace` cover this.
- Fixed review finding: validation drops were not counted; `Transport.RecordDrop` and `TestOSPFTransportRecordDrop` cover structured reasons.
- Fixed review finding: stopping a DR/BDR runtime did not leave AllDRouters; `TestOSPFStopLeavesAllDRouters` covers this.
- Fixed review finding: inactivity timing used a coarse dead-interval ticker; `TestOSPFNeighborInactivityDelayTracksDeadline` covers exact deadline scheduling.
- Fixed review findings: BackupSeen needed a 2-Way gate and priority-zero broadcast needed DROther; `TestOSPFISMBackupSeenRequiresTwoWay` and `TestOSPFISMPriorityZeroLeavesWaitingAsDROther` cover these.

### Documentation Updates
- Updated `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, `docs/plugin-development/metrics.md`, `docs/functional-tests.md`, `docs/DESIGN.md`, `docs/architecture/core-design.md`, and `docs/guide/plugins.md`.
- Verification: `make ze-doc-test` passed (`artifact://462`).

### Deviations from Plan
- The planned split across `hello.go`, `neighbor.go`, and `snapshot.go` stayed in `iface.go` because the package is small and the IS-IS circuit precedent keeps runtime, timers, and snapshot state together.
- `test/ospf/ospf-interface-runtime.ci` is Linux-only and skips on Darwin because the iface backend required to run the daemon fixture is unavailable on Darwin.
- Per-child commit steps are deferred to the final combined OSPF commit script requested by the user; the learned summary is still written now.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Build the OSPF Interface State Machine and per-interface Hello protocol | done | `internal/plugins/ospf/iface/iface.go`, `internal/plugins/ospf/iface/ism.go` | Broadcast, p2p, loopback, passive, Hello TX/RX, and timers are covered. |
| Keep the neighbour list ISM-scoped only | done | `internal/plugins/ospf/iface/iface.go` | Stores heard, 2-Way, priority, declared DR, declared BDR only; no NSM states. |
| Run RFC 2328 DR/BDR election | done | `internal/plugins/ospf/iface/election.go` | BDR first, sticky DR, priority-zero ineligible, Router ID tie-break. |
| Emit interface state, DR, and neighbour events | done | `internal/plugins/ospf/events.go`, `internal/plugins/ospf/register.go` | Events registered on OSPF namespace; event sink wired from engine to runtime. |
| Provide interface-table snapshot API for ospf-13 | done | `internal/plugins/ospf/iface/iface.go`, `internal/plugins/ospf/instance.go` | `Snapshot` includes area, type, state, cost, priority, timers, DR, BDR, neighbour count. |
| Do not implement NSM, LSDB flooding, SPF, or auth in this child | done | package boundaries | No imports from future NSM, LSDB, SPF, or auth packages. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestOSPFISMBroadcastUpToWaiting` | Broadcast starts Waiting when priority is non-zero. |
| AC-2 | done | `TestOSPFDRElectionTwoNodes`, `TestOSPFISMPriorityZeroLeavesWaitingAsDROther` | Wait/election path transitions to DR, Backup, or DROther. |
| AC-3 | done | `TestOSPFISMBackupSeenShortCircuitsWait`, `TestOSPFISMBackupSeenRequiresTwoWay` | BackupSeen is gated on 2-Way Hello content. |
| AC-4 | done | `TestOSPFISMP2PUpToPointToPoint` | P2P skips election and has no DR/BDR. |
| AC-5 | done | `TestOSPFDRElectionTwoNodes` | Equal priority uses Router ID tie-break; Hello DR/BDR fields use interface addresses. |
| AC-6 | done | `TestOSPFDRElectionSticky`, `TestOSPFDRElectionHigherPriorityJoinsNoDisplace` | Sitting DR is sticky against higher-priority joiner. |
| AC-7 | done | `TestOSPFDRElectionPriorityZeroIneligible`, `TestOSPFISMPriorityZeroLeavesWaitingAsDROther` | Priority 0 is heard but ineligible and starts DROther. |
| AC-8 | done | `TestOSPFHelloTwoWayDetection` | Echoed local Router ID marks neighbour 2-Way. |
| AC-9 | done | `TestOSPFNeighborInactivityReElection`, `TestOSPFNeighborInactivityDelayTracksDeadline` | Dead neighbours are removed on RouterDeadInterval deadline and election reruns. |
| AC-10 | done | `TestOSPFHelloMismatches`, `TestOSPFTransportRecordDrop` | Network mask, HelloInterval, RouterDeadInterval, and E-bit mismatches return structured reasons and transport records drops. |
| AC-11 | done | `TestOSPFHelloOriginationFields`, `TestOSPFHelloEbitSetNormalClearStub` | Hello fields include timers, options, priority, neighbours, DR, and BDR. |
| AC-12 | done | `TestOSPFPassiveInterfaceNoHello`, `TestOSPFPassiveAndLoopbackRecordsDoNotOpenTransport` | Passive records exist and send no Hellos. |
| AC-13 | done | `TestOSPFInterfaceEvents`, `TestOSPFDRElectionTwoNodes`, `TestOSPFStopLeavesAllDRouters` | DR/neighbor/state events and AllDRouters joins/leaves are wired. |
| AC-14 | done | `TestOSPFISMLoopbackNoHello`, `TestOSPFPassiveAndLoopbackRecordsDoNotOpenTransport` | Loopback records stay Loopback and send no Hellos. |
| AC-15 | done | `TestOSPFCarrierFlapRestoresRunningInterface` | Link down marks runtime Down and link up restores Waiting. |
| AC-16 | done | `TestOSPFInterfaceSnapshot`, `test/ospf/ospf-interface-runtime.ci` | Snapshot exposes area, state, type, timers, priority, DR/BDR, neighbours. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| ISM state, loopback, p2p, priority-zero, BackupSeen | done | `internal/plugins/ospf/iface/iface_test.go` | Covers state transitions and election triggers. |
| Hello receive/send/validation | done | `internal/plugins/ospf/iface/iface_test.go` | Covers Two-Way, E-bit, timer mismatch, DR/BDR address fields. |
| Reconcile, passive/loopback records, link-down | done | `internal/plugins/ospf/instance_test.go` | Covers engine wiring and config reload refresh. |
| Metrics/events/drop recording | done | `internal/plugins/ospf/{events_test.go,transport/metrics_test.go,iface/iface_test.go}` | Covers event registration/emission and drop metric labels. |
| Functional OSPF config/runtime fixture | done | `test/ospf/ospf-interface*.ci` | Runtime fixture skips on darwin because iface backend is unavailable. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/ospf/iface/iface.go` | done | Runtime, Hello RX/TX, timers, snapshot, event sink. |
| `internal/plugins/ospf/iface/ism.go` | done | State names and network/area constants. |
| `internal/plugins/ospf/iface/election.go` | done | RFC 2328 DR/BDR election helper. |
| `internal/plugins/ospf/instance.go` | done | Dispatcher handler, runtime lifecycle, reconcile, metrics, snapshots. |
| `internal/plugins/ospf/events.go` | done | Interface-state, DR-change, neighbor-change events. |
| `internal/plugins/ospf/transport/transport.go` | done | Drop recording and AllDRouters membership path consumed by ISM. |
| `test/ospf/ospf-interface*.ci` | done | Config and runtime snapshot fixtures. |
| Docs | done | Configuration, architecture, plugin, metrics, and functional-test docs updated. |

### Audit Summary
- **Total items:** 16 ACs, 5 test groups, 8 file groups
- **Done:** all listed items
- **Partial:** none
- **Skipped:** none
- **Changed:** added loopback enum and OSPF ISM event types while implementing AC-13/AC-14

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Two broadcast nodes elect exactly one DR and one BDR and agree | unit + functional test | `TestOSPFDRElectionTwoNodes`, `test/ospf/ospf-interface.ci` |
| A higher-priority joiner does not displace the sitting DR (sticky) | unit test | `TestOSPFDRElectionHigherPriorityJoinsNoDisplace` |
| Point-to-point interfaces reach Point-to-Point with no election | unit + functional test | `TestOSPFISMP2PUpToPointToPoint`, `test/ospf/ospf-interface.ci` |
| Mismatching Hellos (mask / intervals / E-bit) are dropped, no neighbour | unit test | `TestOSPFHelloMismatches` |
| `show ip ospf interface` reflects ISM state, DR/BDR, priority, timers | functional test | `test/ospf/ospf-interface-runtime.ci` on Linux; skipped on Darwin |

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Hello DR/BDR fields were Router ID derived instead of interface addresses | `internal/plugins/ospf/iface/iface.go` | Fixed Hello source-address tracking and encoded field tests. |
| 2 | ISSUE | Reconcile did not restart or refresh runtimes after router ID or area type changes | `internal/plugins/ospf/instance.go` | Fixed runtime refresh criteria and added config reload test. |
| 3 | ISSUE | Interface state, DR, and neighbour events were not emitted on the OSPF namespace | `internal/plugins/ospf/events.go` | Added event payloads, registration, and event sink tests. |
| 4 | ISSUE | Hello validation drops were returned but not counted in transport drop metrics | `internal/plugins/ospf/instance.go`, `transport/transport.go` | Added `RecordDrop` and metric test. |
| 5 | ISSUE | Stopping an elected DR/BDR interface did not leave AllDRouters | `internal/plugins/ospf/iface/iface.go` | Fixed stop path and added leave test. |
| 6 | ISSUE | Neighbour inactivity expiry could lag nearly one dead interval | `internal/plugins/ospf/iface/iface.go` | Scheduled by next exact neighbour deadline. |
| 7 | ISSUE | Functional coverage only validated config, not daemon runtime snapshot | `test/ospf/ospf-interface.ci` | Added `ospf-interface-runtime.ci` with Darwin skip. |

### Fixes applied
- Fixed all Run 1 findings with regression tests listed in Implementation Summary.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | BackupSeen accepted one-way Hellos | `internal/plugins/ospf/iface/iface.go` | Require 2-Way before BackupSeen short-circuits Waiting. |
| 2 | ISSUE | Inactivity worker still used coarse sleeping after the first pass | `internal/plugins/ospf/iface/iface.go` | Added exact next-deadline calculation. |
| 3 | ISSUE | Priority-zero broadcast interface started Waiting instead of DROther | `internal/plugins/ospf/iface/iface.go` | Start priority-zero broadcast runtime in DROther. |
| 4 | clean | Review pass 3 reported zero findings | `OSPF5Review3` | No action. |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (`OSPF5Review3`)
- [x] All NOTEs recorded above: none

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (read/find)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/ospf/iface/{iface,ism,election}.go` | yes | `go test ./internal/plugins/ospf/iface` passed (`artifact://478`) |
| `internal/plugins/ospf/instance.go` and `events.go` | yes | `go test ./internal/plugins/ospf/...` passed (`artifact://480`) |
| `test/ospf/ospf-interface.ci` and `test/ospf/ospf-interface-runtime.ci` | yes | `make ze-ospf-test` passed 3/3, runtime skipped on Darwin (`artifact://484`) |
| Docs listed in Documentation Updates | yes | `make ze-doc-test` passed (`artifact://462`) |
| Learned summary | yes | `plan/learned/959-ospf-5-interface-ism.md`; `.counter` bumped to 960 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | Broadcast starts Waiting, BackupSeen elects, p2p skips election, loopback sends no Hello | `go test ./internal/plugins/ospf/iface` (`artifact://478`) |
| AC-5..AC-7 | DR/BDR election, sticky DR, higher-priority joiner, priority-zero ineligible | `TestOSPFDRElectionTwoNodes`, `TestOSPFDRElectionHigherPriorityJoinsNoDisplace`, `TestOSPFDRElectionPriorityZeroIneligible` (`artifact://478`) |
| AC-8..AC-11 | 2-Way detection, inactivity expiry, mismatch drops, Hello fields | `TestOSPFHelloTwoWayDetection`, `TestOSPFNeighborInactivityDelayTracksDeadline`, `TestOSPFHelloMismatches`, `TestOSPFHelloOriginationFields` (`artifact://478`) |
| AC-12..AC-16 | Passive/loopback records, events, link-down, snapshot | `go test ./internal/plugins/ospf/...` and `make ze-ospf-test` (`artifact://480`, `artifact://484`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| OSPF config enables active, passive, and loopback interfaces | `test/ospf/ospf-interface.ci` | `make ze-ospf-test` passed on Darwin (`artifact://484`) |
| Runtime snapshot for `show ip ospf interface` data | `test/ospf/ospf-interface-runtime.ci` | Present and Linux-only; Darwin skip recorded by `make ze-ospf-test` (`artifact://484`) |
| Packet dispatcher to Hello handler and runtime reconcile | unit tests | `go test ./internal/plugins/ospf/...` (`artifact://480`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-3 | confirmed | transport sender, dispatcher, and packet codec used by OSPF runtime tests (`artifact://480`) |
| A-4..A-5 | confirmed | DR election and neighbour table tests in `iface` package (`artifact://478`) |
| A-6..A-8 | confirmed | carrier flap, passive/loopback, and area-type refresh tests (`artifact://480`) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Config and syntax docs mention OSPF interface network types including loopback | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` | `make ze-doc-test` passed (`artifact://462`) |
| Metrics docs list OSPF interface state and DR election metrics | `docs/plugin-development/metrics.md` | `make ze-doc-test` passed (`artifact://462`) |
| Functional and architecture docs mention OSPF interface runtime coverage | `docs/functional-tests.md`, `docs/DESIGN.md`, `docs/architecture/core-design.md`, `docs/guide/plugins.md` | `make ze-doc-test` passed (`artifact://462`) |
| Known branch-wide review prechecks | `make ze-validate` still reports pre-existing central validator and staged OSPF exported-symbol issues (`artifact://487`); `audit-test-relaxation.py` reports unrelated deleted L2TP/UI tests | recorded, not introduced by this child |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-16 all demonstrated
- [x] End-to-End User Stories: every story has a working path and a passing test or Linux-only fixture
- [x] Wiring Test table complete: every row has a concrete test name
- [x] `/ze-review` gate clean: Review Gate section filled with final 0 BLOCKER, 0 ISSUE
- [x] Targeted child gates pass: `go test` (`artifact://480`), `make ze-lint-changed` (`artifact://482`), `make ze-ospf-test` (`artifact://484`)
- [x] Feature code integrated under `internal/plugins/ospf` and OSPF docs/tests
- [x] Integration completeness proven through dispatcher, runtime reconcile, event sink, metrics, and functional fixtures
- [x] Documentation Update Checklist answered with source evidence
- [x] Architecture docs and guides updated where changed behavior is documented
- [x] Critical Review passes: review pass 3 reported 0 findings
- [x] Risks & Assumptions: every A-N confirmed; no broken assumptions

### Quality Gates (SHOULD pass)
- [x] RFC constraint comments added for ISM/election/Hello receive/send enforcing code
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed; LSP rename tool issue reported

### Design
- [x] No premature abstraction: OSPFv2 iface package is separate from OSPFv3 and future NSM/LSDB packages
- [x] No speculative features: NBMA, P2MP, virtual links, NSM, LSDB, SPF, and auth remain outside this child
- [x] Single responsibility per component
- [x] Explicit behaviour for broadcast, p2p, passive, and loopback interfaces
- [x] Minimal coupling: no NSM, LSDB, SPF, or auth imports

### TDD
- [x] Tests written
- [x] Tests fail evidence captured during phase work and review fixes
- [x] Tests pass: `go test ./internal/plugins/ospf/iface ./internal/plugins/ospf/... ./internal/component/config ./internal/component/config/cli ./internal/component/plugin/all ./internal/test/cli` (`artifact://480`)
- [x] Boundary tests for numeric inputs covered by ospf-4 config validation and OSPF5 priority-zero tests
- [x] Functional tests for end-to-end behavior: `make ze-ospf-test` (`artifact://484`)
- [x] Interop tests for protocol features: FRR interop is owned by ospf-13; this child provides Ze runtime fixture
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [x] Critical Review passes: all quality checks recorded in Review Gate and Pre-Commit Verification
- [x] Partial/Skipped items have explicit scope justification: Linux runtime fixture skips only on Darwin; FRR interop is owned by ospf-13
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Learned summary written to `plan/learned/959-ospf-5-interface-ism.md`
- [ ] **Commit A:** deferred to final combined OSPF commit script requested by the user
- [ ] **Commit B:** deferred to final combined OSPF commit script requested by the user
