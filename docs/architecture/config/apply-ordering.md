# Config Apply Ordering: the operation graph

This page keeps three things apart. The REQUIREMENT is the owner's, and it is
quoted. The DESIGN is what the requirement asks the core to do. The CURRENT
STATE is what the code does today, and each claim names the function that
produces the behavior.

## The requirement

The owner stated it on 2026-09-11. The words below are his, unedited.

> Yes, BGP need an IP to bind. So adding IP must be done before dealing with peers, however if an IP is deleted to move, the move require BGP for the peer to be stopped, then the move then readded.
> so the order is:
> - stop all BGP deconfigured
> - stop all BGP which IP needs to move
> - remove all IP deconfigured
> - add all IP moved or added
> - setup BGP for new IPs
> if the change for the IP is like an MTU change, then the IP does not need to be removed. If the IP is changed from interface then it does. If it is not easy to establish what action will lead to what, be safe and deconf/reconf

He then wrote:

> The same can be said from all protocol which need to use IP.

Two refinements followed on the same day. The first covers a socket that binds
the wildcard address:

> if you binded to 0.0.0.0 moving the IP is then a non-issue but you have to assume the binding will be specific

The second covers the routing context, and it looks forward to VRF support that
Ze does not have:

> Also changing the binding with VRF - down the line - will change what can speak with the IP and may cause issue too

BGP is the instance the owner used. The requirement binds every component that
binds a local IP address.

## The design

### Binder and provider

A BINDER is a component that binds a local IP address. The list today is bgp,
ike, l2tp, dhcp, ntp, gnmi, tftp, and any listener a plugin opens. The interface
layer is the PROVIDER of the addresses they bind. This page uses one word for
each role and never a second one.

### The five phases

A commit applies in this order:

1. Stop the binders the new config removes.
2. Stop the binders whose bound address is DISTURBED.
3. Remove the addresses that are removed or disturbed.
4. Add the addresses that are new or moved.
5. Start the binders against the new addresses.

Phases 2 and 5 are the two halves of one move. A binder that holds a disturbed
address is stopped before its address leaves, and started after the new address
arrives.

### When an address is disturbed

An address is disturbed when the commit removes it, when it changes interface,
or when its prefix length changes. Each of those needs the address to be removed
and added again.

A change that leaves the address row intact does not disturb it. An MTU edit and
a description edit on the interface are the cases the owner named.

### When a binder is disturbed

Disturbance is a property of the pair of one binder and one address. It is not a
property of the address alone. The rule above answers whether the ADDRESS is
disturbed. How the binder bound answers whether the BINDER is.

| The binder is bound to | Its address is disturbed | Phases 2 and 5 |
|------------------------|--------------------------|----------------|
| A specific address | Yes | Apply. Stop the binder, then start it |
| The wildcard, `0.0.0.0` or `::` | Yes | Do not apply. The binder keeps running |

A wildcard socket was never tied to the address, so an address that arrives,
moves or leaves does not invalidate it.

### The binding context

A binder is disturbed when the BINDING CONTEXT it holds stops meaning what it
meant. The address is one part of that context. The routing context is another.
The three disturbances above are an instance of that rule, and the list is not
closed.

VRF is the case that proves the list is not closed. The same address in two VRFs
is reachable by different peers. A change of VRF therefore changes who can speak
to the binder, with the address, the prefix length and the interface all
unchanged. A VRF change is a disturbance of its own.

VRF cuts across the wildcard rule above, and the two read as a contradiction
until you state it. A wildcard binder is undisturbed by an address that moves
inside its own VRF. It is disturbed by a change of the VRF it binds in, because
a socket bound to `0.0.0.0` in one VRF does not serve another.

Ze has no VRF support today. This is a constraint on the design. It is not a
claim about the code.

### The fail-safe default

Where the core cannot establish whether a binding context still means what it
meant, it treats the binder as disturbed. It stops it. One rule answers two
questions. The first is a change whose effect on an address is not established.
The second is a binder whose bind is not known to be the wildcard.

The owner's words for it are "If it is not easy to establish what action will
lead to what, be safe and deconf/reconf", and "you have to assume the binding
will be specific".

The default is not symmetric, and the cost of each error is the reason:

| The core assumes | The truth | The cost |
|------------------|-----------|----------|
| Specific | The bind is the wildcard | One stop and one start of a session that would have survived |
| Wildcard | The bind is specific | A binder holds an address that is gone |

The second is the failure the requirement exists to prevent. The first costs the
operator one session restart. Make the cheap error. `ai/rules/principles.md`
states the general form: a value that is silently wrong must not be reachable.

### What dual presence becomes

Make-before-break keeps the old address on the host while the new one arrives,
so that nothing bound to it loses its binding. With the binder stopped across
the move, there is no binding to protect, and the window has no work to do.

A move of ONE address can carry no such window at all. One address cannot sit on
two interfaces at the same time, which is what the uniqueness rule states.

Remove, then add, with the binder stopped, is correct under the requirement and
carries less machinery. The design asks for no dual-presence window.

## What is not built

The four gaps below are the distance between the design above and the code. The
rest of this page describes the code.

**Phase 2 is absent.** A binder emits operations only when its OWN config
changed. An address that moves between interfaces changes the `interface` root
and leaves the `bgp` root identical, so nothing stops the peer.
`decomposeBGPOperations` emits a remove and an add for a peer when
`activePeer.localAddress != candidatePeer.localAddress`, which is the address
VALUE changing. A peer whose config bytes are identical returns before that
test. The session therefore stays up while its address leaves one interface and
arrives on another.
<!-- source: internal/component/bgp/plugin/operation.go -- decomposeBGPOperations -->

What happens instead runs outside the transaction, on the kernel's own address
notifications. `handleAddrRemovedPayload` stops the listener bound to a removed
address, and `handleAddrAddedPayload` starts it again when the address arrives.
Neither one stops or starts a peer, and neither is ordered against the commit.
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- handleAddrAddedPayload, handleAddrRemovedPayload -->

**Phases 2 and 5 are out of reach for a binder nobody decomposes.** Both phases
need one binder to take TWO steps in one commit, a stop before the addresses
move and a start after. A participant with no decomposer gets one coarse
section-apply node, and one node cannot be split in two. Two roots register a
decomposer, `interface` and `bgp`, so every other binder has one lump.
<!-- source: internal/component/config/transaction/operation.go -- RegisterOperationDecomposer -->
<!-- source: internal/component/config/transaction/solver.go -- placeSectionNodes, appendSectionNodes -->

**The dual-presence policy contradicts the requirement.** `tryRelaxCycle`
removes the cross-interface edges of an address cycle, which leaves both
addresses present while they swap. `markDualPresence` labels the creations it
freed. That is make-before-break, which the requirement does not ask for.

`test/reload/config-apply-ordering-address-swap.ci` asserts the window. It also
asserts one TCP connection for the whole run. That is the claim that the BGP
session bound to the moving address never restarted. Under the requirement that
session is stopped in phase 2 and started in phase 5, so the test states the
inherited policy rather than the owner's.
<!-- source: internal/component/config/transaction/solver.go -- tryRelaxCycle, markDualPresence -->

**A name check stands in for phase 5.** `sortParticipantsBGPLast` sorts the
participant named `bgp` to the tail of the slice, so the one binder somebody
noticed applies last. Its own comment gives the reason, which is that it matches
the ordering of the reload path it replaced. It is a crude stand-in for phase 5
for one binder, and not a designed invariant. `ai/rules/principles.md` bans a
central list of this shape. The core file spells one component's name, and no
other binder is named at all.
<!-- source: internal/component/plugin/server/reload_tx.go -- sortParticipantsBGPLast, bgpParticipantName -->

**The wildcard carve-out is verified for BGP only.** Ze's BGP binds specific
addresses. `CreateReactorFromTree` sets no global listen address.
`startMultiListeners` opens one listener for each passive peer's local address,
through `startListenerForAddressPort`. The carve-out therefore frees no BGP
listener today.

Whether another binder binds the wildcard is not established here. `listenDHCP`
binds `:67` and ties the socket to a device with `SO_BINDTODEVICE`. That is a
wildcard ADDRESS bind whose context is the device. Reading each remaining
binder's own listen call settles the rest.
<!-- source: internal/component/bgp/config/loader_create.go -- CreateReactorFromTree, the reactor.Config with no ListenAddr -->
<!-- source: internal/component/bgp/reactor/reactor.go -- startMultiListeners, startListenerForAddressPort -->
<!-- source: internal/plugins/dhcpserver/socket_linux.go -- listenDHCP -->

## The current state

Every section below describes the code as it is today. Read it against "What is
not built" above, which names the four places where the code and the requirement
disagree.

Config reload applied changes surface by surface with no cross-surface order.
Dependent operations could run in the wrong order. One example is an interface
added after the BGP peer that binds it. Another is an address removed while a
peer still binds it. The operation graph replaced that ad-hoc order.

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
apply. So a root nobody decomposes is ordered against the operations the other
roots emit, and applied by the `config-apply` callback every plugin implements.

Coverage is what makes the ordering unconditional. Until 2026-09-08 the
orchestrator abandoned the operation path for the WHOLE transaction as soon as
one participant could not be decomposed, and fell back to the unordered section
apply, so the operations that were ordered correctly lost their order with it.
Every reload took that fallback in practice: a dozen plugins declare the `bgp`
root and none of them decomposes. The fallback is deleted, and a coarse node is
what replaced it.
<!-- source: internal/component/config/transaction/orchestrator.go -- operationNodes -->
<!-- source: internal/component/config/transaction/executor.go -- applySection -->

**A coarse node is placed after the last create and modify, so it runs before
the destructions.** That is the sequence the inherited design stated: create the
address, update the services that bind it, destroy the old address last. The
requirement asks for a different sequence, and one node cannot carry it, because
phases 2 and 5 need two. A root nobody decomposes gets the one position the core
can state for it, and the core knows nothing more about that root.

The position is a placement rather than an edge, and `placeSectionNodes` takes
it once the sort is done. An edge from every create and to every destroy would
close a cycle with `iface-remove-address-before-add-same-address`, which orders
a destroy BEFORE a create. Moving one address between two interfaces, while any
uncovered root has a diff, would then abort a reload that works today. A coarse
node carries no edge at all, so moving it constrains nothing and no other
operation changes place.

Where a destroy is forced ahead of a create the two halves cannot both hold,
and the creations win. The section applies the config's end state.
<!-- source: internal/component/config/transaction/solver.go -- placeSectionNodes -->
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
breaks it by removing the cross-interface edges, which leaves both addresses
present for the duration of the swap, and then marks the creations it freed
with `AllowDual` (see the note on that flag below). The window this produces is
make-before-break, which "What is not built" above names as a policy the
requirement does not ask for. "Address operation" is a
verb and a kind: an operation
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
dual-presence window covers today, so "an address create never meets an address
that is already there" was never true and is not what the ordering relies on.
<!-- source: internal/component/iface/operation.go -- decomposeIfaceOperations -->

A root outside `iface` that declares an address in `Produces` would be a second
producer of one resource. Nothing does today, and the derivation gives two
producers of one resource no edge between them: what orders them is the
uniqueness rule above.
