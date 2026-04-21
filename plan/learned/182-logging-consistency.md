# 182 — Logging Consistency

## Objective

Unify Ze's logging system: hierarchical env-var lookup, plugin processes obey env vars,
migrate printf-style `internal/trace/` package to slog, add config-file log settings.

## Decisions

- Subsystem names follow package path without `internal/component/plugin/` prefix: `bgp.reactor.peer`, `bgp.routes`, `config`, etc.
- Hierarchical resolution: `ze.log.bgp.fsm` → `ze.log.bgp` → `ze.log`; dot notation beats underscore at same level.
- `LazyLogger()` wraps `sync.Once` — defers logger creation until first use so config-file settings take effect before any logger initialises.
- Config-file `environment { log { } }` block calls `os.Setenv()` early in `main()` so the existing env-var lookup requires no change.
- `RelayLevel()` replaces `IsPluginRelayEnabled()` — returns `(slog.Level, bool)` for level-aware relay filtering.
- `PluginLogger(subsystem, cliLevel)` — CLI flag wins if non-"disabled", otherwise env-var hierarchy.

## Patterns

- `routesLogger` declared in both `reactor.go` and `peer.go` because peer.go contains most route operations.
- `ApplyLogConfig()` maps config keys to `os.Setenv("ze.log.<key>", value)` — no new types needed.
- Trace category → slog subsystem: `trace.Routes` → `bgp.routes`, `trace.FSM` → `bgp.reactor.peer`.

## Gotchas

- Package-level `var logger = slogutil.Logger(...)` runs before `main()` reads config; without `LazyLogger()`, config-file log settings were ignored.
- `routesLogger` needed in peer.go, not just reactor.go — discovered during trace migration.

## Files

- `internal/slogutil/slogutil.go` — hierarchical lookup, `PluginLogger`, `LazyLogger`, `ApplyLogConfig`
- `internal/slogutil/slogutil_test.go` — 35 tests including 15 new
- `internal/component/plugin/` (server, process, filter, subscribe, coordinator) — subsystem names updated
- `internal/component/bgp/reactor/` (reactor, peer, session) — trace replaced, loggers added
- `internal/component/config/loader.go` — trace replaced, configLogger added
- `internal/trace/` — deleted
- `ai/rules/go-standards.md` — logging section rewritten
