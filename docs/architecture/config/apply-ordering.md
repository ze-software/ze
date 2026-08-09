# Config Apply Ordering: the operation graph

Config reload applied changes surface by surface with no cross-surface order.
Dependent operations could run in the wrong order: add an interface after the
BGP peer that binds it, tear a peer down before removing its address, or swap
two interface addresses with no dual-presence window. The operation graph
replaced that ad-hoc order.

## The pipeline

`BuildOperationGraph` builds the graph. `TopologicalSort` orders it. The
executor runs `Verify`, then `Execute`, then `Commit`.

<!-- source: internal/component/config/transaction/operation.go -- Operation, the graph foundation -->
<!-- source: internal/component/config/transaction/depgraph.go -- BuildOperationGraph -->
<!-- source: internal/component/config/transaction/solver.go -- TopologicalSort, cycle relaxation -->
<!-- source: internal/component/config/transaction/executor.go -- Verify, Execute, Commit, rollback -->

## The decisions

**Decomposition and constraint rules live in the owning component, never in a
central switch.** `iface` and `bgp` each register their own decomposition
through `init()`. Remove a component and its operation handling goes with it.
<!-- source: internal/component/iface/operation.go -- iface-owned decomposition -->
<!-- source: internal/component/bgp/plugin/operation.go -- BGP-owned decomposition -->
<!-- source: internal/component/bgp/reactor/operation.go -- peer add, remove and modify primitives -->

**`verify -> execute -> commit` with rollback, not best-effort apply.** Rollback
replays the inverse operations in reverse order and excludes the operation that
failed.

**Address-only cross-interface cycles relax. Everything else is rejected.** A
swap of two addresses between interfaces is a cycle by construction. The solver
breaks it with `AllowDual`, which permits both addresses to be present for the
duration of the swap. A cycle that is not address-only, or that is inside one
interface, is rejected instead of relaxed.

**Settlement waiters are armed before the apply**, so a readiness event that
arrives fast is not missed.

The graph is reached from the real reload path: `runTxCoordinator` calls
`SetOperationPlanner`, and the orchestrator runs the operation path.
<!-- source: internal/component/plugin/server/reload_tx.go -- runTxCoordinator -->
<!-- source: internal/component/plugin/server/reload_tx.go -- SetOperationPlanner wiring -->
<!-- source: internal/component/config/transaction/orchestrator.go -- the operation path -->

The external contract is mandatory for v1 plugins: SDK types in
`pkg/plugin/sdk/sdk_types.go`, RPC transport in `internal/component/plugin/ipc/rpc.go`
(`config-operation-decompose`, `verify`, `apply`, `rollback`, `commit`), bridge in
`internal/component/plugin/server/config_tx_bridge.go`.

## What the tests do not reach

The functional rotation, swap and reip tests rotate BGP router-ids. They emit
`REMOVE_PEER` and `ADD_PEER` only, never `ADD_ADDRESS` or `REMOVE_ADDRESS`, so
they exercise the whole decompose to graph to executor to bridge to RPC to
reactor path and none of the `AllowDual` machinery. That machinery is covered by
`TestTopologicalSortCycleResolution` (2-way swap) and
`TestTopologicalSortThreeWayRotation` (three `AllowDual`) in `solver_test.go`.
The create and delete tests emit real iface interface operations and are
Linux-only.

The operation-count warning at 10000 and the cycle-depth rejection at 100 are
named in the design and not enforced in the graph builder. Config is operator
supplied and bounded, so neither is a security boundary.

The step-3 owner state check is not implemented in `armSettlementWaiters`, which
subscribes and nothing more. The current decomposers never emit an `ADD_ADDRESS`
for an address that is already present.
