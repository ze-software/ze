# 327 — Arch-5: Engine Supervisor Implementation

## Objective

Build the `ze.Engine` supervisor satisfying the `ze.Engine` interface, composing Bus, ConfigProvider, and PluginManager into a generic lifecycle manager for subsystems.

## Decisions

- `NewEngine(bus, config, plugins)` takes pre-built components — Engine is a compositor, not a factory; caller owns construction.
- Subsystems started in registration order, stopped in reverse; failure during start triggers rollback of already-started subsystems.
- Stop is idempotent (second call returns nil) and "first error wins" but still stops all subsystems.
- Hub.Orchestrator replacement explicitly deferred — Engine stands alone, wiring happens in a follow-up; avoids deep coupling with process forking and SubsystemManager.

## Patterns

- Engine type exported directly (not `EngineImpl`) since it's the canonical implementation — no reason to hide it.
- Thread safety via `sync.RWMutex` on subsystem slice; `RegisterSubsystem` after `Start` returns an error.
- Constructor injection pattern: caller builds all sub-components before passing to Engine.

## Gotchas

None.

## Files

- `internal/engine/engine.go` — Engine implementation (140 lines), satisfies `pkg/ze/engine.go` interface
- `internal/engine/engine_test.go` — 14 unit tests covering full lifecycle
