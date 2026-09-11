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
`OperationExecutor` runs verify, execute and commit over the sorted list. The
intended order is: add the address, then update the services that bind it, then
remove the old address last, with a dual-presence window when two interfaces
swap addresses.

Four defects hold that ordering to two config roots.

| ID | Defect | Operator symptom |
|----|--------|------------------|
| D1 | `TxCoordinator.Execute` takes the operation path only when operations exist AND no participant is uncovered. One participant with a diff and no operation drops the whole transaction to the unordered section apply, including the address operations that were ordered correctly. The fallback logs at `Info` and the transaction reports success | A commit that changes an interface address and a firewall rule together gets no address ordering. A service is updated before its address exists, or an address is removed while a listener still binds it |
| D2 | `RegisterOperationDecomposer` has two non-test callers: `internal/component/bgp/plugin/operation.go` (root `bgp`) and `internal/component/iface/operation.go` (root `interface`). The same two files are the only registrants of constraint rules and settlement rules. `decomposeRootOperations` returns an empty result for every other root | firewall, dhcp, ike, l2tp, static, ntp and the rest declare no ordering, so a service in those roots that binds an address has no edge saying so. Each of them also triggers D1 |
| D3 | The ordering vocabulary is a central enumeration in the plugin ABI: 21 `ConfigOperationType` constants and 9 `ResourceKind` constants in `pkg/plugin/rpc/types.go`. Seven operation kinds have no user outside their own declaration and the two alias files | A new root joins the ordering only by editing a central enum, which `ai/rules/principles.md` bans. The vocabulary was written for roots that never arrived |
| D4 | The functional rotation, swap and reip tests emit `remove-peer` and `add-peer` only, never `add-address` or `remove-address`. The `AllowDual` dual-presence machinery is proven by `TestTopologicalSortCycleResolution` and `TestTopologicalSortThreeWayRotation` in `solver_test.go` only | The dual-presence window has never run through decompose, graph, executor, bridge, RPC and reactor. A break in that path is invisible |

Goal: every config root with a diff is a node in the operation graph, the
ordering is derived from what each operation produces and consumes rather than
from named operation pairs, the section-apply fallback is deleted, and the
dual-presence window is proven on the real path.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/apply-ordering.md` - the design of the subsystem this spec changes.
  → Decision: decomposition and constraint rules live in the owning component, never in a central switch. This spec keeps that and extends it to the roots that own no decomposer.
  → Constraint: "Address-only cross-interface cycles relax. Everything else is rejected." The relaxation stays; only the test that decides "is this an address operation" changes.
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
- The solver reads the operation type in two places only, `isAddressOperation` and `markDualPresence`, and both are asking "does this create or destroy an address".
- `AllowDual` is written by `markDualPresence` and read by nothing outside `solver.go` and the wire type. The dual-presence window comes from the removed cycle edges, not from the flag.

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
- Address-only cross-interface cycles relax. Every other cycle is rejected.
- `ConfigOperation` keeps its JSON keys `id`, `root`, `owner`, `type`, `target` and `params`, and `type` keeps its kebab-case values.
- Settlement waiters are armed before the apply is emitted.
- The two iface rules that are not produce/consume facts keep working: same-address uniqueness across interfaces, and make-before-break within one interface.

**Behavior to change:**
- A transaction with a diff on a root that owns no decomposer takes the operation path, not the section apply. The `Info` log line "operations do not cover every participant, using section apply" stops existing, because its branch stops existing.
- Ordering edges between a producer and a consumer of one resource are derived from declared produce and consume sets. The nine hand-written rules that state a produce/consume fact are deleted.
- The solver stops reading the operation type. It reads a new verb (`create`, `destroy`, `modify`) and the target resource kind.
- `ConfigOperationType` stops being a core enumeration. The seven unused constants are deleted, and the constants iface and bgp dispatch on move into those two packages.
- An operation with no verb is refused at planning with a named error, rather than ordered as if it were `modify`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator edits the config file and sends SIGHUP, or commits from the CLI editor. `Server.reloadConfig` computes a `config.ConfigDiff` over the running and candidate trees.
- Format at entry: per-root JSON sections in `transaction.DiffSection` (`root`, `added`, `removed`, `changed`).

### Transformation Path
1. `runTxCoordinator` builds participants from the affected plugin registrations, builds per-root `DiffSection` slices, starts `configTxBridge`, and installs the planner.
2. `TxCoordinator.Execute` runs phase 1 verify for every participant with a diff.
3. The planner walks the sorted roots. A root with an in-process decomposer, or a participant declaring decomposition, produces operations. Every other root produces nothing today. After this spec the orchestrator synthesizes one coarse node per uncovered participant.
4. `BuildOperationGraph` adds an edge for every registered constraint rule that matches a pair, and one for every producer and consumer pair over the same resource identity.
5. `TopologicalSort` orders the graph, relaxes an address-only cross-interface cycle, and marks the dual-presence members.
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
| A-7 | Nothing outside `solver.go` reads `Params.AllowDual`, so the dual-presence window is produced by the removed edges alone | grep over `internal/` and `pkg/`: the only non-test hits are `solver.go` and the field declaration | Deleting or keeping the flag changes apply behavior | The dual-presence functional test asserts both addresses present during the window, independent of the flag | confirmed (phase 4) by reading the producer and every reference. `markDualPresence` (`solver.go`) is the only writer, and it is also the last reader: nothing reads the field back. The whole non-test population is that function, the field declaration in `pkg/plugin/rpc/types.go`, and the doc comments beside them. `applyIfaceOperation` (`internal/component/iface/operation.go`) dispatches on the label and never touches `Params.AllowDual`, so the applier cannot behave differently for a dual-marked create. `test/reload/config-apply-ordering-address-swap.ci` asserts the window from the kernel's own netlink notifications and reads no flag |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Total coverage puts every participant into one ordered sequence, so a slow root delays roots that used to apply beside it | Reload wall time grows across the reload functional tests | The executor already applies one operation at a time. Measure the reload suite before and after, and report the delta rather than hide it |
| R-2 | Derived edges over-connect: a resource many roots consume produces a dense graph and a new cycle `tryRelaxCycle` rejects, turning a working reload into an aborted transaction | A reload test that passed now aborts with `operation dependency cycle` | Land the derivation against the existing iface and bgp fixtures first and compare the edge set, before any new root declares produce/consume |
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
| SIGHUP swapping two addresses between two interfaces | → | `tryRelaxCycle`, then `OperationExecutor.Execute` to the netlink backend | `test/reload/config-apply-ordering-address-swap.ci` |
| An operation whose label the core does not name | → | `OperationExecutor` emits it on the owner's event, the owner dispatches on the label | `TestOperationPathCarriesUnknownLabel` in `executor_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One reload changes an interface address and a static route in the same transaction | The address operations keep their order: the new address is present before the route that binds it is installed, and the old address is removed after. No log line says the transaction used the section apply |
| AC-2 | One reload whose only diff is in a root that registers no decomposer | The root's change is applied and the transaction reports committed. The applied result is identical to what the section apply produced before this spec |
| AC-3 | A root declares that its operation consumes an address, and registers no constraint rule | The address create runs before that operation, and the address destroy runs after it |
| AC-4 | One reload swaps address A from interface X to interface Y and address B from Y to X | Both addresses are present at the same time for the duration of the swap, and neither interface is left without an address at any observable point. The transaction commits |
| AC-5 | A decomposer emits an operation whose label no core package names, carrying a verb and a resource kind the core does name | The core orders it by verb and resource, carries the label unchanged to the owner, and the owner dispatches on it. No core package compares that label to a constant |
| AC-6 | An operation arrives with no verb | The transaction aborts with an error naming the plugin, the root and the operation id. Nothing is applied |
| AC-7 | One participant's apply fails, in a transaction mixing decomposed operations and a coarse root node | Rollback reaches both kinds: the decomposed operations replay their inverses in reverse order, and the coarse node's participant receives the section rollback. The transaction reports rolled back |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Changes an interface address and a static route in one commit | config file → diff → planner → graph (derived edges) → solver → executor → bridge → iface backend and the static plugin | `test/reload/config-apply-ordering-mixed-root.ci` |
| 2 | Swaps the addresses of two interfaces in one commit | config file → diff → iface decomposer → graph → `tryRelaxCycle` → executor → netlink | `test/reload/config-apply-ordering-address-swap.ci` |
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
| `TestTopologicalSortRelaxesCycleByVerbAndKind` | `internal/component/config/transaction/solver_test.go` | The relaxation decides on the verb plus the address resource kind, not on the operation label | PASS, and observed RED under a label-comparing `isAddressOperation` |
| `TestTopologicalSortRejectsNonAddressCycle` | `internal/component/config/transaction/solver_test.go` | Preserved rejection, restated against the verb test | PASS. It fences preserved behavior, so no phase-2 revert reddens it |
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
| `TestTopologicalSortPlacesSectionNodeBetweenCreatesAndDestroys` | `internal/component/config/transaction/solver_test.go` | B-1. A coarse node runs after the creations and before the destructions | PASS, and observed RED before the placement: two of its three cases put the coarse node first or in the middle of the creations |
| `TestTopologicalSortSectionNodeJoinsAnAddressSwapWithoutACycle` | `internal/component/config/transaction/solver_test.go` | R-2. A swap plus one uncovered root still sorts, which an edge-shaped position would not | PASS before and after. It fences the shape of the answer, not a change of behavior |
| `TestExecuteAppliesCoarseNodeAfterTheResourcesItBinds` | `internal/component/config/transaction/orchestrator_test.go` | B-1 through `Execute`, the door an operator reaches | PASS, and observed RED at `[iface-add-zx section-apply-static iface-add-address-zx]` |
| `TestReloadRefusesAPluginOperationCarryingTheReservedSectionApplyLabel` | `internal/component/plugin/server/reload_test.go` | B-2. `ReloadConfig` refuses a plugin-supplied `section-apply` label | PASS, and observed RED with the reserved-type refusal deleted |
| `TestReloadRefusesAPluginOperationDeclaringAResourceWithNoIdentity` | `internal/component/plugin/server/reload_test.go` | B-2. `ReloadConfig` refuses a blank resource entry, from the door rather than the helper | PASS, and observed RED with `validateResourceRefs` unwired |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Operations in one transaction | 0 to unbounded | The design names a 10000 warning the builder does not enforce. This spec adds no limit | N/A | N/A |
| Cycle members considered for relaxation | 2 to the graph size | Three-way rotation, already covered by `TestTopologicalSortThreeWayRotation` | 1, a self edge, rejected as a cycle | N/A |
| Coarse nodes per transaction | 0 to one per participant | One per uncovered participant | N/A | N/A |

This spec introduces no new numeric input. The table records the existing
bounds so implementation does not invent one.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-apply-ordering-mixed-root` | `test/reload/config-apply-ordering-mixed-root.ci` | An interface address plus a static route in one commit keeps address ordering (AC-1) | PASS in the QEMU guest (3.2s), and observed RED under its own revert. RED: the reinstated uncovered-participant condition sends the transaction to the unordered section apply, the static plugin installs the route before the address exists, and the kernel answers `fib-kernel: add route failed prefix=172.30.0.0/24 error="network is unreachable"`; the driver then reports `ZE-OBSERVER-FAIL: the renumber and the static route did not both land`. GREEN after restore: 11 of 11 steps pass. Log: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/cao/qemu-walk-clean.log` |
| `config-apply-ordering-coarse-root` | `test/reload/config-apply-ordering-coarse-root.ci` | A commit touching only a root with no decomposer still applies (AC-2) | PASS, and observed RED under its own revert |
| `config-apply-ordering-address-swap` | `test/reload/config-apply-ordering-address-swap.ci` | Two interfaces swap addresses, both present for the window, and a BGP peer bound to one of them stays reachable (AC-4, closes D4) | PASS in the QEMU guest (1.9s), and observed RED under its own revert. RED: with `tryRelaxCycle` returning the unreduced edge set the reload answers `config verify failed: operation dependency cycle`, nothing is applied, and the driver reports `ZE-OBSERVER-FAIL: the addresses never swapped`. GREEN after restore: 11 of 11 steps pass, the peer exchange holds its one connection across the swap, and the second `expect=stderr:contains` step, which is `OK: dual-presence window observed`, is one of them. D4 is closed: the window is now measured from the kernel's own notifications through decompose, graph, solver, executor, bridge, RPC and netlink. Log: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/cao/qemu-walk-clean.log`. Its two assertion functions were already proven by `TestAssertSwapWindowRefusesBreakBeforeMake` and `TestAssertMixedRootOrderRefusesARouteInstalledFirst` (`internal/test/fixture/register_config_apply_ordering_test.go`) |
| `config-apply-ordering-mixed-rollback` | `test/reload/config-apply-ordering-mixed-rollback.ci` | A failed apply in a mixed transaction rolls back both node kinds (AC-7) | PASS (8.9s), and observed RED under its own revert. The tree that reddened 36 of the 63 reload tests healed; the whole suite is now 44 pass, 19 skip. Its connection-2 assertion was strengthened after the first run: it asserted the End-of-RIB alone, which passes while the restored peer announces nothing, so it now asserts the `192.168.1.0/24` UPDATE as well. The revert that reddens it is rebuilding the peer from the operation's config subtree instead of from the running peer (`runningPeerSettings`, `internal/component/bgp/reactor/operation.go`); under it the peer exchange fails on a message mismatch and every other reload test stays green. Logs: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/functional-reload-{1,red-revert}.log` |

`config-apply-ordering-address-swap` and `config-apply-ordering-mixed-root`
carry `option=needs-linux:caps=net-admin`, matching
`config-apply-ordering-create.ci`: they assign addresses to real devices and
read them back.

**Both were walked on 2026-09-11 and both discriminate.** They skip on a darwin
host, so the route that ran them is QEMU: `./le qemu netns-test` does not cover
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
  (`tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/cao/pairbuild.log`).
- **The guest has ONE routing table.** The first walk ran the greens first, and
  each left its dummy devices, addresses and static routes behind. The
  mixed-root RED then failed with `add route failed ... error="file exists"`,
  which is the fixture's environment and not the revert, so that red was
  discarded. The walk now deletes `zdual0`, `zdual1`, `zmix0` and both static
  prefixes before each run and prints the table it starts from.

**Discrimination walk (`ai/rules/interop-and-goal-validation.md`, required, not
assumed):** for each of the four tests, revert the change it fences, rebuild the
daemon so the revert takes effect, run the test, record the RED output in the
closure section, restore the fix, and record GREEN. The reverts are: for
`mixed-root`, restore the uncovered-participant condition; for `coarse-root`,
delete the coarse node synthesis; for `address-swap`, make `tryRelaxCycle`
return the unreduced edge set; for `mixed-rollback`, skip the section rollback
for coarse nodes. A test that does not go RED under its own revert has not been
shown to discriminate and is not evidence.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `tx-protocol-external-plugin` | `test/reload/tx-protocol-external-plugin.ci` (existing, extended) | A Ze external plugin over the plugin hub transport | The ABI change reaches a plugin in a separate process: it takes part in an ordered transaction, and its coarse node is applied through `config-apply` | EXTENDED, RUN, PASS (4.8s) in the same suite run that greened `mixed-rollback`; it was RED for the tree's reason above until that tree healed. The test asserted only absences (exit 0, four rejected timeouts), which passes against a daemon that applies nothing. It now carries a second process, `tx-protocol-external-plugin-observer` (`internal/test/fixture/register_tx_protocol_external_plugin.go`), declaring the `bgp` root and decomposing nothing, and two positive assertions: the observer's own line from `OnConfigApply`, and `bgp config operation journal committed`, which only `OnConfigOperationCommit` writes and therefore only the operation path produces |

No third-party daemon speaks this protocol: the config transaction ABI runs
between the Ze engine and a plugin process. The external-plugin test is the
cross-process peer this spec owes.

## Files to Modify
- `internal/component/config/transaction/operation.go` - add the verb type and the produce/consume declaration to the registry surface, delete the two `ResourceRelation` values that state produce/consume facts, delete the seven unused operation constant aliases. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/config/transaction/depgraph.go` - derive edges from produce and consume sets over resource identity, beside the surviving rule-driven edges. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/config/transaction/solver.go` - `isAddressOperation` and `markDualPresence` decide on the verb plus the address resource kind, not on the operation label. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/config/transaction/executor.go` - route a coarse root node to the section apply event and the section rollback, keep the per-operation route for everything else. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/config/transaction/orchestrator.go` - synthesize one coarse node per uncovered participant, delete the section-apply fallback and its log line, keep `participantsWithoutOperations` as the synthesis input. Doc: `docs/architecture/config/transaction-protocol.md`
- `internal/component/plugin/server/reload_tx.go` - the planner refuses an operation with no verb, and reports an uncovered root to the orchestrator rather than making it invisible. Doc: `docs/architecture/config/transaction-protocol.md`
- `internal/component/plugin/server/config_tx_bridge.go` - dispatch a coarse root node through `SendConfigApply`. Doc: `docs/architecture/config/transaction-protocol.md`
- `internal/component/iface/operation.go` - declare produce and consume on each emitted operation, delete the five produce/consume constraint rules, keep the two that are not, move the four operation labels into this package. Doc: `docs/architecture/config/apply-ordering.md`
- `internal/component/bgp/plugin/operation.go` - declare consumption of the local address on peer and listener operations, delete all four constraint rules, move the peer and listener labels out of the shared ABI.
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
| 10 | Test infrastructure changed? | No | The four new `.ci` tests use existing runner options. No new tool, option or harness |
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
| "Address-only cross-interface cycles relax. Everything else is rejected." | The rule survives. The test behind it moves from the operation label to the verb plus the resource kind, so the mechanism sentence beside it is wrong |
| "The external contract is mandatory for v1 plugins: SDK types in `pkg/plugin/sdk/sdk_types.go` ..." | The contract gains the verb and the produce/consume sets, and a payload without a verb is refused. The sentence must state the refusal |
| The whole first paragraph of "What the tests do not reach": "they exercise the whole decompose to graph to executor to bridge to RPC to reactor path and none of the `AllowDual` machinery" | `config-apply-ordering-address-swap.ci` reaches it. The paragraph is replaced by what that test proves and by what remains unreached |
| "The current decomposers never emit an `ADD_ADDRESS` for an address that is already present." | This becomes false the moment a root outside iface declares an address it consumes. Re-verify at implementation, then rewrite or delete |

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
   - Tests: `TestExecuteRefusesOperationWithNoVerb`, `TestOperationPathCarriesUnknownLabel`, `TestSDKPluginWithoutOperationCallbacksAnswersUnknownMethod`, `TestTopologicalSortRelaxesCycleByVerbAndKind`
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
| Data flow | The solver names no root, no component and no operation label. Grep it for the strings `bgp`, `interface`, `peer` and `address` after the change |
| Rule: `ai/rules/no-layering.md` | The fallback branch, the nine rules, the seven unused constants and the two produce/consume `ResourceRelation` values are DELETED, not left unreachable |
| Rule: `ai/rules/principles.md` | No operation is ordered on a default. A missing verb aborts, and never becomes `modify` |
| Rule: `ai/rules/stale-comments.md` | The comment in `Execute` justifying the fallback, and the comment on `participantsWithoutOperations` describing the all-or-nothing decomposer contract, both describe deleted behavior |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The section-apply fallback is gone | `grep -n "using section apply" internal/component/config/transaction/orchestrator.go` returns nothing |
| The seven unused operation constants are gone | `grep -rn "OperationStartDHCP" --include=*.go .` and the same for the other six return nothing |
| The nine produce/consume constraint rules are gone | `grep -rn "RegisterConstraintRule" --include=*.go internal/` shows two registrations, both in `internal/component/iface/operation.go` |
| The solver reads no operation label | `grep -n "OperationAdd" internal/component/config/transaction/solver.go` and the same for `OperationRemove` return nothing |
| Coverage is total | `TestParticipantsWithoutOperationsEmptyAfterSynthesis` passes |
| The four new functional tests exist and pass | `./le test functional filter config-apply-ordering` |
| Each new functional test discriminates | The recorded RED and GREEN output pair for each of the four, in the closure section |
| Docs match the code | `./le doc check verify` passes |
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
- `AllowDual` is written and never read outside the solver. The dual-presence window comes from the edges `tryRelaxCycle` removes, so the flag today labels the result rather than instructing the applier.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A coarse root node is applied through the existing `config-apply` RPC | Route it through `config-operation-apply` like every other node | `config-operation-apply` has no SDK default handler, so a plugin that never registered one answers "unknown method" and the transaction aborts. The section route reaches every participant the section apply reaches today, which is the property the coarse node exists to keep |
| The coarse node owes no per-operation verify | Emit `config-operation-verify` for it, or invent a no-op verify | Phase 1 already verified the whole candidate config for that participant before the planner ran. A second verify is a second answer to a question already answered |
| The constraint rule registry survives, with only its produce/consume users deleted | Delete the registry and derive everything | Two iface rules state facts that are not produce/consume: same-address uniqueness across interfaces, and make-before-break within one interface. Deriving those from produce/consume would be wrong, so the registry keeps a real job and the derivation takes the rest |
| A missing verb aborts the transaction | Default to `modify` | A default orders the operation as if it had no dependencies, which is the silently wrong answer `ai/rules/principles.md` names. Pre-release, with no shipped external plugin, the refusal costs nobody |
| The label stays on the wire as free text | Delete it and dispatch on the verb plus the resource | The owner needs it: `applyIfaceOperation` and `applyBGPOperation` dispatch on it, and a log line reads it. The core stops reading it, which is the part that mattered |
| Operation labels move into the owning packages | Keep them in `pkg/plugin/rpc` as documentation | A label list in a shared package is a central enumeration again, and a new root would edit it because it looks like the place labels belong. The owning package already owns the dispatch |

## Known Limitations
- The 10000 operation warning and the 100 cycle-depth rejection named in the design stay unenforced. Config is operator supplied and bounded, so neither is a security boundary. Not this spec's work.
- The step-3 owner state check in `armSettlementWaiters` stays unimplemented. Total coverage does not change that path.
- This spec makes every root ORDERABLE. It does not make every root DECOMPOSED: firewall, dhcp, ike, l2tp, static and ntp still apply as one coarse node each. A coarse node is placed after the last operation that creates or modifies a resource and therefore before the destructions (`placeSectionNodes`, `solver.go`), which is ordering against the operations the other roots emit and none within itself. Where a destroy is forced ahead of a create by a surviving rule, the two halves cannot both hold and the creations win. Per-root decomposition is separate work, one spec per root, each a feature rather than a defect. None is written yet.
- `Params.AllowDual` keeps no reader outside the solver. Whether an applier should act on it, for example to suppress a make-before-break check, belongs to the per-root decomposition work above.

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Each of the four new functional tests observed RED under its own named revert and GREEN after restore, with both outputs recorded. All four are now walked. `coarse-root` and `mixed-rollback` were walked natively (logs in the Functional Tests table). `address-swap` and `mixed-root` were walked in the QEMU guest on 2026-09-11, red first and green after, all four daemons from one tree window: `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/cao/qemu-walk-clean.log`, with the one-tree evidence in `.../cao/pairbuild.log`
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

**Round 1 disposition, 2026-09-11.** Four findings are repaired in this tree and
the rest are still open.

| # | Disposition |
|---|-------------|
| B-1 | FIXED. `placeSectionNodes` (`solver.go`) puts a coarse node after the last operation that creates or modifies a resource, so it runs before the destructions. The position is a placement and not an edge, because an edge from every create and to every destroy closes a cycle with `iface-remove-address-before-add-same-address` and would abort an address move that works today. Evidence: `.../scratch/cao-b1-solver-red.log`, `.../scratch/cao-b1-orchestrator-red.log`, `.../scratch/cao-b1-green.log` |
| B-2 | FIXED. Both guards are driven from `Server.ReloadConfig`, through the plugin's own decompose RPC. Evidence: `.../scratch/cao-b2-red-reserved.log`, `.../scratch/cao-b2-red-blank.log`, `.../scratch/cao-b2-green-restored.log` |
| I-4 | FIXED. `test/weakened/4c26aef3.md` and the TDD row above now say what the code does: the decomposer emits no listener operation, so the two listener rules ordered nothing and their coverage is LOST rather than replaced |
| I-5 | FIXED. `ResourceBridgeMember`, `ResourceSysctl`, `ResourceDHCP` and `ResourceTunnel` are deleted from `pkg/plugin/rpc/types.go` and both alias files. `ResourceRelationSameResource` is deleted, and `resourceKey`, `opAddrIface` and `firstNonZeroUint16`, which had no other user, go with it. The tests that used them now spell their own kind and use `ResourceRelationAny`, which builds the same two-node cycles |
| I-1, I-2, I-3, I-6, N-1, N-2, N-3 | Open. Not routed |

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

Verified at the producer and found sound: A-1 (one `emitSectionApply` for both
routes), A-2 (phase-1 verify precedes the planner, `Verify` skips a coarse node),
A-3 (`initCallbackDefaults` registers no operation default), rollback reach for
both node kinds, `runningPeerSettings` reading the reactor's own peer, the two
gained derived edges with none lost, the blank-entry refusal, every deliverable
grep, and the QEMU discrimination walk. R-2 was walked over four config shapes
and NOT reproduced.
