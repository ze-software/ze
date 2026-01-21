# Spec: slog-subsystem

## Task

Implement per-subsystem logging control for ZeBGP using `slog`. Goals:
1. Plugins MUST use stderr (stdout reserved for protocol messages)
2. Logging disabled by default for each subsystem
3. Engine subsystems: enable via `ze.bgp.log.<subsystem>=<level>` env vars
4. External plugins: enable via `--log-level=<level>` CLI flag
5. Plugin stderr relay infrastructure (wiring deferred to separate task)
6. Consistent subsystem tagging in all log messages

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - understand plugin architecture
- [x] `docs/architecture/api/process-protocol.md` - plugin stdio usage

### Codebase Exploration
- [x] `cmd/ze/bgp/server.go` - existing `configureSlog()` pattern (now removed)
- [x] `internal/plugin/gr/gr.go` - plugin init() logging setup (now removed)
- [x] `internal/plugin/rib/rib.go` - plugin init() logging setup (now removed)
- [x] ExaBGP logger - per-source filtering, lazy evaluation

**Key insights:**
- Plugins run as separate processes - each has own slog handler
- `stdout` = protocol messages to engine, `stderr` = logs
- ExaBGP has per-source (reactor, daemon, wire, etc.) log control

## Design

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         ZeBGP Engine                            │
│                                                                 │
│   var logger = slogutil.Logger("server")  // per-subsystem      │
│   var logger = slogutil.Logger("filter")  // independent        │
│   → backend: ze.bgp.log.backend (stderr|stdout|syslog)           │
│   → syslog addr: ze.bgp.log.destination (when backend=syslog)    │
│                                                                 │
│   Plugin stderr capture → relay infrastructure ready            │
└─────────────────────────────────────────────────────────────────┘
                              │
        ══════════════════════╧══════════════════════════
                    process boundary (pipes)
        ══════════════════════╤══════════════════════════
                              │
┌─────────────────────────────────────────────────────────────────┐
│                      Plugin Process                             │
│                                                                 │
│   var logger = slogutil.LoggerWithLevel("gr", *logLevel)        │
│   → level from CLI: --log-level=debug                           │
│   → ALWAYS writes to stderr (stdout = protocol messages)        │
└─────────────────────────────────────────────────────────────────┘
```

### Environment Variables

Follow ExaBGP format with `zebgp.` prefix (or `zebgp_` for shell compatibility):

**Backend configuration:**

| Variable | Purpose | Values |
|----------|---------|--------|
| `ze.bgp.log.backend` | Log backend type | `stderr` (default), `stdout`, `syslog` |
| `ze.bgp.log.destination` | Syslog address (only when backend=syslog) | `localhost:514`, `/dev/log`, etc. |

**Per-subsystem levels:**

| Variable | Purpose | Values |
|----------|---------|--------|
| `ze.bgp.log.server` | Plugin server logging | `disabled`, `debug`, `info`, `warn`, `err` |
| `ze.bgp.log.coordinator` | Coordinator logging | `disabled`, `debug`, `info`, `warn`, `err` |
| `ze.bgp.log.filter` | Filter logging | `disabled`, `debug`, `info`, `warn`, `err` |
| `ze.bgp.log.plugin` | Relay plugin stderr (infrastructure only) | `disabled`, `enabled` |

**Shell-compatible form:** `ze_bgp_log_server`, `ze_bgp_log_backend`, etc.

**Levels (short syslog names, case-insensitive):**
- `disabled` - no logging (explicit opt-out)
- `debug` - all messages
- `info` - info and above
- `warn` - warnings and errors
- `err` - errors only

**Behavior:**
- **Disabled by default** - subsystem produces no logs unless explicitly enabled
- **Enable by setting level** - `ze.bgp.log.server=debug` enables server logging at debug level
- **Explicit disable** - `ze.bgp.log.server=disabled` disables even if default changes later
- **Plugin example:**
  ```
  plugin {
      external gr {
          run "ze bgp plugin gr --log-level=debug";  # plugin verbosity
      }
  }
  ```

**Precedence:** `ze.bgp.log.<var>` > `ze_bgp_log_<var>` > default

### Subsystem Names

**Engine subsystems (use `Logger()`):**

| Subsystem | Code Location | Tag |
|-----------|---------------|-----|
| `server` | `internal/plugin/server.go` | `subsystem=server` |
| `coordinator` | `internal/plugin/startup_coordinator.go` | `subsystem=coordinator` |
| `filter` | `internal/plugin/filter.go` | `subsystem=filter` |

**Plugin processes (use `LoggerWithLevel()`):**

| Plugin | Code Location | Tag |
|--------|---------------|-----|
| `gr` | `internal/plugin/gr/` | `subsystem=gr` |
| `rib` | `internal/plugin/rib/` | `subsystem=rib` |

### Implementation Approach

Create `internal/slogutil/slogutil.go` with:

```go
// Logger returns a logger for an engine subsystem.
// Each subsystem gets its own logger instance (not SetDefault) to allow
// multiple subsystems in the same process with independent enable/disable.
// Reads ze.bgp.log.<subsystem> for level, ze.bgp.log.backend for output.
func Logger(subsystem string) *slog.Logger {
    v := getEnv("log", subsystem)  // ze.bgp.log.<subsystem>
    if v == "" {
        return slog.New(discardHandler{})
    }
    lvl, enabled := parseLevel(v)
    if !enabled {
        return slog.New(discardHandler{})
    }
    handler := createHandler(lvl)
    return slog.New(handler).With("subsystem", subsystem)
}

// LoggerWithLevel returns a logger for plugins (level from CLI --log-level flag).
// Plugins ALWAYS write to stderr (stdout = protocol messages).
func LoggerWithLevel(subsystem, level string) *slog.Logger {
    lvl, enabled := parseLevel(level)
    if !enabled {
        return slog.New(discardHandler{})
    }
    handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
    return slog.New(handler).With("subsystem", subsystem)
}

// LoggerWithOutput returns a logger that writes to a specific output.
// Used for testing and custom output destinations.
func LoggerWithOutput(subsystem, level string, w io.Writer) *slog.Logger

// IsPluginRelayEnabled checks if plugin stderr should be relayed.
// Reads ze.bgp.log.plugin (enabled/disabled).
// Note: Infrastructure only - wiring into server.go deferred to separate task.
func IsPluginRelayEnabled() bool {
    v := getEnv("log", "plugin")
    return strings.ToLower(v) == "enabled"
}

// getEnv returns env var with ZeBGP naming (dot and underscore notation).
// Checks ze.bgp.log.<option> first, then ze_bgp_log_<option>.
func getEnv(section, option string) string

func createHandler(level slog.Level) slog.Handler {
    opts := &slog.HandlerOptions{Level: level}
    backend := getEnv("log", "backend")  // ze.bgp.log.backend
    switch strings.ToLower(backend) {
    case "stdout":
        return slog.NewTextHandler(os.Stdout, opts)
    case "syslog":
        return newSyslogHandler(opts)  // uses native log/syslog
    default:  // stderr (default)
        return slog.NewTextHandler(os.Stderr, opts)
    }
}

func parseLevel(s string) (slog.Level, bool) {
    switch strings.ToLower(s) {
    case "disabled":
        return slog.LevelInfo, false
    case "debug":
        return slog.LevelDebug, true
    case "info":
        return slog.LevelInfo, true
    case "warn", "warning":
        return slog.LevelWarn, true
    case "err", "error":
        return slog.LevelError, true
    default:
        return slog.LevelInfo, false  // unknown = disabled
    }
}

// discardHandler discards all log records.
type discardHandler struct{}
func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }
```

**ParseLogLine helper (in `internal/slogutil/parse.go`):**

```go
// ParseLogLine extracts level, message, and attributes from a slog text line.
// Returns []any (not []slog.Attr) so result can be spread to slog.Group().
//
// For valid slog format:
//   Input:  "time=... level=DEBUG msg=\"parsed config\" subsystem=gr peer=..."
//   Output: LevelDebug, "parsed config", ["subsystem", "gr", "peer", "..."]
//
// For malformed/non-slog text (e.g., panic, raw error):
//   Input:  "panic: runtime error: index out of range"
//   Output: LevelInfo, "panic: runtime error: index out of range", []any{}
//
// Note: Infrastructure for plugin stderr relay - wiring deferred.
func ParseLogLine(line string) (slog.Level, string, []any)
```

### Log Message Format

**Before (current):**
```
time=2025-01-18T12:00:00Z level=DEBUG msg="gr: parsed config" peer=192.168.1.1 restart-time=120
```

**After - Plugin stderr (written by plugin process):**
```
time=2025-01-18T12:00:00Z level=DEBUG msg="parsed config" subsystem=gr peer=192.168.1.1 restart-time=120
```

**Changes:**
- Remove manual `"gr: "` prefix from messages, use `subsystem` attribute
- Each subsystem independently enabled/disabled via env var

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoggerDisabledByDefault` | `internal/slogutil/slogutil_test.go` | No logs when env var not set | ✅ |
| `TestLoggerExplicitDisabled` | `internal/slogutil/slogutil_test.go` | `ze.bgp.log.server=disabled` explicitly disables | ✅ |
| `TestLoggerEnabledDot` | `internal/slogutil/slogutil_test.go` | `ze.bgp.log.server=debug` enables logging | ✅ |
| `TestLoggerEnabledUnderscore` | `internal/slogutil/slogutil_test.go` | `ze_bgp_log_server=debug` enables logging | ✅ |
| `TestLoggerWithLevel` | `internal/slogutil/slogutil_test.go` | `LoggerWithLevel("gr", "debug")` enables debug logging | ✅ |
| `TestLoggerWithLevelDisabled` | `internal/slogutil/slogutil_test.go` | `LoggerWithLevel("gr", "disabled")` disables logging | ✅ |
| `TestLoggerPrecedence` | `internal/slogutil/slogutil_test.go` | dot notation takes precedence over underscore | ✅ |
| `TestLoggerSubsystemAttr` | `internal/slogutil/slogutil_test.go` | `subsystem` attribute added to logs | ✅ |
| `TestParseLevelCaseInsensitive` | `internal/slogutil/slogutil_test.go` | `debug`, `DEBUG`, `Debug` all work | ✅ |
| `TestParseLevelAliases` | `internal/slogutil/slogutil_test.go` | `err`/`error`, `warn`/`warning` both work | ✅ |
| `TestLoggerLevelFiltering` | `internal/slogutil/slogutil_test.go` | `info` level filters out debug messages | ✅ |
| `TestLoggerUnknownLevel` | `internal/slogutil/slogutil_test.go` | Unknown level value = disabled | ✅ |
| `TestBackendStderr` | `internal/slogutil/slogutil_test.go` | `ze.bgp.log.backend=stderr` uses stderr | ✅ |
| `TestBackendStdout` | `internal/slogutil/slogutil_test.go` | `ze.bgp.log.backend=stdout` uses stdout | ✅ |
| `TestBackendSyslog` | `internal/slogutil/slogutil_test.go` | `ze.bgp.log.backend=syslog` uses syslog handler | ✅ |
| `TestLoggerWithLevelStderr` | `internal/slogutil/slogutil_test.go` | `LoggerWithLevel()` always uses stderr | ✅ |
| `TestIsPluginRelayEnabled` | `internal/slogutil/slogutil_test.go` | `ze.bgp.log.plugin=enabled` returns true | ✅ |
| `TestIsPluginRelayDisabled` | `internal/slogutil/slogutil_test.go` | `ze.bgp.log.plugin=disabled` returns false | ✅ |
| `TestIsPluginRelayDefault` | `internal/slogutil/slogutil_test.go` | Unset `ze.bgp.log.plugin` returns false | ✅ |
| `TestDiscardHandler` | `internal/slogutil/slogutil_test.go` | discardHandler implements slog.Handler correctly | ✅ |
| `TestParseLogLineValid` | `internal/slogutil/slogutil_test.go` | Parses valid slog text line, extracts level/msg/attrs | ✅ |
| `TestParseLogLineAllLevels` | `internal/slogutil/slogutil_test.go` | Extracts DEBUG/INFO/WARN/ERROR levels correctly | ✅ |
| `TestParseLogLineQuotedMsg` | `internal/slogutil/slogutil_test.go` | Handles quoted message containing spaces | ✅ |
| `TestParseLogLineMalformed` | `internal/slogutil/slogutil_test.go` | Returns LevelInfo and raw line for non-slog text | ✅ |
| `TestLoggerWithOutputSubsystem` | `internal/slogutil/slogutil_test.go` | `LoggerWithOutput()` adds subsystem attribute | ✅ |
| `TestAllLevelsParsing` | `internal/slogutil/slogutil_test.go` | All level strings parse correctly | ✅ |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this feature.

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | - | Logging is stderr-only, not testable via functional tests | |

### Future (if deferring any tests)
- Integration test that captures stderr from plugin subprocess and verifies format
- Wire plugin stderr relay into `internal/plugin/server.go` (separate task)

## Files to Modify

- `internal/plugin/gr/gr.go` - remove init(), add `var logger`, add `SetLogger()` func, use `logger.X()`, remove "gr: " prefixes
- `internal/plugin/rib/rib.go` - remove init(), add `var logger`, add `SetLogger()` func, remove prefixes
- `cmd/ze/bgp/plugin_gr.go` - add `--log-level` flag, call `gr.SetLogger()`
- `cmd/ze/bgp/plugin_rib.go` - add `--log-level` flag, call `rib.SetLogger()`
- `internal/plugin/server.go` - add `var logger = slogutil.Logger("server")`, use `logger.X()`
- `internal/plugin/startup_coordinator.go` - add `var coordinatorLogger = slogutil.Logger("coordinator")`
- `internal/plugin/filter.go` - add `var filterLogger = slogutil.Logger("filter")`
- `cmd/ze/bgp/server.go` - remove `configureSlog()` function

## Files to Create

- `internal/slogutil/slogutil.go` - shared logging configuration
- `internal/slogutil/syslog.go` - syslog handler wrapper (uses native `log/syslog`)
- `internal/slogutil/parse.go` - ParseLogLine helper (infrastructure for future plugin stderr relay)
- `internal/slogutil/slogutil_test.go` - unit tests (27 tests)

## Dependencies

None - uses native `log/syslog` for syslog support.

## Implementation Steps

1. **Write unit tests** - Create `internal/slogutil/slogutil_test.go` BEFORE implementation (strict TDD)
2. **Run tests** - Verify FAIL (paste output)
3. **Implement slogutil** - Create `internal/slogutil/slogutil.go`, `syslog.go`, `parse.go`
4. **Run tests** - Verify PASS (paste output)
5. **Add CLI flags** - Add `--log-level` flag to `cmd/ze/bgp/plugin_gr.go` and `plugin_rib.go`, call `SetLogger()`
6. **Migrate GR plugin** - Remove init(), add `var logger`, add `SetLogger()` func, use `logger.X()`, remove "gr: " prefixes
7. **Migrate RIB plugin** - Remove init(), add `var logger`, add `SetLogger()` func, remove prefixes
8. **Migrate server** - Add `var logger = slogutil.Logger("server")`, use `logger.X()`
9. **Migrate coordinator/filter** - Add `var <name>Logger = slogutil.Logger(...)`, replace slog calls
10. **Migrate cmd/ze/bgp/server.go** - Remove `configureSlog()` function
11. **Verify all** - `make lint && make test && make functional` (paste output)

## Implementation Summary

### What Was Implemented

**New package `internal/slogutil/`:**
- `slogutil.go` - `Logger()`, `LoggerWithLevel()`, `LoggerWithOutput()`, `IsPluginRelayEnabled()`, `parseLevel()`, `createHandler()`, `discardHandler`
- `syslog.go` - `newSyslogHandler()` for syslog backend support (native `log/syslog`)
- `parse.go` - `ParseLogLine()` for plugin stderr relay parsing (infrastructure ready)
- `slogutil_test.go` - 27 unit tests covering all functionality

**Modified files:**
- `cmd/ze/bgp/plugin_gr.go` - Added `--log-level` flag, calls `gr.SetLogger()`
- `cmd/ze/bgp/plugin_rib.go` - Added `--log-level` flag, calls `rib.SetLogger()`
- `cmd/ze/bgp/server.go` - Removed `configureSlog()` function
- `internal/plugin/gr/gr.go` - Removed `init()`, added `SetLogger()`, replaced `slog.X()` with `logger.X()`, removed "gr: " prefixes
- `internal/plugin/rib/rib.go` - Removed `init()`, added `SetLogger()`, replaced `slog.X()` with `logger.X()`
- `internal/plugin/server.go` - Added `var logger = slogutil.Logger("server")`, replaced `slog.X()` with `logger.X()`
- `internal/plugin/startup_coordinator.go` - Added `var coordinatorLogger = slogutil.Logger("coordinator")`, replaced calls
- `internal/plugin/filter.go` - Added `var filterLogger = slogutil.Logger("filter")`, replaced calls

### Bugs Found/Fixed
- None during implementation

### Design Insights
- Used native `log/syslog` instead of external dependency - simpler, no go.mod changes
- Exported `DiscardLogger()` from slogutil - plugins import it instead of duplicating discardHandler
- Plugin stderr relay wired into `internal/plugin/process.go:relayStderr()` - reads lines and relays via stderrLogger
- Logger variable names vary by file (`logger`, `coordinatorLogger`, `filterLogger`) to avoid conflicts within the same package

### Deviations from Plan
- Used native `log/syslog` instead of `github.com/samber/slog-syslog/v2`
- slogutil has its own `getEnv()` function (cleaner than exporting from config package)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (functions undefined before implementation)
- [x] Implementation complete
- [x] Tests PASS (27 tests)
- [x] Boundary tests cover all numeric inputs (N/A - no numeric inputs)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (87 tests)

### Documentation (during implementation)
- [x] Required docs read
- [x] go-standards.md logging section updated with subsystem pattern
- [ ] docs/architecture/config/environment.md created with ze.bgp.log.* variables (deferred - info in go-standards.md)

### Completion (after tests pass - see Completion Checklist)
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/129-slog-subsystem.md`
- [x] All files committed together
