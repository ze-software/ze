# 198 — Plugin Invocation

## Objective

Implement three plugin invocation modes: Fork (`name`/`/path`), Internal (`ze.name`, goroutine + socket pair), and Direct (`ze-name`, synchronous in-process call).

## Decisions

- Internal mode (`ze.name`) uses a decode-only function (not the full 5-stage startup protocol) to avoid deadlock — if the engine calls into the plugin synchronously during plugin startup, the plugin cannot complete its own startup while waiting for the engine.
- Single binary with `--mode` flag (not separate binaries) — one artifact to deploy, mode selected at invocation time.

## Patterns

- `DirectBridge` connects internal plugins via `net.Pipe()` — same socket interface as Fork mode, zero serialization overhead.
- Numeric serial (plugin→engine direction) differentiates from alpha serial (engine→plugin direction) for bidirectional multiplexing.

## Gotchas

- Full 5-stage startup protocol in Internal mode deadlocks: engine blocks waiting for plugin `ready`, plugin blocks waiting for engine to send config. Decode-only path skips startup entirely.

## Files

- `internal/plugin/process/` — Fork, Internal, Direct mode dispatch
- `internal/plugin/bridge/` — DirectBridge
- `internal/plugin/cli/plugin_common.go` — mode flag handling
