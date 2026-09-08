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

**Every participant with a diff is a node, decomposer or not.** A participant
the planner produced no operation for gets one COARSE node, synthesized by
`operationNodes`, which the executor applies through that participant's section
apply. So a root nobody decomposes is ordered relative to the other roots, and
applied by the `config-apply` callback every plugin implements.

Coverage is what makes the ordering unconditional. Until 2026-09-08 the
orchestrator abandoned the operation path for the WHOLE transaction as soon as
one participant could not be decomposed, and fell back to the unordered section
apply, so the operations that were ordered correctly lost their order with it.
Every reload took that fallback in practice: a dozen plugins declare the `bgp`
root and none of them decomposes. The fallback is deleted, and a coarse node is
what replaced it.
<!-- source: internal/component/config/transaction/orchestrator.go -- operationNodes -->
<!-- source: internal/component/config/transaction/executor.go -- applySection -->

**A coarse node's position is a tie-break, never an ordering claim.** It sits
after the decomposed operations, because an unconstrained node keeps its slice
position through the sort and a root that declares no operations consumes the
resources the decomposed roots produce more often than it produces them. An
edge decides the order wherever one exists.
<!-- source: internal/component/iface/operation.go -- iface-owned decomposition -->
<!-- source: internal/component/bgp/plugin/operation.go -- BGP-owned decomposition -->
<!-- source: internal/component/bgp/reactor/operation.go -- peer add, remove and modify primitives -->

**`verify -> execute -> commit` with rollback, not best-effort apply.** Rollback
replays the inverse operations in reverse order and excludes the operation that
failed. A coarse node's inverse is the transaction-wide section rollback the
orchestrator broadcasts, so the executor emits no per-operation rollback for it.

**A modify-peer swaps the running session's settings in place where the change
allows it.** The decision is `peerSettingsSwapPlan`, the same one the section
apply's reconcile takes, so which path a reload travels does not decide whether
the operator's session bounces.
<!-- source: internal/component/bgp/reactor/operation.go -- swapPeerForOperation -->
<!-- source: internal/component/bgp/reactor/peer_settings_apply.go -- peerSettingsSwapPlan -->

**A test that reaches a plugin: `test/reload/config-apply-ordering-coarse-root.ci`.**
It changes one root nothing decomposes and asserts the line the receiving plugin
prints from its own `config-apply` handler.

**The engine orders by a verb and a resource kind, never by an operation
label.** An operation carries `create`, `destroy` or `modify` plus the kind of
the resource it targets, and that pair is the whole vocabulary the graph and the
solver read. The label stays on the operation as the emitting component's own
word for the work, and the owner dispatches on it. So a root joins the ordering
by declaring a verb, not by adding a constant to a list the engine owns: the
list is gone, and `interface` and `bgp` now spell their own labels.
<!-- source: pkg/plugin/rpc/types.go -- OperationVerb, ConfigOperation -->
<!-- source: internal/core/bgp/configop/configop.go -- the bgp root's labels -->

**An operation with no verb aborts the transaction.** It is refused at planning,
with an error naming the plugin, the root and the operation id, and never read
as `modify`: a default would give a create the dependencies of a change in
place, which is the silently wrong value `ai/rules/principles.md` bans. Ze is
pre-release with no shipped external plugin, so no compatibility shim accepts a
payload without one.
<!-- source: internal/component/config/transaction/operation.go -- ValidateOperationVerbs -->

**Address-only cross-interface cycles relax. Everything else is rejected.** A
swap of two addresses between interfaces is a cycle by construction. The solver
breaks it with `AllowDual`, which permits both addresses to be present for the
duration of the swap. "Address operation" is a verb and a kind: an operation
that creates or destroys a resource of kind `address`, whatever it is labelled.
A cycle that is not address-only, or that is inside one interface, is rejected
instead of relaxed.

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
`internal/component/plugin/server/config_tx_bridge.go`. The contract carries the
kebab-case keys `verb`, `produces` and `consumes` beside `type`, and a payload
that carries no `verb` is refused rather than ordered. The SDK re-exports the
three verb values and no operation label: a label belongs to the plugin that
emits it.

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
