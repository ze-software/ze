# 425 — Architecture Phase 0: System Boundaries (Umbrella)

## Context

Ze's internal architecture had unclear component boundaries. `plugin.Server` was a god object spanning 6 concerns (plugin lifecycle, event subscription, event dispatch, RPC routing, BGP hooks, startup coordination). Three unrelated things were called "Hub." The bus was not content-agnostic — it contained `BGPHooks` callbacks and typed event constants. BGP daemon was conflated with plugins. The original goal was to establish interface-defined boundaries between five components: Engine, Bus, ConfigProvider, PluginManager, and Subsystem.

A later revision reduced the model from 5 components to 4. The Bus was found to duplicate the stream event system in `internal/component/plugin/server/dispatch.go`: both provided in-process pub/sub with fan-out, and the stream system already supported schema-validated event types, DirectBridge zero-copy for internal plugins, and TLS delivery to external plugins. The Bus was the weaker of the two (no schema, no external participation, never fully wired). It was absorbed into the stream system via a new public `pkg/ze.EventBus` interface backed by `Server.Emit` and `Server.Subscribe`. The updated model is **4 components**: Engine, ConfigProvider, PluginManager, Subsystem.

## Decisions

- **Originally 5 components** (Engine, Bus, ConfigProvider, PluginManager, Subsystem), **now 4** after the Bus was absorbed into the stream system — each component extracted incrementally across 9 phases
- **Interfaces in `pkg/ze/`** — public so external plugins can depend on them, chosen over `internal/` interfaces
- **Bus was content-agnostic** (original) — payload `[]byte`, hierarchical `/` topics, prefix-based subscription. Replaced by namespaced events keyed by `(namespace, event-type)` strings in the stream system.
- **Subsystem ≠ Plugin** — BGP daemon is a subsystem (owns TCP/FSM), bgp-rib/rs/gr are plugins. Not a type hierarchy.
- **9 phases** (expanded from original 6) — phases 7-9 added for wiring subsystem, config provider, and plugin manager into the production startup path
- **Bus removal** — Decided during the config-transaction protocol work. The transaction orchestrator was migrated off the bus to the stream system (phases 1, 4a, 4b-i, 4b-ii of `spec-config-tx-protocol`), then the remaining bus consumers were migrated in a follow-up, and the standalone bus implementation was deleted. See also the config transaction protocol documentation.

## Consequences

- Any new subsystem (BMP, RPKI, telemetry) can be added by implementing `ze.Subsystem` and registering with Engine
- Plugins communicate through the `ze.EventBus` interface, backed by the stream system — no direct imports between plugin packages
- `internal/component/plugin/` has zero imports from `internal/component/bgp/` or any subsystem
- ConfigProvider is the single authority for config — editor, web UI, subsystems, plugins all use same interface
- DirectBridge optimization is invisible to the EventBus interface — in-process plugins dispatch through the bridge hot path, external plugins use the stream RPC path
- One pub/sub backbone (the stream event registry) instead of two; external plugin authors have a single `ze.EventBus` import

## Gotchas

- Phase 5 (Engine) was highest risk — new startup sequence changed component initialization order, required careful test coverage
- BGPHooks elimination (Phase 6) touched the most code — every event publishing path had to be redirected from callback injection to direct Bus publish
- The umbrella expanded from 6 to 9 phases because wiring components into the production startup path (phases 7-9) was substantial separate work from extracting the implementations (phases 2-6)
- `check-existing-patterns.sh` hook blocked idiomatic `func New()` for the Bus constructor — required `NewBus()` workaround
- The bus was never fully wired: only ~14 production call sites used it, most of which were unfinished. The migration to the stream system was mechanical because the API shapes mapped 1:1 (Publish → Emit, Subscribe → Subscribe, topic → namespace+event-type pair).

## Files

- `pkg/ze/` — 4 component interfaces (engine.go, config.go, plugin.go, subsystem.go) plus `eventbus.go` which is the replacement for the old `bus.go`
- `internal/component/plugin/server/engine_event.go` — `Server.Emit` / `Server.Subscribe` implementing `ze.EventBus`
- `internal/component/plugin/events.go` — namespace and event type registry (config, bgp, rib, sysrib, fib, interface)
- `internal/component/engine/engine.go` — Engine implementation (supervisor, startup/shutdown ordering)
- `internal/component/config/provider.go` — ConfigProvider implementation
- `internal/component/plugin/manager.go` — PluginManager implementation
- Child learned summaries: 323 through 328, 419 through 421, plus the config-transaction protocol learned summary
