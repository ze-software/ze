# Spec: Config Apply Ordering (Operation Graph Solver)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/7 |
| Updated | 2026-05-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/transaction-protocol.md` - current transaction system
4. `internal/component/config/transaction/orchestrator.go` - current orchestrator
5. `internal/component/iface/register.go` - iface plugin apply
6. `internal/component/bgp/plugin/register.go` - BGP plugin apply
7. `internal/component/bgp/reactor/reactor_iface.go` - BGP/iface event coordination
8. `internal/component/iface/migrate_linux.go` - MigrateInterface make-before-break pattern

## Task

Replace the current "give each plugin its full config diff" apply model with an operation-level
decomposition that enforces correct ordering of config changes across components.

Today, the transaction orchestrator emits `apply-<plugin>` to each plugin concurrently (within a
dependency tier). Each plugin receives its full DiffSection and applies it as a monolithic batch.
No plugin reads the Added/Removed/Changed fields. Cross-component dependencies (e.g., BGP needs
an interface address to exist before binding a listener) are handled by EventBus side-effects, but
nobody waits for settlement during the commit path.

This fails for interleaved cases: re-IPing an interface with a BGP peer change requires
iface-add, bgp-remove, bgp-add, iface-remove, in that order. IP swaps across interfaces create
dependency cycles requiring multi-round execution.

The solution: decompose config diffs into typed atomic operations, build a dependency graph using
constraint rules, topologically sort (with cycle detection + fallback), and execute operations in
order with settlement between dependent steps.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/transaction-protocol.md` - current transaction system
  -> Constraint: plugins receive full DiffSection via apply-<plugin> events, ack with apply-ok/apply-failed
  -> Constraint: orchestrator uses TopologicalTiers for plugin ordering, reverse-tier for rollback
  -> Decision: plugin autonomy: "Plugins decide what they need" -- this spec changes that for ordering
- [ ] `docs/architecture/config/yang-config-design.md` - YANG config model
  -> Constraint: YANG schema defines the config tree structure, used by diff engine
- [ ] `docs/architecture/core-design.md` - registration pattern
  -> Constraint: components register via init() in register.go, core discovers through registries

### Learned Summaries
- [ ] `plan/learned/537-config-tx-protocol.md` - transaction protocol design history
  -> Decision: EventGateway interface for testability, per-plugin event types, reverse-tier rollback
  -> Decision: bus migration to EventBus, stream-based coordination
- [ ] `plan/learned/535-config-tx-consumers.md` - how plugins participate in transactions
  -> Constraint: all 5 config-owning plugins use full-state reconciliation from pendingCfg, ignore DiffSection
  -> Decision: ConfigJournal interface in registry package to avoid import cycles
- [ ] `plan/learned/492-iface-3-bgp-react.md` - BGP reactions to interface events
  -> Decision: BGP subscribes to addr-added/addr-removed, starts/stops listeners reactively
  -> Constraint: reactor handler must not hold r.mu during EventBus operations (deadlock risk)
- [ ] `plan/learned/779-transactional-config-commit.md` - transactional commit across all surfaces
  -> Decision: runtime is authoritative, candidate promoted only after reload succeeds
- [ ] `plan/learned/758-config-graph.md` - config dependency graph
  -> Constraint: graph has 7 edge types but NO address nodes and NO uses-address edges
  -> Decision: graph derived from validation code, not declared separately

**Key insights:**
- No plugin reads DiffSection.Added/Removed/Changed; all use full-state reconciliation
- EventBus Emit is synchronous, but addr-added comes from async netlink monitor, not applyConfig
- MigrateInterface implements correct make-before-break via subscribe+wait+timeout on listener-ready
- Config graph exists but needs address nodes and uses-address edges for constraint solver
- Dependency chains are up to 4 deep (Interface -> Address -> BGP Listener -> BGP Peer -> BFD)
- No atomic cross-interface address move at netlink/VPP level; make-before-break is the only pattern
- FRR uses destroy-first ordering (opposite of make-before-break) and accepts session loss
- VyOS uses static priority numbers; neither FRR nor VyOS handles dependency cycles

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/transaction/orchestrator.go` - TxCoordinator.Execute: verify -> apply -> commit
  -> Constraint: runApply iterates participants, emits apply-<plugin> for all, waits for all acks
  -> Constraint: no ordering within a tier, no add/remove phase split
- [ ] `internal/component/config/transaction/types.go` - DiffSection has Root, Added, Removed, Changed
  -> Constraint: fields exist but are dead data at consumer side
- [ ] `internal/component/iface/register.go` - OnConfigApply calls applyConfig(cfg, previousCfg, b)
  -> Constraint: monolithic batch, no EventBus settlement wait between add and remove
- [ ] `internal/component/iface/config_apply.go` - applyConfig phases: create -> properties -> add-addr -> remove-addr -> delete
  -> Decision: iface already does add-before-remove internally, but no cross-component wait
- [ ] `internal/component/bgp/plugin/register.go` - OnConfigApply calls ReconcilePeersWithJournal
  -> Constraint: BGP does not emit events during apply; listener-ready only fires reactively
- [ ] `internal/component/bgp/reactor/reactor_iface.go` - SubscribeInterfaceEvents
  -> Decision: BGP subscribes to addr-added/removed, starts/stops listeners
  -> Constraint: addr-added comes from netlink monitor (async), not from applyConfig
- [ ] `internal/component/iface/migrate_linux.go` - MigrateInterface 5-phase protocol
  -> Decision: subscribe to listener-ready, select with timeout, rollback on failure
  -> Constraint: only available via explicit RPC, not wired into commit path

**Behavior to preserve:**
- Transaction protocol phases (verify -> apply -> commit/rollback)
- Plugin registration pattern (init() in register.go)
- EventBus event types and namespaces (interface/addr-added, bgp/listener-ready, etc.)
- Journal-based rollback for each plugin
- Tiered deadline computation from TopologicalTiers
- Reverse-tier rollback ordering

**Behavior to change:**
- Replace monolithic per-plugin apply with operation-level execution
- Orchestrator drives individual operations, not whole-config apply events
- Add settlement waits between dependent operations
- Handle dependency cycles with fallback to temporary dual-presence

## Data Flow (MANDATORY)

### Entry Point
- CLI `commit` / web commit / API commit / SIGHUP / managed push
- Config diff computed by comparing candidate tree vs active tree
- Diff sections keyed by config root (interface, bgp, routing, service, etc.)

### Transformation Path
1. **Config diff** (existing): candidate tree vs active tree -> `map[string][]DiffSection`
2. **Operation decomposition** (NEW): DiffSection -> `[]ConfigOperation` (typed atomic operations)
3. **Dependency graph** (NEW): `[]ConfigOperation` -> DAG with edges from constraint rules
4. **Cycle detection** (NEW): find cycles, insert fallback operations if needed
5. **Topological sort** (NEW): DAG -> ordered `[]ConfigOperation`
6. **Execution** (NEW): iterate operations, call plugin handlers, wait for settlement between dependent ops
7. **Settlement** (NEW): after each operation, wait for side-effect events (subscribe + timeout)
8. **Ack/rollback**: on failure, reverse executed operations via journals

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Orchestrator -> Plugin | Operation event via EventGateway (replaces apply-<plugin>) | [ ] |
| Plugin -> Kernel | Backend calls (AddAddress, RemoveAddress, etc.) | [ ] |
| Kernel -> Netlink Monitor | Async netlink events | [ ] |
| Netlink Monitor -> EventBus | addr-added/addr-removed events | [ ] |
| EventBus -> BGP Reactor | Synchronous handler delivery | [ ] |
| BGP Reactor -> EventBus | listener-ready event | [ ] |
| Orchestrator <- EventBus | Settlement subscription (wait for side-effect completion) | [ ] |

### Integration Points
- `TxCoordinator.Execute` - currently drives verify/apply/commit; will drive operation execution
- `EventGateway` - currently emits apply-<plugin>; will emit per-operation events
- `registry.TopologicalTiers` - used for tier ordering; operations add finer-grained ordering within tiers
- `ze config graph` - existing dependency graph; extended with address nodes for constraint rules
- `MigrateInterface` pattern - settlement primitive (subscribe + select + timeout) reused generically

### Architectural Verification
- [ ] No bypassed layers (operations flow through EventGateway like current apply events)
- [ ] No unintended coupling (constraint rules are data, not hardcoded cross-imports)
- [ ] No duplicated functionality (extends existing orchestrator, does not recreate it)
- [ ] Zero-copy preserved where applicable (operations reference config subtrees, not copies)

## Constraint Rules

### Resource Lifecycle Rules

| ID | Rule | Rationale |
|----|------|-----------|
| R1 | Interface must exist before addresses can be added to it | Kernel rejects ADDR_ADD on missing device |
| R2 | Address must be assigned before a service can bind to it | BGP listener, BFD session need the address |
| R3 | All services on an address must be removed before the address is removed | Avoids TCP errors, session drops |
| R4 | BGP peers must be drained (NOTIFICATION cease) before their listener is stopped | RFC 4486 graceful cease |
| R5 | IP address on only one interface at a time (hard); fall back to temporary dual-presence if cycle requires it | Routing ambiguity, ARP confusion |
| R6 | Underlying interface must exist before tunnels referencing it | GRE/XFRM/WG physical-dev or local-interface |
| R7 | Bridge members must exist before being added to a bridge | Kernel rejects missing enslaved devices |
| R8 | DHCP client requires interface to exist and be admin UP | Cannot send DHCP packets without carrier |
| R9 | Static route next-hop should be reachable when route is installed | Unreachable next-hop creates blackhole |
| R10 | Connected routes are implicit, auto-follow address add/remove | No explicit ordering needed, driven by EventBus |

### Operation Ordering Rules (derived from resource rules)

| ID | Constraint | Produces edge |
|----|-----------|---------------|
| O1 | ADD_INTERFACE before ADD_ADDRESS on same interface | ADD_INTERFACE -> ADD_ADDRESS |
| O2 | ADD_ADDRESS before ADD_PEER using it as local-address | ADD_ADDRESS -> ADD_PEER |
| O3 | REMOVE_PEER before REMOVE_ADDRESS it depends on | REMOVE_PEER -> REMOVE_ADDRESS |
| O4 | REMOVE_ADDRESS before REMOVE_INTERFACE (clear before delete) | REMOVE_ADDRESS -> REMOVE_INTERFACE |
| O5 | ADD_INTERFACE before ADD_TUNNEL referencing it | ADD_INTERFACE -> ADD_TUNNEL |
| O6 | ADD_ADDRESS before ADD_LISTENER on that address | ADD_ADDRESS -> ADD_LISTENER |
| O7 | REMOVE_LISTENER before REMOVE_ADDRESS | REMOVE_LISTENER -> REMOVE_ADDRESS |
| O8 | ADD_INTERFACE before ADD_BRIDGE_MEMBER | ADD_INTERFACE -> ADD_BRIDGE_MEMBER |
| O9 | REMOVE_BRIDGE_MEMBER before REMOVE_INTERFACE | REMOVE_BRIDGE_MEMBER -> REMOVE_INTERFACE |
| O10 | ADD_ADDRESS before ADD_STATIC_ROUTE with that next-hop | ADD_ADDRESS -> ADD_STATIC_ROUTE |
| O11 | REMOVE_STATIC_ROUTE before REMOVE_ADDRESS of next-hop | REMOVE_STATIC_ROUTE -> REMOVE_ADDRESS |

### Settlement Rules

| ID | After operation | Wait for | Timeout |
|----|----------------|----------|---------|
| S1 | ADD_ADDRESS (iface) | (interface, addr-added) from netlink monitor | 5s |
| S2 | ADD_PEER with local-address | (bgp, listener-ready) if new listener needed | 10s |
| S3 | REMOVE_PEER | BGP NOTIFICATION sent and peer FSM reaches Idle | 5s |
| S4 | ADD_INTERFACE | (interface, created) from netlink monitor | 5s |

### Cycle Detection and Resolution

When the operation graph contains a cycle (e.g., IP swap: A:1->2, B:2->1):

1. **Detect**: standard cycle detection during topological sort
2. **Classify**: determine if R5 can be relaxed (temporary dual-presence)
3. **Break**: if relaxable, insert ADD_ADDRESS operations that accept dual-presence, execute the cycle as: add-new-everywhere -> move-services -> remove-old-everywhere
4. **Fail**: if not relaxable (e.g., conflicting routing), reject the commit at verify time with an explanation of the cycle

### Dependency Chain Examples

**Re-IP with peer change** (interface eth0: 10.0.0.1->10.0.0.2, peer local-address changes):
```
1. ADD_ADDRESS(eth0, 10.0.0.2)     -- add new IP
2. [settle: wait for addr-added]
3. REMOVE_PEER(old-peer)           -- drain old peer (depends on: nothing blocking)
4. ADD_PEER(new-peer, local=10.0.0.2) -- add new peer (depends on: step 1)
5. [settle: wait for listener-ready]
6. REMOVE_ADDRESS(eth0, 10.0.0.1)  -- remove old IP (depends on: step 3, no services left)
```

**IP swap** (A:1->2, B:2->1):
```
-- Cycle detected: REMOVE_ADDRESS(B,2) blocks ADD_ADDRESS(A,2) via R5,
-- but ADD_ADDRESS(B,1) blocks REMOVE_ADDRESS(A,1) via R5.
-- R5 fallback: allow temporary dual-presence.
1. ADD_ADDRESS(A, 2) [dual-presence: 2 exists on both A and B temporarily]
2. ADD_ADDRESS(B, 1) [dual-presence: 1 exists on both A and B temporarily]
3. [settle: wait for addr-added events]
4. MOVE_SERVICES(from B:2 to A:2)  -- BGP peers, listeners
5. MOVE_SERVICES(from A:1 to B:1)
6. [settle: wait for listener-ready events]
7. REMOVE_ADDRESS(B, 2)
8. REMOVE_ADDRESS(A, 1)
```

**Three-way rotation** (A:1->2, B:2->3, C:3->1):
```
-- Same pattern: add all new, move services, remove all old
1. ADD_ADDRESS(A, 2) [dual]
2. ADD_ADDRESS(B, 3) [dual]
3. ADD_ADDRESS(C, 1) [dual]
4. [settle]
5. MOVE_SERVICES(B:2 -> A:2)
6. MOVE_SERVICES(C:3 -> B:3)
7. MOVE_SERVICES(A:1 -> C:1)
8. [settle]
9. REMOVE_ADDRESS(B, 2)
10. REMOVE_ADDRESS(C, 3)
11. REMOVE_ADDRESS(A, 1)
```

## Operation Types

| Operation | Component | Parameters | Rollback |
|-----------|-----------|------------|----------|
| ADD_INTERFACE | iface | name, type (dummy/veth/bridge/tunnel/wg/xfrm), spec | DELETE_INTERFACE |
| REMOVE_INTERFACE | iface | name | recreate with spec from old config |
| ADD_ADDRESS | iface | interface, cidr, allow-dual (bool) | REMOVE_ADDRESS |
| REMOVE_ADDRESS | iface | interface, cidr | ADD_ADDRESS |
| SET_PROPERTY | iface | interface, property, value | SET_PROPERTY with old value |
| ADD_BRIDGE_MEMBER | iface | bridge, member | REMOVE_BRIDGE_MEMBER |
| REMOVE_BRIDGE_MEMBER | iface | bridge, member | ADD_BRIDGE_MEMBER |
| ADD_PEER | bgp | peer config (address, local-address, group, families) | REMOVE_PEER |
| REMOVE_PEER | bgp | peer address | ADD_PEER with old config |
| MODIFY_PEER | bgp | peer address, changed fields | MODIFY_PEER with old fields |
| ADD_LISTENER | bgp | address, port | REMOVE_LISTENER |
| REMOVE_LISTENER | bgp | address, port | ADD_LISTENER |
| ADD_STATIC_ROUTE | static | prefix, next-hop, metric | REMOVE_STATIC_ROUTE |
| REMOVE_STATIC_ROUTE | static | prefix, next-hop | ADD_STATIC_ROUTE with old config |
| SET_ADMIN_DISTANCE | sysrib | protocol, distance | SET_ADMIN_DISTANCE with old value |
| SET_SYSCTL | sysctl | key, value | SET_SYSCTL with old value |
| START_DHCP | iface | interface, unit, params | STOP_DHCP |
| STOP_DHCP | iface | interface, unit | START_DHCP with old params |
| ADD_TUNNEL | iface | tunnel spec | REMOVE_TUNNEL |
| REMOVE_TUNNEL | iface | name | ADD_TUNNEL with old spec |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `commit` with iface+BGP change | -> | Operation decomposer + solver + executor | `test-config-apply-ordering-reip.ci` |
| CLI `commit` with IP swap | -> | Cycle detection + dual-presence fallback | `test-config-apply-ordering-swap.ci` |
| CLI `commit` with interface create + BGP peer | -> | ADD_INTERFACE -> ADD_ADDRESS -> ADD_PEER ordering | `test-config-apply-ordering-create.ci` |
| CLI `commit` with interface delete + BGP peer remove | -> | REMOVE_PEER -> REMOVE_ADDRESS -> REMOVE_INTERFACE ordering | `test-config-apply-ordering-delete.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Commit changes interface IP and BGP peer local-address | Operations execute in order: add-addr, remove-old-peer, add-new-peer, settle, remove-old-addr. BGP session on new address established before old address removed. |
| AC-2 | Commit swaps IPs between two interfaces with BGP peers | Cycle detected, dual-presence fallback used. Both BGP sessions survive the transition (no NOTIFICATION cease). |
| AC-3 | Commit creates new interface, adds address, adds BGP peer | Operations ordered: create-iface -> add-addr -> settle -> add-peer. No ordering violation. |
| AC-4 | Commit removes BGP peer, removes address, removes interface | Operations ordered: remove-peer -> settle -> remove-addr -> remove-iface. Address not removed while peer still active. |
| AC-5 | Commit with no cross-component dependencies | Falls back to current behavior (concurrent apply within tier). No regression. |
| AC-6 | Operation fails mid-execution | All previously executed operations rolled back via journals in reverse order. State returns to pre-commit. |
| AC-7 | Settlement timeout (e.g., addr-added never arrives) | Commit fails with descriptive error. Rollback of completed operations. |
| AC-8 | Constraint rules are data, not hardcoded | Rules defined in a registry, discoverable. New rules can be added by registering them. |
| AC-9 | Three-way IP rotation across three interfaces | Cycle detected, dual-presence fallback. All three BGP sessions survive. |
| AC-10 | Config diff with no address or peer changes | No operations generated beyond current behavior. Zero overhead for simple changes. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecomposeAddInterface` | `internal/component/config/transaction/decompose_test.go` | DiffSection -> ADD_INTERFACE operation | |
| `TestDecomposeAddAddress` | same | DiffSection -> ADD_ADDRESS operation | |
| `TestDecomposeRemovePeer` | same | DiffSection -> REMOVE_PEER operation | |
| `TestDecomposeReIP` | same | Combined iface+BGP diff -> correct operation set | |
| `TestDependencyGraphSimple` | `internal/component/config/transaction/depgraph_test.go` | O1-O11 rules produce correct edges | |
| `TestDependencyGraphCycle` | same | IP swap detected as cycle | |
| `TestCycleFallback` | same | Cycle broken with dual-presence operations | |
| `TestTopologicalSort` | same | Operations sorted respecting all edges | |
| `TestExecutorOrdering` | `internal/component/config/transaction/executor_test.go` | Operations execute in sorted order | |
| `TestExecutorSettlement` | same | Executor waits for settlement events between dependent ops | |
| `TestExecutorTimeout` | same | Settlement timeout triggers rollback | |
| `TestExecutorRollback` | same | Failed operation rolls back all prior operations in reverse | |
| `TestNoOpPassthrough` | same | Config with no ordering-sensitive changes uses current path | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Settlement timeout | 1s-60s | 60s | 0s (rejected) | N/A (capped) |
| Operation count per commit | 0-10000 | 10000 | N/A | warn at 10000 |
| Cycle depth | 0-100 | 100 | N/A | reject at 100 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-config-apply-ordering-reip` | `test/reload/test-config-apply-ordering-reip.ci` | Change interface IP and BGP peer, verify session survives | |
| `test-config-apply-ordering-swap` | `test/reload/test-config-apply-ordering-swap.ci` | Swap IPs between two interfaces, verify both sessions survive | |
| `test-config-apply-ordering-create` | `test/reload/test-config-apply-ordering-create.ci` | Create interface + add peer, verify correct ordering | |
| `test-config-apply-ordering-delete` | `test/reload/test-config-apply-ordering-delete.ci` | Remove peer + remove interface, verify peer drained first | |
| `test-config-apply-ordering-no-change` | `test/reload/test-config-apply-ordering-nochange.ci` | Simple config change (no ordering deps), verify no regression | |

### Future
- Three-way rotation test (requires 3 interfaces + 3 peers, complex setup)
- VPP backend settlement test (requires QEMU + VPP)

## Files to Modify

- `internal/component/config/transaction/orchestrator.go` - extend Execute to use operation graph when ordering-sensitive changes detected
- `internal/component/config/transaction/types.go` - add ConfigOperation type, OperationType enum
- `docs/architecture/config/transaction-protocol.md` - document operation graph extension

## Files to Create

- `internal/component/config/transaction/operation.go` - ConfigOperation type, OperationType constants
- `internal/component/config/transaction/decompose.go` - diff -> operations decomposer
- `internal/component/config/transaction/depgraph.go` - dependency graph builder from constraint rules
- `internal/component/config/transaction/solver.go` - topological sort, cycle detection, fallback
- `internal/component/config/transaction/executor.go` - operation executor with settlement
- `internal/component/config/transaction/rules.go` - constraint rule registry (R1-R10, O1-O11, S1-S4)
- `internal/component/config/transaction/decompose_test.go`
- `internal/component/config/transaction/depgraph_test.go`
- `internal/component/config/transaction/executor_test.go`
- `internal/component/config/transaction/solver_test.go`
- `test/reload/test-config-apply-ordering-reip.ci`
- `test/reload/test-config-apply-ordering-swap.ci`
- `test/reload/test-config-apply-ordering-create.ci`
- `test/reload/test-config-apply-ordering-delete.ci`
- `test/reload/test-config-apply-ordering-nochange.ci`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | No new config nodes |
| CLI commands/flags | No | Transparent to user |
| Functional test | Yes | `test/reload/*.ci` |
| Doctor check | No | No new runtime dependencies |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Transparent to user |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/transaction-protocol.md` |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |

### Implementation Phases

1. **Phase: Operation types and constraint rules** -- define types, register rules
   - Tests: TestDecomposeAddInterface, TestDependencyGraphSimple
   - Files: operation.go, rules.go
   - Verify: types compile, rules produce correct edges in unit tests

2. **Phase: Decomposer** -- config diff -> operations
   - Tests: TestDecomposeReIP, TestDecomposeAddAddress, TestDecomposeRemovePeer
   - Files: decompose.go, decompose_test.go
   - Verify: re-IP scenario produces correct 6-operation sequence

3. **Phase: Dependency graph + solver** -- build DAG, detect cycles, topological sort
   - Tests: TestDependencyGraphCycle, TestCycleFallback, TestTopologicalSort
   - Files: depgraph.go, solver.go, depgraph_test.go, solver_test.go
   - Verify: IP swap cycle detected and resolved

4. **Phase: Executor with settlement** -- execute operations in order, wait for events
   - Tests: TestExecutorOrdering, TestExecutorSettlement, TestExecutorTimeout, TestExecutorRollback
   - Files: executor.go, executor_test.go
   - Verify: operations execute in order with settlement waits

5. **Phase: Wire into orchestrator** -- integrate with TxCoordinator.Execute
   - Tests: TestNoOpPassthrough
   - Files: orchestrator.go
   - Verify: ordering-sensitive commits use operation graph; simple commits use current path

6. **Functional tests** -- end-to-end scenarios
   - Files: test/reload/*.ci
   - Verify: all AC scenarios pass

7. **Full verification** -> `make ze-verify`

### Critical Review Checklist
| Check | What to verify |
|-------|---------------|
| Completeness | Every AC has implementation with file:line |
| Correctness | Operation ordering matches constraint rules exactly |
| Performance | Simple commits (no ordering deps) have zero overhead |
| Rollback | Every operation has a correct inverse; reverse execution tested |
| Settlement | Timeout handling does not leave state half-applied |
| Cycle detection | All cycle patterns (2-way swap, 3-way rotation, deeper) handled |
| Rule: no-layering | Old monolithic apply path removed for ordering-sensitive cases |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| ConfigOperation type | `grep -r "type ConfigOperation" internal/` |
| Constraint rules registry | `grep -r "RegisterRule\|ConstraintRule" internal/` |
| Decomposer | `grep -r "func Decompose" internal/` |
| Dependency graph builder | `grep -r "func BuildDepGraph" internal/` |
| Cycle detection | `grep -r "DetectCycles\|CycleBreaker" internal/` |
| Executor | `grep -r "func.*Execute.*Operation" internal/` |
| Settlement | `grep -r "AwaitEvent\|Settlement" internal/` |
| Functional tests | `ls test/reload/test-config-apply-ordering-*.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Operation parameters validated before execution (no injection via config values) |
| Resource exhaustion | Cycle detection bounded (max 100 depth); operation count bounded |
| Rollback completeness | No state leak on partial failure; all operations journaled |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Cycle detection misses a pattern | Add test case, extend solver |
| Settlement timeout in functional test | Increase timeout or fix event emission |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Operation graph decomposition (A) | Phased apply with two phases (B), Multi-round intermediate configs (C) | Only A can express interleaved cross-component ordering (iface-add, bgp-remove, bgp-add, iface-remove) |
| R5 hard with soft fallback | R5 always hard (reject cycles), R5 always soft, R5 configurable per-address | Try hard first (find cycle-free ordering), fall back to temporary dual-presence if cycle requires it |
| Constraint rules as data registry | Hardcoded ordering in orchestrator, per-plugin declared ordering | Rules as data: testable, extensible, discoverable. New components register new rules. |
| Settlement via subscribe+wait+timeout | Synchronous emit (doesn't work: netlink monitor is async), No settlement (FRR approach) | MigrateInterface already uses this pattern successfully. Generic AwaitEvent primitive. |

## Known Limitations
- Settlement depends on netlink monitor responsiveness; slow monitors increase commit time
- Dual-presence fallback may cause brief routing ambiguity during IP swaps
- Operation decomposition requires understanding config semantics (not purely schema-driven)
- External plugins cannot participate in operation-level execution (they still get full apply events)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

## Review Resolution 2026-05-27

This section resolves the critical review findings without deleting earlier design text. Where this section conflicts with an earlier section, this section supersedes it.

### Position: External Plugins Receive Operation Callbacks

External plugins MUST receive operation callbacks too. The operation solver changes the config transaction contract, not only the in-process implementation. If external plugins only receive the old full-diff apply callback, the system has two config semantics and cannot guarantee ordering, rollback, or exact failure behavior for config roots owned by external plugins.

| Decision | Requirement |
|----------|-------------|
| External parity | Internal and external plugins use the same operation callback contract. |
| Transport | The orchestrator still publishes through `EventGateway`; the production bridge translates operation events to SDK/RPC callbacks, as it does today for verify/apply/rollback. |
| Capability declaration | Plugins declare operation support during Stage 1. The declaration includes roots, operation types, and whether the plugin can decompose its root. |
| Exact or reject | If an ordering-sensitive commit targets a root whose owner cannot provide the needed operation callbacks, verify rejects the commit with a clear error. |
| Fallback | The old full-diff apply path remains valid only for transactions with no ordering-sensitive operations and no dependencies crossing into opaque roots. |
| Payload shape | Operation payloads are self-contained value types crossing the plugin boundary. No pointers, no component-owned handles. |

### Operation Callback Contract

The spec needs a concrete plugin contract before implementation starts. This contract is now mandatory.

| Callback | Direction | Purpose | Applies to external plugins |
|----------|-----------|---------|-----------------------------|
| `config-operation-decompose` | engine to plugin | Convert old root, candidate root, and diff section into typed operations owned by that plugin. | Yes |
| `config-operation-verify` | engine to plugin | Validate one operation before graph execution. This catches unsupported backend features and impossible inverses before mutation. | Yes |
| `config-operation-apply` | engine to plugin | Apply one operation and record its inverse in the plugin journal for the transaction. | Yes |
| `config-operation-rollback` | engine to plugin | Roll back already-applied operations for this transaction in reverse order, or apply explicit inverse operations when supplied by the executor. | Yes |
| `config-operation-commit` | engine to plugin | Finalize operation journals and promote pending config state after all operations succeed. | Yes |

The existing full-config `OnConfigVerify` remains the first verification gate. It validates the candidate as a whole and prepares pending state. Operation decomposition and operation verify run after full verify and before mutation. This preserves candidate validation while allowing the executor to interleave mutations safely.

### Decomposition Ownership

Operation decomposition MUST be registered by the component or plugin that owns the config semantics. The transaction package owns the generic machinery only.

| Concern | Owner | Reason |
|---------|-------|--------|
| Operation model, IDs, graph, topological sort, rollback sequencing | `internal/component/config/transaction` or a leaf subpackage under it | Generic transaction machinery, no component-specific schema knowledge. |
| Interface operation decomposition | `internal/component/iface` | Only iface knows units, VLAN OS names, backend constraints, tunnels, DHCP side effects, and current apply ordering. |
| BGP operation decomposition | `internal/component/bgp` | Only BGP knows peer identity, local-address use, listener sharing, connection mode, and BFD coupling. |
| Static route operation decomposition | `internal/plugins/static` | Static routes own next-hop, BFD profile, and route table semantics. |
| Constraint rule registration | Each owner registers its own rule data at init or Stage 1 | Keeps rules discoverable without central hardcoding. |
| Cross-component solver | Transaction package | It combines registered operations and rules into one execution graph. |

This avoids putting iface/BGP/static schema parsing into the transaction package. The transaction package must never import those implementations directly. Internal components register decomposers through a leaf registry. External plugins provide decomposition through the SDK/RPC callback.

### Diff Input Contract for Decomposers

The earlier `DiffSection -> []ConfigOperation` shorthand is insufficient. Decomposers receive all of the following inputs:

| Input | Source | Why needed |
|-------|--------|------------|
| Active root subtree | current runtime config tree | Needed to build inverse operations and detect removals with old values. |
| Candidate root subtree | verified candidate config tree | Needed to build add/modify operations with full desired values. |
| DiffSection | existing diff builder | Useful for narrowing the work set. |
| Root name | participant registration | Allows root-specific decomposer lookup. |

`DiffSection.Added`, `Removed`, and `Changed` are JSON strings containing flat slash-separated config paths grouped by top-level root. `Changed` values are objects with old and new values. Decomposers MUST NOT rely on `DiffSection` alone when full config context is needed.

### Rollback Ownership

Operation rollback is plugin-owned but executor-ordered.

| Requirement | Detail |
|-------------|--------|
| Journal scope | Each plugin keeps a transaction journal containing only operations it has applied. |
| Operation inverse | Every applied operation records an exact inverse before the operation is reported successful. |
| Executor ordering | On failure, the executor sends rollback to plugins in the reverse order of applied operations, preserving cross-component dependencies. |
| Plugin rollback callback | The callback rolls back only operations from the transaction ID supplied by the executor. |
| Existing broadcast rollback | The existing transaction rollback event remains for full-diff fallback. Operation execution uses operation rollback acks so the executor knows which operation failed or rolled back. |
| Finalization | `config-operation-commit` promotes plugin pending state and discards operation journals only after all operations and settlements succeed. |

### Settlement Race Fix

Settlement waiters MUST be armed before the operation that can trigger the event.

| Step | Requirement |
|------|-------------|
| 1 | Build settlement predicates while building the operation graph. |
| 2 | Before applying an operation, subscribe to all events that can satisfy its dependent settlement predicates. |
| 3 | Immediately after subscribing, run an owner-provided state check to detect already-satisfied conditions. |
| 4 | Apply the operation. |
| 5 | Wait until predicates are satisfied or timeout expires. |
| 6 | Unsubscribe waiters before moving to the next operation. |

This supersedes the earlier wording that waits after each operation without specifying when subscriptions are installed.

### BGP Listener and Session Semantics

`listener-ready` cannot be assumed to fire after `ADD_PEER` in current code. The BGP operation handler must explicitly make readiness observable.

| Operation | Required readiness behavior |
|-----------|-----------------------------|
| `ADD_PEER` | If the operation creates or reuses a listener, BGP returns readiness in the operation result and emits `bgp/listener-ready` for the local address. |
| `ADD_ADDRESS` | If BGP starts a listener reactively from `interface/addr-added`, the existing `bgp/listener-ready` event remains the settlement signal. |
| `REMOVE_PEER` | Removal uses graceful teardown when a session exists, not bare `Stop`. It sends Cease with the appropriate config-change subcode, then waits for the peer FSM to leave Established or for the peer to be absent. |
| `REMOVE_LISTENER` | The listener is stopped only after no peers still depend on it. |

Session survival means the following:

| Case | Acceptance meaning |
|------|--------------------|
| Good case | If the TCP tuple can remain valid, the existing BGP TCP session stays Established throughout the operation sequence. |
| Bad case | If the tuple or local address must change, a replacement listener and replacement session must be ready before the old address or listener is removed. Graceful cease after replacement is acceptable. |
| Never acceptable | Removing the last viable local address or listener before a replacement exists, causing an avoidable period with no possible BGP session. |

AC-1, AC-2, and AC-9 must assert this definition rather than a blanket "no NOTIFICATION" rule for every topology.

### Current Behavior Correction

The current production bridge serializes verify/apply RPC dispatch in participant order because engine event handlers run synchronously. Therefore AC-5 is corrected:

| AC | Corrected expected behavior |
|----|-----------------------------|
| AC-5 | Transactions with no ordering-sensitive operations use the existing full-diff transaction path and preserve today's participant ordering and deadline behavior. No operation graph is built. Do not claim concurrent apply as the current production fallback. |

### Config Graph File Impact

The file plan is extended by this review section.

| File | Change |
|------|--------|
| `internal/component/config/graph.go` | Add address node kind and address-use edge kind if the solver reuses config graph data. |
| `internal/component/config/graph_test.go` | Cover address nodes and service address-use edges. |
| `internal/component/config/transaction/events/` | Add operation event type registration if operation callbacks use the existing config namespace event registry. |
| `pkg/plugin/rpc/` | Add operation callback request and response value types for external plugin transport. |
| `pkg/plugin/sdk/` | Add operation callback registration methods. |
| `internal/component/plugin/server/config_tx_bridge.go` | Translate operation events to SDK/RPC callbacks for internal and external plugins. |

### AC-9 Test Correction

AC-9 is in scope. The earlier Future row deferring the three-way rotation test is superseded.

| Test | Location | Status requirement |
|------|----------|--------------------|
| `test-config-apply-ordering-rotation` | `test/reload/test-config-apply-ordering-rotation.ci` | Mandatory before this spec can be closed. |

### Resolved Review Findings

| Finding | Resolution |
|---------|------------|
| Missing plugin operation contract | Added mandatory SDK/RPC operation callback contract for internal and external plugins. |
| Rollback underspecified | Added plugin-owned, executor-ordered operation rollback contract. |
| Settlement race | Settlement waiters are armed before operations and checked with state predicates. |
| `ADD_PEER` readiness mismatch | BGP operation handler must return readiness and emit `listener-ready` when applicable. |
| Diff format ambiguity | Decomposers receive active root, candidate root, root name, and diff; diff path format documented. |
| BGP drain mismatch | `REMOVE_PEER` must use graceful teardown and wait for peer departure. |
| Config graph file omission | Added graph files and operation event/RPC/SDK bridge files to impacted files. |
| AC-9 deferred while required | AC-9 rotation functional test is mandatory. |
| Current concurrency overstatement | AC-5 corrected to match current serialized bridge behavior. |

### Remaining Design Choices for Implementation

| Choice | Default for this spec | Reason |
|--------|-----------------------|--------|
| Registry package location | Add a small leaf registry under `internal/component/config/transaction` unless import cycles force `internal/component/config/operation`. | Keeps generic operation machinery close to the transaction coordinator while avoiding imports of component implementations. |
| Operation event names | Use `config` namespace with registered per-plugin operation event types, mirroring `verify-<plugin>` and `apply-<plugin>`. | Preserves the current EventGateway and bridge pattern. |
| External plugin without operation support | Reject only if its root participates in ordering-sensitive dependencies. Otherwise use full-diff fallback. | Preserves existing plugins where safe, but exact-or-reject for unsafe commits. |

### User Decisions 2026-05-27

| Decision | User choice | Effect |
|----------|-------------|--------|
| Decomposition ownership | Component-owned decomposition via registry. | iface, BGP, static, and external plugins own semantic decomposition; the transaction package owns only generic graph, solver, executor, settlement, and rollback machinery. |
| External callbacks | Mandatory in v1. | SDK/RPC operation callbacks are in the first implementation scope. Ordering-sensitive external roots cannot be handled by legacy full-diff apply. |
| BGP continuity | Use the current wording in this review section. | Tests require TCP sessions to stay Established when the tuple can remain valid; otherwise replacement listener/session readiness must precede old address/listener removal. |

## Review Gate

### Run 1 (closure review, 2026-07-03)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
Fresh read-only closure review of the committed implementation (commit `4cf41371c`):
wiring traced end-to-end from the real reload path (`plugin/server/reload.go`
`runTxCoordinator` -> `SetOperationPlanner` -> `orchestrator.runOperationPath` ->
`BuildOperationGraph` -> `TopologicalSort` -> executor `Verify/Execute/Commit`);
component-owned decomposers/rules registered and invoked (`iface/operation.go`,
`bgp/plugin/operation.go`, `bgp/reactor/operation.go`); rollback replays inverses in
reverse excluding the failed op; cycle relaxation limited to address-only
cross-interface cycles (`solver.go`). All 6 `test/reload/test-config-apply-ordering-*.ci`
present; rotation/swap/reip/nochange run on every platform, create/delete Linux-only.
**0 BLOCKER, 0 ISSUE.** Non-blocking limitations recorded in `plan/learned/1055-config-apply-ordering.md`
(IP-cycle/dual-presence covered by unit tests not functional; boundary caps
unenforced; settlement step-3 owner-check unimplemented).

### Final status
- [x] Closure review shows 0 BLOCKER, 0 ISSUE (committed-implementation review, 2026-07-03)
