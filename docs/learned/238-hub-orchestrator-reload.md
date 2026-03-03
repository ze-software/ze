# 238 — Hub Orchestrator Reload

## Objective

Wire SIGHUP config reload in the hub orchestrator path: re-read config, diff plugin definitions, stop removed plugins, start added plugins, forward SIGHUP to children.

## Decisions

- Hub reload deliberately simpler than BGP in-process reload — children self-reload via their own `ReloadFromDisk()`.
- Any reload failure triggers orchestrator shutdown (fail-safe over degraded operation).
- `Reload()` lives in `reload.go`, not `hub.go` (single responsibility).

## Patterns

- Hub orchestrator reload pattern: diff old vs new plugin defs → stop removed → start added → forward signal.
- Children are responsible for their own reload logic; orchestrator only manages lifecycle.

## Gotchas

- Hub reload is not symmetric with BGP reload — BGP re-establishes sessions; hub children self-reload.

## Files

- `internal/hub/reload.go` — hub orchestrator reload logic
