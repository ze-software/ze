---
paths:
  - "**/*.go"
---

# Go Coding Standards

Rationale: `.claude/rationale/go-standards.md`

## Required

- Go 1.21+ features (slog, generics)
- `golangci-lint` must pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context as first param: `context.Context`
- Fail-early: propagate parse/config errors immediately, never silently default

## Logging: `log/slog` only

- Use `slogutil.Logger("subsystem")` for engine code
- Use `slogutil.PluginLogger("name", level)` for plugins
- Per-subsystem: `ze.log.<path>=<level>` env vars (hierarchical, most-specific wins)
- Levels: `disabled`, `debug`, `info`, `warn`, `err`
- Config: `environment { log { level warn; bgp.routes debug; } }`
- Priority: CLI flag > env var > config > default (WARN)
- **Debug logging is permanent** — use `logger.Debug()`, never `fmt.Printf` for debugging

## Forbidden

- `panic()` for error handling
- `f, _ := func()` (ignoring errors)
- Global mutable state
- `init()` except registry patterns
- `log.Printf` (legacy log package)
- Silent defaults: `if x == "" { x = "0.0.0.0/0" }`
