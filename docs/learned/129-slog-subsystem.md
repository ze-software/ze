# 129 — Per-Subsystem Logging (slogutil)

## Objective

Implement per-subsystem logging control using `slog`, with logging disabled by default. Engine subsystems enable via `ze.bgp.log.<subsystem>=<level>` env vars; plugins enable via `--log-level` CLI flag. Plugins always write to stderr (stdout reserved for protocol).

## Decisions

- Each subsystem gets its own logger instance (not `slog.SetDefault`) to allow multiple subsystems in the same process with independent enable/disable.
- Chose native `log/syslog` over `github.com/samber/slog-syslog/v2` — no go.mod changes needed.
- Logger variable names vary by file (`logger`, `coordinatorLogger`, `filterLogger`) to avoid name conflicts within the same package.
- `DiscardLogger()` exported from slogutil so plugins import it instead of duplicating `discardHandler`.
- Env vars support both dot notation (`ze.bgp.log.server`) and underscore notation (`ze_bgp_log_server`); dot takes priority.

## Patterns

- Plugin stderr relay is wired in `internal/plugin/process.go:relayStderr()` — reads lines from plugin stderr and relays via stderrLogger.
- `ParseLogLine()` in `parse.go` extracts level/msg/attrs from slog text format, falling back to raw line for malformed input (panics, raw errors).

## Gotchas

- `ze.bgp.log.plugin=enabled` served dual purpose: enable stderr relay AND act as log level. "enabled" is not a valid log level for `slogutil.Logger()`, causing discard logger. Known design issue.

## Files

- `internal/slogutil/slogutil.go` — `Logger()`, `LoggerWithLevel()`, `LoggerWithOutput()`, `IsPluginRelayEnabled()`, `discardHandler`
- `internal/slogutil/syslog.go` — syslog backend
- `internal/slogutil/parse.go` — `ParseLogLine()` for plugin stderr relay
