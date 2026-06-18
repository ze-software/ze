# Spec: isis-8-dis-broadcast

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-isis-5-adjacency.md, spec-isis-6-lsdb.md, spec-isis-7-flooding.md |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` - umbrella (row isis-8, package layout, risk R-4)
4. `docs/research/isis-implementation-guide.md` sec 4b (DIS election), sec 4c (CSNP cadence), sec 6 (broadcast circuits, multicast MACs)
5. `plan/spec-isis-5-adjacency.md` - circuit abstraction + LAN adjacency table this spec layers on
6. `plan/spec-isis-6-lsdb.md` - LSP origination hooks the pseudo-node LSP reuses
7. `plan/spec-isis-7-flooding.md` - CSNP/SRM/SSN mechanism the LAN CSNP cadence drives

## Task

Add broadcast-LAN behaviour to the IS-IS engine: Designated IS (DIS) election
on broadcast circuits, pseudo-node LSP origination by the elected DIS, and the
star-topology own-LSP encoding that follows from a pseudo-node existing. This
layers on the circuit and adjacency machinery from spec-isis-5, the LSDB and
origination machinery from spec-isis-6, and the flooding and CSNP machinery
from spec-isis-7. It does not introduce a new transport or a new LSDB store;
it adds the LAN-specific election logic and a new class of locally originated
LSP (the pseudo-node).

A broadcast segment with N routers would form a full mesh of N*(N-1)/2
adjacencies and N*(N-1) Extended IS Reachability entries if every router
advertised every peer. IS-IS collapses this to a star: one router per level is
elected DIS, the DIS originates a pseudo-node LSP that lists every router on the
segment as a neighbour (metric 0), and every router (including the DIS) points
its own LSP at the pseudo-node rather than at each peer. DIS election runs
independently per level: a node can be DIS for L1, L2, both, or neither on a
given circuit.

This is `internal/component/isis/circuit/dis.go` plus pseudo-node origination
hooks in `internal/component/isis/lsdb/`. P2P circuits (spec-isis-5) have no DIS
and no pseudo-node and are untouched by this spec.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations -- these survive compaction. -->
- [ ] `docs/research/isis-implementation-guide.md` sec 4b "DIS Election", sec 4c "CSNP Synchronisation", sec 6 "Circuit Types and Network Model" - election algorithm, pseudo-node LSP, LAN CSNP cadence, multicast MACs
  → Decision: election is per level (L1 and L2 each elect their own DIS on the same circuit); a node tracks two independent DIS roles per broadcast circuit
  → Constraint: election compares (priority, MAC); highest priority wins, MAC address breaks ties (ISO/IEC 10589 §8.4.5)
  → Constraint: pseudo-node LSP uses LAN ID `<systemid>.<pseudonodeid>` with a non-zero pseudo-node ID; lists all segment routers with metric 0
  → Constraint: non-DIS routers do NOT advertise each peer; they advertise the pseudo-node as a single neighbour in Extended IS Reachability TLV 22 (star, not mesh)
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - pseudo-node LSP is built buffer-first and stored as raw bytes + metadata like any other LSP
  → Constraint: pseudo-node LSP origination reuses the spec-isis-6 origination path (raw-bytes LSDB entry), it does not add a parallel store
- [ ] `ai/rules/plugin-self-containment.md` - DIS election and pseudo-node code live entirely under `internal/component/isis/`
  → Constraint: no DIS or pseudo-node spelling leaks into generic packages

### RFC Summaries (MUST for protocol work)
- [ ] `iso/short/iso10589.md` - IS-IS base (created by isis-2; this spec relies on §8.4.5 DIS election and §7.3 LSP/SNP)
  → Constraint: §8.4.5 DIS election by priority then MAC; re-elect on DIS loss (hold time) or priority change; pseudo-node LSP origination/withdrawal on role change
  → Constraint: §6.7.3 DIS sends periodic CSNPs on the LAN to keep the segment synchronised (cadence ties into spec-isis-7)
- [ ] `rfc/short/rfc5305.md` - Extended IS Reachability TLV 22 (created by isis-2)
  → Constraint: own LSP encodes the pseudo-node as one TLV 22 neighbour with metric = circuit metric; pseudo-node LSP encodes each member with metric 0

**Key insights:** (minimal context to resume after compaction)
- Election compares (priority 0..127, MAC) per level; ties broken by higher MAC; a node may be DIS for L1, L2, both, or neither on one circuit.
- The DIS allocates a non-zero pseudo-node ID and originates LSP(s) with LAN ID `<systemid>.<pseudonodeid>` listing every router on the segment at metric 0; it withdraws/purges them when it loses the role.
- Every router (DIS and non-DIS) points its own LSP at the pseudo-node, not at individual peers (star topology).
- The DIS drives the LAN CSNP cadence using the spec-isis-7 CSNP mechanism.
- Election must be damped to avoid pseudo-node churn on a flapping LAN (umbrella risk R-4).

## Current Behavior (MANDATORY)

**Source files read:** (architecture survey; depends on isis-5/6/7 being in place)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/ldp/register.go` - closest existing protocol-engine template; Ze has no IS-IS broadcast behaviour today (no DIS election, no pseudo-node LSP)
  → Constraint: this spec is entirely new; nothing to preserve in the broadcast path
- [ ] `internal/component/isis/circuit/circuit.go` (from spec-isis-5) - broadcast circuit abstraction, LAN IIH send/receive, per-circuit neighbour table
  → Constraint: DIS election consumes the existing neighbour table and per-circuit priority/MAC; it does not re-implement adjacency
- [ ] `internal/component/isis/lsdb/origination.go` (from spec-isis-6) - LSP origination, SRM/SSN flags, aging, and purge
  → Constraint: pseudo-node LSP origination reuses this path; the only new thing is a non-zero pseudo-node ID and the member list
- [ ] `internal/component/isis/lsdb/csnp.go` (from spec-isis-7) - CSNP build/send and the flooding timers
  → Constraint: the LAN CSNP cadence selects the DIS as sender; it reuses the spec-isis-7 CSNP code

**Behavior to preserve:**
- P2P circuits (spec-isis-5) form adjacencies with no DIS and no pseudo-node; this spec must not alter P2P
- LSDB store, flooding, and CSNP mechanism (spec-isis-6/7) semantics unchanged; pseudo-node LSPs are ordinary LSPs in the store
- Own-LSP origination for P2P neighbours continues to list the neighbour directly (only broadcast neighbours collapse to the pseudo-node)

**Behavior to change:**
- Broadcast circuits gain a per-level DIS role with election on Hello receipt and on neighbour loss
- The elected DIS originates and maintains one or more pseudo-node LSPs and drives the LAN CSNP cadence
- Own LSPs replace per-broadcast-peer Extended IS Reachability entries with a single pseudo-node neighbour entry

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- LAN IIH PDUs arriving on a broadcast circuit (carrying each neighbour's priority and source MAC), delivered by the spec-isis-5 receive path
- Neighbour-lost events (hold-timer expiry) from the spec-isis-5 adjacency FSM
- Local priority change from a config apply on the broadcast circuit (spec-isis-4 config path)

### Transformation Path
1. **Collect candidates:** for the circuit and level, gather the local (priority, MAC) plus each Up LAN neighbour's (priority, MAC) from the spec-isis-5 neighbour table
2. **Elect:** select the highest (priority, then MAC) as DIS for that level; compute whether the local node is the winner
3. **Damp:** apply election damping so a transient candidate change does not immediately re-originate the pseudo-node (umbrella R-4)
4. **Role transition:** if the local node becomes DIS for the level, allocate a non-zero pseudo-node ID and originate the pseudo-node LSP via the spec-isis-6 origination path; if it loses the role, purge/withdraw the pseudo-node LSP and release the pseudo-node ID
5. **Pseudo-node LSP content:** build (buffer-first) a pseudo-node LSP with LAN ID `<systemid>.<pseudonodeid>` listing every router on the segment (DIS included) as a TLV 22 neighbour at metric 0
6. **Own LSP update:** trigger re-origination of the local node's own LSP so the broadcast circuit is represented by a single TLV 22 entry pointing at the pseudo-node (metric = circuit metric), not by per-peer entries
7. **LAN CSNP:** while DIS for the level, periodically build and send a CSNP on the circuit using the spec-isis-7 CSNP mechanism to keep the segment synchronised

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Adjacency table ↔ DIS election | read per-circuit, per-level (priority, MAC) of Up LAN neighbours (spec-isis-5) | [ ] |
| DIS election ↔ LSDB origination | role gained/lost triggers pseudo-node LSP originate / purge (spec-isis-6) | [ ] |
| DIS role ↔ flooding/CSNP | DIS drives periodic LAN CSNP (spec-isis-7 CSNP build/send) | [ ] |
| DIS state ↔ own-LSP origination | pseudo-node existence flips own LSP from per-peer to single pseudo-node neighbour | [ ] |
| Config ↔ election | per-circuit DIS priority change re-runs election | [ ] |

### Integration Points
- `internal/component/isis/circuit/dis.go` - per-level DIS election state machine and damping, owned by the broadcast circuit (spec-isis-5)
- `internal/component/isis/lsdb/` - pseudo-node LSP origination, withdrawal/purge, and the own-LSP star encoding (extends spec-isis-6 origination)
- spec-isis-7 CSNP build/send - reused for the LAN CSNP cadence; the DIS becomes the sender
- spec-isis-5 neighbour table - the candidate source for election

### Architectural Verification
- [ ] No bypassed layers (pseudo-node LSP goes through the spec-isis-6 origination path and the spec-isis-7 flooding path, not a side channel)
- [ ] No unintended coupling (DIS election reads the adjacency table but does not modify adjacency FSM internals; P2P path untouched)
- [ ] No duplicated functionality (CSNP reuses spec-isis-7; LSP store reuses spec-isis-6; no second LSDB)
- [ ] Zero-copy preserved (pseudo-node LSP built buffer-first, stored as raw bytes + metadata like any LSP)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The spec-isis-5 neighbour table exposes per-neighbour (priority, MAC) per level for the circuit | spec-isis-5 LAN IIH parse stores neighbour priority/MAC | Election needs to re-parse Hellos itself | `TestISISDISElection` reads the table | unvalidated |
| A-2 | The spec-isis-6 origination path can originate an LSP with a non-zero pseudo-node ID and an arbitrary TLV 22 member list | spec-isis-6 origination is keyed on LSPID incl. pseudo-node byte | Need a pseudo-node-specific origination path | pseudo-node build unit test | unvalidated |
| A-3 | The spec-isis-7 CSNP build/send can be driven on demand by the DIS on a chosen circuit | spec-isis-7 exposes a per-circuit CSNP send | Need a DIS-owned CSNP builder | `isis-dis.ci` LAN sync check | unvalidated |
| A-4 | A single pseudo-node LSP fragment is sufficient for the test segment sizes; multi-fragment pseudo-node LSPs follow the spec-isis-6 fragmentation path unchanged | spec-isis-6 handles LSP fragmentation generically | Need pseudo-node-aware fragmentation | boundary test with many members | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | DIS election churn on a flapping LAN re-originates the pseudo-node repeatedly (umbrella R-4) | repeated pseudo-node sequence bumps in soak | election damping (hold the role across transient candidate changes); damping unit test |
| R-2 | Stale pseudo-node LSP after losing the DIS role creates a phantom node in SPF | another node's SPF still sees the old pseudo-node | purge (zero-age, sequence bump) the pseudo-node on role loss before yielding; re-election test asserts purge |
| R-3 | Own LSP still lists individual broadcast peers alongside the pseudo-node (double counting) | SPF sees both star and mesh edges | own-LSP encoding asserts a single pseudo-node TLV 22 entry for the circuit; functional check |
| R-4 | L1 and L2 DIS roles entangled on the same circuit | changing L1 priority moves the L2 DIS | per-level election state; test elects different DIS per level on one segment |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| three nodes Up on a shared broadcast segment | → | per-level DIS election selects one DIS | `TestISISDISElection` |
| local node becomes DIS for a level | → | pseudo-node LSP originated via spec-isis-6 path, appears in the LSDB | `TestISISDISElection` |
| pseudo-node exists for the circuit | → | own LSP lists the pseudo-node as the single TLV 22 neighbour | `TestISISDISElection` |
| three Ze nodes on a shared segment (functional) | → | one DIS elected, pseudo-node LSP reflects all members, star topology | `test/isis/isis-dis.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Three Up routers on a broadcast segment, distinct priorities, per level | Exactly one DIS is elected per level; the highest (priority, MAC) wins |
| AC-2 | Two routers with equal priority on the segment | The router with the higher MAC address is elected DIS (tiebreak) |
| AC-3 | Local node is DIS for a level | A pseudo-node LSP with LAN ID `<systemid>.<pseudonodeid>` (non-zero pseudo-node ID) is originated and lists every router on the segment as a TLV 22 neighbour at metric 0 |
| AC-4 | A new member joins / a member leaves the segment | The DIS re-originates the pseudo-node LSP so it reflects the current member set |
| AC-5 | The local DIS priority is raised/lowered so the winner changes | Election re-runs; the role transfers; the old DIS purges its pseudo-node LSP and the new DIS originates one |
| AC-6 | The current DIS is lost (hold timer expiry, no Hello) | A new DIS is elected from the remaining routers; the stale pseudo-node is purged and a fresh one originated |
| AC-7 | A node is DIS for the circuit (DIS or non-DIS) | Its own LSP advertises the broadcast circuit as a single TLV 22 entry pointing at the pseudo-node, not one entry per peer |
| AC-8 | DIS priority set to 0 on every router on the segment | A DIS is still elected (MAC tiebreak); priority 0 does not forbid winning, it only lowers preference |
| AC-9 | Local node is DIS for L1 only on a circuit that runs both levels | A pseudo-node LSP is originated for L1 only; the L2 DIS role and L2 pseudo-node are independent |
| AC-10 | Rapid candidate flap on the segment within the damping window | The DIS role and pseudo-node LSP are not re-churned for transient changes (damping holds) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects three Ze nodes to one Ethernet segment with IS-IS broadcast | LAN IIH → adjacency table → per-level election → one DIS | `TestISISDISElection`, `test/isis/isis-dis.ci` |
| 2 | Expects the LAN to appear as a single node in the topology | DIS → pseudo-node LSP (members at metric 0) → flooded → other nodes' LSDB | `test/isis/isis-dis.ci` |
| 3 | Raises a node's DIS priority to make it the LAN DIS | config apply → re-election → role transfer → old pseudo-node purged, new originated | `TestISISDISElection` (priority-change case) |
| 4 | Pulls the DIS off the segment | hold-timer expiry → re-election → stale pseudo-node purged → new DIS | `TestISISDISElection` (DIS-loss case) |
| 5 | Meshes a Ze LAN with FRR isisd on the same segment | full broadcast protocol over the wire (interop) | `test/interop/scenarios/isis-lan-dis-frr` (owned by spec-isis-13) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDISElectionPriority` | `internal/component/isis/circuit/dis_test.go` | highest priority wins per level | |
| `TestDISElectionMACTiebreak` | `internal/component/isis/circuit/dis_test.go` | equal priority resolved by higher MAC | |
| `TestDISElectionPerLevel` | `internal/component/isis/circuit/dis_test.go` | L1 and L2 elect independent DIS on one circuit | |
| `TestDISReElectOnLoss` | `internal/component/isis/circuit/dis_test.go` | DIS loss triggers re-election and stale-pseudo-node purge | |
| `TestDISReElectOnPriorityChange` | `internal/component/isis/circuit/dis_test.go` | priority change transfers the role | |
| `TestDISElectionDamping` | `internal/component/isis/circuit/dis_test.go` | transient candidate flap does not re-churn the role/pseudo-node | |
| `TestPseudoNodeLSPBuild` | `internal/component/isis/lsdb/pseudonode_test.go` | pseudo-node LSP lists all members at metric 0 with correct LAN ID | |
| `TestPseudoNodePurgeOnRoleLoss` | `internal/component/isis/lsdb/pseudonode_test.go` | losing DIS purges (zero-age, seq bump) and releases the pseudo-node ID | |
| `TestOwnLSPPointsAtPseudoNode` | `internal/component/isis/lsdb/pseudonode_test.go` | own LSP encodes the circuit as one TLV 22 pseudo-node neighbour, not per-peer | |
| `TestISISDISElection` | `internal/component/isis/circuit/dis_test.go` | wiring: three nodes elect one DIS, pseudo-node LSP appears, own LSP points at it | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DIS priority | 0..127 | 127 | N/A (0 valid, lowest preference) | 128 |
| Pseudo-node ID | 1..255 | 255 | 0 (0 means non-pseudo-node) | 256 |
| Pseudo-node LSP member count (one fragment) | 0..fragment limit | fragment limit | N/A | overflow → fragment (spec-isis-6) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-dis` | `test/isis/isis-dis.ci` | three nodes on a shared segment elect one DIS; pseudo-node LSP reflects all members; own LSPs point at the pseudo-node | |

### Interop Tests (MANDATORY for protocol features)
<!-- LAN DIS interop with FRR is owned by spec-isis-13, not this spec. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-lan-dis-frr` | `test/interop/scenarios/` | FRR isisd | LAN DIS election + pseudo-node interop (owned by spec-isis-13) | |

### Future (if deferring any tests)
- LAN DIS interop with FRR (`isis-lan-dis-frr`) is implemented under spec-isis-13 (CLI/diag/interop), per the umbrella; this spec only references it.

## Files to Modify
- `internal/component/isis/lsdb/origination.go` (or equivalent from spec-isis-6) - add pseudo-node LSP origination, withdrawal/purge, and the own-LSP star encoding hook
- `internal/component/isis/circuit/circuit.go` (from spec-isis-5) - hold the per-level DIS state and invoke election on Hello receipt, neighbour loss, and priority change

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | DIS priority leaf already added in spec-isis-4 per-interface config; no new schema here |
| YANG validation constraints | No | priority range 0..127 already on the existing leaf (spec-isis-4) |
| CLI commands/flags | No | DIS shown via `show isis interface` / `show isis database` (spec-isis-13); no new command |
| CLI grammar (action before identifier) | No | n/a (no new command) |
| Editor autocomplete | No | n/a (no new leaf) |
| Functional test for new RPC/API | Yes | `test/isis/isis-dis.ci` |
| Pipe completeness | No | n/a (no new show output here; rendering owned by spec-isis-13) |
| Doctor check for runtime dependencies | No | broadcast multicast covered by the spec-isis-3 transport doctor check |
| Prometheus counters/metrics | Yes | this spec OWNS and registers its rows from the umbrella "Metrics (canonical)" table: `ze_isis_dis_elections_total{level}` and `ze_isis_pseudonode_lsps{level}` (gauge). Pseudo-node purges increment `ze_isis_purges_total{level}` (owned by isis-6). Per-owner registration here, not in isis-13 |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/isis.md` (DIS/pseudo-node behaviour on LANs) |
| 2 | Config syntax changed? | No | DIS priority leaf added by spec-isis-4 |
| 3 | CLI command added/changed? | No | DIS display owned by spec-isis-13 |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/isis.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/isis.md` (pseudo-node LSP / LAN ID encoding) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `iso/short/iso10589.md` (§8.4.5 DIS election) |
| 10 | Test infrastructure changed? | No | `test/isis/` already established by earlier children |
| 11 | Affects daemon comparison? | No | covered by umbrella row |
| 12 | Internal architecture changed? | No | covered by spec-isis-4 (component) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (DIS election / pseudo-node counters) |
| 15 | Registered plugin/event/command/capability changed? | No | none |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/component/isis/circuit/dis.go` - per-level DIS election state machine and damping
- `internal/component/isis/circuit/dis_test.go` - election unit + wiring tests (`TestISISDISElection` and the per-case tests)
- `internal/component/isis/lsdb/pseudonode.go` - pseudo-node LSP build/originate/purge and the own-LSP star encoding hook
- `internal/component/isis/lsdb/pseudonode_test.go` - pseudo-node build, purge-on-role-loss, own-LSP-points-at-pseudo-node tests
- `test/isis/isis-dis.ci` - functional test: three nodes, one DIS, pseudo-node reflects all members

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - add a per-level DIS state to the broadcast circuit and a stub election; write the failing wiring test
   - Tests: `TestISISDISElection`
   - Files: `internal/component/isis/circuit/dis.go`, hook into `circuit/circuit.go`
   - Verify: the circuit holds per-level DIS state and calls a stub election; `TestISISDISElection` fails because the election and pseudo-node are stubs
2. **Phase: Election algorithm** - compare (priority, MAC) per level over the spec-isis-5 neighbour table; compute local DIS-ness
   - Tests: `TestDISElectionPriority`, `TestDISElectionMACTiebreak`, `TestDISElectionPerLevel`
   - Files: `internal/component/isis/circuit/dis.go`
   - Verify: highest priority wins, MAC breaks ties, L1 and L2 are independent
3. **Phase: Pseudo-node LSP origination** - allocate a non-zero pseudo-node ID, build the pseudo-node LSP (members at metric 0) buffer-first, originate via the spec-isis-6 path
   - Tests: `TestPseudoNodeLSPBuild`
   - Files: `internal/component/isis/lsdb/pseudonode.go`
   - Verify: pseudo-node LSP has the right LAN ID and member set; appears in the LSDB and is flooded (spec-isis-7)
4. **Phase: Own-LSP star encoding** - re-originate the own LSP to point at the pseudo-node as a single TLV 22 neighbour for the circuit
   - Tests: `TestOwnLSPPointsAtPseudoNode`
   - Files: `internal/component/isis/lsdb/pseudonode.go`, own-LSP origination hook (spec-isis-6)
   - Verify: own LSP has one pseudo-node entry for the circuit, no per-peer entries
5. **Phase: Re-election and purge** - re-run election on neighbour loss and priority change; purge the pseudo-node and release the ID on role loss
   - Tests: `TestDISReElectOnLoss`, `TestDISReElectOnPriorityChange`, `TestPseudoNodePurgeOnRoleLoss`
   - Files: `internal/component/isis/circuit/dis.go`, `internal/component/isis/lsdb/pseudonode.go`
   - Verify: role transfers cleanly; old pseudo-node purged (zero-age, seq bump) before yielding
6. **Phase: Election damping** - hold the role across transient candidate changes within the damping window
   - Tests: `TestDISElectionDamping`
   - Files: `internal/component/isis/circuit/dis.go`
   - Verify: a flap inside the window does not re-originate the pseudo-node (umbrella R-4)
7. **Phase: LAN CSNP cadence** - while DIS, drive the periodic CSNP on the circuit using the spec-isis-7 CSNP send
   - Files: `internal/component/isis/circuit/dis.go` (timer), reuse spec-isis-7 CSNP build/send
   - Verify: the segment stays synchronised; `isis-dis.ci` confirms members converge
8. **Functional test** - `test/isis/isis-dis.ci`: three nodes, one DIS, pseudo-node reflects all members, star topology
9. **RFC refs** - add `// ISO/IEC 10589 §8.4.5` comments above the election and pseudo-node code
10. **Full verification** - `make ze-verify`
11. **Complete spec** - fill audit tables, write learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; every AC has a test |
| Feature completeness | Each user story has a working path: election → role → pseudo-node → own-LSP star → LAN CSNP |
| Correctness | Election order is (priority desc, MAC desc); per-level independence holds; pseudo-node LAN ID is `<systemid>.<pseudonodeid>` with non-zero pseudo-node ID; members at metric 0 |
| Naming | DIS/pseudo-node terms stay inside `internal/component/isis/`; counters use kebab-case metric names |
| Data flow | Pseudo-node LSP flows through spec-isis-6 origination and spec-isis-7 flooding; no side store or side channel |
| Rule: plugin-self-containment | No DIS or pseudo-node spelling leaks into generic packages |
| Rule: no-duplication | CSNP reuses spec-isis-7; LSDB reuses spec-isis-6; election does not re-parse Hellos (reads spec-isis-5 table) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| DIS election code | `ls internal/component/isis/circuit/dis.go` |
| Pseudo-node origination code | `ls internal/component/isis/lsdb/pseudonode.go` |
| Election + pseudo-node unit tests | `go test ./internal/component/isis/circuit/ ./internal/component/isis/lsdb/ -run 'DIS|PseudoNode|OwnLSP'` |
| Functional test | `ls test/isis/isis-dis.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Neighbour priority/MAC read from the spec-isis-5 table are bounded (priority 0..127); no unbounded member list without fragmentation |
| Resource exhaustion | Pseudo-node member count bounded by LSP fragmentation (spec-isis-6); election damping caps re-origination rate |
| Spoofing | A neighbour cannot force itself DIS beyond advertising a higher (priority, MAC); election only considers Up adjacencies on the circuit |
| State consistency | Role loss always purges the pseudo-node before yielding, leaving no phantom node for SPF |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read ISO/IEC 10589 §8.4.5 / Current Behavior |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## Known Limitations
- LAN DIS interop with FRR (`isis-lan-dis-frr`) is validated under spec-isis-13, not this spec.
- Single-topology only; per-topology DIS (RFC 5120 MT) is out of scope per the umbrella.

## RFC Documentation

Add `// ISO/IEC 10589 §8.4.5: "<quoted requirement>"` above the election and pseudo-node origination code, and `// RFC 5305` above the TLV 22 pseudo-node neighbour encoding.
MUST document: election order (priority then MAC), per-level independence, re-election triggers (DIS loss, priority change), pseudo-node origination/purge on role change.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| One DIS elected per level on a LAN | functional test | `test/isis/isis-dis.ci` |
| Pseudo-node LSP reflects all members | functional test | `test/isis/isis-dis.ci` |
| Re-election on priority change and DIS loss | unit test | `TestDISReElectOnPriorityChange`, `TestDISReElectOnLoss` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/isis/circuit/`, `internal/component/isis/lsdb/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features (out-of-scope MT DIS honoured)
- [ ] Single responsibility (election in circuit, origination in lsdb)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (reads spec-isis-5 table; reuses spec-isis-6/7 paths)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A -- owned by spec-isis-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-isis-8-dis-broadcast.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-isis-8-dis-broadcast.md` only
