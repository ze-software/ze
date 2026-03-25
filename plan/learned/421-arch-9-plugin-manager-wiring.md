# 421 -- Arch-9: PluginManager Wiring

## Context

The Engine used a nop `stubPluginManager()`. The real `Manager` existed (built in arch-3) but was a stub. Plugin startup was entirely owned by `pluginserver.Server` — a god object doing process management, 5-stage protocol, event delivery, command dispatch, and more. The goal was to extract process lifecycle into PluginManager.

## Decisions

- **Two-phase startup** over single-phase — chosen because Engine starts plugins before subsystems, but Server (which runs the 5-stage protocol) is created inside the reactor (a subsystem). Phase 1 (spawn) doesn't need Server. Phase 2 (handshake) does.
- **ProcessSpawner interface** in `plugin/types.go` with `SpawnMore` + `GetProcessManager` — chosen over importing manager from server (would create coupling). Server type-asserts `GetProcessManager() any` to `*process.ProcessManager`.
- **`GetProcessManager()` returns `any`** — concrete type would create import cycle (plugin → process → plugin). Explicit error on type assertion failure prevents nil panic.
- **Deleted legacy `spawnProcessesDirect`** — no-layering rule. Server requires ProcessSpawner; no fallback path.
- **StartupHooks approach abandoned** — first attempt defined `RunPluginStartup(ctx)` on Server, but PluginManager.StartAll fires before Server exists. Two-phase solves the ordering problem.

## Consequences

- Zero nop stubs remain in Engine — Bus, ConfigProvider, PluginManager all real.
- PluginManager owns process lifecycle (spawn, stop). Server owns protocol wiring (5-stage, subscriptions, commands, DirectBridge).
- `SpawnMore` enables auto-load: Server discovers unclaimed families/events/send-types after Stage 1, calls `pm.SpawnMore()` to create new processes.
- TLS acceptor shared across spawn phases via `ensureAcceptor` — created once, reused.
- Deep review found and fixed: nil procManager panic, invisible auto-loaded plugins, Phase 1 deadlock on failure.

## Gotchas

- `any` return type on `GetProcessManager()` is a compromise forced by import cycle — document why and add explicit error on assertion failure.
- `runPluginPhase` is called up to 4 times (explicit, families, events, send-types) — each call overwrites `s.procManager`. Manager accumulates all ProcessManagers in `procManagers` slice.
- Phase 1 failure in `runPluginStartup` didn't call `signalStartupComplete()` — pre-existing bug that would deadlock reactor. Fixed.
- Pre-existing concurrency issues found but not fixed (separate scope): processCount race, procManager race, goroutine leaks, map alias. Documented in `CONCURRENCY-ISSUES.md`.

## Files

- `internal/component/plugin/types.go` — ProcessSpawner interface
- `internal/component/plugin/manager/manager.go` — real Manager with spawn/stop/TLS
- `internal/component/plugin/server/server.go` — spawner field, SetProcessSpawner
- `internal/component/plugin/server/startup.go` — uses spawner, deleted legacy path
- `internal/component/bgp/reactor/reactor.go` — processSpawner field, wires to Server
- `cmd/ze/hub/main.go` — real Manager, removed nopPluginManager
- `docs/architecture/plugin-manager-wiring.md` — architecture doc with Mermaid diagrams
