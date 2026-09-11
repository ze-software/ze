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

The three entries below are the distance between the design above and the code.
The rest of this page describes the code.

**Phase 2 reaches a binder that decomposes.** BGP is the only one. What it does
for BGP is in "Phases 2 and 5, for BGP" below. For every other binder it does
nothing, and the next entry says why.

**Phases 2 and 5 are out of reach for a binder nobody decomposes.** Both phases
need one binder to take TWO steps in one commit, a stop before the addresses
move and a start after. A participant with no decomposer gets one coarse
section-apply node, and one node cannot be split in two. Two roots register a
decomposer, `interface` and `bgp`, so every other binder has one lump.
<!-- source: internal/component/config/transaction/operation.go -- RegisterOperationDecomposer -->
<!-- source: internal/component/config/transaction/solver.go -- placeSectionNodes, appendSectionNodes -->

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
not built" above, which names the places where the code and the requirement
disagree.

Config reload applied changes surface by surface with no cross-surface order.
Dependent operations could run in the wrong order. One example is an interface
added after the BGP peer that binds it. Another is an address removed while a
peer still binds it. The operation graph replaced that ad-hoc order.

## Phases 2 and 5, for BGP

A commit that disturbs an address stops the BGP sessions bound to it before the
address leaves the host and starts them after it arrives. The peer's own config
does not have to change, and in the case the requirement is written for it does
not change at all.

Three steps produce that, and each is owned by the layer that can answer it.

**The core computes WHICH addresses are disturbed.** `DisturbedAddresses` reads
the planned operations. It returns every address named by an operation whose
verb is destroy and whose `Produces` names an address. That is the removal, the
move between interfaces and the prefix-length change. Each of the three is
applied by removing the address. An MTU edit emits no address operation, so
it disturbs nothing.
<!-- source: internal/component/config/transaction/operation.go -- DisturbedAddresses -->

**The planner carries the set to every root that decomposes.** It decomposes
the roots that have a diff and computes the set from that plan. Where the set
is not empty it decomposes again, with the set on
`DecomposeRequest.DisturbedAddresses`. The second pass asks every root that
registered a decomposer, not only the roots with a diff: the binder the
requirement is about has no diff of its own. A commit that disturbs no address
makes one pass and asks nobody extra. A plugin that declares its decomposition
receives the set on the same event as its diff.
<!-- source: internal/component/plugin/server/reload_tx.go -- operationPlannerFromTrees, bindingRoots -->

A binder with no diff is not an affected plugin. It would be outside the
transaction, where nothing can ask it and nothing can send it the operations it
answers with. So every running plugin that declares a decomposition joins, with
no section. `filterDiffs` then finds it no diff, so it receives no verify event,
no section apply and no coarse node. A commit that disturbs nothing costs it
nothing.
<!-- source: internal/component/plugin/server/reload.go -- appendDecomposingPlugins -->

**The owning component decides what STOPPING means.** `decomposeBGPOperations`
emits a remove-peer and an add-peer for each peer whose local address is in the
set. That is beside the pair it already emitted for a peer whose address value
changed. The core never reads the `bgp` root, and `bgp` never reads the
`interface` root.
<!-- source: internal/component/bgp/plugin/operation.go -- decomposeBGPOperations, peerBindingDisturbed -->

**The ordering is derived, not declared.** The pair consumes the address. The
remove-peer therefore runs before the destroy that produces that address, and
the add-peer after the create that produces it. Those are the two derived edges
a peer with a changed address has always earned, so phase 2 added no rule.

**A peer whose source address the kernel picks is stopped too.** A peer with
`connection.local.ip` absent or `auto` binds an address Ze did not choose. Ze
cannot say whether this commit takes that address away. The fail-safe default
answers it: the session is stopped and started as soon as any address is
disturbed. Its two operations declare every disturbed address, so the ordering
still holds. Ze binds specific addresses everywhere else, so the wildcard
carve-out frees no BGP session.
<!-- source: internal/component/bgp/plugin/operation.go -- peerAddressConsumes -->

What phase 2 does NOT cover is in "What is not built" above: a binder nobody
decomposes.

**Every interface commit answers the address question.** The disturbed set is
computed from the address operations the plan carries, so a root that produces
none reads as quiet. `decomposeIfaceOperations` used to produce none whenever
ANY key in its diff was one it had no primitive for, which made a commit that
edited an MTU AND moved an address look like a commit that disturbed nothing:
the binder stayed up while the coarse section apply moved the address under it.
An MTU edit on its own took the same path and was correct there, and the two
cannot be told apart from outside the decomposer.

It now decomposes the whole root on every call. An address and an interface of
a type it can create become one operation each, and every other key rides one
`configure-interfaces` operation, which applies the interface config as a
whole. Four constraint rules place that operation after every operation that
moves an address or an interface, so the end state it applies is the state the
plan has already reached and only the keys no operation carries are left for it.
No key is classified, so nothing in the root can be dropped by misreading one.

Two cases stay with that operation rather than becoming operations of their
own. An interface whose type this package has no create primitive for (a
tunnel, a wireguard device, an xfrm device) is created and deleted by it, and
an address arriving on an interface it creates waits for it, because an
add-address operation would name a device that does not exist yet. Every
address LEAVING the host still becomes an operation, whatever carries the
interface, because that operation is what the core reads the disturbance out
of.
<!-- source: internal/component/iface/operation.go -- decomposeIfaceOperations, ifaceConfigureOperation -->

The kernel-driven path is still there and is not a substitute.
`handleAddrRemovedPayload` stops the LISTENER bound to a removed address and
`handleAddrAddedPayload` starts it again. Neither stops a peer, and neither is
ordered against the commit.
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- handleAddrAddedPayload, handleAddrRemovedPayload -->

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

One rule survives, in `iface`, and it states a fact about two operations over
DIFFERENT resources, which no pair of declarations can carry: every address the
commit removes leaves the host before any address the commit adds arrives
(`iface-remove-address-before-add-address`). That is phases 3 and 4, in the
order the requirement gives them. A root that adds a rule today is almost
certainly describing a produce and consume fact instead.

Two rules became that one. The first ordered a removal before the addition of
the SAME address, which is one address living on one interface. The second held
the new address on an interface until the old one left, so no interface was ever
bare: that was make-before-break. Deleting only the second would have left a
renumber unordered, because the old address and the new one are two different
addresses and the planner emits its additions first.
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

**A coarse node is placed between phase 4 and phase 5.** It runs after the
addresses and the interfaces the commit adds, and before the first operation
that creates or modifies something which binds them. The core knows nothing
about a root nobody decomposes, so the fail-safe default reads that participant
as a binder, and a binder belongs in phase 5. Inside phase 5 it runs before the
decomposed starts, because a binder has to find the subsystems it uses already
configured: the `bgp` root's peers start after the plugins that configure the
RIB, the graceful-restart state and the filters have applied their sections.

That order is derived from the verb and the resource kind every operation
already declares. `isAddressingKind` is the only place the engine reads a kind
for anything but identity: a root that PROVIDES addressing declares `address` or
`interface`, and a root that BINDS it declares its own kind and lands on the
phase 5 side with no edit to the engine.

One node still cannot carry phases 2 and 5 both, so a coarse binder is started
against the new addresses and never stopped before the old ones go. That is the
limit "What is not built" above states.

The position is a placement rather than an edge, and `placeSectionNodes` takes
it once the sort is done. A coarse node stands for a whole participant section,
so it names no resource and nothing in the graph can state where it goes; an
edge invented for it would join cycles the operator never wrote.

The two bounds cross only where a binder start sorts ahead of an address the
commit adds, which no edge can produce for a binder that declares the address it
binds. The addressing bound wins there, because a binder started before its
address is the failure the requirement exists to prevent.
<!-- source: internal/component/config/transaction/solver.go -- placeSectionNodes, sectionNodePosition -->
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

**Every cycle is rejected.** A swap of two addresses between interfaces used to
be a cycle by construction, and the solver relaxed it by removing the
cross-interface edges, which left both addresses present for the duration of the
swap. That window was make-before-break. With the rule that closed the cycle
deleted, a swap carries one destroy and one create for each address with a single
edge between them, so there is no cycle to break and nothing relaxes one.
`TopologicalSort` answers `ErrOperationCycle` for any graph it cannot order.

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

The applied ORDER is read on the real path from the kernel's own notifications.
`test/reload/config-apply-ordering-address-swap.ci` gives two interfaces each
other's address in one commit and reads the notifications back through `ip
monitor`. It asserts what the requirement asks for: each address leaves the
interface that held it before it arrives on the one that takes it, and no
interface ever holds both.
`test/reload/config-apply-ordering-mixed-root.ci` proves the same ordering with a
second root in the transaction, using a static route whose gateway resolves only
against the new address. Both carry `option=needs-linux:caps=net-admin`, so they
run in the QEMU VM and skip on a host without CAP_NET_ADMIN.

Neither has been walked under the break-before-make policy. Both passed a
discrimination walk in the QEMU guest under the dual-presence policy, and both
assertion functions changed with the policy on 2026-09-11, on a darwin host
where the files skip. What HAS run is the two-arm unit test over each assertion
function (`internal/test/fixture/register_config_apply_ordering_test.go`), which
accepts the requirement's order and refuses the policy it replaced. The walk the
two `.ci` files owe is named in their own DISCRIMINATION headers and is
outstanding.

The rotation, swap and reip tests reach none of that. They rotate BGP
router-ids, so they emit peer operations only, two peers changing router-id
share no resource, and the graph gives them no edge to make a cycle from. Each
of their headers now says so and points at the address test that carries the
claim its own file name suggests. The create and delete tests emit real iface
interface operations and are Linux-only.

The operation-count warning at 10000 and the cycle-depth rejection at 100 are
named in the design and not enforced in the graph builder. Config is operator
supplied and bounded, so neither is a security boundary.

The step-3 owner state check is not implemented in `armSettlementWaiters`, which
subscribes and nothing more.

The `configure-interfaces` operation is applied by the same function the
section apply calls, so what a reload applies does not depend on which of the
two paths it travels. The operation path used to apply neither, since a
participant that owns operations receives no section apply: an interface reload
through it never republished the name mapping, never reconciled the DHCP
clients and never reconciled the router advertisements.
<!-- source: internal/component/iface/register.go -- applyPendingConfig, OnConfigApply, OnConfigOperationApply -->

The iface decomposer skips an address that the ACTIVE config already gives to
the SAME interface with the same prefix length, and skips nothing else: an
address moving to another interface, and an address whose prefix length changes,
each produce a create beside the destroy of the old one. The surviving rule puts
the destroy first, so an address create never meets the address it replaces.
<!-- source: internal/component/iface/operation.go -- decomposeIfaceOperations -->

A root outside `iface` that declares an address in `Produces` would be a second
producer of one resource. Nothing does today, and the derivation gives two
producers of one resource no edge between them: what orders them is the
uniqueness rule above.
