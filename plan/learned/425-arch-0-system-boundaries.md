# 425 — Architecture Phase 0: System Boundaries (Umbrella)

## Context

Ze's internal architecture had unclear component boundaries. `plugin.Server` was a god object spanning 6 concerns (plugin lifecycle, event subscription, event dispatch, RPC routing, BGP hooks, startup coordination). Three unrelated things were called "Hub." The bus was not content-agnostic — it contained `BGPHooks` callbacks and typed event constants. BGP daemon was conflated with plugins. The goal was to establish interface-defined boundaries between five components: Engine, Bus, ConfigProvider, PluginManager, and Subsystem.

## Decisions

- **5 components** (Engine, Bus, ConfigProvider, PluginManager, Subsystem) over a monolithic refactor — each component extracted incrementally across 9 phases
- **Interfaces in `pkg/ze/`** — public so external plugins can depend on them, chosen over `internal/` interfaces
- **Bus is content-agnostic** — payload always `[]byte`, bus never type-asserts, chosen over typed event system. Like RabbitMQ/Kafka.
- **Topics hierarchical with `/`** — prefix-based subscription matching, chosen over flat namespaces or regex
- **Subsystem ≠ Plugin** — BGP daemon is a subsystem (owns TCP/FSM), bgp-rib/rs/gr are plugins (react to bus events). Not a type hierarchy.
- **9 phases** (expanded from original 6) — phases 7-9 added for wiring subsystem, config provider, and plugin manager into the production startup path

## Consequences

- Any new subsystem (BMP, RPKI, telemetry) can be added by implementing `ze.Subsystem` and registering with Engine
- Plugins communicate only through Bus — no direct imports between plugin packages
- `internal/component/plugin/` has zero imports from `internal/component/bgp/` or any subsystem
- ConfigProvider is the single authority for config — editor, web UI, subsystems, plugins all use same interface
- DirectBridge optimization is invisible to the Bus interface — in-process plugins get function calls, external get serialization
- `spec-iface-bus` (now split into `spec-iface-0-umbrella` through `spec-iface-4-advanced`) is unblocked

## Gotchas

- Phase 5 (Engine) was highest risk — new startup sequence changed component initialization order, required careful test coverage
- BGPHooks elimination (Phase 6) touched the most code — every event publishing path had to be redirected from callback injection to direct Bus publish
- The umbrella expanded from 6 to 9 phases because wiring components into the production startup path (phases 7-9) was substantial separate work from extracting the implementations (phases 2-6)
- `check-existing-patterns.sh` hook blocked idiomatic `func New()` for the Bus constructor — required `NewBus()` workaround

## Files

- `pkg/ze/` — all 5 component interfaces (engine.go, bus.go, config.go, plugin.go, subsystem.go)
- `internal/bus/bus.go` — Bus implementation (hierarchical topics, prefix matching, per-consumer delivery)
- `internal/component/engine/engine.go` — Engine implementation (supervisor, startup/shutdown ordering)
- `internal/component/config/provider.go` — ConfigProvider implementation
- `internal/component/plugin/manager.go` — PluginManager implementation
- Child learned summaries: 323 through 328, 419 through 421
