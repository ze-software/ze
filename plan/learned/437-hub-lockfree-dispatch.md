# 437 -- Hub Lock-Free Dispatch

## Context

Three registries (SchemaRegistry, SubsystemManager, CommandRegistry) protect their maps with `sync.RWMutex`. After startup, no goroutine ever writes to these maps -- all mutations happen during the 5-stage plugin protocol. The read-locks on the dispatch hot path were technically uncontended but obscured the immutability invariant. The goal was to make this invariant explicit via freeze-after-init, removing locks from both dispatch paths (Orchestrator's Hub.RouteCommand and Server's Dispatcher.Dispatch).

## Decisions

- **Freeze creates a shallow-copied map snapshot stored via `atomic.Pointer`** over deep-copying values. The pointed-to structs (`*Schema`, `*SubsystemHandler`, `*RegisteredCommand`) are shared between frozen and mutable maps. This is safe because schemas are immutable after registration, and handlers have their own per-instance locks for mutable state.
- **Only hot-path methods use the frozen path** (FindHandler, Get, Lookup) over freezing all read methods. CLI/query methods (ListRPCs, Complete, All) stay on RLock because they are not on the dispatch hot path.
- **Pre-freeze fallback to RLock** over requiring Freeze before any read. During startup, internal calls may invoke FindHandler before Freeze. The fallback makes this safe without caller awareness of freeze state.
- **Unregister republishes a new snapshot** over invalidating the frozen pointer. Readers always see a consistent snapshot -- either the old one or the new one, never a nil that forces them back to the lock path.

## Consequences

- Dispatch hot path is lock-free after startup. Future concurrent dispatch optimizations can assume no lock contention on registry lookups.
- Every post-freeze mutation path (Unregister, UnregisterAll) must republish the frozen snapshot. Forgetting this was the main bug caught during deep review -- CommandRegistry initially missed this while SubsystemManager had it.
- SchemaRegistry has no Unregister (schemas are permanent), so it needs no republish logic.

## Gotchas

- **CommandRegistry.Unregister/UnregisterAll must republish the frozen snapshot.** The initial implementation missed this because SubsystemManager.Unregister was the model, and CommandRegistry was done later without replicating the republish. Deep review caught it from 5 independent angles.
- **SubsystemManager.AllCommands had a pre-existing data race:** it read `handler.commands` directly for preallocation instead of using the lock-safe `handler.Commands()` method. Fixed by collecting via Commands() first.
- **Two distinct dispatch paths exist** that were not obvious from the original spec. Hub.RouteCommand (Orchestrator/config) and Dispatcher.Dispatch (Server/CLI/API) use different registries. The spec had to be updated during design to cover both.

## Files

- `internal/component/plugin/server/schema.go` -- frozenSchema, Freeze(), findHandlerIn()
- `internal/component/plugin/server/subsystem.go` -- frozenSubsystems, Freeze(), findHandlerByCommand(), updated Get/FindHandler/Unregister
- `internal/component/plugin/server/command_registry.go` -- frozenCommands, Freeze(), republishFrozen(), updated Lookup/Unregister/UnregisterAll
- `internal/component/hub/hub.go` -- Freeze() calls in Orchestrator.Start()
- `internal/component/plugin/server/startup.go` -- Freeze() calls in signalStartupComplete()
- `docs/architecture/hub-architecture.md` -- documented freeze-after-init
