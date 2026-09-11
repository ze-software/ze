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

**An ordering fact is DECLARED by the operation, and only what no declaration
can state is written as a rule.** Each operation names the resources it owns in
`Produces` and the resources it needs in `Consumes`, and the graph derives one
edge per producer and consumer pair over the same resource identity: a create
runs before what consumes it, and a destroy runs after every destroy that
consumes it. Nine hand-written rules said exactly that for two roots, and each
one had to spell the other root's operation labels to say it. They are deleted.

Two rules survive, both in `iface`, and both state a fact about two operations
over DIFFERENT resources, which no pair of declarations can carry: one address
lives on one interface (`iface-remove-address-before-add-same-address`), and an
interface is never left without an address
(`iface-add-address-before-remove-same-interface`). A root that adds a rule
today is almost certainly describing a produce and consume fact instead.
<!-- source: internal/component/config/transaction/depgraph.go -- addDerivedEdges -->

**A declared resource that names nothing orders nothing.** An entry carrying no
identifying value has an empty identity, `ValidateOperations` refuses the
operation that declares one, and the graph index leaves it out. An operation
crosses a JSON boundary from a plugin process, so an entry read as "any
resource" would let a hostile plugin order itself against every operation in the
transaction.

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

**A remove-peer's inverse restores the peer the reactor was running.** The
settings come from that peer, not from the config subtree the operation carries.
The routes a peer originates, its filter chains, its loop-detection policy and
its redistribution bindings are added by the full config loader after the
reactor's own config parser has run, so a peer rebuilt from the subtree comes
back with a session and no routes.
<!-- source: internal/component/bgp/reactor/operation.go -- runningPeerSettings -->
<!-- source: internal/component/bgp/config/peers.go -- peersAndDynamicGroups -->

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
<!-- source: internal/component/config/transaction/operation.go -- ValidateOperations -->

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

The dual-presence window runs on the real path, and has been observed doing so.
`test/reload/config-apply-ordering-address-swap.ci` gives two interfaces each
other's address in one commit, which is the four-node cycle `tryRelaxCycle`
relaxes, and reads the kernel's own notifications back through `ip monitor`. It
asserts what the window means: both creates land before either destroy, so both
addresses are present at the same time, and neither interface is left without an
address at any notification.
`test/reload/config-apply-ordering-mixed-root.ci` proves the same ordering with a
second root in the transaction, using a static route whose gateway resolves only
against the new address. Both carry `option=needs-linux:caps=net-admin`, so they
run in the QEMU VM and skip on a host without CAP_NET_ADMIN. Both have been
through the discrimination walk `ai/rules/interop-and-goal-validation.md`
requires, in the QEMU guest on Ze's runtime kernel. Under a `tryRelaxCycle` that
returns the unreduced edge set, the swap answers `config verify failed:
operation dependency cycle` and nothing is applied; under the uncovered
participant condition the orchestrator no longer carries, the mixed-root route
installs before its address and the kernel answers `network is unreachable`.
Each test passes when its revert is restored, so the window is demonstrated here
rather than asserted.

The rotation, swap and reip tests reach none of that. They rotate BGP
router-ids, so they emit peer operations only, two peers changing router-id
share no resource, and the graph gives them no edge to make a cycle from. Each
of their headers now says so and points at the address test that carries the
claim its own file name suggests. The create and delete tests emit real iface
interface operations and are Linux-only.

What stays unreached is `Params.AllowDual`. `markDualPresence` writes it and
nothing outside `solver.go` reads it, so the window comes from the removed edges
alone and the flag labels the result rather than instructing the applier. No
test can tell a build that sets it from one that does not, and the address-swap
test asserts the window without reading it.

The operation-count warning at 10000 and the cycle-depth rejection at 100 are
named in the design and not enforced in the graph builder. Config is operator
supplied and bounded, so neither is a security boundary.

The step-3 owner state check is not implemented in `armSettlementWaiters`, which
subscribes and nothing more.

The iface decomposer skips an address that the ACTIVE config already gives to
the SAME interface with the same prefix length, and skips nothing else: an
address moving to another interface, and an address whose prefix length changes,
each produce a create beside the destroy of the old one. That is the pair the
dual-presence window exists for, so "an address create never meets an address
that is already there" was never true and is not what the ordering relies on.
<!-- source: internal/component/iface/operation.go -- decomposeIfaceOperations -->

A root outside `iface` that declares an address in `Produces` would be a second
producer of one resource. Nothing does today, and the derivation gives two
producers of one resource no edge between them: what orders them is the
uniqueness rule above.
