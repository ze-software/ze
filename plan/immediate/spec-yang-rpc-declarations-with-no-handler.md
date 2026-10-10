# Spec: yang-rpc-declarations-with-no-handler

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 1/3 |
| Handoff | - |
| Updated | 2026-10-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator adds and removes BGP peers on a running router through the
configured route (`set bgp peer ...` then `commit`) or the runtime lifecycle
commands. This spec preserves both routes and requires proof of their RIB
effects. The configured lifecycle fixture
`test/plugin/rest-peer-set-delete-lifecycle.ci` records the original
peer-table proof; the broader lifecycle and persistence criteria below remain
the completion contract.
The original schema inventory published two copies of `peer-add` and
`peer-save`, among other RPC declarations. Six duplicate or obsolete
declarations were removed on September 13, as recorded below. The
`ze-bgp-cmd-peer-api:peer-save` declaration and its module were simply not
deleted that day; no owner ruled them retained. `peer-save` is served and must
be declared exactly once, under its served name (`ze-bgp:peer-save`).
`ze-bgp-cmd-peer-api.yang` was removed by
`spec-rpc-published-name-does-not-reach-its-handler`, closed 2026-10-08
(9c346cf094), after its two unique facts moved. This spec waits on nothing.

**The model this spec builds.** Three RPCs act on one runtime working set, and
the configuration file is reconciled to that set on demand.

| RPC | What it does | What it does NOT do |
|---|---|---|
| `peer-create` | Builds the peer in the reactor and starts it | Write the configuration file |
| `peer-delete` | Stops the peer and removes it from the reactor | Write the configuration file |
| `peer-save` | Makes the configuration file state the running peer set | Change the running set |

`peer-save` persists presence AND absence. A peer created at runtime is written
into the file. A configured peer deleted at runtime is taken out of it. This
sits BESIDE the configured route, and neither replaces the other.

**Current runtime entry points.** `handleBgpPeerAdd` (`create.go`) now reaches
`Reactor.AddDynamicPeer`, and `create bgp peer` is declared in `ze-peer-cmd.yang`
as `ze-bgp:peer-add`. `handleBgpPeerRemove` calls `Reactor.RemovePeer`.
`handleBgpPeerSave` (`save.go`) is registered as `ze-bgp:peer-save`, reached by
`update bgp config`; it rejects arguments, compares the running and stored peer
sets and saves through an editor whose commit writer calls
`registry.RuntimeConfigCommit`.

Those entry points are implemented. They do not by themselves demonstrate
AC-1 through AC-13, nor implement this spec's required `peer-create` and
`peer-delete` wire naming. The `ze-bgp-cmd-peer-api:peer-save` declaration
published a different module prefix from the served save handler; it was
replaced by one declaration under the served name `ze-bgp:peer-save`
(`ze-bgp-api.yang`, revision 2026-10-08), and the module is gone.

**Runtime origin and reload survival remain requirements.** The original design
required a distinction between configured, listen-range and command-created
peers so an unrelated commit cannot remove a runtime-created peer. The current
save implementation reads the reactor's maintained configuration tree rather
than a new origin marker. That implementation choice must be assessed against
AC-3, AC-7, AC-9 and AC-12; it is not permission to drop runtime survival,
presence/absence persistence or the distinction from listen-range peers.

**Both front doors, and what each can reach.** An operator types the command.
A plugin calls `Plugin.DispatchCommand` (`pkg/plugin/sdk/sdk_engine.go`), which
routes the same string through the engine's command dispatcher.
`plugin01APIPeerRemove` (`internal/test/fixture/plugin_fixture_01_api.go`)
already drives `delete bgp peer 127.0.0.1` that way, under
`test/plugin/api-peer-remove.ci`. A third caller reaches the same commands:
`convertNeighborCreate` (`internal/exabgp/bridge/bridge_neighbor.go`) translates
ExaBGP's `create neighbor` to `create bgp peer` and `delete neighbor` to `delete
bgp peer`, and the `api-peer-lifecycle` profile
(`internal/le/interoplab/bgp/exabgp_helpers.go`) sends `create neighbor` then
`announce route 1.1.0.0/24`, whose UPDATE bytes
`test/exabgp-compat/api/api-peer-lifecycle.ci` asserts. A plugin cannot start a
configuration transaction: the SDK's config surface is
`OnConfigOperationDecompose` and `OnConfigOperationApply`
(`pkg/plugin/sdk/sdk_callbacks.go`), which RECEIVE an operation the daemon
decided, and `ze-config-cli-cmd.yang` declares read commands only. REST is the
one initiating surface, through `ConfigSessionManager.Commit`
(`internal/component/api/config_session.go`). So `peer-save` is what gives a
plugin any way to persist a peer at all.

**The persistence seam now exists.** `handleBgpPeerSave` installs
`registry.RuntimeConfigCommit` as the editor's commit writer before calling
`Save`. The earlier claim that prefix update was the only configuration-writing
handler no longer holds. AC-10 through AC-12 still require a functional proof
that the persisted and running configurations agree, including a subsequent
unrelated commit and restart.

**`peer-save` takes no selector.** It acts on the whole
running set. A selector cannot express the half the owner asked for: after
`peer-delete 10.0.0.1` the peer is gone from the running set, so a selector
naming it selects nothing, and the ABSENCE is the fact being persisted. A
selector form would answer "0 peers saved" whether the peer was deleted, never
existed, or the word was mistyped, which is the silently wrong value
`ai/rules/principles.md` bans. An operator who wants one specific peer in the
file already has `set bgp peer ...` and `commit`. That reading constrains where
the command node goes: the `update bgp peer` subtree in `ze-peer-cmd.yang`
declares a mandatory `selector` leaf that every node under it inherits, so a
selector-free save cannot live there. The implemented command is
`update bgp config`, which must continue to pass the grammar feeders
(`./le cli grammar`, `ai/rules/cli.md`).

**One verb, and the sites that carry the other two spellings.** The tree
declares `peer-remove`, the owner says `peer-delete`, and the served handler is
`ze-delete:bgp-peer`. `ai/rules/cli.md` states "Ze is unreleased, so a second
spelling MUST be renamed outright rather than aliased".
`docs/contributing/ze-go-style.md` states "Ze keeps `delete` for config, `clear`
for counters, and `remove` for a route", so `remove` names a route operation and
`peer-remove` takes a word that is spoken for. The operator types `delete bgp
peer`, and `create` and `delete` are the runtime-resource lifecycle pair the
verb table declares (`Verbs`, `internal/component/command/verbs.go`). The three
published methods are therefore `ze-bgp:peer-create`, `ze-bgp:peer-delete` and
`ze-bgp:peer-save`, one prefix for one family.

| Site | What changes |
|---|---|
| `internal/component/bgp/yang/ze-bgp-api.yang` | `rpc peer-add` becomes `peer-create`, `rpc peer-remove` becomes `peer-delete` |
| `internal/component/bgp/plugins/cmd/peer/yang/ze-bgp-cmd-peer-api.yang` | Removed by `spec-rpc-published-name-does-not-reach-its-handler` (closed 2026-10-08) |
| `internal/component/cmd/delete/yang/ze-cli-delete-api.yang` | That spec pointed `delete bgp peer` at its `rpc bgp-peer` after deleting `ze-bgp-api` `peer-remove`; the rpc moves to `ze-bgp-api` as `peer-delete` and the module is removed |
| `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` | `ze:command "ze-bgp:peer-add"` and `ze:command "ze-delete:bgp-peer"` |
| `internal/component/bgp/plugins/cmd/peer/peer.go` | the two `RPCRegistration` wire methods |
| `internal/component/bgp/plugins/cmd/peer/yang/cmd_schema_test.go` | asserts the `ze:command` string |
| `internal/component/cmd/delete/yang/self_containment_test.go` | maps the wire method to its owning package |
| `internal/component/command/help_test.go` | a fixture row carries the wire method |
| `internal/component/config/yang/command_test.go` | asserts `GetCommandExtension` answers the wire method |
| `internal/core/ipc/yang_test.go` | `TestYANGBGPAPIRPCs` and `TestExtractRPCs` name lists |
| `docs/architecture/exabgp-bridge.md` | states which command `create neighbor` reaches |

**The other declarations, judged under the same model.**
`ze-bgp-cmd-peer-api.yang` publishes under the `ze-bgp-cmd-peer` wire prefix,
while save is served under `ze-bgp`. The disposition is done: the module was
removed by `spec-rpc-published-name-does-not-reach-its-handler` (9c346cf094),
after `peer-save` was declared exactly once under its served name
`ze-bgp:peer-save` and the `session` input leaf of `session-peer-ready` moved.
No owner ever ruled the module retained (Design Insights, 2026-10-08).
The two `ze-cli-set-api.yang` names were deletion candidates because their
own sibling records the removal: `ze-cli-set-cmd.yang` carries `revision
2026-06-03` reading "Removed set bgp peer with/save.", above a comment reading
"Peer config goes through the editor, and the parallel runtime-then-persist path
is dropped." That decision is what the owner has now reversed, and the new path
is the three RPCs above rather than a revival of `set bgp peer with`.
`peer-update-hex` holds no wire method of its own: `handleUpdate`
(`internal/component/bgp/plugins/cmd/update/update_text.go`) is registered once
as `ze-bgp:peer-update` and switches on the encoding word to reach text, hex,
b64 and cursor, so the declaration is deleted. `command-help` and
`command-complete` in `ze-rib-api.yang`
(`internal/component/bgp/plugins/rib/yang/ze-rib-api.yang`) publish
`ze-rib:command-help` and `ze-rib:command-complete`, which no handler serves,
while `internal/plugins/meta/cmd/help.go` registers `ze-bgp:command-help` and
`ze-bgp:command-complete`. The RIB pair is deleted. `peer-show`,
`peer-show-capabilities` and `peer-show-statistics` in `ze-bgp-api.yang` are
near misses of a rename: `ze-bgp:peer-detail`, `ze-bgp:peer-capabilities` and
`ze-bgp:peer-statistics` are registered and answer what the three describe, so
the three declarations are repointed at the served names.

**Test evidence recorded before the September 13 removal.** `test/plugin/api-peer-remove.ci` proves
a plugin can delete a peer and that it leaves `show bgp peer list`.
`test/plugin/rest-peer-set-delete-lifecycle.ci` proves the configured route over
REST. `test/editor/workflow/workflow-peer-lifecycle.et` proves `set`, `delete`
and `commit` over the configuration tree. None of the three reads a RIB.
`create bgp peer` has unit tests over the handler with a mock reactor
(`create_test.go`) and no functional test of its own. One case comes closest:
`test/exabgp-compat/native/api-peer-lifecycle.conf` drives ze through the
ExaBGP bridge, and its `.ci` asserts the UPDATE bytes a route announced through
the created peer puts on the wire. Whether that case is red at HEAD is
UNVERIFIED here, and `./le test functional exabgp-test` settles it. The owner's bar
is that both routes work with the RIBs, so create means routes reach the
Adj-RIB-In and the RIB, and delete means they leave and the withdrawals reach
consumers. That investigation recorded no test meeting the whole bar; the
current implementation still owes a fresh proof against AC-1 through AC-13.

**The command-contract gate still misses the declaration population.**
Its advertised purpose is "every YANG command node has a handler, and every
handler a node" (`internal/le/docvalid/actions.go`). `Validate`
(`internal/le/docvalid/contract.go`) keeps only modules whose name ends in
`-cmd`, so an `-api` module is never opened, and it collects a node only where
`GetCommandExtension` answers a `ze:command` value, which a YANG `rpc` never
carries. The same function computes `orphanLocalHandlers` and hands it to the
report, while `contractSatisfied` reads `orphanYANG` and `orphanHandlers` alone,
so the rows the run prints under "Local handlers with no YANG command" change no
verdict. Both holes remain owned here. Before the September 13 deletion,
`TestEveryCommandNodeHasASummary` recorded six refusals, including the
`peer-save` declaration that was not deleted that day. That historical result must
not be used as the current gate result. Two tests in the original investigation
asserted dead declarations existed, without checking whether a handler answered:
`TestYANGBGPAPIRPCs` asserts each
name is present, and `TestExtractRPCs` matches the whole set with
`assert.ElementsMatch` (`internal/core/ipc/yang_test.go`). The rows for this
class are already written in `plan/journal/unwired-feature.md`, dated 2026-09-03
and 2026-09-05, so this spec adds none.

**Six of the seven were deleted on 2026-09-13, on the owner's ruling.** The
declarations went, and no handler was written for any of them:
`ze-bgp-cmd-peer-api:peer-add`, `ze-bgp-cmd-update-api:peer-update-hex`,
`ze-cli-set-api:bgp-peer-with`, `ze-cli-set-api:bgp-peer-save`,
`ze-rib-api:command-help` and `ze-rib-api:command-complete`. Each one was a
duplicate of a live declaration or the residue of a removal, so deleting it
removed a published method and no capability. `ze-cli-set-api.yang` now declares
no rpc at all, and whether the module file goes is still open.

**Of the seven declarations selected for removal, `peer-save` was not deleted
on September 13.** That was the boundary of that day's cleanup, not an owner
ruling and not a reduction of the runtime-peer feature to one node. The handler and help text now exist, so the
earlier claim that help-shape must remain red until they are written is stale.
No current help-shape result is claimed here.

This spec still owns the create/delete lifecycle and RIB effects in AC-1
through AC-9, whole-set persistence in AC-10 through AC-13, and the published
method/handler gates in AC-14 through AC-16. AC-17 records the six removals and
the single served declaration of `peer-save` separately. None of those broader requirements was discharged by
deleting duplicate declarations.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the peer command table and the create/delete/save model
  → Decision: create and delete change the running set only; `update bgp config` alone writes the file
  → Constraint: the published methods are `ze-bgp:peer-create`, `ze-bgp:peer-delete`, `ze-bgp:peer-save`, one prefix for one family
- [ ] `docs/architecture/exabgp-bridge.md` - `create neighbor` and `delete neighbor` translate to `create bgp peer` and `delete bgp peer`
  → Constraint: the bridge sends the command TEXT, so the wire-method rename does not change what the bridge reaches
- [ ] `docs/contributing/documentation-testing.md` - the command-contract gate
  → Decision: a published rpc no handler serves, and a local handler with no node, each fail the verdict (949091e15a)
- [ ] `docs/functional-tests.md` section 4 - a second daemon started on the first one's file
  → Constraint: the runner never reaps the first daemon, so `le test fixture daemon/await-exit` waits on its exit

### RFC Summaries (Scope: protocol)
- [ ] None. The spec changes command names, the reload comparison and the delete event order; it adds no RFC behavior. The WITHDRAW a delete causes is existing RFC 4271 UPDATE encoding, asserted as bytes in `test/plugin/api-peer-create-delete-rib.ci`.

**Key insights:** (minimal context to resume after compaction)
- A config apply discards every runtime `subscribe` (`Server.DiscardRuntimeSubscriptions`), so a plugin that must see events after a commit subscribes again after it.
- `go test` without `ze_le` plus the `feature-gates.txt` tags links no BGP handler, so live-tree tests run with those tags.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/cmd/peer/create.go` - `handleBgpPeerCreate` parses the keywords and calls `AddDynamicPeer`
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `handleBgpPeerDelete` calls `RemovePeer`; `RPCRegistration` publishes the wire methods
- [ ] `internal/component/bgp/plugins/cmd/peer/save.go` - `handleBgpPeerSave` refuses arguments and saves the running peer set through `registry.RuntimeConfigCommit`
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `recordPeerConfig`, `dropPeerConfig`, `peerConfigNameFor`, `ReloadRunning`, `withPeerEntries`
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `AddDynamicPeer`, `doRemovePeer`
- [ ] `internal/component/plugin/server/reload.go` - `reloadConfig` diffs the candidate against `ReloadRunning`
- [ ] `internal/component/plugin/server/delivery_graph.go` - `DiscardRuntimeSubscriptions`

**Behavior to preserve:** (unless the user explicitly said to change it)
- `create bgp peer <addr> asn <asn> [...]`, `delete bgp peer <selector>` and `update bgp config` keep their command text, so the ExaBGP bridge and every `.ci` that types them are unchanged.
- The configured route (`set bgp peer ...` then `commit`) keeps working: `test/plugin/rest-peer-set-delete-lifecycle.ci`.

**Behavior to change:** (only what the user asked for)
- Wire methods `ze-bgp:peer-add` and `ze-bgp:delete-peer` (documented by `ze-cli-delete-api:bgp-peer`) become `ze-bgp:peer-create` and `ze-bgp:peer-delete`, declared in `ze-bgp-api.yang`; `ze-cli-delete-api.yang` is removed.
- A commit or SIGHUP reload whose candidate does not declare a created peer no longer plans its removal (it was refused with "remove-peer ... is not running").
- A removed peer's down event reaches the processes it fed, so the RIB withdraws its routes towards other peers.
- The `update bgp config` refusal of a selector says it acts on the whole running peer set.
- The command-contract gate fails on an unserved published rpc and on a local handler with no node.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A YANG API module is loaded at start, and its `rpc` statements are read as schema text.
- An operator or an AI agent reads a method name from `ze schema methods` or from `ze help ai --json`.
- An operator types `create bgp peer`, `delete bgp peer` or `update bgp config`; a plugin sends the same text through `Plugin.DispatchCommand`.

### Transformation Path
1. Module load and resolve in `internal/component/config/yang/loader.go`.
2. RPC extraction in `ExtractRPCs` (`internal/component/config/yang/rpc.go`).
3. Wire method construction in `RegisterRPCs` (`internal/component/plugin/server/schema.go`).
4. Publication in `cmdMethods` (`internal/component/config/schema/cli/main.go`) and in `Build` (`internal/component/aihelp/aihelp.go`).
5. Dispatch through the handler registry: `handleBgpPeerCreate`, `handleBgpPeerDelete`, `handleBgpPeerSave`.
6. Reactor: `AddDynamicPeer` records the peer's tree (`recordPeerConfig`); `RemovePeer` drops it (`dropPeerConfig`) and publishes the down event; a reload compares the candidate against `ReloadRunning`, and `SetConfigTree` carries undeclared created peers forward (`withPeerEntries`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema ↔ handler registry | the `ze:command` node's method and the `RPCRegistration` name must agree; the command-contract gate compares them | Yes: `./le doc yang-contract command-contract`, 418 commands, all validated (2026-10-10) |
| Plugin ↔ engine | `Plugin.DispatchCommand` sends command text | Yes: fixtures 13 and 01 drive create, delete and save that way |
| Reload ↔ reactor | `reloadConfig` asks `ReloadRunning(candidate)` | Yes: `TestUnrelatedReloadKeepsACreatedPeer`; the AC-9 lines of `api-peer-create-delete-rib.ci` |

### Integration Points
- `Validate` and `contractSatisfied` (`internal/le/doc/yangcontract/contract.go`) - the gate extended rather than duplicated.
- `ReactorConfigurator.ReloadRunning` (`internal/component/plugin/types.go`) - the one new reactor method; `Coordinator.ReloadRunning` forwards it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Plugin and operator reach the same handler through the dispatcher; the reload still diffs and decomposes |
| No unintended coupling (components stay isolated) | Yes | The reload reaches the reactor through the `ReactorConfigurator` interface only |
| No duplicated functionality (extends existing, does not recreate) | Yes | The created-peer set lives in the reactor's existing config tree; no second store |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding path changed |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The methods register through `RPCRegistration`; the YANG modules through generated glue |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Plugin dispatch of the command text reaches the same handler the operator reaches | `Plugin.DispatchCommand` routes through the engine dispatcher | AC-4/AC-8 would prove a different path than AC-1/AC-5 | fixture 13 runs both through the dispatcher and asserts the reactor state | confirmed |
| A-2 | The ExaBGP bridge sends command text, so the rename does not reach it | `convertNeighborCreate` (`internal/exabgp/bridge/bridge_neighbor.go`) | `create neighbor` would stop working | `api-peer-lifecycle` (ExaBGP api suite) PASS on 2026-10-10 against a ze built from this checkout | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A candidate declares a created peer's address under another name | decompose plans a remove and an add for one address | Unverified edge; see Known Limitations |
| R-2 | A plugin's runtime subscription is discarded by the commit AC-9 runs | the WITHDRAW event never reaches the plugin | fixture 13 subscribes inside `lifecycleDeleteWithdraws`, after the commit |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A commit or reload tears down a runtime peer, or a delete leaves routes behind on other peers |
| How is it reverted? | Revert the spec's commits; no config migration, the file format is unchanged |
| Who else touches this path? | `internal/component/plugin/server/reload.go` is shared by every config apply; the ExaBGP bridge types the same commands |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `create bgp peer` through `Plugin.DispatchCommand` | → | `handleBgpPeerCreate` → `Reactor.AddDynamicPeer` | `test/plugin/api-peer-create-delete-rib.ci` (`lifecycleCreate`) |
| `delete bgp peer` through `Plugin.DispatchCommand` | → | `handleBgpPeerDelete` → `Reactor.RemovePeer` → down event → RIB withdraw | `test/plugin/api-peer-create-delete-rib.ci` (`lifecycleDeleteWithdraws`) |
| REST commit and SIGHUP reload | → | `reloadConfig` → `reactorAPIAdapter.ReloadRunning` | `test/plugin/api-peer-create-delete-rib.ci` (`lifecycleSurvivesUnrelatedCommit`) |
| `update bgp config` through `Plugin.DispatchCommand` | → | `handleBgpPeerSave` | `test/plugin/api-peer-save.ci` |
| `./le doc yang-contract command-contract` | → | `contractSatisfied`, `publishedRPCs`, `unservedRPCs` | `TestEveryPublishedMethodHasAHandler` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator types `create bgp peer 10.0.0.1 asn 65001` on a running daemon | `show bgp peer list` names 10.0.0.1, and the peer dials the address |
| AC-2 | The peer of AC-1 reaches Established and sends an UPDATE carrying 192.0.2.0/24 | `show bgp peer 10.0.0.1 rib` and `show bgp rib` both answer the prefix, and the best path for it names that peer |
| AC-3 | AC-1 has run | The configuration file on disk is byte-identical to what it was before, and `show config` names no peer 10.0.0.1 |
| AC-4 | A plugin calls `ze-bgp:peer-create` for 10.0.0.1 through the command dispatcher | The answer carries the peer address, the remote AS and the outcome, and AC-1 through AC-3 hold identically |
| AC-5 | An operator types `delete bgp peer 10.0.0.1` while that peer holds 192.0.2.0/24 | `show bgp peer list` names no 10.0.0.1, and 192.0.2.0/24 is absent from `show bgp rib` |
| AC-6 | A second peer had received 192.0.2.0/24 from Ze before AC-5 | That peer receives a WITHDRAW for 192.0.2.0/24, and a plugin subscribed to route events is told the route is gone |
| AC-7 | AC-5 has run against a peer the configuration file declares | The configuration file on disk is unchanged, and a daemon started on that file brings the peer up again |
| AC-8 | A plugin calls `ze-bgp:peer-delete` for 10.0.0.1 | AC-5 through AC-7 hold identically |
| AC-9 | An unrelated leaf is set and committed while a peer created by AC-1 is running | The created peer stays up, keeps its session, and keeps its Adj-RIB-In |
| AC-10 | `ze-bgp:peer-save` runs after AC-1 | The configuration file names peer 10.0.0.1 with the AS and every other value the create command stated, and a daemon started on that file brings the peer up |
| AC-11 | `ze-bgp:peer-save` runs after AC-5 removed a peer the file declared | The configuration file names that peer no longer, and a daemon started on that file does not bring it up |
| AC-12 | `ze-bgp:peer-save` runs at all | The running configuration and the file agree afterwards, so a later commit of an unrelated leaf removes no peer |
| AC-13 | An operator types a word after `peer-save` | The command is refused, and the refusal says the command takes no selector and acts on the whole running set |
| AC-14 | `ze schema methods` and `ze help ai --json` are read on a built daemon | Every method they publish has a registered handler |
| AC-15 | A YANG `-api` module declares an rpc whose published wire method no handler serves | `./le doc yang-contract command-contract` fails and names the module, the rpc and the wire method |
| AC-16 | A registered handler has no YANG command node and no rpc declaration | `./le doc yang-contract command-contract` fails, rather than printing the row under a passing verdict |
| AC-17 | The six removed declarations (`ze-bgp-cmd-peer-api:peer-add`, `ze-bgp-cmd-update-api:peer-update-hex`, `ze-cli-set-api:bgp-peer-with`, `ze-cli-set-api:bgp-peer-save`, `ze-rib-api:command-help`, `ze-rib-api:command-complete`) remain absent, and `peer-save` is served | The removed methods are no longer published; `peer-save` is declared exactly once, under its served name `ze-bgp:peer-save`, consistent with AC-14/AC-15; no `ze-bgp-cmd-peer:peer-save` method is published; and `TestEveryCommandNodeHasASummary` reports no refusal for an RPC declaration |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Creates a peer at runtime and receives its routes | `create bgp peer` → `handleBgpPeerCreate` → `AddDynamicPeer` → session → Adj-RIB-In → RIB best path → forward to a configured peer | `test/plugin/api-peer-create-delete-rib.ci` |
| 2 | Deletes a peer and sees its routes withdrawn | `delete bgp peer` → `handleBgpPeerDelete` → `RemovePeer` → down event → RIB → WITHDRAW to the configured peer, route event to a plugin | `test/plugin/api-peer-create-delete-rib.ci` |
| 3 | Commits an unrelated leaf, or reloads, while a created peer runs | REST commit or SIGHUP → `reloadConfig` → `ReloadRunning` → no removal planned | `test/plugin/api-peer-create-delete-rib.ci` |
| 4 | Saves the running peer set and restarts on the file | `update bgp config` → `handleBgpPeerSave` → `RuntimeConfigCommit` → file → `ze start` on that file | `test/plugin/api-peer-save.ci` |
| 5 | Creates a neighbor through the ExaBGP API | `create neighbor` → bridge → `create bgp peer` → UPDATE on the wire | `test/exabgp-compat/api/api-peer-lifecycle.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnrelatedReloadKeepsACreatedPeer` | `internal/component/bgp/reactor/reactor_peers_dynamic_test.go` | a candidate without the created peer plans no removal and carries it forward (AC-9) | red with the reconcile skip disabled, green after (4ddd3c3914) |
| `TestCandidateNamingACreatedPeerTakesItOver` | `internal/component/bgp/reactor/reactor_peers_dynamic_test.go` | a candidate that declares the created peer owns it | red with the reconcile skip disabled, green after (4ddd3c3914) |
| `TestRemovedPeerStaysInTheIndexUntilItsDownEvent` | `internal/component/bgp/reactor/delivery_graph_test.go` | the down event of a removed peer reaches the processes it fed (AC-6) | red with the old publish order restored, green after (dcfdebb6ba) |
| `TestPeerSaveRefusesASelector` | `internal/component/bgp/plugins/cmd/peer/save_test.go` | the refusal names the whole running peer set (AC-13) | red before, green after (650f778d2d) |
| `TestAnOrphanLocalHandlerFailsTheVerdict` | `internal/le/doc/yangcontract/contract_test.go` | AC-16 | green (949091e15a) |
| `TestAnUnservedRPCDeclarationFailsTheVerdict` | `internal/le/doc/yangcontract/contract_test.go` | AC-15 | green (949091e15a) |
| `TestADeliberatelyOrphanedDeclarationIsNamed` | `internal/le/doc/yangcontract/contract_test.go` | AC-15 names module, rpc and wire method | green (949091e15a) |
| `TestEveryPublishedMethodHasAHandler` | `internal/le/doc/yangcontract/contract_test.go` | AC-14 over the live tree | green |
| `TestEveryCommandNodeHasASummary` | `internal/le/doc/yangcontract/helpshape_test.go` | AC-17: no refusal for an rpc declaration | green |
| `TestYANGBGPAPIRPCs`, `TestExtractRPCs` | `internal/core/ipc/yang_test.go` | the renamed rpc names | green (3b9366fa72) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input added; `asn` parsing is unchanged | N-A | N-A | N-A |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-peer-create-delete-rib` | `test/plugin/api-peer-create-delete-rib.ci` | AC-1 to AC-9: create, RIB, file untouched, unrelated commit and reload, delete with WITHDRAW, configured-peer delete, fresh daemon on the unchanged file | PASS (7 OK lines and the second daemon) |
| `api-peer-save` | `test/plugin/api-peer-save.ci` | AC-10 to AC-13: save writes presence and absence, a fresh daemon starts on the saved file, a selector is refused | PASS |
| `api-peer-remove` | `test/plugin/api-peer-remove.ci` | delete through the plugin dispatcher | PASS in the plugin suite (876/881; the 5 reds are journalled and none is this spec's) |
| `rest-peer-set-delete-lifecycle` | `test/plugin/rest-peer-set-delete-lifecycle.ci` | the configured route is unchanged | PASS in the plugin suite |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | See the interop decision below | N-A |

**Interop decision: not applicable.** The spec renames two published wire methods
of the command API, changes which peers a reload plans to remove, and orders a
removed peer's down event before its index entry goes. No BGP message format,
capability, attribute or FSM transition changed, so there is no behavior another
BGP implementation could disagree with that Ze's own tests cannot see: the
UPDATE and WITHDRAW bytes the lifecycle produces are asserted as hex against the
`le test peer` speaker in `api-peer-create-delete-rib.ci`. The one external
caller of these commands is the ExaBGP API bridge, and
`test/exabgp-compat/api/api-peer-lifecycle.ci` (`create neighbor`, then an
announce whose UPDATE bytes it asserts) passed on 2026-10-10 against a ze built
from this checkout (`le test exabgp api --pattern api-peer-lifecycle`, exabgp==5.0.13).

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-api.yang` - `peer-add` renamed `peer-create`, `peer-delete` declared
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `ze:command` and `ze:rpc` for create and delete; descriptions say a reload keeps a created peer
- `internal/component/bgp/plugins/cmd/peer/peer.go`, `create.go`, `save.go` - registrations, handler names, refusal text
- `internal/component/bgp/reactor/reactor_api.go`, `reactor.go`, `reactor_peers.go` - created-peer set, `ReloadRunning`, down event before index removal
- `internal/component/plugin/types.go`, `coordinator.go`, `server/reload.go` - `ReloadRunning` on the reactor interface and its use
- `internal/component/cmd/delete/yang/embed.go`, `register.go` - regenerated glue
- `internal/component/plugin/all/all.go` - regenerated composition root
- `internal/le/doc/yangcontract/contract.go`, `report.go` - the gate's verdict
- `cmd/ze/hub/main_reload_test.go` and the test mock reactors - the new interface method
- `docs/architecture/api/commands.md`, `docs/architecture/exabgp-bridge.md`, `docs/guide/command-reference.md`, `docs/contributing/documentation-testing.md`, `docs/functional-tests.md`

## Files to Create
- `internal/test/fixture/plugin_fixture_13_peer_lifecycle.go`, `register_peer_create_delete_rib.go` - the lifecycle driver
- `internal/test/fixture/daemon_await_exit_fixture.go`, `register_daemon_await_exit.go` - the barrier before the second daemon
- `test/plugin/api-peer-create-delete-rib.ci` - functional test for AC-1 to AC-9
- Removed: `internal/component/cmd/delete/yang/ze-cli-delete-api.yang`

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-bgp-api.yang` rpc `peer-create` and `peer-delete`; `ze-peer-cmd.yang` |
| YANG validation constraints | N-A | no leaf added; the rpc inputs are unchanged |
| YANG custom validators | N-A | none added |
| CLI commands/flags | N-A | the command text is unchanged; only the wire method behind it |
| CLI grammar (keyword before value) | N-A | grammar unchanged; `./le doc yang-contract command-contract` validates all 418 commands |
| Editor autocomplete | N-A | no new value type |
| Functional test for new RPC/API | Yes | `test/plugin/api-peer-create-delete-rib.ci`, `test/plugin/api-peer-save.ci` |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | the commands existed; their model is documented in `docs/architecture/api/commands.md` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (source anchors; a reload keeps a created peer) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (create/delete/save model and wire methods) |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/command-reference.md`; `docs/guide/config-reload.md` (a created peer the file does not declare survives a reload) |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | `DispatchCommand` is unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` section 4 (second daemon on the first one's file); `docs/contributing/documentation-testing.md` (gate verdict) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/exabgp-bridge.md`; `docs/architecture/api/commands.md` states the reload rule |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | published method names; the generated site files (`cli-commands.json`, `llms.txt`) were regenerated on gh-pages 09c9ef7a3e (committed, not pushed) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Updated: `docs/architecture/api/commands.md` (Design doc of `create.go`, `save.go`, fixture 13). Unaffected: `docs/architecture/core-design.md` (Design doc of `reactor.go`, `reactor_api.go`, `reactor_peers.go`, `coordinator.go`, `yangcontract/contract.go`, `report.go`) describes the adapter, peer add/remove and the gate at a level this change does not alter; `docs/architecture/api/process-protocol.md` (`reload.go`, `types.go`) describes process management, not the reload diff; `docs/architecture/hub-architecture.md` (`cmd/ze/hub/main_reload_test.go`) describes SIGHUP orchestration, and the test only adds the new method to its mock |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/commands.md` and `docs/features/api-commands.md` checked against `ze-peer-cmd.yang`; the stale "Remove dynamic peer" line corrected |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring** -- rename the wire methods, close the gate holes
   - Tests: `TestEveryPublishedMethodHasAHandler`, `TestYANGBGPAPIRPCs`, `TestExtractRPCs`
   - Files: `ze-bgp-api.yang`, `ze-peer-cmd.yang`, `peer.go`, `create.go`, `yangcontract/contract.go`
   - Verify: 949091e15a, 3b9366fa72; `./le doc yang-contract command-contract` green
2. **Phase: Reload keeps created peers** -- `ReloadRunning`
   - Tests: `TestUnrelatedReloadKeepsACreatedPeer`, `TestCandidateNamingACreatedPeerTakesItOver`
   - Verify: 4ddd3c3914
3. **Phase: Delete withdraws** -- down event before the index entry goes
   - Tests: `TestRemovedPeerStaysInTheIndexUntilItsDownEvent`
   - Verify: dcfdebb6ba
4. **Phase: Functional proof** -- fixtures 13 and 01, the second daemon
   - Tests: `api-peer-create-delete-rib`, `api-peer-save`
   - Verify: 14fbd132b7, 650f778d2d, 631d1160b5, 760f08569f

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A reload carries forward only the created peers the candidate does not declare; a candidate that declares one takes it over |
| Naming | One prefix for one family: `ze-bgp:peer-create`, `ze-bgp:peer-delete`, `ze-bgp:peer-save`; no `peer-add` or `peer-remove` left outside YANG revision text |
| Data flow | The reload asks the reactor through `ReactorConfigurator`; the reactor owns the created-peer set |
| Rule: `ai/rules/principles.md` | `update bgp config` with a selector is refused rather than answering "0 peers saved" |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Wire methods renamed | `./le doc yang-contract command-contract` lists `ze-bgp:peer-create`, `ze-bgp:peer-delete`, `ze-bgp:peer-save`; `git grep -nE "peer-add\b|peer-remove\b" -- docs ai internal cmd` finds only the YANG revision text |
| `ze-cli-delete-api.yang` removed | `git ls-files internal/component/cmd/delete/yang` |
| Reload keeps created peers | `TestUnrelatedReloadKeepsACreatedPeer`; the AC-9 lines of `api-peer-create-delete-rib.ci` |
| Delete withdraws | `api-peer-create-delete-rib.ci` `expect=bgp:conn=1:seq=2:contains=02000418C000020000` |
| Save persists presence and absence | `api-peer-save.ci` |
| Gate fails on orphans | `TestAnOrphanLocalHandlerFailsTheVerdict`, `TestAnUnservedRPCDeclarationFailsTheVerdict` |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | `create bgp peer` keywords go through the existing handler, which refuses an address already in use and a keyword Ze cannot honor; `update bgp config` refuses any argument |
| Authorization | The commands sit behind the existing dispatcher; no new entry point is added |
| Fail-open | `Coordinator.ReloadRunning` asks the attached reactor; with no reactor it answers its own tree, where no runtime-created peer can exist |
| Resource exhaustion | No per-event loop or allocation added; the created-peer set grows with operator commands only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- 2026-10-08, AC-14..AC-16 gate holes closed in `internal/le/doc/yangcontract/contract.go`:
  `contractSatisfied` takes the whole result and reads all four orphan sets; `publishedRPCs`
  reads every `-api` rpc through `RegisterRPCs`, the derivation both help surfaces use, and
  refuses a loader with no `-api` module; `unservedRPCs` names module, rpc and wire method.
  → Decision: the served set for a declaration includes the skipped editor handlers, because
  the skip list exempts a handler from needing a node, never a declaration from needing a handler.
- The strengthened gate is red on the live tree, truthfully: 14 local handlers with no node and
  117 published rpc methods with no handler (journal: `plan/journal/gate-excludes-part-of-its-population.md`,
  two 2026-10-08 rows). The 117 belong to `spec-rpc-published-name-does-not-reach-its-handler.md`,
  whose owner design (2026-09-06) removes `WireModule`. That set includes this spec's own
  `ze-bgp-cmd-peer:peer-save` and `ze-bgp:peer-remove`, which AC-17 (one declaration under
  `ze-bgp:peer-save`) and the peer-create/peer-delete rename resolve.
  The 14 local handlers: `clear debug`, `delete debug module`, `delete debug profile name`,
  `set debug active name`, `set debug module`, `set debug profile name`, `set debug timeout`,
  `show debug profile` (`internal/plugins/debug/register.go`); `explain`, `skills`, `support`
  (their `internal/plugins/<name>/register.go`); `generate wireguard keypair`
  (`internal/plugins/diag/register.go`); `show config graph`, `validate config`
  (`internal/component/config/cli/register.go`).
  → Constraint: `go test` without the le feature tags links no BGP handler, so the live-tree tests
  must run with `ze_le` plus `feature_tags feature-gates.txt`, as `./le` itself is built.
  -> Decision (owner, 2026-10-08): the 117 unserved published RPCs are fixed by running spec-rpc-published-name-does-not-reach-its-handler next (its 2026-09-06 design removes the module-name derivation); they are not renamed here.
  -> Decision (owner, 2026-10-08): every local command gets a YANG node (one declaration per command, the registration rule); explain, skills, support and validate become top-level operational verbs.
  -> Decision (owner, 2026-10-08): the peer-save "retention ruling" is dropped; it was an agent label, not an owner ruling. ze-bgp-cmd-peer-api.yang is removed by spec-rpc-published-name-does-not-reach-its-handler after its two unique facts move (the peer-save rpc declaration under its served name, and the session input leaf of session-peer-ready).
  -> Decision (owner, 2026-10-08): the 117 published methods with no handler, found by the
  command-contract gate (949091e15a), are handed to `spec-rpc-published-name-does-not-reach-its-handler`,
  which runs next; AC-15's gate turns green there.
  -> Closure status (2026-10-08): this spec CANNOT close yet. Still owed here: the RIB-effect and
  persistence proofs AC-1 through AC-13, and the `peer-add`/`peer-remove` to
  `peer-create`/`peer-delete` rename. AC-14, AC-15 and AC-17 also wait on the other spec landing.
  -> Superseded 2026-10-09: the other spec closed 2026-10-08 (9c346cf094); this spec waits on
  nothing. Its model changed the wire names: the method is the ze:command node's own, and the
  prefix comes from the registering package (756e85f100), so `delete bgp peer` was
  `ze-bgp:delete-peer` documented by `ze-cli-delete-api:bgp-peer`. The rename lands both
  commands on `ze-bgp:peer-create` / `ze-bgp:peer-delete`, declared in `ze-bgp-api.yang`;
  `ze-cli-delete-api.yang` held only that rpc and is removed. Handlers renamed
  `handleBgpPeerCreate` / `handleBgpPeerDelete`.
- 2026-10-08, decision 2a implemented: the 14 local handlers each have a `ze:command` node at
  the path they register. Debug profile commands in `ze-debug-cmd.yang`; `generate wireguard
  keypair` in `ze-diag-cmd.yang`; `show config graph` and top-level `validate config` in
  `ze-config-cli-cmd.yang`; new modules `ze-explain-cmd`, `ze-skills-cmd`, `ze-support-cmd` under
  `internal/plugins/<name>/yang/` (glue by `./le yang glue write`, composition root by
  `./le plugin imports write`). Gate: 14 -> 0 local orphans, YANG commands 404 -> 418.
  -> Constraint: a node is declared only at the registered path, never below it.
  `registry.LookupLocal` refuses a local match when the argv reaches a declared command further
  down, so `show debug profile name <n>` keeps `name` as a leaf, and `skills list|get` is a
  `one-of` modifier group rather than two commands.
  -> Constraint: `support` declares no leaf. Its grammar is Go flags plus an optional bare
  positional, which the model cannot state (a bare optional positional is refused by the CLI
  grammar), so the node carries the summary and the root's own help keeps the options.
  Walked into, journalled: `doctor` is not scanned by the gate (gate-excludes-part-of-its-population);
  `ze help` lists a root-won verb twice (help-lists-a-root-verb-twice); `set debug module` drops
  every option after the first (silent-fall-through).

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The reload compares the candidate against a running tree without the undeclared created peers (`ReloadRunning`) | A new origin marker on every peer; making the decomposer skip created peers | The reactor already holds each created peer's tree (`recordPeerConfig`), so leaving those entries out of the comparison needs no new state, and the diff and decomposer stay unaware of the distinction |
| `update bgp config` takes no selector | A selector form | After a delete the peer is gone, so a selector would select nothing and answer "0 saved" for a delete, a typo and a peer that never existed alike |
| The down event is published before the peer leaves the delivery index | Publishing after removal | After removal the graph has no edge to the processes the peer fed, so the RIB never heard the peer went down and sent no WITHDRAW |
| A fresh daemon on the file proves AC-7, AC-10 and AC-11 | A SIGHUP reload of the file | The ACs say "a daemon started on that file"; a reload reuses runtime state a start does not have |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- Implemented but unverified: a candidate that declares a created peer's ADDRESS under a different peer NAME. No AC names this case and no test exercises it; review decides whether it needs a test here.
- `recoveryRoutes` reads `ribOut[Destination]`, so a destination not attached to `bgp-rib` gets no WITHDRAW when a source goes down. Not specific to peer delete (every source-down), journalled in `plan/journal/silent-fall-through.md` (2026-10-10); AC-6 holds where the destination attaches `bgp-rib`, which the test configures.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer added at runtime reaches the Adj-RIB-In, the RIB and the best path, and writes nothing to the file | user workflow + data correctness | `test/plugin/api-peer-create-delete-rib.ci` PASS: "the created peer's route is in the RIB and is the best path", "the configuration file and the running configuration name no created peer"; forward asserted as `contains=18C00002` on the configured peer |
| A peer removed at runtime takes its routes out, and the withdrawals reach consumers | data correctness | same `.ci`: configured peer receives `02000418C000020000` (WITHDRAW 192.0.2.0/24) and a subscribed plugin is told; `TestRemovedPeerStaysInTheIndexUntilItsDownEvent` red with the old publish order |
| A runtime peer survives an unrelated commit and a reload | user workflow | same `.ci`: "an unrelated commit keeps the created peer and its routes", "a reload keeps the created peer and its routes" (one TCP connection asserted); `TestUnrelatedReloadKeepsACreatedPeer` |
| Deleting a configured peer leaves the file alone and a daemon started on it brings the peer back | user workflow | same `.ci`, second daemon via `le test fixture daemon/await-exit` then `ze start`; red in an export with delete persisting: "a daemon started on the unchanged file does not run the configured peer 127.0.0.1" (session scratch `red-rib.log`) |
| `peer-save` persists presence and absence of the whole running set | user workflow | `test/plugin/api-peer-save.ci` PASS including a second daemon started on the saved file; red in an export where save writes nothing: "a daemon started on the saved file runs no peer 192.0.2.7" (session scratch `red-save.log`) |
| `peer-save` refuses a selector and says why | security/negative | `TestPeerSaveRefusesASelector` (red before 650f778d2d); fixture 01 asserts the refusal text through the dispatcher |
| One verb per operation: `ze-bgp:peer-create`, `ze-bgp:peer-delete`, `ze-bgp:peer-save` | gate output | `./le doc yang-contract command-contract` 2026-10-10: 418 YANG commands, "All commands validated", rows map `ze-bgp:peer-create` to `create > bgp > peer`, `ze-bgp:peer-delete` to `delete > bgp > peer`, `ze-bgp:peer-save` to `update > bgp > config` |
| Every published method has a handler; the gate fails on an orphan declaration or handler | gate + unit | `TestEveryPublishedMethodHasAHandler`, `TestAnUnservedRPCDeclarationFailsTheVerdict`, `TestAnOrphanLocalHandlerFailsTheVerdict` (949091e15a) |
| The six removed declarations stay absent; `peer-save` declared once | gate | `TestEveryCommandNodeHasASummary` green; command-contract lists one `ze-bgp:peer-save` row |
| The ExaBGP bridge still reaches the renamed commands | interop-adjacent | `test/exabgp-compat/api/api-peer-lifecycle.ci` PASS 2026-10-10 (exabgp==5.0.13) |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

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
