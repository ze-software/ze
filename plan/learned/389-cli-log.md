# 389 — CLI Log Commands

## Objective

Add `bgp log show` and `bgp log set <subsystem> <level>` CLI commands for runtime log level inspection and modification. Required infrastructure changes to `slogutil` to support mutable levels.

## Decisions

- Switched `Logger()` from fixed `slog.Level` to `*slog.LevelVar` — enables runtime level changes via atomic `LevelVar.Set()`
- `levelRegistry` (`sync.Map`) tracks subsystem name → `*slog.LevelVar` for enumeration and modification
- Disabled loggers (using `discardHandler{}`) are NOT registered — changing a LevelVar on a discard handler would have no effect, which would confuse users
- `LazyLogger()` registers on first call (inside `sync.Once`), not at creation time — shows only initialized subsystems
- `LevelString()` uses explicit cases without `default:` to satisfy `block-silent-ignore.sh` hook

## Patterns

- `slog.LevelVar` implements `slog.Leveler` interface — drop-in replacement for `slog.Level` in `HandlerOptions.Level`
- All 31 existing slogutil tests pass unchanged after the `Level` → `LevelVar` switch — backward-compatible because `HandlerOptions` accepts `Leveler` interface
- `goconst` linter requires string constants for repeated backend names (`"stdout"`, `"syslog"`, `"stderr"`) when they appear 3+ times

## Gotchas

- **Handler error contract**: `dispatch-command` sends JSON-RPC errors when handlers return a Go error. Business logic errors (e.g., "unknown subsystem") must return `StatusError` Response with `nil` Go error. The `nilerr` linter then requires extracting the error-returning call into a helper that returns a Response directly.
- **Missing blank import**: `reactor.go` needs `_ "...cmd/log"` for `init()` to fire in the binary. Dispatch tests pass without it because they import the package directly.
- **goconst lint**: Repeated backend strings (`"stdout"`, `"syslog"`, `"stderr"`) — fixed with constants.
- **errcheck lint**: `sync.Map.Range()` with bare type assertions triggers errcheck — use checked form.
- **block-silent-ignore hook**: `default:` case in switch blocked. Used explicit cases + fallback return.

## Files

- `internal/core/slogutil/slogutil.go` — LevelRegistry, LevelVar switch, ListLevels/SetLevel/LevelString/ResetLevelRegistry
- `internal/component/bgp/plugins/cmd/log/` — handler package (log.go, doc.go, schema/)
- `internal/component/bgp/reactor/reactor.go` — blank import added
- `test/plugin/cli-log-show.ci`, `test/plugin/cli-log-set.ci` — functional tests
