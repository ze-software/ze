# Spec: ospf-6-neighbor-nsm

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-5-interface-ism.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - umbrella, row "ospf-6", Shared Contracts ("Packet receive dispatcher", "Area + interface config model"), the Metrics canonical table, the dependency graph, the `neighbor/` package layout
4. `docs/research/ospf-implementation-guide.md` §5c (Neighbor State Machine, lines ~291-321), §5d (Database Synchronisation ExStart->Exchange->Loading->Full, lines ~323-337), §5e (flooding receive procedure, only the LS Request/LS Update interaction this spec drains), trap #4 (MTU Mismatch in DD, lines ~1454-1458)
5. `plan/spec-ospf-5-interface-ism.md` - ISM, DR/BDR identity, Hello receive that drives HelloReceived / 2-WayReceived / 1-WayReceived and the NeighborChange (AdjOK?) trigger
6. `plan/spec-ospf-2-wire.md` - DD / LS Request / LS Update packet codec (I/M/MS flags, DD sequence, interface MTU field, LSA-header lists) consumed here
7. `plan/learned/931-isis-5-adjacency.md` - the sibling IS-IS adjacency FSM (state machine + per-circuit neighbour table structure) OSPF mirrors above the wire

## Task

Build the OSPF Neighbor State Machine (NSM) and the database-synchronisation
exchange for Ze. A neighbour is the per-remote-router runtime object that an
OSPF-enabled interface (spec-ospf-5 ISM) creates when it first hears a Hello from
a router on that segment. The NSM implements the RFC 2328 §10.1 states (Down,
Attempt, Init, 2-Way, ExStart, Exchange, Loading, Full) and the §10.2/§10.3
events that drive transitions between them. It decides, via the §10.4 adjacency
predicate, whether a given neighbour should become FULLY adjacent (on a broadcast
network only with the DR and the BDR; on point-to-point always), and for those
neighbours it runs the §10.6/§10.8/§10.9 Database Description exchange: the
master/slave negotiation (the I, M, MS flag bits and the DD sequence number) in
ExStart, the description of each side's LSDB as lists of LSA headers in Exchange,
the population and draining of the per-neighbour LS Request list in Loading, and
the transition to Full when the request list empties.

The NSM consumes the typed DD / LS Request / LS Update packets from the
spec-ospf-2 codec, delivered through the spec-ospf-4 packet receive dispatcher
(common-header Type 2 DD, 3 LS Request, 4 LS Update; Type 1 Hello reaches the
ISM in spec-ospf-5 and the ISM in turn fires HelloReceived / 2-WayReceived /
1-WayReceived on the NSM). It honours the RFC 2328 §10.6 / trap #4 MTU check:
a DD whose advertised interface MTU exceeds our interface MTU is rejected (the
neighbour does not progress past ExStart) unless the interface is configured
`mtu-ignore`. It arms the InactivityTimer (RouterDeadInterval) on every Hello and
fires KillNbr (-> Down) when it expires. It re-evaluates the adjacency predicate
(AdjOK?) whenever the ISM signals a DR/BDR change, dropping a no-longer-wanted
adjacency back to 2-Way.

The deliverable is: two Ze nodes, having reached 2-Way via spec-ospf-5 Hellos,
form a FULL adjacency (point-to-point, or with the DR/BDR on a broadcast LAN) by
completing the DD exchange and draining the LS Request list, expose that
neighbour through a snapshot API that spec-ospf-13 renders as
`show ip ospf neighbor`, and tear the adjacency down (back to Down) when the
InactivityTimer expires or the ISM signals the interface down. SeqNumberMismatch
and BadLSReq restart the exchange from ExStart.

This spec covers the NSM and the DD/LS Request exchange but NOT the LSDB store,
self-LSA origination, or the §13 flooding procedure (`plan/spec-ospf-7-lsdb-flooding.md`,
which owns the LSDB the request list is compared against and the LS Update sender
that answers the LS Requests). It does NOT cover the ISM, Hello validation, or
DR/BDR election (`plan/spec-ospf-5-interface-ism.md`). It does NOT cover
authentication of packets (`plan/spec-ospf-12-auth.md`, a verify hook on the
receive path before dispatch). The per-neighbour retransmit list and delayed-ack
machinery of §13 are owned by spec-ospf-7; this spec owns only the LS Request
list (the synchronisation list) and its drain to Full.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/research/ospf-implementation-guide.md` §5c (Neighbor State Machine) - the eight steady/transient states, the thirteen events, the LAN vs point-to-point difference
  -> Decision: track one neighbour record per (interface, neighbour Router ID); the InactivityTimer watches RouterDeadInterval; 2-Way is the steady state for neighbours we do NOT make fully adjacent
  -> Constraint: the `should_adj` predicate (§10.4) is the architectural reason for the DR -- on broadcast, form a full adjacency only if the local router is DR/BDR OR the neighbour is DR/BDR; on point-to-point always; AdjOK? re-runs the predicate on every DR/BDR change and drops back to 2-Way if it flips to "no"
- [ ] `docs/research/ospf-implementation-guide.md` §5d (Database Synchronisation) - ExStart master/slave negotiation (I/M/MS, DD sequence), Exchange (LSA-header lists), Loading (LS Request drain), failures (SeqNumberMismatch / BadLSReq -> ExStart)
  -> Constraint: in ExStart compare Router IDs -- larger Router ID is master; the master increments the DD sequence and drives, the slave echoes; ExchangeDone fires when both sides have sent a DD with M=0 and the slave has acked; LoadingDone when the LS Request list empties
  -> Constraint: when a DD describes an LSA whose `(Type, LS ID, Advertising Router)` is absent from our LSDB or is newer than our copy (RFC 2328 §13.1 freshness compare), add the header to the LS Request list for that neighbour
- [ ] `docs/research/ospf-implementation-guide.md` trap #4 (MTU Mismatch in DD) - reject a DD whose advertised interface MTU exceeds ours unless `mtu-ignore`
  -> Constraint: the check is made on the ExStart/Exchange transition and a mismatch forces the NSM back to ExStart indefinitely; implement the strict check AND the `mtu-ignore` override
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, no-alloc hot path
  -> Constraint: the LS Request list stores LSA keys `(Type, LS ID, Advertising Router)` plus the requested freshness, not copies of LSA bodies; the DD LSA-header list is read from the codec without copying bodies
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/registration-dispatch.md` - self-contained component, registration not switch
  -> Constraint: neighbour and NSM code live under `internal/plugins/ospf/neighbor/`; the DD/LS Request handlers register with the spec-ospf-4 dispatcher; the `show ip ospf neighbor` snapshot is produced here and rendered by spec-ospf-13

### RFC Summaries (MUST for protocol work; existing, read before implementation)
- [ ] `rfc/short/rfc2328.md` - OSPF Version 2 (created by spec-ospf-2 via `/ze-rfc`)
  -> Constraint: §10.1 states; §10.2 event-driven transitions; §10.3 the state x event table; §10.4 `should_adj`; §10.6 DD packet (I/M/MS bits, interface MTU, DD sequence, options); §10.8 ExStart/Exchange; §10.9 Loading; §10.10 an explanatory example
  -> Constraint: §10.5 the DD-received handling, including the duplicate-DD detection on the slave (re-send the last DD) and master (drop the duplicate); §10.7 the LS-Request-received handling (BadLSReq if the requested LSA is not in our LSDB)

**Key insights:** (minimal context to resume after compaction)
- Eight states: Down, Attempt, Init, 2-Way, ExStart, Exchange, Loading, Full (Attempt is NBMA-only and dormant in v1; broadcast + point-to-point only). Thirteen events: HelloReceived, Start, 2-WayReceived, NegotiationDone, ExchangeDone, BadLSReq, LoadingDone, AdjOK?, SeqNumberMismatch, 1-WayReceived, KillNbr, InactivityTimer, LLDown.
- HelloReceived restarts the InactivityTimer and (from Down) moves to Init. 2-WayReceived (we see our own Router ID in the neighbour's Hello) takes Init -> 2-Way; then `should_adj` decides whether to fire the Start-equivalent into ExStart or stay at 2-Way.
- ExStart: send DD with I=1,M=1,MS=1, an initial DD sequence, no LSA headers; larger Router ID becomes master; NegotiationDone -> Exchange. Exchange: master drives, slave echoes, both walk their LSDB sending LSA-header batches; ExchangeDone -> Loading (or straight to Full if nothing to request). Loading: drain the LS Request list; LoadingDone -> Full.
- MTU check (trap #4): reject a DD whose advertised interface MTU > ours unless `mtu-ignore`; mismatch holds the neighbour at ExStart.
- SeqNumberMismatch / BadLSReq -> ExStart (discard the partial exchange). InactivityTimer / KillNbr / LLDown -> Down. AdjOK? re-runs `should_adj` on DR/BDR change and drops to 2-Way if adjacency is no longer wanted.
- The neighbour record is keyed per (interface, neighbour Router ID); on a LAN many neighbours remain at 2-Way forever (the steady state for non-adjacent pairs).
- This spec OWNS the metrics `ze_ospf_neighbors{area,interface,state}`, `ze_ospf_adjacencies_full{area}`, `ze_ospf_nsm_events_total{event}` (exact names, per the umbrella Metrics canonical table). spec-ospf-13 only scrapes/asserts them.

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey; this child consumes spec-ospf-2/4/5 outputs)
- [ ] Ze has no OSPF neighbour machinery; nothing forms OSPF adjacencies today
  -> Constraint: this is entirely new; nothing to preserve in the OSPF namespace
- [ ] `internal/plugins/ospf/instance.go` (spec-ospf-4) owns the packet receive dispatcher keyed by the common-header Type (Shared Contracts "Packet receive dispatcher", owner ospf-4)
  -> Constraint: this spec registers DD (Type 2), LS Request (Type 3), and LS Update (Type 4, for the Loading drain) handlers with the spec-ospf-4 dispatcher; the transport holds no protocol switch and the NSM does not classify packet types itself
- [ ] `internal/plugins/ospf/iface/` (spec-ospf-5) owns the ISM, Hello receive/validation, and DR/BDR election
  -> Constraint: the ISM creates/destroys neighbour records and fires HelloReceived / 2-WayReceived / 1-WayReceived / NeighborChange(AdjOK?) on the NSM; the NSM does not parse Hellos itself and does not run DR election
- [ ] `internal/plugins/ospf/packet/` (spec-ospf-2) decodes DD (I/M/MS, MTU, DD sequence, options, LSA-header list), LS Request, and LS Update
  -> Constraint: the NSM calls the codec; it does not index raw bytes inline
- [ ] `internal/plugins/ospf/events.go` (spec-ospf-4) defines the event namespace this spec emits adjacency up/down through
  -> Constraint: emit adjacency-up (reached Full) / adjacency-down through that namespace, consumed by spec-ospf-7 (LSDB origination triggers) and spec-ospf-13 (display)

**Behavior to preserve:**
- spec-ospf-2 packet codec, spec-ospf-4 dispatcher + lifecycle + events namespace, and spec-ospf-5 ISM contracts are consumed unchanged
- The single OSPF source name, admin distance, and Loc-RIB / redistribution model (umbrella Shared Contracts) are untouched here -- this spec does not install routes
- Other protocols (BGP, LDP, IS-IS) untouched

**Behavior to change:**
- New `internal/plugins/ospf/neighbor/` package (neighbour record, NSM, DD exchange, LS Request list)
- spec-ospf-5 ISM gains the hook to create a neighbour and fire HelloReceived / 2-WayReceived / 1-WayReceived / AdjOK? on it (the ISM/NSM coupling lives at the spec-ospf-5 boundary; this spec supplies the NSM the ISM drives)
- New OSPF adjacency-up (Full) / adjacency-down events are emitted (consumed by spec-ospf-7 origination and spec-ospf-13 display)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Inbound DD (Type 2) / LS Request (Type 3) / LS Update (Type 4) packets are delivered by the spec-ospf-4 packet receive dispatcher (Shared Contracts "Packet receive dispatcher") to the handlers this spec registers; the transport hands `(ifindex, src, payload)` to spec-ospf-4 which validates version/area/checksum/auth and switches on the common-header Type
- The ISM (spec-ospf-5), on a validated Hello, creates or looks up the neighbour record and fires HelloReceived, and (per the Hello's neighbour list) 2-WayReceived or 1-WayReceived, and on a DR/BDR change fires AdjOK?
- The InactivityTimer (RouterDeadInterval) fires when no Hello has arrived in time
- A KillNbr / LLDown signal arrives from the ISM (interface down or OSPF disabled on the interface)

### Transformation Path
1. **Hello path (spec-ospf-5 -> NSM):** validated Hello -> ISM creates/looks up the neighbour by (interface, Router ID) -> fires HelloReceived (restart InactivityTimer; Down -> Init), and 2-WayReceived (our Router ID present in the Hello -> Init -> 2-Way) or 1-WayReceived (our Router ID absent -> back to Init) -> on 2-Way the NSM evaluates `should_adj` and, if adjacency is wanted, transitions to ExStart
2. **DD path:** DD packet -> spec-ospf-4 dispatcher -> registered DD handler -> spec-ospf-2 decode (I/M/MS, interface MTU, DD sequence, options, LSA-header list) -> MTU check (reject if advertised MTU > ours and not `mtu-ignore`) -> ExStart master/slave negotiation (larger Router ID master; NegotiationDone -> Exchange) -> Exchange: compare each described LSA header to the LSDB (spec-ospf-7), adding unknown/newer headers to the LS Request list; send our own DD describing our LSDB in batches -> ExchangeDone (both sent M=0, slave acked) -> Loading (or Full if the request list is empty)
3. **LS Request drain (Loading):** send LS Request packets for the LS Request list in chunks -> spec-ospf-7 LS Update sender answers (LS Update Type 4) -> the registered LS Update handler removes satisfied entries from the LS Request list (the LSAs themselves are installed by spec-ospf-7) -> LoadingDone when the list empties -> Full
4. **Failures:** a DD sequence/flag inconsistency or duplicate-DD on the master fires SeqNumberMismatch; an LS Request for an LSA we do not hold fires BadLSReq -> both return the NSM to ExStart, discarding the partial exchange (request list cleared)
5. **Neighbour table:** on every transition, update the per-interface neighbour record (Router ID, state, DR/BDR as seen in the neighbour's Hello, neighbour interface address, dead-time / InactivityTimer expiry, DD sequence, master/slave role)
6. **Events + metrics:** on reaching Full emit adjacency-up and bump `ze_ospf_adjacencies_full`; on leaving Full (or to Down) emit adjacency-down; set `ze_ospf_neighbors{area,interface,state}` on every transition; bump `ze_ospf_nsm_events_total{event}` per fired event
7. **Timeout / down:** InactivityTimer expiry -> Down -> adjacency-down; KillNbr / LLDown from the ISM -> Down -> adjacency-down -> the ISM removes the neighbour record

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Dispatcher <-> NSM | spec-ospf-4 dispatcher routes DD (Type 2) / LS Request (Type 3) / LS Update (Type 4) to this spec's registered handlers | [ ] |
| ISM <-> NSM | spec-ospf-5 ISM creates the neighbour and fires HelloReceived / 2-WayReceived / 1-WayReceived / AdjOK? / KillNbr / LLDown; the ISM supplies DR/BDR identity for `should_adj` | [ ] |
| Codec <-> NSM | typed DD / LS Request / LS Update structs from the spec-ospf-2 packet codec | [ ] |
| NSM <-> LSDB | the LS Request list is built by comparing DD-described LSA headers against the spec-ospf-7 LSDB (freshness §13.1); the drain consumes LS Updates spec-ospf-7 installs | [ ] |
| NSM <-> events | adjacency-up (Full) / adjacency-down on the spec-ospf-4 events namespace, consumed by spec-ospf-7 origination and spec-ospf-13 | [ ] |
| NSM <-> CLI | neighbour-table snapshot API consumed by spec-ospf-13 `show ip ospf neighbor` | [ ] |

### Integration Points
- New `internal/plugins/ospf/neighbor/` (neighbour record, NSM, DD exchange driver, LS Request list, per-interface neighbour table)
- spec-ospf-4 instance owns the packet receive dispatcher this spec registers DD/LS Request/LS Update handlers with, and the events namespace
- spec-ospf-5 ISM creates/destroys neighbour records, fires the Hello-derived events and AdjOK?, and supplies DR/BDR identity
- spec-ospf-2 codec supplies DD/LS Request/LS Update encode/decode
- spec-ospf-7 LSDB supplies the freshness compare for the LS Request list and the LS Update sender that answers LS Requests; it consumes adjacency-up to (re-)originate the Router-LSA and Network-LSA (downstream, not built here)

### Architectural Verification
- [ ] No bypassed layers (transport -> spec-ospf-4 dispatcher -> registered handler -> codec -> NSM -> neighbour table -> events; no inline byte parsing in the NSM and no packet-type switch outside the spec-ospf-4 dispatcher)
- [ ] No unintended coupling (neighbour/NSM does not run DR election (spec-ospf-5), does not own the LSDB or flooding/retransmit lists (spec-ospf-7), and does not install routes (spec-ospf-8))
- [ ] No duplicated functionality (the packet codec, the dispatcher, and the ISM are reused, not reimplemented)
- [ ] Zero-copy preserved where applicable (LS Request list holds LSA keys, not bodies; DD header list read without copying bodies)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The spec-ospf-5 ISM creates the neighbour record and fires HelloReceived / 2-WayReceived / 1-WayReceived / AdjOK? on the NSM; the NSM never parses Hellos itself | umbrella dependency graph (ospf-6 depends on ospf-5); §5c "the ISM also records 2-Way as an interface-level event" | the NSM must parse Hellos and run part of the ISM, breaking layering | wiring test driving the NSM from synthetic ISM events `TestOSPFNSMDownToFull` | unvalidated |
| A-2 | The spec-ospf-2 DD codec exposes the I/M/MS flags, the advertised interface MTU, the DD sequence number, the options byte, and the LSA-header list | umbrella Shared Contracts (DD codec owned by ospf-2); §10.6 | the MTU check and master/slave negotiation cannot be implemented here | unit test `TestOSPFDDNegotiation`, `TestOSPFDDMTUMismatch` | unvalidated |
| A-3 | The spec-ospf-7 LSDB exposes a freshness compare for an LSA key `(Type, LS ID, Advertising Router)` so the LS Request list can be populated; until ospf-7 lands, the NSM compares against an injected LSDB interface | umbrella architecture (per-area LSDB owner ospf-7); §5d / §13.1 | the LS Request list cannot be populated correctly; over-request or under-request | unit test against a fake LSDB `TestOSPFLSRequestListPopulated` | unvalidated |
| A-4 | A neighbour is keyed per (interface, neighbour Router ID); on a broadcast LAN many neighbours stay at 2-Way and only DR/BDR pairs reach Full | §5c (2-Way is the LAN steady state), §10.4 `should_adj` | the neighbour table mis-keys duplicates or forms too many full adjacencies (N*(N-1)/2 instead of 2*(N-1)) | unit test `TestOSPFShouldAdjBroadcast`, table-keying test | unvalidated |
| A-5 | The LS Update (Type 4) handler this spec registers can remove satisfied entries from the LS Request list; the LSA install itself is owned by spec-ospf-7 | umbrella Shared Contracts (LSDB/flooding owner ospf-7); §10.9 | the request list never drains and the neighbour is stuck in Loading | unit test `TestOSPFLoadingDrainToFull` | unvalidated |
| A-6 | The neighbour interface address and the neighbour's view of DR/BDR are available from the ISM (Hello source / Hello DR/BDR fields) for the snapshot and the SPF next-hop on point-to-point | §5c; Shared Contracts "Next-hop derivation for SPF" (owner ospf-8) | the snapshot is missing the address column and ospf-8 lacks a P2P next-hop | unit test `TestOSPFNeighborSnapshot` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Master/slave negotiation deadlocks (both think they are master, or both slave) against a real peer | adjacency stuck in ExStart | strict §10.8 Router-ID compare; `TestOSPFDDNegotiation` both orderings; FRR interop in spec-ospf-13 |
| R-2 | MTU check applied too late (after Exchange starts) or never, causing silent flooding loss (trap #4) | adjacency reaches Full then drops LSAs; hard to diagnose | apply the check on the first DD before NegotiationDone; `TestOSPFDDMTUMismatch` + `TestOSPFDDMTUIgnore` |
| R-3 | LS Request list never empties (request for an LSA the peer cannot supply, or a satisfied entry not removed) | neighbour stuck in Loading | BadLSReq -> ExStart on a missing-LSA request; remove on every matching LS Update; `TestOSPFLoadingDrainToFull`, `TestOSPFBadLSReqRestart` |
| R-4 | AdjOK? on a DR/BDR change does not drop a no-longer-wanted adjacency, leaving a stale Full adjacency on a LAN | extra Full adjacencies after a DR change | re-run `should_adj` on AdjOK? and transition Full/Exchange/Loading -> 2-Way clearing the request list; `TestOSPFAdjOKDropsToTwoWay` |
| R-5 | InactivityTimer / event-loop race causes a flap or a stale neighbour record | neighbour reappears after Down or never times out | single-writer neighbour record per interface; deterministic timer test with a fake clock; `TestOSPFInactivityTimerKills` |
| R-6 | Duplicate-DD handling wrong (slave fails to re-send the last DD, or master reprocesses a duplicate) causing a needless SeqNumberMismatch | adjacency churns ExStart<->Exchange | implement §10.5 duplicate detection (slave re-sends, master drops); `TestOSPFDuplicateDD` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ISM fires HelloReceived/2-WayReceived on a new neighbour | -> | NSM transitions Down -> Init -> 2-Way | `TestOSPFNSMDownToFull` |
| 2-Way reached on a point-to-point interface | -> | `should_adj` yes -> ExStart -> ... -> Full | `TestOSPFNSMDownToFull` |
| DD delivered by the spec-ospf-4 dispatcher to the registered DD handler | -> | master/slave negotiation, Exchange, LS Request list populated | `TestOSPFDDNegotiation`, `TestOSPFLSRequestListPopulated` |
| LS Update delivered for a requested LSA | -> | LS Request list drains; LoadingDone -> Full | `TestOSPFLoadingDrainToFull` |
| two engines on an in-memory point-to-point circuit | -> | both neighbours reach Full and emit adjacency-up | `TestOSPFAdjacencyFull` |
| DD advertising an MTU larger than ours, no `mtu-ignore` | -> | DD rejected, neighbour held at ExStart | `TestOSPFDDMTUMismatch` |
| InactivityTimer expires with no Hello | -> | neighbour to Down, adjacency-down emitted | `TestOSPFInactivityTimerKills` |
| `show ip ospf neighbor` snapshot requested | -> | neighbour-table snapshot API returns the Full neighbour | `test/ospf/ospf-neighbor.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A Hello is received from a new router on an interface | The ISM creates a neighbour and fires HelloReceived; the NSM moves Down -> Init and arms the InactivityTimer (RouterDeadInterval) |
| AC-2 | A Hello listing our own Router ID arrives (Init); later a Hello omitting it arrives (2-Way) | 2-WayReceived takes Init -> 2-Way; a subsequent 1-WayReceived takes 2-Way (or beyond) back to Init and clears any partial exchange |
| AC-3 | 2-Way reached on a point-to-point interface | `should_adj` returns yes; the NSM proceeds to ExStart |
| AC-4 | 2-Way reached on a broadcast interface where neither we nor the neighbour is DR/BDR (DROther<->DROther) | `should_adj` returns no; the neighbour stays at 2-Way and never forms a full adjacency |
| AC-5 | 2-Way reached on a broadcast interface where we or the neighbour is DR or BDR | `should_adj` returns yes; the NSM proceeds to ExStart |
| AC-6 | Two routers in ExStart, ours with the larger Router ID | We become master (MS set, DD sequence we choose and increment); on NegotiationDone we move to Exchange; the smaller-Router-ID peer becomes slave and echoes our DD sequence |
| AC-7 | A DD packet advertises an interface MTU larger than our interface MTU, `mtu-ignore` not set | The DD is rejected; the neighbour does not progress past ExStart |
| AC-8 | The same oversized-MTU DD with `mtu-ignore` set on the interface | The MTU is ignored and the exchange proceeds normally |
| AC-9 | In Exchange, a DD describes an LSA whose `(Type, LS ID, Advertising Router)` is absent from our LSDB or newer than our copy | The LSA header is added to the LS Request list for that neighbour |
| AC-10 | Both sides have sent a DD with M=0 and the slave has acknowledged | ExchangeDone fires; the NSM moves to Loading, or directly to Full if the LS Request list is empty |
| AC-11 | In Loading, LS Updates arrive carrying the requested LSAs | Each satisfied entry is removed from the LS Request list; when it empties, LoadingDone fires and the NSM moves to Full |
| AC-12 | An LS Request arrives for an LSA we do not hold, OR a DD sequence/flag inconsistency is detected | BadLSReq (resp. SeqNumberMismatch) fires; the NSM returns to ExStart and discards the partial exchange (request list cleared) |
| AC-13 | A DR/BDR change is signalled (AdjOK?) and the predicate flips from "should adjacency" to "should not" | The NSM drops from Full/Exchange/Loading back to 2-Way and clears the LS Request list |
| AC-14 | No Hello arrives within RouterDeadInterval | InactivityTimer fires; the neighbour transitions to Down and an adjacency-down event is emitted |
| AC-15 | KillNbr / LLDown signalled by the ISM (interface down) | The neighbour transitions to Down unconditionally and adjacency-down is emitted |
| AC-16 | `show ip ospf neighbor` snapshot requested | The snapshot returns per neighbour: Router ID, state, DR/BDR (priority/role as seen), dead-time, neighbour address, and the interface |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures OSPF on two linked nodes and expects a full adjacency | config (ospf-4) -> ISM 2-Way (ospf-5) -> `should_adj` -> ExStart/Exchange/Loading -> Full -> adjacency-up event | `TestOSPFAdjacencyFull`, `test/ospf/ospf-neighbor.ci` |
| 2 | Connects two routers on a LAN with three more DROther routers | DROther<->DROther stay 2-Way; only DR/BDR pairs reach Full (`should_adj`) | `TestOSPFShouldAdjBroadcast` |
| 3 | Misconfigures the MTU on one side (no `mtu-ignore`) | DD MTU check rejects -> neighbour stuck at ExStart (trap #4) | `TestOSPFDDMTUMismatch` |
| 4 | Pulls the cable / brings the link down | ISM KillNbr/LLDown -> NSM Down -> adjacency-down | `TestOSPFInactivityTimerKills`, `TestOSPFKillNbr` |
| 5 | Runs `show ip ospf neighbor` | CLI (ospf-13) -> snapshot API -> neighbour record (state, DR/BDR, dead-time, address, interface) | `test/ospf/ospf-neighbor.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFNSMDownToInit` | `internal/plugins/ospf/neighbor/nsm_test.go` | HelloReceived from a new neighbour -> Init; InactivityTimer armed | |
| `TestOSPFNSMDownToFull` | `internal/plugins/ospf/neighbor/nsm_test.go` | complete Down -> Init -> 2-Way -> ExStart -> Exchange -> Loading -> Full happy path, used by the wiring test | |
| `TestOSPFNSMTwoWayReceived` | `internal/plugins/ospf/neighbor/nsm_test.go` | our Router ID in the Hello -> 2-Way; absent -> 1-WayReceived back to Init | |
| `TestOSPFShouldAdjPointToPoint` | `internal/plugins/ospf/neighbor/nsm_test.go` | `should_adj` always yes on point-to-point | |
| `TestOSPFShouldAdjBroadcast` | `internal/plugins/ospf/neighbor/nsm_test.go` | broadcast: yes only if we or the neighbour is DR/BDR; DROther<->DROther stays 2-Way | |
| `TestOSPFDDNegotiation` | `internal/plugins/ospf/neighbor/dd_test.go` | ExStart master/slave by Router-ID compare (both orderings); NegotiationDone -> Exchange; I/M/MS handling | |
| `TestOSPFDDMTUMismatch` | `internal/plugins/ospf/neighbor/dd_test.go` | DD with advertised MTU > ours rejected; neighbour held at ExStart | |
| `TestOSPFDDMTUIgnore` | `internal/plugins/ospf/neighbor/dd_test.go` | same oversized MTU with `mtu-ignore` -> exchange proceeds | |
| `TestOSPFDuplicateDD` | `internal/plugins/ospf/neighbor/dd_test.go` | §10.5 duplicate-DD: slave re-sends its last DD, master drops the duplicate (no SeqNumberMismatch) | |
| `TestOSPFLSRequestListPopulated` | `internal/plugins/ospf/neighbor/lsreq_test.go` | DD header absent/newer vs the (fake) LSDB -> added to the LS Request list (§13.1 freshness); present-and-not-newer skipped | |
| `TestOSPFLoadingDrainToFull` | `internal/plugins/ospf/neighbor/lsreq_test.go` | LS Updates remove satisfied entries; empty list -> LoadingDone -> Full | |
| `TestOSPFExchangeDoneEmptyToFull` | `internal/plugins/ospf/neighbor/nsm_test.go` | ExchangeDone with an empty request list -> Full directly (skips Loading) | |
| `TestOSPFBadLSReqRestart` | `internal/plugins/ospf/neighbor/lsreq_test.go` | LS Request for an LSA we lack -> BadLSReq -> ExStart, request list cleared | |
| `TestOSPFSeqNumberMismatchRestart` | `internal/plugins/ospf/neighbor/dd_test.go` | DD sequence/flag inconsistency -> SeqNumberMismatch -> ExStart, partial exchange discarded | |
| `TestOSPFAdjOKDropsToTwoWay` | `internal/plugins/ospf/neighbor/nsm_test.go` | AdjOK? after a DR/BDR change flips `should_adj` to no -> Full/Exchange/Loading -> 2-Way, request list cleared | |
| `TestOSPFInactivityTimerKills` | `internal/plugins/ospf/neighbor/nsm_test.go` | InactivityTimer expiry (fake clock) -> Down; adjacency-down emitted | |
| `TestOSPFKillNbr` | `internal/plugins/ospf/neighbor/nsm_test.go` | KillNbr / LLDown -> Down unconditionally; adjacency-down emitted | |
| `TestOSPFNeighborTableKeying` | `internal/plugins/ospf/neighbor/table_test.go` | neighbours keyed per (interface, Router ID); two on one LAN keyed distinctly | |
| `TestOSPFNeighborSnapshot` | `internal/plugins/ospf/neighbor/table_test.go` | snapshot returns Router ID, state, DR/BDR, dead-time, address, interface | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DD advertised interface MTU (bytes) | 0..65535 (compared <= our MTU) | our-MTU | N/A | our-MTU+1 (reject unless `mtu-ignore`) |
| DD sequence number (uint32) | 0..0xFFFFFFFF | 0xFFFFFFFF | N/A | wraps (uint32) |
| RouterDeadInterval (seconds, from ospf-4/5 config) | 1..65535 | 65535 | 0 | 65536 (clamp/validate in ospf-4) |
| Neighbour Router ID (dotted-quad uint32) | 0.0.0.1..255.255.255.255 | 255.255.255.255 | 0.0.0.0 (invalid Router ID) | N/A |

### Functional Tests
<!-- New RPCs/APIs MUST have functional tests -- unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-neighbor` | `test/ospf/ospf-neighbor.ci` | two nodes form a full adjacency, it reaches Full, `show ip ospf neighbor` lists it (state/DR-BDR/dead-time/address/interface), link-down tears it down | |

### Interop Tests (MANDATORY for protocol features)
<!-- See ai/rules/interop-and-goal-validation.md. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (deferred) | - | - | FRR `ospfd` adjacency/DD-exchange interop lives in spec-ospf-13 (`ospf-p2p-frr`, `ospf-broadcast-frr`); not duplicated here | |

### Future (if deferring any tests)
- FRR `ospfd` interop (P2P, broadcast/DR DD exchange) is owned by spec-ospf-13 (CLI/diag/interop), not this child; this spec proves adjacency formation between two Ze engines on an in-memory / veth circuit.
- Raw-IP / multicast two-engine forming over real Layer 3 is a QEMU integration test (Linux-only, `ai/rules/qemu-testing.md`); the cross-engine logic is proven here on an in-memory circuit and the live socket path is exercised by spec-ospf-3's transport tests.

## Files to Modify
<!-- MUST include feature code, not only test files -->
- `internal/plugins/ospf/instance.go` (spec-ospf-4) - register the DD (Type 2), LS Request (Type 3), and LS Update (Type 4) handlers with the packet receive dispatcher; expose the neighbour snapshot for spec-ospf-13
- `internal/plugins/ospf/iface/` (spec-ospf-5) - the ISM creates/destroys neighbour records and fires HelloReceived / 2-WayReceived / 1-WayReceived / AdjOK? / KillNbr / LLDown on the NSM; supply DR/BDR identity for `should_adj` (coupling at the ISM/NSM boundary; the NSM is supplied here)
- `internal/plugins/ospf/events.go` (spec-ospf-4) - add adjacency-up (Full) / adjacency-down event payloads if not already present

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | `mtu-ignore`, `dead-interval`, `priority` live in `ze-ospf-conf.yang` (spec-ospf-4); consumed here |
| YANG validation constraints | No | dead-interval / priority ranges enforced in spec-ospf-4 |
| CLI commands/flags | No | `show ip ospf neighbor` is registered/rendered in spec-ospf-13; this spec only supplies the snapshot API |
| CLI grammar (action before identifier) | No | `ai/rules/cli-grammar.md` (applied in spec-ospf-13) |
| Editor autocomplete | No | n/a (no new config leaves here) |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-neighbor.ci` |
| Pipe completeness | No | snapshot rendering and pipes handled in spec-ospf-13 |
| Doctor check for runtime dependencies | No | `CAP_NET_RAW` / socket check owned by spec-ospf-3 |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_ospf_neighbors{area,interface,state}`, `ze_ospf_adjacencies_full{area}`, `ze_ospf_nsm_events_total{event}` (umbrella Metrics canonical table). Per-owner registration here, NOT in ospf-13 (ospf-13 only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | adjacency surfaced via `show ip ospf neighbor` in spec-ospf-13 |
| 2 | Config syntax changed? | No | no new leaves here (spec-ospf-4 owns `mtu-ignore`/`dead-interval`/`priority`) |
| 3 | CLI command added/changed? | No | `show ip ospf neighbor` documented in spec-ospf-13 |
| 4 | API/RPC added/changed? | No | snapshot RPC registered in spec-ospf-13 |
| 5 | Plugin added/changed? | No | component change is internal to OSPF |
| 6 | Has a user guide page? | No | `docs/guide/ospf.md` covered by spec-ospf-13 |
| 7 | Wire format changed? | No | DD / LS Request / LS Update wire format documented by spec-ospf-2 |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` (§10 NSM + DD exchange) -- created by spec-ospf-2 |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- new `test/ospf/ospf-neighbor.ci` |
| 11 | Affects daemon comparison? | No | comparison row owned by spec-ospf-13 |
| 12 | Internal architecture changed? | No | new `neighbor/` subpackage noted in umbrella architecture layout |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `ze_ospf_neighbors` / `ze_ospf_adjacencies_full` / `ze_ospf_nsm_events_total` owned and registered HERE (umbrella canonical table), documented in `docs/plugin-development/metrics.md`; ospf-13 only scrapes/surfaces |
| 15 | Registered plugin/event/command/capability changed? | No | adjacency events live in the OSPF events namespace (spec-ospf-4) |
| 16 | Changed source file referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/plugins/ospf/neighbor/neighbor.go` - neighbour record (Router ID, state, neighbour-reported DR/BDR + priority, neighbour interface address, InactivityTimer expiry, DD sequence, master/slave role, options)
- `internal/plugins/ospf/neighbor/nsm.go` - the Down/Attempt/Init/2-Way/ExStart/Exchange/Loading/Full state machine: the state x event transition table, `should_adj` (§10.4), HelloReceived/2-WayReceived/1-WayReceived/Start/NegotiationDone/ExchangeDone/LoadingDone/AdjOK?/SeqNumberMismatch/BadLSReq/KillNbr/InactivityTimer/LLDown handling, InactivityTimer arm/reset
- `internal/plugins/ospf/neighbor/nsm_test.go` - NSM transition unit tests with synthetic events and a fake clock
- `internal/plugins/ospf/neighbor/dd.go` - Database Description exchange driver: ExStart master/slave negotiation (I/M/MS, DD sequence), the MTU check (`mtu-ignore`), Exchange DD send/receive, §10.5 duplicate-DD handling, ExchangeDone detection
- `internal/plugins/ospf/neighbor/dd_test.go` - DD negotiation / MTU / duplicate / SeqNumberMismatch tests
- `internal/plugins/ospf/neighbor/lsreq.go` - the per-neighbour LS Request list: population from DD-described headers (§13.1 freshness vs the LSDB), drain on LS Update, BadLSReq on a missing-LSA request, LoadingDone detection
- `internal/plugins/ospf/neighbor/lsreq_test.go` - LS Request list population, drain, and BadLSReq tests (fake LSDB)
- `internal/plugins/ospf/neighbor/table.go` - per-interface neighbour table, per-(interface, Router ID) keying, snapshot API for spec-ospf-13
- `internal/plugins/ospf/neighbor/table_test.go` - table keying and snapshot tests
- `internal/plugins/ospf/adjacency_full_test.go` - two-engine wiring: both neighbours reach Full on an in-memory point-to-point circuit (`TestOSPFAdjacencyFull`)
- `test/ospf/ospf-neighbor.ci` - functional test: two engines reach Full, `show ip ospf neighbor` lists the neighbour, link-down tears it down

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

1. **Phase: Wiring (MANDATORY FIRST)** -- create the neighbour record + NSM skeleton, register the DD/LS Request/LS Update handlers with the spec-ospf-4 dispatcher, wire the spec-ospf-5 ISM to create a neighbour and fire HelloReceived/2-WayReceived, write the failing wiring test
   - Tests: `TestOSPFNSMDownToFull` (fails: NSM is a stub past 2-Way)
   - Files: `neighbor/neighbor.go` (struct), `neighbor/nsm.go` (skeleton + transition table stub), handler registration in `instance.go`, ISM hook in `iface/`
   - Verify: a neighbour is created on Hello; the dispatcher routes DD/LS Request/LS Update to the registered handlers; the NSM stub holds it at 2-Way so the wiring test fails for the right reason
2. **Phase: NSM core (Down..2-Way) + should_adj** -- HelloReceived/2-WayReceived/1-WayReceived transitions, InactivityTimer arm/reset, the §10.4 `should_adj` predicate (point-to-point always; broadcast only if we or the neighbour is DR/BDR), KillNbr/LLDown/InactivityTimer -> Down, adjacency-down events
   - Tests: `TestOSPFNSMDownToInit`, `TestOSPFNSMTwoWayReceived`, `TestOSPFShouldAdjPointToPoint`, `TestOSPFShouldAdjBroadcast`, `TestOSPFInactivityTimerKills`, `TestOSPFKillNbr`
   - Files: `neighbor/nsm.go`, `neighbor/neighbor.go`
   - Verify: Init/2-Way reached on the right Hello content; DROther<->DROther stays 2-Way; timeout and KillNbr drive Down with adjacency-down
3. **Phase: DD exchange (ExStart/Exchange)** -- master/slave negotiation (Router-ID compare, I/M/MS, DD sequence), the MTU check (`mtu-ignore`), Exchange DD send/receive in batches, §10.5 duplicate-DD handling, ExchangeDone, SeqNumberMismatch -> ExStart
   - Tests: `TestOSPFDDNegotiation`, `TestOSPFDDMTUMismatch`, `TestOSPFDDMTUIgnore`, `TestOSPFDuplicateDD`, `TestOSPFSeqNumberMismatchRestart`
   - Files: `neighbor/dd.go`, `neighbor/nsm.go`
   - Verify: larger Router ID becomes master; oversized MTU rejected unless `mtu-ignore`; duplicate DD does not restart; sequence/flag inconsistency restarts at ExStart
4. **Phase: LS Request list (Loading -> Full)** -- populate from DD-described headers (§13.1 freshness vs the LSDB), send LS Requests, drain on LS Update, BadLSReq on a missing-LSA request, LoadingDone -> Full (or ExchangeDone straight to Full when empty), adjacency-up event + `ze_ospf_adjacencies_full`
   - Tests: `TestOSPFLSRequestListPopulated`, `TestOSPFLoadingDrainToFull`, `TestOSPFExchangeDoneEmptyToFull`, `TestOSPFBadLSReqRestart`
   - Files: `neighbor/lsreq.go`, `neighbor/nsm.go`
   - Verify: only unknown/newer headers requested; list drains to Full; missing-LSA request restarts via BadLSReq
5. **Phase: AdjOK? + neighbour table + snapshot** -- re-run `should_adj` on a DR/BDR change and drop Full/Exchange/Loading -> 2-Way clearing the request list; per-(interface, Router ID) keying; snapshot API for spec-ospf-13
   - Tests: `TestOSPFAdjOKDropsToTwoWay`, `TestOSPFNeighborTableKeying`, `TestOSPFNeighborSnapshot`
   - Files: `neighbor/nsm.go`, `neighbor/table.go`
   - Verify: AdjOK? flip drops to 2-Way; correct keying; snapshot exposes Router ID/state/DR-BDR/dead-time/address/interface
6. **Phase: Two-engine wiring** -- two OSPF instances on an in-memory point-to-point circuit reach Full and emit adjacency-up
   - Tests: `TestOSPFAdjacencyFull`
   - Files: `adjacency_full_test.go`
   - Verify: both neighbours reach Full under -race
7. **Functional test** -- `test/ospf/ospf-neighbor.ci`: two engines reach Full, `show ip ospf neighbor` lists the neighbour, link-down tears it down
8. **RFC refs** -- add `// RFC 2328 Section 10.1/10.3` above the NSM transition table, `// RFC 2328 Section 10.4` above `should_adj`, `// RFC 2328 Section 10.6` above the MTU check, `// RFC 2328 Section 10.8/10.9` above the DD/Loading drivers
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-ospf-6-neighbor-nsm.md`; two commits (code+spec+learned, then `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; every End-to-End User Story has a working path |
| Correctness | States/events match RFC 2328 §10.1/§10.3; `should_adj` matches §10.4; master/slave by Router-ID compare (§10.8); MTU check on the first DD (trap #4); LS Request freshness per §13.1 |
| Naming | Package `neighbor`; events on the spec-ospf-4 namespace; snapshot fields match the `show ip ospf neighbor` columns (state, DR/BDR, dead-time, address, interface) |
| Data flow | spec-ospf-4 dispatcher -> registered DD/LS Request/LS Update handler -> codec -> NSM -> table -> events; no inline byte parsing; no packet-type switch outside the dispatcher; no DR election (ospf-5), no LSDB/flooding ownership (ospf-7), no route install (ospf-8) |
| CLI grammar | n/a here (rendering in spec-ospf-13) |
| Doctor checks | n/a here (transport owns `CAP_NET_RAW` in spec-ospf-3) |
| YANG validation | `mtu-ignore` / `dead-interval` / `priority` enforced in spec-ospf-4 and respected here |
| Prometheus counters | `ze_ospf_neighbors` / `ze_ospf_adjacencies_full` / `ze_ospf_nsm_events_total` defined, registered, and updated on transitions/events (exact umbrella names only) |
| Rule: plugin-self-containment | neighbour/NSM code and the snapshot API live under `internal/plugins/ospf/` |
| Rule: no premature LSDB coupling | the LS Request list compares against an LSDB interface (ospf-7); flooding/retransmit lists stay in ospf-7 |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| neighbor package | `ls internal/plugins/ospf/neighbor/neighbor.go internal/plugins/ospf/neighbor/nsm.go internal/plugins/ospf/neighbor/dd.go internal/plugins/ospf/neighbor/lsreq.go internal/plugins/ospf/neighbor/table.go` |
| Functional test | `ls test/ospf/ospf-neighbor.ci` |
| NSM transitions | `go test ./internal/plugins/ospf/neighbor/ -run TestOSPFNSM` |
| Two-engine Full | `go test ./internal/plugins/ospf/... -run TestOSPFAdjacencyFull` |
| Metrics owned here | `grep -R "ze_ospf_neighbors\|ze_ospf_adjacencies_full\|ze_ospf_nsm_events_total" internal/plugins/ospf/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every DD / LS Request / LS Update field validated before use (delegated to the spec-ospf-2 codec; the NSM never indexes raw bytes); the DD LSA-header count is bounded |
| Spoofing | A neighbour Router ID equal to our own Router ID is rejected before any record creation (no phantom self-neighbour); state changes gate on `should_adj` and DR/BDR identity from the ISM |
| MTU mismatch (trap #4) | The strict check is applied before NegotiationDone so a mismatched DD cannot drive the exchange forward and silently lose flooded LSAs |
| Resource exhaustion | Cap the per-interface neighbour count and the LS Request list size; bound the InactivityTimer / DD-retransmit timers; reject malformed packets without per-packet allocation |
| Authentication | packet authentication is a spec-ospf-12 verify hook in the spec-ospf-4 dispatcher BEFORE the NSM is reached; this spec must not bypass it |
| Privilege | none new (socket privilege owned by spec-ospf-3) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2328 §10 summary / research guide §5c/§5d |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| Interop mismatch (later, spec-ospf-13) | Capture with tcpdump, compare to FRR `ospfd`, fix codec/NSM |
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
| The ISM (ospf-5) creates the neighbour and fires Hello-derived events; the NSM never parses Hellos | The NSM owns Hello parse too | Keeps the §10.4 `should_adj` predicate fed by the ISM's DR/BDR identity without the NSM re-implementing DR election; matches RFC 2328's split of §9 (ISM) from §10 (NSM) |
| The LS Request list holds LSA keys and freshness, compared against an LSDB interface | Hold copies of described LSAs | Zero-copy (ai/rules/buffer-first.md); decouples this spec from the spec-ospf-7 LSDB representation; the LSA install stays in ospf-7 |
| MTU check applied on the first DD before NegotiationDone | Check after Exchange begins, or never | trap #4: a late or absent check silently loses flooded LSAs; the strict check plus `mtu-ignore` matches FRR/Cisco/Juniper behaviour |
| One neighbour per (interface, Router ID); 2-Way is the LAN steady state | Form a full adjacency with every neighbour | §10.4: cuts LAN adjacencies from N*(N-1)/2 to 2*(N-1); only DR/BDR pairs go Full |

## Known Limitations
- No LSDB store, self-LSA origination, or §13 flooding/retransmit-list machinery here (spec-ospf-7); the LS Request list compares against an LSDB interface and the drain consumes LS Updates spec-ospf-7 installs.
- No DR/BDR election or Hello validation here (spec-ospf-5); the NSM consumes ISM-supplied DR/BDR identity and Hello-derived events.
- No packet authentication here (spec-ospf-12 inserts a verify hook in the spec-ospf-4 dispatcher before the NSM).
- NBMA-only states/events (Attempt, the poll/Start handling) are present in the state set for completeness but dormant in v1 (broadcast + point-to-point only, per the umbrella scope).
- Raw-IP / multicast two-engine forming over real Layer 3 is a QEMU integration test (Linux-only); cross-engine logic is proven on an in-memory circuit here.

## RFC Documentation

Add `// RFC 2328 Section 10.1: "<quoted requirement>"` above the state definitions,
`// RFC 2328 Section 10.3: "<quoted requirement>"` above the state x event transition table,
`// RFC 2328 Section 10.4: "<quoted requirement>"` above `should_adj`,
`// RFC 2328 Section 10.6: "<quoted requirement>"` above the DD MTU check,
and `// RFC 2328 Section 10.8/10.9: "<quoted requirement>"` above the DD exchange / Loading drivers.
MUST document: the state transitions, the `should_adj` predicate, the MTU-check rule, the master/slave negotiation, the LS Request freshness compare (§13.1), and the InactivityTimer constraint.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Two Ze nodes reach a FULL adjacency via the DD exchange | unit + two-engine wiring test | `TestOSPFAdjacencyFull`; NSM `TestOSPFNSMDownToFull`, DD `TestOSPFDDNegotiation`, drain `TestOSPFLoadingDrainToFull`; `test/ospf/ospf-neighbor.ci` |
| Broadcast forms Full only with DR/BDR; DROther stays 2-Way | unit test | `TestOSPFShouldAdjBroadcast` |
| MTU mismatch (trap #4) holds the neighbour at ExStart unless `mtu-ignore` | unit test | `TestOSPFDDMTUMismatch`, `TestOSPFDDMTUIgnore` |
| InactivityTimer / KillNbr tears the adjacency down | unit test | `TestOSPFInactivityTimerKills`, `TestOSPFKillNbr` |
| `show ip ospf neighbor` snapshot available | unit test + wiring | `TestOSPFNeighborSnapshot`; snapshot wired for spec-ospf-13 rendering |
| FRR `ospfd` adjacency/DD interop | interop scenario | `test/interop/scenarios/ospf-p2p-frr/` -- owned/run by spec-ospf-13 |

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | pending | `/ze-review` not run yet for this design spec | this spec | run during implementation; record concrete findings here |

### Fixes applied
- Pending: record concrete fixes after `/ze-review` reports BLOCKER or ISSUE findings.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/neighbor/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (covered by spec-ospf-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospf-6-neighbor-nsm.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-6-neighbor-nsm.md` only (preserves edited spec in git history from commit A)
