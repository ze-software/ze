# 323 — Architecture Phase 1: Boundary Interfaces

## Objective

Create the `pkg/ze/` package with five boundary interfaces (Bus, Subsystem, ConfigProvider, PluginManager, Engine) as pure definitions — no existing code changes, no behavior changes. First step of the system-boundaries architectural refactor.

## Decisions

- Interfaces live in `pkg/ze/` (public package) so external plugins can depend on them without importing `internal/`
- Bus payload is always `[]byte` — bus never type-asserts, content-agnostic like RabbitMQ/Kafka
- Topics use hierarchical `/` separator with prefix-based subscription matching
- Five components: Engine (supervisor), Bus (pub/sub), ConfigProvider, PluginManager, Subsystem — Subsystem is NOT a Plugin (BGP daemon owns TCP/FSM; bgp-rib/rs/gr are plugins)
- `pkg/ze/` has zero imports from `internal/` — verified as hard constraint

## Patterns

- Interface satisfaction verified with compile-time type assertions in test file — catches method signature drift without running any logic
- Phase approach: define interfaces first, build implementations in later phases, wire into existing code in final phase — avoids big-bang refactors

## Gotchas

- None. Pure addition of new files.

## Files

- `pkg/ze/bus.go` — Bus, Consumer, Event, Topic, Subscription
- `pkg/ze/config.go` — ConfigProvider, ConfigChange
- `pkg/ze/engine.go` — Engine
- `pkg/ze/plugin.go` — PluginManager, PluginProcess, Capability
- `pkg/ze/subsystem.go` — Subsystem
- `pkg/ze/ze_test.go` — interface satisfaction tests
