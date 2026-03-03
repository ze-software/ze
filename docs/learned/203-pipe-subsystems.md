# 203 — Pipe Subsystems

## Objective

Run subsystems as forked processes communicating over pipes, replacing `init()` self-registration.

## Decisions

- Single `ze-subsystem` binary with `--mode` flag, not separate binaries per subsystem — one artifact to deploy; mode selected at invocation time.
- Numeric serial (plugin→engine direction) vs alpha serial (engine→plugin direction) for bidirectional multiplexing — distinguishes in-flight requests by direction without an extra framing field.

## Patterns

- Same pipe + goroutine pattern as plugins; subsystems use the same transport layer as plugins.

## Gotchas

- None.

## Files

- `cmd/ze-subsystem/` — single binary, --mode dispatch
- `internal/plugin/process/` — subsystem fork + pipe management
