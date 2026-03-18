---
paths:
  - "**/*.go"
---

# Go Standards

Rationale: `.claude/rationale/go-standards.md`

## Required

- Go 1.21+ features (slog, generics)
- `golangci-lint` must pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context as first param: `context.Context`
- Fail-early: propagate parse/config errors immediately, never silently default

## Logging: `log/slog` only

- Engine: `slogutil.Logger("subsystem")`
- Plugins: `slogutil.PluginLogger("name", level)`
- Per-subsystem: `ze.log.<path>=<level>` env vars (hierarchical, most-specific wins)
- Levels: `disabled`, `debug`, `info`, `warn`, `err`
- Config: `environment { log { level warn; bgp.routes debug; } }`
- Priority: CLI flag > env var > config > default (WARN)
- Debug logging is permanent — `logger.Debug()`, never `fmt.Printf`

## Dependencies

Never add new third-party imports (not already in `go.mod`) without asking the user first.

## Environment Variables: `internal/core/env` only

**BLOCKING:** All Ze environment variable access MUST use `env.Get()` / `env.Set()` or typed helpers. Never use `os.Getenv()` or `os.Setenv()` for Ze-specific vars.

| Getters | Use |
|---------|-----|
| `env.Get("ze.foo.bar")` | String lookup (case-insensitive, dot/underscore agnostic) |
| `env.GetInt("ze.foo", 0)` | Integer with default |
| `env.GetInt64("ze.foo", 0)` | Int64 with default |
| `env.GetBool("ze.foo", false)` | Boolean (true/false/1/0) with default |
| `env.IsEnabled("ze.foo")` | Enabling check (1/true/yes/on/enable/enabled) |
| `env.GetDuration("ze.foo", 5*time.Second)` | Duration with default |

| Setters | Use |
|---------|-----|
| `env.Set("ze.foo", "val")` | String (updates cache + os env) |
| `env.SetInt("ze.foo", 42)` | Integer |
| `env.SetBool("ze.foo", true)` | Boolean ("true"/"false") |

**Cache:** Built once from `os.Environ()` on first `Get()`. `Set*()` updates both cache and os env. Tests that use `os.Setenv` directly must call `env.ResetCache()`.

**Registration required:** Every env var must be registered via package-level `var _ = env.MustRegister(...)`. Calling `env.Get()` with an unregistered key aborts the process.

**`os.Getenv` IS OK for:** System env vars (`HOME`, `PATH`, `XDG_*`, `NO_COLOR`, `USER`, `SSH_*`).

## Forbidden

- `panic()` for error handling
- `f, _ := func()` (ignoring errors)
- Global mutable state
- `init()` except registry patterns
- `log.Printf` (legacy log package)
- Silent defaults: `if x == "" { x = "0.0.0.0/0" }`
- `os.Getenv("ZE_*")` or `os.Getenv("ze.*")` -- use `env.Var()` instead
