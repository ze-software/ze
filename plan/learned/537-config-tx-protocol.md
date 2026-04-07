# 537 -- Config Transaction Protocol

## Context

Ze's config reload was a sequential RPC loop in `plugin/server/reload.go`: verify plugins one at a time, then apply concurrently with no rollback. A failed apply left the engine in a partial state with no recovery. There was no transaction exclusion, no plugin-estimated deadlines, no way for a plugin to declare interest in another plugin's config diffs, and no journaled rollback for plugins that needed to undo changes safely.

The goal was to replace the RPC loop with a transaction protocol that supports rollback, transaction exclusion, dependency-aware deadlines, and an SDK journal for plugin-side undo. The original spec was written against `pkg/ze.Bus`. After several phases landed, the spec author discovered the bus was a half-finished duplicate of the existing stream event system in `internal/component/plugin/server/dispatch.go`: both provided in-process pub/sub fan-out, and the stream system was the more capable of the two (schema validation via the event-type registry, DirectBridge zero-copy for internal plugins, TLS delivery to external plugins). The protocol pivoted from the bus to the stream system, and the bus was scheduled for removal.

## Decisions

- **Transaction orchestrator as a typed coordinator over an `EventGateway` interface** (over a god object that takes the whole `Server`). The interface has two methods (`EmitConfigEvent`, `SubscribeConfigEvent`) and tests inject a `testGateway` fake without instantiating any plugin server. Production wires `ConfigEventGateway` (in `internal/component/plugin/server/engine_event_gateway.go`) which delegates to `Server.EmitEngineEvent` and `Server.SubscribeEngineEvent`.

- **Per-plugin event types for engine -> plugin** (`verify-<plugin>`, `apply-<plugin>`) over a single broadcast event with plugin filtering done locally. Each plugin subscribes only to its own event type and receives only its filtered diffs. Per-plugin types are registered dynamically when plugins start. Reserved plugin names (`ok`, `failed`, `abort`) are blocked at registration to prevent collision with the broadcast ack types.

- **Broadcast event types for plugin -> engine** (`verify-ok`, `apply-failed`, etc.) over per-plugin ack channels. The orchestrator subscribes to the broadcast types once and demultiplexes by the `Plugin` field in each ack payload. This keeps the registry small and avoids dynamic registration churn during transactions.

- **Reverse-tier rollback ordering using `registry.TopologicalTiers`** over flat ack collection. Plugins in the deepest dependency tier (most dependents) have their rollback acks drained first; broken plugin restarts happen between tiers so a dependency never starts tearing down state while a dependent is mid-restart. The orchestrator buffers acks that arrive from a tier not yet being drained.

- **Dependency-graph deadline as `sum_k(max_{p in tier k}(budget(p)))`** over the previous flat `max(budget for all plugins)`. Plugins within a tier run concurrently so the tier cost is the max; tiers are serialized so tier costs are summed. This gives the critical path through the dependency graph and prevents premature timeouts when a chain of dependent plugins exceeds the longest single budget.

- **`tierFn` as a package-level overridable variable in the transaction package** (defaulting to `registry.TopologicalTiers`) over passing a registry handle through every constructor. Tests inject a fake tier function to control the tier shape without mutating the global plugin registry. Unregistered plugins (the common test case) collapse into a single tier, so the new code degenerates to the previous flat behaviour and existing tests still pass.

- **Bus removal: replace `pkg/ze.Bus` with `pkg/ze.EventBus`** over keeping the bus interface as a thin wrapper. The two pub/sub systems provided overlapping functionality but the bus was never fully wired and only had ~14 production call sites. The mapping between bus topics and stream `(namespace, event-type)` pairs was natural (split on `/`). The migration was mechanical and left the stream system as the single backbone -- although only chain 1 (RIB cascade) landed in this session; chains 2-4 are deferred (see Consequences and Files sections).

- **`Server.Emit` and `Server.Subscribe` as wrapper methods** over a separate adapter type. The Server already had `EmitEngineEvent` and `SubscribeEngineEvent` with matching shapes; the public wrapper methods let `Server` satisfy `ze.EventBus` directly and the existing internal call sites stay unchanged.

- **`ConfigureEventBus` callback alongside the deprecated `ConfigureBus`** over a hard cut. Both callbacks fire if both are set, allowing each consumer to migrate independently. The deprecated callback's staticcheck warning at the legacy dispatch site in `inprocess.go` is suppressed with `//nolint:staticcheck` markers carrying the migration rationale; the dispatch and the markers are deleted together by chain 4.

- **Bus migration in chain-bounded commits** over a single atomic landing. Chain 1 = RIB cascade (rib_bestchange + sysrib + fibkernel + fibp4 + tests). Chain 2 = interface monitor + BGP reactor + reactor_iface + reactor_bus deletion. Chain 3 = DHCP + fibkernel external monitor + BGP server EOR. Chain 4 = delete `pkg/ze/bus.go`, `internal/component/bus/`, `Registration.ConfigureBus`, the `inprocess.go` dispatch, and the `bus.NewBus()` call in `cmd/ze/hub/main.go`. Each chain compiles and ze-verifies before the next; only chain 1 landed in this session.

- **Phase 7 (`reload.go` wired to `TxCoordinator.Execute`) landed via an engine-side RPC bridge** over extending the plugin SDK with stream-based config subscription. The bridge (`internal/component/plugin/server/config_tx_bridge.go`) subscribes to the orchestrator's `verify-<plugin>` / `apply-<plugin>` / `rollback` engine events and translates each into the existing `conn.SendConfigVerify` / `SendConfigApply` / `SendConfigRollback` RPC calls, then emits `verify-ok` / `verify-failed` / `apply-ok` / `apply-failed` / `rollback-ok` acks back on the stream. This preserves the orchestrator's state machine (tiered deadlines, reverse-tier rollback, broken plugin restart) without requiring SDK changes: plugins keep their `OnConfigVerify` / `OnConfigApply` / `OnConfigRollback` callbacks exactly as they are. `reload.go` now calls `runTxCoordinator` via `reload_tx.go` which computes the participant list from affected plugins and the diff map from the existing `buildDiffSections` helper.

- **AC-17 (external plugin over the hub transport) implemented through the same RPC bridge** over a separate Python-over-TLS harness. External plugins (e.g. `ze plugin bgp-watchdog`) connect back via MuxConn, and the bridge dispatches RPCs over that connection verbatim. `test/reload/test-tx-protocol-external-plugin.ci` launches bgp-watchdog as an external plugin, triggers a reload via SIGHUP, and proves the transaction reaches the external process through the plugin hub transport with no special-casing.

- **Wildcard config root expansion lives in `buildTxInputs`** over teaching `TxCoordinator.filterDiffs` about `"*"`. The orchestrator does exact-match lookups on `ConfigRoots`, so the wiring layer expands a `["*"]` declaration into the concrete list of roots present in the current diff before the participant reaches the coordinator. Keeps the orchestrator simple and matches the legacy reload path's wildcard semantics.

- **Apply-rollback-on-failure replaces the legacy partial-apply+SetConfigTree branch** over keeping the old `reactor.SetConfigTree(newTree)` call when apply errors. `TestReloadApplyErrorReturned` was updated to assert the new semantic: on any apply failure the orchestrator rolls back and the reactor config tree stays at the old state. This matches the spec's transactional guarantee and is the whole point of introducing the coordinator.

## Consequences

- One pub/sub backbone in the engine WILL be the stream event system once chains 2-4 land. Today, after chain 1, the engine has both backbones in parallel: the bus (used by interface monitor, BGP reactor, BGP server, ifacedhcp, fibkernel external monitor) and the stream system (used by the RIB cascade, the orchestrator, and all migrated consumers). The `pkg/ze.EventBus` interface is the public API for both internal and external plugin authors going forward.
- The orchestrator is independently testable: `EventGateway` lets unit tests run the full state machine without instantiating any plugin server, so the 17 transaction tests cover verify, apply, rollback, timeouts, broken plugin restart, budget updates, per-plugin filtering, WantsConfig delivery, no-op exclusion, reverse-tier ordering, dependency-graph deadline, and the tier-cycle fallback.
- Dependency-aware deadlines mean a chain like `sysrib (5s) -> fib-kernel (3s)` correctly gets 8s on the critical path, where the previous flat formula returned 5s and would have timed fib-kernel out.
- Reverse-tier rollback enables safe broken-plugin restart: a broken `fib-kernel` is restarted before `sysrib` starts tearing down its routes, so the kernel route table is consistent throughout the rollback.
- Anyone adding a new internal component now has a single import (`pkg/ze.EventBus`) and one set of constants (`internal/component/plugin/events.go` namespaces) for cross-component events. There is no second pub/sub system to learn or maintain.
- The `arch-0` learned summary moves from 5 components to 4: Engine, ConfigProvider, PluginManager, Subsystem.
- Plugin SDK keeps its RPC callbacks for `OnConfigVerify` / `OnConfigApply` / `OnConfigRollback`. The engine-side RPC bridge makes stream-based orchestration compatible with the existing SDK, so external plugin authors face no API change when upgrading to the transaction protocol.
- `reload.go`'s crash-handling checkpoints (verify-phase conn==nil, pre-apply alive check, apply-phase conn==nil) are absorbed into the bridge: a missing process or closed connection surfaces as `verify-failed` / `apply-failed` / `rollback-ok CodeBroken` acks that the orchestrator reacts to through its normal state machine.

## Gotchas

- The `EventVerifyFor("ok")` and `EventApplyFor("ok")` strings would collide with the broadcast ack `verify-ok` and `apply-ok`. `ValidatePluginName` rejects the reserved set (`ok`, `failed`, `abort`). The orchestrator constructor calls it for every participant; missing this check produces a silent collision that breaks ack routing.
- The orchestrator's rollback ack channel is sized to `activeCount`. A plugin that sends more acks than expected (duplicate, retry, malicious) is dropped with a warning rather than blocking the engine handler dispatch loop. The dispatch path runs synchronously inside the publisher's goroutine, so a blocked send would block the publisher.
- `TopologicalTiers` returns an error on a dependency cycle. The orchestrator falls back to a single tier on error, logs a warning, and continues; the alternative (failing the transaction) would make a registry bug catastrophic. Same fallback applies in `computeTieredDeadline`.
- The test fakes (`testGateway`) skip the production stream system entirely. Tests that work with the fake do not exercise the validation in `Server.EmitEngineEvent` (which rejects unknown event types). This is intentional -- testGateway accepts any event type so tests stay focused on orchestrator logic. The validation is covered by `internal/component/plugin/events_test.go` and the production `engine_event_gateway.go` integration.
- The bus migration was tempting to do as a thin shim implementing `ze.Bus` over `Server.Emit`. This was rejected because the bus had prefix-based subscription matching that the stream system does not support; reproducing the prefix matching inside a shim would have meant maintaining a parallel subscription router with cycle-detection, exactly the complexity the migration was meant to remove. Migrating each call site to the typed `(namespace, event-type)` API was simpler and removed more code than it added.

- The original bus migration plan was dispatched to a sub-agent in an isolated worktree. The sub-agent completed chain 1 (vet + race tests green for sysrib, fibkernel, fibp4, rib in the worktree) but was blocked by a staticcheck deprecation warning in `inprocess.go`: every call to the now-`Deprecated:`-tagged `ConfigureBus` triggered SA1019, including the legacy dispatch path that intentionally stays during migration. The fix was to add `//nolint:staticcheck` markers on the two legacy lines with a comment explaining that the deprecation IS the migration plan. After the suppression landed in the orchestrator's main repo and the agent's chain 1 work was salvaged via `git cherry-pick` from the worktree branch, chain 1 was unblocked. Chains 2-4 stayed in their pre-migration state because the agent did not get to them before stopping.
- `tierFn` is a package-level variable so tests can override it. This is a global with `-race` implications if two tests in the same package mutate it concurrently. The package-level helper `withTierFn(t, fn)` uses `t.Cleanup` to restore the previous value; tests should not run with the package-level var mutated outside this helper.

## Files

### Foundations and orchestrator
- `pkg/ze/eventbus.go` -- public `EventBus` interface (`Emit`, `Subscribe`)
- `internal/component/plugin/server/engine_event.go` -- `Server.Emit` and `Server.Subscribe` wrapper methods
- `internal/component/plugin/events.go` -- namespace constants (`config`, `bgp`, `rib`, `sysrib`, `fib`, `interface`) and event-type validation maps
- `internal/component/plugin/registry/registry.go` -- `Registration.ConfigureEventBus`, `SetEventBus`, `GetEventBus`
- `internal/component/plugin/inprocess.go` -- invokes `ConfigureEventBus` for in-process plugins
- `cmd/ze/hub/main.go` -- registers the plugin server as the `EventBus` instance after `NewServer`

### Phase 7 reload wiring
- `internal/component/plugin/server/reload.go` -- calls `runTxCoordinator` instead of the hand-rolled verify/apply loop
- `internal/component/plugin/server/reload_tx.go` -- `runTxCoordinator`, `buildTxInputs`, `expandWildcardRoots`, `restartPluginFn`, `txResultToError`
- `internal/component/plugin/server/config_tx_bridge.go` -- engine-side RPC bridge translating `verify-<plugin>` / `apply-<plugin>` / `rollback` events into plugin RPCs
- `internal/component/plugin/ipc/rpc.go` -- `SendConfigRollback` helper
- `internal/component/plugin/server/reload_test.go` -- `TestReloadTxCoordinatorRollback` exercises the bridge + orchestrator rollback path; `TestReloadApplyErrorReturned` updated to the new "no partial commit" semantic
- `test/reload/test-tx-protocol-external-plugin.ci` -- AC-17: external bgp-watchdog plugin participates in a reload transaction via the hub transport

### Transaction package
- `internal/component/config/transaction/orchestrator.go` -- `TxCoordinator`, state machine, deadline computation, ack collection, reverse-tier rollback, broken plugin restart
- `internal/component/config/transaction/gateway.go` -- `EventGateway` interface
- `internal/component/config/transaction/topics.go` -- event-type constants, `EventVerifyFor`/`EventApplyFor` helpers, `ValidatePluginName`, `ReservedPluginNames`
- `internal/component/config/transaction/types.go` -- payload structs (`VerifyEvent`, `ApplyEvent`, `RollbackEvent`, ack types, `DiffSection`, `AppliedEvent`)
- `internal/component/config/transaction/orchestrator_test.go` -- 17 unit tests including the three new tier-aware tests
- `internal/component/plugin/server/engine_event_gateway.go` -- production adapter satisfying `EventGateway`

### SDK
- `pkg/plugin/sdk/journal.go`, `pkg/plugin/sdk/journal_test.go` -- record/rollback/discard journal
- `pkg/plugin/sdk/sdk_callbacks.go` -- `OnConfigVerify`, `OnConfigApply`, `OnConfigRollback`
- `pkg/plugin/rpc/types.go` -- registration fields `WantsConfig`, `VerifyBudget`, `ApplyBudget`

### Documentation
- `docs/architecture/config/transaction-protocol.md` -- rewritten in stream-system terms
- `plan/learned/425-arch-0-system-boundaries.md` -- updated from 5 to 4 components, bus absorption recorded
