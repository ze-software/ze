# Spec: config-apply-ordering-covers-every-root

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze orders config changes across components with an operation graph:
`BuildOperationGraph` builds it, `TopologicalSort` orders it, and
`OperationExecutor` runs verify, execute and commit over the sorted list.

**The order a commit applies is the owner's, and it is quoted verbatim in
`docs/architecture/config/apply-ordering.md` under "The requirement". That page
is the specification. Where it and this spec disagree, the page is right.** This
spec was written from a paraphrase of the owner's May 2026 words, and the
paraphrase said make-before-break: add the address, update the services that
bind it, remove the old address last, with both addresses present while two
interfaces swap. That is not what he asked for, and every sentence derived from
it is corrected here rather than repeated.

The owner's order removes before it adds, with the binders stopped around the
move. Five phases, quoted on the page and summarized here:

| Phase | What runs |
|-------|-----------|
| 1 | Stop the binders the new config removes |
| 2 | Stop the binders whose bound address is disturbed |
| 3 | Remove the addresses that are removed or disturbed |
| 4 | Add the addresses that are new or moved |
| 5 | Start the binders against the new addresses |

The vocabulary is the page's, and this spec uses no second word for any of it. A
BINDER is a component that binds a local IP address: bgp, ike, l2tp, dhcp, ntp,
gnmi, tftp, and any listener a plugin opens. The interface layer is the PROVIDER
of the addresses they bind. An address is DISTURBED when the commit removes it,
when it changes interface, or when its prefix length changes, and not by a
change that leaves the address row intact. Where the core cannot establish
whether a binding context still means what it meant, it treats the binder as
disturbed and stops it.

Four defects hold that ordering to two config roots.

| ID | Defect | Operator symptom |
|----|--------|------------------|
| D1 | `TxCoordinator.Execute` takes the operation path only when operations exist AND no participant is uncovered. One participant with a diff and no operation drops the whole transaction to the unordered section apply, including the address operations that were ordered correctly. The fallback logs at `Info` and the transaction reports success | A commit that changes an interface address and a firewall rule together gets no address ordering. A service is updated before its address exists, or an address is removed while a listener still binds it |
| D2 | `RegisterOperationDecomposer` has two non-test callers: `internal/component/bgp/plugin/operation.go` (root `bgp`) and `internal/component/iface/operation.go` (root `interface`). The same two files are the only registrants of constraint rules and settlement rules. `decomposeRootOperations` returns an empty result for every other root | firewall, dhcp, ike, l2tp, static, ntp and the rest declare no ordering, so a service in those roots that binds an address has no edge saying so. Each of them also triggers D1 |
| D3 | The ordering vocabulary is a central enumeration in the plugin ABI: 21 `ConfigOperationType` constants and 9 `ResourceKind` constants in `pkg/plugin/rpc/types.go`. Seven operation kinds have no user outside their own declaration and the two alias files | A new root joins the ordering only by editing a central enum, which `ai/rules/principles.md` bans. The vocabulary was written for roots that never arrived |
| D4 | The functional rotation, swap and reip tests emit `remove-peer` and `add-peer` only, never `add-address` or `remove-address`. The applied address order is asserted by unit tests over the solver alone | No test reads what a commit applies through decompose, graph, executor, bridge, RPC and reactor, so a break anywhere on that path is invisible. It is also what let the paraphrase above survive four months: nothing ever read the applied order off a kernel, so a make-before-break test and a break-before-make test both passed against their own model |

Goal: every config root with a diff is a node in the operation graph, the
ordering is derived from what each operation produces and consumes rather than
from named operation pairs, the section-apply fallback is deleted, and the
applied order is read off a real kernel and matches the owner's five phases.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/apply-ordering.md` - the design of the subsystem this spec changes.
  → Decision: decomposition and constraint rules live in the owning component, never in a central switch. This spec keeps that and extends it to the roots that own no decomposer.
  → Constraint (CORRECTED 2026-09-11): the page now carries the owner's words under "The requirement", and they outrank every sentence in this spec. His order removes before it adds, so the cross-interface cycle the relaxation existed to break cannot form: each address carries one destroy, one create and a single edge between them. `tryRelaxCycle` is deleted and every cycle is rejected.
- [ ] `docs/architecture/config/transaction-protocol.md` - the participant, verify, apply and rollback protocol the operation path sits inside.
  → Decision: phase 1 verifies the whole candidate config for every participant before any operation runs, so a coarse root node owes no second verify.
  → Constraint: the operation path reaches operation OWNERS only, because `runOperationPath` keys its events off `op.Owner`. A root with no node is never applied by that path.
- [ ] `docs/architecture/api/process-protocol.md` - the five `config-operation-*` callbacks and the section callbacks.
  → Constraint: `config-operation-apply` has no default handler in the SDK, so a plugin that never called `OnConfigOperationApply` answers "unknown method". A coarse root node must therefore reach the plugin through `config-apply`.
- [ ] `ai/rules/principles.md` - registration over central enumeration, and no silently wrong value.
  → Constraint: the operation vocabulary must stop being a list the core owns. A payload the core cannot order is refused, never ordered arbitrarily.
- [ ] `ai/rules/no-layering.md` - replacing X with Y deletes X first.
  → Constraint: the section-apply fallback and the produce/consume constraint rules are deleted, not kept beside the new mechanism.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is `config`. No RFC governs the config transaction
ordering, and the plugin RPC contract is Ze's own.

**Key insights:**
- Coverage is what makes ordering unconditional. D1 is not a bug in the graph, it is the graph never being used.
- The solver reads the operation type in two places only, `isAddressOperation` and `markDualPresence`, and both are asking "does this create or destroy an address". Both served make-before-break, so both are deleted with it rather than rewritten against the verb.
- The requirement was lost to a paraphrase before this spec was written, and D4 is why the loss survived. No test read the order a commit APPLIES, so a suite built on the paraphrase was as green as a suite built on the requirement.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/transaction/operation.go` - declares the registry: `RegisterOperationDecomposer`, `RegisterConstraintRule`, `RegisterSettlementRule` and their read sides. `ConstraintRule` carries `Before` and `After` `OperationSelector` values plus a `ResourceRelation`. `OperationSelector` matches on `Type` and `ResourceKind`. The file aliases every operation and resource constant from `pkg/plugin/rpc`.
- [ ] `internal/component/config/transaction/depgraph.go` - `BuildOperationGraph` walks every registered rule against every ordered pair of operations, calls `matchesSelector` on both ends and `operationsRelated` on the pair, and adds one edge per unique pair. `operationsRelated` implements the five `ResourceRelation` values. `resourceKey` builds the identity string per `ResourceKind` and has a default branch for a kind the core does not name.
- [ ] `internal/component/config/transaction/solver.go` - `TopologicalSort` runs `kahnSort`, then `tryRelaxCycle` when nodes remain, then `kahnSort` again, then `markDualPresence`. `isAddressOperation` matches `OperationAddAddress` or `OperationRemoveAddress` with a target kind of `ResourceAddress`. `tryRelaxCycle` rejects a cycle unless every member is an address operation, and removes only the cross-interface edges.
- [ ] `internal/component/config/transaction/executor.go` - `Verify`, `Execute`, `Commit` and `rollbackApplied` each emit a per-operation event keyed by `op.Owner` and wait for the matching ack. An operation with an empty owner is an error. `armSettlementWaiters` subscribes before the apply is emitted.
- [ ] `internal/component/config/transaction/orchestrator.go` - `Execute` runs `runVerify` for every participant, then calls the planner, then `participantsWithoutOperations`. It calls `runOperationPath` only when operations exist and nothing is uncovered. Otherwise it logs at `Info` and falls through to `runApply`, the unordered section apply. `filterDiffs` is the single predicate deciding whether a participant takes part in verify, in apply and in the coverage test.
- [ ] `internal/component/plugin/server/reload_tx.go` - `runTxCoordinator` builds participants and diffs, starts the bridge, and installs the planner from `operationPlannerFromTrees`. The planner walks the sorted roots, takes the in-process decomposer when one is registered, and otherwise calls `decomposeRootOperations`. That function returns an empty result when no participant declares operations for the root. `validateOperationDeclarations` rejects an operation whose owner did not declare its type for its root.
- [ ] `internal/component/iface/operation.go` - registers `decomposeIfaceOperations` for root `interface`, seven constraint rules and two settlement rules. Five of the seven rules state a produce/consume fact (`iface-add-interface-before-address`, `iface-remove-address-before-interface`, `iface-add-interface-before-tunnel`, `iface-add-interface-before-bridge-member`, `iface-remove-bridge-member-before-interface`). Two do not (`iface-remove-address-before-add-same-address` is address uniqueness, `iface-add-address-before-remove-same-interface` is make-before-break). `applyIfaceOperation` dispatches on the operation type and never reads `Params.AllowDual`.
- [ ] `internal/component/bgp/plugin/operation.go` - registers `decomposeBGPOperations` for root `bgp`, four constraint rules and one settlement rule. All four rules state one fact: an address must exist before a peer or a listener binds it, and must outlive both.
- [ ] `pkg/plugin/rpc/types.go` - declares `ConfigOperationType` with 21 constants, `ResourceKind` with 9, `ResourceRef`, `ConfigOperationParams` (which carries `AllowDual`), `ConfigOperation`, and the operation callback payload pairs. `OperationStartDHCP`, `OperationStopDHCP`, `OperationSetSysctl`, `OperationAddStaticRoute`, `OperationRemoveStaticRoute`, `OperationSetDistance` and `OperationSetProperty` have no user outside this file, `pkg/plugin/sdk/sdk_types.go`, `internal/component/config/transaction/operation.go` and tests.
- [ ] `pkg/plugin/sdk/sdk_types.go` - re-exports the rpc operation types and constants unchanged for external plugin authors.
- [ ] `internal/component/plugin/server/config_tx_bridge.go` - `subscribeOperationDecompose`, `subscribeOperationVerify`, `subscribeOperationApply`, `subscribeOperationRollback` and `subscribeOperationCommit` each translate a per-plugin stream event into the matching RPC on the plugin connection. `phaseKind.runRPC` drives the section path: `SendConfigVerify` for verify and `SendConfigApply` for apply.
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `initCallbackDefaults` registers defaults for `config-verify`, `config-apply` and `config-rollback`, and for none of the five `config-operation-*` callbacks. An unregistered callback returns "unknown method" from `sdk_dispatch.go`.
- [ ] `test/reload/config-apply-ordering-rotation.ci`, `-swap.ci`, `-reip.ci` - all three rotate BGP router-ids and assert the peers reconnect. Their own comments say the address operations they stand in for are not the ones they emit.
- [ ] `test/reload/config-apply-ordering-create.ci`, `-delete.ci` - emit real interface create and delete operations. Both are gated `option=needs-linux:caps=net-admin`.
- [ ] `test/reload/tx-iface-address-swap.ci` - renumbers one address within one subnet on ONE interface and reads the result back with `ip addr show`. It produces no cross-interface cycle, so it never reaches `tryRelaxCycle`.

**Behavior to preserve:**
- Verify, then execute, then commit, with rollback replaying inverse operations in reverse order and excluding the failed operation.
- Phase 1 full-config verify for every participant, before any operation runs.
- The section apply path itself (`runApply`, `phaseApply.runRPC`, `SendConfigApply`). It becomes how a coarse root node is applied, instead of how a whole transaction escapes ordering.
- `ConfigOperation` keeps its JSON keys `id`, `root`, `owner`, `type`, `target` and `params`, and `type` keeps its kebab-case values.
- Settlement waiters are armed before the apply is emitted.
- One iface rule states a fact no produce and consume pair can carry, and it keeps working: every address the commit removes leaves the host before any address the commit adds arrives.

**Behavior to change:**
- A transaction with a diff on a root that owns no decomposer takes the operation path, not the section apply. The `Info` log line "operations do not cover every participant, using section apply" stops existing, because its branch stops existing.
- Ordering edges between a producer and a consumer of one resource are derived from declared produce and consume sets. The nine hand-written rules that state a produce/consume fact are deleted.
- The solver stops reading the operation type. It reads a new verb (`create`, `destroy`, `modify`) and the target resource kind.
- `ConfigOperationType` stops being a core enumeration. The seven unused constants are deleted, and the constants iface and bgp dispatch on move into those two packages.
- An operation with no verb is refused at planning with a named error, rather than ordered as if it were `modify`.
- Make-before-break is DELETED, not rewritten against the verb. `Params.AllowDual`, `markDualPresence`, `tryRelaxCycle`, `isAddressOperation`, `opInterface`, the rule `iface-add-address-before-remove-same-interface` and the two relations `same-interface` and `same-address` all go. Every cycle is now rejected: `TopologicalSort` answers `ErrOperationCycle` for any graph it cannot order.
- The rule `iface-remove-address-before-add-same-address` widens to `iface-remove-address-before-add-address`, which states phases 3 and 4 directly. Without the widening a renumber would be unordered, because the old address and the new one are two different addresses and the planner emits its additions first.
- The core computes which addresses a commit disturbs (`DisturbedAddresses`), the planner carries the set to every root that registers a decomposer even when that root has no diff (`operationPlannerFromTrees`, `bindingRoots`, `appendDecomposingPlugins`), and the owning component decides what stopping means (`decomposeBGPOperations`, `peerBindingDisturbed`). That is phases 2 and 5.
- The iface decomposer covers its whole root on every call. An address and an interface of a type it can create become one operation each, and every other key rides one `configure-interfaces` operation that applies the interface config as a whole (`decomposeIfaceOperations`, `ifaceConfigureOperation`). It used to produce NO operation whenever one key in its diff had no primitive, so a commit that edited an MTU and moved an address read as a commit that disturbed nothing.
- A coarse node is placed at the phase 4 to phase 5 boundary, after the addressing the commit adds and before the first operation that binds it (`placeSectionNodes`, `sectionNodePosition`, `providesAddressing`). `sortParticipantsBGPLast` and `bgpParticipantName` are deleted, so no core package names a root to decide the order.
- A rolled-back peer returns as the reactor was running it, not as the operation's config subtree describes it (`runningPeerSettings`), and a modify-peer the running session can take swaps its settings in place (`swapPeerForOperation`, `peerSettingsSwapPlan`).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator edits the config file and sends SIGHUP, or commits from the CLI editor. `Server.reloadConfig` computes a `config.ConfigDiff` over the running and candidate trees.
- Format at entry: per-root JSON sections in `transaction.DiffSection` (`root`, `added`, `removed`, `changed`).

### Transformation Path
1. `runTxCoordinator` builds participants from the affected plugin registrations, builds per-root `DiffSection` slices, starts `configTxBridge`, and installs the planner.
2. `TxCoordinator.Execute` runs phase 1 verify for every participant with a diff.
3. The planner walks the sorted roots. A root with an in-process decomposer, or a participant declaring decomposition, produces operations. Every other root produces nothing today. After this spec the orchestrator synthesizes one coarse node per uncovered participant.
3a. `DisturbedAddresses` reads the planned operations and returns every address a destroy produces. Where that set is not empty the planner decomposes a second time, carrying the set on `DecomposeRequest.DisturbedAddresses` to every root that registered a decomposer, whether or not that root has a diff. That is how a binder with no diff of its own emits its own stop and its own start.
4. `BuildOperationGraph` adds an edge for every registered constraint rule that matches a pair, and one for every producer and consumer pair over the same resource identity.
5. `TopologicalSort` orders the graph and rejects any cycle. `placeSectionNodes` then puts each coarse node at the phase 4 to phase 5 boundary, using `sectionNodePosition`.
6. `OperationExecutor` emits verify, then apply, then commit per operation on the owner's stream event. A coarse root node is applied through the participant's section apply event instead.
7. `configTxBridge` turns each stream event into the plugin RPC: `config-operation-verify`, `config-operation-apply` and `config-operation-commit` for a decomposed operation, and `config-apply` for a coarse root node.
8. The owning component applies the change: `applyIfaceOperation` reaches the netlink backend, `applyBGPOperation` reaches the reactor.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ transaction | `DiffSection` per root, built by `buildTxInputs` | No |
| Orchestrator ↔ planner | `OperationPlanner` func, `OperationPlanRequest` | No |
| Transaction ↔ plugin (in-process and external) | engine stream events, then the RPC methods `config-operation-*` and `config-apply` | No |
| Core ↔ owning component | `ConfigOperation` value carrying verb, target, produces, consumes and params | No |
| Plugin SDK ↔ external plugin author | `pkg/plugin/sdk` aliases of the rpc types | No |

### Integration Points
- `transaction.RegisterOperationDecomposer` - signature unchanged. Roots join by registering, and the core keeps no list of them.
- `transaction.RegisterConstraintRule` - survives for facts that are not produce/consume. Its produce/consume users are deleted.
- `TxCoordinator.participantsWithoutOperations` - changes from a fallback trigger into the input of coarse-node synthesis.
- `configTxBridge.subscribeOperationApply` and `phaseApply.runRPC` - the two apply routes a node can take.
- `transaction.DisturbedAddresses` and `DecomposeRequest.DisturbedAddresses` - the core establishes WHICH addresses are disturbed and the owning component decides what stopping means. The core reads no root's semantics and keeps no list of binders.
- `transaction.OperationDecomposerRoots` and `bindingRoots` - the roots the planner asks on its second pass, which is every root that registered a decomposer rather than every root with a diff.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | To fill at implementation: the coarse node reaches the plugin through the existing section apply RPC, not a new one |
| No unintended coupling (components stay isolated) | No | To fill: the solver names no root, no component and no operation label after the change |
| No duplicated functionality (extends existing, does not recreate) | No | To fill: derived edges replace nine rules, and no surviving rule restates a derived fact |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A on this path: config operations are small value structs crossing a JSON boundary, not wire encoding |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | To fill: `pkg/plugin/rpc/types.go` holds no per-root operation constant when this lands |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A coarse root node applied through `config-apply` applies exactly what the section apply applies today, because it carries the same `DiffSection` slice `filterDiffs` produces | `orchestrator.go` `runApply` and `filterDiffs`; `config_tx_bridge.go` `phaseApply.runRPC` | The coarse node changes what an uncovered participant receives, and a root silently applies a different change | Unit test comparing the payload the coarse node emits against the payload `runApply` emits for the same participant and the same diffs | confirmed (phase 1). One function emits both, `emitSectionApply` (`orchestrator.go`), and both take their diffs from `filterDiffs`: `runApply` calls it directly, the coarse node through `sectionDiffsFor`. `TestExecuteCoarseNodeAppliesSection` asserts the emitted `ApplyEvent.Diffs` equals the participant's diffs |
| A-2 | Phase 1 verify already covers a coarse root node, so the node owes no per-operation verify | `orchestrator.go` `Execute` runs `runVerify` for every participant before the planner | A coarse node reaches apply with its root unverified | Unit test asserting the participant received `config-verify` and the coarse node emits no `config-operation-verify` | confirmed (phase 1). `Execute` runs `runVerify` over every participant with diffs before it calls the planner (`orchestrator.go`), and `OperationExecutor.Verify` skips a coarse node. `TestExecuteCoarseNodeEmitsNoOperationVerify` asserts one full-config verify and no operation verify, apply or commit event for that participant |
| A-3 | The five `config-operation-*` callbacks have no SDK default, so routing a coarse node to `config-operation-apply` fails with "unknown method" | `pkg/plugin/sdk/sdk_callbacks.go` `initCallbackDefaults`; `sdk_dispatch.go` | The design's reason for the section route is wrong, and a simpler route exists | Read at both sites during design; re-assert with a unit test over a plugin that registers no operation callbacks | confirmed (phase 1) by reading the producers. `initCallbackDefaults` (`pkg/plugin/sdk/sdk_callbacks.go`) registers defaults for `config-verify`, `config-apply`, `config-rollback`, deliver-event, deliver-batch, validate-open, doctor-check, enrich-show, bye and post-startup, and for none of the five `config-operation-*` callbacks. An unregistered method answers "unknown method" at both dispatch sites, `handleBridgeCallback` and the event loop (`pkg/plugin/sdk/sdk_dispatch.go`). `TestSDKPluginWithoutOperationCallbacksAnswersUnknownMethod` (phase 2) now asserts it at the dispatcher: each of the five answers "unknown method" to a plugin that registered none, and the same plugin answers `config-apply` |
| A-4 | `ResourceKind` is already open, because `resourceKey` has a default branch, so a root can carry a kind the core does not name | `depgraph.go` `resourceKey` | The vocabulary change has to open `ResourceKind` as well as the operation label | Unit test with an unnamed resource kind on both ends of a produce/consume pair | confirmed (phase 3) by reading the producer. `resourceKey` (`depgraph.go`) ends in a default branch keying an unnamed kind by its own string plus the first non-empty name, interface or address. The derivation's `resourceIdentity` keeps that branch, so a produce and a consume entry of kind `wireguard-tunnel` pair by identity like every other. `TestBuildOperationGraphUnknownResourceKindOrders` asserts it, and asserts a second resource of that kind earns no edge |
| A-5 | The `static` plugin (`ConfigRoots` is `static`) is a real uncovered participant, usable as the third root in the mixed-root functional test | `internal/plugins/static/register.go`; only iface and bgp register decomposers or operation callbacks | The mixed-root test needs a different third root | Run the mixed-root test before the fix and confirm the fallback log line appears | confirmed as a FACT about static, and the premise under it is broken (phase 1). `static` declares `WantsConfig: []string{pluginName, "interface"}` (`internal/plugins/static/register.go`) and registers no decomposer: `RegisterOperationDecomposer` has exactly two non-test callers, `internal/component/iface/operation.go` and `internal/component/bgp/plugin/operation.go`. It is therefore a real uncovered participant. What is broken is "uncovered participants are rare": a dozen BGP plugins declare the `bgp` root and none decomposes (`rib`, `gr`, `rpki`, `bmp`, `rs`, `watchdog`, `hostname`, `softver`, `llnh`, `healthcheck`, `route_refresh`, the `filter_*` set), so EVERY bgp reload took the fallback. Phase 1's coarse-root functional test uses the `rsvp-te` root instead of `static`, because the static plugin applies to the kernel FIB and the coarse-root case needs no privilege |
| A-6 | No other `plan/` spec is editing these files | Working tree inspection at design time | A merge conflict, or two designs for one defect | `git status` plus a grep of `plan/` for `apply-ordering` before implementation starts | confirmed (2026-09-08, before phase 1). `git status` showed no modification to the transaction, iface, bgp-plugin, plugin-server or rpc files. Two other specs mention `apply-ordering` and neither edits them: `spec-vpp-interface-in-use` reads `internal/component/iface/operation.go` to say its VPP referential-integrity verify is NOT that ordering, and `spec-peer-deactivate-and-bulk-route-purge` (skeleton) lists the page as required reading. That second spec states `OperationRemovePeer` lives in `pkg/plugin/rpc/types.go`, which phase 2 made false; it is another spec's row to correct |
| A-7 | Nothing outside `solver.go` reads `Params.AllowDual`, so the dual-presence window is produced by the removed edges alone | grep over `internal/` and `pkg/`: the only non-test hits are `solver.go` and the field declaration | Deleting or keeping the flag changes apply behavior | The address-swap functional test asserts the applied order from the kernel, independent of the flag | confirmed (phase 4) by reading the producer and every reference, then made MOOT at phase 5. `markDualPresence` (`solver.go`) was the only writer and also the last reader: nothing read the field back. `applyIfaceOperation` (`internal/component/iface/operation.go`) dispatched on the label and never touched `Params.AllowDual`, so the applier could not behave differently for a dual-marked create. The flag labelled a policy the owner never asked for, so phase 5 deleted the field, its writer and the relaxation that fed it. A grep of `internal` and `pkg` for `AllowDual` and `markDualPresence` now returns only two history comments |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Total coverage puts every participant into one ordered sequence, so a slow root delays roots that used to apply beside it | Reload wall time grows across the reload functional tests | The executor already applies one operation at a time. Measure the reload suite before and after, and report the delta rather than hide it. MEASURED on 2026-09-11, over the deadline change of review round 2: 44 of 44 pass both times, 182.3s before (`.../scratch/cao-functional-reload.log`) and 162.9s after (`.../scratch/cao2-functional-reload-after.log`). No test moved outside the run-to-run spread the "slow" lines already report, and none timed out. The budget itself is no longer the cost the suite measures: the deadline bounds the wait for a plugin that does not answer, so a healthy reload never reaches it |
| R-2 | Derived edges over-connect: a resource many roots consume produces a dense graph and a new cycle, turning a working reload into an aborted transaction. Phase 5 raised this risk, because every cycle is now rejected and no relaxation survives | A reload test that passed now aborts with `operation dependency cycle` | Land the derivation against the existing iface and bgp fixtures first and compare the edge set, before any new root declares produce/consume. A coarse node is a placement rather than an edge, so it joins no cycle (`TestTopologicalSortSectionNodeJoinsAnAddressSwapWithoutACycle`) |
| R-3 | Deleting the fallback turns a previously silent ordering loss into a transaction abort when the graph cannot be built | An abort on a config the operator applied yesterday | The coarse node makes every participant representable, so an abort now means a real cycle. Name the operation ids and the cycle members in the abort message |
| R-4 | The coarse node's rollback differs from a decomposed operation's rollback, because the participant expects `config-rollback` rather than `config-operation-rollback` | A rollback test leaves one participant unrolled | Cover both kinds in the rollback unit test and in the mixed rollback functional test, before the fallback is deleted |
| R-5 | The verb is a second spelling of what the label already implies, so a decomposer sets the two inconsistently | A peer create ordered after its address destroy | Refuse an operation with no verb at planning, and never infer the verb from the label |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every config reload. A wrong order removes an address while a listener binds it (peers drop), or creates a peer before its address exists (the peer never establishes). A wrong coverage decision silently discards a root's config change while reporting success, which is the failure the current fallback exists to avoid |
| How is it reverted? | Single commit revert. No config migration, no on-disk state, and no peer-visible artifact survives the process. The plugin ABI change is visible to an external plugin binary built against the old SDK, so a revert also reverts that contract |
| Who else touches this path? | `internal/component/iface`, `internal/component/bgp/plugin`, `internal/component/plugin/server` (reload and bridge), and every plugin declaring `ConfigRoots` or `WantsConfig`, which is most of `internal/plugins/`. Externally, any plugin built against `pkg/plugin/sdk` |
| What happens to a v1 external plugin sending the old payload? | Its `type` value still parses, because the JSON key and the kebab-case values are unchanged. Its payload carries no verb, so the planner refuses the operation and the transaction aborts with a message naming the plugin, the root and the missing field. It is refused, never ordered as `modify` by default (`ai/rules/principles.md`: a value that is silently wrong must not be reachable). Ze is pre-release with no shipped external plugin, so no compatibility shim is written (`ai/rules/no-layering.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP on a config changing an interface address and a static route | → | `TxCoordinator.Execute` takes `runOperationPath` | `test/reload/config-apply-ordering-mixed-root.ci` |
| SIGHUP on a config whose only diff is in a root with no decomposer | → | the orchestrator synthesizes a coarse node, the bridge sends `config-apply` | `TestExecuteCoarseNodeAppliesSection` in `orchestrator_test.go` |
| A decomposer declaring produces and consumes, registering no constraint rule | → | `BuildOperationGraph` derived edge | `TestBuildOperationGraphDerivesProducerBeforeConsumer` in `depgraph_test.go` |
| SIGHUP swapping two addresses between two interfaces | → | `TopologicalSort`, then `OperationExecutor.Execute` to the netlink backend | `test/reload/config-apply-ordering-address-swap.ci` |
| SIGHUP moving an address a BGP peer binds, with no diff in the `bgp` root | → | `DisturbedAddresses`, the planner's second decompose pass, `decomposeBGPOperations` | `test/reload/config-apply-ordering-address-swap.ci`, and `TestReloadStopsABinderWhoseAddressMovesAndItsOwnConfigDidNot` in `reload_disturbed_test.go` |
| SIGHUP editing an MTU and moving an address in one commit | → | `decomposeIfaceOperations` covers the whole root, so the address destroy still reaches `DisturbedAddresses` | `TestIfaceOperationDecomposerMixedDiffStillMovesTheAddress` in `internal/component/iface/operation_test.go` |
| An operation whose label the core does not name | → | `OperationExecutor` emits it on the owner's event, the owner dispatches on the label | `TestOperationPathCarriesUnknownLabel` in `executor_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One reload changes an interface address and a static route in the same transaction | The address operations keep their order: the new address is present before the route that binds it is installed, and the old address is removed after. No log line says the transaction used the section apply |
| AC-2 | One reload whose only diff is in a root that registers no decomposer | The root's change is applied and the transaction reports committed. The applied result is identical to what the section apply produced before this spec |
| AC-3 | A root declares that its operation consumes an address, and registers no constraint rule | The address create runs before that operation, and the address destroy runs after it |
| AC-4 | One reload swaps address A from interface X to interface Y and address B from Y to X, with a BGP peer bound to A | Each address leaves the interface that held it before it arrives on the one that takes it, and no interface holds both at any observable point. The peer bound to A is stopped before A leaves X and started after A arrives on Y. The transaction commits. CORRECTED 2026-09-11: this row required the opposite, both addresses present for the whole swap, which was the paraphrase |
| AC-8 | One reload changes an interface MTU and moves an address a binder holds, in the same commit | The address destroy still reaches `DisturbedAddresses`, so the binder is stopped and started. The MTU reaches the component on the `configure-interfaces` operation |
| AC-9 | One reload changes an address a binder holds in a way that leaves the address row intact | No binder is stopped. `DisturbedAddresses` returns nothing, so the planner makes one decompose pass and asks no root that has no diff |
| AC-5 | A decomposer emits an operation whose label no core package names, carrying a verb and a resource kind the core does name | The core orders it by verb and resource, carries the label unchanged to the owner, and the owner dispatches on it. No core package compares that label to a constant |
| AC-6 | An operation arrives with no verb | The transaction aborts with an error naming the plugin, the root and the operation id. Nothing is applied |
| AC-7 | One participant's apply fails, in a transaction mixing decomposed operations and a coarse root node | Rollback reaches both kinds: the decomposed operations replay their inverses in reverse order, and the coarse node's participant receives the section rollback. The transaction reports rolled back |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Changes an interface address and a static route in one commit | config file → diff → planner → graph (derived edges) → solver → executor → bridge → iface backend and the static plugin | `test/reload/config-apply-ordering-mixed-root.ci` |
| 2 | Swaps the addresses of two interfaces in one commit, with a peer bound to one of them | config file → diff → iface decomposer → `DisturbedAddresses` → bgp decomposer → graph → solver → executor → netlink and the reactor | `test/reload/config-apply-ordering-address-swap.ci` |
| 3 | Changes only a config root that owns no decomposer | config file → diff → planner (no operations) → coarse node → executor → bridge `config-apply` → plugin | `test/reload/config-apply-ordering-coarse-root.ci` |
| 4 | Reloads a config whose apply fails half way through a mixed transaction | executor → apply failure → `rollbackApplied` and the section rollback | `test/reload/config-apply-ordering-mixed-rollback.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildOperationGraphDerivesProducerBeforeConsumer` | `internal/component/config/transaction/depgraph_test.go` | A create operation producing a resource precedes a create operation consuming it, with no constraint rule registered | PASS, and observed RED with the derivation removed |
| `TestBuildOperationGraphDerivesConsumerBeforeProducerOnDestroy` | `internal/component/config/transaction/depgraph_test.go` | A destroy operation consuming a resource precedes the destroy of its producer | PASS, and observed RED with the derivation removed |
| `TestBuildOperationGraphNoDerivedEdgeAcrossDifferentResources` | `internal/component/config/transaction/depgraph_test.go` | Produce and consume of different resource identities create no edge | PASS. It fences an absence, so no revert reddens it |
| `TestBuildOperationGraphUnknownResourceKindOrders` | `internal/component/config/transaction/depgraph_test.go` | A resource kind no core constant names still matches by identity (A-4) | PASS, and observed RED with the derivation removed |
| `TestBuildOperationGraphDerivedEdgesMatchDeletedRules` | `internal/component/config/transaction/depgraph_test.go` | On the existing iface and bgp operation fixtures, the derived edge set equals the set the nine deleted rules produced | PASS, with two named additions (see Design Insights). Observed RED with the derivation removed |
| `TestTopologicalSortSwapsAddressesBreakBeforeMake` | `internal/component/config/transaction/solver_test.go` | A cross-interface swap sorts both destroys before either create, and it needs no relaxation because it closes no cycle. REPLACES the phase-2 row `TestTopologicalSortRelaxesCycleByVerbAndKind`, which asserted the paraphrase | EXISTS. Not run in this documentation pass |
| `TestTopologicalSortRotatesAddressesBreakBeforeMake` | `internal/component/config/transaction/solver_test.go` | A three-way rotation sorts the same way, so no cycle forms there either | EXISTS. Not run in this documentation pass |
| `TestTopologicalSortRejectsEveryCycle` | `internal/component/config/transaction/solver_test.go` | Every cycle answers `ErrOperationCycle`, which is the fail-closed replacement for `tryRelaxCycle` | EXISTS. Not run in this documentation pass |
| `TestTopologicalSortNonAddressCycleFails` | `internal/component/config/transaction/solver_test.go` | Preserved rejection. The spec named it `TestTopologicalSortRejectsNonAddressCycle` until 2026-09-11; that name never existed in the tree | EXISTS. Not run in this documentation pass |
| `TestTopologicalSortOrdersASwapWhoseLabelsItDoesNotKnow` | `internal/component/config/transaction/solver_test.go` | AC-5 at the solver: the sort reads the verb and the resource kind, never the label | EXISTS. Not run in this documentation pass |
| `TestExecuteMixedRootTakesOperationPath` | `internal/component/config/transaction/orchestrator_test.go` | A transaction with one decomposed root and one uncovered participant runs `runOperationPath` (AC-1) | PASS |
| `TestExecuteCoarseNodeAppliesSection` | `internal/component/config/transaction/orchestrator_test.go` | The coarse node emits the participant's section apply event with the diffs `runApply` would have sent (A-1, AC-2) | PASS |
| `TestExecuteCoarseNodeEmitsNoOperationVerify` | `internal/component/config/transaction/orchestrator_test.go` | The coarse node owes no per-operation verify (A-2) | PASS |
| `TestParticipantsWithoutOperationsEmptyAfterSynthesis` | `internal/component/config/transaction/orchestrator_test.go` | Coverage is total, so no fallback branch is reachable | PASS |
| `TestExecuteRefusesOperationWithNoVerb` | `internal/component/config/transaction/orchestrator_test.go` | AC-6, fail closed with a named error | PASS, and observed RED with the guard deleted |
| `TestOperationPathCarriesUnknownLabel` | `internal/component/config/transaction/executor_test.go` | AC-5, the label reaches the owner unchanged | PASS, and observed RED with the executor comparing the label to a core list |
| `TestExecuteRollsBackMixedTransaction` | `internal/component/config/transaction/executor_test.go` | AC-7 across both node kinds | PASS |
| `TestIfaceOperationsDeclareProduceAndConsume` | `internal/component/iface/operation_test.go` | The iface decomposer declares the sets the deleted rules used to state | PASS, and observed RED with the declarations removed |
| `TestBGPOperationsDeclareConsumeAddress` | `internal/component/bgp/plugin/operation_test.go` | The bgp decomposer declares address consumption for its peer operations. It emits no listener operation, so no listener declaration exists to assert (corrected at review round 1, I-4) | PASS, and observed RED with the derivation removed |
| `TestSDKPluginWithoutOperationCallbacksAnswersUnknownMethod` | `pkg/plugin/sdk/sdk_test.go` | A-3, the reason a coarse node takes the section route | PASS |
| `TestTopologicalSortPlacesSectionNodeBetweenAddressingAndBinders` | `internal/component/config/transaction/solver_test.go` | B-1, R2-B-2. A coarse node runs after the addressing this commit adds and before the first operation that binds it, which is the phase 4 to phase 5 boundary. RENAMED at phase 5 from `TestTopologicalSortPlacesSectionNodeBetweenCreatesAndDestroys`, whose name stated the paraphrase's placement | EXISTS. Not run in this documentation pass |
| `TestTopologicalSortSectionNodeJoinsAnAddressSwapWithoutACycle` | `internal/component/config/transaction/solver_test.go` | R-2. A swap plus one uncovered root still sorts, which an edge-shaped position would not | PASS before and after. It fences the shape of the answer, not a change of behavior |
| `TestExecuteAppliesCoarseNodeAfterTheResourcesItBinds` | `internal/component/config/transaction/orchestrator_test.go` | B-1 through `Execute`, the door an operator reaches | PASS, and observed RED at `[iface-add-zx section-apply-static iface-add-address-zx]` |
| `TestReloadRefusesAPluginOperationCarryingTheReservedSectionApplyLabel` | `internal/component/plugin/server/reload_test.go` | B-2. `ReloadConfig` refuses a plugin-supplied `section-apply` label | PASS, and observed RED with the reserved-type refusal deleted |
| `TestReloadRefusesAPluginOperationDeclaringAResourceWithNoIdentity` | `internal/component/plugin/server/reload_test.go` | B-2. `ReloadConfig` refuses a blank resource entry, from the door rather than the helper | PASS, and observed RED with `validateResourceRefs` unwired |
| `TestApplyDeadlineSumsEveryParticipantBudget` | `internal/component/config/transaction/orchestrator_test.go` | I-1. The apply and verify deadlines cover every participant in sequence | PASS, and observed RED at `apply deadline = 10s, want 20s`. Replaces `TestOrchestratorDependencyGraphDeadline` |
| `TestApplyDeadlineDefaultsWhenNoParticipantDeclaresABudget` | `internal/component/config/transaction/orchestrator_test.go` | I-1. A participant set that declares no budget takes the 30-second default rather than a zero deadline | PASS. It keeps the half of `TestOrchestratorTieredDeadlineCycleFallback` that survives the tier computation's deletion |
| `TestExecuteRefusesAParticipantCoveredForOneRootAndNotAnother` | `internal/component/config/transaction/orchestrator_test.go` | I-2. A participant that decomposes one root and leaves another aborts the transaction, named | PASS, and observed RED at `state = committed (err <nil>), want aborted` |
| `TestExecuteRollsBackAnAppliedCoarseNode` | `internal/component/config/transaction/orchestrator_test.go` | AC-7, I-3. An APPLIED coarse node's participant receives the section rollback | PASS, and observed RED with `publishRollback` deleted from the ordered path |
| `TestApplyConfigOperationModifyPeerSwapsInPlaceAndKeepsTheSession` | `internal/component/bgp/reactor/operation_test.go` | I-6. A modify-peer the running session can take keeps the peer, and the rollback keeps it too | PASS, and observed RED with the swap forced to false |
| `TestBGPStopsThePeerBoundToAnAddressThatChangesInterface` | `internal/component/bgp/plugin/operation_disturbed_test.go` | Phases 2 and 5, AC-4. An address that changes interface makes the bgp decomposer emit a remove-peer and an add-peer for the peer bound to it | EXISTS. Not run in this documentation pass |
| `TestBGPLeavesThePeerAloneWhenTheAddressRowIsIntact` | `internal/component/bgp/plugin/operation_disturbed_test.go` | AC-9. A change that leaves the address row intact disturbs nothing, so no peer is stopped | EXISTS. Not run in this documentation pass |
| `TestBGPStopsThePeerWhoseSourceAddressTheKernelPicks` | `internal/component/bgp/plugin/operation_disturbed_test.go` | The fail-safe default. A peer whose `connection.local.ip` is absent or `auto` binds an address Ze did not choose, so it is stopped as soon as any address is disturbed | EXISTS. Not run in this documentation pass |
| `TestReloadStopsABinderWhoseAddressMovesAndItsOwnConfigDidNot` | `internal/component/plugin/server/reload_disturbed_test.go` | Phase 2 through the planner: a root with no diff of its own receives the disturbed set and joins the transaction | EXISTS. Not run in this documentation pass |
| `TestReloadAsksNoUnchangedRootWhenNoAddressMoves` | `internal/component/plugin/server/reload_disturbed_test.go` | A commit that disturbs nothing makes one decompose pass and costs an unchanged root nothing | EXISTS. Not run in this documentation pass |
| `TestReloadStopsABinderWhenTheCommitAlsoEditsAnMTU` | `internal/component/plugin/server/reload_iface_mixed_test.go` | AC-8 through the planner, over the real iface decomposer | EXISTS. Not run in this documentation pass |
| `TestReloadMovesTheAddressWithNoOtherInterfaceChange` | `internal/component/plugin/server/reload_iface_mixed_test.go` | The same path with no MTU beside the move, so the MTU is not what produces the stop | EXISTS. Not run in this documentation pass |
| `TestReloadDisturbsNothingWhenOnlyTheMTUChanges` | `internal/component/plugin/server/reload_iface_mixed_test.go` | AC-9 through the planner: an MTU edit alone emits no address operation and stops no binder | EXISTS. Not run in this documentation pass |
| `TestIfaceOperationDecomposerMixedDiffStillMovesTheAddress` | `internal/component/iface/operation_test.go` | AC-8 at the decomposer. A diff carrying an MTU and a move emits the address destroy, the address create and one `configure-interfaces`, and `DisturbedAddresses` reads the address out of it | EXISTS. Not run in this documentation pass |
| `TestIfaceOperationDecomposerUnsupportedTypeRidesTheConfigureOperation` | `internal/component/iface/operation_test.go` | An interface type this package has no create primitive for rides the configure operation instead of taking the whole root's address operations down | EXISTS. Not run in this documentation pass |
| `TestIfaceOperationDecomposerBackendChangeRidesTheConfigureOperation` | `internal/component/iface/operation_test.go` | The same, for a key the decomposer has no primitive for | EXISTS. Not run in this documentation pass |
| `TestReloadTxAppliesCoarseSectionsBeforeBinderStarts` | `internal/component/plugin/server/reload_test.go` | R2-B-2 end to end. The `bgp` root's sibling sections apply before `bgp`'s own peer create, with the decomposing participant registered FIRST so neither a name check nor the slice order can produce the answer | EXISTS. Not run in this documentation pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Operations in one transaction | 0 to unbounded | The design names a 10000 warning the builder does not enforce. This spec adds no limit | N/A | N/A |
| Cycle members | N-A since phase 5. No cycle is relaxed, so no member count is read. A three-way rotation now sorts without one (`TestTopologicalSortRotatesAddressesBreakBeforeMake`) | N/A | 1, a self edge, rejected as a cycle | N/A |
| Coarse nodes per transaction | 0 to one per participant | One per uncovered participant | N/A | N/A |

This spec introduces no new numeric input. The table records the existing
bounds so implementation does not invent one.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-apply-ordering-mixed-root` | `test/reload/config-apply-ordering-mixed-root.ci` | An interface address plus a static route in one commit keeps address ordering (AC-1) | PASS in the QEMU guest (925ms) under the owner's order, and observed RED under each of two reverts. RED 1, the constraint rule `iface-remove-address-before-add-address` unregistered (`internal/component/iface/operation.go`): `ZE-OBSERVER-FAIL: the new address 10.93.0.1/24 arrived before the old one 10.92.0.1/24 was removed`. RED 2, `sectionNodePosition` made to answer 0 so the coarse node runs at the head (`internal/component/config/transaction/solver.go`): the static section applies before its gateway's prefix exists, the kernel refuses the route, and the driver reports `ZE-OBSERVER-FAIL: the renumber and the static route did not both land` over an event stream carrying the renumber and no route. GREEN: 11 of 11 steps pass. Log: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/dw/walk-1.log` |
| `config-apply-ordering-coarse-root` | `test/reload/config-apply-ordering-coarse-root.ci` | A commit touching only a root with no decomposer still applies (AC-2) | PASS, and observed RED under its own revert |
| `config-apply-ordering-address-swap` | `test/reload/config-apply-ordering-address-swap.ci` | Two interfaces swap addresses, each address leaves its old interface before it arrives on the new one, and the peer bound to one of them is stopped and started around the move (AC-4) | PASS (3.2s, 11 of 11 steps) in the QEMU guest on 2026-09-11, and RED under one revert for EACH half. Phases 2 and 5: restoring the pre-`284620ac2` early return in `decomposeBGPOperations` stops the peer operations being emitted, and the driver reports `ZE-OBSERVER-FAIL: the check peer never wrote session-returned.txt: the session bound to 10.90.0.1/24 was not stopped and started around the move, so phases 2 and 5 did not happen`. The peer log carries the mechanism: it held its connection open, and the close that follows is the teardown 20 seconds later. Phases 3 and 4: unregistering `iface-remove-address-before-add-address` gives `ZE-OBSERVER-FAIL: interface zdual0 held both addresses after add address 10.91.0.1/24 dev zdual0: the move was made before it was broken`. What made the file able to pass at all is three repairs, none of them a weakened assertion: the check peer now holds its session open across the reload (`option=linger`, `endSequence` in `internal/test/peer/reject.go`), so a second connection can only follow a daemon-side drop; the driver waits for the peer's `session-returned.txt` marker before it signals, so the reload finishes and prints `sighup reload complete`; and `action=rewrite` answers completion like the other action arms, so the daemon's shutdown NOTIFICATION is no longer matched against an empty expectation list. Logs: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/sw/walk-2.log` (the walk), `.../sw/walk-1.log` (the same walk one fix earlier, whose peer output shows the drop and the return in order), one-tree evidence in the `sw-pairbuild` and `sw-rebuild` job logs beside them |
| `config-apply-ordering-mixed-rollback` | `test/reload/config-apply-ordering-mixed-rollback.ci` | A failed apply in a mixed transaction rolls back both node kinds (AC-7) | PASS (8.9s), and observed RED under its own revert. The tree that reddened 36 of the 63 reload tests healed; the whole suite is now 44 pass, 19 skip. Its connection-2 assertion was strengthened after the first run: it asserted the End-of-RIB alone, which passes while the restored peer announces nothing, so it now asserts the `192.168.1.0/24` UPDATE as well. The revert that reddens it is rebuilding the peer from the operation's config subtree instead of from the running peer (`runningPeerSettings`, `internal/component/bgp/reactor/operation.go`); under it the peer exchange fails on a message mismatch and every other reload test stays green. Logs: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/functional-reload-{1,red-revert}.log` |

`config-apply-ordering-address-swap` and `config-apply-ordering-mixed-root`
carry `option=needs-linux:caps=net-admin`, matching
`config-apply-ordering-create.ci`: they assign addresses to real devices and
read them back.

**Both were walked on 2026-09-11 under the owner's order, and both pass.**
`mixed-root` discriminates and passes. `address-swap` discriminates on phases 3
and 4 and, after the repairs in its row above, on phases 2 and 5 as well. They
skip on a darwin host, so the route that ran them is QEMU: `./le qemu netns-test` does not cover
the reload suite (its selector is `firewall,policy,ospf,ospfv3,pppoe`,
`internal/le/qemu/actions.go`), so the route is `./le qemu run kernel
tmp/kernel/build/vmlinuz packages "iproute2 libcap"` carrying a guest script
that runs `ze-test bgp reload -p 1 <test>` against each daemon in turn, per
`docs/architecture/testing/qemu-integration.md`. The guest booted Ze's runtime
kernel 7.2 and ran as root, which is what supplies `caps=net-admin`.

Two things the walk had to establish, and both cost a run:

- **One tree produced every binary.** This checkout is shared and its Go source
  changed inside a 40-second window while the walk was being prepared, so a
  green built before a red and a red built after it are not a pair. The four
  daemons were therefore built in one script that builds green, both reverts,
  and green again, and compares the two greens byte for byte. They are
  identical, and so are the two builds of the harness, so nothing that reaches
  a binary moved while the reverts were compiled
  (`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/dw/pairbuild-4.log`).
  The green daemon carried md5 `315e5ca496b701bc01cbeed3de814448` in every one
  of the four pairbuilds this walk ran, which is the same fact measured four
  times.
- **The guest has ONE routing table.** The first walk ran the greens first, and
  each left its dummy devices, addresses and static routes behind. The
  mixed-root RED then failed with `add route failed ... error="file exists"`,
  which is the fixture's environment and not the revert, so that red was
  discarded. The walk now deletes `zdual0`, `zdual1`, `zmix0` and both static
  prefixes before each run and prints the table it starts from.

**Discrimination walk (`ai/rules/interop-and-goal-validation.md`, required, not
assumed):** for each of the four tests, revert the change it fences, rebuild the
daemon so the revert takes effect, run the test, record the RED output in the
closure section, restore the fix, and record GREEN. The reverts are: for `mixed-root`,
unregister `iface-remove-address-before-add-address` and, separately, make
`sectionNodePosition` answer 0; for `coarse-root`, delete the coarse node
synthesis; for `address-swap`, unregister
`iface-remove-address-before-add-address`; for `mixed-rollback`, skip the
section rollback for coarse nodes. A test that does not go RED under its own revert has not been
shown to discriminate and is not evidence.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `tx-protocol-external-plugin` | `test/reload/tx-protocol-external-plugin.ci` (existing, extended) | A Ze external plugin over the plugin hub transport | The ABI change reaches a plugin in a separate process: it takes part in an ordered transaction, and its coarse node is applied through `config-apply` | EXTENDED, RUN, PASS (4.8s) in the same suite run that greened `mixed-rollback`; it was RED for the tree's reason above until that tree healed. The test asserted only absences (exit 0, four rejected timeouts), which passes against a daemon that applies nothing. It now carries a second process, `tx-protocol-external-plugin-observer` (`internal/test/fixture/register_tx_protocol_external_plugin.go`), declaring the `bgp` root and decomposing nothing, and two positive assertions: the observer's own line from `OnConfigApply`, and `bgp config operation journal committed`, which only `OnConfigOperationCommit` writes and therefore only the operation path produces |

No third-party daemon speaks this protocol: the config transaction ABI runs
between the Ze engine and a plugin process. The external-plugin test is the
cross-process peer this spec owes.

## Files to Modify
- `internal/component/config/transaction/operation.go` - add the verb type and the produce/consume declaration to the registry surface, delete the two `ResourceRelation` values that state produce/consume facts, delete the seven unused operation constant aliases. Add `DisturbedAddresses`, which reads the planned operations and returns every address a destroy produces, and `OperationDecomposerRoots`, which names the roots the planner asks on its second pass. Delete the relations `same-interface` and `same-address` with make-before-break. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/config/transaction/depgraph.go` - derive edges from produce and consume sets over resource identity, beside the surviving rule-driven edges. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/config/transaction/solver.go` - delete `tryRelaxCycle`, `markDualPresence`, `isAddressOperation` and `opInterface` with make-before-break, so `TopologicalSort` answers `ErrOperationCycle` for every graph it cannot order. `placeSectionNodes` and `sectionNodePosition` put a coarse node at the phase 4 to phase 5 boundary, reading the verb and `providesAddressing` and no operation label. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/config/transaction/executor.go` - route a coarse root node to the section apply event and the section rollback, keep the per-operation route for everything else. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/config/transaction/orchestrator.go` - synthesize one coarse node per uncovered participant, delete the section-apply fallback and its log line, keep `participantsWithoutOperations` as the synthesis input. Doc: `docs/architecture/config/transaction-protocol.md`
- `internal/component/config/transaction/orchestrator_budget.go` - NEW at review round 2. The deadline and budget concern, moved out of `orchestrator.go` when the file crossed 1000 lines. The deadline is the sum of the participants' budgets. Doc: `docs/architecture/config/transaction-protocol.md`
- `internal/component/bgp/reactor/operation.go` - the modify-peer operation asks `peerSettingsSwapPlan` before it removes and re-adds, which is the decision the section apply already took, and a rolled-back peer returns as the reactor had it (`runningPeerSettings`). Named here at review round 2 (I-6): the operation path went live in this spec, so what it reaches is this spec's
- `internal/component/plugin/server/reload_tx.go` - the planner refuses an operation with no verb, and reports an uncovered root to the orchestrator rather than making it invisible. It also runs the second decompose pass: `bindingRoots` names the roots to ask, `DecomposeRequest.DisturbedAddresses` carries the set, and `checkDisturbanceSettled` refuses a plan whose second pass disturbs a different set from its first. `sortParticipantsBGPLast` and `bgpParticipantName` are deleted. Doc: `docs/architecture/config/transaction-protocol.md`
- `internal/component/plugin/server/reload.go` - `appendDecomposingPlugins` joins every running plugin that declares a decomposition to the transaction with no section, so a binder with no diff can still be asked. `filterDiffs` then gives it no verify event, no section apply and no coarse node, so a commit that disturbs nothing costs it nothing. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/plugin/server/config_tx_bridge.go` - dispatch a coarse root node through `SendConfigApply`. Doc: `docs/architecture/config/transaction-protocol.md`
- `internal/component/iface/operation.go` - declare produce and consume on each emitted operation, delete the five produce/consume constraint rules and the make-before-break rule `iface-add-address-before-remove-same-interface`, widen `iface-remove-address-before-add-same-address` into `iface-remove-address-before-add-address`, move the four operation labels into this package. `decomposeIfaceOperations` now covers the whole root on every call: an address and an interface of a type it can create become one operation each, and every other key rides `ifaceConfigureOperation`, which four new placement rules order after every operation that moves an address or an interface. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/bgp/plugin/operation.go` - declare consumption of the local address on peer and listener operations, delete all four constraint rules, move the peer and listener labels out of the shared ABI. `decomposeBGPOperations` also emits a remove-peer and an add-peer for every peer whose local address is in the disturbed set (`peerBindingDisturbed`), and `peerAddressConsumes` makes those two operations declare every disturbed address so the derived edges still place them. That is phases 2 and 5, for BGP.
  → Decision (phase 2): the labels went to a new leaf, `internal/core/bgp/configop`, not into this package. The `bgp` root emits its operations here and APPLIES them in `internal/component/bgp/reactor`, and neither side can import the other: the reactor importing `bgp/plugin` pulls a plugin's registration into the engine, and `bgp/plugin` importing the reactor closes a cycle, because `events_import_test.go` is `package reactor` and blank-imports `bgp/plugin`. A leaf both import is the remaining option, and `internal/core/bgp/events` is the precedent. The property the spec wanted holds: no shared package carries a per-root label list. Doc: `docs/architecture/config/apply-ordering.md`
- `pkg/plugin/rpc/types.go` - add the verb and the produce/consume fields to `ConfigOperation`, delete the seven unused operation constants and the constants that move into iface and bgp, keep `ConfigOperationType` as the free-text label type. Docs: `docs/architecture/api/ipc_protocol.md`, `docs/plugin-development/protocol.md`
- `pkg/plugin/sdk/sdk_types.go` - re-export what survives, drop the aliases of the deleted constants. Doc: `docs/architecture/api/process-protocol.md`
- `docs/architecture/config/apply-ordering.md` - the sentences named in the Documentation Update Checklist
- `docs/architecture/config/transaction-protocol.md` - the operation type table, the constraint rule table and the decomposer table
- `docs/architecture/api/process-protocol.md` - the operation callback payload shape
- `docs/plugin-development/protocol.md` - the `config-operations` registration field and the callback table
- `test/reload/config-apply-ordering-rotation.ci`, `-swap.ci`, `-reip.ci` - correct the comments describing address operations these tests do not emit, and point them at the new address-swap test

## Files to Create
- `test/reload/config-apply-ordering-mixed-root.ci` - AC-1
- `test/reload/config-apply-ordering-coarse-root.ci` - AC-2
- `test/reload/config-apply-ordering-address-swap.ci` - AC-4, closes D4
- `test/reload/config-apply-ordering-mixed-rollback.ci` - AC-7

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config leaf changes. The ordering is derived from the diff, never declared by the operator |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | No | No command surface changes. The CLI editor commit path reaches the same `reloadConfig` |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | Yes | The plugin ABI changes shape: `test/reload/tx-protocol-external-plugin.ci` extended, plus the four new `.ci` tests above |
| Pipe completeness | N-A | No command output added |
| Env var registration | No | No env var |
| Doctor check for runtime dependencies | No | No new file path, socket, service, module, port or certificate. The change is internal to an existing path |
| Prometheus counters/metrics | No | None added. The deleted `Info` fallback log line is the only observability surface removed, and it goes because its branch goes |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP wire surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Ordering already exists and is documented. This spec makes it reach every root, which is a correctness change, not a new feature |
| 2 | Config syntax changed? | No | No config leaf added, removed or renamed |
| 3 | CLI command added/changed? | No | No command surface |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`: the `config-operation-*` payload gains the verb and the produce/consume sets, and a coarse root node routes through `config-apply` |
| 5 | Plugin added/changed? | No | No plugin added or removed |
| 6 | Has a user guide page? | No | `docs/guide/config-reload.md` anchors `orchestrator.go` but makes no claim about ordering, coverage or the section apply (grep for "order" and "section apply" returns nothing), so it is unaffected. The operation graph has no operator-visible knob of its own |
| 7 | Wire format changed? | No | No protocol wire format. The plugin RPC payload is covered by rows 4 and 8 |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md` (the `config-operations` field and the operation callback table), and `ai/rules/plugins.md` if it restates the registration field list |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC governs this path |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md`, two rows, both UPDATED on 2026-09-11. `option=linger` now also holds a connection BETWEEN connections, until the remote closes it, which is what lets a `.ci` assert that the DAEMON stopped and restarted a session (`endSequence`, `internal/test/peer/reject.go`); a `conn_map` peer is excluded and the row says why. And `action=rewrite` answers completion when it is the last item a peer block queues, as `action=sighup` and `action=sigterm` already did. No new option and no new tool |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about config transaction ordering |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/apply-ordering.md` and `docs/architecture/config/transaction-protocol.md`. See the sentence table below |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The stream event set is unchanged, but `docs/features/plugins.md` and `docs/plugin-overview.md` were checked on 2026-09-09: neither names an operation label, `ConfigOperationType`, or a `config-operation-*` callback, so neither carries a copy and neither is edited |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED. `./le spec citation anchors spec plan/immediate/spec-config-apply-ordering-covers-every-root.md` printed the result recorded under "Citation anchors" below |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/config/transaction-protocol.md` shows an operation table with `add-address` and `add-peer` rows, and a worked swap example beside its rule table. Both must be rewritten against the verb model |

**Sentences in `docs/architecture/config/apply-ordering.md` that stop being true:**

| Sentence | Why it stops being true |
|----------|------------------------|
| "`iface` and `bgp` each register their own decomposition through `init()`." | Still true, and no longer the whole story. After this spec every root with a diff is represented, decomposer or not, so the paragraph must say what a root without a decomposer gets |
| "Address-only cross-interface cycles relax. Everything else is rejected." | DELETED at phase 5, not rewritten. The relaxation existed to break a cycle make-before-break created, and that rule is gone, so the page now says "Every cycle is rejected" |
| "The external contract is mandatory for v1 plugins: SDK types in `pkg/plugin/sdk/sdk_types.go` ..." | The contract gains the verb and the produce/consume sets, and a payload without a verb is refused. The sentence must state the refusal |
| The whole first paragraph of "What the tests do not reach": "they exercise the whole decompose to graph to executor to bridge to RPC to reactor path and none of the `AllowDual` machinery" | `config-apply-ordering-address-swap.ci` reaches it. The paragraph is replaced by what that test proves and by what remains unreached |
| "The current decomposers never emit an `ADD_ADDRESS` for an address that is already present." | Rewritten at phase 5 against the iface decomposer as it now is: it skips an address the active config already gives to the SAME interface with the same prefix length, and skips nothing else |
| The page gains a section it did not have: "The requirement", carrying the owner's words verbatim | The paraphrase in this spec's Task section is what made the subsystem wrong. The page is now the authority for the order, and this spec points at it rather than restating it |
| The page gains "What is not built" | Phases 2 and 5 reach BGP only, the wildcard carve-out is verified for BGP only, and VRF is a constraint with no code. A reader who stops at "The design" would otherwise read the requirement as shipped |

`docs/architecture/config/transaction-protocol.md` loses its operation type
table (the 21 constants), nine rows of its constraint rule table (all deleted
rules), and the decomposer table's claim that iface and bgp are the two
decomposers.

**Citation anchors (row 16, recorded output):**

`./le spec citation anchors spec plan/immediate/spec-config-apply-ordering-covers-every-root.md`
printed no blocking result. Every page a changed file DECLARES through its
`// Design:` header is already named in Files to Modify, so nothing blocks. It
reported 15 pages that only `<!-- source: -->` mention a changed file, which is
advisory:

| Page | Mentioned by | Verdict |
|------|--------------|---------|
| `docs/architecture/api/architecture.md` | `pkg/plugin/rpc/types.go` | Answered (phase 2): no copy. Its only "operation" lines are route operations and gNMI Set. Unedited |
| `docs/architecture/api/commands.md` | `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go` | Answered (phase 2): no copy. Its "operation" lines are route operations and the report bus. Unedited |
| `docs/architecture/api/wire-format.md` | `pkg/plugin/rpc/types.go` | Unaffected: BGP wire format, not the plugin RPC payload |
| `docs/architecture/core-design.md` | `internal/component/config/transaction/orchestrator.go` | Check whether it states the section apply is the reload apply path |
| `docs/architecture/plugin-manager-wiring.md` | `pkg/plugin/rpc/types.go` | Answered (phase 2): no copy. The word "operation" does not appear. Unedited |
| `docs/architecture/update-cache.md` | `pkg/plugin/rpc/types.go` | Unaffected: BGP update caching |
| `docs/features.md` | `pkg/plugin/rpc/types.go` | Unaffected: no config ordering claim |
| `docs/functional-tests.md` | `pkg/plugin/rpc/types.go` | Check whether it lists the reload ordering tests by name |
| `docs/guide/config-reload.md` | `internal/component/config/transaction/orchestrator.go` | Unaffected: no ordering, coverage or section apply claim |
| `docs/guide/plugins.md` | `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go` | Answered (phase 2): no copy. Its "operation" lines describe the iface plugin and filter-modify. Unedited |
| `docs/plugin-development/README.md` | `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go` | Answered (phase 2): no copy. The word "operation" does not appear. Unedited |
| `docs/plugin-development/commands.md` | `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go` | Unaffected: command dispatch, not config transactions |
| `docs/plugin-development/handlers.md` | `pkg/plugin/rpc/types.go` | Answered (phase 2): it documents no `OnConfigOperation*` handler and the word "operation" does not appear. The callback table that does is `docs/plugin-development/protocol.md`, which this phase edited. Unedited |
| `docs/plugin-development/schema.md` | `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go` | Unaffected: YANG schema registration |
| `docs/plugin-development/testing.md` | `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go` | Answered (phase 2): its example plugin registers no operation callback and the word "operation" does not appear. Unedited |

Each "check" row is answered during phase 5 and the answer is recorded there.
This spec does not answer them from memory.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the coarse node reachable, and prove a mixed-root transaction takes the operation path
   - Tests: `TestExecuteMixedRootTakesOperationPath`, `TestExecuteCoarseNodeAppliesSection`, `TestParticipantsWithoutOperationsEmptyAfterSynthesis`, `test/reload/config-apply-ordering-coarse-root.ci`
   - Files: `orchestrator.go` (synthesis, and the deleted fallback), `executor.go` (section route for a coarse node), `config_tx_bridge.go` (dispatch to `SendConfigApply`)
   - Verify: the wiring tests fail first because the coarse node does not exist, then pass. The fallback branch and its log line are gone, so no test can reach them
2. **Phase: Vocabulary** -- the verb plus the resource replaces the operation type enum
   - Tests: `TestExecuteRefusesOperationWithNoVerb`, `TestOperationPathCarriesUnknownLabel`, `TestSDKPluginWithoutOperationCallbacksAnswersUnknownMethod`, `TestTopologicalSortOrdersASwapWhoseLabelsItDoesNotKnow`
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go`, `internal/component/config/transaction/operation.go`, `solver.go`, `reload_tx.go`, and the labels moving into `internal/component/iface/operation.go` and `internal/component/bgp/plugin/operation.go`
   - Verify: no core package compares an operation label to a constant, the seven unused constants are deleted, and a missing verb aborts with a named error
3. **Phase: Derived edges** -- produce and consume replace the nine hand-written rules
   - Tests: the five `depgraph_test.go` derivation tests, `TestIfaceOperationsDeclareProduceAndConsume`, `TestBGPOperationsDeclareConsumeAddress`
   - Files: `depgraph.go`, `operation.go`, `internal/component/iface/operation.go`, `internal/component/bgp/plugin/operation.go`
   - Verify: the nine rules are deleted, the two iface rules that are not produce/consume facts survive, and the derived edge set equals the deleted rules' edge set on the existing fixtures
4. **Phase: Proof on the real path** -- close D4
   - Tests: `config-apply-ordering-address-swap.ci`, `config-apply-ordering-mixed-root.ci`, `config-apply-ordering-mixed-rollback.ci`, and the discrimination walk for all four new tests
   - Files: the four new `.ci` files, and the comment corrections in the three existing ordering tests
   - Verify: each new test is observed RED under its own named revert and GREEN after restore, with both outputs recorded
   - Observed at phase 1, and owed an answer here: `config-apply-ordering-rotation` timed out once in five `./le functional reload` runs over the phase-1 tree, at 30.0s against a 2.2s average, reporting "all expected messages received but test still timed out". It is one of the three tests D4 says emit `remove-peer` and `add-peer` rather than the address operations they stand in for, and this phase rewrites them. Reproduce it before rewriting, so the rewrite is known to remove the timeout rather than to hide it
     - Answer (phase 4): NOT REPRODUCED, in 3 completed runs. Six `./le functional reload` runs were started before the rotation comment was touched; four never reached a test, because another session's uncommitted work left the tree unable to build the isolated binaries. `config-apply-ordering-rotation` passed at 7.2s and 4.0s in the two that ran, and at 4.0s again in the one later phase-4 run that completed. Both are above the suite's per-test average of 2.8s and neither is near the 30s deadline. Phase 4 changed only that test's comment header, so the timeout is neither removed nor hidden by this phase: two runs are too few to call it gone, and the next session that gets a buildable tree should repeat it. Logs: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/rotation-repro-{1..6}.log`
5. **Phase: Documentation** -- the pages named in the checklist, written inside this same work
   - Files: `docs/architecture/config/apply-ordering.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`
   - Verify: `./le doc check verify` passes, and every anchor named in row 16 is either updated or named as unaffected with a reason

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an implementation at file:line, and the coarse node path has one at both the orchestrator and the bridge |
| Feature completeness | Each of the four user stories runs end to end in a `.ci` test, not only in a unit test over the graph |
| Correctness | The derived edges reproduce the exact edge set the nine deleted rules produced on the existing iface and bgp fixtures. An edge the old rules produced and the derivation does not is a regression, not a simplification |
| Correctness | The coarse node applies exactly what `runApply` applied for the same participant and the same diffs (A-1) |
| Naming | New JSON keys are kebab-case: `verb`, `produces`, `consumes` |
| Naming | The verb values are `create`, `destroy` and `modify` in every surface: Go constant, JSON value, doc table |
| Data flow | The solver names no ROOT, no PARTICIPANT and no operation LABEL. It reads the verb and the resource kind, so `ResourceAddress` and `ResourceInterface` are expected in `providesAddressing` and are what the phase line is drawn with. Grep it for a root name (`bgp`), a participant name (`rib`, `static`) and an operation label (`add-peer`, `configure-interfaces`) after the change |
| Correctness | Every sentence in this spec agrees with `docs/architecture/config/apply-ordering.md`, "The requirement". Where it does not, the spec is wrong. This spec was written from a paraphrase, so a sentence that reads correct from inside the spec is exactly the failure mode |
| Rule: `ai/rules/no-layering.md` | The fallback branch, the nine rules, the seven unused constants and the two produce/consume `ResourceRelation` values are DELETED, not left unreachable. So is make-before-break: `Params.AllowDual`, `markDualPresence`, `tryRelaxCycle`, `isAddressOperation`, `opInterface`, `sortParticipantsBGPLast` and `iface-add-address-before-remove-same-interface` |
| Rule: `ai/rules/principles.md` | No operation is ordered on a default. A missing verb aborts, and never becomes `modify` |
| Rule: `ai/rules/stale-comments.md` | The comment in `Execute` justifying the fallback, and the comment on `participantsWithoutOperations` describing the all-or-nothing decomposer contract, both describe deleted behavior |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The section-apply fallback is gone | `grep -n "using section apply" internal/component/config/transaction/orchestrator.go` returns nothing |
| The seven unused operation constants are gone | `grep -rn "OperationStartDHCP" --include=*.go .` and the same for the other six return nothing |
| The nine produce/consume constraint rules are gone | `grep -rn "RegisterConstraintRule" --include=*.go internal/` shows two call sites, both in `internal/component/iface/operation.go`. One registers `iface-remove-address-before-add-address`; the other is a loop over four `*-before-configure` placements |
| The solver reads no operation label | `grep -n "OperationAdd" internal/component/config/transaction/solver.go` and the same for `OperationRemove` return nothing |
| Make-before-break is gone, not left unreachable | `grep -rn "AllowDual\|markDualPresence\|tryRelaxCycle\|sortParticipantsBGPLast" --include="*.go" internal pkg` returns only comments recording the deletion |
| Phase 2 reaches a binder with no diff of its own | `TestReloadStopsABinderWhoseAddressMovesAndItsOwnConfigDidNot` passes, and `test/reload/config-apply-ordering-address-swap.ci` reddens when the pre-`284620ac2` early return in `decomposeBGPOperations` is restored |
| Coverage is total | `TestParticipantsWithoutOperationsEmptyAfterSynthesis` passes |
| The four new functional tests exist and pass | `./le test functional filter config-apply-ordering` |
| Each new functional test discriminates | The recorded RED and GREEN output pair for each of the four, in the closure section |
| Docs match the code | `./le doc check verify` names none of the four pages this spec edits. It FAILS tree-wide on this checkout and every cause belongs to another session: 6 stale source anchors, all into `internal/component/bgp/reactor/session_bfd_strict.go`; two `request bgp rib` rows in the generated command-equivalents surface; 7 RPC long-help rules over `ze-bgp-cmd-*`, `ze-cli-set-api` and `ze-rib-api`; one `ze-policyroute-conf` summary over its character cap. Log: `.../scratch/cao2-doc-check-verify.log`. `ai/DOCS-TO-CODE.md` is stale tree-wide and one of this spec's files, `orchestrator_budget.go`, is new since the last regeneration |
| The gate is green on a committed tree | `./le verify worktree` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | An operation arrives from an external plugin process over JSON, so the verb, the target kind and the produce and consume entries are attacker-influenced when a plugin is hostile. Each is validated at planning: an unknown verb is refused, and a produce or consume entry with an empty identity is refused rather than matching everything |
| Resource exhaustion | Derived edges are quadratic in the number of operations sharing one resource identity. A plugin returning many operations that all consume one resource multiplies edges. Measure the graph build on the largest realistic config and record the number |
| Error leakage | The abort message names the plugin, the root and the operation id. It must not carry the operation params, which can hold config values such as keys |
| Authorization that could fail open | The declaration check is the guard that an owner declared what it emits. Extending the operation shape must not let an undeclared operation through: drive its test from the planner entry point, never from the helper alone (`ai/rules/evidence.md`) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Derived edge set differs from the deleted rules' edge set | DESIGN. Either the derivation is wrong, or a rule stated more than produce/consume |
| A previously green reload test now aborts with a cycle | DESIGN. R-2. Do not relax the cycle check to make it pass |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A new functional test will not go RED under its revert | The test does not discriminate. Rewrite the assertion, and do not record it as proof |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The graph was never the problem. Reach was. Two components declared ordering, and one condition in `Execute` threw that ordering away whenever a third root joined the transaction.
- The comment defending the fallback is right about its tradeoff and wrong about its premise. It reasons that the operation path cannot reach an uncovered participant, which is true only while no node represents that participant.
- A vocabulary that must be edited centrally to be extended predicts its own disuse. Seven of the 21 operation kinds were written for roots that never arrived, which is the measurement of that prediction.
- The derivation reproduced the nine deleted rules' edge set on the iface and bgp fixtures, and added exactly two edges, both of which a deleted rule's ID promised and its body never produced. `iface-remove-address-before-interface` related its pair through the interface an ADDRESS names (`opAddrIface`, which reads `Target.Interface`), and an interface operation carries its name in `Target.Name`, so that rule produced NO edge on any operation the iface decomposer emits: the interface delete had no ordering against its own address removal. `bgp-add-address-before-peer` selected the `add-peer` label, so a `modify-peer` binding the same address got no edge while its address moved interface. The measured pre-deletion edge set is recorded in `TestBuildOperationGraphDerivedEdgesMatchDeletedRules`.
- A rule that names another root's operation labels is the shape of the defect above. Both dead rules were dead in a way no test could see, because a rule that matches nothing and a rule that is correct look identical from outside the graph.
- `AllowDual` was written and never read outside the solver. The window came from the edges `tryRelaxCycle` removed, so the flag labelled a result rather than instructing the applier. Phase 5 deleted all of it with the policy it served.
- A paraphrase of a requirement is a second declaration of it, and it drifts like any other copy. The owner gave this order in May 2026, a spec kept a summary of it, the words were lost, and the subsystem was built from the summary. The repair is to QUOTE him on one page and point every other artifact at that page.
- The defect could not be found from inside the code, because the code agreed with itself. Every test, every comment and both `.ci` files stated make-before-break, so the suite was as green under the wrong policy as under the right one. Only the owner's own sentence could tell them apart, and it was not in the tree.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A coarse root node is applied through the existing `config-apply` RPC | Route it through `config-operation-apply` like every other node | `config-operation-apply` has no SDK default handler, so a plugin that never registered one answers "unknown method" and the transaction aborts. The section route reaches every participant the section apply reaches today, which is the property the coarse node exists to keep |
| The coarse node owes no per-operation verify | Emit `config-operation-verify` for it, or invent a no-op verify | Phase 1 already verified the whole candidate config for that participant before the planner ran. A second verify is a second answer to a question already answered |
| The constraint rule registry survives, with only its produce/consume users deleted | Delete the registry and derive everything | `iface-remove-address-before-add-address` states a fact about two operations over DIFFERENT resources, which no pair of declarations can carry: every address the commit removes leaves the host before any address it adds arrives. That is phases 3 and 4. The four `*-before-configure` rules place an operation that owns no resource. So the registry keeps a real job and the derivation takes the rest |
| A missing verb aborts the transaction | Default to `modify` | A default orders the operation as if it had no dependencies, which is the silently wrong answer `ai/rules/principles.md` names. Pre-release, with no shipped external plugin, the refusal costs nobody |
| The label stays on the wire as free text | Delete it and dispatch on the verb plus the resource | The owner needs it: `applyIfaceOperation` and `applyBGPOperation` dispatch on it, and a log line reads it. The core stops reading it, which is the part that mattered |
| Operation labels move into the owning packages | Keep them in `pkg/plugin/rpc` as documentation | A label list in a shared package is a central enumeration again, and a new root would edit it because it looks like the place labels belong. The owning package already owns the dispatch |

## Known Limitations
- The 10000 operation warning and the 100 cycle-depth rejection named in the design stay unenforced. Config is operator supplied and bounded, so neither is a security boundary. Not this spec's work.
- The step-3 owner state check in `armSettlementWaiters` stays unimplemented. Total coverage does not change that path.
- This spec makes every root ORDERABLE. It does not make every root DECOMPOSED: firewall, dhcp, ike, l2tp, static and ntp still apply as one coarse node each. A coarse node is placed after the addressing the commit adds and before the first operation that binds it (`placeSectionNodes`, `sectionNodePosition`, `solver.go`), which is ordering against the operations the other roots emit and none within itself. Per-root decomposition is separate work, one spec per root, each a feature rather than a defect. None is written yet.
- **Phases 2 and 5 are out of reach for any binder nobody decomposes, which is every binder except BGP.** Both phases need one binder to take TWO steps in one commit, a stop before the addresses move and a start after. A participant with no decomposer gets one coarse node, and one node cannot be split in two. Two roots register a decomposer, `interface` and `bgp`, so ike, l2tp, dhcp, ntp, gnmi, tftp and every plugin listener get one lump each: started against the new addresses, never stopped before the old ones go. `docs/architecture/config/apply-ordering.md`, "What is not built", is the authority for this and it carries the same limit.
- **The wildcard refinement is verified for BGP only.** The owner's second refinement says a socket bound to `0.0.0.0` or `::` is not disturbed by an address that moves. Ze's BGP binds specific addresses: `CreateReactorFromTree` sets no listen address and `startMultiListeners` opens one listener per passive peer's local address, so the carve-out frees no BGP session today. Whether any other binder binds the wildcard is NOT established. `listenDHCP` binds `:67` and ties the socket to a device with `SO_BINDTODEVICE`, which is a wildcard address bind whose context is the device. Reading each remaining binder's own listen call is what settles the rest, and this spec did not do it.
- **VRF is a forward-looking constraint with nothing implemented.** The owner's third refinement says a change of VRF changes who can speak to a binder, with the address, the prefix length and the interface all unchanged. Ze has no VRF support, so there is no code here to be right or wrong. It is recorded on the page so that the disturbance list is known to be open rather than closed.
- A peer whose `connection.local.ip` is absent or `auto` binds an address Ze did not choose, so Ze cannot say whether a commit takes it away. The fail-safe default stops and starts it as soon as ANY address is disturbed (`peerAddressConsumes`). That costs the operator one session restart on a commit that moved an unrelated address.

## RFC Documentation (Scope: protocol)

Not applicable. Scope is `config`. No RFC governs the config transaction
ordering or the plugin RPC contract, so no requirement is quoted above
enforcing code.

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
- [ ] AC-1..AC-9 all demonstrated. AC-8 and AC-9 were added on 2026-09-11, when the spec was corrected against the owner's requirement: they state the disturbance test the subsystem now applies, which no earlier AC named
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Each of the four new functional tests observed RED under its own named revert and GREEN after restore, with both outputs recorded. ALL FOUR are walked. `coarse-root` and `mixed-rollback` were walked natively (logs in the Functional Tests table). `mixed-root` was walked in the QEMU guest on 2026-09-11 under the owner's order, red first under each of two reverts and green after: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/dw/walk-1.log`, one-tree evidence in `.../dw/pairbuild-4.log`. `address-swap` was walked in the same guest later that day, once its scaffolding could express the claim: red under the phase 2 and 5 revert, red under the phase 3 and 4 revert, green after restore, 11 of 11 steps. Log `.../sw/walk-2.log`, and its row in the Functional Tests table carries the failure text of each red
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `pkg/*`), not library-only
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

## Review Gate

Round 1, independent reader, 2026-09-11. Findings artifact:
`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/review-findings-cao.md`.
Verdict: **findings** -- 2 BLOCKER, 6 ISSUE, 3 NOTE. The gate is NOT clean.

| # | Severity | File / symbol | Finding |
|---|----------|---------------|---------|
| B-1 | BLOCKER | `internal/component/config/transaction/orchestrator.go` `operationNodes`, `depgraph.go` `addDerivedEdges`, `solver.go` `kahnSort` | A coarse node carries no verb, no target and no produce/consume set, and `RegisterConstraintRule` refuses an empty selector, so it can earn NO edge from either mechanism. Its position is the slice tie-break alone. Reproduced: `add-interface` + `add-address` + one coarse node sorts to add-interface, coarse node, add-address, so a static route installs before its address exists. AC-1 holds only for the renumber case the `.ci` picks, and `docs/architecture/config/apply-ordering.md` claims "a root nobody decomposes is ordered relative to the other roots", which is false |
| B-2 | BLOCKER | `internal/component/plugin/server/reload_tx.go` `validateOperationDeclarations` | The reserved `section-apply` label refusal has no test at any level. `ErrOperationBlankResource` is driven only from `ValidateOperations` itself, never from an entry point, which the spec's own Security Review Checklist forbids |
| I-1 | ISSUE | `internal/component/config/transaction/orchestrator.go` `computeTieredDeadline`, `runOperationPath` | The deadline is the per-tier MAX because a tier applied concurrently. The ordered path applies one node at a time under one absolute deadline, and every reload now takes it, so N participants cost up to N x budget. The comment is stale and R-1's "measure the reload suite before and after" is recorded nowhere |
| I-2 | ISSUE | `internal/component/config/transaction/orchestrator.go` `participantsWithoutOperations` | Coverage is per participant, so a participant that decomposes root A and has a diff on root B it does not decompose gets no coarse node and B reaches nothing while the transaction commits. Unreachable today (iface and bgp each declare one root) and unguarded |
| I-3 | ISSUE | `test/reload/config-apply-ordering-mixed-rollback.ci`, `executor_test.go` `TestExecuteRollsBackMixedTransaction` | No test rolls back an APPLIED coarse node. The `.ci` fails before the observer's node runs; the unit test asserts only that an applied coarse node receives NO per-operation rollback |
| I-4 | ISSUE | `test/weakened/4c26aef3.md`, this spec's TDD table | Both claim the bgp decomposer declares a LISTENER's address consumption. `internal/core/bgp/configop/configop.go` carries three peer labels and `decomposeBGPOperations` emits no listener operation |
| I-5 | ISSUE | `pkg/plugin/rpc/types.go`, `transaction/operation.go`, `depgraph.go` | `ResourceBridgeMember`, `ResourceSysctl`, `ResourceDHCP`, `ResourceTunnel` have no user outside their declaration and the two alias files, which is D3's shape. `ResourceRelationSameResource` has no rule. `resourceIdentity` and `resourceKey` keep listener and static-route branches no producer emits |
| I-6 | ISSUE | `internal/component/bgp/reactor/operation.go` `swapPeerForOperation` | New product behavior in a file Files to Modify does not name, covered by no AC, and by no test in which the swap returns true |
| N-1 | NOTE | `solver.go` `markDualPresence`, `pkg/plugin/rpc/types.go` `ConfigOperationParams.AllowDual` | Written and never read; dead weight on the plugin ABI |
| N-2 | NOTE | `orchestrator.go` `subscribeAcks` | Section-apply acks land in `o.applyOKCh` on the ordered path and nothing drains it, so a coarse node's budget refresh is lost |
| N-3 | NOTE | Deliverables Checklist | `./le doc check verify` does not pass on this tree. Every named cause is another session's work, so nothing here is this spec's to repair; the row is untrue as written |

**Round 1 disposition, 2026-09-11.** Every finding is dispositioned in this
tree. Both BLOCKERs and four ISSUEs are repaired, I-1 and I-2 changed product
code, I-3 and I-6 were missing tests and now exist, and the three NOTEs are
answered at the producer. N-2 is the one the reviewer read differently from the
code: the acks are undrained and nothing is lost by it.

| # | Disposition |
|---|-------------|
| B-1 | FIXED. `placeSectionNodes` (`solver.go`) puts a coarse node after the last operation that creates or modifies a resource, so it runs before the destructions. The position is a placement and not an edge, because an edge from every create and to every destroy closes a cycle with `iface-remove-address-before-add-same-address` and would abort an address move that works today. Evidence: `.../scratch/cao-b1-solver-red.log`, `.../scratch/cao-b1-orchestrator-red.log`, `.../scratch/cao-b1-green.log` |
| B-2 | FIXED. Both guards are driven from `Server.ReloadConfig`, through the plugin's own decompose RPC. Evidence: `.../scratch/cao-b2-red-reserved.log`, `.../scratch/cao-b2-red-blank.log`, `.../scratch/cao-b2-green-restored.log` |
| I-4 | FIXED. `test/weakened/4c26aef3.md` and the TDD row above now say what the code does: the decomposer emits no listener operation, so the two listener rules ordered nothing and their coverage is LOST rather than replaced |
| I-5 | FIXED. `ResourceBridgeMember`, `ResourceSysctl`, `ResourceDHCP` and `ResourceTunnel` are deleted from `pkg/plugin/rpc/types.go` and both alias files. `ResourceRelationSameResource` is deleted, and `resourceKey`, `opAddrIface` and `firstNonZeroUint16`, which had no other user, go with it. The tests that used them now spell their own kind and use `ResourceRelationAny`, which builds the same two-node cycles |
| I-1 | FIXED. The deadline is the SUM of the participants' budgets (`computeSequentialDeadline`, `orchestrator_budget.go`), because nothing in a phase runs concurrently: an engine handler fires inside the emitter's goroutine and the bridge performs the plugin RPC inside it, and the ordered path applies one node at a time under one absolute instant. R-1 measured, see that row. Evidence: `.../scratch/cao2-i1-deadline-red.log` (apply deadline = 10s, want 20s), `.../scratch/cao2-i1-deadline-green.log` |
| I-2 | FIXED. `checkOperationRootCoverage` (`orchestrator.go`) refuses a participant that owns an operation for one root it has diffs on and none for another, naming the plugin and the root. Driven from `Execute`. Evidence: `.../scratch/cao2-i2-coverage-red.log` (`state = committed (err <nil>), want aborted`), `.../scratch/cao2-i2-coverage-green.log` |
| I-3 | FIXED (test). `TestExecuteRollsBackAnAppliedCoarseNode` (`orchestrator_test.go`) applies the coarse node, then fails the destroy that sorts after it, and asserts the participant sees its section apply and then the rollback. Evidence: `.../scratch/cao2-i3-rollback-red.log` under the revert that deletes `publishRollback` from the ordered path, `.../scratch/cao2-i3-rollback-green.log` |
| I-6 | FIXED (test). `TestApplyConfigOperationModifyPeerSwapsInPlaceAndKeepsTheSession` (`internal/component/bgp/reactor/operation_test.go`) edits the import filter chain alone, so `peerSettingsSwapPlan` answers "no restart", and asserts the peer pointer is unchanged. Evidence: `.../scratch/cao2-i6-swap-red.log` under a `swapped := false` revert (`the apply rebuilt the peer`), `.../scratch/cao2-i6-swap-green.log` |
| N-1 | CONFIRMED, no code. `AllowDual` still has one writer (`markDualPresence`) and no reader outside `solver.go`: `placeSectionNodes` neither writes nor reads it. The Known Limitation stands. One sentence of `docs/architecture/config/apply-ordering.md` said the solver "breaks the cycle with `AllowDual`", which the same page contradicts 50 lines later; it now says the removed edges break it and the flag marks the result |
| N-2 | NOT A LEAK. The acks do land undrained in `o.applyOKCh`, and what they carry is read by nobody: `emitApplyOK` (`config_tx_bridge.go`) fills both budget fields from `proc.Registration()`, which is where `buildTxInputs` (`reload_tx.go`) already read them, and the coordinator is built per transaction (`runTxCoordinator`). The buffer holds one per participant, which is the most that can arrive. `subscribeAcks` now says so, and the "Next transaction" row of `docs/architecture/config/transaction-protocol.md` is corrected: it named a transaction the coordinator does not live to see |
| N-3 | FIXED. The Deliverables row states what this spec's pages owe and names the tree-wide failures as other sessions'. Evidence: `.../scratch/cao2-doc-check-verify.log` |

**Found while fixing B-1, fixed here, and journalled in
`plan/journal/refactor-removes-feature.md`:** `TestReloadTxApplyBGPLast` was RED
on this tree. `participantsWithoutOperations` sorted the uncovered names
alphabetically, so `bgp` took its coarse node FIRST and lost what
`sortParticipantsBGPLast` exists to give it. The names now come back in
participant order, which `buildTxInputs` already makes deterministic.

**The QEMU evidence still describes the code.** `placeSectionNodes` moves coarse
nodes only, and the relative order of every other operation is unchanged by
construction, so `address-swap` sorts as it did. In `mixed-root` the coarse node
lands where it landed before, between the address create and the address
destroy, by decision rather than by tie-break. What did change is the order of
two coarse nodes AGAINST EACH OTHER, which neither test asserts. Both tests skip
on darwin and were not re-run: this is a reading of the change, not a fresh
walk. The native reload suite is 44 of 44 with 19 skipped
(`.../scratch/cao-functional-reload.log`).

**Round 2 leaves the QEMU evidence describing the code.** Neither fix changes
what `address-swap` and `mixed-root` exercise, and neither RED depended on a
deadline. The deadline only grew, and it bounds the wait for a plugin that does
not answer: both tests complete in seconds, and their REDs are a cycle abort and
a route installed before its address, neither of which is a timeout. The
coverage guard is inert across the whole first-party tree, because it fires only
for a participant that owns operations for one of its roots and not another, and
the two participants that own any declare one root each with no `*` wildcard
(`internal/component/iface/register.go`, `internal/component/bgp/plugin/register.go`,
and `ConfigOperations` has no other non-test declarer). Both tests still skip on
darwin and were not re-run: this is a reading of the change. The native reload
suite is 44 of 44 with 19 skipped after it
(`.../scratch/cao2-functional-reload-after.log`).

Verified at the producer and found sound: A-1 (one `emitSectionApply` for both
routes), A-2 (phase-1 verify precedes the planner, `Verify` skips a coarse node),
A-3 (`initCallbackDefaults` registers no operation default), rollback reach for
both node kinds, `runningPeerSettings` reading the reactor's own peer, the two
gained derived edges with none lost, the blank-entry refusal, every deliverable
grep, and the QEMU discrimination walk. R-2 was walked over four config shapes
and NOT reproduced.

---

Round 2, independent reader, 2026-09-11. Findings artifact:
`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/review-findings-cao-round2.md`.
Verdict: **findings** -- 2 BLOCKER, 1 ISSUE, 1 NIT. The gate is NOT clean.

Scope is round 1's repairs and what they touched. Every round-1 disposition was
re-verified at the producer. B-2, I-1, I-2, I-3, I-4, I-5, I-6, N-1, N-2 and N-3
hold. B-1 is half repaired, and the half it left is the first finding below.

| # | Severity | File / symbol | Finding |
|---|----------|---------------|---------|
| R2-B-1 | BLOCKER | `internal/component/config/transaction/solver.go` `placeSectionNodes` | The coarse node runs AFTER destroys whenever a create is delayed behind one, which `iface-remove-address-before-add-same-address` does on every cross-interface address move. Reproduced over the two rules iface registers: moving `10.0.0.1` from `eth0` to `eth1` together with deleting `eth2` (or removing a second address) sorts to remove, remove, add, coarse. The uncovered root is applied after an address and an interface it binds are gone, which is D1's own symptom. The page, the function comment and the round-1 B-1 disposition all state "so it runs before the destructions" as a guarantee; the Known Limitation names only the narrower case where a rule FORCES a destroy ahead of a create, and in the reproduction no rule forced those destroys anywhere. `TestTopologicalSortPlacesSectionNodeBetweenCreatesAndDestroys` has three cases and none delays a create |
| R2-B-2 | BLOCKER | `solver.go` `placeSectionNodes` with `orchestrator.go` `participantsWithoutOperations` | The `bgp` participant applies FIRST on every reload that touches a peer. It decomposes, so it owns operations and takes no coarse node, while its siblings on the `bgp` root (`rib`, `gr`, `rpki`, `bmp`, the `filter_*` set) each get one, and the placement puts every coarse node after the last create-or-modify, which is the peer operation. Reproduced: `bgp-modify-peer`, then `section-apply-rib`, `section-apply-gr`, `section-apply-rpki`. `sortParticipantsBGPLast` exists to prevent exactly this and `TestReloadTxApplyBGPLast` guards it, but that test registers no bgp decomposer, so its three participants are all coarse and the participant order survives. Before this spec every bgp reload took the deleted fallback and bgp applied last |
| R2-I-1 | ISSUE | `docs/architecture/config/transaction-protocol.md` | Two sentences still describe the deleted tiered deadline: "Engine enforces the dependency-graph-aware critical path" in the principles table, and "The engine knows the dependency graph for deadline computation" in section 9. `computeSequentialDeadline` calls `tierFn` never. The `apply-ordering.md` sentence quoted in R2-B-1 is the second page edit owed |
| R2-N-1 | NIT | `orchestrator_budget.go` `computeRollbackDeadline` | It returns `3 * applyDeadline` and is the only other reader of the field. The apply deadline grew from a per-tier max to an uncapped sum, `MaxBudgetSeconds` caps each participant and nothing caps the total, so the rollback wait grew with it. The page states the apply-side trade and not the multiplier |

**The QEMU evidence still describes the code, for what those two tests assert.**
`address-swap` is a pure swap and `mixed-root` a renumber on one interface: the
probe sorts both with the coarse node between the creates and the destroys, so
no address operation moved, and neither RED is a timeout the larger deadline
could reach. The caveat R2-B-1 adds is that the placement is shape-dependent and
both `.ci` tests exercise the shape that works, so neither is evidence about a
cross-interface move in a mixed transaction.

Reproduction harness: a `go test -overlay` probe under
`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/r2probe/`,
which injects a test into `internal/component/config/transaction` without
editing the tree.

## Phase 5 (2026-09-11): the ordering policy becomes the owner's

The contract for this phase is `docs/architecture/config/apply-ordering.md`,
section "The requirement", which carries the owner's words verbatim. It outranks
the Task and Acceptance Criteria sections above, which were written from a
paraphrase that had lost the requirement.

**What a reload applies now.** The binders the commit removes stop, then the
binders whose bound address is disturbed stop, then every address the commit
removes leaves the host, then every address the commit adds arrives, then the
binders start against them. Inside that last phase, the participants that apply
a whole section come before the decomposed starts, so a peer finds its RIB and
its filters configured before it opens a socket.

**Make-before-break is deleted, not left beside its replacement.** Gone:
`Params.AllowDual` (`pkg/plugin/rpc/types.go`), `markDualPresence`,
`tryRelaxCycle`, `isAddressOperation` and `opInterface`
(`internal/component/config/transaction/solver.go`), the constraint rule
`iface-add-address-before-remove-same-interface`
(`internal/component/iface/operation.go`), and the two constraint relations
those rules were the only users of, `same-interface` and `same-address`
(`internal/component/config/transaction/operation.go`).

`tryRelaxCycle` was judged on its merits and deleted for a reason of its own: the
cycle it existed to break cannot form any more. A cross-interface swap closed a
four-node cycle only because the make-before-break rule ordered an addition
ahead of a removal on the same interface. With that rule gone each address
carries one destroy, one create and a single edge between them, so a swap and a
three-way rotation both sort without relaxation
(`TestTopologicalSortSwapsAddressesBreakBeforeMake`,
`TestTopologicalSortRotatesAddressesBreakBeforeMake`). Every cycle is now
rejected, which is the fail-closed answer.

**One rule was widened rather than deleted.** Deleting only the make-before-break
rule would have left a RENUMBER unordered: the old address and the new one are
two different addresses, the same-address rule related them not at all, and the
decomposer emits its additions before its removals, so the graph would have
applied make-before-break by accident and re-armed the IPv4 primary/secondary
hazard `internal/plugins/iface/netlink/addr_primary.go` documents. So
`iface-remove-address-before-add-same-address` became
`iface-remove-address-before-add-address`, which states phases 3 and 4 directly.
That is the one edit this phase made in `internal/component/iface/` beyond the
rule it was sent to delete.

**The name check is gone and the ordering falls out of the graph.**
`sortParticipantsBGPLast` and `bgpParticipantName`
(`internal/component/plugin/server/reload_tx.go`) are deleted.
`placeSectionNodes` now puts a coarse node after the last operation that creates
an address or an interface, and before the first create-or-modify that targets
anything else. That is the gap between phase 4 and phase 5, stated in the verb
and resource-kind vocabulary the engine already orders by, with no root, no
participant and no operation label named.

Every round-2 finding is answered:

| Finding | Disposition |
|---------|-------------|
| R2-I-1 | FIXED (round 3). `docs/architecture/config/transaction-protocol.md` said the engine enforces a dependency-graph-aware critical path and knows the graph for deadline computation. Both now say what `computeSequentialDeadline` does: a flat sum over the participants, with tiers deciding only the order rollback acks are drained in |
| R2-N-1 | FIXED (round 3). The same page's deadline section now states that the rollback deadline is three times the apply deadline, so it grew with the sum |
| R2-B-1 | MOOT. Its premise was that a coarse node must precede the destructions, which was the inherited sequence. The requirement removes before it adds, so a coarse node runs after the destructions by design, and the page and the function comment now say so |
| R2-B-2 | FIXED. The coarse nodes of the `bgp` root's siblings sort before `bgp`'s own peer create or modify, because a peer operation targets `ResourcePeer` and is therefore a phase 5 start. Fenced end to end by `TestReloadTxAppliesCoarseSectionsBeforeBinderStarts`, which registers the decomposing participant FIRST so neither a name check nor the slice order can produce the answer, and at the solver by two new cases of `TestTopologicalSortPlacesSectionNodeBetweenAddressingAndBinders` |

**What the kernel says, and what is still open.**
Both files carry `option=needs-linux:caps=net-admin` and skip on darwin, where
this phase was implemented, so both were walked in a QEMU guest on 2026-09-11
against Ze's own runtime kernel. `mixed-root` passes and reddens under each of
two reverts, so the owner's order holds across two roots on a real kernel:
remove the address, add the address, then apply the section of the root that
binds it.

`address-swap` proves phases 3 and 4 the same way, and since later on 2026-09-11
it proves phases 2 and 5 as well, on the same kernel. It passes, 11 of 11 steps,
and it reddens under one revert for each half.

Reaching that took three repairs to its scaffolding, and none of them is a
weakened assertion. The walk first found two defects there, and neither was a
property of the apply order. Its driver sent SIGTERM as soon as `ip addr` showed
the addresses had moved, which is the end of phase 4, so the daemon died inside
the reload and `reloadComplete()` never printed the line the file expects. And
its `option=tcp_connections:value=2` fenced nothing: `ze-peer` closed each
connection when that connection's expectations were met, so the session was
already gone when the reload started and a daemon emitting no peer operation
reached two connections on its own retry timer. Measured under that revert:
`ze-peer` reported "successful" after two connections
(`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/dw/walk-4.log`).

The repairs answer both. `option=linger` now holds a connection open between
connections until the remote closes it (`endSequence`,
`internal/test/peer/reject.go`), so a second connection can only follow a
daemon-side drop. The peer writes `session-returned.txt` once the returned
session has re-announced its route, and the driver waits for that file before it
signals the daemon, so the reload finishes. And `action=rewrite` answers
completion like the other action arms, so the daemon's shutdown NOTIFICATION is
no longer matched against an empty expectation list. The file's DISCRIMINATION
header carries both reds, the green and their log paths.

---

Round 3, independent reader, 2026-09-11. Findings artifact:
`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/review-findings-cao-round3.md`.
Verdict: **findings** -- 5 BLOCKER, 6 ISSUE, 2 NIT. The gate is NOT clean.

The contract read first is `docs/architecture/config/apply-ordering.md`,
"The requirement". Where this spec and that page disagree, the page wins and
the spec is wrong, which is R3-B-5.

| # | Severity | File / symbol | Finding |
|---|----------|---------------|---------|
| R3-B-1 | BLOCKER | `internal/component/bgp/plugin/operation.go` `peerAddressConsumes`, with `transaction/depgraph.go` `addDerivedEdges` | The phase 5 START earns no edge whenever no CREATE produces the disturbed address, so it sorts ahead of the phase 3 removal. Reproduced: `[bgp-remove-peer-p1, bgp-add-peer-p1, iface-remove-address-eth0-10.0.0.1/24, iface-configure]` sorts in that order, so the session restarts, binds the address, and the address is removed under it. Reachable when an operator deletes an address a peer still names, and on any `auto`-source peer when a commit deletes an address. The page and the function comment both state the opposite as a guarantee |
| R3-B-2 | BLOCKER | `transaction/solver.go` `isAddressingKind`, `addsAddressing`, `startsABinder` | The fail-safe falls the wrong way for an operation whose target kind the engine cannot read: an unknown or EMPTY kind is read as a phase 5 binder, so the coarse nodes are inserted BEFORE it. `ifaceConfigureOperation` is that shape, and it is what creates a tunnel, wireguard or xfrm interface and the addresses on one. Reproduced: an address arriving on a configure-created device sorts to `[iface-remove-address, section-apply-static, iface-configure]`, so the uncovered root applies before the device and its address exist |
| R3-B-3 | BLOCKER | `internal/component/plugin/server/reload.go` (the `affected` loop over `pm.AllProcesses()`), `orchestrator.go` `participantsWithoutOperations` | The applied order of two coarse sections is non-deterministic, because `AllProcesses` ranges a map and nothing re-orders it. `TestReloadTxAppliesCoarseSectionsBeforeBinderStarts` fails 2 runs in 20 at HEAD. The `participantsWithoutOperations` comment and the round-1 B-1 disposition both claim `buildTxInputs` makes this deterministic |
| R3-B-4 | BLOCKER | this spec's TDD Test Plan and Boundary Tests tables | Four rows claim evidence that does not exist: `TestTopologicalSortRelaxesCycleByVerbAndKind`, `TestTopologicalSortRejectsNonAddressCycle`, `TestTopologicalSortPlacesSectionNodeBetweenCreatesAndDestroys`, `TestTopologicalSortThreeWayRotation`. Each was renamed or replaced in `test/weakened/4c26aef3.md` and the spec's tables were not carried forward |
| R3-B-5 | BLOCKER | this spec's Task, Required Reading, Current Behavior, Data Flow, Risks, Wiring Test, Acceptance Criteria, User Stories, Boundary Tests, Documentation checklist, Key Design Decisions, Known Limitations | Fourteen places state the superseded make-before-break policy. AC-4 demands the opposite of what the code and `config-apply-ordering-address-swap.ci` now assert. The findings artifact lists all fourteen |
| R3-I-1 | ISSUE | `docs/architecture/config/transaction-protocol.md` | R2-I-1 is not fixed: lines 22 and 518 still credit the engine with a dependency-graph-aware deadline that `computeSequentialDeadline` does not compute. Beside them the page now contradicts itself, line 973 saying the solver attempts relaxation and line 993 saying nothing relaxes a cycle, under a heading still named "dual-presence fallback" |
| R3-I-2 | ISSUE | this spec's `## Review Gate` | Round 2's R2-I-1 and R2-N-1 have no disposition row. The Phase 5 section answers only the two BLOCKERs |
| R3-I-3 | ISSUE | `orchestrator.go` `participantsWithoutOperations` | Its first sentence says the names are "sorted so the synthesized nodes are the same on every run". The body neither sorts nor is deterministic, and the same comment later says sorting would say nothing |
| R3-I-4 | ISSUE | `test/weakened/4c26aef3.md` | The file contradicts itself: its prose leaves `TestIfaceSameSubnetSwapOrdersAddBeforeRemove` RED and out of the table, and a table row in the same file says that test was renamed and inverted. The renamed test exists and passes; the old name does not exist. Every other claimed replacement was verified present |
| R3-I-5 | ISSUE | `reload.go` `appendDecomposingPlugins`, `orchestrator_budget.go` `computeSequentialDeadline` | Every decomposing plugin joins the transaction on every reload, and the deadline sums the budgets of participants no phase reaches |
| R3-I-6 | ISSUE | `iface/operation.go` `ifaceDiffHasChanges`, `bgp/plugin/operation.go` `bgpDiffTouchesPeer` | Both answer `false` on a JSON parse error, which the caller reads as "this root changed nothing". Unreachable from the first-party producer, and a parse error must still not be spelled the same way as no change |
| R3-N-1 | NIT | this spec's Critical Review Checklist | The row requiring `solver.go` to contain none of `bgp`, `interface`, `peer`, `address` is unmeetable by design: `isAddressingKind` reads two resource kinds, which `apply-ordering.md` justifies |
| R3-N-2 | NIT | `continue.md` | Stale: four commits, and reverts named against `tryRelaxCycle` and the uncovered-participant condition, neither of which exists |

**What round 3 checked and found sound.** `DisturbedAddresses` matches the
owner's definition on all three disturbances and on the MTU exclusion. The
second planner pass fails closed through `checkDisturbanceSettled`. A commit
that disturbs nothing sends a decomposing plugin no event. No cycle is
constructible from the first-party declarations, so R-2 is not reproduced. The
interface decomposer drops no key. A root that both provides and binds
addressing is handled, because the phase line is read per operation. And the
`internal/test/peer` linger claim is TRUE:
`test/reload/config-apply-ordering-address-swap.ci` is the only peer block in
any `.ci`, `.et` or `.msg` under `test/` that carries `option=linger`, carries
no `option=conn_map`, and is owed a connection past the first.

**Round 3 disposition, 2026-09-11.** The two spec BLOCKERs were repaired first,
in `d4f54da586`. The three product defects and the six issues follow, each with
the test that fences it.

| # | Disposition |
|---|-------------|
| R3-B-1 | FIXED, in the ORDERING and not in the emission. The start is owed: the peer is still configured, and suppressing it would leave the reactor without a peer the config declares, with nothing able to bring it back -- `handleAddrAddedPayload` starts a LISTENER for a peer the reactor still holds, and a later commit that re-adds the address disturbs nothing, so the decomposer is never asked. `addDerivedEdges` gains a third direction: a destroy runs before every create and modify that consumes what it takes away. The MODIFY arm of it is reachable by a third-party binder root only, and the claim that it gives `bgp-modify-peer` an ordering it never had is withdrawn (R4-N-1): `decomposeBGPOperations` emits a modify only where `peerBindingDisturbed` answered false, and `DisturbedAddresses` names every address a destroy produces, so a modify-peer never consumes an address the same commit removes. `bcba32f48e`'s message carries the withdrawn claim and cannot be edited. Fenced by `TestBGPStartsThePeerAfterTheAddressItBindsIsRemoved` and `TestBGPStartsTheAutoSourcePeerAfterTheRemovals` (`internal/component/bgp/plugin/operation_disturbed_test.go`), which drive the real `interface` and `bgp` decomposers, and by a case of `TestBuildOperationGraphDerivedEdgesMatchDeletedRules` |
| R3-B-2 | FIXED. `providesAddressing` (`solver.go`, renamed from `isAddressingKind`) reads an operation that declares NOTHING, neither a target kind nor a produced resource (`Target.Kind == "" && len(Produces) == 0`), as providing addressing, so the coarse nodes wait for it. Round 4's repair narrowed that branch to those two facts together, and this row states the rule as it stands. `addsAddressing` now accepts a modify, which is the verb `ifaceConfigureOperation` carries. A declared kind still decides the side it is on, so a binder joins phase 5 by declaring its own kind and no enumeration of binder kinds enters the engine. Fenced by the "after an operation whose kind the engine cannot read" case of `TestTopologicalSortPlacesSectionNodeBetweenAddressingAndBinders` |
| R3-B-3 | FIXED. `participantsWithoutOperations` sorts the uncovered names again. The sort was removed in round 1 so that `sortParticipantsBGPLast` could decide the order; that function is deleted, and nothing pinned it since. Fenced by `TestParticipantsWithoutOperationsSortsTheCoarseNodes`, and `TestReloadTxAppliesCoarseSectionsBeforeBinderStarts` now passes 50 runs of 50 |
| R3-B-4 | FIXED in `d4f54da586`. The TDD Test Plan and Boundary Tests tables name tests that exist: `TestTopologicalSortSwapsAddressesBreakBeforeMake`, `TestTopologicalSortRotatesAddressesBreakBeforeMake`, `TestTopologicalSortNonAddressCycleFails` and `TestTopologicalSortPlacesSectionNodeBetweenAddressingAndBinders`. Verified by round 4, which resolved each of the four names |
| R3-B-5 | FIXED in `d4f54da586`. The fourteen make-before-break sentences are corrected, and AC-4 states the requirement's order: no interface holds both addresses. What survives is in Current Behavior, which describes the tree before this spec. Verified by round 4, which found no residue outside that section |
| R3-N-1 | FIXED (round 5). The Critical Review Checklist row asked the solver to contain none of `bgp`, `interface`, `peer` and `address`, which `providesAddressing` cannot meet: it reads `ResourceAddress` and `ResourceInterface` to draw the phase line. The row now polices what it was written to police, which is that the solver names no root, no participant and no operation label |
| R3-N-2 | FIXED (round 5). `continue.md` lists every commit of this spec and names the two QEMU reverts as the tree carries them |
| R3-I-1 | FIXED. See the R2-I-1 row above: both sentences now describe the flat sum, the "cycle relaxation" sentence and heading are gone, and the `solver.go` source anchor names the placement |
| R3-I-2 | FIXED. R2-I-1 and R2-N-1 have disposition rows in the round-2 table above |
| R3-I-3 | FIXED with R3-B-3. The comment says what the function does and why the order is arbitrary but stable |
| R3-I-4 | FIXED. The false paragraph is deleted from `test/weakened/4c26aef3.md`. The table row is the true statement: the test was renamed to `TestIfaceSameSubnetSwapOrdersRemoveBeforeAdd`, its assertion inverted, and it passes |
| R3-I-5 | NOT A DEFECT, and the comment that gave a false reason is corrected. The sum must include a diffless decomposing participant, because that participant receives one per-operation apply for each operation it emits under the same absolute instant, and the binder the requirement is about is exactly that participant. What the sum over-counts is a decomposer that emits nothing this time, which nothing can know before the planner has run, and the over-count only lengthens the wait for a plugin that has hung |
| R3-I-6 | FIXED. `ifaceDiffHasChanges` and `bgpDiffTouchesPeer` answer `(bool, error)`, and each decomposer aborts the transaction on a diff section that will not parse. Fenced at both entry points by `TestIfaceOperationDecomposerRefusesADiffItCannotParse` and `TestBGPOperationDecomposerRefusesADiffItCannotParse` |

**The QEMU evidence still describes what the two tests assert, and one applied
order moved.** `address-swap` moves two addresses that are both re-created, so
every peer operation keeps the edges it had and the new destroy-before-consume
edge adds one the create already implied. In `mixed-root` the coarse static
section now applies AFTER `interface-configure` rather than before it, because
that operation declares no kind and the fail-safe reads it as addressing. What
the file asserts is unchanged and still holds: the old address leaves, the new
one arrives, and only then is the route that binds it installed. Its RED, making
`sectionNodePosition` answer 0, still puts the section before both addresses.
Each test has one uncovered participant, so the coarse sort orders nothing
either of them can see. Both tests skip on darwin and were not re-run: this is a
reading of the change, not a fresh walk.

---

Round 4, independent reader, 2026-09-11. Findings artifact:
`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/review-findings-cao-round4.md`.
Verdict: **findings** -- 2 BLOCKER, 2 ISSUE, 2 NIT. The gate is NOT clean.

Scope is round 3's repairs in `bcba32f48e` and `d4f54da586` and what they
touched. Every case below drives the REAL `interface` and `bgp` decomposers
through `OperationDecomposerFor`, then `BuildOperationGraph` and
`TopologicalSort` with the registered rules, in a `go test -overlay` probe under
`.../scratch/r4probe/` that edits no file in the tree.

| # | Severity | File / symbol | Finding |
|---|----------|---------------|---------|
| R4-B-1 | BLOCKER | `internal/component/iface/operation.go` `ifaceConfigureOperation` and the `configureCreates` branch of `decomposeIfaceOperations`, with `transaction/depgraph.go` `addDerivedEdges` | A phase 5 START earns NO edge when the address it consumes ARRIVES through the configure operation, so it sorts ahead of phase 4. An address on an interface this package has no create primitive for (tunnel, wireguard, xfrm) produces no `add-address`: `configureCreates` skips it, and `ifaceConfigureOperation` declares neither `Produces` nor `Consumes`, so nothing in the graph can be ordered after it. Reproduced (`r4probe/run3.log`, `TestR4CNewXFRMAndPeer`): a new xfrm0 carrying `10.0.0.9/30` with a new passive peer bound to it sorts `[bgp-add-peer-p9, interface-configure]`. `AddPeer` (`bgp/reactor/reactor_peers.go`) then opens the listener for a passive peer, the bind fails because the address is not on the host, and the whole transaction rolls back. This is the hole R3-B-2 repaired for a COARSE node, left open on the decomposed side: `providesAddressing` applies the fail-safe to the section nodes and nothing applies it to a binder start. `decomposeIfaceOperations` states the opposite, that "an address arriving on an interface it creates waits for it" |
| R4-B-2 | BLOCKER | `internal/component/config/transaction/solver.go` `sectionNodePosition`, `addsAddressing` | A decomposed binder start that sorts BEFORE `interface-configure` runs before EVERY coarse section. `addsAddressing` now accepts a modify with an absent kind, which `interface-configure` is, and four constraint rules put that operation after every resource operation, so `addressingDone` advances past it and any binder start already sorted is left ahead of the coarse nodes. Reproduced (`r4probe/run6.log`, `TestR4FPeerEditPlusAddressAdd`) on an ordinary config -- change a peer's hold-time, add an address on another interface -- as `[bgp-modify-peer-p1, interface-add-address-dum1-10.0.0.5_24, interface-configure, section-apply-gr, section-apply-rib, section-apply-rpki]`, and again on an address swap with two peers (`run5.log`), where one peer starts before the coarse sections and the other after. `docs/architecture/config/apply-ordering.md` publishes the guarantee this breaks: "Inside phase 5 it runs BEFORE the decomposed starts ... the `bgp` root's peers start after the plugins that configure the RIB, the graceful-restart state and the filters have applied their sections." R2-B-2's disposition calls this fixed; its fence, `TestReloadTxAppliesCoarseSectionsBeforeBinderStarts`, carries no addressing operation and cannot see the case |
| R4-I-1 | ISSUE | this spec's `## Review Gate`, round 3 disposition table | `R3-B-4`, `R3-B-5` (both BLOCKER), `R3-N-1` and `R3-N-2` have no disposition row. This is the omission round 3 itself raised as R3-I-2 about round 2. The two BLOCKERs WERE repaired in `d4f54da586`, verified here, so the gap is the record. The two NITs are not repaired |
| R4-I-2 | ISSUE | this spec's Critical Review Checklist, `continue.md` | R3-N-1 and R3-N-2 are still live. The checklist still requires the solver to name none of `bgp`, `interface`, `peer`, `address`, which `providesAddressing` makes unmeetable by reading `ResourceAddress` and `ResourceInterface`. `continue.md` still lists four commits where six exist, and still names the two QEMU reverts as a `tryRelaxCycle` returning the unreduced edge set and a reinstated uncovered-participant condition, neither of which exists |
| R4-N-1 | NIT | `bcba32f48e`'s message and the `R3-B-1` disposition row | Both claim the third derived edge "also gives a modify-peer the ordering it never had against the removal of the address it binds". No first-party decomposition reaches it: `decomposeBGPOperations` emits `bgp-modify-peer` only where `peerBindingDisturbed` answered false, and `DisturbedAddresses` names every address a destroy produces, so a modify-peer never consumes an address this commit removes |
| R4-N-2 | NIT | `test/weakened/4c26aef3.md` | The file ends "The table below carries every row in this file." above a table header with no rows. The row loss is by design, because `PruneLanded` drops rows that landed in an earlier commit, so what is left is the sentence |

**Round 3's repairs, verified at the producer.** The third derived edge holds:
probed on the real decomposers, a delete of a bound address now sorts
`[bgp-remove-peer-p1, interface-remove-address, interface-configure,
bgp-add-peer-p1]`, and an `auto`-source peer takes the same order. It closes no
cycle: the only edge whose head is a destroy is `edgeConsumeBeforeDestroy`,
whose tail is also a destroy, and no first-party rule points a non-destroy at a
destroy, so every cycle lies wholly inside the destroys and the new edge, which
always runs destroy to non-destroy, can lie on none. A hostile rule ordering a
create ahead of a destroy does close one, and `TopologicalSort` answers
`ErrOperationCycle` with nothing applied. `providesAddressing` holds for a
coarse node. `participantsWithoutOperations` sorts, the comment matches, 25 runs
of the two fence tests are green, and nothing else reads participant order to
decide an applied order. Both diff predicates answer the parse error, both
decomposers abort, and no plugin can trigger it because the diff sections are
built by the core from a `map[string]any` (`buildDiffSections`). The spec
correction leaves no make-before-break residue outside Current Behavior, which
describes the tree before the change.

**The QEMU evidence for `mixed-root` still holds under the moved order.**
`assertMixedRootOrder` reads three kernel notifications only, and
`interface-configure` produces none of its own on that config because the
operations have already reached the end state it applies. The recorded RED that
makes `sectionNodePosition` answer 0 still puts the static section before both
addresses; the other RED is untouched. Not re-walked: this is a reading.

**The five phases.** 1, 2 and 3 hold for `bgp`, at `decomposeBGPOperations`,
`peerBindingDisturbed` with the second planner pass, and
`decomposeIfaceOperations` with `iface-remove-address-before-add-address`. 4 is
PARTLY, because an address the configure operation applies can be ordered
against nothing (R4-B-1). 5 is NO, by R4-B-1 and R4-B-2.

**Round 4 disposition, 2026-09-12.** Both BLOCKERs had ONE cause: the configure
operation declared nothing, so nothing in the graph could be ordered against
what it does, and the solver carried a special case for it instead. The repair
is in three parts, and the evidence paths below are under
`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/`.

| # | Disposition |
|---|-------------|
| R4-B-1 | FIXED at the declaration. `ifaceConfigureOperation` takes the interfaces `decomposeIfaceOperations` lets it create and the addresses that arrive on them, and declares both in `Produces`. `addDerivedEdges` derives the produce-before-consume edge from the DECLARATION rather than from the verb, so a modify that produces a resource is ordered exactly as a create is. The reviewer's own probe answers the other way now: `TestR4CNewXFRMAndPeer` sorts `[interface-configure, bgp-add-peer-p9]` (`r5-probe.log`). Fenced by `TestIfaceConfigureOperationDeclaresTheAddressingItCreates` (`internal/component/iface/operation_test.go`), which drives the real decomposer and sorts the graph, and by `TestBuildOperationGraphDerivesModifyProducerBeforeConsumer`. RED `r5-red-iface.log`, `r5-red-tx.log`; GREEN `r5-green-1.log`, `r5-green-2.log` |
| R4-B-2 | FIXED in the SORT, because no single insertion point can satisfy both of `placeSectionNodes`'s bounds while the order interleaves phase 4 and phase 5. `operationPhase` (`solver.go`) puts an operation on one of three rungs, a stop, the addressing, and a start, and `kahnSort` drains the lowest rung that has a ready operation, so those three fall in that order over the whole commit and not only over the operations one resource relates. Phases 3 and 4 share the addressing rung on purpose: which removal precedes which addition is stated by `iface-remove-address-before-add-address` as an EDGE, a rung would state it a second time, and the edge is what holds against a rule pointing the other way. It is a tie-break and not an edge: an operation joins its rung only once every edge into it has run, so no dependency is overridden and no cycle can be closed. `TestR4FPeerEditPlusAddressAdd` now sorts `[interface-add-address-dum1, interface-configure, section-apply-gr, section-apply-rib, section-apply-rpki, bgp-modify-peer-p1]`, and `TestR4ESwapWithBGPSiblings` puts both peer starts after all three sections (`r5-probe.log`). Fenced by `TestTopologicalSortOrdersThePhasesTheEdgesLeaveFree`, and the fence R4-B-2 called blind, `TestReloadTxAppliesCoarseSectionsBeforeBinderStarts`, now carries an addressing operation the binder start does not consume. RED `r5-red-tx.log` and `r5-server-red.log` (`[op-binder-start, op-add-addressing, subsystem-a, subsystem-b]` under a `kahnSort` with the phase queues disabled); GREEN `r5-server-green.log` |
| R4-I-1 | FIXED. The round-3 disposition table gains rows for R3-B-4, R3-B-5, R3-N-1 and R3-N-2, and this table is round 4's |
| R4-I-2 | FIXED. The Critical Review Checklist row now polices what it was written to police: no ROOT, no PARTICIPANT and no operation LABEL in the solver, with `ResourceAddress` and `ResourceInterface` named as expected. `continue.md` takes each file's DISCRIMINATION header as the authority on its reverts. Its commit count was still one short when this row was written, which is R5-N-1, and the count is now gone rather than corrected |
| R4-N-1 | FIXED. The R3-B-1 row withdraws the claim that the third edge gives `bgp-modify-peer` an ordering it never had, and says the modify arm is reachable by a third-party binder root only. `bcba32f48e`'s message carries the withdrawn claim and cannot be edited |
| R4-N-2 | FIXED. `test/weakened/4c26aef3.md` says the rows landed and `PruneLanded` dropped them, so the empty table is the design. The same paragraph's claim that the two QEMU walks are outstanding is corrected, because both were walked on 2026-09-11 |

**The absent-kind branch of `providesAddressing` survives, and it is narrower.**
It reads an operation that declares NOTHING, no target kind and no produced
resource, and `interface-configure` is no longer that operation. Deleting it
would flip the engine's fail-safe for an opaque third-party operation from
provider to binder, which is the direction "The fail-safe default" of
`docs/architecture/config/apply-ordering.md` costs at a binder holding an
address that is gone. It is still reached first-party by a configure operation
that creates nothing, an MTU edit for example, and by an address arriving on an
EXISTING device of a type this package cannot create, which the decomposer
leaves to the configure operation undeclared and the phase rank then orders.

**What the new edge was tried against.** Five shapes, driven through the real
decomposers where they could be (`r5-adversarial.log`): an address moving onto a
device the configure operation creates, two such devices trading addresses, a
destroy that consumes an address the configure operation produces (no edge, the
destroy consumer is excluded), a third-party consumer of a modified peer (edge,
no cycle), and a third-party `add-address` on the interface the configure
operation creates. Only the last closes a cycle, no first-party decomposition
emits it, and `TopologicalSort` answers `ErrOperationCycle` with nothing
applied.

**The QEMU evidence still describes the code, and each recorded revert was
re-measured on the real decomposers.** `mixed-root` reads three kernel
notifications and its static section is still the only coarse node, placed after
both address operations. In `address-swap` every peer operation keeps the edges
it had, and the rungs agree with them. The revert both files share, unregistering
`iface-remove-address-before-add-address`, still lands the addition first on both
shapes, which is what makes each of them RED: measured at the decomposers in
`r5-rulecheck.log`, where the renumber sorts
`[add-address-zdual0-10.93.0.1, remove-address-zdual0-10.92.0.1, ...]` and the
swap sorts both additions ahead of both removals. That measurement is why phases
3 and 4 share one rung: a rung of their own would have enforced the order the
revert removes, and both recorded REDs would have gone green with the defect
present. `sectionNodePosition` answering 0 still puts the section at the head.
Both tests skip on darwin and were not re-run: the ORDER is measured here, the
kernel's answer to it is the reading.

---

Round 5, independent reader, 2026-09-12. Findings artifact:
`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/review-findings-cao-round5.md`.
Verdict: **findings** -- 1 BLOCKER, 0 ISSUE, 2 NIT. The gate is NOT clean.

Scope is round 4's repair in `1f9993071` and what it touched, plus the three
re-verifications that disposition asked for: the five phases at their producers,
the fail-safe direction at every point that decides whether something is
disturbed, and the QEMU evidence under the moved order. Every order below was
measured by driving the REAL `interface` and `bgp` decomposers through
`OperationDecomposerFor`, then `BuildOperationGraph` and `TopologicalSort` with
the registered rules, in `go test -overlay` probes under `.../scratch/r5rprobe/`
that edit no file in the tree.

| # | Severity | File / symbol | Finding |
|---|----------|---------------|---------|
| R5-B-1 | BLOCKER | `internal/component/iface/operation.go` `decomposeIfaceOperations`, with `config_apply.go` `bindDevices` and `desiredState` | The decomposer DISCARDS the interface listing's error (`infos, _ = b.ListInterfaces()`), and an ethernet entry it cannot bind contributes no address to EITHER side, so a commit that moves an ethernet address answers "nothing disturbed" and no binder is stopped. Reproduced (`.../scratch/r5rprobe/run-eth.log`, `TestR5EthernetMoveWithNoListing`): 10.0.0.9/24 moving eth0 to eth1 with a passive peer bound to it sorts `[interface-configure, section-apply-gr, section-apply-rib]` with an empty disturbed set and no `bgp-remove-peer-p9`. Two paths then move the address under the live session, each taking a listing of its own: `applyPendingConfig` (`config_apply.go:957`, which LOGS the very failure the decomposer swallows) and `reconcileOnReadyWithJournal` (`config_apply.go:1264`), the second running OUTSIDE any transaction on `vppevents.EventConnected`. A PARTIAL failure is worse than the total one: where only the SOURCE entry reads unbound, an `add-address` is emitted for the destination and no `remove-address` for the source, so both interfaces hold the address at once, which is the make-before-break AC-4 forbids. The comment above the line claims "That is the same fail-safe direction the apply path takes" -- it is not, because the apply path's deferral changes nothing while the decomposer's silence is read downstream as a fact about the whole commit. The requirement answers it the other way: "If it is not easy to establish what action will lead to what, be safe and deconf/reconf". Ethernet is the only kind reachable, because `bindDevices` returns nil for a config with no ethernet entry and `deviceFor` then reports every other name bound; it is also the only kind a production BGP box binds. NOT introduced by `1f9993071`: the line arrived in this spec's own `f9cf246e49` |
| R5-N-1 | NIT | `continue.md`, `# spec-config-apply-ordering-covers-every-root` | "Fifteen commits are in HEAD" and "round 5's repairs are uncommitted". HEAD carries SIXTEEN: `1f9993071` is the commit that wrote those words and has no row of its own, and no round-5 repair existed when they were written. Same class as R3-N-2 and R4-I-2, both of which were about this file's commit list |
| R5-N-2 | NIT | this spec's `## Review Gate`, the R3-B-2 disposition row | "`providesAddressing` ... reads an operation that declares NO kind as providing addressing" is no longer true. `1f9993071` narrowed it to an operation declaring NOTHING: the absent-kind branch answers only when `Target.Kind == "" && len(Produces) == 0`. Round 4's disposition states the narrowing; the round 3 row still states the old rule |

**The three-rung sort, tried adversarially.** Eight shapes, and none of the three
failures the claim would have to rule out occurred. An edge pointing a binder
start ahead of an addressing operation: the EDGE won. Every rung populated at
once: nothing starved, because `popLowestPhase` scans every rung on each pop. A
chain the edges fully determine, fed in three input orders: identical to a plain
single-queue Kahn sort, so the rungs change no result the edges already decided.
A produce-and-consume cycle: `ErrOperationCycle`, because an operation joins a
queue only at indegree 0 and a tie-break can neither close nor hide one. Two
hundred free operations over all three rungs, 26 builds: byte-identical every
time, with no addressing or stop operation after any start. Coarse nodes with
only starts and with only stops: the phase-4 to phase-5 boundary holds at both
ends. Two shapes are recorded and not reported, because no first-party
decomposition reaches either: a destroy declaring nothing lands on the ADDRESSING
rung rather than the stop rung, and an opaque third-party operation lands on the
addressing rung where it can precede an address the commit adds.

**The configure operation's deletion arm is right, and for a reason wider than
the four rules.** Probed on the real decomposers, an xfrm device removed with a
peer bound to the address it carried sorts `[bgp-remove-peer-p9,
interface-remove-address-xfrm0-10.0.0.9_30, interface-configure,
section-apply-gr, section-apply-rib, bgp-add-peer-p9]`. The configure operation's
`Produces` is always empty or addressing, so `providesAddressing` puts it on the
addressing rung, ahead of every binder start and behind every stop, whatever the
four `*-before-configure` rules say. An address moving ONTO a device it creates
takes the same order.

**The deleted `VerbModify` branch changed one operation's edges and no other.**
`interface-configure` now starts edges to the consumers of what it produces,
which is the repair. `bgp-modify-peer` declares `Produces: ResourcePeer`, and
nothing in `internal` or `pkg` declares `ResourcePeer` in `Consumes`, so it
starts no edge and no order moved for it.

**The five phases.** 1 holds, and the stop sorts first even against an addition
it shares no resource with. 2 holds for every interface kind except an ethernet
entry the decomposer cannot bind, which is R5-B-1. 3, 4 and 5 hold, 4 and 5
repaired. The MTU carve-out holds: an MTU edit disturbs nothing and emits the
configure operation alone.

**The QEMU evidence still describes the code.** Both recorded reverts still
reorder under the rungs, measured at the decomposers rather than re-walked.
`mixed-root` sorts `[remove-address-zmix0-10.92.0.1,
add-address-zmix0-10.93.0.1, interface-configure, section-apply-static]`;
`assertMixedRootOrder` reads exactly three notifications and the configure
operation emits none of its own on that config. Unregistering
`iface-remove-address-before-add-address` sorts the ADDITION first, on the
renumber and on the swap alike, which is the message each driver prints.
`sectionNodePosition` answering 0 still puts the static section ahead of both
addresses, because `placeSectionNodes` extracts the coarse node and re-inserts it
at the given index whatever rung it drained on. The rungs made neither revert
pass, which is the claim the commit message makes about phases 3 and 4 sharing
one rung, measured here rather than reasoned.

**`test/weakened/4c26aef3.md`: every row true.** The empty table is by design and
the file says so, naming `PruneLanded`, which exists at
`internal/le/testweakened/shard.go` and drops exactly the rows it claims. The
paragraph about the two QEMU walks is corrected and agrees with both files'
DISCRIMINATION headers. Every named replacement test resolves.

**Round 5 disposition, 2026-09-12.** The evidence paths below are under
`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/`.

| # | Disposition |
|---|-------------|
| R5-B-1 | FIXED by REFUSING the commit. `decomposeIfaceListing` (`internal/component/iface/operation.go`) takes the one listing that resolves both sides' hardware selectors, and answers an error where the config carries an ethernet entry and no listing is available. `decomposeIfaceOperations` returns that error, `operationPlannerFromTrees` returns it, and `TxCoordinator.Execute` aborts the transaction before the executor runs, which is the shape round 3 gave `ifaceDiffHasChanges` and `bgpDiffTouchesPeer`. The false comment claiming the discard was "the same fail-safe direction the apply path takes" is deleted with the discard. Fenced by `TestIfaceOperationDecomposerRefusesAnEthernetMoveItCannotBind` (`internal/component/iface/operation_test.go`), which takes the decomposer out of the registry the way the planner does and runs the reviewer's own eth0-to-eth1 move three ways: with a listing, with no backend, and with a listing that fails. RED `r5b1-red.log` (both blind arms answered no error, and the control already read `10.0.0.9` as disturbed); GREEN `r5b1-green.log` |
| R5-N-1 | FIXED. `continue.md` gains the row for `1f9993071` and drops both counts. The commit count was a second copy of the table under it, and the review-round sentence was a second copy of this section: the paragraph now names where each is declared instead. The R4-I-2 row below is corrected for the same reason |
| R5-N-2 | FIXED. The R3-B-2 disposition row states the rule as it stands: `providesAddressing` answers the absent-kind branch only for an operation that declares NOTHING, `Target.Kind == "" && len(Produces) == 0` |

**Why refusing, and not treating every selected entry as disturbed.** To say an
address is disturbed this root must EMIT a destroy for it, and the executor
applies that operation by handing the interface name straight to the backend
(`applyIfaceOperation`). An entry that binds to no device has no name to put
there but the logical one, which is the fallback `deviceFor` exists to prevent: it
reaches whatever kernel device happens to carry the entry's name. So the second
shape turns an unknown into a destructive wrong action, while the abort leaves
the running config untouched and tells the operator why. The requirement asks for
deconf and reconf where the effect cannot be established; deconf is out of reach
here, so applying nothing is what is left of it.

**What a PARTIAL listing does.** Every backend answers the whole listing or an
error: `netlinkBackend.ListInterfaces` fails the call on `listLinks`,
`vppBackendImpl.ListInterfaces` on `dumpAllInterfaces`, and the stub answers
`unsupported()`. None returns a short list with a nil error, so the partial
failure surfaces as the total one and takes the abort. What remains is an entry
UNBOUND against a listing that answered, and that is not silence: either its
device is absent, so its addresses are not on the host and there is nothing to
remove, or more than one present device answers its `mac/match` selector, which
`validateSelectors` refuses inside the transaction at the configure operation.
