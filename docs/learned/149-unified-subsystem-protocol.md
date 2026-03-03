# 149 — Unified Subsystem Protocol

## Objective

Eliminate the central `RegisterDefaultHandlers()` that required knowledge of all subsystems, enabling self-registration so adding a new command group requires no edits to central files.

## Decisions

- Rejected the full 5-stage async protocol approach from the spec in favor of `init()` self-registration. The async protocol (channels, coordinator, 5-stage messages for internal handlers) was over-engineered for synchronous in-process function calls.
- External plugins need async protocol (separate processes, stdin/stdout). Internal handlers are just functions in the same process — adding goroutines/channels for sync function calls creates complexity with no benefit.
- Deleted `internal/subsystem/` and `internal/subsystems/` packages entirely after recognizing they duplicated the existing plugin infrastructure for a problem that `init()` solves in ~5 lines.

## Patterns

- Each handler file registers itself via `init()` into a global registry; `LoadBuiltins(d)` loads all registered handlers into a dispatcher. No central file needs to list or import subsystems.
- `RegisterDefaultHandlers(d)` remains as a backward-compatible alias for `LoadBuiltins(d)`.

## Gotchas

- The spec's elaborate async protocol design was produced before recognizing that "uniformity with external plugins" does not require mimicking their transport mechanism when the communication is in-process. Only use async protocol for things that are actually async.

## Files

- `internal/plugin/command.go` — added global builtin registry, `RegisterBuiltin()`, `LoadBuiltins()`
- All handler files (`handler.go`, `cache.go`, `route.go`, `bgp.go`, etc.) — converted to `init()` self-registration
